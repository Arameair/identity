package authority

// Scoped grants and single-use request tickets.
//
// A grant says: this authenticated REQUESTER (a paired Processor, or a
// registered installation) may ask this DESTINATION installation to perform
// this one CAPABILITY for this SUBJECT identity, until it expires or is
// revoked. Source scope is always "none" in P1 (no information source is
// granted). Grants are created only from a consumed Admin approval.
//
// A ticket is minted for one request under a live grant. It is signed with a
// key only Identity and the destination installation know, bound to the
// request body digest, valid for TicketTTL and accepted once. The receiving
// feature verifies the ticket AND asks Identity, live, before any work; that
// live check consumes the ticket and re-checks the grant (so revocation is
// seen on the very next request, and no decision is cached).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/profile"
)

const (
	grantsFile   = "grants.json"
	grantsSchema = "identity.grants/1"
	TicketTTL    = 60 * time.Second
	maxTickets   = 2000
)

var grantIDRE = regexp.MustCompile(`^gnt_[0-9a-f]{20}$`)

func ValidGrantID(s string) bool { return grantIDRE.MatchString(s) }

// GrantRequest is the exact proposal the Admin approves.
type GrantRequest struct {
	Requester   string     `json:"requester"`
	Destination string     `json:"destination_installation_id"`
	Subject     string     `json:"subject_identity_id"`
	Capability  string     `json:"capability"`
	SourceScope string     `json:"source_scope"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RequestedBy string     `json:"requested_by"`
}

type Grant struct {
	ID           string     `json:"grant_id"`
	Version      int        `json:"version"`
	Requester    string     `json:"requester"`
	Destination  string     `json:"destination_installation_id"`
	Subject      string     `json:"subject_identity_id"`
	Capability   string     `json:"capability"`
	SourceScope  string     `json:"source_scope"`
	ApprovedBy   string     `json:"approved_by_admin_id"`
	ApprovalID   string     `json:"approval_id"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedBy    string     `json:"revoked_by,omitempty"`
	RevokeReason string     `json:"revoke_reason,omitempty"`
}

// Status: ACTIVE | EXPIRED | REVOKED.
func (g Grant) Status(now time.Time) string {
	switch {
	case g.RevokedAt != nil:
		return "REVOKED"
	case g.ExpiresAt != nil && !now.Before(*g.ExpiresAt):
		return "EXPIRED"
	}
	return "ACTIVE"
}

type grantFile struct {
	Schema string  `json:"schema"`
	Grants []Grant `json:"grants"`
}

type ticketRec struct {
	payload  TicketPayload
	consumed bool
}

type Grants struct {
	mu      sync.Mutex
	path    string
	list    []Grant
	loadErr string
	tickets map[string]*ticketRec // in memory only: a restart forgets them (fail closed)
	Now     func() time.Time
}

var (
	ErrGrantsDamaged = errors.New("grants.json is damaged or unreadable; every protected request is denied until it is repaired")
	ErrNoGrant       = errors.New("no such grant")
)

func OpenGrants(dir string) *Grants {
	s := &Grants{path: filepath.Join(dir, grantsFile), Now: time.Now, tickets: map[string]*ticketRec{}}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.loadErr = ErrGrantsDamaged.Error()
	default:
		var f grantFile
		if json.Unmarshal(b, &f) != nil || f.Schema != grantsSchema {
			s.loadErr = ErrGrantsDamaged.Error()
		} else {
			s.list = f.Grants
		}
	}
	return s
}

func (s *Grants) LoadError() string { s.mu.Lock(); defer s.mu.Unlock(); return s.loadErr }

func (s *Grants) commit(next []Grant) error {
	if err := profile.WriteJSON(s.path, grantFile{Schema: grantsSchema, Grants: next}); err != nil {
		return err
	}
	s.list = next
	return nil
}

