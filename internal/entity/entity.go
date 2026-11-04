// Package entity is Identity's local entity store for a single installation: create my own entity,
// unlock it with the owner passphrase, and answer "may this service do this action now?".
//
// What makes an entity:
//   - entity.json: a random entity ID, the name, when it was created, its granted actions, and the
//     passphrase VERIFIER (PBKDF2-HMAC-SHA256 from internal/authority; the passphrase is never stored);
//   - credential: a random 32-byte enrollment credential created with the entity on this computer.
//     entity.json records only its SHA-256. Unlocking needs BOTH the passphrase and this credential,
//     so knowing a passphrase is not enough to become an entity on another computer.
//
// Rules:
//   - Create works only when no entity exists here. An existing entity is never re-minted or replaced.
//   - entity.json damaged, credential missing or not matching → LOCKED with the reason, nothing allowed.
//   - Unlocking lasts until Lock, the time limit, or a restart (it is kept only in memory).
//   - Every authorization decision is appended to decisions.jsonl.
//
// Threat model (honest): the files are protected by OS file permissions (0600, owner-only folder).
// A process running as the same OS user, or root, can read them and try offline passphrase guesses;
// PBKDF2 slows that down, it doesn't prevent it. This is not encryption at rest.
package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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

	"github.com/Arameair/identity/internal/authority"
)

const Schema = "tass-identity.entity/1"

// States.
const (
	NoEntity = "NO_ENTITY"
	Locked   = "LOCKED"
	Unlocked = "UNLOCKED"
)

// DefaultGrants: what a new entity may do once unlocked. No code execution.
//   github.sync   sync the entity's own document and read lessons, ONLY on the repository its owner
//                 configured for this installation (nothing is configured by default)
//   lesson.learn  study a lesson; it counts as learned only after the owner approves it
var DefaultGrants = []string{"conversation", "model.request", "thought.process", "pulse.run", "action.read_entity",
	"github.sync", "lesson.learn"}

var (
	ErrExists      = errors.New("an entity already exists on this installation; it is never replaced or re-created")
	ErrNoEntity    = errors.New("no entity here yet: create one first")
	ErrBadPass     = errors.New("wrong passphrase")
	ErrThrottled   = errors.New("too many wrong passphrases: wait a little and try again")
	ErrNotUsable   = errors.New("this entity can't be unlocked")
	nameRe         = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._'-]{0,39}$`)
	errDamagedBase = "entity is LOCKED: "
)

type Entity struct {
	Schema         string             `json:"schema"`
	EntityID       string             `json:"entity_id"`
	Name           string             `json:"name"`
	CreatedAt      time.Time          `json:"created_at"`
	Grants         []string           `json:"grants"`
	Verifier       authority.Verifier `json:"verifier"`
	CredentialHash string             `json:"credential_sha256"`
}

