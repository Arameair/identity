// Package authority holds Identity's P1 authority records: the original
// Admin Identity, operation-bound Admin approvals, feature installation
// identities and scoped grants. Everything here fails closed: a damaged or
// unreadable file disables the protected operation; it never resets it.
package authority

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/profile"
)

const AdminSchema = "identity.admin/1"

var adminIDRE = regexp.MustCompile(`^adm_[0-9a-f]{20}$`)

// GitHubBinding is the external sign-in account bound at enrollment. It is
// an authentication alias and distribution input, NOT the authority: the
// Admin Identity is the local record below. The numeric ID is what binds
// (GitHub keeps it when a login is renamed); the login is only a label.
type GitHubBinding struct {
	ID          int64     `json:"id"`
	Login       string    `json:"login_label"`
	Mode        string    `json:"mode"` // real | TEST FIXTURE
	VerifiedAt  time.Time `json:"verified_at"`
	ProofMethod string    `json:"proof_method"`
}

// Admin is the original, durable Admin Identity of this installation.
type Admin struct {
	Schema        string        `json:"schema"`
	AdminID       string        `json:"admin_id"`
	SystemID      string        `json:"system_id"`
	EstablishedAt time.Time     `json:"established_at"`
	ApprovalID    string        `json:"enrollment_approval_id"`
	GitHub        GitHubBinding `json:"github_binding"`
	Authenticator Verifier      `json:"authenticator"`
	// PrimaryRequester is the paired program (e.g. processor:tp_…) the owner
	// used to establish the Admin. Approval pages warn when a request comes
	// from anything else (Grok M3). Absent in admin.json made before cleanup.
	PrimaryRequester string `json:"primary_requester,omitempty"`
}

// Public is what may be shown (no verifier).
type AdminPublic struct {
	AdminID           string        `json:"admin_id"`
	SystemID          string        `json:"system_id"`
	EstablishedAt     time.Time     `json:"established_at"`
	GitHub            GitHubBinding `json:"github_binding"`
	AuthenticatorKind string        `json:"authenticator"`
	PrimaryRequester  string        `json:"primary_requester,omitempty"`
}

func (a Admin) Public() AdminPublic {
	return AdminPublic{AdminID: a.AdminID, SystemID: a.SystemID, EstablishedAt: a.EstablishedAt, GitHub: a.GitHub, AuthenticatorKind: a.Authenticator.Kind,
		PrimaryRequester: a.PrimaryRequester}
}

type throttle struct {
	Failures    int       `json:"failures"`
	LockedUntil time.Time `json:"locked_until"`
	Lockouts    int       `json:"lockouts"`
}

// AdminStore keeps admin.json and the passphrase attempt throttle.
type AdminStore struct {
	mu      sync.Mutex
	dir     string
	admin   *Admin
	loadErr string
	thr     throttle
	thrErr  string
	Now     func() time.Time
}

var (
	ErrAdminExists    = errors.New("this installation already has its Admin Identity; enrollment happens once and cannot transfer ownership")
	ErrNoAdmin        = errors.New("no Admin Identity is established on this installation yet")
	ErrAdminDamaged   = errors.New("the Admin record (admin.json) is damaged or unreadable; every Admin-approved action is blocked until it is repaired. It was NOT treated as missing")
	ErrBadPassphrase  = errors.New("that Admin passphrase is not correct")
	ErrLockedOut      = errors.New("too many wrong Admin passphrases; wait and try again")
	ErrThrottleDamage = errors.New("the passphrase attempt record is damaged; approvals are blocked until it is repaired")
)

const (
	maxFailures  = 5
	baseLockout  = 5 * time.Minute
	maxLockout   = time.Hour
	adminFile    = "admin.json"
	throttleFile = "admin-throttle.json"
)

func OpenAdmin(dir string) *AdminStore {
	s := &AdminStore{dir: dir, Now: time.Now}
	b, err := os.ReadFile(filepath.Join(dir, adminFile))
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.loadErr = ErrAdminDamaged.Error() + " (" + err.Error() + ")"
	default:
		var a Admin
		if json.Unmarshal(b, &a) != nil || a.Schema != AdminSchema || !adminIDRE.MatchString(a.AdminID) || a.Authenticator.Kind != AuthenticatorKind || a.GitHub.ID == 0 {
			s.loadErr = ErrAdminDamaged.Error()
		} else {
			s.admin = &a
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, throttleFile)); err == nil {
		if json.Unmarshal(b, &s.thr) != nil {
			s.thrErr = ErrThrottleDamage.Error()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		s.thrErr = ErrThrottleDamage.Error()
	}
	return s
}

