package entity

// Protected GitHub mailbox publishing (Arameair/TASS #24, Model B: Thought-Processor #134 D2-a).
//
// Two bound, single-use actions on the PR #47 pattern. Neither lets Core write to GitHub: the
// Communication Bridge (service "github-publisher") holds the only write credential and redeems
// the decision immediately before it posts, once.
//
//	github.mailbox.reply  Core may obtain it for ONE exact reply to ONE accepted mailbox comment,
//	                      while the owner grant of the same name is held.
//	github.mailbox.share  Core can NEVER obtain it. Only the owner, on Identity's own page and with
//	                      the passphrase, approves the exact bytes of one share; Identity then mints
//	                      the decision itself from the text it showed (ApproveShare).
//
// Binding: lowercase hex SHA-256 of these seven lines joined with "\n" (no trailing newline),
// computed independently by Core, Identity (shares) and the Bridge:
//
//	tass.github.mailbox/1
//	github-publisher
//	<action>
//	<repo, e.g. Arameair/TASS>
//	<issue number>
//	<source id: the accepted comment ID for a reply, the share ID for a share>
//	<lowercase hex SHA-256 of the exact payload bytes>

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	GitHubMailboxReply = "github.mailbox.reply"
	GitHubMailboxShare = "github.mailbox.share"
	PublisherService   = "github-publisher"
	MaxShareText       = 1000
	maxShareRequests   = 20
)

func init() {
	BoundActions[GitHubMailboxReply] = true  // owner grant github.mailbox.reply required
	BoundActions[GitHubMailboxShare] = false // UNLOCKED only: the per-share owner approval is the authority
	BoundRedeemers[GitHubMailboxReply] = PublisherService
	BoundRedeemers[GitHubMailboxShare] = PublisherService
	SinglePointOfUse[GitHubMailboxReply] = true
	SinglePointOfUse[GitHubMailboxShare] = true
	OwnerMintOnly[GitHubMailboxShare] = true
	KnownSkills[GitHubMailboxReply] = "let the entity REPLY in the team mailbox (Arameair/TASS #24) to accepted team comments (each reply uses its own single-use approval, redeemed by the Communication Bridge just before it posts; TASS itself holds no GitHub write credential; owner shares always need your approval of the exact text)"
	ownerGrants[GitHubMailboxReply] = GitHubMailboxReply
}

// OwnerMintOnly: bound actions no service may obtain; only an owner-authenticated path mints them.
var OwnerMintOnly = map[string]bool{}

// GitHubMailboxBinding is the shared binding formula (see the package comment above).
func GitHubMailboxBinding(action, repo string, issue int, sourceID, payload string) string {
	ph := sha256.Sum256([]byte(payload))
	lines := strings.Join([]string{"tass.github.mailbox/1", PublisherService, action, repo,
		strconv.Itoa(issue), sourceID, hex.EncodeToString(ph[:])}, "\n")
	h := sha256.Sum256([]byte(lines))
	return hex.EncodeToString(h[:])
}

var shareIDRe = regexp.MustCompile(`^share_[0-9]{8}T[0-9]{6}_[0-9a-f]{8}$`)
var repoRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,39}/[A-Za-z0-9._-]{1,100}$`)

// ShareRequest is one share Core asked the owner to approve. The text is held in memory only
// (an Identity restart forgets requests, like bound decisions) and shown to the owner verbatim.
type ShareRequest struct {
	ShareID    string    `json:"share_id"`
	Repo       string    `json:"repo"`
	Issue      int       `json:"issue"`
	Text       string    `json:"text,omitempty"` // owner page only; cleared in service responses
	TextSHA256 string    `json:"text_sha256"`
	EntityID   string    `json:"entity_id"`
	At         time.Time `json:"at"`
	State      string    `json:"state"` // PENDING | APPROVED | REJECTED
	DecisionID string    `json:"decision_id,omitempty"`
	Binding    string    `json:"binding,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

type shareTable struct {
	mu sync.Mutex
	m  map[string]*ShareRequest
}

func (s *Store) shares() *shareTable {
	s.shareOnce.Do(func() { s.shareTab = &shareTable{m: map[string]*ShareRequest{}} })
	return s.shareTab
}

var (
	ErrShareInvalid = errors.New("share request is not valid")
	ErrShareUnknown = errors.New("no such share request (unknown, or Identity restarted)")
	ErrShareState   = errors.New("share request is not pending")
)

