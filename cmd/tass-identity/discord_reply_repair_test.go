package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/entity"
)

// Repairs for the independent audit of identity #47 (NEEDS REPAIR):
//   A. discord.reply could be re-answered allow=true through the non-consuming CHECK route.
//   B. concurrent redemptions of one decision must yield exactly one success.
//   C. one key on two service lines silently collapsed two services into one.

func (f discordFixture) check(key, id, action, binding string) (int, map[string]any) {
	return f.raw("POST", "/v1/decisions/redeem", key, map[string]any{"decision_id": id, "action": action, "binding": binding, "check": true})
}

// Repair A: the old bypass. Redeem once, then ask "check" repeatedly: must never answer allow.
func TestDiscordReplyCheckBypassIsClosed(t *testing.T) {
	f := discordSetup(t)
	f.grant(map[string]any{"skill": dReply, "passphrase": pass})
	bind := discordBinding("m_bypass", "11", "22", "33", "reply")
	id := f.mint(coreKey, dReply, bind)["decision_id"].(string)

	// A check before the redeem is refused and does not consume the decision.
	if _, r := f.check(doorKey, id, dReply, bind); r["allow"] != false {
		t.Fatalf("check before redeem allowed: %v", r)
	}
	if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != true {
		t.Fatalf("the one legitimate redeem failed: %v", r)
	}
	// After the one redeem, no service can get allow=true again by checking.
	for i := 0; i < 5; i++ {
		for _, key := range []string{doorKey, engineKey, coreKey} {
			if _, r := f.check(key, id, dReply, bind); r["allow"] != false {
				t.Fatalf("check #%d after redeem answered allow (%v)", i, r)
			}
		}
	}
	// Same at the Store level, independent of HTTP.
	if r := f.st.RedeemBound(entity.DoorService, id, dReply, bind, false); r.Allow || !strings.Contains(r.Reason, "single point of use") {
		t.Fatalf("store-level check: allow=%v reason=%q", r.Allow, r.Reason)
	}
	if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != false {
		t.Fatal("redeemed a second time")
	}
}

// Audit finding 1: the check must also stay refused after the original 120-second window, when
// the used decision is still in memory (its expiry was moved to +24 h for Engine checks).
func TestDiscordReplyCheckRefusedAfterTheOriginalWindow(t *testing.T) {
	f := discordSetup(t)
	f.grant(map[string]any{"skill": dReply, "passphrase": pass})
	now := time.Now()
	f.st.Now = func() time.Time { return now }
	bind := discordBinding("m_late", "11", "22", "33", "reply")
	id := f.mint(coreKey, dReply, bind)["decision_id"].(string)
	if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != true || r["mode"] != "redeem" {
		t.Fatalf("redeem: %v", r)
	}
	for _, later := range []time.Duration{entity.BoundTTL + time.Second, time.Hour, 23 * time.Hour} {
		now = now.Add(later)
		if _, r := f.check(doorKey, id, dReply, bind); r["allow"] != false {
			t.Fatalf("check %v later answered allow: %v", later, r)
		}
		if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != false {
			t.Fatalf("redeem %v later answered allow: %v", later, r)
		}
	}
}

