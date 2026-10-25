package logbook

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSurvivesRestartAndKeepsIDsIncreasing(t *testing.T) {
	dir := t.TempDir()
	b1, err := Open(dir, "run_one")
	if err != nil {
		t.Fatal(err)
	}
	b1.Info("lifecycle", "identity.started", "", "ok", "first run", nil)
	b1.Error("github", "github.signin", "req_1", "denied", "user cancelled", map[string]string{"status": "denied"})
	last := b1.Stats().LastID

	b2, err := Open(dir, "run_two")
	if err != nil {
		t.Fatal(err)
	}
	got := b2.After(0, 0)
	if len(got) != 2 || got[0].RunID != "run_one" || got[1].Event != "github.signin" {
		t.Fatalf("history not reloaded: %+v", got)
	}
	e := b2.Info("lifecycle", "identity.started", "", "ok", "second run", nil)
	if e.ID <= last || e.RunID != "run_two" {
		t.Fatalf("id/run not continued: %+v (last %d)", e, last)
	}
	if p := b2.Recent(10, true); len(p) != 1 || p[0].Level != "error" {
		t.Fatalf("problems filter: %+v", p)
	}
}

func TestEntriesAreRedactedOnDisk(t *testing.T) {
	b, _ := Open(t.TempDir(), "r")
	b.Info("web", "x", "", "ok", "link http://127.0.0.1:7798/#key=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa token ghu_SECRETSECRETSECRETSECRET1234", map[string]string{"auth": "Authorization: Bearer SECRETbearer123"})
	raw, _ := b.ReadRaw()
	for _, bad := range []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "ghu_SECRET", "SECRETbearer"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("%q written to disk: %s", bad, raw)
		}
	}
}

func TestTamperedLinesAreSkippedAndCounted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, FileName), []byte("{not json\n{\"id\":5,\"ts\":\"2026-10-25T00:00:00Z\",\"event\":\"ok\"}\ngarbage\n"), 0o600)
	b, err := Open(dir, "r")
	if err != nil {
		t.Fatal(err)
	}
	s := b.Stats()
	if s.UnreadableLines != 2 || s.InMemory != 1 || s.LastID != 5 {
		t.Fatalf("stats %+v", s)
	}
	if e := b.Info("c", "e", "", "", "", nil); e.ID != 6 {
		t.Fatalf("next id %d", e.ID)
	}
}

func TestRotationIsBounded(t *testing.T) {
	dir := t.TempDir()
	b, _ := Open(dir, "r")
	big := strings.Repeat("x", 4000)
	for i := 0; i < 1200; i++ { // ~4.8 MB of detail -> several rotations
		b.Info("c", "e", "", "", big, nil)
	}
	if b.Stats().Rotations < 3 {
		t.Fatalf("expected rotations, got %+v", b.Stats())
	}
	files, _ := filepath.Glob(filepath.Join(dir, FileName+"*"))
	if len(files) > Keep+1 {
		t.Fatalf("too many log files kept: %v", files)
	}
	for _, f := range files {
		fi, _ := os.Stat(f)
		if fi.Size() > MaxBytes {
			t.Fatalf("%s exceeds cap: %d", f, fi.Size())
		}
	}
	if b.Stats().InMemory > RingSize {
		t.Fatal("ring unbounded")
	}
}

func TestWriteFailureIsVisibleNotFatal(t *testing.T) {
	_ = runtime.GOOS
	dir := t.TempDir()
	b, _ := Open(dir, "r")
	// Replace the log file with a directory: every append now fails, even as root.
	os.Remove(filepath.Join(dir, FileName))
	os.Mkdir(filepath.Join(dir, FileName), 0o700)
	b.Info("c", "e", "", "", "", nil)
	if s := b.Stats(); s.WriteErrors == 0 || s.InMemory == 0 {
		t.Fatalf("write failure not recorded: %+v", s)
	}
}
