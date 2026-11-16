package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/entity"
)

// Discord Milestone A (identity #46, Thought-Processor #127): discord.reply is a bound action with
// its own non-default owner grant. Core obtains a single-use decision for one exact reply; only the
// Discord door redeems it, once, and Identity re-checks the grant at that moment.
// Contract: docs/DISCORD_REPLY_CONTRACT.md. All values here are fixtures.

const (
	doorKey = "0123456789abcdef0123456789abcdef-door!!"
	dReply  = "discord.reply"
	pass    = "correct horse battery"
)

// discordBinding is the contract's binding, written out independently of any production code.
func discordBinding(inboundID, sourceMessageID, senderID, channelID, replyText string) string {
	text := sha256.Sum256([]byte(replyText))
	sum := sha256.Sum256([]byte(strings.Join([]string{
		"tass.discord.reply/1", "discord-door", inboundID, sourceMessageID, senderID, channelID,
		"discord:" + channelID, hex.EncodeToString(text[:]),
	}, "\n")))
	return hex.EncodeToString(sum[:])
}

type discordFixture struct {
	st     *entity.Store
	mint   func(key, action, binding string) map[string]any
	redeem func(key, id, action, binding string) map[string]any
	grant  func(body any) int
	raw    func(method, path, key string, body any) (int, map[string]any)
	held   func(g string) bool
}

func discordSetup(t *testing.T) discordFixture {
	t.Helper()
	st, _ := entity.Open(t.TempDir(), time.Hour)
	h := routes(st, map[string]string{engineKey: "engine", coreKey: "core", doorKey: entity.DoorService}, "page-secret")
	owner := map[string]string{"X-Identity-Page": "page-secret"}
	req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", owner, map[string]string{"name": "TASS", "passphrase": pass})
	raw := func(method, path, key string, body any) (int, map[string]any) {
		return req(t, h, method, "identity:8796", path, map[string]string{"Authorization": "Bearer " + key}, body)
	}
	return discordFixture{
		st: st, raw: raw,
		mint: func(key, action, binding string) map[string]any {
			_, out := raw("POST", "/v1/authorize", key, map[string]string{"action": action, "binding": binding, "detail": "reply to m_fixture"})
			return out
		},
		redeem: func(key, id, action, binding string) map[string]any {
			_, out := raw("POST", "/v1/decisions/redeem", key, map[string]any{"decision_id": id, "action": action, "binding": binding})
			return out
		},
		grant: func(body any) int {
			c, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/skill", owner, body)
			return c
		},
		held: func(g string) bool {
			for _, x := range st.State().Entity.Grants {
				if x == g {
					return true
				}
			}
			return false
		},
	}
}

func TestDiscordReplyBindingMatchesTheDocumentedVector(t *testing.T) {
	const want = "f5ab28d15e23a3850274a3604fb999d433bb3980d9e4ff970bec8029d095a25c"
	got := discordBinding("m_4d442e5526b608cfe449", "1562034986897375235", "2100000000000000001", "1100000000000000001",
		"TEST FIXTURE reply: hello from TASS")
	if got != want {
		t.Fatalf("binding = %s, want %s", got, want)
	}
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "DISCORD_REPLY_CONTRACT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), want) || !strings.Contains(string(doc), "tass.discord.reply/1") {
		t.Fatal("docs/DISCORD_REPLY_CONTRACT.md does not carry the tested binding vector")
	}
	// Every bound field changes the binding: message, sender, channel (and so destination), content.
	base := []string{"m_a", "1", "2", "3", "hello"}
	seen := map[string]bool{discordBinding(base[0], base[1], base[2], base[3], base[4]): true}
	for i := range base {
		v := append([]string(nil), base...)
		v[i] += "x"
		b := discordBinding(v[0], v[1], v[2], v[3], v[4])
		if seen[b] {
			t.Fatalf("changing field %d did not change the binding", i)
		}
		seen[b] = true
	}
}

