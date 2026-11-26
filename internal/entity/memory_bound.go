package entity

// Memory decisions for the controlled integration (TASS #24 7182931062 / 7183443146; isolated candidate).
//
// memory.read / memory.write are bound, single-use decisions. Identity is the AUTHORITY for scope:
// Core states the cleartext scope (channel, thread, audience, record digest); Identity checks the
// audience against the owner's table memory-audiences.json in Identity's own data folder (missing or
// unlisted thread = deny), and computes the binding itself with the contract formula shared with the
// memory gate (memory boundary/identity.go). A Core that claims a wider audience, or another thread,
// gets no decision. Only the memory-gate service redeems them; outbound publishing stays with the
// Bridge (github_mailbox.go).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

const (
	MemoryRead     = "memory.read"
	MemoryWrite    = "memory.write"
	memoryAudFile  = "memory-audiences.json" // {"github|github:Arameair/TASS#24": "team"}
	memoryContract = "tass-memory-v1"
)

// MemoryGateService redeems memory decisions with its own service key. It is redeem-only (RedeemOnly):
// it can never obtain decisions, and Core can never redeem them. In the integration candidate the gate
// still runs inside Core's process and holds that key, so the separation is real at Identity but not at
// the host (Model B blocker until the gate runs as its own process/user).
const MemoryGateService = "memory-gate"

func init() {
	BoundActions[MemoryRead] = false // UNLOCKED + owner's audience table
	BoundActions[MemoryWrite] = false
	BoundRedeemers[MemoryRead] = MemoryGateService
	BoundRedeemers[MemoryWrite] = MemoryGateService
}

// MemoryScope is what Core asks to read or write, in clear.
type MemoryScope struct {
	Channel  string `json:"channel"`
	Thread   string `json:"thread"`
	Audience string `json:"audience"`
	Record   string `json:"record"` // sha256 hex of the exact record (write); "" for read
}

var (
	memThreadRe = regexp.MustCompile(`^[A-Za-z0-9_.:/#@-]{1,128}$`)
	memAudRe    = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)
)

// MemoryBinding is the contract formula shared with the memory gate.
func MemoryBinding(action, entityID string, sc MemoryScope) string {
	h := sha256.New()
	for _, p := range []string{memoryContract, action, entityID, "conversation", sc.Channel, sc.Thread, sc.Audience, sc.Record} {
		fmt.Fprintf(h, "%d:%s|", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Store) memoryAudience(channel, thread string) (string, bool) {
	b, err := os.ReadFile(s.path(memoryAudFile))
	if err != nil {
		return "", false
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil {
		return "", false
	}
	a, ok := m[channel+"|"+thread]
	return a, ok && memAudRe.MatchString(a)
}

// MintMemory: only Core, only memory.read/write, only for a scope the owner's table allows.
func (s *Store) MintMemory(service, action string, sc MemoryScope, detail, entityID string) BoundDecision {
	deny := func(why string) BoundDecision {
		d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: action,
			Detail: trim(detail, 200), Reason: why}, Mode: "mint"}
		s.auditBound(d)
		return d
	}
	switch {
	case action != MemoryRead && action != MemoryWrite:
		return deny(action + " is not a memory action")
	case !memThreadRe.MatchString(sc.Thread) || (sc.Channel != "github" && sc.Channel != "local" && sc.Channel != "discord"):
		return deny("invalid memory scope")
	case action == MemoryWrite && !validBinding(sc.Record):
		return deny("memory.write needs the record digest")
	case action == MemoryRead && sc.Record != "":
		return deny("memory.read takes no record")
	}
	want, ok := s.memoryAudience(sc.Channel, sc.Thread)
	if !ok {
		return deny("no owner-approved audience for " + sc.Channel + " " + sc.Thread)
	}
	if sc.Audience != want {
		return deny(fmt.Sprintf("audience %q is not the owner's audience for this thread", sc.Audience))
	}
	st := s.State()
	ent := entityID
	if ent == "" && st.Entity != nil {
		ent = st.Entity.EntityID
	}
	return s.MintBound(service, action, MemoryBinding(action, ent, sc), detail, entityID)
}
