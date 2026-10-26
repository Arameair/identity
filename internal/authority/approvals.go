package authority

// Admin approvals bind ONE fresh Admin decision to ONE exact proposed
// operation. A paired Thought Processor can ask for an approval, but it
// cannot approve it: the Admin approves in Identity's own page with the
// Admin authenticator. Committing the operation then needs the approval ID,
// which is checked for kind, requester, expiry, state and digest, and is
// consumed before the operation runs (so it can never run twice).

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/profile"
)

// Kinds of Admin approvals (elevated actions, owner decision #20 risk-based).
const (
	KindEnrollAdmin = "enroll_admin"
	KindInstall     = "install_feature"
	KindGrant       = "grant_capability"
	KindUninstall   = "uninstall_feature"
	KindRevokeGrant = "revoke_grant" // Admin emergency revocation of any grant
)

// Approval states.
const (
	Pending  = "PENDING"
	Approved = "APPROVED"
	Denied   = "DENIED"
	Expired  = "EXPIRED"
	Consumed = "CONSUMED"
)

const (
	ApprovalTTL    = 10 * time.Minute
	maxPending     = 20
	keepApprovals  = 300
	approvalsFile  = "approvals.json"
	approvalSchema = "identity.approvals/1"
)

var approvalIDRE = regexp.MustCompile(`^apr_[0-9a-f]{20}$`)

func ValidApprovalID(s string) bool { return approvalIDRE.MatchString(s) }

