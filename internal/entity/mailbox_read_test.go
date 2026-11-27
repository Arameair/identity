package entity

import (
	"strings"
	"testing"
)

// mailbox.read (TASS #24 rev 5): owner grant, pinned thread, Core-redeemed, single use, and a
// revocation that commits before redemption is refused.
func TestMailboxReadBoundDecision(t *testing.T) {
	const pass = "correct horse battery"
	s := open(t, t.TempDir())
	st, err := s.Create("Synthetic", pass)
	if err != nil {
		t.Fatal(err)
	}
	ent := st.Entity.EntityID
	bind := strings.Repeat("ab", 32)
	detail := PinnedMailboxThread + " jn_test"

	if d := s.MintBound("core", MailboxRead, bind, detail, ent); d.Allow {
		t.Fatal("minted without the owner's mailbox.read grant")
	}
	if _, err := s.SetSkillGrant(pass, MailboxRead, true); err != nil {
		t.Fatal(err)
	}
	if g := s.State().Entity.Grants; !contains(g, MailboxRead) || contains(g, SkillDiscover) || contains(g, GitHubMailboxReply) {
		t.Fatalf("grant must be exactly mailbox.read: %v", g)
	}
	if d := s.MintBound("core", MailboxRead, bind, "Arameair/TASS#25 jn_test", ent); d.Allow {
		t.Fatal("minted for an unpinned thread")
	}
	if d := s.MintBound("core", MailboxRead, bind, "Arameair/TASS#240", ent); d.Allow {
		t.Fatal("minted for a thread that merely starts with the pinned one")
	}
	d := s.MintBound("core", MailboxRead, bind, detail, ent)
	if !d.Allow {
		t.Fatalf("mint: %+v", d)
	}
	if r := s.RedeemBound("engine", d.ID, MailboxRead, bind, true); r.Allow {
		t.Fatal("engine redeemed a Core-performed action")
	}
	if RedeemOnly("core") {
		t.Fatal("core must keep its normal routes")
	}
	if r := s.RedeemBound("core", d.ID, MailboxRead, bind, true); !r.Allow {
		t.Fatalf("redeem: %+v", r)
	}
	if r := s.RedeemBound("core", d.ID, MailboxRead, bind, true); r.Allow {
		t.Fatal("redeemed twice")
	}
	if r := s.RedeemBound("core", d.ID, MailboxRead, bind, false); r.Allow {
		t.Fatal("a single-point-of-use decision answered a check")
	}

	// Revocation committed before redemption → refused at the boundary.
	d2 := s.MintBound("core", MailboxRead, bind, detail, ent)
	if !d2.Allow {
		t.Fatalf("mint 2: %+v", d2)
	}
	if _, err := s.SetSkillGrant("", MailboxRead, false); err != nil {
		t.Fatal(err)
	}
	if r := s.RedeemBound("core", d2.ID, MailboxRead, bind, true); r.Allow || !strings.Contains(r.Reason, "no grant") {
		t.Fatalf("redeemed after revocation: %+v", r)
	}
}
