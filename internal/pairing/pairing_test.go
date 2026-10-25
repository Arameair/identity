package pairing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/redact"
)

func TestPairAuthenticateUnpair(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	code, exp, err := s.NewCode()
	if err != nil || len(code) != 9 || time.Until(exp) > CodeTTL {
		t.Fatalf("code %q %v %v", code, exp, err)
	}
	if strings.Contains(redact.String("code "+code), code) {
		t.Fatal("pairing code not redacted")
	}
	token, err := s.Pair(strings.ToLower(strings.ReplaceAll(code, "-", "")), "tp_test01", "Processor on test")
	if err != nil || len(token) != 64 {
		t.Fatalf("pair: %v", err)
	}
	if _, err := s.Pair(code, "tp_test02", ""); err == nil {
		t.Fatal("code reused")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "paired-clients.json"))
	if strings.Contains(string(raw), token) || !strings.Contains(string(raw), hashToken(token)) {
		t.Fatal("token stored in clear or hash missing")
	}
	if c, ok := Open(dir).Authenticate(token); !ok || c.ProcessorID != "tp_test01" {
		t.Fatal("pairing did not survive reopen")
	}
	if _, ok := s.Authenticate(strings.Repeat("a", 64)); ok {
		t.Fatal("wrong token accepted")
	}
	if ok, _ := s.Unpair("tp_test01"); !ok {
		t.Fatal("unpair")
	}
	if _, ok := s.Authenticate(token); ok {
		t.Fatal("token works after unpair")
	}
}

func TestCodeExpiresAndLocksOut(t *testing.T) {
	s := Open(t.TempDir())
	now := time.Now()
	s.now = func() time.Time { return now }
	code, _, _ := s.NewCode()
	now = now.Add(CodeTTL + time.Second)
	if _, err := s.Pair(code, "tp_test01", ""); err != ErrBadCode {
		t.Fatalf("expired code accepted: %v", err)
	}
	code, _, _ = s.NewCode()
	for i := 0; i < maxFailures; i++ {
		s.Pair("WRON-GGGG", "tp_test01", "")
	}
	if _, err := s.Pair(code, "tp_test01", ""); err != ErrLocked {
		t.Fatalf("no lockout after %d failures: %v", maxFailures, err)
	}
	now = now.Add(lockDuration + time.Second)
	if _, err := s.Pair(code, "tp_test01", ""); err != ErrBadCode {
		t.Fatal("guessed-at code should have been burned")
	}
}

func TestDamagedFileDisablesPairing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "paired-clients.json"), []byte("{broken"), 0o600)
	s := Open(dir)
	if s.LoadError() == "" {
		t.Fatal("damage not reported")
	}
	if _, _, err := s.NewCode(); err == nil {
		t.Fatal("pairing allowed on damaged file")
	}
}

func TestProofDependsOnToken(t *testing.T) {
	if Proof("a", "x") == Proof("b", "x") || Proof("a", "x") != Proof("a", "x") {
		t.Fatal("proof not keyed")
	}
}
