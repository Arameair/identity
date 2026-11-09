package entity

// Bound, single-use decisions (Thought-Processor Stage 2, design rev 4 §2).
//
// Some actions are only ever allowed for ONE exact thing: a binding (sha256 hex of the exact tuple
// the caller computed) is required. Core may obtain ("mint") such a decision; only the Engine may
// redeem it, once, before it expires. Redeeming re-checks that the entity is still UNLOCKED, still
// the same entity, and (for actions that need one) that the grant is still held at that moment.
// A redeemed decision can be CHECKED again by the Engine (same rules, not consumed) at its later
// points of use; that is how a revocation mid-run is seen.
//
// Bound decisions live in memory only: an Identity restart forgets all of them, which fails closed
// (nothing can be redeemed). Every mint, redeem and check is appended to decisions.jsonl.

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// BoundActions: action → whether a grant of the same name is required (besides UNLOCKED).
var BoundActions = map[string]bool{
	"cognition.nudge.process": true,  // run one nudge: owner grant cognition.nudge-process
	"cognition.nudge.stop":    false, // stop one submitted nudge: UNLOCKED only (owner decision D15)
}

// BoundTTL is how long a minted decision may wait to be redeemed.
var BoundTTL = 120 * time.Second

const (
	MintService   = "core"
	RedeemService = "engine"
	maxBound      = 1000
)

type bound struct {
	d         Decision
	binding   string
	expiresAt time.Time
	used      bool
}

type boundTable struct {
	mu sync.Mutex
	m  map[string]*bound
}

func (s *Store) bt() *boundTable {
	s.boundOnce.Do(func() { s.bound = &boundTable{m: map[string]*bound{}} })
	return s.bound
}

// BoundDecision is a Decision plus its binding and expiry.
type BoundDecision struct {
	Decision
	Binding   string    `json:"binding,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Mode      string    `json:"mode,omitempty"` // mint | redeem | check
	For       string    `json:"for_decision,omitempty"`
}

func validBinding(b string) bool {
	if len(b) != 64 {
		return false
	}
	_, err := hex.DecodeString(b)
	return err == nil
}

// allowNow is the shared rule: UNLOCKED, the entity (if given) is this one, and the grant if required.
func (s *Store) allowNow(action, entityID string) (ok bool, reason string, ent string) {
	st := s.State()
	if st.Entity != nil {
		ent = st.Entity.EntityID
	}
	switch {
	case st.State != Unlocked:
		return false, st.State + ": " + st.Reason, ent
	case entityID != "" && entityID != ent:
		return false, "that entity isn't this installation's entity", ent
	case BoundActions[action] && !contains(st.Entity.Grants, action):
		return false, "the entity has no grant for " + action, ent
	}
	return true, "unlocked; " + action + " allowed", ent
}

// MintBound: only Core, only for a bound action, only with a binding.
func (s *Store) MintBound(service, action, binding, detail, entityID string) BoundDecision {
	d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: action, Detail: trim(detail, 200)},
		Binding: binding, Mode: "mint"}
	_, isBound := BoundActions[action]
	switch {
	case !isBound:
		d.Reason = action + " is not a bound action"
	case service != MintService:
		d.Reason = "only " + MintService + " may obtain " + action
	case !validBinding(binding):
		d.Reason = "a binding (sha256 hex) is required for " + action
	default:
		ok, why, ent := s.allowNow(action, entityID)
		d.EntityID, d.Reason = ent, why
		if ok {
			t := s.bt()
			t.mu.Lock()
			now := s.Now()
			for id, b := range t.m { // drop expired ones so memory stays bounded
				if now.After(b.expiresAt) {
					delete(t.m, id)
				}
			}
			if len(t.m) >= maxBound {
				d.Reason = "too many pending bound decisions"
			} else {
				d.Allow, d.ExpiresAt = true, now.UTC().Add(BoundTTL)
				t.m[d.ID] = &bound{d: d.Decision, binding: binding, expiresAt: d.ExpiresAt}
				t.m[d.ID].d.Allow = true
			}
			t.mu.Unlock()
		}
	}
	s.auditBound(d)
	return d
}

// RedeemBound: only the Engine; consumes the decision. consume=false is a CHECK of an already
// redeemed decision (same rules, the decision must have been redeemed before).
func (s *Store) RedeemBound(service, decisionID, action, binding string, consume bool) BoundDecision {
	mode := "redeem"
	if !consume {
		mode = "check"
	}
	d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: action},
		Binding: binding, Mode: mode, For: decisionID}
	if service != RedeemService {
		d.Reason = "only " + RedeemService + " may " + mode + " " + action
		s.auditBound(d)
		return d
	}
	t := s.bt()
	t.mu.Lock()
	b := t.m[decisionID]
	switch {
	case b == nil:
		d.Reason = "no such bound decision (unknown, expired and dropped, or Identity restarted)"
	case b.d.Action != action:
		d.Reason = "the decision is for another action"
	case b.binding != binding:
		d.Reason = "the binding does not match the decision"
	case consume && b.used:
		d.Reason = "the decision was already used"
	case consume && s.Now().After(b.expiresAt):
		d.Reason = "the decision expired"
	case !consume && !b.used:
		d.Reason = "the decision has not been redeemed"
	default:
		ok, why, ent := s.allowNow(action, b.d.EntityID)
		d.EntityID, d.Reason = ent, why
		if ok {
			d.Allow = true
			if consume {
				b.used = true
				b.expiresAt = s.Now().Add(24 * time.Hour) // keep it checkable during the run
			}
		}
	}
	t.mu.Unlock()
	s.auditBound(d)
	return d
}

func (s *Store) auditBound(d BoundDecision) {
	b, err := json.Marshal(d)
	if err != nil {
		return
	}
	if f, err := os.OpenFile(s.path("decisions.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		f.Write(append(b, '\n'))
		f.Close()
	}
}

// Dir is the Store's data folder (tests reopen it to simulate a restart).
func (s *Store) Dir() string { return s.dir }
