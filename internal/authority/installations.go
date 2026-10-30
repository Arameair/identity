package authority

// Feature installation identities. Every installed copy of a feature gets
// its own durable installation ID, minted here by Identity, even while it is
// stopped and before it has any grant. A runtime restart never mints a new
// one; a retried registration with the same install key returns the same ID.
//
// States:
//   PENDING_CONFIRM  ID reserved and a credential issued; Processor has not
//                    yet proven (with that credential) that it saved it.
//   INSTALLED        confirmed. Starts with ZERO grants.
//   ABANDONED        an unconfirmed registration the requester gave up on.
//   UNINSTALLED      tombstone: grants revoked, credential removed, ID kept.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/profile"
)

const (
	InsPending     = "PENDING_CONFIRM"
	InsInstalled   = "INSTALLED"
	InsAbandoned   = "ABANDONED"
	InsUninstalled = "UNINSTALLED"

	// The only trust class Identity will register in P1 (owner rule: untrusted
	// third-party executables stay BLOCKED until provenance + OS isolation).
	TrustLocalFixture = "TRUSTED_LOCAL_TEST_FIXTURE"

	installationsFile   = "installations.json"
	installationsSchema = "identity.installations/1"
)

var (
	insIDRE      = regexp.MustCompile(`^ins_[0-9a-f]{20}$`)
	featureIDRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,47}$`)
	capIDRE      = regexp.MustCompile(`^[a-z][a-z0-9_.-]{2,63}$`)
	sha256RE     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	installKeyRE = regexp.MustCompile(`^[0-9a-f]{32,64}$`)
	versionRE    = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,39}$`)
)

func ValidInstallationID(s string) bool { return insIDRE.MatchString(s) }
func ValidCapability(s string) bool     { return capIDRE.MatchString(s) }

type Capability struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	DataClass   string `json:"data_class"`
}

// InstallRequest is the exact proposal the Admin approves.
type InstallRequest struct {
	FeatureID      string       `json:"feature_id"`
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	API            string       `json:"api"`
	PublisherClaim string       `json:"publisher_claim"`
	SourceKind     string       `json:"source_kind"`
	SourceRef      string       `json:"source_ref"`
	TrustClass     string       `json:"trust_class"`
	ArtifactSHA256 string       `json:"artifact_sha256"`
	ManifestSHA256 string       `json:"manifest_sha256"`
	Capabilities   []Capability `json:"capabilities"`
	InstallKey     string       `json:"install_key"`
	RequestedBy    string       `json:"requested_by"`
}

var ErrUntrusted = errors.New("UNTRUSTED_EXECUTION_BLOCKED: only trusted local test fixtures can be registered in P1; third-party executables stay blocked until release provenance and OS sandboxing are verified")

