package entity

// Memory decisions (TASS #24 7183443146): Identity-authoritative audience and binding. Synthetic only.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const memThread = "github:Arameair/TASS#24"

func memStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s := open(t, dir)
	st, err := s.Create("Synthetic", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, memoryAudFile), []byte(`{"github|`+memThread+`": "team"}`), 0o600)
	return s, st.Entity.EntityID
}

func TestMemoryMintAudienceAuthority(t *testing.T) {
	s, ent := memStore(t)
	rd := MemoryScope{Channel: "github", Thread: memThread, Audience: "team"}
	d := s.MintMemory("core", MemoryRead, rd, "", "")
	if !d.Allow || d.Binding != MemoryBinding(MemoryRead, ent, rd) {
		t.Fatalf("read mint: %+v", d)
	}
	if r := s.RedeemBound("core", d.ID, MemoryRead, MemoryBinding(MemoryRead, ent, rd), true); r.Allow {
		t.Fatal("core (the minter) redeemed its own memory decision")
	}
	if r := s.RedeemBound(MemoryGateService, d.ID, MemoryRead, MemoryBinding(MemoryRead, ent, rd), true); !r.Allow {
		t.Fatalf("redeem: %+v", r)
	}
	if !RedeemOnly(MemoryGateService) || RedeemOnly("core") {
		t.Fatal("memory-gate must be redeem-only and core must stay a minter")
	}
	if d := s.MintMemory(MemoryGateService, MemoryRead, rd, "", ""); d.Allow {
		t.Fatal("memory-gate minted")
	}
	wr := rd
	wr.Record = strings.Repeat("a", 64)
	for name, c := range map[string]struct {
		svc, action string
		sc          MemoryScope
	}{
		"wider audience":      {"core", MemoryRead, MemoryScope{Channel: "github", Thread: memThread, Audience: "public"}},
		"unlisted thread":     {"core", MemoryRead, MemoryScope{Channel: "github", Thread: "github:Arameair/TASS#25", Audience: "team"}},
		"other channel":       {"core", MemoryRead, MemoryScope{Channel: "discord", Thread: memThread, Audience: "team"}},
		"engine mints":        {"engine", MemoryRead, rd},
		"write without rec":   {"core", MemoryWrite, rd},
		"read with record":    {"core", MemoryRead, wr},
		"not a memory action": {"core", "github.sync", rd},
	} {
		if d := s.MintMemory(c.svc, c.action, c.sc, "", ""); d.Allow {
			t.Errorf("%s: minted %+v", name, d)
		}
	}
	w := s.MintMemory("core", MemoryWrite, wr, "", "")
	if !w.Allow {
		t.Fatalf("write mint: %+v", w)
	}
	// A different record (or scope) cannot redeem it; the engine cannot redeem memory decisions.
	other := wr
	other.Record = strings.Repeat("b", 64)
	if r := s.RedeemBound(MemoryGateService, w.ID, MemoryWrite, MemoryBinding(MemoryWrite, ent, other), true); r.Allow {
		t.Fatal("redeemed for another record")
	}
	if r := s.RedeemBound("engine", w.ID, MemoryWrite, w.Binding, true); r.Allow {
		t.Fatal("engine redeemed a memory decision")
	}
	if r := s.RedeemBound(MemoryGateService, w.ID, MemoryWrite, w.Binding, true); !r.Allow {
		t.Fatalf("write redeem: %+v", r)
	}
	if r := s.RedeemBound(MemoryGateService, w.ID, MemoryWrite, w.Binding, true); r.Allow {
		t.Fatal("replayed")
	}
	// No audience table: deny everything.
	os.Remove(s.path(memoryAudFile))
	if d := s.MintMemory("core", MemoryRead, rd, "", ""); d.Allow {
		t.Fatal("minted without the owner's audience table")
	}
	// LOCKED.
	os.WriteFile(s.path(memoryAudFile), []byte(`{"github|`+memThread+`": "team"}`), 0o600)
	s.Lock()
	if d := s.MintMemory("core", MemoryRead, rd, "", ""); d.Allow {
		t.Fatal("minted while LOCKED")
	}
}