func TestDiscordReplyIsNonDefaultAndOnlyTheDoorRedeems(t *testing.T) {
	f := discordSetup(t)
	bind := discordBinding("m_1", "11", "22", "33", "first reply")
	other := discordBinding("m_1", "11", "22", "33", "another reply text")

	// Disabled by default: a new, unlocked entity does not hold it and nothing can be obtained.
	if f.held(dReply) {
		t.Fatal("a new entity holds discord.reply by default")
	}
	for _, g := range entity.DefaultGrants {
		if g == dReply {
			t.Fatal("discord.reply is a default grant")
		}
	}
	if a := f.mint(coreKey, dReply, bind); a["allow"] != false {
		t.Fatalf("obtained without the grant: %v", a)
	}
	// Not implied by the other owner grants.
	for _, s := range []string{"cognition.nudge-process", "cognition.nudge-draft", "cognition.experiment-run", "lesson-workspace.list-lessons"} {
		f.grant(map[string]any{"skill": s, "passphrase": pass})
	}
	if a := f.mint(coreKey, dReply, bind); a["allow"] != false || f.held(dReply) {
		t.Fatalf("another grant allowed a Discord reply: %v", a)
	}
	// Enabling needs the passphrase and adds exactly this grant.
	if c := f.grant(map[string]any{"skill": dReply, "passphrase": "wrong passphrase!!"}); c == 200 || f.held(dReply) {
		t.Fatalf("granted with a wrong passphrase (HTTP %d)", c)
	}
	before := len(f.st.State().Entity.Grants)
	if c := f.grant(map[string]any{"skill": dReply, "passphrase": pass}); c != 200 || !f.held(dReply) {
		t.Fatalf("grant: HTTP %d, held %v", c, f.held(dReply))
	}
	if len(f.st.State().Entity.Grants) != before+1 || f.held("skill.run:"+dReply) {
		t.Fatalf("enabling discord.reply added something else: %v", f.st.State().Entity.Grants)
	}
	// Bound only; only Core obtains.
	if _, a := f.raw("POST", "/v1/authorize", coreKey, map[string]string{"action": dReply}); a["allow"] != false {
		t.Fatalf("an unbound discord.reply decision was allowed: %v", a)
	}
	if a := f.mint(coreKey, dReply, "not-hex"); a["allow"] != false {
		t.Fatal("a bad binding was accepted")
	}
	if a := f.mint(engineKey, dReply, bind); a["allow"] != false {
		t.Fatal("the Engine obtained a discord.reply decision")
	}
	if c, a := f.raw("POST", "/v1/authorize", doorKey, map[string]string{"action": dReply, "binding": bind}); c != 403 || a["allow"] == true {
		t.Fatalf("the door obtained its own decision: HTTP %d %v", c, a)
	}
	d := f.mint(coreKey, dReply, bind)
	id, _ := d["decision_id"].(string)
	if d["allow"] != true || id == "" || d["expires_at"] == nil {
		t.Fatalf("mint: %v", d)
	}
	// Only the door redeems: not Core, not the Engine.
	if r := f.redeem(coreKey, id, dReply, bind); r["allow"] != false {
		t.Fatal("Core redeemed its own decision")
	}
	if r := f.redeem(engineKey, id, dReply, bind); r["allow"] != false {
		t.Fatal("the Engine redeemed a discord.reply decision")
	}
	// Exact binding (other reply text) and exact action.
	if r := f.redeem(doorKey, id, dReply, other); r["allow"] != false {
		t.Fatal("a decision was redeemed for different reply content")
	}
	if r := f.redeem(doorKey, id, "cognition.nudge.process", bind); r["allow"] != false {
		t.Fatal("the door redeemed under another action")
	}
	// None of the refusals consumed it; the door redeems once.
	if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != true || r["service"] != entity.DoorService {
		t.Fatalf("redeem: %v", r)
	}
	if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != false {
		t.Fatal("redeemed twice")
	}
	// The door cannot touch other bound actions, and the Engine still redeems its own.
	n := f.mint(coreKey, "cognition.nudge.process", bindA)
	nid, _ := n["decision_id"].(string)
	if n["allow"] != true {
		t.Fatalf("nudge mint: %v", n)
	}
	if r := f.redeem(doorKey, nid, "cognition.nudge.process", bindA); r["allow"] != false {
		t.Fatal("the door redeemed a nudge decision")
	}
	if r := f.redeem(doorKey, nid, dReply, bindA); r["allow"] != false {
		t.Fatal("the door redeemed a nudge decision as a Discord reply")
	}
	if r := f.redeem(engineKey, nid, "cognition.nudge.process", bindA); r["allow"] != true {
		t.Fatalf("the Engine can no longer redeem its own action: %v", r)
	}
}