func (s *Grants) Create(req GrantRequest, approvalID, adminID string) (Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Grant{}, ErrGrantsDamaged
	}
	g := Grant{ID: "gnt_" + profile.RandHex(10), Version: 1, Requester: req.Requester, Destination: req.Destination,
		Subject: req.Subject, Capability: req.Capability, SourceScope: "none", ApprovedBy: adminID, ApprovalID: approvalID,
		CreatedAt: s.Now().UTC(), ExpiresAt: req.ExpiresAt}
	if err := s.commit(append(append([]Grant(nil), s.list...), g)); err != nil {
		return Grant{}, err
	}
	return g, nil
}

func (s *Grants) Get(id string) (Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Grant{}, ErrGrantsDamaged
	}
	for _, g := range s.list {
		if g.ID == id {
			return g, nil
		}
	}
	return Grant{}, ErrNoGrant
}

// Revoke ends a grant. Revoking narrows access; it is persisted before it is
// reported, and all outstanding tickets for it die with it.
func (s *Grants) Revoke(id, by, reason string) (Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Grant{}, ErrGrantsDamaged
	}
	next := append([]Grant(nil), s.list...)
	for i := range next {
		if next[i].ID != id {
			continue
		}
		if next[i].RevokedAt != nil {
			return next[i], nil
		}
		t := s.Now().UTC()
		next[i].RevokedAt, next[i].RevokedBy, next[i].RevokeReason = &t, by, clean(reason, 120)
		next[i].Version++
		if err := s.commit(next); err != nil {
			return Grant{}, err
		}
		for k, tr := range s.tickets {
			if tr.payload.GrantID == id {
				delete(s.tickets, k)
			}
		}
		return next[i], nil
	}
	return Grant{}, ErrNoGrant
}

