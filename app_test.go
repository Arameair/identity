package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const fakeCommit = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"

// fakeTC is an in-memory stand-in for a Thought Controller host. Tests only.
type fakeTC struct {
	mu      sync.Mutex
	up      bool
	app     string
	commit  string
	iid     string
	sid     string
	aid     string
	starts  int
	reqs    []StartRequest
	entites int
}

func newFake() *fakeTC { return &fakeTC{up: true, app: "thought-controller", commit: fakeCommit} }

func (f *fakeTC) probe(addr string) (*Health, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.up {
		return nil, errors.New("connection refused")
	}
	h := &Health{App: f.app, Protocol: tcProtocol, Simulated: true}
	h.Build.Commit, h.Build.Branch = f.commit, "experiment/fake"
	h.Service.State = "not_started"
	if f.iid != "" {
		h.Service.State, h.Service.InstallationID, h.Service.ServiceID, h.Service.AdminID, h.Service.Entities = "ready", f.iid, f.sid, f.aid, f.entites
	}
	return h, nil
}

func (f *fakeTC) start(addr string, r StartRequest) (StartReply, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, r)
	if r.ApprovedCommit != f.commit {
		return StartReply{Result: "refused", Reason: "version mismatch"}, 409, nil
	}
	if f.iid != "" {
		if f.iid != r.InstallationID {
			return StartReply{Result: "refused", Reason: "busy"}, 409, nil
		}
		return StartReply{Result: "already_running"}, 200, nil
	}
	f.starts++
	f.iid, f.sid, f.aid, f.entites = r.InstallationID, r.ServiceID, r.AdminID, len(r.Entities)
	return StartReply{Result: "started", Simulated: true}, 200, nil
}

func (f *fakeTC) stopService() { f.mu.Lock(); f.iid = ""; f.mu.Unlock() }

func testApp(t *testing.T, dir string, f *fakeTC) *App {
	t.Helper()
	a, err := openApp(Config{DataDir: dir, Port: "0", Commit: "test", Branch: "test"})
	if err != nil {
		t.Fatal(err)
	}
	a.probe, a.start = f.probe, f.start
	a.reg = func(string, any) error { return nil }
	a.stop = func(string, string) error { f.stopService(); return nil }
	a.scan = func(string) []ScanHit { return nil }
	t.Cleanup(a.Close)
	return a
}

func withSuggested(t *testing.T, commit string) {
	old := suggestedTarget.Commit
	suggestedTarget.Commit = commit
	t.Cleanup(func() { suggestedTarget.Commit = old })
}

func TestInstallationIDsAreDistinctAndRestored(t *testing.T) {
	dir := t.TempDir()
	f := newFake()
	a := testApp(t, dir, f)
	if a.p.Inst != nil {
		t.Fatal("a fresh profile must have no installation")
	}
	inst, err := a.CreateInstallation("  Owner  ")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{inst.InstallationID: true, inst.AdminID: true, inst.ControllerServiceID: true}
	if len(ids) != 3 {
		t.Fatal("installation, administrator and service IDs must differ")
	}
	if _, err := a.CreateInstallation("again"); err == nil {
		t.Fatal("second installation in the same profile must be refused")
	}
	if _, err := a.StarterIdentities(); err != nil {
		t.Fatal(err)
	}
	if made, _ := a.StarterIdentities(); len(made) != 0 {
		t.Fatal("starter identities must not be created twice")
	}
	adm, svc, man := a.p.Store.ByRole("administrator"), a.p.Store.ByRole("system_service"), a.p.Store.ByRole("managed")
	if len(adm) != 1 || adm[0].ID != inst.AdminID || len(svc) != 1 || svc[0].ID != inst.ControllerServiceID || len(man) != 2 {
		t.Fatalf("records: %d admin, %d service, %d managed", len(adm), len(svc), len(man))
	}
	if _, err := a.CreateIdentity("x", "administrator", "", false); err == nil {
		t.Fatal("a second administrator can't be created as a managed identity")
	}
	a.SetSetupStep(4, false)
	a.Close()

	b := testApp(t, dir, f) // restart
	if b.p.Inst == nil || *b.p.Inst != *inst && b.p.Inst.InstallationID != inst.InstallationID {
		t.Fatal("installation not restored")
	}
	if b.p.Inst.AdminID != inst.AdminID || b.p.Inst.ControllerServiceID != inst.ControllerServiceID || b.p.Inst.SetupStep != 4 {
		t.Fatalf("restored installation differs: %+v", b.p.Inst)
	}
	if got := b.p.Store.ByRole("managed"); len(got) != 2 || got[0].ID != man[0].ID {
		t.Fatal("managed identities not restored with the same IDs")
	}
}

