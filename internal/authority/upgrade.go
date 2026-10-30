package authority

// Installation upgrade (identity.installation-upgrade/1) — EXPERIMENTAL.
//
// Moves ONE existing installation from the exact artifact it runs now to a
// new, pinned artifact of the SAME feature, keeping its installation ID and
// credential (continuity). It is an elevated action: an Admin approval of
// kind upgrade_feature, requested by the program that installed it, bound to
// the exact from/to digests. Grants for capabilities the new version no
// longer declares are revoked at commit; grants for kept capabilities stay;
// new capabilities get NO grant (each needs its own approval).
//
// Rollback restores the exact previous record (still pinned), only for the
// latest upgrade, only by the same requester, only within RollbackWindow.
// It never restores revoked grants: revocation is final.

import (
	"errors"
	"sort"
	"time"
)

const (
	KindUpgrade    = "upgrade_feature"
	UpgradeAPI     = "identity.installation-upgrade/1"
	RollbackWindow = 30 * time.Minute
	maxUpgradeLog  = 20
)

// Data migration declared by the new version. Identity refuses anything that
// can't be undone; recovery requirements are not satisfiable for those yet.
const (
	MigrationNone       = "none"
	MigrationReversible = "reversible"
)

var (
	ErrUpgradeState        = errors.New("UPGRADE_REFUSED: only an INSTALLED installation can be upgraded")
	ErrUpgradeOwner        = errors.New("UPGRADE_REFUSED: only the program that installed this feature can upgrade it")
	ErrUpgradeStale        = errors.New("UPGRADE_CONFLICT: the installation is no longer at the artifact this upgrade starts from (another upgrade or rollback happened)")
	ErrUpgradeFeature      = errors.New("UPGRADE_REFUSED: the new package is a different feature")
	ErrUpgradeAPI          = errors.New("UPGRADE_REFUSED: the new package speaks a different feature API")
	ErrUpgradeSame         = errors.New("UPGRADE_REFUSED: that is the artifact already installed")
	ErrUpgradeIrreversible = errors.New("UPGRADE_BLOCKED: the new version declares a data migration that can't be undone; Identity refuses it until its recovery requirements exist")
	ErrRollbackRefused     = errors.New("ROLLBACK_REFUSED: no recent upgrade with that approval can be rolled back")
)

type UpgradeRequest struct {
	InstallationID     string         `json:"installation_id"`
	FromArtifactSHA256 string         `json:"from_artifact_sha256"`
	To                 InstallRequest `json:"to"`
	DataMigration      string         `json:"data_migration"`
	SecurityFix        bool           `json:"security_fix"`
	// Entitlement is what the requester says about any non-security
	// entitlement (future: a separately authorized payment decision). It is
	// recorded for the Admin to see and NEVER grants or replaces authority.
	Entitlement string `json:"entitlement,omitempty"`
	RequestedBy string `json:"requested_by"`
}

// InstallSnapshot is everything an upgrade changes, so rollback is exact.
type InstallSnapshot struct {
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	PublisherClaim string       `json:"publisher_claim"`
	SourceKind     string       `json:"source_kind"`
	SourceRef      string       `json:"source_ref"`
	TrustClass     string       `json:"trust_class"`
	TrustBasis     string       `json:"trust_basis,omitempty"`
	ArtifactSHA256 string       `json:"artifact_sha256"`
	ManifestSHA256 string       `json:"manifest_sha256"`
	Capabilities   []Capability `json:"capabilities"`
}

type UpgradeEvent struct {
	ApprovalID     string          `json:"approval_id"`
	At             time.Time       `json:"at"`
	FromVersion    string          `json:"from_version"`
	ToVersion      string          `json:"to_version"`
	FromArtifact   string          `json:"from_artifact_sha256"`
	ToArtifact     string          `json:"to_artifact_sha256"`
	DataMigration  string          `json:"data_migration"`
	SecurityFix    bool            `json:"security_fix,omitempty"`
	Entitlement    string          `json:"entitlement,omitempty"`
	GrantsRevoked  []string        `json:"grants_revoked,omitempty"`
	Previous       InstallSnapshot `json:"previous"`
	RolledBackAt   *time.Time      `json:"rolled_back_at,omitempty"`
	RollbackReason string          `json:"rollback_reason,omitempty"`
}

func snapshot(i Installation) InstallSnapshot {
	return InstallSnapshot{Name: i.Name, Version: i.Version, PublisherClaim: i.PublisherClaim, SourceKind: i.SourceKind, SourceRef: i.SourceRef,
		TrustClass: i.TrustClass, TrustBasis: i.TrustBasis, ArtifactSHA256: i.ArtifactSHA256, ManifestSHA256: i.ManifestSHA256,
		Capabilities: append([]Capability(nil), i.Capabilities...)}
}

func restore(i *Installation, s InstallSnapshot) {
	i.Name, i.Version, i.PublisherClaim, i.SourceKind, i.SourceRef = s.Name, s.Version, s.PublisherClaim, s.SourceKind, s.SourceRef
	i.TrustClass, i.TrustBasis, i.ArtifactSHA256, i.ManifestSHA256 = s.TrustClass, s.TrustBasis, s.ArtifactSHA256, s.ManifestSHA256
	i.Capabilities = append([]Capability(nil), s.Capabilities...)
}

