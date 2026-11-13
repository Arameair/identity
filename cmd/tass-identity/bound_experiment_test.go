package main

import (
	"testing"
	"time"

	"github.com/Arameair/identity/internal/entity"
)

// Stage 3B (Thought-Processor #120, owner decision D7): cognition.experiment.run is its own bound action
// with its own owner grant, cognition.experiment-run. It is not a default, not implied by any other grant,
// and never shares authority with cognition.nudge.process.

const expRun = "cognition.experiment.run"

func TestExperimentRunIsASeparateNonDefaultBoundAction(t *testing.T) {
	st, call, grant := boundSetup(t)
	mint := func(key, action, binding string) map[string]any {
		return call("/v1/authorize", key, map[string]string{"action": action, "binding": binding, "detail": "exp_x A"})
	}
	redeem := func(key, id, action, binding string, check bool) map[string]any {
		return call("/v1/decisions/redeem", key, map[string]any{"decision_id": id, "action": action, "binding": binding, "check": check})
	}
	held := func(g string) bool {
		for _, x := range st.State().Entity.Grants {
			if x == g {
				return true
			}
		}
		return false
	}
	// Not a default: a new, unlocked entity does not hold it, and nothing can be minted.
	if held(expRun) {
		t.Fatal("a new entity holds cognition.experiment.run by default")
	}
	if a := mint(coreKey, expRun, bindA); a["allow"] != false {
		t.Fatalf("minted without the grant: %v", a)
	}
	// Not implied by the nudge grants (D7: do not reuse cognition.nudge.process).
	grant(map[string]any{"skill": "cognition.nudge-process", "passphrase": "correct horse battery"})
	grant(map[string]any{"skill": "cognition.nudge-draft", "passphrase": "correct horse battery"})
	if a := mint(coreKey, expRun, bindA); a["allow"] != false || held(expRun) {
		t.Fatalf("the nudge grants allowed an experiment: %v", a)
	}
	// Enabling it needs the passphrase.
	if c := grant(map[string]any{"skill": "cognition.experiment-run", "passphrase": "wrong passphrase!!"}); c == 200 || held(expRun) {
		t.Fatalf("granted with a wrong passphrase (HTTP %d)", c)
	}
	if c := grant(map[string]any{"skill": "cognition.experiment-run", "passphrase": "correct horse battery"}); c != 200 || !held(expRun) {
		t.Fatalf("grant: HTTP %d, held %v", c, held(expRun))
	}
	if held("skill.run:cognition.experiment-run") || held("skill.discover") {
		t.Fatalf("the experiment grant added a skill grant: %v", st.State().Entity.Grants)
	}
	// Bound only: no unbound decision, no bad binding, only Core mints.
	if a := call("/v1/authorize", coreKey, map[string]string{"action": expRun}); a["allow"] != false {
		t.Fatalf("an unbound experiment decision was allowed: %v", a)
	}
	if a := mint(coreKey, expRun, "not-hex"); a["allow"] != false {
		t.Fatal("a bad binding was accepted")
	}
	if a := mint(engineKey, expRun, bindA); a["allow"] != false {
		t.Fatal("the Engine minted its own decision")
	}
	d := mint(coreKey, expRun, bindA)
	id, _ := d["decision_id"].(string)
	if d["allow"] != true || id == "" || d["expires_at"] == nil {
		t.Fatalf("mint: %v", d)
	}
	// Only the Engine redeems; the exact binding and action; once.
	if r := redeem(coreKey, id, expRun, bindA, false); r["allow"] != false {
		t.Fatal("Core redeemed")
	}
	if r := redeem(engineKey, id, expRun, bindB, false); r["allow"] != false {
		t.Fatal("another binding (another arm, snapshot or variant) redeemed")
	}
	if r := redeem(engineKey, id, "cognition.nudge.process", bindA, false); r["allow"] != false {
		t.Fatal("an experiment decision was redeemed as a nudge")
	}
	if r := redeem(engineKey, id, expRun, bindA, false); r["allow"] != true {
		t.Fatalf("redeem: %v", r)
	}
	if r := redeem(engineKey, id, expRun, bindA, false); r["allow"] != false {
		t.Fatal("redeemed twice")
	}
	if r := redeem(engineKey, id, expRun, bindA, true); r["allow"] != true {
		t.Fatalf("check of a redeemed decision: %v", r)
	}
	// A nudge decision cannot run an experiment.
	n := mint(coreKey, "cognition.nudge.process", bindA)
	if r := redeem(engineKey, n["decision_id"].(string), expRun, bindA, false); r["allow"] != false {
		t.Fatal("a nudge decision was redeemed as an experiment")
	}
	// Revocable: after revoking, the redeemed decision no longer checks, and nothing new is minted;
	// the nudge grant is untouched.
	grant(map[string]any{"skill": "cognition.experiment-run", "revoke": true})
	if held(expRun) || !held("cognition.nudge.process") {
		t.Fatalf("revoke: %v", st.State().Entity.Grants)
	}
	if r := redeem(engineKey, id, expRun, bindA, true); r["allow"] != false {
		t.Fatal("check allowed after the grant was revoked")
	}
	if a := mint(coreKey, expRun, bindA); a["allow"] != false {
		t.Fatal("minted after the grant was revoked")
	}
	// Locked: nothing.
	grant(map[string]any{"skill": "cognition.experiment-run", "passphrase": "correct horse battery"})
	d2 := mint(coreKey, expRun, bindA)
	st.Lock()
	if r := redeem(engineKey, d2["decision_id"].(string), expRun, bindA, false); r["allow"] != false {
		t.Fatal("redeemed while LOCKED")
	}
	if a := mint(coreKey, expRun, bindA); a["allow"] != false {
		t.Fatal("minted while LOCKED")
	}
}

func TestExperimentRunDecisionExpiresAndDoesNotSurviveARestart(t *testing.T) {
	st, call, grant := boundSetup(t)
	grant(map[string]any{"skill": "cognition.experiment-run", "passphrase": "correct horse battery"})
	if entity.BoundTTL != 120*time.Second {
		t.Fatalf("TTL is %v, want 120 s", entity.BoundTTL)
	}
	now := time.Now()
	st.Now = func() time.Time { return now }
	d := call("/v1/authorize", coreKey, map[string]string{"action": expRun, "binding": bindA})
	now = now.Add(entity.BoundTTL + time.Second)
	if r := call("/v1/decisions/redeem", engineKey, map[string]any{"decision_id": d["decision_id"], "action": expRun, "binding": bindA}); r["allow"] != false {
		t.Fatal("an expired decision was redeemed")
	}
	d2 := call("/v1/authorize", coreKey, map[string]string{"action": expRun, "binding": bindA})
	st2, _ := entity.Open(st.Dir(), time.Hour)
	if r := st2.RedeemBound("engine", d2["decision_id"].(string), expRun, bindA, true); r.Allow {
		t.Fatal("a decision survived an Identity restart")
	}
}
