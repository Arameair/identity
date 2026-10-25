package features

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/github"
)

var acct = &github.Account{Login: "fixture-owner", ID: 4242}

func TestDraftsAreLocalIdempotentAndDurable(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	d1, created, err := s.Save("thought-processor", acct, "TEST FIXTURE", "idn_x")
	if err != nil || !created || d1.Delivery != DraftDelivery {
		t.Fatalf("%+v %v %v", d1, created, err)
	}
	d2, created, _ := s.Save("thought-processor", acct, "TEST FIXTURE", "idn_x")
	if created || d2.ID != d1.ID || len(s.All()) != 1 {
		t.Fatal("duplicate draft created")
	}
	// survives restart
	s2 := Open(dir)
	if len(s2.All()) != 1 || s2.All()[0].ID != d1.ID {
		t.Fatal("draft lost on reopen")
	}
	if ok, _ := s2.Withdraw("thought-processor", acct.ID, "TEST FIXTURE"); !ok || len(Open(dir).All()) != 0 {
		t.Fatal("withdraw failed")
	}
}

func TestDraftNeedsVerifiedAccountAndKnownFeature(t *testing.T) {
	s := Open(t.TempDir())
	if _, _, err := s.Save("thought-processor", nil, "real", "idn_x"); err == nil {
		t.Fatal("draft without account")
	}
	if _, _, err := s.Save("teleporter", acct, "real", "idn_x"); err == nil {
		t.Fatal("unknown feature accepted")
	}
}

func TestTamperedDraftsLeftUntouched(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "drafts.json")
	forged := `[{"id":"draft_1","feature_id":"thought-engine","github_id":1,"delivery":"GRANTED"}]`
	os.WriteFile(p, []byte(forged), 0o600)
	s := Open(dir)
	if s.LoadError() == "" {
		t.Fatal("forged 'GRANTED' draft accepted")
	}
	if _, _, err := s.Save("thought-processor", acct, "real", "idn_x"); err == nil {
		t.Fatal("save allowed on damaged file")
	}
	if b, _ := os.ReadFile(p); string(b) != forged {
		t.Fatal("damaged file overwritten")
	}
}

func TestCardsNeverClaimGrant(t *testing.T) {
	s := Open(t.TempDir())
	now := time.Now()
	gv := github.View{Status: github.Verified, Mode: "TEST FIXTURE", Account: acct, Publisher: &github.Publisher{Repos: []github.RepoCheck{
		{Repo: "Arameair/Thought-Processor", Result: "VISIBLE", CheckedAt: now},
		{Repo: "Arameair/Thought-engine", Result: "NOT VISIBLE", CheckedAt: now},
	}}}
	s.Save("thought-processor", acct, "TEST FIXTURE", "idn_x")
	cards := s.Cards(gv)
	if cards[1].Grant != InstallDenied || cards[0].Grant != GrantNone {
		t.Fatalf("install rule grant: %+v", cards)
	}
	if cards[0].AvailKind != "available" || cards[0].RequestKind != "draft" || cards[1].AvailKind != "unavailable" || cards[1].RequestKind != "not_requested" {
		t.Fatalf("%+v", cards)
	}
	for _, c := range append(cards, s.Cards(github.View{Status: github.SignedOut})...) {
		if (c.Grant != GrantNone && c.Grant != InstallAllowed && c.Grant != InstallDenied) || strings.Contains(c.Request, "SUBMITTED") && !strings.Contains(c.Request, "NOT SUBMITTED") {
			t.Fatalf("card claims more than evidence: %+v", c)
		}
	}
	if out := s.Cards(github.View{Status: github.SignedOut}); out[0].AvailKind != "unknown" || out[0].Draft != nil {
		t.Fatal("signed-out card shows account-specific state")
	}
}
