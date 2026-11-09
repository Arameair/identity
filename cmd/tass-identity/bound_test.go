package main

import (
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/entity"
)

// Stage 2 (Thought-Processor #100, design rev 4): bound, single-use decisions for nudge process/stop.

const coreKey = "0123456789abcdef0123456789abcdef-core!!"

func boundSetup(t *testing.T) (*entity.Store, func(path string, svcKey string, body any) map[string]any, func(body any) int) {
	t.Helper()
	st, _ := entity.Open(t.TempDir(), time.Hour)
	h := routes(st, map[string]string{engineKey: "engine", coreKey: "core"}, "page-secret")
	owner := map[string]string{"X-Identity-Page": "page-secret"}
	req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", owner, map[string]string{"name": "TASS", "passphrase": "correct horse battery"})
	call := func(path, key string, body any) map[string]any {
		_, out := req(t, h, "POST", "identity:8796", path, map[string]string{"Authorization": "Bearer " + key}, body)
		return out
	}
	grant := func(body any) int {
		c, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/skill", owner, body)
		return c
	}
	return st, call, grant
}

var bindA = strings.Repeat("a", 64)
var bindB = strings.Repeat("b", 64)

func TestBoundProcessDecisionLifecycle(t *testing.T) {
	_, call, grant := boundSetup(t)
	mint := func(key, action, binding string) map[string]any {
		return call("/v1/authorize", key, map[string]string{"action": action, "binding": binding, "detail": "nudge_x thought_1"})
	}
	redeem := func(key, id, action, binding string, check bool) map[string]any {
		return call("/v1/decisions/redeem", key, map[string]any{"decision_id": id, "action": action, "binding": binding, "check": check})
	}
	// No unbound path for a bound action, even with the grant.
	grant(map[string]any{"skill": "cognition.nudge-process", "passphrase": "correct horse battery"})
	if a := call("/v1/authorize", coreKey, map[string]string{"action": "cognition.nudge.process"}); a["allow"] != false {
		t.Fatal("bound action allowed without a binding")
	}
	if a := mint(coreKey, "cognition.nudge.process", "not-hex"); a["allow"] != false {
		t.Fatal("bad binding accepted")
	}
	if a := mint(engineKey, "cognition.nudge.process", bindA); a["allow"] != false {
		t.Fatal("the engine minted a decision")
	}
	d := mint(coreKey, "cognition.nudge.process", bindA)
	if d["allow"] != true || d["expires_at"] == nil {
		t.Fatalf("mint: %v", d)
	}
	id := d["decision_id"].(string)
	if r := redeem(coreKey, id, "cognition.nudge.process", bindA, false); r["allow"] != false {
		t.Fatal("core redeemed")
	}
	if r := redeem(engineKey, id, "cognition.nudge.process", bindB, false); r["allow"] != false {
		t.Fatal("redeemed with another binding")
	}
	if r := redeem(engineKey, id, "cognition.nudge.stop", bindA, false); r["allow"] != false {
		t.Fatal("redeemed as another action")
	}
	if r := redeem(engineKey, id, "cognition.nudge.process", bindA, true); r["allow"] != false {
		t.Fatal("checked before redeem")
	}
	if r := redeem(engineKey, id, "cognition.nudge.process", bindA, false); r["allow"] != true {
		t.Fatalf("redeem: %v", r)
	}
	if r := redeem(engineKey, id, "cognition.nudge.process", bindA, false); r["allow"] != false {
		t.Fatal("redeemed twice")
	}
	if r := redeem(engineKey, id, "cognition.nudge.process", bindA, true); r["allow"] != true {
		t.Fatal("check after redeem")
	}
	// Revocation is seen by the next check (the Engine's P1/P2).
	grant(map[string]any{"skill": "cognition.nudge-process", "revoke": true})
	if r := redeem(engineKey, id, "cognition.nudge.process", bindA, true); r["allow"] != false {
		t.Fatal("check after revoke allowed")
	}
	if a := mint(coreKey, "cognition.nudge.process", bindA); a["allow"] != false {
		t.Fatal("minted without the grant")
	}
	// Neither the draft grant nor thought.process implies it.
	grant(map[string]any{"skill": "cognition.nudge-draft", "passphrase": "correct horse battery"})
	if a := mint(coreKey, "cognition.nudge.process", bindA); a["allow"] != false {
		t.Fatal("draft grant implied process")
	}
}

func TestBoundDecisionExpiresAndIsMemoryOnly(t *testing.T) {
	st, call, grant := boundSetup(t)
	grant(map[string]any{"skill": "cognition.nudge-process", "passphrase": "correct horse battery"})
	now := time.Now()
	st.Now = func() time.Time { return now }
	d := call("/v1/authorize", coreKey, map[string]string{"action": "cognition.nudge.process", "binding": bindA})
	now = now.Add(entity.BoundTTL + time.Second)
	if r := call("/v1/decisions/redeem", engineKey, map[string]any{"decision_id": d["decision_id"], "action": "cognition.nudge.process", "binding": bindA}); r["allow"] != false {
		t.Fatal("expired decision redeemed")
	}
	// A new Store on the same folder (= Identity restart) knows no bound decisions.
	d2 := call("/v1/authorize", coreKey, map[string]string{"action": "cognition.nudge.process", "binding": bindA})
	st2, _ := entity.Open(st.Dir(), time.Hour)
	if r := st2.RedeemBound("engine", d2["decision_id"].(string), "cognition.nudge.process", bindA, true); r.Allow {
		t.Fatal("a bound decision survived a restart")
	}
}

func TestBoundStopNeedsNoGrantButUnlock(t *testing.T) {
	st, call, _ := boundSetup(t)
	// No grant at all: stop is still possible (D15)...
	d := call("/v1/authorize", coreKey, map[string]string{"action": "cognition.nudge.stop", "binding": bindB})
	if d["allow"] != true {
		t.Fatalf("stop mint: %v", d)
	}
	if r := call("/v1/decisions/redeem", engineKey, map[string]any{"decision_id": d["decision_id"], "action": "cognition.nudge.stop", "binding": bindB}); r["allow"] != true {
		t.Fatalf("stop redeem: %v", r)
	}
	// ...but not while LOCKED.
	st.Lock()
	if d := call("/v1/authorize", coreKey, map[string]string{"action": "cognition.nudge.stop", "binding": bindB}); d["allow"] != false {
		t.Fatal("stop minted while LOCKED")
	}
}
