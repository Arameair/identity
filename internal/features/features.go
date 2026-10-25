// Package features holds the small publisher feature catalog and the local
// access-request drafts.
//
// There is NO publisher submission service and NO grant service yet. So:
//   - a draft is SAVED LOCALLY — NOT SUBMITTED, never "requested"
//   - grant state is always NOT GRANTED
//   - "available" means only that GitHub showed the verified account the
//     feature's repository; visibility is not an entitlement
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/profile"
)

type Feature struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
	InstallRule bool   `json:"install_rule"` // installable through Processor under an access rule
}

// Catalog is embedded in this experimental build. It is not a signed
// publisher catalog.
var Catalog = []Feature{
	{ID: "thought-processor", Name: "Thought Processor", Repo: "Arameair/Thought-Processor",
		Description: "Connection and diagnostic workspace: shows services, their truthful state, logs and flow."},
	{ID: "thought-engine", Name: "Thought Engine", Repo: "Arameair/Thought-engine", InstallRule: true,
		Description: "Spark → Trace → Thought processing. Thought Processor can install it if your Identity allows it."},
}

func Repos() []string {
	out := []string{}
	for _, f := range Catalog {
		out = append(out, f.Repo)
	}
	return out
}

func Find(id string) (Feature, bool) {
	for _, f := range Catalog {
		if f.ID == id {
			return f, true
		}
	}
	return Feature{}, false
}

// State labels, shown verbatim in the UI and diagnostics.
const (
	AvailVisible = "AVAILABLE — repository visible to your verified account"
	AvailHidden  = "UNAVAILABLE — GitHub does not show this repository to your account"
	AvailUnknown = "UNKNOWN — sign in to GitHub to check"
	AvailFailed  = "UNKNOWN — GitHub check failed"
	ReqNone      = "NOT REQUESTED"
	ReqDraft     = "DRAFT SAVED LOCALLY — NOT SUBMITTED"
	GrantNone    = "NOT GRANTED — no publisher grant service exists yet"
	// InstallAllowed applies only to features with an install rule (Thought
	// Engine, rule repo-visibility-v0 approved by the owner 2026-10-25).
	InstallAllowed = "INSTALL ALLOWED — rule repo-visibility-v0: your account can see the repository (interim rule, not a publisher grant)"
	InstallDenied  = "NOT ALLOWED — rule repo-visibility-v0: your account can't see the repository"
	DraftDelivery  = "NOT SUBMITTED — no publisher submission service exists; nothing was sent to anyone"
)

