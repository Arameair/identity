package entity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExistingEntityOwnerEnablesOnlyLearningGrants(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	before, err := s.Create("TASS", "correct horse battery")
	if err != nil { t.Fatal(err) }
	id := before.Entity.EntityID
	// Reproduce an existing installation made before the two learning grants existed.
	p := filepath.Join(dir, "entity.json")
	raw, err := os.ReadFile(p)
	if err != nil { t.Fatal(err) }
	var e Entity
	if err := json.Unmarshal(raw, &e); err != nil { t.Fatal(err) }
	var old []string
	for _, g := range e.Grants {
		if g != "github.sync" && g != "lesson.learn" { old = append(old, g) }
	}
	e.Grants = old
	raw, _ = json.MarshalIndent(e, "", "  ")
	if err := os.WriteFile(p, raw, 0o600); err != nil { t.Fatal(err) }
	if d := s.Authorize("core", "lesson.learn", "", ""); d.Allow { t.Fatal("old entity unexpectedly had grant") }
	if _, err := s.EnableLearningGrants("incorrect passphrase"); err != ErrBadPass { t.Fatalf("bad passphrase: %v", err) }
	if d := s.Authorize("core", "github.sync", "", ""); d.Allow { t.Fatal("bad passphrase granted access") }
	st, err := s.EnableLearningGrants("correct horse battery")
	if err != nil { t.Fatal(err) }
	if st.Entity.EntityID != id || st.State != Unlocked { t.Fatalf("entity changed: %+v", st) }
	if len(st.Entity.Grants) != len(old)+2 { t.Fatalf("wrong grants: %v", st.Entity.Grants) }
	for _, action := range []string{"github.sync", "lesson.learn"} {
		if d := s.Authorize("core", action, "", id); !d.Allow { t.Fatalf("grant missing: %v", d) }
	}
	if d := s.Authorize("core", "github.write", "", ""); d.Allow { t.Fatal("unrequested privilege added") }
	if _, err := s.EnableLearningGrants("correct horse battery"); err != nil { t.Fatalf("idempotent: %v", err) }
	if st := s.State(); len(st.Entity.Grants) != len(old)+2 { t.Fatalf("duplicate grant: %v", st.Entity.Grants) }
	s.Lock()
	if _, err := s.EnableLearningGrants("correct horse battery"); err != ErrNotUsable { t.Fatalf("locked must deny: %v", err) }
	if d := s.Authorize("core", "lesson.learn", "", ""); d.Allow { t.Fatal("locked but allowed") }
	if b, _ := os.ReadFile(filepath.Join(dir, "decisions.jsonl")); !strings.Contains(string(b), "grants.enable_learning") || strings.Contains(string(b), "correct horse battery") {
		t.Fatal("audit absent or passphrase leaked")
	}
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "correct horse battery") { t.Fatal("passphrase stored") }
}
