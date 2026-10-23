package main

// The store keeps each identity in its own small JSON file:
//
//	<data>/identities/<id>.json
//
// One file per identity means a damaged file can only affect that one
// record, and creating a new identity never rewrites an existing file.
// Every write goes to a temporary file first, is flushed to disk, then
// renamed into place, so a failed save leaves earlier records untouched.
import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const schemaVersion = 1

// Identity is the whole Gate 1 record. Nothing secret belongs here.
type Identity struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	DisplayName   string    `json:"display_name"`
	EntityType    string    `json:"entity_type"`
	CreatedAt     time.Time `json:"created_at"`
}

// EntityTypes are the supported kinds, in display order.
var EntityTypes = []struct {
	Key, Label string
}{
	{"ai_companion", "AI companion"},
	{"business", "Business"},
	{"person", "Person"},
	{"project", "Project"},
	{"other", "Other"},
}

func typeLabel(k string) string {
	for _, t := range EntityTypes {
		if t.Key == k {
			return t.Label
		}
	}
	return ""
}

// Problem is a file in the identities folder that could not be loaded.
// It is reported, never deleted or overwritten.
type Problem struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

var (
	ErrInvalid       = errors.New("invalid input")
	ErrDuplicateName = errors.New("an identity with this name and type already exists")
	ErrNotFound      = errors.New("no identity with that ID")
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// fileOps is the part of the file system the store writes with; tests
// replace it to simulate a full or read-only disk.
type fileOps interface {
	CreateTemp(dir, pattern string) (writeFile, error)
	Rename(from, to string) error
	Remove(name string) error
}

type writeFile interface {
	io.Writer
	Name() string
	Sync() error
	Close() error
}

type osOps struct{}

func (osOps) CreateTemp(dir, p string) (writeFile, error) { return os.CreateTemp(dir, p) }
func (osOps) Rename(a, b string) error                    { return os.Rename(a, b) }
func (osOps) Remove(n string) error                       { return os.Remove(n) }

type Store struct {
	dir      string // <data>/identities
	ops      fileOps
	mu       sync.Mutex
	items    map[string]Identity
	problems []Problem
	requests map[string]string // create request ID -> identity ID (repeat-click safety)
	failNext bool              // owner test tool: make the next save fail
	now      func() time.Time
}

func openStore(dataDir string) (*Store, error) { return openStoreWith(dataDir, osOps{}) }

func openStoreWith(dataDir string, ops fileOps) (*Store, error) {
	dir := filepath.Join(dataDir, "identities")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("can't create the identities folder %s: %w", dir, err)
	}
	s := &Store{dir: dir, ops: ops, items: map[string]Identity{}, requests: map[string]string{}, now: time.Now}
	return s, s.Load()
}

// Load reads every record. Bad files are listed as problems and left alone;
// good records still load.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("can't read the identities folder: %w", err)
	}
	items := map[string]Identity{}
	var problems []Problem
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue // temp files (".tmp-*") from an interrupted save are ignored
		}
		id, err := readRecord(filepath.Join(s.dir, name))
		if err == nil && name != id.ID+".json" {
			err = fmt.Errorf("file name doesn't match the ID inside it (%s)", id.ID)
		}
		if err == nil {
			if _, dup := items[id.ID]; dup {
				err = fmt.Errorf("duplicate ID %s", id.ID)
			}
		}
		if err != nil {
			problems = append(problems, Problem{File: name, Reason: err.Error()})
			continue
		}
		items[id.ID] = id
	}
	s.items, s.problems = items, problems
	return nil
}

func readRecord(path string) (Identity, error) {
	var id Identity
	b, err := os.ReadFile(path)
	if err != nil {
		return id, fmt.Errorf("can't read: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&id); err != nil {
		return id, fmt.Errorf("not a valid identity record: %v", err)
	}
	switch {
	case id.SchemaVersion != schemaVersion:
		return id, fmt.Errorf("unsupported schema_version %d (this app reads %d)", id.SchemaVersion, schemaVersion)
	case !uuidRe.MatchString(id.ID):
		return id, fmt.Errorf("invalid id %q", id.ID)
	case typeLabel(id.EntityType) == "":
		return id, fmt.Errorf("unknown entity_type %q", id.EntityType)
	case id.CreatedAt.IsZero():
		return id, errors.New("missing created_at")
	}
	if _, err := cleanName(id.DisplayName); err != nil {
		return id, err
	}
	return id, nil
}

// cleanName trims and checks a display name.
func cleanName(n string) (string, error) {
	n = strings.TrimSpace(n)
	switch {
	case n == "":
		return "", fmt.Errorf("%w: name is empty", ErrInvalid)
	case !utf8.ValidString(n):
		return "", fmt.Errorf("%w: name is not valid text", ErrInvalid)
	case utf8.RuneCountInString(n) > 80:
		return "", fmt.Errorf("%w: name is longer than 80 characters", ErrInvalid)
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: name contains control characters", ErrInvalid)
		}
	}
	return n, nil
}

