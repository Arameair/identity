package authority

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func init() { SetIterationsForTests(1000) }

// RFC 7914 §11 PBKDF2-HMAC-SHA256 test vectors.
func TestPBKDF2Vectors(t *testing.T) {
	got := hex.EncodeToString(pbkdf2SHA256([]byte("passwd"), []byte("salt"), 1, 64))
	want := "55ac046e56e3089fec1691c22544b605f94185216dde0465e68b9d57c20dacbc49ca9cccf179b645991664b39d77ef317c71b845b1e30bd509112041d3a19783"
	if got != want {
		t.Fatalf("vector 1: %s", got)
	}
	got = hex.EncodeToString(pbkdf2SHA256([]byte("Password"), []byte("NaCl"), 80000, 64))
	want = "4ddcd8f60b98be21830cee5ef22701f9641a4418d04c0414aeff08876b34ab56a1d425a1225833549adb841b51c9b3176a272bdebba1d078478f62b397f33c8d"
	if got != want {
		t.Fatalf("vector 2: %s", got)
	}
}

func binding(id int64) GitHubBinding {
	return GitHubBinding{ID: id, Login: "owner", Mode: "TEST FIXTURE", VerifiedAt: time.Now(), ProofMethod: "test"}
}

func TestAdminEnrollOnceRaceAndPersist(t *testing.T) {
	dir := t.TempDir()
	s := OpenAdmin(dir)
	if st, _, _ := s.State(); st != "NOT_ENROLLED" {
		t.Fatal(st)
	}
	if _, err := s.Enroll("idn_x", "apr_1", binding(7), "short"); !errors.Is(err, ErrWeakPassphrase) {
		t.Fatalf("weak passphrase accepted: %v", err)
	}
	if _, err := s.Enroll("idn_x", "apr_1", binding(0), "correct horse battery"); err == nil {
		t.Fatal("enrollment without a GitHub-verified account")
	}
	// Two racing enrollments (two Identity processes can't share a profile,
	// but two stores on the same folder model the worst case): exactly one wins.
	s2 := OpenAdmin(dir)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, st := range []*AdminStore{s, s2} {
		wg.Add(1)
		go func(i int, st *AdminStore) {
			defer wg.Done()
			_, errs[i] = st.Enroll("idn_x", "apr_1", binding(int64(7+i)), "correct horse battery")
		}(i, st)
	}
	wg.Wait()
	wins := 0
	for _, e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, ErrAdminExists) {
			t.Fatalf("unexpected error: %v", e)
		}
	}
	if wins != 1 {
		t.Fatalf("%d enrollments won", wins)
	}
	r := OpenAdmin(dir)
	a, err := r.Get()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Enroll("idn_x", "apr_2", binding(99), "another long passphrase"); !errors.Is(err, ErrAdminExists) {
		t.Fatalf("second enrollment after restart: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, adminFile))
	if contains(string(b), "correct horse") {
		t.Fatal("passphrase stored in clear")
	}
	if a.GitHub.ID != 7 && a.GitHub.ID != 8 {
		t.Fatal("binding lost")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestDamagedAdminIsNotTreatedAsMissing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, adminFile), []byte("{broken"), 0o600)
	s := OpenAdmin(dir)
	if st, _, _ := s.State(); st != "DAMAGED" {
		t.Fatal(st)
	}
	if _, err := s.Enroll("idn_x", "apr", binding(1), "correct horse battery"); !errors.Is(err, ErrAdminDamaged) {
		t.Fatalf("enrollment over a damaged admin record: %v", err)
	}
	if _, err := s.Authenticate("anything at all"); !errors.Is(err, ErrAdminDamaged) {
		t.Fatal(err)
	}
}