func clean(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// BuiltinFixturePins are the exact artifact digests of the reviewed trusted
// local test fixtures (Thought-Processor processor/testfeatures). Thought
// Processor carries the same list. Grok M1: a package's own manifest can
// never award trust; only an exact pinned digest can.
var BuiltinFixturePins = map[string]string{
	"3cd5e7a60f193f21bd15e14ee1532259fe1088434164ef6af4e3a66a36d493df": "built-in pin: p1-deterministic-worker 0.1.0 (reviewed P1 test fixture)",
}

// TrustPins are the artifact digests this Identity will register.
type TrustPins struct{ m map[string]string }

// NewTrustPins combines built-in pins with operator pins (-trust-fixture-artifact).
func NewTrustPins(operator []string) (TrustPins, error) {
	m := map[string]string{}
	for k, v := range BuiltinFixturePins {
		m[k] = v
	}
	for _, d := range operator {
		d = strings.ToLower(strings.TrimSpace(d))
		if !sha256RE.MatchString(d) {
			return TrustPins{}, errors.New("-trust-fixture-artifact needs a 64-character SHA-256")
		}
		if _, ok := m[d]; !ok {
			m[d] = "operator pin (-trust-fixture-artifact)"
		}
	}
	return TrustPins{m: m}, nil
}

// Basis says why a digest is trusted ("" = not pinned).
func (p TrustPins) Basis(d string) string { return p.m[d] }

var ErrNotPinned = errors.New("UNTRUSTED_EXECUTION_BLOCKED: ARTIFACT_NOT_PINNED — Identity has no pin for this exact artifact digest. A claimed trust class cannot grant trust; only the reviewed fixture (or a digest the operator pinned with -trust-fixture-artifact) can be registered")

// Validate checks shape and the P1 trust rule (exact pinned artifact).
func (r *InstallRequest) Validate(pins TrustPins) error {
	switch {
	case !featureIDRE.MatchString(r.FeatureID):
		return errors.New("feature_id must be 2–48 lowercase letters, digits or -")
	case r.Name == "" || len(r.Name) > 80:
		return errors.New("name must be 1–80 characters")
	case !versionRE.MatchString(r.Version):
		return errors.New("version is not valid")
	case r.API != "thought.feature-api/1":
		return errors.New("api must be thought.feature-api/1")
	case !sha256RE.MatchString(r.ArtifactSHA256) || !sha256RE.MatchString(r.ManifestSHA256):
		return errors.New("artifact_sha256 and manifest_sha256 must be hex SHA-256")
	case !installKeyRE.MatchString(r.InstallKey):
		return errors.New("install_key must be 32–64 hex characters")
	case r.SourceKind != "local-trusted-package":
		return ErrUntrusted
	case r.TrustClass != TrustLocalFixture:
		return ErrUntrusted
	case len(r.Capabilities) > 16:
		return errors.New("too many capabilities")
	case pins.Basis(r.ArtifactSHA256) == "":
		return ErrNotPinned
	}
	seen := map[string]bool{}
	for _, c := range r.Capabilities {
		if !capIDRE.MatchString(c.ID) || seen[c.ID] || len(c.Description) > 200 || len(c.DataClass) > 40 {
			return errors.New("capability ids must be unique, 3–64 of a-z 0-9 _ . -")
		}
		seen[c.ID] = true
	}
	r.PublisherClaim, r.SourceRef = clean(r.PublisherClaim, 120), clean(r.SourceRef, 200)
	return nil
}

type Installation struct {
	ID             string       `json:"installation_id"`
	FeatureID      string       `json:"feature_id"`
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	API            string       `json:"api"`
	PublisherClaim string       `json:"publisher_claim"`
	PublisherProof string       `json:"publisher_proof"`
	SourceKind     string       `json:"source_kind"`
	SourceRef      string       `json:"source_ref"`
	TrustClass     string       `json:"trust_class"`
	TrustBasis     string       `json:"trust_basis,omitempty"`
	ArtifactSHA256 string       `json:"artifact_sha256"`
	ManifestSHA256 string       `json:"manifest_sha256"`
	Capabilities   []Capability `json:"capabilities"`
	InstallKey     string       `json:"install_key"`
	RequestedBy    string       `json:"requested_by"`
	ApprovalID     string       `json:"approval_id"`
	AdministeredBy string       `json:"administered_by"`
	State          string       `json:"state"`
	CreatedAt      time.Time    `json:"created_at"`
	ConfirmedAt    *time.Time   `json:"confirmed_at,omitempty"`
	EndedAt        *time.Time   `json:"ended_at,omitempty"`
	EndedBy        string       `json:"ended_by,omitempty"`
	CredentialGen  int          `json:"credential_generation"`
	// Upgrades: approved in-place upgrades (newest last); see upgrade.go.
	Upgrades []UpgradeEvent `json:"upgrades,omitempty"`
}

func (i Installation) HasCapability(c string) bool {
	for _, x := range i.Capabilities {
		if x.ID == c {
			return true
		}
	}
	return false
}

type insFile struct {
	Schema        string         `json:"schema"`
	Installations []Installation `json:"installations"`
}

type Installations struct {
	mu      sync.Mutex
	path    string
	list    []Installation
	loadErr string
	Now     func() time.Time
}

var (
	ErrInstallationsDamaged = errors.New("installations.json is damaged or unreadable; feature registration and checks are blocked until it is repaired")
	ErrNoInstallation       = errors.New("no such installation")
)

func OpenInstallations(dir string) *Installations {
	s := &Installations{path: filepath.Join(dir, installationsFile), Now: time.Now}
	b, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		s.loadErr = ErrInstallationsDamaged.Error()
	default:
		var f insFile
		if json.Unmarshal(b, &f) != nil || f.Schema != installationsSchema {
			s.loadErr = ErrInstallationsDamaged.Error()
		} else {
			s.list = f.Installations
		}
	}
	return s
}

