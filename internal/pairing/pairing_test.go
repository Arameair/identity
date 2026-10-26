package pairing

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/redact"
)

func signed(secret, pid, method, path, body string, at time.Time) (ts, nonce, mac string) {
	rk, _ := DeriveKeys(secret)
	ts = strconv.FormatInt(at.Unix(), 10)
	nonce = strings.Repeat("ab", 8) + strconv.FormatInt(at.UnixNano()%1e9, 16)
	return ts, nonce, RequestMAC(rk, method, path, ts, nonce, []byte(body))
}

func TestPairVerifyUnpair(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	code, exp, err := s.NewCode()
	if err != nil || len(code) != 9 || time.Until(exp) > CodeTTL {
		t.Fatalf("code %q %v %v", code, exp, err)
	}
	if strings.Contains(redact.String("code "+code), code) {
		t.Fatal("pairing code not redacted")
	}
	secret, err := s.Pair(strings.ToLower(strings.ReplaceAll(code, "-", "")), "tp_test01", "Processor on test")
	if err != nil || len(secret) != 64 {
		t.Fatalf("pair: %v", err)
	}
	if _, err := s.Pair(code, "tp_test02", ""); err == nil {
		t.Fatal("code reused")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "paired-clients.json"))
	if strings.Contains(string(raw), secret) || !strings.Contains(string(raw), "request_key") {
		t.Fatal("secret stored in clear, or keys missing")
	}
	now := time.Now()
	ts, nonce, mac := signed(secret, "tp_test01", "GET", "/api/v1/status", "", now)
	c, sk, err := Open(dir).Verify("tp_test01", "GET", "/api/v1/status", ts, nonce, mac, nil)
	_, wantSK := DeriveKeys(secret)
	if err != nil || c.ProcessorID != "tp_test01" || string(sk) != string(wantSK) {
		t.Fatalf("pairing did not survive reopen: %v", err)
	}
	// Replay of the exact same request is refused.
	if _, _, err := s.Verify("tp_test01", "GET", "/api/v1/status", ts, nonce, mac, nil); err != nil {
		t.Fatalf("first use refused: %v", err)
	}
	if _, _, err := s.Verify("tp_test01", "GET", "/api/v1/status", ts, nonce, mac, nil); err != ErrReplay {
		t.Fatalf("replay accepted: %v", err)
	}
	// Changing anything signed breaks it.
	ts, nonce, mac = signed(secret, "tp_test01", "POST", "/api/v1/identities", `{"name":"a"}`, now.Add(time.Millisecond))
	for _, tc := range []struct{ m, p, body string }{{"POST", "/api/v1/identities", `{"name":"b"}`}, {"POST", "/api/v1/shutdown", `{"name":"a"}`}, {"GET", "/api/v1/identities", `{"name":"a"}`}} {
		if _, _, err := s.Verify("tp_test01", tc.m, tc.p, ts, nonce, mac, []byte(tc.body)); err != ErrBadMAC {
			t.Fatalf("tampered %v accepted: %v", tc, err)
		}
	}
	// Stale or future time refused.
	ts, nonce, mac = signed(secret, "tp_test01", "GET", "/x", "", now.Add(-2*MaxClockSkew))
	if _, _, err := s.Verify("tp_test01", "GET", "/x", ts, nonce, mac, nil); err != ErrStale {
		t.Fatalf("stale accepted: %v", err)
	}
	// Someone who only knows the processor id can't sign.
	ts, nonce, mac = signed(strings.Repeat("0", 64), "tp_test01", "GET", "/x", "", now)
	if _, _, err := s.Verify("tp_test01", "GET", "/x", ts, nonce, mac, nil); err != ErrBadMAC {
		t.Fatalf("wrong secret accepted: %v", err)
	}
	if ok, _ := s.Unpair("tp_test01"); !ok {
		t.Fatal("unpair")
	}
	ts, nonce, mac = signed(secret, "tp_test01", "GET", "/x", "", now.Add(2*time.Millisecond))
	if _, _, err := s.Verify("tp_test01", "GET", "/x", ts, nonce, mac, nil); err != ErrUnknownClient {
		t.Fatalf("works after unpair: %v", err)
	}
}

func TestOldBearerPairingMustPairAgain(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "paired-clients.json"), []byte(`[{"processor_id":"tp_old01","label":"old","token_sha256":"`+strings.Repeat("a", 64)+`","paired_at":"2026-10-25T00:00:00Z"}]`), 0o600)
	ts, nonce, mac := signed(strings.Repeat("1", 64), "tp_old01", "GET", "/x", "", time.Now())
	if _, _, err := Open(dir).Verify("tp_old01", "GET", "/x", ts, nonce, mac, nil); err != ErrOldPairing {
		t.Fatalf("old v1 pairing not flagged: %v", err)
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

func TestProofsAreKeyed(t *testing.T) {
	a, b := []byte("a"), []byte("b")
	if Proof(a, "x") == Proof(b, "x") || ResponseProof(a, "n", 200, nil) == ResponseProof(a, "n", 404, nil) || ResponseProof(a, "n", 200, nil) == ResponseProof(a, "m", 200, nil) {
		t.Fatal("proof not keyed/bound")
	}
	rk, sk := DeriveKeys("s")
	if string(rk) == string(sk) {
		t.Fatal("request and response keys must differ")
	}
}