// newUUID returns a random version-4 UUID.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

type CreateResult struct {
	Identity Identity `json:"identity"`
	Repeat   bool     `json:"repeat"` // same request seen before: nothing new was created
}

// Create makes and saves one identity. requestID (from the form) makes a
// repeated click or retry return the first result instead of a second copy.
func (s *Store) Create(name, entityType, requestID string, allowDuplicateName bool) (CreateResult, error) {
	name, err := cleanName(name)
	if err != nil {
		return CreateResult{}, err
	}
	if typeLabel(entityType) == "" {
		return CreateResult{}, fmt.Errorf("%w: choose a supported entity type", ErrInvalid)
	}
	if len(requestID) > 100 {
		return CreateResult{}, fmt.Errorf("%w: request_id too long", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if requestID != "" {
		if prev, ok := s.requests[requestID]; ok {
			if id, ok := s.items[prev]; ok {
				return CreateResult{Identity: id, Repeat: true}, nil
			}
		}
	}
	if !allowDuplicateName {
		for _, it := range s.items {
			if it.EntityType == entityType && strings.EqualFold(it.DisplayName, name) {
				return CreateResult{}, fmt.Errorf("%w (ID %s)", ErrDuplicateName, it.ID)
			}
		}
	}
	var id string
	for tries := 0; ; tries++ {
		if id, err = newUUID(); err != nil {
			return CreateResult{}, fmt.Errorf("couldn't generate an ID: %w", err)
		}
		_, inMem := s.items[id]
		_, statErr := os.Stat(filepath.Join(s.dir, id+".json"))
		if !inMem && errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if tries > 5 {
			return CreateResult{}, errors.New("couldn't find an unused ID")
		}
	}
	rec := Identity{SchemaVersion: schemaVersion, ID: id, DisplayName: name, EntityType: entityType,
		CreatedAt: s.now().UTC()}
	if err := s.writeRecord(rec); err != nil {
		return CreateResult{}, err // nothing added; earlier records untouched
	}
	s.items[id] = rec
	if requestID != "" {
		s.requests[requestID] = id
	}
	return CreateResult{Identity: rec}, nil
}

func (s *Store) writeRecord(rec Identity) error {
	if s.failNext {
		s.failNext = false
		return errors.New("save failed: simulated failure (owner test tool) — nothing was written")
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	f, err := s.ops.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("save failed: can't write in %s: %w", s.dir, err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = s.ops.Remove(tmp)
		}
	}()
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("save failed while writing: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("save failed while flushing to disk: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save failed while closing: %w", err)
	}
	if err := s.ops.Rename(tmp, filepath.Join(s.dir, rec.ID+".json")); err != nil {
		return fmt.Errorf("save failed while moving into place: %w", err)
	}
	ok = true
	// Read it back: "Saved" is only shown for a record that is really on disk.
	got, err := readRecord(filepath.Join(s.dir, rec.ID+".json"))
	if err != nil || got != rec {
		return fmt.Errorf("save could not be verified after writing: %v", err)
	}
	return nil
}

func (s *Store) List() []Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Identity, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Store) Get(id string) (Identity, error) {
	if !uuidRe.MatchString(id) {
		return Identity{}, fmt.Errorf("%w: not a valid ID", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return Identity{}, ErrNotFound
	}
	return it, nil
}

func (s *Store) Problems() []Problem {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Problem(nil), s.problems...)
}

// ---------- owner test tools (acceptance F) ----------

func (s *Store) FailNextSave() {
	s.mu.Lock()
	s.failNext = true
	s.mu.Unlock()
}

func (s *Store) FailNextPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failNext
}

// WriteCorruptTestFile adds a deliberately broken file so the owner can see
// how problems are reported. Its name marks it as a test file.
func (s *Store) WriteCorruptTestFile() (string, error) {
	name := fmt.Sprintf("corrupt-test-%d.json", s.now().UnixNano())
	if err := os.WriteFile(filepath.Join(s.dir, name), []byte(`{"schema_version":1,"id":"not-finished`), 0o600); err != nil {
		return "", err
	}
	return name, s.Load()
}

// RemoveCorruptTestFiles deletes only files made by WriteCorruptTestFile.
func (s *Store) RemoveCorruptTestFiles() (int, error) {
	m, _ := filepath.Glob(filepath.Join(s.dir, "corrupt-test-*.json"))
	n := 0
	for _, f := range m {
		if err := os.Remove(f); err != nil {
			return n, err
		}
		n++
	}
	return n, s.Load()
}