// Repair A: every way a door redemption can be wrong is refused. Each case uses a fresh, valid
// decision; for the cases that are wrong only in the request, the decision is still redeemable
// afterwards (a refusal never consumes).
func TestDiscordReplyNegativeRedemptions(t *testing.T) {
	bind := discordBinding("m_neg", "11", "22", "33", "reply")
	other := discordBinding("m_neg", "11", "22", "33", "different reply")
	type tc struct {
		name        string
		before      func(f discordFixture, now *time.Time)
		key, action string
		binding     string
		otherID     bool
		stillUsable bool
	}
	cases := []tc{
		{name: "wrong service: core", key: coreKey, action: dReply, binding: bind, stillUsable: true},
		{name: "wrong service: engine", key: engineKey, action: dReply, binding: bind, stillUsable: true},
		{name: "wrong action", key: doorKey, action: "cognition.nudge.process", binding: bind, stillUsable: true},
		{name: "wrong action: experiment", key: doorKey, action: "cognition.experiment.run", binding: bind, stillUsable: true},
		{name: "mismatched binding", key: doorKey, action: dReply, binding: other, stillUsable: true},
		{name: "empty binding", key: doorKey, action: dReply, binding: "", stillUsable: true},
		{name: "unknown decision id", key: doorKey, action: dReply, binding: bind, otherID: true, stillUsable: true},
		{name: "expired", key: doorKey, action: dReply, binding: bind,
			before: func(f discordFixture, now *time.Time) { *now = now.Add(entity.BoundTTL + time.Second) }},
		{name: "revoked", key: doorKey, action: dReply, binding: bind,
			before: func(f discordFixture, _ *time.Time) { f.grant(map[string]any{"skill": dReply, "revoke": true}) }},
		{name: "locked", key: doorKey, action: dReply, binding: bind,
			before: func(f discordFixture, _ *time.Time) { f.st.Lock() }},
		{name: "reused", key: doorKey, action: dReply, binding: bind,
			before: func(f discordFixture, _ *time.Time) {}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := discordSetup(t)
			f.grant(map[string]any{"skill": dReply, "passphrase": pass})
			now := time.Now()
			f.st.Now = func() time.Time { return now }
			id := f.mint(coreKey, dReply, bind)["decision_id"].(string)
			if c.name == "reused" {
				if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != true {
					t.Fatalf("first redeem: %v", r)
				}
			}
			if c.before != nil {
				c.before(f, &now)
			}
			use := id
			if c.otherID {
				use = "dec_0000000000000000"
			}
			if r := f.redeem(c.key, use, c.action, c.binding); r["allow"] != false {
				t.Fatalf("%s was redeemed: %v", c.name, r)
			}
			if _, r := f.check(c.key, use, c.action, c.binding); r["allow"] != false {
				t.Fatalf("%s was allowed by check: %v", c.name, r)
			}
			if c.stillUsable {
				if r := f.redeem(doorKey, id, dReply, bind); r["allow"] != true {
					t.Fatalf("a refused attempt consumed the decision: %v", r)
				}
			}
		})
	}
}

// The established non-Discord flow is unchanged: the Engine redeems once, then CHECKS at later
// points of use, and a check sees a revocation.
func TestEngineRedeemThenCheckStillWorks(t *testing.T) {
	f := discordSetup(t)
	for _, g := range []string{"cognition.nudge-process", "cognition.experiment-run"} {
		if c := f.grant(map[string]any{"skill": g, "passphrase": pass}); c != 200 {
			t.Fatalf("grant %s: HTTP %d", g, c)
		}
	}
	for _, action := range []string{"cognition.nudge.process", "cognition.experiment.run"} {
		id := f.mint(coreKey, action, bindA)["decision_id"].(string)
		if r := f.redeem(engineKey, id, action, bindA); r["allow"] != true {
			t.Fatalf("%s redeem: %v", action, r)
		}
		for i := 0; i < 3; i++ {
			if _, r := f.check(engineKey, id, action, bindA); r["allow"] != true || r["mode"] != "check" {
				t.Fatalf("%s check #%d: %v", action, i, r)
			}
		}
		if r := f.redeem(engineKey, id, action, bindA); r["allow"] != false {
			t.Fatalf("%s redeemed twice", action)
		}
	}
	id := f.mint(coreKey, "cognition.nudge.process", bindA)["decision_id"].(string)
	f.redeem(engineKey, id, "cognition.nudge.process", bindA)
	f.grant(map[string]any{"skill": "cognition.nudge-process", "revoke": true})
	if _, r := f.check(engineKey, id, "cognition.nudge.process", bindA); r["allow"] != false {
		t.Fatal("a check did not see the revocation")
	}
	// cognition.nudge.stop (no grant needed) keeps redeem-then-check too.
	sid := f.mint(coreKey, "cognition.nudge.stop", bindA)["decision_id"].(string)
	if r := f.redeem(engineKey, sid, "cognition.nudge.stop", bindA); r["allow"] != true {
		t.Fatalf("stop redeem: %v", r)
	}
	if _, r := f.check(engineKey, sid, "cognition.nudge.stop", bindA); r["allow"] != true {
		t.Fatalf("stop check: %v", r)
	}
	if entity.SinglePointOfUse["cognition.nudge.process"] || entity.SinglePointOfUse["cognition.nudge.stop"] ||
		entity.SinglePointOfUse["cognition.experiment.run"] || !entity.SinglePointOfUse[dReply] {
		t.Fatalf("SinglePointOfUse is not exactly {discord.reply}: %v", entity.SinglePointOfUse)
	}
}