func TestAuthenticateThrottlePersists(t *testing.T) {
	dir := t.TempDir()
	s := OpenAdmin(dir)
	if _, err := s.Enroll("idn_x", "apr", binding(1), "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxFailures; i++ {
		if _, err := s.Authenticate("wrong passphrase!!"); !errors.Is(err, ErrBadPassphrase) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, err := s.Authenticate("correct horse battery"); !errors.Is(err, ErrLockedOut) {
		t.Fatalf("not locked out: %v", err)
	}
	if _, err := OpenAdmin(dir).Authenticate("correct horse battery"); !errors.Is(err, ErrLockedOut) {
		t.Fatalf("lockout lost on restart: %v", err)
	}
	s.Now = func() time.Time { return time.Now().Add(baseLockout + time.Second) }
	if _, err := s.Authenticate("correct horse battery"); err != nil {
		t.Fatalf("after lockout: %v", err)
	}
}

func TestApprovalLifecycle(t *testing.T) {
	dir := t.TempDir()
	s := OpenApprovals(dir)
	ap, tok, err := s.Create(KindGrant, map[string]string{"x": "1"}, []string{"do x"}, "processor:tp_a")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Begin(ap.ID, KindGrant, "processor:tp_a"); !errors.Is(err, ErrApprovalState) {
		t.Fatalf("pending approval usable: %v", err)
	}
	if _, err := s.OpenView(ap.ID, "bad"); err == nil {
		t.Fatal("bad view token")
	}
	if _, err := s.OpenView(ap.ID, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenView(ap.ID, tok); err == nil {
		t.Fatal("view token reused")
	}
	if _, err := s.Decide(ap.ID, true, "adm_1", "test"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Begin(ap.ID, KindInstall, "processor:tp_a"); !errors.Is(err, ErrApprovalKind) {
		t.Fatal("wrong kind accepted")
	}
	if _, _, err := s.Begin(ap.ID, KindGrant, "processor:tp_b"); !errors.Is(err, ErrApprovalOwner) {
		t.Fatal("other requester accepted")
	}
	// Tampering with the stored action is detected.
	r := OpenApprovals(dir)
	r.list[0].Action = []byte(`{"x":"2"}`)
	if _, _, err := r.Begin(ap.ID, KindGrant, "processor:tp_a"); err == nil {
		t.Fatal("tampered action accepted")
	}
	got, fresh, err := s.Begin(ap.ID, KindGrant, "processor:tp_a")
	if err != nil || !fresh || got.State != Consumed {
		t.Fatal(err)
	}
	if _, _, err := s.Begin(ap.ID, KindGrant, "processor:tp_a"); !errors.Is(err, ErrApprovalUncertain) {
		t.Fatalf("crashed-before-finish approval reused: %v", err)
	}
	s.Finish(ap.ID, "gnt_x", "GRANTED")
	if got, fresh, err := s.Begin(ap.ID, KindGrant, "processor:tp_a"); err != nil || fresh || got.Result != "gnt_x" {
		t.Fatal("idempotent replay should return the recorded result, not run again")
	}
	// Expiry.
	ap2, _, _ := s.Create(KindGrant, map[string]string{"y": "1"}, nil, "processor:tp_a")
	s.Decide(ap2.ID, true, "adm_1", "test")
	s.Now = func() time.Time { return time.Now().Add(ApprovalTTL + time.Second) }
	if _, _, err := s.Begin(ap2.ID, KindGrant, "processor:tp_a"); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("expired approval usable: %v", err)
	}
	// Damaged file fails closed.
	os.WriteFile(filepath.Join(dir, approvalsFile), []byte("nope"), 0o600)
	if _, _, err := OpenApprovals(dir).Create(KindGrant, 1, nil, "x"); !errors.Is(err, ErrApprovalsDamaged) {
		t.Fatal("damaged approvals usable")
	}
}

func TestGrantTicketCheck(t *testing.T) {
	dir := t.TempDir()
	g := OpenGrants(dir)
	key := []byte("0123456789abcdef0123456789abcdef")
	gr, _ := g.Create(GrantRequest{Requester: "processor:tp_a", Destination: "ins_1", Subject: "ent_1", Capability: "worker.test.synthetic"}, "apr", "adm")
	if _, why := g.FindActive("processor:tp_b", "ins_1", "ent_1", "worker.test.synthetic"); why != DenyNoGrant {
		t.Fatal("other requester found a grant")
	}
	tk, p, err := g.Issue(gr, "idn", "d1", key)
	if err != nil {
		t.Fatal(err)
	}
	if pp, err := ParseTicket(tk, key); err != nil || pp.TicketID != p.TicketID {
		t.Fatal("ticket does not verify")
	}
	if _, err := ParseTicket(tk, []byte("other key other key other key 00")); err == nil {
		t.Fatal("ticket verifies with another key")
	}
	cr := CheckRequest{TicketID: p.TicketID, GrantID: gr.ID, Requester: "processor:tp_a", Capability: "worker.test.synthetic", Subject: "ent_1", Digest: "d1"}
	if _, why := g.Check("ins_2", cr); why != DenyWrongAudience {
		t.Fatal(why)
	}
	bad := cr
	bad.Digest = "d2"
	if _, why := g.Check("ins_1", bad); why != DenyMismatch {
		t.Fatal(why)
	}
	if _, why := g.Check("ins_1", cr); why != "" {
		t.Fatal(why)
	}
	if _, why := g.Check("ins_1", cr); why != DenyTicketReplayed {
		t.Fatal("replay:", why)
	}
	_, p2, _ := g.Issue(gr, "idn", "d3", key)
	g.Now = func() time.Time { return time.Now().Add(TicketTTL + time.Second) }
	cr.TicketID, cr.Digest = p2.TicketID, "d3"
	if _, why := g.Check("ins_1", cr); why != DenyTicketExpired {
		t.Fatal("expired ticket:", why)
	}
	g.Now = time.Now
	_, p3, _ := g.Issue(gr, "idn", "d4", key)
	g.Revoke(gr.ID, "test", "test")
	cr.TicketID, cr.Digest = p3.TicketID, "d4"
	if _, why := g.Check("ins_1", cr); why != DenyTicketUnknown {
		t.Fatal("ticket survived revocation:", why)
	}
	if _, why := g.FindActive("processor:tp_a", "ins_1", "ent_1", "worker.test.synthetic"); why != DenyRevoked {
		t.Fatal(why)
	}
	// Persisted; tickets are not.
	g2 := OpenGrants(dir)
	if gg, _ := g2.Get(gr.ID); gg.Status(time.Now()) != "REVOKED" || gg.Version != 2 {
		t.Fatal("revocation not persisted")
	}
	// Expiring grant.
	exp := time.Now().Add(10 * time.Second)
	ge, _ := g2.Create(GrantRequest{Requester: "processor:tp_a", Destination: "ins_1", Subject: "ent_2", Capability: "worker.test.synthetic", ExpiresAt: &exp}, "apr", "adm")
	g2.Now = func() time.Time { return exp.Add(time.Second) }
	if _, why := g2.FindActive("processor:tp_a", "ins_1", "ent_2", "worker.test.synthetic"); why != DenyExpired {
		t.Fatal("expired grant active:", why, ge.ID)
	}
	os.WriteFile(filepath.Join(dir, grantsFile), []byte("x"), 0o600)
	if _, why := OpenGrants(dir).Check("ins_1", cr); why != "GRANTS_UNAVAILABLE" {
		t.Fatal("damaged grants allowed something")
	}
}

func TestInstallReserveIdempotentAndTrustRule(t *testing.T) {
	dir := t.TempDir()
	s := OpenInstallations(dir)
	req := InstallRequest{FeatureID: "p1-deterministic-worker", Name: "W", Version: "0.1.0", API: "thought.feature-api/1",
		SourceKind: "local-trusted-package", TrustClass: TrustLocalFixture, ArtifactSHA256: sha("a"), ManifestSHA256: sha("b"),
		InstallKey: "00112233445566778899aabbccddeeff", RequestedBy: "processor:tp_a",
		Capabilities: []Capability{{ID: "worker.test.synthetic", Description: "x", DataClass: "SYNTHETIC"}}}
	pins, err := NewTrustPins([]string{sha("a")})
	if err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(pins); err != nil {
		t.Fatal(err)
	}
	u := req
	u.TrustClass = "THIRD_PARTY"
	if err := u.Validate(pins); !errors.Is(err, ErrUntrusted) {
		t.Fatal("untrusted class accepted")
	}
	u = req
	u.SourceKind = "github-release"
	if err := u.Validate(pins); !errors.Is(err, ErrUntrusted) {
		t.Fatal("download source accepted")
	}
	// Grok M1: claiming the fixture trust class is not enough; the exact
	// artifact must be pinned.
	u = req
	u.ArtifactSHA256 = sha("new code that calls itself a fixture")
	if err := u.Validate(pins); !errors.Is(err, ErrNotPinned) {
		t.Fatalf("unpinned artifact with a self-declared fixture class accepted: %v", err)
	}
	if _, err := NewTrustPins([]string{"not-a-digest"}); err == nil {
		t.Fatal("bad operator pin accepted")
	}
	a, created, err := s.Reserve(req, "apr_1", "adm", pins.Basis(req.ArtifactSHA256))
	if err != nil || !created {
		t.Fatal(err)
	}
	b, created, err := OpenInstallations(dir).Reserve(req, "apr_2", "adm", "")
	if err != nil || created || b.ID != a.ID {
		t.Fatal("retry minted a second installation")
	}
	req2 := req
	req2.InstallKey = "ffeeddccbbaa99887766554433221100"
	c, created, _ := s.Reserve(req2, "apr_3", "adm", "")
	if !created || c.ID == a.ID {
		t.Fatal("second copy must get its own ID")
	}
	clash := req
	clash.ArtifactSHA256 = sha("other")
	if _, _, err := s.Reserve(clash, "apr_4", "adm", ""); err == nil {
		t.Fatal("install key reused for a different artifact")
	}
}

func sha(s string) string {
	h := [32]byte{}
	copy(h[:], s)
	return hex.EncodeToString(h[:])
}