type Draft struct {
	ID          string    `json:"id"`
	FeatureID   string    `json:"feature_id"`
	GitHubLogin string    `json:"github_login"`
	GitHubID    int64     `json:"github_id"`
	Mode        string    `json:"github_mode"` // real | TEST FIXTURE
	SystemID    string    `json:"system_id"`
	CreatedAt   time.Time `json:"created_at"`
	Delivery    string    `json:"delivery"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	drafts  []Draft
	loadErr string // non-empty: file damaged, drafts disabled, file left untouched
}

func Open(root string) *Store {
	s := &Store{path: filepath.Join(root, "drafts.json"), drafts: []Draft{}}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.loadErr = "drafts file could not be read: " + err.Error()
	default:
		var d []Draft
		if err := json.Unmarshal(b, &d); err != nil {
			s.loadErr = "drafts.json is damaged; it was left untouched and drafts are disabled until it is fixed or moved"
		} else {
			for _, x := range d {
				if x.ID == "" || x.FeatureID == "" || x.GitHubID == 0 || x.Delivery != DraftDelivery {
					s.loadErr = "drafts.json contains an entry that is not a valid local draft; left untouched, drafts disabled"
					break
				}
			}
			if s.loadErr == "" {
				s.drafts = d
			}
		}
	}
	return s
}

func (s *Store) LoadError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadErr
}

// Save creates a draft for (feature, account). A second call for the same
// pair returns the existing draft (created=false) instead of duplicating it.
func (s *Store) Save(featureID string, acct *github.Account, mode, systemID string) (Draft, bool, error) {
	if _, ok := Find(featureID); !ok {
		return Draft{}, false, errors.New("unknown feature")
	}
	if acct == nil || acct.ID == 0 {
		return Draft{}, false, errors.New("connect and verify a GitHub account first: a draft records which verified account it is for")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Draft{}, false, errors.New(s.loadErr)
	}
	for _, d := range s.drafts {
		if d.FeatureID == featureID && d.GitHubID == acct.ID && d.Mode == mode {
			return d, false, nil
		}
	}
	d := Draft{ID: "draft_" + profile.RandHex(8), FeatureID: featureID, GitHubLogin: acct.Login, GitHubID: acct.ID, Mode: mode,
		SystemID: systemID, CreatedAt: time.Now().UTC(), Delivery: DraftDelivery}
	s.drafts = append(s.drafts, d)
	if err := profile.WriteJSON(s.path, s.drafts); err != nil {
		s.drafts = s.drafts[:len(s.drafts)-1]
		return Draft{}, false, fmt.Errorf("draft could not be saved: %w", err)
	}
	return d, true, nil
}

// Withdraw deletes the local draft for (feature, account).
func (s *Store) Withdraw(featureID string, githubID int64, mode string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return false, errors.New(s.loadErr)
	}
	keep := []Draft{}
	found := false
	for _, d := range s.drafts {
		if d.FeatureID == featureID && d.GitHubID == githubID && d.Mode == mode {
			found = true
			continue
		}
		keep = append(keep, d)
	}
	if !found {
		return false, nil
	}
	old := s.drafts
	s.drafts = keep
	if err := profile.WriteJSON(s.path, s.drafts); err != nil {
		s.drafts = old
		return false, err
	}
	return true, nil
}

func (s *Store) All() []Draft {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Draft(nil), s.drafts...)
}

func (s *Store) For(featureID string, githubID int64, mode string) *Draft {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.drafts {
		if d.FeatureID == featureID && d.GitHubID == githubID && d.Mode == mode {
			x := d
			return &x
		}
	}
	return nil
}

// Card is a feature with every state derived from real evidence.
type Card struct {
	Feature
	Availability string     `json:"availability"`
	AvailKind    string     `json:"avail_kind"` // available | unavailable | unknown
	Request      string     `json:"request"`
	RequestKind  string     `json:"request_kind"` // not_requested | draft
	Grant        string     `json:"grant"`
	NextStep     string     `json:"next_step,omitempty"`
	Draft        *Draft     `json:"draft,omitempty"`
	CheckedAt    *time.Time `json:"checked_at,omitempty"`
}

func (s *Store) Cards(gv github.View) []Card {
	out := []Card{}
	for _, f := range Catalog {
		c := Card{Feature: f, Availability: AvailUnknown, AvailKind: "unknown", Request: ReqNone, RequestKind: "not_requested", Grant: GrantNone}
		if gv.Status == github.Verified && gv.Account != nil {
			if rc, ok := gv.RepoVisible(f.Repo); ok {
				t := rc.CheckedAt
				c.CheckedAt = &t
				owner := strings.SplitN(f.Repo, "/", 2)[0]
				switch rc.Result {
				case "VISIBLE":
					c.Availability, c.AvailKind = AvailVisible, "available"
					if f.InstallRule {
						c.Grant = InstallAllowed
						c.NextStep = "Install it from Thought Processor (pair Processor with Identity first)."
					}
				case "NOT VISIBLE":
					c.Availability, c.AvailKind = AvailHidden, "unavailable"
					if f.InstallRule {
						c.Grant = InstallDenied
					}
					if strings.EqualFold(gv.Account.Login, owner) {
						c.NextStep = "You own this repository: install Identity's GitHub App on it (GitHub → the app → Install App → Configure → add " + strings.SplitN(f.Repo, "/", 2)[1] + "), then press Check again."
					} else {
						c.NextStep = "Ask " + owner + " to give " + gv.Account.Login + " access to " + f.Repo + "."
					}
				default:
					c.Availability = AvailFailed
					c.NextStep = "Press Check again on the GitHub step; if it repeats, Copy Diagnostics."
				}
			}
			if d := s.For(f.ID, gv.Account.ID, gv.Mode); d != nil {
				c.Draft, c.Request, c.RequestKind = d, ReqDraft, "draft"
			}
		}
		out = append(out, c)
	}
	return out
}
