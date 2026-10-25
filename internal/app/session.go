package app

import (
	"crypto/subtle"
	"net/http"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/profile"
	"github.com/Arameair/identity/internal/redact"
)

// Local operator session.
//
// Proves only that the browser was handed this run's private open link
// (printed in Identity's console and saved 0600 in the profile runtime
// folder). 127.0.0.1 alone is NOT treated as authentication. Keys and
// sessions die with the process.
type Sessions struct {
	mu       sync.Mutex
	key      string
	sess     map[string]sessionRec
	failures int
	lockedTo time.Time
}

type sessionRec struct {
	csrf    string
	created time.Time
}

const CookieName = "identity_b_session"

func NewSessions() *Sessions {
	return &Sessions{key: profile.RandHex(24), sess: map[string]sessionRec{}}
}

func (s *Sessions) Key() string { return s.key }

func (s *Sessions) Exchange(key string) (id, csrf string, ok, locked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.lockedTo) {
		return "", "", false, true
	}
	if subtle.ConstantTimeCompare([]byte(key), []byte(s.key)) != 1 {
		s.failures++
		if s.failures >= 10 {
			s.lockedTo, s.failures = time.Now().Add(time.Minute), 0
		}
		return "", "", false, false
	}
	s.failures = 0
	id, csrf = profile.RandHex(32), profile.RandHex(32)
	s.sess[id] = sessionRec{csrf: csrf, created: time.Now()}
	redact.Register("session_cookie", id)
	redact.Register("csrf", csrf)
	return id, csrf, true, false
}

func (s *Sessions) Lookup(r *http.Request) (sessionRec, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || len(c.Value) != 64 {
		return sessionRec{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.sess[c.Value]
	return rec, ok
}

func (s *Sessions) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sess)
}
