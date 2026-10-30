package authority

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func upgradeFixture(t *testing.T) (*Installations, *Grants, Installation, TrustPins, InstallRequest) {
	t.Helper()
	dir := t.TempDir()
	s := OpenInstallations(dir)
	g := OpenGrants(dir)
	req := InstallRequest{FeatureID: "p1-deterministic-worker", Name: "W", Version: "0.1.0", API: "thought.feature-api/1",
		SourceKind: "local-trusted-package", TrustClass: TrustLocalFixture, ArtifactSHA256: sha("v1"), ManifestSHA256: sha("m1"),
		InstallKey: "00112233445566778899aabbccddeeff", RequestedBy: "processor:tp_a",
		Capabilities: []Capability{{ID: "worker.keep", Description: "k"}, {ID: "worker.drop", Description: "d"}}}
	pins, err := NewTrustPins([]string{sha("v1"), sha("v2")})
	if err != nil {
		t.Fatal(err)
	}
	in, _, err := s.Reserve(req, "apr_1", "adm", pins.Basis(req.ArtifactSHA256))
	if err != nil {
		t.Fatal(err)
	}
	in, err = s.Update(in.ID, func(x *Installation) error { x.State, x.CredentialGen = InsInstalled, 1; return nil })
	if err != nil {
		t.Fatal(err)
	}
	to := req
	to.Version, to.ArtifactSHA256, to.ManifestSHA256 = "0.2.0", sha("v2"), sha("m2")
	to.Capabilities = []Capability{{ID: "worker.keep", Description: "k"}, {ID: "worker.new", Description: "n"}}
	return s, g, in, pins, to
}