func (s *Installations) LoadError() string { s.mu.Lock(); defer s.mu.Unlock(); return s.loadErr }

func (s *Installations) commit(next []Installation) error {
	if err := profile.WriteJSON(s.path, insFile{Schema: installationsSchema, Installations: next}); err != nil {
		return err
	}
	s.list = next
	return nil
}

// Reserve creates (or, for the same install key, returns) an installation.
// created reports whether a new ID was minted.
func (s *Installations) Reserve(req InstallRequest, approvalID, adminID, trustBasis string) (Installation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Installation{}, false, ErrInstallationsDamaged
	}
	for _, x := range s.list {
		if x.InstallKey == req.InstallKey {
			if x.FeatureID != req.FeatureID || x.ArtifactSHA256 != req.ArtifactSHA256 || x.RequestedBy != req.RequestedBy {
				return Installation{}, false, errors.New("that install key already belongs to a different installation request")
			}
			return x, false, nil
		}
	}
	in := Installation{ID: "ins_" + profile.RandHex(10), FeatureID: req.FeatureID, Name: req.Name, Version: req.Version, API: req.API,
		PublisherClaim: req.PublisherClaim, PublisherProof: "NONE — publisher name is a claim in the manifest; no signature was verified",
		SourceKind: req.SourceKind, SourceRef: req.SourceRef, TrustClass: req.TrustClass, TrustBasis: trustBasis,
		ArtifactSHA256: req.ArtifactSHA256, ManifestSHA256: req.ManifestSHA256, Capabilities: req.Capabilities,
		InstallKey: req.InstallKey, RequestedBy: req.RequestedBy, ApprovalID: approvalID, AdministeredBy: adminID,
		State: InsPending, CreatedAt: s.Now().UTC()}
	next := append(append([]Installation(nil), s.list...), in)
	if err := s.commit(next); err != nil {
		return Installation{}, false, err
	}
	return in, true, nil
}

func (s *Installations) idx(id string) int {
	for i, x := range s.list {
		if x.ID == id {
			return i
		}
	}
	return -1
}

func (s *Installations) Get(id string) (Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Installation{}, ErrInstallationsDamaged
	}
	i := s.idx(id)
	if i < 0 {
		return Installation{}, ErrNoInstallation
	}
	return s.list[i], nil
}

// Update applies fn to one installation and saves; rolled back on failure.
func (s *Installations) Update(id string, fn func(*Installation) error) (Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != "" {
		return Installation{}, ErrInstallationsDamaged
	}
	i := s.idx(id)
	if i < 0 {
		return Installation{}, ErrNoInstallation
	}
	next := append([]Installation(nil), s.list...)
	cp := next[i]
	if err := fn(&cp); err != nil {
		return Installation{}, err
	}
	next[i] = cp
	if err := s.commit(next); err != nil {
		return Installation{}, err
	}
	return cp, nil
}

func (s *Installations) List() []Installation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Installation(nil), s.list...)
}
