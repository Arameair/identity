package entity

import (
	"os"
	"strings"
	"testing"
)

// mailbox.read: owner grant AND owner-mapped per-entity thread; Identity derives the binding;
// redeemed once by the protected reader (never Core), checkable afterwards; revocation refuses.
func TestMailboxReadBoundDecision(t *testing.T) {
	const pass = "correct horse battery"
	s := open(t, t.TempDir())
	st, err := s.Create("Synthetic", pass)
	if err != nil {
		t.Fatal(err)
	}
	ent := st.Entity.EntityID
	rd := MailboxReaderService
	const thread = "Arameair/TASS#24"
	if _, err := s.SetSkillGrant(pass, MailboxRead, true); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.MintMailbox("core", ent, "tj_a"); d.Allow {
		t.Fatal("minted without an owner-mapped thread")
	}
	os.WriteFile(s.path(mailboxScopeFile), []byte(`{"ent_other":"Arameair/Other#1","`+ent+`":"`+thread+`"}`), 0o600)
	if g := s.State().Entity.Grants; !contains(g, MailboxRead) || contains(g, SkillDiscover) || contains(g, GitHubMailboxReply) {
		t.Fatalf("grant must be exactly mailbox.read: %v", g)
	}
	d, got := s.MintMailbox("core", ent, "tj_a")
	if !d.Allow || got != thread || d.Binding != MailboxBinding(ent, thread, "tj_a") {
		t.Fatalf("mint: %+v thread=%q", d, got)
	}
	// No route may mint another thread or a caller-chosen binding.
	for _, c := range []struct{ bind, detail string }{
		{MailboxBinding(ent, "Arameair/Other#1", "tj_a"), "Arameair/Other#1 tj_a"}, // another entity's thread
		{MailboxBinding(ent, "Arameair/TASS#25", "tj_a"), "Arameair/TASS#25 tj_a"}, // unmapped thread
		{strings.Repeat("ab", 32), thread + " tj_a"},                               // caller binding
		{MailboxBinding("ent_other", thread, "tj_a"), thread + " tj_a"},            // other entity binding
		{MailboxBinding(ent, thread, "../x"), thread + " ../x"},                    // bad junction
	} {
		if x := s.MintBound("core", MailboxRead, c.bind, c.detail, ent); x.Allow {
			t.Fatalf("minted %q", c.detail)
		}
	}
	if x, _ := s.MintMailbox(rd, ent, "tj_a"); x.Allow {
		t.Fatal("the reader minted a decision")
	}
	if !RedeemOnly(rd) || RedeemOnly("core") {
		t.Fatal("the reader must be redeem-only and Core must stay a minter")
	}
	bind := d.Binding
	for _, svc := range []string{"core", "engine"} {
		if r := s.RedeemBound(svc, d.ID, MailboxRead, bind, true); r.Allow {
			t.Fatalf("%s redeemed a reader decision", svc)
		}
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, MailboxBinding(ent, "Arameair/Other#1", "tj_a"), true); r.Allow {
		t.Fatal("reader redeemed for another thread")
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, bind, false); r.Allow {
		t.Fatal("checked before redemption")
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
	// Owner remaps the thread: decisions already minted keep their binding, new mints use the new thread.
	d2, _ := s.MintMailbox("core", ent, "tj_b")
	if _, err := s.SetSkillGrant("", MailboxRead, false); err != nil {
		t.Fatal(err)
	}
	if r := s.RedeemBound(rd, d.ID, MailboxRead, bind, false); r.Allow || !strings.Contains(r.Reason, "no grant") {
		t.Fatalf("check after revocation: %+v", r)
	}
	if r := s.RedeemBound(rd, d2.ID, MailboxRead, d2.Binding, true); r.Allow {
		t.Fatalf("redeemed after revocation: %+v", r)
	}
	if x, _ := s.MintMailbox("core", ent, "tj_c"); x.Allow {
		t.Fatal("minted after revocation")
	}
}
