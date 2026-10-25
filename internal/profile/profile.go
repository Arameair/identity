// Package profile owns Identity's private data root.
//
// Layout (all created 0700/0600):
//
//	<root>/profile.json      durable System ID — never regenerated once written
//	<root>/settings.json     setup progress, Processor address, last verified account (no secrets)
//	<root>/drafts.json       local access-request drafts (NOT SUBMITTED)
//	<root>/github-app.json   optional public GitHub App Client ID (not a secret)
//	<root>/logs/             persistent activity log
//	<root>/runtime/          lock + this run's private open link (deleted on exit)
//
// Nothing secret is ever written here: GitHub access tokens live in memory only.
package profile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const Schema = "identity.profile/1"

var idRE = regexp.MustCompile(`^idn_[0-9a-f]{24}$`)

type Profile struct {
	Schema    string    `json:"schema"`
	SystemID  string    `json:"system_id"`
	CreatedAt time.Time `json:"created_at"`
	Label     string    `json:"label"`
}

// VerifiedAccount is a non-secret record of the last GitHub account that
// GitHub itself confirmed. It is history, not a live sign-in.
type VerifiedAccount struct {
	Login      string    `json:"login"`
	ID         int64     `json:"id"`
	Name       string    `json:"name,omitempty"`
	ProfileURL string    `json:"profile_url,omitempty"`
	VerifiedAt time.Time `json:"verified_at"`
	Mode       string    `json:"mode"` // real | TEST FIXTURE
}

type Settings struct {
	SetupCompletedAt *time.Time       `json:"setup_completed_at,omitempty"`
	ProcessorAddress string           `json:"processor_address,omitempty"`
	LastAccount      *VerifiedAccount `json:"last_verified_account,omitempty"`
}

type Store struct {
	Root    string
	Profile Profile
	Created bool // true if this run created the profile

	mu       sync.Mutex
	settings Settings
	lock     *fileLock
}

func RandHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// ErrLocked means another Identity process already uses this profile.
var ErrLocked = errors.New("profile in use")

// Open creates or loads a profile and takes its exclusive lock.
// A damaged profile.json is never overwritten: Open fails and says so.
func Open(root string) (*Store, error) {
	root = filepath.Clean(root)
	for _, d := range []string{root, filepath.Join(root, "logs"), filepath.Join(root, "runtime")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("cannot create profile folder %s: %w", d, err)
		}
	}
	lk, err := acquireLock(filepath.Join(root, "runtime", "identity.lock"))
	if err != nil {
		return nil, fmt.Errorf("%w: another Identity is already running with profile %s (%v)", ErrLocked, root, err)
	}
	s := &Store{Root: root, lock: lk}
	fail := func(e error) (*Store, error) { lk.release(); return nil, e }

	p := filepath.Join(root, "profile.json")
	b, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.Profile = Profile{Schema: Schema, SystemID: "idn_" + RandHex(12), CreatedAt: time.Now().UTC(), Label: "Identity"}
		if err := writeJSON(p, s.Profile); err != nil {
			return fail(fmt.Errorf("cannot save new profile: %w", err))
		}
		s.Created = true
	case err != nil:
		return fail(fmt.Errorf("cannot read %s: %w", p, err))
	default:
		if err := json.Unmarshal(b, &s.Profile); err != nil || s.Profile.Schema != Schema || !idRE.MatchString(s.Profile.SystemID) {
			return fail(fmt.Errorf("profile file %s is damaged or from another program; it was left untouched. Use a different -data folder, or restore the file", p))
		}
	}

	sp := filepath.Join(root, "settings.json")
	if b, err := os.ReadFile(sp); err == nil {
		if err := json.Unmarshal(b, &s.settings); err != nil {
			return fail(fmt.Errorf("settings file %s is damaged; it was left untouched. Fix or move it, then start again", sp))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail(err)
	}
	return s, nil
}

func (s *Store) Close() {
	os.Remove(s.OpenLinkPath())
	if s.lock != nil {
		s.lock.release()
	}
}

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// UpdateSettings applies fn and saves; on save failure the change is rolled back.
func (s *Store) UpdateSettings(fn func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.settings
	if old.LastAccount != nil {
		cp := *old.LastAccount
		old.LastAccount = &cp
	}
	fn(&s.settings)
	if err := writeJSON(filepath.Join(s.Root, "settings.json"), s.settings); err != nil {
		s.settings = old
		return err
	}
	return nil
}

func (s *Store) OpenLinkPath() string { return filepath.Join(s.Root, "runtime", "open-link.txt") }

// SaveOpenLink stores this run's private link (0600) so the window can be
// reopened. It is deleted when Identity exits.
func (s *Store) SaveOpenLink(link string) error {
	return os.WriteFile(s.OpenLinkPath(), []byte(link+"\n"), 0o600)
}

// DisplayPath shortens the home directory to ~ so diagnostics don't reveal
// the Windows/Unix user name.
func DisplayPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return p
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// WriteJSON is exported for sibling packages that keep files in the profile.
func WriteJSON(path string, v any) error { return writeJSON(path, v) }