func TestNewTestProfileIsIsolated(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t, dir, newFake())
	first, _ := a.CreateInstallation("Owner")
	a.StarterIdentities()
	firstProfile := a.p.Name
	name, err := a.NewTestProfile()
	if err != nil || a.p.Name != name || a.p.Inst != nil || len(a.p.Store.List()) != 0 {
		t.Fatalf("new profile should be empty: %v", err)
	}
	second, _ := a.CreateInstallation("Second admin")
	a.StarterIdentities()
	if second.InstallationID == first.InstallationID || second.AdminID == first.AdminID || second.ControllerServiceID == first.ControllerServiceID {
		t.Fatal("second installation must get entirely new IDs")
	}
	for _, x := range a.p.Store.List() {
		if x.InstallationID != second.InstallationID {
			t.Fatal("second profile sees a record from the first")
		}
	}
	if err := a.SwitchProfile(firstProfile); err != nil || a.p.Inst.InstallationID != first.InstallationID {
		t.Fatal("switching back must restore the first installation")
	}
	if err := a.SwitchProfile("../etc"); err == nil {
		t.Fatal("bad profile name must be refused")
	}
	// Active profile survives a restart.
	a.Close()
	b := testApp(t, dir, newFake())
	if b.p.Name != firstProfile {
		t.Fatalf("active profile not remembered: %s", b.p.Name)
	}
}

func TestConsentAndSimulatedLauncher(t *testing.T) {
	withSuggested(t, fakeCommit)
	dir := t.TempDir()
	f := newFake()
	a := testApp(t, dir, f)
	inst, _ := a.CreateInstallation("Owner")
	a.StarterIdentities()

	r, _ := a.Check("t", true)
	if r.State != "NOT_RUNNING" || r.Attempt != nil || f.starts != 0 {
		t.Fatalf("no consent: must not start: %+v", r)
	}
	a.Decide("deny")
	if r, _ = a.Check("t", true); r.Attempt != nil || f.starts != 0 {
		t.Fatal("declined: must not start")
	}
	if err := a.Decide("approve"); err != nil {
		t.Fatal(err)
	}
	if a.p.Controller.Target == nil || a.p.Controller.Target.Commit != fakeCommit {
		t.Fatal("approving pins the suggested build")
	}
	r, _ = a.Check("approval", true)
	if r.State != "VERIFIED" || r.Attempt == nil || r.Attempt.Result != "started" || f.starts != 1 || !r.CanOpen {
		t.Fatalf("approve: exactly one simulated start, then VERIFIED: %+v", r)
	}
	if len(f.reqs[0].Entities) != 2 || f.reqs[0].ServiceID != inst.ControllerServiceID || f.reqs[0].ConsentID == "" {
		t.Fatalf("start request must carry Identity's IDs and identities: %+v", f.reqs[0])
	}
	if r, _ = a.Check("again", true); r.State != "VERIFIED" || r.Attempt != nil || f.starts != 1 {
		t.Fatal("already running: reuse, no new request")
	}
	// Controller stopped (e.g. host restarted): saved consent is reused, no prompt.
	f.stopService()
	if r, _ = a.Check("restart", true); r.State != "VERIFIED" || r.Attempt == nil || f.starts != 2 {
		t.Fatalf("saved consent should restart once: %+v", r)
	}
	// Concurrent checks while stopped: one start only.
	f.stopService()
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); a.Check("concurrent", true) }()
	}
	wg.Wait()
	if f.starts != 3 {
		t.Fatalf("concurrent checks must start once, starts=%d", f.starts)
	}
	// Host gone: no attempt, honest NOT_FOUND.
	f.mu.Lock()
	f.up = false
	f.mu.Unlock()
	n := len(f.reqs)
	if r, _ = a.Check("gone", true); r.State != "NOT_FOUND" || r.Attempt != nil || len(f.reqs) != n || !strings.Contains(r.ConsentMsg, "already running") {
		t.Fatalf("not found: %+v", r)
	}
	f.mu.Lock()
	f.up = true
	f.mu.Unlock()
	// A different pinned build needs fresh approval and is a version mismatch.
	f.stopService()
	other := "0123456789012345678901234567890123456789"
	if _, err := a.SetTarget("experiment/other", other, false); err != nil {
		t.Fatal(err)
	}
	if a.p.Controller.Consent.validFor(a.p.Controller.Target) {
		t.Fatal("changing the target must invalidate the approval")
	}
	if r, _ = a.Check("changed", true); r.State != "VERSION_MISMATCH" || r.Attempt != nil {
		t.Fatalf("changed target: %+v", r)
	}
	// Re-approving the other build: the running host refuses (wrong commit); we never pretend.
	a.SetTarget("x", fakeCommit, false)
	a.Decide("approve")
	a.Decide("revoke")
	if r, _ = a.Check("revoked", true); r.Attempt != nil || r.State != "NOT_RUNNING" {
		t.Fatalf("revoked: %+v", r)
	}
	if a.p.Controller.Counters.Started != 3 {
		t.Fatalf("counters: %+v", a.p.Controller.Counters)
	}
}

