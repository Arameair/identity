package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/entity"
)

// Protected GitHub mailbox (Arameair/TASS #24, Model B). Fixture values only.

const pubKey = "0123456789abcdef0123456789abcdef-pub!!!"

// ghBinding is the contract formula, written out independently of production code.
func ghBinding(action, repo, issue, source, payload string) string {
	p := sha256.Sum256([]byte(payload))
	s := sha256.Sum256([]byte(strings.Join([]string{"tass.github.mailbox/1", "github-publisher", action, repo, issue,
		source, hex.EncodeToString(p[:])}, "\n")))
	return hex.EncodeToString(s[:])
}

func hexOf(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

type ghFix struct {
	st    *entity.Store
	raw   func(method, path, key string, body any) (int, map[string]any)
	owner func(method, path string, body any) (int, map[string]any)
}

func ghSetup(t *testing.T) ghFix {
	t.Helper()
	st, _ := entity.Open(t.TempDir(), time.Hour)
	h := routes(st, map[string]string{engineKey: "engine", coreKey: "core", pubKey: entity.PublisherService}, "page-secret")
	ownerHdr := map[string]string{"X-Identity-Page": "page-secret"}
	req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", ownerHdr, map[string]string{"name": "TASS", "passphrase": pass})
	return ghFix{st: st,
		raw: func(method, path, key string, body any) (int, map[string]any) {
			return req(t, h, method, "identity:8796", path, map[string]string{"Authorization": "Bearer " + key}, body)
		},
		owner: func(method, path string, body any) (int, map[string]any) {
			return req(t, h, method, "127.0.0.1:8796", path, ownerHdr, body)
		}}
}

func TestGitHubMailboxBindingMatchesContract(t *testing.T) {
	want := ghBinding(entity.GitHubMailboxShare, "Arameair/TASS", "24", "share_20261124T120000_0123abcd", "hello team")
	if got := entity.GitHubMailboxBinding(entity.GitHubMailboxShare, "Arameair/TASS", 24, "share_20261124T120000_0123abcd", "hello team"); got != want {
		t.Fatalf("binding %s != %s", got, want)
	}
	if entity.GitHubMailboxBinding(entity.GitHubMailboxShare, "Arameair/TASS", 25, "share_20261124T120000_0123abcd", "hello team") == want ||
		entity.GitHubMailboxBinding(entity.GitHubMailboxReply, "Arameair/TASS", 24, "share_20261124T120000_0123abcd", "hello team") == want ||
		entity.GitHubMailboxBinding(entity.GitHubMailboxShare, "Arameair/TASS", 24, "share_20261124T120000_0123abcd", "hello team ") == want {
		t.Fatal("issue, action and every payload byte must change the binding")
	}
}

func TestGitHubReplyMintedByCoreRedeemedOnlyByPublisherOnce(t *testing.T) {
	f := ghSetup(t)
	b := ghBinding(entity.GitHubMailboxReply, "Arameair/TASS", "24", "7131310377", "TASS: hi")
	_, m := f.raw("POST", "/v1/authorize", coreKey, map[string]string{"action": entity.GitHubMailboxReply, "binding": b})
	if m["allow"] != false || !strings.Contains(m["reason"].(string), "no grant") {
		t.Fatalf("reply without the owner grant must be refused: %v", m)
	}
	if c, _ := f.owner("POST", "/v1/grants/skill", map[string]any{"skill": entity.GitHubMailboxReply, "passphrase": pass}); c != 200 {
		t.Fatalf("enable grant: %d", c)
	}
	_, m = f.raw("POST", "/v1/authorize", coreKey, map[string]string{"action": entity.GitHubMailboxReply, "binding": b})
	id, _ := m["decision_id"].(string)
	if m["allow"] != true {
		t.Fatalf("mint: %v", m)
	}
	for _, k := range []string{coreKey, engineKey} {
		if _, r := f.raw("POST", "/v1/decisions/redeem", k, map[string]any{"decision_id": id, "action": entity.GitHubMailboxReply, "binding": b}); r["allow"] != false {
			t.Fatalf("only the publisher may redeem: %v", r)
		}
	}
	if c, r := f.raw("POST", "/v1/authorize", pubKey, map[string]string{"action": "conversation"}); c != 403 {
		t.Fatalf("publisher key is redeem-only: %d %v", c, r)
	}
	if _, r := f.raw("POST", "/v1/decisions/redeem", pubKey, map[string]any{"decision_id": id, "action": entity.GitHubMailboxReply, "binding": b + ""}); r["allow"] != true {
		t.Fatalf("publisher redeem: %v", r)
	}
	if _, r := f.raw("POST", "/v1/decisions/redeem", pubKey, map[string]any{"decision_id": id, "action": entity.GitHubMailboxReply, "binding": b}); r["allow"] != false {
		t.Fatalf("second redeem must fail: %v", r)
	}
	if _, r := f.raw("POST", "/v1/decisions/redeem", pubKey, map[string]any{"decision_id": id, "action": entity.GitHubMailboxReply, "binding": b, "check": true}); r["allow"] != false {
		t.Fatalf("single point of use: check must be refused: %v", r)
	}
}

func TestGitHubShareNeedsOwnerApprovalOfExactBytes(t *testing.T) {
	f := ghSetup(t)
	const sid = "share_20261124T120000_0123abcd"
	b := ghBinding(entity.GitHubMailboxShare, "Arameair/TASS", "24", sid, "hello team")
	// Core can never mint a share, grant or not.
	if _, m := f.raw("POST", "/v1/authorize", coreKey, map[string]string{"action": entity.GitHubMailboxShare, "binding": b}); m["allow"] != false {
		t.Fatalf("core minted a share: %v", m)
	}
	ent := f.st.State().Entity.EntityID
	c, r := f.raw("POST", "/v1/github/mailbox/share-requests", coreKey, map[string]any{"share_id": sid, "repo": "Arameair/TASS",
		"issue": 24, "text": "hello team", "entity_id": ent})
	if c != 200 || r["state"] != "PENDING" || r["text"] != nil {
		t.Fatalf("request: %d %v", c, r)
	}
	if c, _ := f.raw("POST", "/v1/github/mailbox/share-requests", pubKey, map[string]any{"share_id": sid, "repo": "Arameair/TASS", "issue": 24, "text": "x", "entity_id": ent}); c == 200 {
		t.Fatal("only core may request a share")
	}
	// The owner page sees the exact text; approval must name its exact hash and the passphrase.
	_, l := f.owner("GET", "/v1/github/mailbox/share-requests", nil)
	reqs := l["requests"].([]any)
	if len(reqs) != 1 || reqs[0].(map[string]any)["text"] != "hello team" {
		t.Fatalf("owner list: %v", l)
	}
	if c, _ := f.owner("POST", "/v1/github/mailbox/share-approve", map[string]any{"share_id": sid, "text_sha256": hexOf("hello team!"), "passphrase": pass, "approve": true}); c != 403 {
		t.Fatal("approval of different bytes must be refused")
	}
	if c, _ := f.owner("POST", "/v1/github/mailbox/share-approve", map[string]any{"share_id": sid, "text_sha256": hexOf("hello team"), "passphrase": "wrong", "approve": true}); c != 403 {
		t.Fatal("approval without the passphrase must be refused")
	}
	if c, _ := f.raw("POST", "/v1/github/mailbox/share-approve", coreKey, map[string]any{"share_id": sid, "text_sha256": hexOf("hello team"), "passphrase": pass, "approve": true}); c != 403 {
		t.Fatal("a service key must not reach the owner approval route")
	}
	if _, p := f.raw("GET", "/v1/github/mailbox/share-requests/"+sid+"?entity_id="+ent, coreKey, nil); p["state"] != "PENDING" || p["decision_id"] != nil {
		t.Fatalf("not approved yet: %v", p)
	}
	c, a := f.owner("POST", "/v1/github/mailbox/share-approve", map[string]any{"share_id": sid, "text_sha256": hexOf("hello team"), "passphrase": pass, "approve": true})
	if c != 200 || a["state"] != "APPROVED" || a["binding"] != b {
		t.Fatalf("approve: %d %v", c, a)
	}
	_, p := f.raw("GET", "/v1/github/mailbox/share-requests/"+sid+"?entity_id="+ent, coreKey, nil)
	if p["decision_id"] != a["decision_id"] {
		t.Fatalf("core reads the decision ID: %v", p)
	}
	if _, p2 := f.raw("GET", "/v1/github/mailbox/share-requests/"+sid+"?entity_id=ent_other", coreKey, nil); p2["state"] != nil {
		t.Fatal("another entity must not read the request")
	}
	id := a["decision_id"].(string)
	if _, r := f.raw("POST", "/v1/decisions/redeem", pubKey, map[string]any{"decision_id": id, "action": entity.GitHubMailboxShare,
		"binding": ghBinding(entity.GitHubMailboxShare, "Arameair/TASS", "24", sid, "hello team, edited")}); r["allow"] != false {
		t.Fatal("changed bytes must not redeem")
	}
	if _, r := f.raw("POST", "/v1/decisions/redeem", pubKey, map[string]any{"decision_id": id, "action": entity.GitHubMailboxShare, "binding": b}); r["allow"] != true {
		t.Fatalf("publisher redeem of the approved share: %v", r)
	}
	if c, _ := f.owner("POST", "/v1/github/mailbox/share-approve", map[string]any{"share_id": sid, "text_sha256": hexOf("hello team"), "passphrase": pass, "approve": true}); c != 403 {
		t.Fatal("a share is approved once")
	}
}

func TestGitHubShareRedeemRechecksLockAtPointOfUse(t *testing.T) {
	f := ghSetup(t)
	const sid = "share_20261124T120001_0123abcd"
	ent := f.st.State().Entity.EntityID
	f.raw("POST", "/v1/github/mailbox/share-requests", coreKey, map[string]any{"share_id": sid, "repo": "Arameair/TASS", "issue": 24, "text": "lock me", "entity_id": ent})
	_, a := f.owner("POST", "/v1/github/mailbox/share-approve", map[string]any{"share_id": sid, "text_sha256": hexOf("lock me"), "passphrase": pass, "approve": true})
	f.st.Lock()
	if _, r := f.raw("POST", "/v1/decisions/redeem", pubKey, map[string]any{"decision_id": a["decision_id"], "action": entity.GitHubMailboxShare,
		"binding": a["binding"]}); r["allow"] != false {
		t.Fatalf("LOCKED Identity must refuse redemption: %v", r)
	}
}
