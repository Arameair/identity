// Package pairing lets a local Thought Processor prove, on every request, that
// a person paired it with this Identity.
//
//  1. Processor asks (POST /api/v1/pair/request), or the person presses
//     Pair in Identity's troubleshooting page. Identity shows a one-time code
//     (8 characters, 5 minutes, single use) only in its own window.
//  2. They type it into Processor; Processor sends it to POST /api/v1/pair.
//  3. Identity answers with a random 256-bit client token. Identity keeps
//     only its SHA-256; Processor keeps the token in its own profile.
//
// Later requests carry the token as a bearer header; Identity also signs its
// answers with an HMAC keyed by that token, so Processor can check the answer
// came from the Identity it paired with (see Proof).
package pairing

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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
	"github.com/Arameair/identity/internal/redact"
)

const (
	CodeTTL      = 5 * time.Minute
	maxFailures  = 5
	lockDuration = time.Minute
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // no 0/O, 1/I/L
)

var idRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{4,64}$`)

// ValidProcessorID reports whether id is an acceptable processor_id.
func ValidProcessorID(id string) bool { return idRE.MatchString(id) }

type Client struct {
	ProcessorID string    `json:"processor_id"`
	Label       string    `json:"label"`
	TokenSHA256 string    `json:"token_sha256"`
	PairedAt    time.Time `json:"paired_at"`
	LastUsed    time.Time `json:"last_used,omitempty"`
}

// PublicClient is what the UI and diagnostics may show (no hash).
type PublicClient struct {
	ProcessorID string    `json:"processor_id"`
	Label       string    `json:"label"`
	PairedAt    time.Time `json:"paired_at"`
	LastUsed    time.Time `json:"last_used,omitempty"`
}

type Store struct {
	mu       sync.Mutex
	path     string
	clients  []Client
	loadErr  string
	code     string
	codeExp  time.Time
	failures int
	lockedTo time.Time
	now      func() time.Time
}

func Open(root string) *Store {
	s := &Store{path: filepath.Join(root, "paired-clients.json"), now: time.Now}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.loadErr = "paired-clients.json could not be read: " + err.Error()
	default:
		if err := json.Unmarshal(b, &s.clients); err != nil {
			s.loadErr = "paired-clients.json is damaged; it was left untouched and pairing is disabled until it is fixed or moved"
			s.clients = nil
		}
	}
	return s
}

func (s *Store) LoadError() string { s.mu.Lock(); defer s.mu.Unlock(); return s.loadErr }

func randomCode() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	out := make([]byte, 0, 9)
	for i, x := range b {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, codeAlphabet[int(x)%len(codeAlphabet)])
	}
	return string(out)
}

// NewCode replaces any pending code with a fresh one.
func (s *Store) NewCode() (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return "", time.Time{}, errors.New(s.loadErr)
	}
	if s.code != "" {
		redact.Forget(s.code)
	}
	s.code, s.codeExp = randomCode(), s.now().Add(CodeTTL)
	redact.Register("pairing_code", s.code)
	return s.code, s.codeExp, nil
}

// Pending returns the code currently waiting to be used, if any.
func (s *Store) Pending() (string, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.code == "" || s.now().After(s.codeExp) {
		return "", time.Time{}
	}
	return s.code, s.codeExp
}

func (s *Store) CancelCode() {
	s.mu.Lock()
	defer s.mu.Unlock()
	redact.Forget(s.code)
	s.code = ""
}

var (
	ErrLocked     = errors.New("too many wrong pairing codes; wait a minute and ask Identity for a new code")
	ErrBadCode    = errors.New("that pairing code is wrong or expired; ask Identity for a new one")
	ErrBadRequest = errors.New("a processor_id (4–64 letters, digits, . _ -) is required")
)

func normalize(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "")
	if len(code) == 8 && !strings.Contains(code, "-") {
		code = code[:4] + "-" + code[4:]
	}
	return code
}

func hashToken(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }

// Pair consumes the pending code and returns a new client token.
func (s *Store) Pair(code, processorID, label string) (string, error) {
	if !idRE.MatchString(processorID) {
		return "", ErrBadRequest
	}
	if len(label) > 80 {
		label = label[:80]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return "", errors.New(s.loadErr)
	}
	now := s.now()
	if now.Before(s.lockedTo) {
		return "", ErrLocked
	}
	valid := s.code != "" && now.Before(s.codeExp) &&
		subtle.ConstantTimeCompare([]byte(normalize(code)), []byte(s.code)) == 1
	if !valid {
		s.failures++
		if s.failures >= maxFailures {
			s.failures, s.lockedTo = 0, now.Add(lockDuration)
			redact.Forget(s.code)
			s.code = "" // a guessed-at code is burned
		}
		return "", ErrBadCode
	}
	redact.Forget(s.code)
	s.code, s.failures = "", 0
	tb := make([]byte, 32)
	if _, err := rand.Read(tb); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tb)
	keep := []Client{}
	for _, c := range s.clients {
		if c.ProcessorID != processorID {
			keep = append(keep, c)
		}
	}
	keep = append(keep, Client{ProcessorID: processorID, Label: label, TokenSHA256: hashToken(token), PairedAt: now.UTC()})
	if err := profile.WriteJSON(s.path, keep); err != nil {
		return "", errors.New("pairing could not be saved: " + err.Error())
	}
	s.clients = keep
	redact.Register("pairing_token", token)
	return token, nil
}

// Authenticate checks a bearer token and returns the paired client.
func (s *Store) Authenticate(token string) (PublicClient, bool) {
	if len(token) != 64 {
		return PublicClient{}, false
	}
	h := hashToken(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.clients {
		if subtle.ConstantTimeCompare([]byte(h), []byte(c.TokenSHA256)) == 1 {
			s.clients[i].LastUsed = s.now().UTC()
			redact.Register("pairing_token", token)
			return public(s.clients[i]), true
		}
	}
	return PublicClient{}, false
}

func public(c Client) PublicClient {
	return PublicClient{ProcessorID: c.ProcessorID, Label: c.Label, PairedAt: c.PairedAt, LastUsed: c.LastUsed}
}

func (s *Store) Clients() []PublicClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []PublicClient{}
	for _, c := range s.clients {
		out = append(out, public(c))
	}
	return out
}

// Unpair removes a paired Processor; its token stops working immediately.
func (s *Store) Unpair(processorID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keep, found := []Client{}, false
	for _, c := range s.clients {
		if c.ProcessorID == processorID {
			found = true
			continue
		}
		keep = append(keep, c)
	}
	if !found {
		return false, nil
	}
	if err := profile.WriteJSON(s.path, keep); err != nil {
		return false, err
	}
	s.clients = keep
	return true, nil
}

// Proof is the HMAC-SHA256, keyed by the client token, of the canonical
// string. Processor recomputes it to check the answer came from the Identity
// that issued its token.
func Proof(token, canonical string) string {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte(canonical))
	return hex.EncodeToString(m.Sum(nil))
}