type Approval struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Action      json.RawMessage `json:"action"`
	Digest      string          `json:"action_sha256"`
	Summary     []string        `json:"summary"`
	RequestedBy string          `json:"requested_by"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
	State       string          `json:"state"`
	DecidedAt   *time.Time      `json:"decided_at,omitempty"`
	DecidedBy   string          `json:"decided_by,omitempty"`
	Method      string          `json:"method,omitempty"`
	ConsumedAt  *time.Time      `json:"consumed_at,omitempty"`
	Result      string          `json:"result,omitempty"`
	Outcome     string          `json:"outcome,omitempty"`
	// ConfirmedUntilRevoked: the Admin ticked the extra "no expiry" box.
	ConfirmedUntilRevoked bool   `json:"confirmed_until_revoked,omitempty"`
	ViewToken             string `json:"view_token_sha256"`
	ViewOpened            bool   `json:"view_opened"`
}

// View is what callers may see (no token hash).
func (a Approval) View(now time.Time) map[string]any {
	st := a.State
	if (st == Pending || st == Approved) && !now.Before(a.ExpiresAt) {
		st = Expired
	}
	return map[string]any{"id": a.ID, "kind": a.Kind, "action": a.Action, "action_sha256": a.Digest, "summary": a.Summary,
		"requested_by": a.RequestedBy, "created_at": a.CreatedAt, "expires_at": a.ExpiresAt, "state": st,
		"decided_at": a.DecidedAt, "decided_by": a.DecidedBy, "method": a.Method, "consumed_at": a.ConsumedAt,
		"result": a.Result, "outcome": a.Outcome}
}

type approvalFile struct {
	Schema    string     `json:"schema"`
	Approvals []Approval `json:"approvals"`
}

type Approvals struct {
	mu      sync.Mutex
	path    string
	list    []Approval
	loadErr string
	Now     func() time.Time
}

var (
	ErrApprovalsDamaged  = errors.New("approvals.json is damaged or unreadable; Admin-approved actions are blocked until it is repaired")
	ErrApprovalUnknown   = errors.New("no such approval")
	ErrApprovalState     = errors.New("that approval is not approved")
	ErrApprovalExpired   = errors.New("that approval has expired; ask again")
	ErrApprovalKind      = errors.New("that approval is for a different kind of action")
	ErrApprovalOwner     = errors.New("that approval was requested by a different program")
	ErrApprovalUsed      = errors.New("that approval was already used")
	ErrApprovalUncertain = errors.New("that approval was used but the outcome was not recorded (Identity stopped mid-action); request a new approval — it is never replayed automatically")
	ErrTooManyPending    = errors.New("too many approvals are waiting; approve, deny or let some expire first")
	ErrViewToken         = errors.New("that approval link is not valid (wrong, already used, or expired)")
)

func OpenApprovals(dir string) *Approvals {
	s := &Approvals{path: filepath.Join(dir, approvalsFile), Now: time.Now}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.loadErr = ErrApprovalsDamaged.Error()
	default:
		var f approvalFile
		if json.Unmarshal(b, &f) != nil || f.Schema != approvalSchema {
			s.loadErr = ErrApprovalsDamaged.Error()
		} else {
			s.list = f.Approvals
		}
	}
	return s
}

func (s *Approvals) LoadError() string { s.mu.Lock(); defer s.mu.Unlock(); return s.loadErr }

func (s *Approvals) save(list []Approval) error {
	if len(list) > keepApprovals {
		sort.SliceStable(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
		list = list[:keepApprovals]
	}
	return profile.WriteJSON(s.path, approvalFile{Schema: approvalSchema, Approvals: list})
}

func digestOf(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte("identity approval view token\n" + t))
	return hex.EncodeToString(h[:])
}

// Create records a PENDING approval for the exact action and returns it with
// a one-time view token for Identity's approval page.
func (s *Approvals) Create(kind string, action any, summary []string, requestedBy string) (Approval, string, error) {
	raw, err := json.Marshal(action)
	if err != nil {
		return Approval{}, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Approval{}, "", ErrApprovalsDamaged
	}
	now := s.Now()
	pending := 0
	for _, a := range s.list {
		if a.State == Pending && now.Before(a.ExpiresAt) {
			pending++
		}
	}
	if pending >= maxPending {
		return Approval{}, "", ErrTooManyPending
	}
	token := profile.RandHex(24)
	ap := Approval{ID: "apr_" + profile.RandHex(10), Kind: kind, Action: raw, Digest: digestOf(raw), Summary: summary,
		RequestedBy: requestedBy, CreatedAt: now.UTC(), ExpiresAt: now.Add(ApprovalTTL).UTC(), State: Pending, ViewToken: hashToken(token)}
	next := append(append([]Approval(nil), s.list...), ap)
	if err := s.save(next); err != nil {
		return Approval{}, "", err
	}
	s.list = next
	return ap, token, nil
}

func (s *Approvals) find(id string) int {
	for i, a := range s.list {
		if a.ID == id {
			return i
		}
	}
	return -1
}

func (s *Approvals) Get(id string) (Approval, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.find(id); i >= 0 {
		return s.list[i], true
	}
	return Approval{}, false
}

// OpenView exchanges the one-time view token for the approval (single use).
func (s *Approvals) OpenView(id, token string) (Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Approval{}, ErrApprovalsDamaged
	}
	i := s.find(id)
	if i < 0 || token == "" {
		return Approval{}, ErrViewToken
	}
	a := s.list[i]
	if a.ViewOpened || a.State != Pending || !s.Now().Before(a.ExpiresAt) ||
		subtle.ConstantTimeCompare([]byte(hashToken(token)), []byte(a.ViewToken)) != 1 {
		return Approval{}, ErrViewToken
	}
	next := append([]Approval(nil), s.list...)
	next[i].ViewOpened = true
	if err := s.save(next); err != nil {
		return Approval{}, err
	}
	s.list = next
	return next[i], nil
}

// Decide records the Admin's decision. The caller MUST have authenticated
// the Admin (AdminStore.Authenticate) for this exact approval first.
func (s *Approvals) Decide(id string, approve bool, adminID, method string, untilRevokedConfirmed ...bool) (Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Approval{}, ErrApprovalsDamaged
	}
	i := s.find(id)
	if i < 0 {
		return Approval{}, ErrApprovalUnknown
	}
	now := s.Now()
	if s.list[i].State != Pending {
		return Approval{}, ErrApprovalState
	}
	if !now.Before(s.list[i].ExpiresAt) {
		return Approval{}, ErrApprovalExpired
	}
	next := append([]Approval(nil), s.list...)
	t := now.UTC()
	next[i].DecidedAt, next[i].DecidedBy, next[i].Method = &t, adminID, method
	next[i].ConfirmedUntilRevoked = approve && len(untilRevokedConfirmed) > 0 && untilRevokedConfirmed[0]
	next[i].State = Denied
	if approve {
		next[i].State = Approved
	}
	if err := s.save(next); err != nil {
		return Approval{}, err
	}
	s.list = next
	return next[i], nil
}

// Begin claims an APPROVED approval for execution, exactly once. If the same
// requester asks again after a successful execution, it gets the recorded
// result back (fresh=false) so a retry is idempotent and never runs twice.
func (s *Approvals) Begin(id, kind, requester string) (Approval, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Approval{}, false, ErrApprovalsDamaged
	}
	i := s.find(id)
	if i < 0 {
		return Approval{}, false, ErrApprovalUnknown
	}
	a := s.list[i]
	if a.Kind != kind {
		return Approval{}, false, ErrApprovalKind
	}
	if a.RequestedBy != requester {
		return Approval{}, false, ErrApprovalOwner
	}
	if a.State == Consumed {
		if a.Result != "" {
			return a, false, nil
		}
		return Approval{}, false, ErrApprovalUncertain
	}
	if a.State != Approved {
		return Approval{}, false, ErrApprovalState
	}
	now := s.Now()
	if !now.Before(a.ExpiresAt) {
		return Approval{}, false, ErrApprovalExpired
	}
	if digestOf(a.Action) != a.Digest {
		return Approval{}, false, errors.New("approval record does not match its digest; refused")
	}
	next := append([]Approval(nil), s.list...)
	t := now.UTC()
	next[i].State, next[i].ConsumedAt = Consumed, &t
	if err := s.save(next); err != nil {
		return Approval{}, false, err
	}
	s.list = next
	return next[i], true, nil
}

// Finish records the outcome of a begun approval.
func (s *Approvals) Finish(id, result, outcome string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.find(id)
	if i < 0 {
		return ErrApprovalUnknown
	}
	next := append([]Approval(nil), s.list...)
	next[i].Result, next[i].Outcome = result, outcome
	if err := s.save(next); err != nil {
		return err
	}
	s.list = next
	return nil
}

// Recent lists approvals newest first (views only).
func (s *Approvals) Recent(n int) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	out := []map[string]any{}
	for i := len(s.list) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, s.list[i].View(now))
	}
	return out
}
