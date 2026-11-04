package entity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/authority"
)

func init() { authority.SetIterationsForTests(1000) }

func open(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFreshInstallHasNoEntityAndAllowsNothing(t *testing.T) {
	s := open(t, t.TempDir())
	if st := s.State(); st.State != NoEntity {
		t.Fatalf("state %v", st)
	}
	if d := s.Authorize("engine", "model.request", "", ""); d.Allow {
		t.Fatal("nothing may be allowed without an entity")
	}
	if _, err := s.Unlock("whatever passphrase"); err != ErrNoEntity {
		t.Fatalf("unlock without entity: %v", err)
	}
}

func TestCreateUnlockLockAuthorize(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	if _, err := s.Create("Nova", "short"); err == nil {
		t.Fatal("weak passphrase accepted")
	}
	st, err := s.Create("Nova", "correct horse battery")
	if err != nil || st.State != Unlocked || !strings.HasPrefix(st.Entity.EntityID, "ent_") {
		t.Fatalf("create: %v %+v", err, st)
	}
	if _, err := s.Create("Other", "correct horse battery"); err != ErrExists {
		t.Fatalf("second create must be refused: %v", err)
	}
	if d := s.Authorize("engine", "model.request", "", st.Entity.EntityID); !d.Allow {
		t.Fatalf("granted action refused: %+v", d)
	}
	if d := s.Authorize("engine", "github.write", "", ""); d.Allow {
		t.Fatal("ungranted action allowed")
	}
	if d := s.Authorize("engine", "model.request", "", "ent_someoneelse"); d.Allow {
		t.Fatal("another entity's ID allowed")
	}
	s.Lock()
	if d := s.Authorize("engine", "model.request", "", ""); d.Allow || !strings.Contains(d.Reason, "LOCKED") {
		t.Fatalf("locked must refuse: %+v", d)
	}
	if _, err := s.Unlock("wrong wrong wrong"); err != ErrBadPass {
		t.Fatalf("wrong passphrase: %v", err)
	}
	if st, err := s.Unlock("correct horse battery"); err != nil || st.State != Unlocked {
		t.Fatalf("unlock: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "decisions.jsonl"))
	if strings.Count(string(b), "\n") != 4 {
		t.Fatalf("every decision recorded: %s", b)
	}
	for _, f := range []string{"entity.json", "credential", "decisions.jsonl"} {
		if fi, _ := os.Stat(filepath.Join(dir, f)); fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode %v", f, fi.Mode().Perm())
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "entity.json")); strings.Contains(string(b), "correct horse") {
		t.Fatal("passphrase stored")
	}
}

func TestRestartIsLockedAndUnlockExpires(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	s.Create("Nova", "correct horse battery")
	if st := open(t, dir).State(); st.State != Locked {
		t.Fatalf("after restart: %s", st.State)
	}
	now := time.Now()
	s.Now = func() time.Time { return now.Add(2 * time.Hour) }
	if st := s.State(); st.State != Locked {
		t.Fatal("unlock must expire")
	}
}

func TestMissingOrForeignCredentialStaysLockedAndIsNeverReminted(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	st, _ := s.Create("Nova", "correct horse battery")
	id := st.Entity.EntityID
	os.Rename(filepath.Join(dir, "credential"), filepath.Join(dir, "credential.gone"))
	s2 := open(t, dir)
	if st := s2.State(); st.State != Locked || !strings.Contains(st.Reason, "credential is missing") {
		t.Fatalf("missing credential: %+v", st)
	}
	if _, err := s2.Unlock("correct horse battery"); err == nil {
		t.Fatal("the passphrase alone must not unlock without the credential")
	}
	if _, err := s2.Create("Nova", "correct horse battery"); err != ErrExists {
		t.Fatal("must never re-mint an entity over a broken one")
	}
	os.WriteFile(filepath.Join(dir, "credential"), []byte("00ff"), 0o600)
	if st := s2.State(); st.State != Locked || !strings.Contains(st.Reason, "doesn't belong") {
		t.Fatalf("foreign credential: %+v", st)
	}
	os.Rename(filepath.Join(dir, "credential.gone"), filepath.Join(dir, "credential"))
	if st, err := s2.Unlock("correct horse battery"); err != nil || st.Entity.EntityID != id {
		t.Fatalf("restored credential: %v", err)
	}
}

func TestDamagedEntityFileIsLocked(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	s.Create("Nova", "correct horse battery")
	os.WriteFile(filepath.Join(dir, "entity.json"), []byte("{broken"), 0o600)
	if st := s.State(); st.State != Locked || !strings.Contains(st.Reason, "damaged") {
		t.Fatalf("%+v", st)
	}
	if d := s.Authorize("engine", "model.request", "", ""); d.Allow {
		t.Fatal("damaged entity allowed")
	}
}

func TestUnlockThrottles(t *testing.T) {
	s := open(t, t.TempDir())
	s.Create("Nova", "correct horse battery")
	s.Lock()
	for i := 0; i < 3; i++ {
		s.Unlock("wrong wrong wrong")
	}
	if _, err := s.Unlock("correct horse battery"); err != ErrThrottled {
		t.Fatalf("expected throttling after 3 wrong tries: %v", err)
	}
}