func TestWrongInstanceAndIncompatible(t *testing.T) {
	withSuggested(t, fakeCommit)
	f := newFake()
	a := testApp(t, t.TempDir(), f)
	a.CreateInstallation("Owner")
	a.Decide("approve")
	f.iid, f.sid, f.aid = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"
	if r, _ := a.Check("t", true); r.State != "WRONG_INSTANCE" || r.Attempt != nil || r.CanOpen {
		t.Fatalf("wrong instance: %+v", r)
	}
	f.iid = ""
	f.app = "identity"
	if r, _ := a.Check("t", true); r.State != "INCOMPATIBLE" || r.Attempt != nil {
		t.Fatalf("incompatible: %+v", r)
	}
}

func TestConsentSurvivesRestartAndStartupUsesIt(t *testing.T) {
	withSuggested(t, fakeCommit)
	dir := t.TempDir()
	f := newFake()
	a := testApp(t, dir, f)
	a.CreateInstallation("Owner")
	a.Decide("approve")
	a.Check("approval", true)
	a.Close()
	f.stopService() // everything stopped
	b := testApp(t, dir, f)
	if !b.p.Controller.Consent.validFor(b.p.Controller.Target) {
		t.Fatal("consent must persist")
	}
	if r, _ := b.Check("Identity startup", true); r.State != "VERIFIED" || f.starts != 2 {
		t.Fatalf("restart should start once without asking: %+v", r)
	}
	if b.p.Controller.Counters.StartRequests != 2 || len(b.p.Controller.Attempts) != 2 {
		t.Fatal("attempts must persist across restarts")
	}
}

func TestAddressMustBeLocal(t *testing.T) {
	a := testApp(t, t.TempDir(), newFake())
	for _, bad := range []string{"example.com:80", "10.0.0.1:7797", "127.0.0.1", "127.0.0.1:99999"} {
		if err := a.SetAddress(bad); err == nil {
			t.Errorf("%s should be refused", bad)
		}
	}
	if err := a.SetAddress("http://localhost:7801/"); err != nil || a.p.Controller.Address != "localhost:7801" {
		t.Fatal("loopback address should be accepted and normalised")
	}
}

func TestInstallFailureWritesNothing(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t, dir, newFake())
	a.failNext = true
	if _, err := a.CreateInstallation("Owner"); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(filepath.Join(a.p.Dir, "installation.json")); !os.IsNotExist(err) || len(a.p.Store.List()) != 0 {
		t.Fatal("nothing may be written on a failed install")
	}
	if _, err := a.CreateInstallation("Owner"); err != nil {
		t.Fatal("next attempt should work")
	}
}

func TestDataFolderNeverV1(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	git := func(a ...string) (string, error) { return "", errors.New("no git") }
	c, _ := resolveConfig("", env(map[string]string{"PC_DATA": "/pc/b.data"}), git, "/home/u")
	if c.DataDir != "/pc/b.data/identity-first-admin" {
		t.Fatalf("PC data: %s", c.DataDir)
	}
	c, _ = resolveConfig("", env(map[string]string{"IDENTITY_DATA": "/v1"}), git, "/home/u")
	if strings.HasPrefix(c.DataDir, "/v1") || strings.Contains(c.DataDir, ".identity-test/") {
		t.Fatalf("must ignore V1's IDENTITY_DATA and default folder: %s", c.DataDir)
	}
}

func TestHTTPGuardsAndDiagnostics(t *testing.T) {
	withSuggested(t, fakeCommit)
	f := newFake()
	a := testApp(t, t.TempDir(), f)
	ts := httptest.NewServer((&server{app: a}).routes())
	defer ts.Close()
	post := func(path, body string, hdr map[string]string) int {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if c := post("/api/install", `{"admin_name":"Secret Owner Name"}`, map[string]string{"Origin": "http://evil.example"}); c != 403 {
		t.Fatalf("cross-origin: %d", c)
	}
	if c := post("/api/install", `{"admin_name":"x","installation_id":"chosen"}`, nil); c != 400 {
		t.Fatal("caller must not choose IDs")
	}
	if c := post("/api/install", `{"admin_name":"Secret Owner Name"}`, nil); c != 201 {
		t.Fatalf("install: %d", c)
	}
	post("/api/identities/starter", `{}`, nil)
	post("/api/controller/consent", `{"decision":"approve"}`, nil)
	res, _ := http.Get(ts.URL + "/api/diagnostics")
	b := new(strings.Builder)
	buf := make([]byte, 1<<16)
	for {
		n, err := res.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	d := b.String()
	if strings.Contains(d, "Secret Owner Name") || strings.Contains(d, "TASS") || !strings.Contains(d, a.p.Inst.ControllerServiceID) ||
		!strings.Contains(d, "started") || !strings.Contains(d, "GitHub requests made: 0") {
		t.Fatalf("diagnostics:\n%s", d)
	}
}