// Validate checks an upgrade against the installation as it is now.
func (u *UpgradeRequest) Validate(cur Installation, pins TrustPins) error {
	switch {
	case cur.State != InsInstalled:
		return ErrUpgradeState
	case u.RequestedBy == "" || u.RequestedBy != cur.RequestedBy:
		return ErrUpgradeOwner
	case u.FromArtifactSHA256 != cur.ArtifactSHA256:
		return ErrUpgradeStale
	case u.To.FeatureID != cur.FeatureID:
		return ErrUpgradeFeature
	case u.To.API != cur.API:
		return ErrUpgradeAPI
	case u.To.ArtifactSHA256 == cur.ArtifactSHA256:
		return ErrUpgradeSame
	case u.DataMigration != MigrationNone && u.DataMigration != MigrationReversible:
		return ErrUpgradeIrreversible
	}
	u.InstallationID = cur.ID
	u.To.RequestedBy, u.To.InstallKey = u.RequestedBy, cur.InstallKey
	return u.To.Validate(pins) // trust class, digests, capability ids, and: the new artifact must be pinned
}

// CapabilityDiff says what an upgrade changes about declared capabilities.
func CapabilityDiff(old, new []Capability) (added, removed, kept []string) {
	o, n := map[string]bool{}, map[string]bool{}
	for _, c := range old {
		o[c.ID] = true
	}
	for _, c := range new {
		n[c.ID] = true
		if o[c.ID] {
			kept = append(kept, c.ID)
		} else {
			added = append(added, c.ID)
		}
	}
	for _, c := range old {
		if !n[c.ID] {
			removed = append(removed, c.ID)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(kept)
	return
}

// ApplyUpgrade commits an approved upgrade atomically. ID, credential
// generation, state, requester and install key are untouched.
func (s *Installations) ApplyUpgrade(u UpgradeRequest, approvalID, basis string, revoked []string) (Installation, error) {
	return s.Update(u.InstallationID, func(x *Installation) error {
		if x.State != InsInstalled {
			return ErrUpgradeState
		}
		if x.ArtifactSHA256 != u.FromArtifactSHA256 {
			return ErrUpgradeStale
		}
		ev := UpgradeEvent{ApprovalID: approvalID, At: s.Now().UTC(), FromVersion: x.Version, ToVersion: u.To.Version,
			FromArtifact: x.ArtifactSHA256, ToArtifact: u.To.ArtifactSHA256, DataMigration: u.DataMigration, SecurityFix: u.SecurityFix,
			Entitlement: u.Entitlement, GrantsRevoked: revoked, Previous: snapshot(*x)}
		t := u.To
		restore(x, InstallSnapshot{Name: clean(t.Name, 80), Version: clean(t.Version, 40), PublisherClaim: clean(t.PublisherClaim, 80),
			SourceKind: t.SourceKind, SourceRef: clean(t.SourceRef, 200), TrustClass: t.TrustClass, TrustBasis: basis,
			ArtifactSHA256: t.ArtifactSHA256, ManifestSHA256: t.ManifestSHA256, Capabilities: t.Capabilities})
		x.Upgrades = append(x.Upgrades, ev)
		if len(x.Upgrades) > maxUpgradeLog {
			x.Upgrades = x.Upgrades[len(x.Upgrades)-maxUpgradeLog:]
		}
		return nil
	})
}

// RollbackUpgrade restores the record from before the latest upgrade.
func (s *Installations) RollbackUpgrade(id, approvalID, requester, reason string, pins TrustPins) (Installation, error) {
	return s.Update(id, func(x *Installation) error {
		n := len(x.Upgrades)
		if n == 0 || x.State != InsInstalled || requester != x.RequestedBy {
			return ErrRollbackRefused
		}
		ev := &x.Upgrades[n-1]
		switch {
		case ev.ApprovalID != approvalID, ev.RolledBackAt != nil, x.ArtifactSHA256 != ev.ToArtifact,
			s.Now().Sub(ev.At) > RollbackWindow, pins.Basis(ev.Previous.ArtifactSHA256) == "":
			return ErrRollbackRefused
		}
		restore(x, ev.Previous)
		t := s.Now().UTC()
		ev.RolledBackAt, ev.RollbackReason = &t, clean(reason, 160)
		return nil
	})
}

// RevokeCapabilities revokes ACTIVE grants whose destination is the
// installation and whose capability is in caps. Returns the grant IDs.
func (s *Grants) RevokeCapabilities(installationID string, caps []string, by, reason string) ([]string, error) {
	if len(caps) == 0 {
		return nil, nil
	}
	drop := map[string]bool{}
	for _, c := range caps {
		drop[c] = true
	}
	s.mu.Lock()
	ids := []string{}
	for _, g := range s.list {
		if g.RevokedAt == nil && g.Destination == installationID && drop[g.Capability] {
			ids = append(ids, g.ID)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		if _, err := s.Revoke(id, by, reason); err != nil {
			return ids, err
		}
	}
	return ids, nil
}