func TestUpgradeValidationRules(t *testing.T) {
	_, _, in, pins, to := upgradeFixture(t)
	ok := UpgradeRequest{FromArtifactSHA256: in.ArtifactSHA256, To: to, DataMigration: MigrationNone, RequestedBy: "processor:tp_a"}
	if u := ok; u.Validate(in, pins) != nil {
		t.Fatal("valid upgrade refused")
	}
	cases := map[string]struct {
		mut  func(*UpgradeRequest, *Installation)
		want error
	}{
		"other program":       {func(u *UpgradeRequest, _ *Installation) { u.RequestedBy = "processor:tp_evil" }, ErrUpgradeOwner},
		"stale from-artifact": {func(u *UpgradeRequest, _ *Installation) { u.FromArtifactSHA256 = sha("old") }, ErrUpgradeStale},
		"unpinned new":        {func(u *UpgradeRequest, _ *Installation) { u.To.ArtifactSHA256 = sha("tampered") }, ErrNotPinned},
		"other feature":       {func(u *UpgradeRequest, _ *Installation) { u.To.FeatureID = "other-feature" }, ErrUpgradeFeature},
		"same artifact":       {func(u *UpgradeRequest, _ *Installation) { u.To.ArtifactSHA256 = sha("v1") }, ErrUpgradeSame},
		"irreversible":        {func(u *UpgradeRequest, _ *Installation) { u.DataMigration = "irreversible" }, ErrUpgradeIrreversible},
		"not installed":       {func(_ *UpgradeRequest, i *Installation) { i.State = InsUninstalled }, ErrUpgradeState},
		"third-party class":   {func(u *UpgradeRequest, _ *Installation) { u.To.TrustClass = "THIRD_PARTY" }, ErrUntrusted},
	}
	for name, c := range cases {
		u, cur := ok, in
		u.To.Capabilities = append([]Capability(nil), ok.To.Capabilities...)
		c.mut(&u, &cur)
		if err := u.Validate(cur, pins); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

func TestUpgradeKeepsIdentityAndRevokesDroppedGrants(t *testing.T) {
	s, g, in, pins, to := upgradeFixture(t)
	keep, _ := g.Create(GrantRequest{Requester: "processor:tp_a", Destination: in.ID, Subject: "ent_1", Capability: "worker.keep"}, "apr_g1", "adm")
	drop, _ := g.Create(GrantRequest{Requester: "processor:tp_a", Destination: in.ID, Subject: "ent_1", Capability: "worker.drop"}, "apr_g2", "adm")
	u := UpgradeRequest{FromArtifactSHA256: in.ArtifactSHA256, To: to, DataMigration: MigrationNone, RequestedBy: "processor:tp_a"}
	if err := u.Validate(in, pins); err != nil {
		t.Fatal(err)
	}
	added, removed, kept := CapabilityDiff(in.Capabilities, to.Capabilities)
	if len(added) != 1 || added[0] != "worker.new" || len(removed) != 1 || removed[0] != "worker.drop" || len(kept) != 1 {
		t.Fatalf("diff %v %v %v", added, removed, kept)
	}
	revoked, err := g.RevokeCapabilities(in.ID, removed, "admin-approval:apr_u", "dropped")
	if err != nil || len(revoked) != 1 || revoked[0] != drop.ID {
		t.Fatalf("revoked %v %v", revoked, err)
	}
	up, err := s.ApplyUpgrade(u, "apr_u", pins.Basis(to.ArtifactSHA256), revoked)
	if err != nil {
		t.Fatal(err)
	}
	if up.ID != in.ID || up.CredentialGen != in.CredentialGen || up.InstallKey != in.InstallKey || up.RequestedBy != in.RequestedBy ||
		up.State != InsInstalled || up.Version != "0.2.0" || up.ArtifactSHA256 != sha("v2") || len(up.Upgrades) != 1 {
		t.Fatalf("upgrade changed identity or missed fields: %+v", up)
	}
	if k, _ := g.Get(keep.ID); k.RevokedAt != nil {
		t.Fatal("grant for kept capability was revoked")
	}
	if d, _ := g.Get(drop.ID); d.RevokedAt == nil {
		t.Fatal("grant for dropped capability survived")
	}
	// Persisted: a fresh load sees the upgrade.
	if re, _ := OpenInstallations(filepath.Dir(s.path)).Get(in.ID); re.ArtifactSHA256 != sha("v2") || len(re.Upgrades) != 1 {
		t.Fatal("upgrade not persisted")
	}
	// Same approval again, or an upgrade that no longer starts from the
	// current artifact, is refused (no double apply).
	if _, err := s.ApplyUpgrade(u, "apr_u", "x", nil); !errors.Is(err, ErrUpgradeStale) {
		t.Fatalf("stale apply accepted: %v", err)
	}
}

func TestUpgradeRollbackRules(t *testing.T) {
	s, _, in, pins, to := upgradeFixture(t)
	u := UpgradeRequest{FromArtifactSHA256: in.ArtifactSHA256, To: to, DataMigration: MigrationReversible, RequestedBy: "processor:tp_a"}
	if err := u.Validate(in, pins); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyUpgrade(u, "apr_u", pins.Basis(to.ArtifactSHA256), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RollbackUpgrade(in.ID, "apr_u", "processor:tp_evil", "x", pins); !errors.Is(err, ErrRollbackRefused) {
		t.Fatal("rollback by another program accepted")
	}
	if _, err := s.RollbackUpgrade(in.ID, "apr_other", "processor:tp_a", "x", pins); !errors.Is(err, ErrRollbackRefused) {
		t.Fatal("rollback with the wrong approval accepted")
	}
	onlyNew, _ := NewTrustPins([]string{sha("v2")})
	if _, err := s.RollbackUpgrade(in.ID, "apr_u", "processor:tp_a", "x", onlyNew); !errors.Is(err, ErrRollbackRefused) {
		t.Fatal("rollback to an artifact that is no longer pinned accepted")
	}
	back, err := s.RollbackUpgrade(in.ID, "apr_u", "processor:tp_a", "health check failed", pins)
	if err != nil || back.ArtifactSHA256 != sha("v1") || back.Version != "0.1.0" || len(back.Capabilities) != 2 || back.ID != in.ID {
		t.Fatalf("rollback did not restore: %+v %v", back, err)
	}
	if _, err := s.RollbackUpgrade(in.ID, "apr_u", "processor:tp_a", "again", pins); !errors.Is(err, ErrRollbackRefused) {
		t.Fatal("double rollback accepted")
	}
	// Window: an old upgrade can't be rolled back.
	s2, _, in2, pins2, to2 := upgradeFixture(t)
	u2 := UpgradeRequest{FromArtifactSHA256: in2.ArtifactSHA256, To: to2, DataMigration: MigrationNone, RequestedBy: "processor:tp_a"}
	u2.Validate(in2, pins2)
	s2.ApplyUpgrade(u2, "apr_u2", "b", nil)
	s2.Now = func() time.Time { return time.Now().Add(RollbackWindow + time.Minute) }
	if _, err := s2.RollbackUpgrade(in2.ID, "apr_u2", "processor:tp_a", "late", pins2); !errors.Is(err, ErrRollbackRefused) {
		t.Fatal("rollback after the window accepted")
	}
}
