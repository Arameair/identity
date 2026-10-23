package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// B + C + D: create, list, open, restore exactly, distinct second identity.
func TestCreateRestoreAndIndependence(t *testing.T) {
	dir := t.TempDir()
	s := mustStore(t, dir)
	tass, err := s.Create("TASS", "ai_companion", "req-1", false)
	if err != nil || tass.Repeat {
		t.Fatal(err)
	}
	if !uuidRe.MatchString(tass.Identity.ID) {
		t.Fatalf("bad id %q", tass.Identity.ID)
	}
	// restart: a brand-new store on the same folder
	s2 := mustStore(t, dir)
	got, err := s2.Get(tass.Identity.ID)
	if err != nil || got != tass.Identity {
		t.Fatalf("restore changed the record:\n before %+v\n after  %+v (%v)", tass.Identity, got, err)
	}
	biz, err := s2.Create("Example Workshop", "business", "req-2", false)
	if err != nil || biz.Identity.ID == tass.Identity.ID {
		t.Fatal(err)
	}
	s3 := mustStore(t, dir)
	list := s3.List()
	if len(list) != 2 || list[0] != tass.Identity || list[1] != biz.Identity {
		t.Fatalf("after second restart: %+v", list)
	}
	// the file on disk holds exactly the spec's fields
	b, _ := os.ReadFile(filepath.Join(dir, "identities", tass.Identity.ID+".json"))
	var raw map[string]any
	json.Unmarshal(b, &raw)
	if len(raw) != 5 || raw["schema_version"] != float64(1) || raw["display_name"] != "TASS" || raw["entity_type"] != "ai_companion" {
		t.Fatalf("record on disk: %s", b)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "identities", tass.Identity.ID+".json")); fi.Mode().Perm()&0o077 != 0 && os.Getenv("OS") != "Windows_NT" {
		t.Fatalf("record readable by others: %v", fi.Mode())
	}
}

func TestValidation(t *testing.T) {
	s := mustStore(t, t.TempDir())
	for _, c := range []struct{ name, typ string }{
		{"", "business"}, {"   ", "business"}, {strings.Repeat("x", 81), "business"},
		{"bad\x00name", "business"}, {"line\nbreak", "business"}, {"TASS", "wizard"}, {"TASS", ""}, {"\xff\xfe", "other"},
	} {
		if _, err := s.Create(c.name, c.typ, "", false); !errors.Is(err, ErrInvalid) {
			t.Errorf("Create(%q, %q) = %v, want invalid", c.name, c.typ, err)
		}
	}
	if len(s.List()) != 0 {
		t.Fatal("invalid input must not create anything")
	}
	if r, err := s.Create("  Ünïcode Café 🙂  ", "other", "", false); err != nil || r.Identity.DisplayName != "Ünïcode Café 🙂" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := s.Get("../../etc/passwd"); !errors.Is(err, ErrInvalid) {
		t.Fatal("path-like IDs must be rejected")
	}
}

func TestRepeatedRequestCreatesOnce(t *testing.T) {
	s := mustStore(t, t.TempDir())
	a, _ := s.Create("TASS", "ai_companion", "same-click", false)
	b, err := s.Create("TASS", "ai_companion", "same-click", false)
	if err != nil || !b.Repeat || b.Identity != a.Identity || len(s.List()) != 1 {
		t.Fatalf("double click made a second identity: %+v %v", b, err)
	}
}

func TestDuplicateNameNeedsConfirmation(t *testing.T) {
	s := mustStore(t, t.TempDir())
	a, _ := s.Create("TASS", "ai_companion", "r1", false)
	if _, err := s.Create("tass", "ai_companion", "r2", false); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("same name+type should ask first: %v", err)
	}
	if _, err := s.Create("TASS", "business", "r3", false); err != nil {
		t.Fatalf("same name, different type is a different entity: %v", err)
	}
	b, err := s.Create("TASS", "ai_companion", "r4", true)
	if err != nil || b.Identity.ID == a.Identity.ID {
		t.Fatalf("confirmed duplicate name gets its own ID: %v", err)
	}
	if got, _ := s.Get(a.Identity.ID); got != a.Identity {
		t.Fatal("original changed")
	}
}

func TestConcurrentCreatesAreDistinct(t *testing.T) {
	dir := t.TempDir()
	s := mustStore(t, dir)
	var wg sync.WaitGroup
	ids := make(chan string, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := s.Create("Entity", "project", "", true)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- r.Identity.ID
		}(i)
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate ID %s", id)
		}
		seen[id] = true
	}
	if n := len(mustStore(t, dir).List()); n != 40 {
		t.Fatalf("restored %d of 40", n)
	}
}

