package app

// API v1 tests: a real Identity HTTP server on a real loopback port, called
// the way Thought Processor calls it (no browser Origin, bearer token, signed
// answers). GitHub is a LOCAL TEST FIXTURE where used.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/pairing"
)

var codeRE = regexp.MustCompile(`\b([A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4})\b`)

// console collects what Identity prints in its own window.
type console struct {
	mu    sync.Mutex
	lines []string
}

func (c *console) announce(l []string) { c.mu.Lock(); c.lines = append(c.lines, l...); c.mu.Unlock() }

func (c *console) lastCode(t *testing.T) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.lines) - 1; i >= 0; i-- {
		if m := codeRE.FindStringSubmatch(c.lines[i]); m != nil {
			return m[1]
		}
	}
	t.Fatal("no pairing code shown in Identity's window")
	return ""
}

func startV1(t *testing.T, dir string, gh github.Config) (*harness, *console) {
	con := &console{}
	h := startAppWith(t, dir, gh, func(o *Options) { o.Announce = con.announce; o.ManagedBy = "thought-processor:tp_test" })
	return h, con
}

var realGH = github.Config{WebBase: github.RealWeb, APIBase: github.RealAPI}

// v1 calls a paired route and verifies Identity's proof over the exact body.
func (h *harness) v1(method, path, body, token string) (int, map[string]any) {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.base+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(NonceHeader, "nonce-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	var raw []byte
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		raw = append(raw, buf[:n]...)
		if err != nil {
			break
		}
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	if resp.StatusCode != 401 { // a refused token can't be signed with
		if got, want := resp.Header.Get(ProofHeader), pairing.Proof(token, ProofCanonical("nonce-123", raw)); got != want {
			h.t.Fatalf("%s %s: proof does not verify (%q)", method, path, got)
		}
		if resp.Header.Get(SystemIDHeader) != h.a.Store.Profile.SystemID {
			h.t.Fatalf("system id header wrong")
		}
	}
	return resp.StatusCode, m
}

func (h *harness) pairV1(con *console, id string) (string, map[string]any) {
	h.t.Helper()
	st, _, b := h.machine("POST", "/api/v1/pair/request", `{"processor_id":"`+id+`","processor_label":"Test Processor"}`, "")
	if st != 202 {
		h.t.Fatalf("pair request %d %s", st, b)
	}
	code := con.lastCode(h.t)
	if strings.Contains(string(b), code) {
		h.t.Fatal("the pairing code must only be shown in Identity's window, never in the answer")
	}
	st, _, b = h.machine("POST", "/api/v1/pair", `{"code":"`+code+`","processor_id":"`+id+`","processor_label":"Test Processor"}`, "")
	if st != 200 {
		h.t.Fatalf("pair %d %s", st, b)
	}
	var r map[string]any
	json.Unmarshal(b, &r)
	return r["token"].(string), r
}

func TestV1PairStatusAndBoundaries(t *testing.T) {
	dir := t.TempDir()
	h, con := startV1(t, dir, realGH)
	defer h.stop()

	// Service file lets Processor find Identity whichever starts first.
	var svc map[string]any
	b, err := os.ReadFile(filepath.Join(dir, "runtime", "service.json"))
	if err != nil || json.Unmarshal(b, &svc) != nil || int(svc["port"].(float64)) != h.a.Port || svc["managed_by"] != "thought-processor:tp_test" {
		t.Fatalf("service.json wrong: %v %s", err, b)
	}
	if strings.Contains(string(b), h.a.Store.Profile.SystemID) {
		t.Fatal("service.json must not be mistaken for proof (no system id)")
	}

	// Paired routes refuse: no token, wrong token, browser origin.
	if st, _, _ := h.machine("GET", "/api/v1/status", "", ""); st != 401 {
		t.Fatalf("status without token: %d", st)
	}
	if st, _, _ := h.machine("GET", "/api/v1/status", "", strings.Repeat("a", 64)); st != 401 {
		t.Fatalf("status with wrong token: %d", st)
	}
	token, pr := h.pairV1(con, "tp_test")
	if pr["identity_system_id"] != h.a.Store.Profile.SystemID {
		t.Fatal("pair answer lacks system id")
	}
	req, _ := http.NewRequest("GET", h.base+"/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "http://evil.example")
	if resp, _ := http.DefaultClient.Do(req); resp.StatusCode != 403 {
		t.Fatalf("browser origin accepted: %d", resp.StatusCode)
	}

	st, s := h.v1("GET", "/api/v1/status", "", token)
	if st != 200 || s["state"] != "READY" || s["api"] != APIVersion || s["identity_system_id"] != h.a.Store.Profile.SystemID {
		t.Fatalf("status %d %v", st, s)
	}
	lc := s["lifecycle"].(map[string]any)
	if lc["ownership"] != "MANAGED" || int(lc["pid"].(float64)) != os.Getpid() {
		t.Fatalf("lifecycle %v", lc)
	}
	if !strings.Contains(s["authority"].(map[string]any)["grants"].(string), "DENIED") {
		t.Fatal("status must say grants are not implemented")
	}

	// Throttle: a second request right away is refused, the first code still works.
	if st, _, _ := h.machine("POST", "/api/v1/pair/request", `{"processor_id":"tp_other","processor_label":"x"}`, ""); st != 429 {
		t.Fatalf("pair request not throttled: %d", st)
	}
	// Wrong code is refused.
	if st, _, _ := h.machine("POST", "/api/v1/pair", `{"code":"AAAA-AAAA","processor_id":"tp_x","processor_label":"x"}`, ""); st != 401 {
		t.Fatalf("wrong code: %d", st)
	}
	// Diagnostics never contain the token.
	h.signIn()
	if _, d := h.do("GET", "/api/diagnostics", "", nil); strings.Contains(d, token) || !strings.Contains(d, "ownership=MANAGED") {
		t.Fatal("diagnostics leak token or miss lifecycle")
	}
}

func TestV1IdentitiesDefaultDenyAndRestartReconnect(t *testing.T) {
	dir := t.TempDir()
	h, con := startV1(t, dir, realGH)
	token, _ := h.pairV1(con, "tp_test")

	st, e := h.v1("POST", "/api/v1/identities", `{"name":"Tass","kind":"companion"}`, token)
	if st != 201 || e["kind"] != "companion" || len(e["authorized_sources"].([]any)) != 0 || len(e["enabled_features"].([]any)) != 0 {
		t.Fatalf("create %d %v", st, e)
	}
	id := e["id"].(string)
	if st, _ := h.v1("POST", "/api/v1/identities", `{"name":"tass","kind":"person"}`, token); st != 409 {
		t.Fatalf("duplicate name: %d", st)
	}
	if st, _ := h.v1("POST", "/api/v1/identities", `{"name":"S","kind":"system"}`, token); st != 400 {
		t.Fatalf("system kind: %d", st)
	}
	h.v1("POST", "/api/v1/identities", `{"name":"Dustin","kind":"person"}`, token)

	// Every authorization is DENIED, with a reason, and never cacheable.
	for _, c := range []struct{ body, state string }{
		{`{"identity_id":"` + id + `","feature":"thought-engine","operation":"observe","source":"discord"}`, "NO_GRANT"},
		{`{"identity_id":"ent_00000000000000000000","feature":"thought-engine","operation":"observe"}`, "UNKNOWN_IDENTITY"},
		{`{"identity_id":"` + id + `","feature":"nope","operation":"x"}`, "UNKNOWN_FEATURE"},
	} {
		st, d := h.v1("POST", "/api/v1/authorize", c.body, token)
		if st != 200 || d["decision"] != "DENIED" || d["state"] != c.state || d["cache"] != "none" || d["valid_for_seconds"].(float64) != 0 {
			t.Fatalf("authorize %s: %d %v", c.body, st, d)
		}
	}

	// Feature states keep install eligibility apart from use.
	st, fs := h.v1("GET", "/api/v1/identities/"+id+"/features", "", token)
	if st != 200 {
		t.Fatalf("features %d", st)
	}
	var eng map[string]any
	for _, f := range fs["features"].([]any) {
		if m := f.(map[string]any); m["feature"] == "thought-engine" {
			eng = m
		}
	}
	el := eng["eligible_to_install"].(map[string]any)
	if el["state"] != NotSignedIn || eng["enabled_for_identity"] != "NOT_GRANTED" || eng["installed"] != notTrackedByIdty || !strings.Contains(el["meaning"].(string), "install only") {
		t.Fatalf("engine feature state %v", eng)
	}

	// Archive / restore / rename.
	if st, a := h.v1("POST", "/api/v1/identities/"+id+"/archive", `{}`, token); st != 200 || a["archived_at"] == nil {
		t.Fatalf("archive %d %v", st, a)
	}
	if _, d := h.v1("POST", "/api/v1/authorize", `{"identity_id":"`+id+`","feature":"thought-engine","operation":"observe"}`, token); d["state"] != "IDENTITY_ARCHIVED" {
		t.Fatalf("archived identity decision %v", d)
	}
	if _, l := h.v1("GET", "/api/v1/identities", "", token); l["count"].(float64) != 1 {
		t.Fatalf("archived identity listed: %v", l)
	}
	h.v1("POST", "/api/v1/identities/"+id+"/restore", `{}`, token)
	if st, r := h.v1("POST", "/api/v1/identities/"+id+"/rename", `{"name":"TASS"}`, token); st != 200 || r["name"] != "TASS" {
		t.Fatalf("rename %d %v", st, r)
	}
	sys := h.a.Store.Profile.SystemID
	h.stop()

	// Restart: same installation, identities kept, Processor reconnects with
	// its saved token — no new pairing (C05).
	h2, _ := startV1(t, dir, realGH)
	defer h2.stop()
	st, s := h2.v1("GET", "/api/v1/status", "", token)
	if st != 200 || s["identity_system_id"] != sys {
		t.Fatalf("reconnect after restart failed: %d %v", st, s)
	}
	if _, l := h2.v1("GET", "/api/v1/identities", "", token); l["count"].(float64) != 2 {
		t.Fatalf("identities lost on restart: %v", l)
	}
}

func TestV1WrongServiceAndUnpair(t *testing.T) {
	hA, conA := startV1(t, t.TempDir(), realGH)
	defer hA.stop()
	hB, _ := startV1(t, t.TempDir(), realGH)
	defer hB.stop()
	token, _ := hA.pairV1(conA, "tp_test")
	// A token from installation A means nothing to installation B.
	if st, _, _ := hB.machine("GET", "/api/v1/status", "", token); st != 401 {
		t.Fatalf("other installation accepted the token: %d", st)
	}
	if hA.a.Store.Profile.SystemID == hB.a.Store.Profile.SystemID {
		t.Fatal("two installations share a system id")
	}
	// Self-unpair: answer is signed, then the token stops working.
	if st, r := hA.v1("POST", "/api/v1/unpair", `{}`, token); st != 200 || r["unpaired"] != true {
		t.Fatalf("unpair %d %v", st, r)
	}
	if st, _, _ := hA.machine("GET", "/api/v1/status", "", token); st != 401 {
		t.Fatalf("token still works after unpair: %d", st)
	}
}

func TestV1GitHubConnectFromProcessor(t *testing.T) {
	gh := ghFixture(t)
	h, con := startV1(t, t.TempDir(), github.Config{WebBase: gh.URL, APIBase: gh.URL, ClientID: "Iv23liFIXTURE0001"})
	defer h.stop()
	token, _ := h.pairV1(con, "tp_test")
	st, g := h.v1("POST", "/api/v1/github/connect", `{}`, token)
	if st != 200 || g["state"] != "WAITING" || g["user_code"] != "QZXW-8812" || g["verification_uri"] == nil {
		t.Fatalf("connect %d %v", st, g)
	}
	deadline := time.Now().Add(5 * time.Second)
	for g["state"] != "VERIFIED" {
		if time.Now().After(deadline) {
			t.Fatalf("never verified: %v", g)
		}
		time.Sleep(30 * time.Millisecond)
		_, g = h.v1("GET", "/api/v1/github", "", token)
	}
	if g["account"].(map[string]any)["login"] != "fixture-owner" || !strings.Contains(g["meaning"].(string), "not, by itself, permission") {
		t.Fatalf("verified view %v", g)
	}
	if _, g = h.v1("POST", "/api/v1/github/disconnect", `{}`, token); g["state"] != "SIGNED_OUT" {
		t.Fatalf("disconnect %v", g)
	}
}

func TestV1ShutdownNeedsConfirmAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	h, con := startV1(t, dir, realGH)
	token, _ := h.pairV1(con, "tp_test")
	if st, _ := h.v1("POST", "/api/v1/shutdown", `{}`, token); st != 400 {
		t.Fatalf("unconfirmed shutdown accepted: %d", st)
	}
	st, r := h.v1("POST", "/api/v1/shutdown", `{"confirm":"shut-down-identity","reason":"person chose Shut down on exit"}`, token)
	if st != 202 || r["stopping"] != true {
		t.Fatalf("shutdown %d %v", st, r)
	}
	select {
	case err := <-h.done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Identity did not stop after confirmed shutdown")
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", "service.json")); !os.IsNotExist(err) {
		t.Fatal("service.json left behind after clean stop")
	}
	logs, _ := os.ReadFile(filepath.Join(dir, "logs", "identity-events.jsonl"))
	if !strings.Contains(string(logs), "person chose Shut down on exit") || !strings.Contains(string(logs), "identity.stopping") {
		t.Fatal("stop reason not logged")
	}
}

func TestV1LocatorAndGenericFeatureRoutes(t *testing.T) {
	dir, loc := t.TempDir(), t.TempDir()
	con := &console{}
	h := startAppWith(t, dir, realGH, func(o *Options) { o.Announce = con.announce; o.LocatorDir = loc })
	token, _ := h.pairV1(con, "tp_test")

	// The per-user locator names this installation, so Processor can find the
	// right Identity without knowing its profile folder.
	p := filepath.Join(loc, h.a.Store.Profile.SystemID+".json")
	var rec map[string]any
	b, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(b, &rec) != nil || int(rec["port"].(float64)) != h.a.Port || rec["identity_system_id"] != h.a.Store.Profile.SystemID {
		t.Fatalf("locator wrong: %v %s", err, b)
	}

	if st, e := h.v1("GET", "/api/v1/features/thought-engine/eligibility", "", token); st != 200 || e["eligible_to_install"].(map[string]any)["state"] != NotSignedIn || e["install_adapter"] == "NONE" {
		t.Fatalf("engine eligibility %d %v", st, e)
	}
	if st, e := h.v1("GET", "/api/v1/features/thought-processor/eligibility", "", token); st != 200 || e["install_adapter"] != "NONE" {
		t.Fatalf("processor eligibility %d %v", st, e)
	}
	if st, e := h.v1("GET", "/api/v1/features/thought-processor/release?platform=windows-amd64", "", token); st != 404 || e["error"] != "NO_INSTALL_ADAPTER" {
		t.Fatalf("no-adapter release %d %v", st, e)
	}
	if st, _ := h.v1("GET", "/api/v1/features/nope/eligibility", "", token); st != 404 {
		t.Fatalf("unknown feature %d", st)
	}
	// Generic route reaches the Engine adapter (refused here: not signed in to GitHub).
	if st, e := h.v1("GET", "/api/v1/features/thought-engine/release?platform=windows-amd64", "", token); st != 403 || e["error"] != "not_allowed" {
		t.Fatalf("generic engine release %d %v", st, e)
	}
	h.stop()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("locator left behind after clean stop")
	}
}
