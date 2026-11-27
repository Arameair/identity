package entity

import (
	"strings"
	"testing"
)

// mailbox.read: owner grant, pinned thread, redeemed once by the protected reader (never Core),
// checkable afterwards, and a revocation committed before redeem or check is refused.
func TestMailboxReadBoundDecision(t *testing.T) {
	const pass = "correct horse battery"
	s := open(t, t.TempDir())
	st, err := s.Create("Synthetic", pass)
	if err != nil {
		t.Fatal(err)
	}
	ent := st.Entity.EntityID
	bind := strings.Repeat("ab", 32)
	detail := PinnedMailboxThread + " tj_test"
	rd := MailboxReaderService

	if d := s.MintBound("core", MailboxRead, bind, detail, ent); d.Allow {
		t.Fatal("minted without the owner's mailbox.read grant")
	}
	if _, err := s.SetSkillGrant(pass, MailboxRead, true); err != nil {
		t.Fatal(err)
	}
	if g := s.State().Entity.Grants; !contains(g, MailboxRead) || contains(g, SkillDiscover) || contains(g, GitHubMailboxReply) {
		t.Fatalf("grant must be exactly mailbox.read: %v", g)
	}
	for _, bad := range []string{"Arameair/TASS#25 tj_test", "Arameair/TASS#240"} {
		if d := s.MintBound("core", MailboxRead, bind, bad, ent); d.Allow {
			t.Fatalf("minted for unpinned thread %q", bad)
		}
	}
	if d := s.MintBound(rd, MailboxRead, bind, detail, ent); d.Allow {
		t.Fatal("the reader minted a decision")
	}
	if !RedeemOnly(rd) || RedeemOnly("core") {
		t.Fatal("the reader must be redeem-only and Core must stay a minter")
	}
	d := s.MintBound("core", MailboxRead, bind, detail, ent)
	if !d.Allow {
		t.Fatalf("mint: %+v", d)
	}
	for _, svc := range []string{"core", "engine"} {
		if r := s.RedeemBound(svc, d.ID, MailboxRead, bind, true); r.Allow {
			t.Fatalf("%s redeemed a reader decision", svc)
		}
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, bind, false); r.Allow {
		t.Fatal("checked before redemption")
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, strings.Repeat("cd", 32), true); r.Allow {
		t.Fatal("redeemed with another binding (another thread or junction)")
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, bind, true); !r.Allow {
		t.Fatalf("redeem: %+v", r)
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, bind, true); r.Allow {
		t.Fatal("redeemed twice")
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, bind, false); !r.Allow {
		t.Fatalf("check after redemption: %+v", r)
	}

	// Revocation committed before the delivery check → refused; before redemption → refused.
	d2 := s.MintBound("core", MailboxRead, bind, detail, ent)
	if _, err := s.SetSkillGrant("", MailboxRead, false); err != nil {
		t.Fatal(err)
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, bind, false); r.Allow || !strings.Contains(r.Reason, "no grant") {
		t.Fatalf("check after revocation: %+v", r)
	}
	if r := s.RedeemBound(rd, d2.ID, MailboxRead, bind, true); r.Allow {
		t.Fatalf("redeemed after revocation: %+v", r)
	}
}