// Public is what anyone may see (no verifier, no credential).
type Public struct {
	EntityID  string    `json:"entity_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Grants    []string  `json:"grants"`
}

type State struct {
	State         string     `json:"state"`
	Reason        string     `json:"reason,omitempty"`
	Entity        *Public    `json:"entity,omitempty"`
	UnlockedUntil *time.Time `json:"unlocked_until,omitempty"`
}

type Decision struct {
	ID       string    `json:"decision_id"`
	At       time.Time `json:"at"`
	Service  string    `json:"service"`
	Action   string    `json:"action"`
	Detail   string    `json:"detail,omitempty"`
	Allow    bool      `json:"allow"`
	Reason   string    `json:"reason"`
	EntityID string    `json:"entity_id,omitempty"`
}

type Store struct {
	dir       string
	unlockFor time.Duration
	Now       func() time.Time

	mu       sync.Mutex
	until    time.Time // unlocked until (zero = locked)
	fails    int
	failedAt time.Time
}

func Open(dir string, unlockFor time.Duration) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0o700)
	return &Store{dir: dir, unlockFor: unlockFor, Now: time.Now}, nil
}

func (s *Store) path(n string) string { return filepath.Join(s.dir, n) }

// load returns the entity, or (nil, nil) when none exists, or an error saying why it is unusable.
func (s *Store) load() (*Entity, error) {
	b, err := os.ReadFile(s.path("entity.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entity.json can't be read: %v", err)
	}
	var e Entity
	if err := json.Unmarshal(b, &e); err != nil || e.Schema != Schema || e.EntityID == "" || e.CredentialHash == "" {
		return &e, errors.New("entity.json is damaged (left untouched)")
	}
	cred, err := os.ReadFile(s.path("credential"))
	if err != nil {
		return &e, errors.New("the enrollment credential is missing: restore it from a backup of this installation (a new entity is never created in its place)")
	}
	h := sha256.Sum256([]byte(strings.TrimSpace(string(cred))))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(e.CredentialHash)) != 1 {
		return &e, errors.New("the enrollment credential doesn't belong to this entity")
	}
	return &e, nil
}

func (e *Entity) public() *Public {
	return &Public{EntityID: e.EntityID, Name: e.Name, CreatedAt: e.CreatedAt, Grants: append([]string(nil), e.Grants...)}
}

func (s *Store) State() State {
	e, err := s.load()
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case e == nil && err == nil:
		return State{State: NoEntity, Reason: "no entity on this installation yet"}
	case err != nil:
		st := State{State: Locked, Reason: errDamagedBase + err.Error()}
		if e != nil && e.EntityID != "" {
			st.Entity = &Public{EntityID: e.EntityID, Name: e.Name}
		}
		return st
	}
	now := s.Now()
	if now.Before(s.until) {
		u := s.until
		return State{State: Unlocked, Entity: e.public(), UnlockedUntil: &u}
	}
	return State{State: Locked, Reason: "locked: the owner unlocks it with the passphrase", Entity: e.public()}
}

func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, path)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Create makes this installation's entity. Refused if any entity (even a damaged one) exists.
func (s *Store) Create(name, passphrase string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path("entity.json")); err == nil {
		return State{}, ErrExists
	}
	name = strings.TrimSpace(name)
	if !nameRe.MatchString(name) {
		return State{}, errors.New("name: 1–40 letters, digits, spaces, . _ ' -")
	}
	v, err := authority.NewVerifier(passphrase)
	if err != nil {
		return State{}, err
	}
	cred := randHex(32)
	h := sha256.Sum256([]byte(cred))
	e := Entity{Schema: Schema, EntityID: "ent_" + randHex(12), Name: name, CreatedAt: s.Now().UTC(),
		Grants: append([]string(nil), DefaultGrants...), Verifier: v, CredentialHash: hex.EncodeToString(h[:])}
	b, _ := json.MarshalIndent(e, "", "  ")
	if err := writeFile(s.path("credential"), []byte(cred+"\n")); err != nil {
		return State{}, err
	}
	// O_EXCL on the final name: two concurrent creates can't both succeed.
	f, err := os.OpenFile(s.path("entity.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return State{}, ErrExists
		}
		return State{}, err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return State{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return State{}, err
	}
	f.Close()
	s.until = s.Now().Add(s.unlockFor)
	u := s.until
	return State{State: Unlocked, Entity: e.public(), UnlockedUntil: &u}, nil
}

// Unlock checks the passphrase against the verifier (and needs the matching credential).
func (s *Store) Unlock(passphrase string) (State, error) {
	e, err := s.load()
	if e == nil && err == nil {
		return State{}, ErrNoEntity
	}
	if err != nil {
		return State{}, fmt.Errorf("%w: %v", ErrNotUsable, err)
	}
	s.mu.Lock()
	wait := time.Duration(0)
	if s.fails >= 3 {
		wait = time.Duration(1<<min(s.fails-3, 8)) * time.Second
	}
	if s.Now().Before(s.failedAt.Add(wait)) {
		s.mu.Unlock()
		return State{}, ErrThrottled
	}
	s.mu.Unlock()
	ok := e.Verifier.Check(passphrase)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !ok {
		s.fails++
		s.failedAt = s.Now()
		return State{}, ErrBadPass
	}
	s.fails = 0
	s.until = s.Now().Add(s.unlockFor)
	u := s.until
	return State{State: Unlocked, Entity: e.public(), UnlockedUntil: &u}, nil
}

func (s *Store) Lock() {
	s.mu.Lock()
	s.until = time.Time{}
	s.mu.Unlock()
}

// Authorize answers whether service may do action for the entity now, and records the decision.
func (s *Store) Authorize(service, action, detail, entityID string) Decision {
	d := Decision{ID: "dec_" + randHex(8), At: s.Now().UTC(), Service: service, Action: action, Detail: trim(detail, 200)}
	st := s.State()
	switch {
	case st.State != Unlocked:
		d.Reason = st.State + ": " + st.Reason
	case entityID != "" && entityID != st.Entity.EntityID:
		d.Reason = "that entity isn't this installation's entity"
	case !contains(st.Entity.Grants, action):
		d.Reason = "the entity has no grant for " + action
	default:
		d.Allow, d.Reason = true, "unlocked; "+action+" is granted"
	}
	if st.Entity != nil {
		d.EntityID = st.Entity.EntityID
	}
	if b, err := json.Marshal(d); err == nil {
		if f, err := os.OpenFile(s.path("decisions.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			f.Write(append(b, '\n'))
			f.Close()
		}
	}
	return d
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
