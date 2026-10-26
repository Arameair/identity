package pairing

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestP1PostReplayRefusedAfterRestart(t *testing.T) {
    dir := t.TempDir()
    s := Open(dir)
    code, _, err := s.NewCode()
    if err != nil { t.Fatal(err) }
    secret, err := s.Pair(code, "tp_p1_replay", "replay test")
    if err != nil { t.Fatal(err) }
    now := time.Now()
    ts, nonce, mac := signed(secret, "tp_p1_replay", "POST", "/api/v1/identities", `{"name":"a","kind":"person"}`, now)
    verify := func(store *Store) error {
        _, _, err := store.Verify("tp_p1_replay", "POST", "/api/v1/identities", ts, nonce, mac, []byte(`{"name":"a","kind":"person"}`))
        return err
    }
    if err := verify(s); err != nil { t.Fatalf("first mutation refused: %v", err) }
    if err := verify(Open(dir)); err != ErrReplay {
        t.Fatalf("POST replay after restart allowed: %v", err)
    }
}

func TestP1ReplayLedgerFailureDeniesSignedMutation(t *testing.T) {
    dir := t.TempDir()
    s := Open(dir)
    code, _, _ := s.NewCode()
    secret, err := s.Pair(code, "tp_p1_failure", "disk test")
    if err != nil { t.Fatal(err) }
    target := filepath.Join(dir, "paired-nonces.json")
    if err := os.Mkdir(target, 0o700); err != nil { t.Fatal(err) }
    ts, nonce, mac := signed(secret, "tp_p1_failure", "POST", "/api/v1/identities", "{}", time.Now())
    verify := func() error {
        _, _, err := s.Verify("tp_p1_failure", "POST", "/api/v1/identities", ts, nonce, mac, []byte("{}"))
        return err
    }
    if err := verify(); err != ErrReplayStore {
        t.Fatalf("mutation allowed despite unavailable replay storage: %v", err)
    }
    if err := os.Remove(target); err != nil { t.Fatal(err) }
    if err := verify(); err != nil { t.Fatalf("mutation still refused after storage recovery: %v", err) }
    if err := verify(); err != ErrReplay { t.Fatalf("nonce was not consumed: %v", err) }
}

func TestP1CorruptLedgerFailsClosed(t *testing.T) {
    dir := t.TempDir()
    s := Open(dir)
    code, _, _ := s.NewCode()
    secret, err := s.Pair(code, "tp_p1_corrupt", "disk test")
    if err != nil { t.Fatal(err) }
    if err := os.WriteFile(filepath.Join(dir, "paired-nonces.json"), []byte("{damaged"), 0o600); err != nil { t.Fatal(err) }
    restarted := Open(dir)
    if restarted.LoadError() == "" { t.Fatal("corrupt ledger not reported") }
    ts, nonce, mac := signed(secret, "tp_p1_corrupt", "POST", "/api/v1/identities", "{}", time.Now())
    if _, _, err := restarted.Verify("tp_p1_corrupt", "POST", "/api/v1/identities", ts, nonce, mac, []byte("{}")); err != ErrReplayStore {
        t.Fatalf("corrupt ledger allowed signed operation: %v", err)
    }
}
