// Package entities keeps the identities this Identity installation is the
// authority for: a person, a companion, a business or a project.
//
// An identity here is a durable record with an immutable ID. It is NOT a
// running service and it carries NO access: every identity starts with zero
// authorized sources and zero enabled features (owner decision C03,
// 2026-10-26). Grants are not implemented in this alpha, so those lists stay
// empty and every authorization check answers DENIED.
//
// Records are kept in <profile>/identities.json (0600). Archiving hides an
// identity from the normal list; nothing is ever hard-deleted by this package.
package entities

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Arameair/identity/internal/profile"
)

const Schema = "identity.entities/1"

// Kinds an identity may have. "system" is reserved for service identities
// and cannot be created through the API.
var Kinds = []string{"person", "companion", "business", "project"}

var idRE = regexp.MustCompile(`^ent_[0-9a-f]{20}$`)

// ValidID reports whether s looks like an identity ID.
func ValidID(s string) bool { return idRE.MatchString(s) }

type Entity struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by"` // e.g. "processor:tp_abc" — who asked
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	// Always empty in this alpha: no grant service exists. Present so callers
	// see the default-deny state explicitly rather than infer it.
	EnabledFeatures   []string `json:"enabled_features"`
	AuthorizedSources []string `json:"authorized_sources"`
}

type file struct {
	Schema     string   `json:"schema"`
	Identities []Entity `json:"identities"`
}

type Store struct {
	mu      sync.Mutex
	path    string
	list    []Entity
	loadErr string
	now     func() time.Time
}

var (
	ErrDamaged  = errors.New("identities.json is damaged; it was left untouched and identity changes are disabled until it is fixed or moved")
	ErrNotFound = errors.New("no identity with that ID")
	ErrName     = errors.New("name must be 1–60 characters, without control characters")
	ErrKind     = errors.New("kind must be one of: person, companion, business, project")
	ErrDupName  = errors.New("an active identity already has that name")
	ErrArchived = errors.New("that identity is archived; restore it first")
)

func Open(root string) *Store {
	s := &Store{path: filepath.Join(root, "identities.json"), now: time.Now}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.loadErr = "identities.json could not be read: " + err.Error()
	default:
		var f file
		if json.Unmarshal(b, &f) != nil || f.Schema != Schema {
			s.loadErr = ErrDamaged.Error()
			return s
		}
		for _, e := range f.Identities {
			if !idRE.MatchString(e.ID) {
				s.loadErr = ErrDamaged.Error()
				return s
			}
		}
		s.list = f.Identities
	}
	return s
}

func (s *Store) LoadError() string { s.mu.Lock(); defer s.mu.Unlock(); return s.loadErr }

func cleanName(n string) (string, error) {
	n = strings.Join(strings.Fields(n), " ")
	if n == "" || utf8.RuneCountInString(n) > 60 {
		return "", ErrName
	}
	for _, r := range n {
		if r < 0x20 || r == 0x7f {
			return "", ErrName
		}
	}
	return n, nil
}

func validKind(k string) bool {
	for _, x := range Kinds {
		if k == x {
			return true
		}
	}
	return false
}

func normalized(e Entity) Entity {
	if e.EnabledFeatures == nil {
		e.EnabledFeatures = []string{}
	}
	if e.AuthorizedSources == nil {
		e.AuthorizedSources = []string{}
	}
	return e
}

func (s *Store) saveLocked(list []Entity) error {
	if err := profile.WriteJSON(s.path, file{Schema: Schema, Identities: list}); err != nil {
		return errors.New("identities could not be saved: " + err.Error())
	}
	s.list = list
	return nil
}

func (s *Store) copyList() []Entity {
	out := make([]Entity, len(s.list))
	copy(out, s.list)
	return out
}

func (s *Store) nameTakenLocked(name, exceptID string) bool {
	for _, e := range s.list {
		if e.ArchivedAt == nil && e.ID != exceptID && strings.EqualFold(e.Name, name) {
			return true
		}
	}
	return false
}

// Create adds a new identity with no access.
func (s *Store) Create(name, kind, createdBy string) (Entity, error) {
	name, err := cleanName(name)
	if err != nil {
		return Entity{}, err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !validKind(kind) {
		return Entity{}, ErrKind
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Entity{}, errors.New(s.loadErr)
	}
	if s.nameTakenLocked(name, "") {
		return Entity{}, ErrDupName
	}
	now := s.now().UTC()
	e := normalized(Entity{ID: "ent_" + profile.RandHex(10), Name: name, Kind: kind, CreatedAt: now, UpdatedAt: now, CreatedBy: createdBy})
	if err := s.saveLocked(append(s.copyList(), e)); err != nil {
		return Entity{}, err
	}
	return e, nil
}

// List returns identities, newest last. Archived ones only if asked.
func (s *Store) List(includeArchived bool) []Entity {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Entity{}
	for _, e := range s.list {
		if e.ArchivedAt == nil || includeArchived {
			out = append(out, normalized(e))
		}
	}
	return out
}

func (s *Store) Get(id string) (Entity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.list {
		if e.ID == id {
			return normalized(e), true
		}
	}
	return Entity{}, false
}

func (s *Store) update(id string, fn func(*Entity) error) (Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Entity{}, errors.New(s.loadErr)
	}
	list := s.copyList()
	for i := range list {
		if list[i].ID == id {
			if err := fn(&list[i]); err != nil {
				return Entity{}, err
			}
			list[i].UpdatedAt = s.now().UTC()
			if err := s.saveLocked(list); err != nil {
				return Entity{}, err
			}
			return normalized(list[i]), nil
		}
	}
	return Entity{}, ErrNotFound
}

func (s *Store) Rename(id, name string) (Entity, error) {
	name, err := cleanName(name)
	if err != nil {
		return Entity{}, err
	}
	return s.update(id, func(e *Entity) error {
		if e.ArchivedAt != nil {
			return ErrArchived
		}
		if s.nameTakenLocked(name, id) {
			return ErrDupName
		}
		e.Name = name
		return nil
	})
}

// Archive hides an identity. It is kept on disk and can be restored.
func (s *Store) Archive(id string) (Entity, error) {
	return s.update(id, func(e *Entity) error {
		if e.ArchivedAt == nil {
			t := s.now().UTC()
			e.ArchivedAt = &t
		}
		return nil
	})
}

func (s *Store) Restore(id string) (Entity, error) {
	return s.update(id, func(e *Entity) error {
		if e.ArchivedAt != nil && s.nameTakenLocked(e.Name, id) {
			return ErrDupName
		}
		e.ArchivedAt = nil
		return nil
	})
}

// Counts returns active and archived totals (for status and diagnostics).
func (s *Store) Counts() (active, archived int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.list {
		if e.ArchivedAt == nil {
			active++
		} else {
			archived++
		}
	}
	return
}