// RequestShare records a share Core asks the owner to approve. Requires UNLOCKED and the same entity.
func (s *Store) RequestShare(service, shareID, repo string, issue int, text, entityID string) (ShareRequest, error) {
	if service != MintService || !shareIDRe.MatchString(shareID) || !repoRe.MatchString(repo) || issue <= 0 ||
		strings.TrimSpace(text) == "" || len(text) > MaxShareText || strings.Contains(text, "<!--") {
		return ShareRequest{}, ErrShareInvalid
	}
	if ok, why, ent := s.allowNow(GitHubMailboxShare, entityID); !ok || ent == "" {
		return ShareRequest{}, errors.New(why)
	}
	t := s.shares()
	t.mu.Lock()
	defer t.mu.Unlock()
	if r, dup := t.m[shareID]; dup {
		return *r, nil
	}
	pending := 0
	for _, r := range t.m {
		if r.State == "PENDING" {
			pending++
		}
	}
	if pending >= maxShareRequests {
		return ShareRequest{}, errors.New("too many share requests waiting for the owner")
	}
	h := sha256.Sum256([]byte(text))
	r := &ShareRequest{ShareID: shareID, Repo: repo, Issue: issue, Text: text, TextSHA256: hex.EncodeToString(h[:]),
		EntityID: entityID, At: s.Now().UTC(), State: "PENDING"}
	t.m[shareID] = r
	s.auditBound(BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: r.At, Service: service,
		Action: "github.mailbox.share.request", Detail: shareID, EntityID: entityID, Allow: true,
		Reason: "waiting for the owner's approval of the exact text"}})
	return *r, nil
}

// ShareRequests lists the requests for the owner page (newest last).
func (s *Store) ShareRequests() []ShareRequest {
	t := s.shares()
	t.mu.Lock()
	defer t.mu.Unlock()
	out := []ShareRequest{}
	for _, r := range t.m {
		out = append(out, *r)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].At.Before(out[j-1].At); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ShareRequestFor is what Core may read about its own request (never another entity's).
func (s *Store) ShareRequestFor(service, shareID, entityID string) (ShareRequest, error) {
	if service != MintService {
		return ShareRequest{}, ErrShareInvalid
	}
	t := s.shares()
	t.mu.Lock()
	defer t.mu.Unlock()
	r := t.m[shareID]
	if r == nil || r.EntityID != entityID {
		return ShareRequest{}, ErrShareUnknown
	}
	return *r, nil
}

// ApproveShare: the owner approves (or rejects) the exact text of one pending share. Approval
// re-checks the passphrase (it lets something be published as the owner's words), then Identity
// mints the github.mailbox.share decision from the text it holds: Core never supplies the binding.
// textSHA256 must match what the page showed, so a request swapped after display is refused.
func (s *Store) ApproveShare(passphrase, shareID, textSHA256 string, approve bool) (ShareRequest, error) {
	t := s.shares()
	t.mu.Lock()
	r := t.m[shareID]
	if r == nil {
		t.mu.Unlock()
		return ShareRequest{}, ErrShareUnknown
	}
	if r.State != "PENDING" {
		t.mu.Unlock()
		return *r, ErrShareState
	}
	if r.TextSHA256 != textSHA256 {
		t.mu.Unlock()
		return *r, errors.New("the text you approved is not the text of this request")
	}
	snapshot := *r
	t.mu.Unlock()

	if approve {
		if err := s.checkOwnerPassphrase(passphrase); err != nil {
			return snapshot, err
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if r.State != "PENDING" || r.TextSHA256 != textSHA256 {
		return *r, ErrShareState
	}
	if !approve {
		r.State = "REJECTED"
		s.auditBound(BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: "owner",
			Action: "github.mailbox.share.reject", Detail: shareID, EntityID: r.EntityID, Reason: "owner rejected"}})
		return *r, nil
	}
	binding := GitHubMailboxBinding(GitHubMailboxShare, r.Repo, r.Issue, r.ShareID, r.Text)
	d := s.mintOwnerBound(GitHubMailboxShare, binding, shareID, r.EntityID)
	if !d.Allow {
		return *r, errors.New(d.Reason)
	}
	r.State, r.DecisionID, r.Binding, r.ExpiresAt = "APPROVED", d.ID, binding, d.ExpiresAt
	return *r, nil
}

// checkOwnerPassphrase: same throttle and verifier as enabling a grant.
func (s *Store) checkOwnerPassphrase(passphrase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.load()
	if err != nil {
		return ErrNotUsable
	}
	if e == nil {
		return ErrNoEntity
	}
	now := s.Now()
	if !now.Before(s.until) {
		return ErrLocked
	}
	wait := time.Duration(0)
	if s.fails >= 3 {
		wait = time.Duration(1<<min(s.fails-3, 8)) * time.Second
	}
	if now.Before(s.failedAt.Add(wait)) {
		return ErrThrottled
	}
	if !e.Verifier.Check(passphrase) {
		s.fails++
		s.failedAt = now
		return ErrBadPass
	}
	s.fails = 0
	return nil
}

// mintOwnerBound mints an OwnerMintOnly action after the owner was authenticated by the caller.
func (s *Store) mintOwnerBound(action, binding, detail, entityID string) BoundDecision {
	d := BoundDecision{Decision: Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: "owner", Action: action,
		Detail: trim(detail, 200)}, Binding: binding, Mode: "mint"}
	ok, why, ent := s.allowNow(action, entityID)
	d.EntityID, d.Reason = ent, why
	if ok {
		t := s.bt()
		t.mu.Lock()
		if len(t.m) >= maxBound {
			d.Reason = "too many pending bound decisions"
		} else {
			d.Allow, d.ExpiresAt = true, s.Now().UTC().Add(BoundTTL)
			t.m[d.ID] = &bound{d: d.Decision, binding: binding, expiresAt: d.ExpiresAt}
			t.m[d.ID].d.Allow = true
		}
		t.mu.Unlock()
	}
	s.auditBound(d)
	return d
}
