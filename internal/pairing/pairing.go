// Package pairing lets a local Thought Processor prove, on every request, that
// a person paired it with this Identity.
//
//  1. Processor asks (POST /api/v1/pair/request), or the person presses
//     Pair in Identity's troubleshooting page. Identity shows a one-time code
//     (8 characters, 5 minutes, single use) only in its own window.
//  2. They type it into Processor; Processor sends it to POST /api/v1/pair.
//  3. Identity answers with a random 256-bit pairing secret, once. Both sides
//     derive two keys from it (DeriveKeys): a request key and a response key.
//     Processor keeps the secret (DPAPI-sealed on Windows); Identity keeps
//     only the two derived keys (0600 file in its profile).
//
// API v2 (identity/2): THE SECRET NEVER TRAVELS AGAIN. Each request carries
// X-Processor-ID, a fresh nonce, the time, and an HMAC over method, path,
// time, nonce and body (RequestMAC). Identity refuses stale times and reused
// nonces. Each answer carries an HMAC with the response key over the request
// nonce, the status and the exact body (ResponseProof).
//
// So a program that grabs Identity's port can neither learn anything it
// could replay against the real Identity, nor sign an answer Processor would
// believe. (v1 sent the secret as a bearer token and signed with it, which
// let such a program do both — fixed here.)
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
	"strconv"
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
	TokenSHA256 string    `json:"token_sha256,omitempty"` // v1 pairings (no longer accepted: pair again)
	RequestKey  string    `json:"request_key,omitempty"`  // v2: hex HMAC key, derived from the secret
	ResponseKey string    `json:"response_key,omitempty"` // v2
	TicketKey   string    `json:"ticket_key,omitempty"`   // feature installations only: signs tickets addressed to it
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
	mu        sync.Mutex
	seen      map[string]time.Time // processor_id|nonce → when (replay protection)
	noncePath string               // durable replay guard for state-changing signed requests
	path      string
	clients   []Client
	loadErr   string
	code      string
	codeOwner string // processor_id the code was shown for ("" = any, e.g. made in Identity's own page)
	codeLabel string // label shown in Identity's console with the code; becomes the paired label
	codeExp   time.Time
	failures  int
	lockedTo  time.Time
	now       func() time.Time
}

func Open(root string) *Store {
	return OpenFiles(root, "paired-clients.json", "paired-nonces.json")
}

// OpenFiles opens a signer registry kept in its own files. Identity keeps
// paired Processors and registered feature installations in SEPARATE
// registries, so a key from one can never authenticate as the other.
func OpenFiles(root, clientsFile, noncesFile string) *Store {
	s := &Store{path: filepath.Join(root, clientsFile), noncePath: filepath.Join(root, noncesFile), now: time.Now}
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
	// A corrupt/unreadable replay ledger is not a fresh installation:
	// deny all signed operations instead of forgetting used nonces.
	if b, err := os.ReadFile(s.noncePath); err == nil {
		if err := json.Unmarshal(b, &s.seen); err != nil || s.seen == nil {
			s.loadErr = "paired-nonces.json is damaged; signed operations are disabled until repaired"
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		s.loadErr = "paired-nonces.json could not be read: " + err.Error()
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

// NewCode replaces any pending code with a fresh one usable by any program
// (Identity's own page). See NewCodeFor.
func (s *Store) NewCode() (string, time.Time, error) { return s.NewCodeFor("", "") }

// NewCodeFor makes a code that only processorID can use (Grok L2: a wrong
// guess from a different program neither burns it nor locks pairing for the
// program the person is actually connecting). The label is shown next to
// the code in Identity's console and becomes the pairing's verified label.
func (s *Store) NewCodeFor(processorID, label string) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codeOwner, s.codeLabel = processorID, label
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
	ErrCodeOwner  = errors.New("that pairing code was shown for a different program; ask Identity for your own code")
)

func normalize(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, " ", "")
	if len(code) == 8 && !strings.Contains(code, "-") {
		code = code[:4] + "-" + code[4:]
	}
	return code
}

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
	if s.codeOwner != "" && processorID != s.codeOwner {
		return "", ErrCodeOwner
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
	if s.codeOwner != "" && s.codeLabel != "" {
		label = s.codeLabel // the label the person saw next to the code
	}
	s.code, s.failures, s.codeOwner, s.codeLabel = "", 0, "", ""
	tb := make([]byte, 32)
	if _, err := rand.Read(tb); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tb)
	rk, sk := DeriveKeys(token)
	keep := []Client{}
	for _, c := range s.clients {
		if c.ProcessorID != processorID {
			keep = append(keep, c)
		}
	}
	keep = append(keep, Client{ProcessorID: processorID, Label: label, RequestKey: hex.EncodeToString(rk),
		ResponseKey: hex.EncodeToString(sk), PairedAt: now.UTC()})
	if err := profile.WriteJSON(s.path, keep); err != nil {
		return "", errors.New("pairing could not be saved: " + err.Error())
	}
	s.clients = keep
	redact.Register("pairing_token", token)
	return token, nil
}

