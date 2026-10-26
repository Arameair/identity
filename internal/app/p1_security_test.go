package app

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/github"
)

// A signed unpair failure must never falsely claim the pairing was revoked.
func TestP1UnpairDiskFailureIsSignedAndRetainsPairing(t *testing.T) {
	dir := t.TempDir()
	h, con := startV1(t, dir, github.Config{WebBase: github.RealWeb, APIBase: github.RealAPI})
	defer h.stop()
	token, _ := h.pairV1(con, "tp_diskfail")
	target := filepath.Join(dir, "paired-clients.json")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	st, body := h.v1("POST", "/api/v1/unpair", `{}`, token)
	if st != http.StatusServiceUnavailable || body["unpaired"] == true {
		t.Fatalf("unpair falsely succeeded: %d %v", st, body)
	}
	if st, _ := h.v1("GET", "/api/v1/status", "", token); st != 200 {
		t.Fatalf("pair should remain usable after failed persistence: %d", st)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if st, resp := h.v1("POST", "/api/v1/unpair", `{}`, token); st != 200 || resp["unpaired"] != true {
		t.Fatalf("unpair after repair: %d %v", st, resp)
	}
	if st, _, _ := h.machine("GET", "/api/v1/status", "", token); st != 401 {
		t.Fatalf("revoked client still authenticated: %d", st)
	}
}

func TestP1OperatorSessionExpires(t *testing.T) {
	sessions := NewSessions()
	id, _, ok, locked := sessions.Exchange(sessions.Key())
	if !ok || locked {
		t.Fatal("cannot open local session")
	}
	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: id})
	if _, ok := sessions.Lookup(req); !ok {
		t.Fatal("new session refused")
	}
	sessions.mu.Lock()
	rec := sessions.sess[id]
	rec.created = time.Now().Add(-SessionTTL - time.Second)
	sessions.sess[id] = rec
	sessions.mu.Unlock()
	if _, ok := sessions.Lookup(req); ok {
		t.Fatal("expired session was accepted")
	}
	if sessions.Count() != 0 {
		t.Fatal("expired session was retained")
	}
}

func TestP1ReplayStorageFailureIsUnavailableNotUnauthorized(t *testing.T) {
	dir := t.TempDir()
	h, con := startV1(t, dir, github.Config{WebBase: github.RealWeb, APIBase: github.RealAPI})
	defer h.stop()
	token, _ := h.pairV1(con, "tp_ledger")
	target := filepath.Join(dir, "paired-nonces.json")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	st, _, body := h.machine("POST", "/api/v1/identities", `{"name":"A","kind":"person"}`, token)
	if st != 503 {
		t.Fatalf("replay ledger failure returned %d (body: %s), expected 503", st, body)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if st, _ := h.v1("POST", "/api/v1/identities", `{"name":"A","kind":"person"}`, token); st != 201 {
		t.Fatalf("signed API could not resume after ledger recovery: %d", st)
	}
}
