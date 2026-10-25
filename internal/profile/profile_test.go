package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSystemIDIsDurable(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open(dir)
	if err != nil || !s1.Created {
		t.Fatal(err)
	}
	id := s1.Profile.SystemID
	s1.UpdateSettings(func(st *Settings) { st.ProcessorAddress = "127.0.0.1:7860" })
	s1.Close()
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.Created || s2.Profile.SystemID != id || s2.Settings().ProcessorAddress != "127.0.0.1:7860" {
		t.Fatalf("profile not durable: %+v %+v", s2.Profile, s2.Settings())
	}
}

func TestSecondProcessRefused(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	if _, err := Open(dir); !errors.Is(err, ErrLocked) {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
}

func TestDamagedProfileLeftUntouched(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "profile.json")
	os.WriteFile(p, []byte(`{"schema":"identity.profile/1","system_id":"tampered"}`), 0o600)
	if _, err := Open(dir); err == nil {
		t.Fatal("tampered profile accepted")
	}
	b, _ := os.ReadFile(p)
	if string(b) != `{"schema":"identity.profile/1","system_id":"tampered"}` {
		t.Fatal("tampered profile was overwritten")
	}
	// lock must have been released so a fixed profile can start
	os.Remove(p)
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("lock leaked after failure: %v", err)
	}
	s.Close()
}

func TestProfilesIsolated(t *testing.T) {
	a, _ := Open(t.TempDir())
	b, _ := Open(t.TempDir())
	defer a.Close()
	defer b.Close()
	if a.Profile.SystemID == b.Profile.SystemID {
		t.Fatal("two profiles share a System ID")
	}
}