// Repair B: many simultaneous redemptions of ONE decision against the real Store and the real HTTP
// handler. Whatever the scheduling, exactly one succeeds. Run under -race.
func TestConcurrentRedeemSucceedsExactlyOnce(t *testing.T) {
	const rounds, workers = 25, 48
	for _, viaHTTP := range []bool{false, true} {
		for _, action := range []string{dReply, "cognition.nudge.process"} {
			name := action
			if viaHTTP {
				name += "/http"
			}
			t.Run(name, func(t *testing.T) {
				f := discordSetup(t)
				f.grant(map[string]any{"skill": dReply, "passphrase": pass})
				f.grant(map[string]any{"skill": "cognition.nudge-process", "passphrase": pass})
				svc, key := entity.DoorService, doorKey
				if action != dReply {
					svc, key = entity.RedeemService, engineKey
				}
				for round := 0; round < rounds; round++ {
					bind := discordBinding("m_race", "11", "22", "33", "reply "+string(rune('a'+round)))
					id, _ := f.mint(coreKey, action, bind)["decision_id"].(string)
					if id == "" {
						t.Fatal("mint failed")
					}
					var (
						wg    sync.WaitGroup
						start = make(chan struct{})
						mu    sync.Mutex
						wins  int
					)
					for w := 0; w < workers; w++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							<-start
							var ok bool
							if viaHTTP {
								ok = f.redeem(key, id, action, bind)["allow"] == true
							} else {
								ok = f.st.RedeemBound(svc, id, action, bind, true).Allow
							}
							if ok {
								mu.Lock()
								wins++
								mu.Unlock()
							}
						}()
					}
					close(start)
					wg.Wait()
					if wins != 1 {
						t.Fatalf("round %d: %d of %d concurrent redemptions succeeded, want exactly 1", round, wins, workers)
					}
					// The decision is now consistently used.
					if f.st.RedeemBound(svc, id, action, bind, true).Allow {
						t.Fatalf("round %d: redeemable after the race", round)
					}
				}
				// The audit agrees: one allowed redeem per round for this action, the rest refused.
				fh, err := os.Open(filepath.Join(f.st.Dir(), "decisions.jsonl"))
				if err != nil {
					t.Fatal(err)
				}
				defer fh.Close()
				allowed, refused := 0, 0
				sc := bufio.NewScanner(fh)
				for sc.Scan() {
					var d struct {
						Mode, Action string
						Allow        bool
					}
					if json.Unmarshal(sc.Bytes(), &d) != nil || d.Mode != "redeem" || d.Action != action {
						continue
					}
					if d.Allow {
						allowed++
					} else {
						refused++
					}
				}
				if allowed != rounds || refused != rounds*workers {
					t.Fatalf("audit: %d allowed / %d refused redeems, want %d / %d", allowed, refused, rounds, rounds*workers)
				}
			})
		}
	}
}

// Repair B, mixed: simultaneous redeem and check attempts on one discord.reply decision. Only the
// single redeem may ever answer allow.
func TestConcurrentRedeemAndCheckOnDiscordReply(t *testing.T) {
	f := discordSetup(t)
	f.grant(map[string]any{"skill": dReply, "passphrase": pass})
	for round := 0; round < 25; round++ {
		bind := discordBinding("m_mix", "11", "22", "33", "reply "+string(rune('a'+round)))
		id := f.mint(coreKey, dReply, bind)["decision_id"].(string)
		var wg sync.WaitGroup
		start := make(chan struct{})
		var mu sync.Mutex
		wins := 0
		for w := 0; w < 40; w++ {
			wg.Add(1)
			go func(consume bool) {
				defer wg.Done()
				<-start
				if f.st.RedeemBound(entity.DoorService, id, dReply, bind, consume).Allow {
					mu.Lock()
					wins++
					mu.Unlock()
				}
			}(w%2 == 0)
		}
		close(start)
		wg.Wait()
		if wins != 1 {
			t.Fatalf("round %d: %d allow answers, want exactly 1", round, wins)
		}
	}
}

// Repair C: one key on two lines (two services sharing a key) is refused at startup.
func TestLoadKeysRefusesASharedKey(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "keys")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	shared := "0123456789abcdef0123456789abcdef-shared"
	for _, body := range []string{
		"core " + shared + "\ndiscord-door " + shared + "\n",
		"discord-door " + shared + "\ncore " + shared + "\n",
		"engine " + shared + "\ndiscord-door " + shared + "\n",
		"core " + shared + "\ncore " + shared + "\n",
	} {
		if m, err := loadKeys(write(body)); err == nil {
			t.Fatalf("a shared key was accepted: %v", m)
		} else if strings.Contains(err.Error(), shared) {
			t.Fatal("the error message prints the key")
		}
	}
	m, err := loadKeys(write("core " + coreKey + "\nengine " + engineKey + "\ndiscord-door " + doorKey + "\n# comment\n"))
	if err != nil || len(m) != 3 || m[doorKey] != entity.DoorService || m[coreKey] != "core" || m[engineKey] != "engine" {
		t.Fatalf("distinct keys: %v %v", m, err)
	}
}