// State: NOT_ENROLLED | ENROLLED | DAMAGED.
func (s *AdminStore) State() (string, *AdminPublic, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return "DAMAGED", nil, s.loadErr
	}
	if s.admin == nil {
		return "NOT_ENROLLED", nil, ""
	}
	p := s.admin.Public()
	return "ENROLLED", &p, ""
}

// Get returns the admin (or ErrNoAdmin / ErrAdminDamaged).
func (s *AdminStore) Get() (Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Admin{}, ErrAdminDamaged
	}
	if s.admin == nil {
		return Admin{}, ErrNoAdmin
	}
	return *s.admin, nil
}

// Enroll establishes the Admin Identity exactly once. The file is created
// with an atomic link-if-absent, so two racing enrollments cannot both win,
// and an existing (even damaged) admin.json is never replaced.
func (s *AdminStore) Enroll(systemID, approvalID string, gh GitHubBinding, passphrase string, primary ...string) (Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Admin{}, ErrAdminDamaged
	}
	if s.admin != nil {
		return Admin{}, ErrAdminExists
	}
	if gh.ID == 0 {
		return Admin{}, errors.New("a GitHub-verified account is required to establish the Admin Identity")
	}
	v, err := NewVerifier(passphrase)
	if err != nil {
		return Admin{}, err
	}
	a := Admin{Schema: AdminSchema, AdminID: "adm_" + profile.RandHex(10), SystemID: systemID, EstablishedAt: s.Now().UTC(),
		ApprovalID: approvalID, GitHub: gh, Authenticator: v}
	if len(primary) > 0 {
		a.PrimaryRequester = primary[0]
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return Admin{}, err
	}
	final := filepath.Join(s.dir, adminFile)
	tmp := filepath.Join(s.dir, ".admin-"+profile.RandHex(6)+".tmp")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Admin{}, err
	}
	_, err = f.Write(append(b, '\n'))
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return Admin{}, fmt.Errorf("Admin record could not be written: %w", err)
	}
	defer os.Remove(tmp)
	if err := os.Link(tmp, final); err != nil {
		if _, statErr := os.Lstat(final); statErr == nil {
			return Admin{}, ErrAdminExists
		}
		return Admin{}, fmt.Errorf("Admin record could not be committed: %w", err)
	}
	s.admin = &a
	return a, nil
}

// Authenticate verifies the Admin passphrase with a persisted attempt
// throttle. A failure to persist the throttle denies the attempt.
func (s *AdminStore) Authenticate(passphrase string) (Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Admin{}, ErrAdminDamaged
	}
	if s.admin == nil {
		return Admin{}, ErrNoAdmin
	}
	if s.thrErr != "" {
		return Admin{}, ErrThrottleDamage
	}
	now := s.Now()
	if now.Before(s.thr.LockedUntil) {
		return Admin{}, fmt.Errorf("%w (until %s)", ErrLockedOut, s.thr.LockedUntil.Local().Format("15:04:05"))
	}
	if s.admin.Authenticator.Check(passphrase) {
		if s.thr.Failures != 0 || s.thr.Lockouts != 0 {
			old := s.thr
			s.thr = throttle{}
			if err := profile.WriteJSON(filepath.Join(s.dir, throttleFile), s.thr); err != nil {
				s.thr = old
				return Admin{}, errors.New("approval blocked: the attempt record could not be saved")
			}
		}
		return *s.admin, nil
	}
	old := s.thr
	s.thr.Failures++
	if s.thr.Failures >= maxFailures {
		d := baseLockout << s.thr.Lockouts
		if d > maxLockout || d <= 0 {
			d = maxLockout
		}
		s.thr.LockedUntil, s.thr.Failures = now.Add(d), 0
		s.thr.Lockouts++
	}
	if err := profile.WriteJSON(filepath.Join(s.dir, throttleFile), s.thr); err != nil {
		s.thr = old
		return Admin{}, errors.New("approval blocked: the attempt record could not be saved")
	}
	return Admin{}, ErrBadPassphrase
}
