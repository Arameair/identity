package entity

// Bound, single-use decisions (Thought-Processor Stage 2, design rev 4 §2).
//
// Some actions are only ever allowed for ONE exact thing: a binding (sha256 hex of the exact tuple
// the caller computed) is required. Core may obtain ("mint") such a decision; only the Engine (or,
// for an action listed in BoundRedeemers, the service that performs that effect) may
// redeem it, once, before it expires. Redeeming re-checks that the entity is still UNLOCKED, still
// the same entity, and (for actions that need one) that the grant is still held at that moment.
// A redeemed decision can be CHECKED again by the Engine (same rules, not consumed) at its later
// points of use; that is how a revocation mid-run is seen. Actions in SinglePointOfUse
// (discord.reply) cannot be checked at all: their one redemption is their only point of use.
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
	EnhanceFastAIWorker: true,
	EnhanceTeamMember: true,
	"cognition.nudge.process": true,  // run one nudge: owner grant cognition.nudge-process
	"cognition.nudge.stop":    false, // stop one submitted nudge: UNLOCKED only (owner decision D15)
	// Stage 3B (Thought-Processor #120, owner decision D7): run ONE arm of one isolated experiment.
	// Its own owner grant, cognition.experiment-run; never implied by, or shared with, the nudge grants.
	"cognition.experiment.run": true,
	// Discord Milestone A (identity #46, Thought-Processor #127): send ONE exact reply to ONE exact
	// inbound Discord message. Its own owner grant, discord.reply; not a default and never implied by
	// any other grant. Redeemed by the Discord door, not the Engine (see BoundRedeemers).
	DiscordReply: true,
}

// BoundTTL is how long a minted decision may wait to be redeemed.
var BoundTTL = 120 * time.Second

const (
	MintService   = "core"
	RedeemService = "engine" // redeems every bound action that has no entry in BoundRedeemers
	maxBound      = 1000

	// DiscordReply is the bound action for one outbound Discord reply.
	DiscordReply = "discord.reply"
	// DoorService is the Discord door's service name in the service-key file. The door is the
	// transport that actually posts, so it (and only it) redeems discord.reply at the point of use.
	DoorService = "discord-door"
)

// BoundRedeemers: bound actions that are redeemed by a service other than the Engine.
// The service that performs the effect redeems; nobody else can.
var BoundRedeemers = map[string]string{
	DiscordReply: DoorService,
	EnhanceFastAIWorker: "enhancement-sender",
	EnhanceTeamMember: "enhancement-sender",
}

// SinglePointOfUse: bound actions whose effect happens once, at the moment of redemption, so a
// redeemed decision must never answer allow again. A CHECK (consume=false) is refused for them
// outright: otherwise the redeemer could re-ask "check" after its one redemption and receive
// allow=true again for as long as the decision stays in memory (the discord.reply check bypass,
// identity #47 audit). Actions not listed keep the Engine's established redeem-then-check flow.
var SinglePointOfUse = map[string]bool{
	EnhanceFastAIWorker: true,
	EnhanceTeamMember: true,
	DiscordReply: true,
}

// redeemerFor is the one service that may redeem (or check) a decision for action.
func redeemerFor(action string) string {
	if svc, ok := BoundRedeemers[action]; ok {
		return svc
	}
	return RedeemService
}

// RedeemOnly reports whether service exists solely to redeem bound decisions. Such a service may
// use no other service route: it cannot ask for plain authorizations or read anything else.
func RedeemOnly(service string) bool {
	for _, svc := range BoundRedeemers {
		if svc == service {
			return true
		}
	}
	return false
}

type bound struct {
	d         Decision
	binding   string
	expiresAt time.Time
	used      bool
	enhancementScope *EnhancementScope
	enhancementEpoch uint64
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
	return s.mintBound(service, action, binding, detail, entityID, nil)
}

// An enhancement scope is supplied only by MintEnhancement under enhancementMu.
func (s *Store) mintBound(service, action, binding, detail, entityID string, enhancementScope *EnhancementScope) BoundDecision {
	d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: action, Detail: trim(detail, 200)},
		Binding: binding, Mode: "mint"}
	_, isBound := BoundActions[action]
	switch {
	case !isBound:
		d.Reason = action + " is not a bound action"
	case IsEnhancementAction(action) && enhancementScope == nil:
		d.Reason = action + " requires a validated enhancement scope"
	case OwnerMintOnly[action]:
		d.Reason = action + " is minted only by the owner's approval on Identity's page"
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
				if enhancementScope != nil {
					scope := *enhancementScope
					t.m[d.ID].enhancementScope = &scope
					t.m[d.ID].enhancementEpoch = s.enhancementEpoch[action]
				}
			}
			t.mu.Unlock()
		}
	}
	if err := s.auditBound(d); err != nil && d.Allow && IsEnhancementAction(action) {
		t := s.bt()
		t.mu.Lock()
		delete(t.m, d.ID)
		t.mu.Unlock()
		d.Allow, d.Reason, d.ExpiresAt = false, "could not write enhancement mint audit", time.Time{}
	}
	return d
}

// RedeemBound: only the action's redeemer (the Engine, or the service named in BoundRedeemers);
// consumes the decision. consume=false is a CHECK of an already redeemed decision (same rules, the
// decision must have been redeemed before).
func (s *Store) RedeemBound(service, decisionID, action, binding string, consume bool) BoundDecision {
	mode := "redeem"
	if !consume {
		mode = "check"
	}
	d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: action},
		Binding: binding, Mode: mode, For: decisionID}
	if want := redeemerFor(action); service != want {
		d.Reason = "only " + want + " may " + mode + " " + action
		s.auditBound(d)
		return d
	}
	if !consume && SinglePointOfUse[action] {
		d.Reason = action + " has a single point of use: it can be redeemed once and never checked"
		s.auditBound(d)
		return d
	}
	if IsEnhancementAction(action) {
		s.enhancementMu.Lock()
		defer s.enhancementMu.Unlock()
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
	case IsEnhancementAction(action) && (b.enhancementScope == nil || b.enhancementEpoch != s.enhancementEpoch[action] || !s.enhancementRecipientAllowed(action, *b.enhancementScope)):
		d.Reason = "enhancement grant was revoked or recipient is no longer pinned"
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
	if err := s.auditBound(d); err != nil && d.Allow && IsEnhancementAction(action) {
		// Keep the decision consumed even if its audit fails.
		d.Allow, d.Reason = false, "could not write enhancement redemption audit"
	}
	return d
}

func (s *Store) auditBound(d BoundDecision) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.path("decisions.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	if err == nil && d.Allow && IsEnhancementAction(d.Action) {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Dir is the Store's data folder (tests reopen it to simulate a restart).
func (s *Store) Dir() string { return s.dir }
