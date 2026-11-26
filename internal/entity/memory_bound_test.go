package entity

// Controlled memory integration (TASS #24 7182931062): memory.read, memory.write and github.publish
// are bound, single-use decisions that Core mints and Core redeems. Synthetic entity only.

import (
	"strings"
	"testing"
)

const (
	bindA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	bindB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestMemoryAndPublishBoundDecisions(t *testing.T) {
	s := open(t, t.TempDir())
	st, err := s.Create("Synthetic", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"memory.read", "memory.write", "github.publish"} {
		if d := s.Authorize("core", action, "", ""); d.Allow {
			t.Fatalf("%s allowed as a plain (unbound) authorization", action)
		}
		if d := s.MintBound("engine", action, bindA, "", ""); d.Allow {
			t.Fatalf("%s minted by engine", action)
		}
		if d := s.MintBound("core", action, "not-hex", "", ""); d.Allow {
			t.Fatalf("%s minted without a valid binding", action)
		}
		d := s.MintBound("core", action, bindA, "synthetic", st.Entity.EntityID)
		if !d.Allow || d.EntityID != st.Entity.EntityID {
			t.Fatalf("%s mint: %+v", action, d)
		}
		if r := s.RedeemBound("engine", d.ID, action, bindA, true); r.Allow {
			t.Fatalf("%s redeemed by engine", action)
		}
		if r := s.RedeemBound("core", d.ID, action, bindB, true); r.Allow || !strings.Contains(r.Reason, "binding") {
			t.Fatalf("%s redeemed with another binding: %+v", action, r)
		}
		other := "memory.read"
		if action == "memory.read" {
			other = "memory.write"
		}
		if r := s.RedeemBound("core", d.ID, other, bindA, true); r.Allow {
			t.Fatalf("%s decision redeemed as %s", action, other)
		}
		if r := s.RedeemBound("core", d.ID, action, bindA, true); !r.Allow {
			t.Fatalf("%s redeem: %+v", action, r)
		}
		if r := s.RedeemBound("core", d.ID, action, bindA, true); r.Allow || !strings.Contains(r.Reason, "already used") {
			t.Fatalf("%s replayed: %+v", action, r)
		}
	}
	// Core still cannot redeem the Engine's cognition actions.
	if r := s.RedeemBound("core", "dec_x", "cognition.nudge.process", bindA, true); r.Allow || !strings.Contains(r.Reason, "only engine") {
		t.Fatalf("core redeemed a cognition action: %+v", r)
	}
	// LOCKED: nothing mints; a decision minted before the lock cannot be redeemed after it.
	d := s.MintBound("core", "memory.write", bindA, "", "")
	s.Lock()
	if m := s.MintBound("core", "memory.write", bindA, "", ""); m.Allow {
		t.Fatal("minted while LOCKED")
	}
	if r := s.RedeemBound("core", d.ID, "memory.write", bindA, true); r.Allow {
		t.Fatal("redeemed while LOCKED")
	}
}

// An Identity restart forgets pending bound decisions (fails closed).
func TestMemoryDecisionForgottenOnRestart(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	if _, err := s.Create("Synthetic", "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	d := s.MintBound("core", "memory.read", bindA, "", "")
	s2 := open(t, dir)
	s2.Unlock("correct horse battery")
	if r := s2.RedeemBound("core", d.ID, "memory.read", bindA, true); r.Allow {
		t.Fatalf("decision survived restart: %+v", r)
	}
}