// RevokeAllFor revokes every grant naming installation id (as destination or requester).
func (s *Grants) RevokeAllFor(installationID, by, reason string) (int, error) {
	s.mu.Lock()
	ids := []string{}
	for _, g := range s.list {
		if g.RevokedAt == nil && (g.Destination == installationID || g.Requester == "installation:"+installationID) {
			ids = append(ids, g.ID)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		if _, err := s.Revoke(id, by, reason); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func (s *Grants) List() []Grant {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Grant(nil), s.list...)
}

// ---- tickets ----

type TicketPayload struct {
	V          int    `json:"v"`
	TicketID   string `json:"tid"`
	GrantID    string `json:"gid"`
	GrantVer   int    `json:"gv"`
	Issuer     string `json:"iss"`
	Requester  string `json:"req"`
	Audience   string `json:"aud"`
	Capability string `json:"cap"`
	Subject    string `json:"sub"`
	Digest     string `json:"dig"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
}

// Deny reasons (machine-readable, shown in UI and tests).
const (
	DenyNoGrant        = "NO_GRANT"
	DenyRevoked        = "GRANT_REVOKED"
	DenyExpired        = "GRANT_EXPIRED"
	DenyTicketUnknown  = "TICKET_UNKNOWN"
	DenyTicketReplayed = "TICKET_ALREADY_USED"
	DenyTicketExpired  = "TICKET_EXPIRED"
	DenyMismatch       = "TICKET_MISMATCH"
	DenyWrongAudience  = "WRONG_RECIPIENT"
	DenyDestination    = "DESTINATION_NOT_INSTALLED"
	DenySubject        = "SUBJECT_UNAVAILABLE"
)

// FindActive returns the active grant for exactly this scope.
func (s *Grants) FindActive(requester, destination, subject, capability string) (Grant, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	reason := DenyNoGrant
	var latest time.Time
	for _, g := range s.list {
		if g.Requester != requester || g.Destination != destination || g.Subject != subject || g.Capability != capability {
			continue
		}
		st := g.Status(now)
		if st == "ACTIVE" {
			return g, ""
		}
		// Report why the most recent matching grant no longer applies.
		if !g.CreatedAt.Before(latest) {
			latest = g.CreatedAt
			reason = map[string]string{"REVOKED": DenyRevoked, "EXPIRED": DenyExpired}[st]
		}
	}
	return Grant{}, reason
}

func ticketMAC(key []byte, b64 string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("identity/2 ticket\n" + b64))
	return hex.EncodeToString(m.Sum(nil))
}

// Issue mints a single-use ticket under grant g for one request digest.
func (s *Grants) Issue(g Grant, issuer, digest string, key []byte) (string, TicketPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	for k, t := range s.tickets {
		if now.Unix() > t.payload.ExpiresAt+60 {
			delete(s.tickets, k)
		}
	}
	if len(s.tickets) >= maxTickets {
		return "", TicketPayload{}, errors.New("too many outstanding tickets; try again shortly")
	}
	p := TicketPayload{V: 1, TicketID: "tkt_" + profile.RandHex(12), GrantID: g.ID, GrantVer: g.Version, Issuer: issuer,
		Requester: g.Requester, Audience: g.Destination, Capability: g.Capability, Subject: g.Subject, Digest: digest,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(TicketTTL).Unix()}
	raw, _ := json.Marshal(p)
	b64 := base64.RawURLEncoding.EncodeToString(raw)
	s.tickets[p.TicketID] = &ticketRec{payload: p}
	return "tk1." + b64 + "." + ticketMAC(key, b64), p, nil
}

// CheckRequest is what the receiving installation reports about the request
// it is holding. Every field must match the ticket Identity issued.
type CheckRequest struct {
	TicketID   string `json:"ticket_id"`
	GrantID    string `json:"grant_id"`
	Requester  string `json:"requester"`
	Capability string `json:"capability"`
	Subject    string `json:"subject_identity_id"`
	Digest     string `json:"request_sha256"`
}

// Check is the live authorization decision. caller is the AUTHENTICATED
// installation asking. On ALLOW the ticket is consumed.
func (s *Grants) Check(caller string, c CheckRequest) (Grant, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Grant{}, "GRANTS_UNAVAILABLE"
	}
	now := s.Now()
	t, ok := s.tickets[c.TicketID]
	if !ok {
		return Grant{}, DenyTicketUnknown
	}
	p := t.payload
	if p.Audience != caller {
		return Grant{}, DenyWrongAudience
	}
	if t.consumed {
		return Grant{}, DenyTicketReplayed
	}
	if now.Unix() >= p.ExpiresAt {
		return Grant{}, DenyTicketExpired
	}
	if p.GrantID != c.GrantID || p.Requester != c.Requester || p.Capability != c.Capability || p.Subject != c.Subject || p.Digest != c.Digest {
		return Grant{}, DenyMismatch
	}
	var g *Grant
	for i := range s.list {
		if s.list[i].ID == p.GrantID {
			g = &s.list[i]
		}
	}
	if g == nil {
		return Grant{}, DenyNoGrant
	}
	switch g.Status(now) {
	case "REVOKED":
		return *g, DenyRevoked
	case "EXPIRED":
		return *g, DenyExpired
	}
	if g.Version != p.GrantVer || g.Destination != caller || g.Capability != c.Capability || g.Subject != c.Subject || g.Requester != c.Requester {
		return *g, DenyMismatch
	}
	t.consumed = true
	return *g, ""
}

// ParseTicket splits a ticket (used by tests and diagnostics; the receiving
// feature does its own verification with its own copy of the key).
func ParseTicket(tk string, key []byte) (TicketPayload, error) {
	parts := strings.Split(tk, ".")
	if len(parts) != 3 || parts[0] != "tk1" {
		return TicketPayload{}, errors.New("not a ticket")
	}
	if !hmac.Equal([]byte(ticketMAC(key, parts[1])), []byte(parts[2])) {
		return TicketPayload{}, errors.New("ticket signature wrong")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TicketPayload{}, err
	}
	var p TicketPayload
	err = json.Unmarshal(raw, &p)
	return p, err
}