// F: corrupt and odd files are reported, left untouched, and don't hide good records.
func TestCorruptFilesAreReportedNotLost(t *testing.T) {
	dir := t.TempDir()
	s := mustStore(t, dir)
	good, _ := s.Create("TASS", "ai_companion", "", false)
	idDir := filepath.Join(dir, "identities")
	goodBytes, _ := os.ReadFile(filepath.Join(idDir, good.Identity.ID+".json"))
	os.WriteFile(filepath.Join(idDir, "broken.json"), []byte(`{"schema_version":1,"id":`), 0o600)
	os.WriteFile(filepath.Join(idDir, "11111111-1111-4111-8111-111111111111.json"), goodBytes, 0o600) // copy: name/ID mismatch
	other := strings.Replace(string(goodBytes), `"schema_version": 1`, `"schema_version": 9`, 1)
	os.WriteFile(filepath.Join(idDir, "22222222-2222-4222-8222-222222222222.json"), []byte(other), 0o600)
	os.WriteFile(filepath.Join(idDir, ".tmp-12345"), []byte("half a save"), 0o600) // interrupted save: ignored
	s2 := mustStore(t, dir)
	if len(s2.List()) != 1 || s2.List()[0] != good.Identity {
		t.Fatalf("good record lost: %+v", s2.List())
	}
	probs := s2.Problems()
	if len(probs) != 3 {
		t.Fatalf("problems: %+v", probs)
	}
	joined := ""
	for _, p := range probs {
		joined += p.File + ": " + p.Reason + "\n"
	}
	for _, want := range []string{"broken.json", "doesn't match", "schema_version 9"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(idDir, "broken.json")); string(b) != `{"schema_version":1,"id":` {
		t.Fatal("problem files must be left untouched")
	}
	// test tools
	name, err := s2.WriteCorruptTestFile()
	if err != nil || len(s2.Problems()) != 4 || !strings.HasPrefix(name, "corrupt-test-") {
		t.Fatal(err)
	}
	if n, _ := s2.RemoveCorruptTestFiles(); n != 1 || len(s2.Problems()) != 3 {
		t.Fatalf("removed %d", n)
	}
}

// F: a failed save reports an error, adds nothing, and keeps earlier records.
type failingOps struct {
	osOps
	failCreate, failRename, failWrite bool
}

type badFile struct{ *os.File }

func (b badFile) Write(p []byte) (int, error) { return 0, errors.New("no space left on device") }

func (f *failingOps) CreateTemp(dir, p string) (writeFile, error) {
	if f.failCreate {
		return nil, errors.New("permission denied")
	}
	file, err := os.CreateTemp(dir, p)
	if err != nil || !f.failWrite {
		return file, err
	}
	return badFile{file}, nil
}
func (f *failingOps) Rename(a, b string) error {
	if f.failRename {
		return errors.New("rename refused")
	}
	return os.Rename(a, b)
}

func TestFailedSaveNeverClaimsSaved(t *testing.T) {
	dir := t.TempDir()
	ops := &failingOps{}
	s, err := openStoreWith(dir, ops)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := s.Create("TASS", "ai_companion", "", false)
	for _, mode := range []string{"create", "write", "rename", "simulated"} {
		*ops = failingOps{failCreate: mode == "create", failWrite: mode == "write", failRename: mode == "rename"}
		if mode == "simulated" {
			s.FailNextSave()
		}
		r, err := s.Create("Example Workshop", "business", "req-"+mode, false)
		if err == nil || !strings.Contains(err.Error(), "save failed") || r.Identity.ID != "" {
			t.Fatalf("%s: %+v %v", mode, r, err)
		}
		if len(s.List()) != 1 {
			t.Fatalf("%s: failed save added a card", mode)
		}
	}
	*ops = failingOps{}
	entries, _ := os.ReadDir(filepath.Join(dir, "identities"))
	if len(entries) != 1 {
		t.Fatalf("leftover files after failed saves: %v", entries)
	}
	if got := mustStore(t, dir).List(); len(got) != 1 || got[0] != first.Identity {
		t.Fatal("earlier identity lost")
	}
	// the same request works once storage is healthy again
	if r, err := s.Create("Example Workshop", "business", "req-rename", false); err != nil || r.Repeat {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestReadOnlyFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions; covered by TestFailedSaveNeverClaimsSaved")
	}
	dir := t.TempDir()
	s := mustStore(t, dir)
	s.Create("TASS", "ai_companion", "", false)
	os.Chmod(filepath.Join(dir, "identities"), 0o500)
	defer os.Chmod(filepath.Join(dir, "identities"), 0o700)
	if _, err := s.Create("B", "business", "", false); err == nil {
		t.Fatal("read-only folder must fail")
	}
}

func TestCreatedAtIsStable(t *testing.T) {
	s := mustStore(t, t.TempDir())
	s.now = func() time.Time { return time.Date(2026, 10, 23, 12, 0, 0, 123456789, time.FixedZone("CDT", -5*3600)) }
	r, _ := s.Create("TASS", "ai_companion", "", false)
	if r.Identity.CreatedAt.Format(time.RFC3339Nano) != "2026-10-23T17:00:00.123456789Z" {
		t.Fatal(r.Identity.CreatedAt)
	}
}