func TestDiscordReplyIsRevocableAtThePointOfUse(t *testing.T) {
	f := discordSetup(t)
	f.grant(map[string]any{"skill": dReply, "passphrase": pass})
	bind := discordBinding("m_2", "11", "22", "33", "reply")

	// Revoked after Core obtained the decision, before the door redeems: not sent.
	d := f.mint(coreKey, dReply, bind)
	if c := f.grant(map[string]any{"skill": dReply, "revoke": true}); c != 200 || f.held(dReply) {
		t.Fatalf("revoke without a passphrase: HTTP %d, held %v", c, f.held(dReply))
	}
	if r := f.redeem(doorKey, d["decision_id"].(string), dReply, bind); r["allow"] != false {
		t.Fatal("redeemed after the grant was revoked")
	}
	if a := f.mint(coreKey, dReply, bind); a["allow"] != false {
		t.Fatal("obtained after the grant was revoked")
	}
	// Locked after Core obtained the decision: not sent.
	f.grant(map[string]any{"skill": dReply, "passphrase": pass})
	d2 := f.mint(coreKey, dReply, bind)
	if d2["allow"] != true {
		t.Fatalf("mint after re-grant: %v", d2)
	}
	f.st.Lock()
	if r := f.redeem(doorKey, d2["decision_id"].(string), dReply, bind); r["allow"] != false {
		t.Fatal("redeemed while LOCKED")
	}
	if a := f.mint(coreKey, dReply, bind); a["allow"] != false {
		t.Fatal("obtained while LOCKED")
	}
}

func TestDiscordReplyDecisionExpiresAndDoesNotSurviveARestart(t *testing.T) {
	f := discordSetup(t)
	f.grant(map[string]any{"skill": dReply, "passphrase": pass})
	bind := discordBinding("m_3", "11", "22", "33", "reply")
	now := time.Now()
	f.st.Now = func() time.Time { return now }
	d := f.mint(coreKey, dReply, bind)
	now = now.Add(entity.BoundTTL + time.Second)
	if r := f.redeem(doorKey, d["decision_id"].(string), dReply, bind); r["allow"] != false {
		t.Fatal("an expired decision was redeemed")
	}
	d2 := f.mint(coreKey, dReply, bind)
	if d2["allow"] != true {
		t.Fatalf("mint: %v", d2)
	}
	st2, _ := entity.Open(f.st.Dir(), time.Hour)
	if r := st2.RedeemBound(entity.DoorService, d2["decision_id"].(string), dReply, bind, true); r.Allow {
		t.Fatal("a decision survived an Identity restart")
	}
}

func TestDoorKeyIsRedeemOnlyAndAuditHasNoText(t *testing.T) {
	f := discordSetup(t)
	f.grant(map[string]any{"skill": dReply, "passphrase": pass})
	// The door's key opens nothing but the redeem route.
	if c, a := f.raw("POST", "/v1/authorize", doorKey, map[string]string{"action": "conversation"}); c != 403 || a["allow"] == true {
		t.Fatalf("the door got a plain authorization: HTTP %d %v", c, a)
	}
	if c, _ := f.raw("GET", "/v1/github/lesson", doorKey, nil); c != 403 {
		t.Fatalf("the door reached the lesson route: HTTP %d", c)
	}
	if !entity.RedeemOnly(entity.DoorService) || entity.RedeemOnly("core") || entity.RedeemOnly("engine") {
		t.Fatal("RedeemOnly is wrong for a known service")
	}
	// A wrong or missing key is refused before anything else.
	if c, _ := f.raw("POST", "/v1/decisions/redeem", "0123456789abcdef0123456789abcdef-wrong!", map[string]any{"decision_id": "dec_x", "action": dReply, "binding": bindA}); c != 401 {
		t.Fatalf("a wrong key reached redeem: HTTP %d", c)
	}
	// Audit: obtain and redeem are recorded with the service and binding, and no reply text.
	const secretText = "PRIVATE reply text that must not be recorded"
	bind := discordBinding("m_4", "11", "22", "33", secretText)
	d := f.mint(coreKey, dReply, bind)
	if r := f.redeem(doorKey, d["decision_id"].(string), dReply, bind); r["allow"] != true {
		t.Fatalf("redeem: %v", r)
	}
	log, err := os.ReadFile(filepath.Join(f.st.Dir(), "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(log)
	if !strings.Contains(s, `"mode":"mint"`) || !strings.Contains(s, `"mode":"redeem"`) ||
		!strings.Contains(s, `"service":"discord-door"`) || !strings.Contains(s, bind) {
		t.Fatalf("decisions.jsonl lacks the obtain/redeem record:\n%s", s)
	}
	if strings.Contains(s, secretText) {
		t.Fatal("decisions.jsonl contains reply text")
	}
}