// Enroll mints a new 256-bit secret for id (no pairing code: the caller has
// already authorized this, e.g. an Admin-approved feature installation).
// An existing entry for id is replaced, so the old secret stops working.
// The secret is returned once; only the derived keys are stored.
func (s *Store) Enroll(id, label string) (string, error) {
	if !idRE.MatchString(id) {
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
	tb := make([]byte, 32)
	if _, err := rand.Read(tb); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(tb)
	rk, sk := DeriveKeys(secret)
	keep := []Client{}
	for _, c := range s.clients {
		if c.ProcessorID != id {
			keep = append(keep, c)
		}
	}
	keep = append(keep, Client{ProcessorID: id, Label: label, RequestKey: hex.EncodeToString(rk),
		ResponseKey: hex.EncodeToString(sk), PairedAt: s.now().UTC()})
	if err := profile.WriteJSON(s.path, keep); err != nil {
		return "", errors.New("credential could not be saved: " + err.Error())
	}
	s.clients = keep
	redact.Register("installation_secret", secret)
	return secret, nil
}

// TicketKey derives the key Identity uses to sign request tickets addressed
// to the holder of secret (a feature installation). Requesters never learn it.
func TicketKey(secret string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("identity/2 ticket key"))
	return m.Sum(nil)
}

// TicketKeyFor returns the stored ticket key for a registered id. Only
// installations enrolled with EnrollWithTicketKey have one.
func (s *Store) TicketKeyFor(id string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.clients {
		if c.ProcessorID == id && c.TicketKey != "" {
			k, err := hex.DecodeString(c.TicketKey)
			return k, err == nil
		}
	}
	return nil, false
}

// EnrollWithTicketKey is Enroll plus a stored ticket key (feature installations).
func (s *Store) EnrollWithTicketKey(id, label string) (string, error) {
	secret, err := s.Enroll(id, label)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.clients {
		if s.clients[i].ProcessorID == id {
			s.clients[i].TicketKey = hex.EncodeToString(TicketKey(secret))
		}
	}
	if err := profile.WriteJSON(s.path, s.clients); err != nil {
		return "", errors.New("credential could not be saved: " + err.Error())
	}
	return secret, nil
}

// Lookup returns the public record for id (label bound at pairing time).
func (s *Store) Lookup(id string) (PublicClient, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.clients {
		if c.ProcessorID == id {
			return public(c), true
		}
	}
	return PublicClient{}, false
}

// Has reports whether id is registered.
func (s *Store) Has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.clients {
		if c.ProcessorID == id {
			return true
		}
	}
	return false
}

// ---- API v2 request authentication ----

const (
	MaxClockSkew = 60 * time.Second
	nonceTTL     = 3 * MaxClockSkew
)

var (
	ErrUnknownClient = errors.New("this program is not paired with this Identity (or the pairing was removed)")
	ErrOldPairing    = errors.New("this pairing was made with an older Identity that sent its secret over the wire; pair again")
	ErrStale         = errors.New("request time is too far from Identity's clock (check the computer's clock)")
	ErrReplay        = errors.New("request was already used (replayed)")
	ErrBadMAC        = errors.New("request signature is wrong")
	ErrReplayStore   = errors.New("replay protection unavailable; signed operation denied")
	nonceRE          = regexp.MustCompile(`^[0-9a-f]{16,64}$`)
)

