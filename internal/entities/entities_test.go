package entities

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCreateListPersistAndDefaultNoAccess(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	e, err := s.Create("  Tass   Companion ", "companion", "processor:tp_test")
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(e.ID) || e.Name != "Tass Companion" || e.Kind != "companion" {
		t.Fatalf("bad entity %+v", e)
	}
	if len(e.EnabledFeatures) != 0 || len(e.AuthorizedSources) != 0 || e.EnabledFeatures == nil {
		t.Fatalf("new identity must start with explicit empty access: %+v", e)
	}
	if _, err := s.Create("tass companion", "person", "x"); err != ErrDupName {
		t.Fatalf("duplicate active name allowed: %v", err)
	}
	if _, err := s.Create("x", "system", "x"); err != ErrKind {
		t.Fatalf("system kind allowed: %v", err)
	}
	if _, err := s.Create("", "person", "x"); err != ErrName {
		t.Fatalf("empty name allowed: %v", err)
	}
	fi, err := os.Stat(filepath.Join(dir, "identities.json"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("identities.json readable by others: %v", fi.Mode())
	}

	s2 := Open(dir) // restart
	got, ok := s2.Get(e.ID)
	if !ok || got.Name != e.Name || !got.CreatedAt.Equal(e.CreatedAt) {
		t.Fatalf("identity did not survive restart: %+v", got)
	}
}

func TestArchiveRestoreRename(t *testing.T) {
	s := Open(t.TempDir())
	a, _ := s.Create("Alpha", "person", "x")
	if _, err := s.Archive(a.ID); err != nil {
		t.Fatal(err)
	}
	if len(s.List(false)) != 0 || len(s.List(true)) != 1 {
		t.Fatal("archived identity still in normal list")
	}
	if _, err := s.Rename(a.ID, "Beta"); err != ErrArchived {
		t.Fatalf("renamed archived identity: %v", err)
	}
	b, _ := s.Create("Alpha", "person", "x") // name free while archived
	if _, err := s.Restore(a.ID); err != ErrDupName {
		t.Fatalf("restore clashed silently: %v", err)
	}
	s.Rename(b.ID, "Gamma")
	if r, err := s.Restore(a.ID); err != nil || r.ArchivedAt != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if _, err := s.Rename("ent_00000000000000000000", "x"); err != ErrNotFound {
		t.Fatal("rename of unknown id")
	}
}

func TestDamagedFileIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "identities.json")
	os.WriteFile(p, []byte("{not json"), 0o600)
	s := Open(dir)
	if s.LoadError() == "" {
		t.Fatal("damage not reported")
	}
	if _, err := s.Create("A", "person", "x"); err == nil {
		t.Fatal("create allowed on damaged store")
	}
	if b, _ := os.ReadFile(p); string(b) != "{not json" {
		t.Fatal("damaged file was overwritten")
	}
}
