package authority

// Local Admin authenticator for hosts without a tested platform
// authenticator (Linux / WSL). Owner decision 2026-10-26 (#20, option C:
// flexible local authentication): prefer a platform authenticator where one
// is genuinely available and tested; otherwise a protected local
// alternative. Windows Hello / passkeys are NOT implemented here (BLOCKED).
//
// What is stored: salt, iteration count and PBKDF2-HMAC-SHA256(passphrase).
// The passphrase itself is never stored, logged or sent to Thought Processor
// (it is typed only into Identity's own approval page).
//
// Honest limits: a process running as the same OS user can read the
// verifier file and try offline guesses, or key-log the browser. A strong
// passphrase and the iteration count raise the cost of guessing; they do not
// create an OS security boundary.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	AuthenticatorKind = "local-passphrase-pbkdf2-sha256"
	MinPassphraseLen  = 12
	maxPassphraseLen  = 256
)

// Iterations is the PBKDF2 work factor (OWASP 2023 recommendation for
// PBKDF2-HMAC-SHA256). Tests lower it through SetIterationsForTests.
var Iterations = 600_000

// SetIterationsForTests makes unit tests fast. Never called by the program.
func SetIterationsForTests(n int) { Iterations = n }

// pbkdf2SHA256 implements RFC 8018 PBKDF2 with HMAC-SHA256 (Go 1.22 has no
// crypto/pbkdf2; this is checked against RFC 7914 §11 test vectors).
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hLen := prf.Size()
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	buf := make([]byte, 4)
	u := make([]byte, hLen)
	for block := 1; block <= blocks; block++ {
		prf.Reset()
		prf.Write(salt)
		binary.BigEndian.PutUint32(buf, uint32(block))
		prf.Write(buf)
		t := prf.Sum(nil)
		copy(u, t)
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for x := range t {
				t[x] ^= u[x]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

// Verifier is the stored, non-reversible form of the Admin passphrase.
type Verifier struct {
	Kind       string `json:"kind"`
	Salt       string `json:"salt"`
	Iterations int    `json:"iterations"`
	Hash       string `json:"hash"`
}

var (
	ErrWeakPassphrase = errors.New("the Admin passphrase must be at least 12 characters (a few words is easiest to remember)")
	ErrLongPassphrase = errors.New("the Admin passphrase is too long (256 characters at most)")
)

// CheckPassphraseRules validates a new passphrase.
func CheckPassphraseRules(p string) error {
	n := utf8.RuneCountInString(p)
	if n < MinPassphraseLen || strings.TrimSpace(p) == "" {
		return ErrWeakPassphrase
	}
	if len(p) > maxPassphraseLen {
		return ErrLongPassphrase
	}
	return nil
}

// NewVerifier derives a verifier for a new passphrase.
func NewVerifier(passphrase string) (Verifier, error) {
	if err := CheckPassphraseRules(passphrase); err != nil {
		return Verifier{}, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return Verifier{}, err
	}
	h := pbkdf2SHA256([]byte(passphrase), salt, Iterations, 32)
	return Verifier{Kind: AuthenticatorKind, Salt: hex.EncodeToString(salt), Iterations: Iterations, Hash: hex.EncodeToString(h)}, nil
}

// Check reports whether passphrase matches (constant-time compare).
func (v Verifier) Check(passphrase string) bool {
	if v.Kind != AuthenticatorKind || v.Iterations < 1 || len(passphrase) > maxPassphraseLen {
		return false
	}
	salt, err1 := hex.DecodeString(v.Salt)
	want, err2 := hex.DecodeString(v.Hash)
	if err1 != nil || err2 != nil || len(want) != 32 {
		return false
	}
	got := pbkdf2SHA256([]byte(passphrase), salt, v.Iterations, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