// DeriveKeys turns the pairing secret into the request and response keys.
func DeriveKeys(secret string) (req, resp []byte) {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("identity/2 request key"))
	req = m.Sum(nil)
	m = hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("identity/2 response key"))
	resp = m.Sum(nil)
	return
}

// RequestMAC is what X-Identity-Auth must contain.
func RequestMAC(reqKey []byte, method, pathAndQuery, unixTime, nonce string, body []byte) string {
	h := sha256.Sum256(body)
	m := hmac.New(sha256.New, reqKey)
	m.Write([]byte("identity/2 request\n" + method + "\n" + pathAndQuery + "\n" + unixTime + "\n" + nonce + "\n" + hex.EncodeToString(h[:])))
	return hex.EncodeToString(m.Sum(nil))
}

// ResponseProof is what X-Identity-Proof contains.
func ResponseProof(respKey []byte, nonce string, status int, body []byte) string {
	m := hmac.New(sha256.New, respKey)
	m.Write([]byte("identity/2 response\n" + nonce + "\n" + strconv.Itoa(status) + "\n"))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// Verify authenticates one request. On success it returns the client and its
// response key (used to sign the answer).
func (s *Store) Verify(processorID, method, pathAndQuery, unixTime, nonce, mac string, body []byte) (PublicClient, []byte, error) {
	if !idRE.MatchString(processorID) || !nonceRE.MatchString(nonce) || len(mac) != 64 {
		return PublicClient{}, nil, ErrUnknownClient
	}
	ts, err := strconv.ParseInt(unixTime, 10, 64)
	now := s.now()
	if err != nil {
		return PublicClient{}, nil, ErrStale
	}
	if d := now.Sub(time.Unix(ts, 0)); d > MaxClockSkew || d < -MaxClockSkew {
		return PublicClient{}, nil, ErrStale
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return PublicClient{}, nil, ErrReplayStore
	}
	idx := -1
	for i, c := range s.clients {
		if c.ProcessorID == processorID {
			idx = i
		}
	}
	if idx < 0 {
		return PublicClient{}, nil, ErrUnknownClient
	}
	c := s.clients[idx]
	if c.RequestKey == "" {
		return PublicClient{}, nil, ErrOldPairing
	}
	rk, err1 := hex.DecodeString(c.RequestKey)
	sk, err2 := hex.DecodeString(c.ResponseKey)
	if err1 != nil || err2 != nil {
		return PublicClient{}, nil, ErrUnknownClient
	}
	if subtle.ConstantTimeCompare([]byte(RequestMAC(rk, method, pathAndQuery, unixTime, nonce, body)), []byte(mac)) != 1 {
		return PublicClient{}, nil, ErrBadMAC
	}
	if s.seen == nil {
		s.seen = map[string]time.Time{}
	}
	for k, t := range s.seen {
		if now.Sub(t) > nonceTTL {
			delete(s.seen, k)
		}
	}
	key := processorID + "|" + nonce
	if _, dup := s.seen[key]; dup {
		return PublicClient{}, nil, ErrReplay
	}
	// Persist mutation replay protection BEFORE permitting the operation.
	// GETs remain in-memory only: they do not modify protected Identity
	// state, and the signed response proof still binds response to request.
	if method == "POST" {
		// Bound disk work and refuse excessive mutation traffic safely.
		if len(s.seen) >= 4096 {
			return PublicClient{}, nil, ErrReplayStore
		}
		pending := make(map[string]time.Time, len(s.seen)+1)
		for k, t := range s.seen {
			pending[k] = t
		}
		pending[key] = now
		if err := profile.WriteJSON(s.noncePath, pending); err != nil {
			return PublicClient{}, nil, ErrReplayStore
		}
	}
	s.seen[key] = now
	s.clients[idx].LastUsed = now.UTC()
	return public(s.clients[idx]), sk, nil
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

// Proof is an HMAC-SHA256 of a canonical string, keyed by the client's
// response key (older Engine-install answers carry it in a "proof" field).
func Proof(respKey []byte, canonical string) string {
	m := hmac.New(sha256.New, respKey)
	m.Write([]byte(canonical))
	return hex.EncodeToString(m.Sum(nil))
}
