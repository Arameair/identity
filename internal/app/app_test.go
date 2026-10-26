package app

// Runtime tests: a real Identity HTTP server on a real loopback port, driven
// over HTTP like the browser does. GitHub and Thought Processor are LOCAL
// TEST FIXTURES here (clearly labelled); the real Processor is exercised by
// scripts/smoke.sh, and real GitHub only by the owner.

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/github"
)

type harness struct {
	t      *testing.T
	a      *App
	base   string
	c      *http.Client
	csrf   string
	cancel context.CancelFunc
	done   chan error
	link   string
	// lastView is the approval page's view from the latest approve() call.
	lastView map[string]any
}

func ghFixture(t *testing.T) *httptest.Server {
	var polls atomic.Int32
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"device_code": "fixdevicecode0001", "user_code": "QZXW-8812",
			"verification_uri": srv.URL + "/login/device", "expires_in": 60, "interval": 5})
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if polls.Add(1) < 2 {
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "ghu_APPFIXTUREtoken000000000000001", "expires_in": 28800})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"login": "fixture-owner", "id": 4242, "html_url": "https://github.com/fixture-owner"})
	})
	mux.HandleFunc("GET /user/installations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"installations": []any{}})
	})
	mux.HandleFunc("GET /repos/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "Thought-Processor") {
			json.NewEncoder(w).Encode(map[string]any{"full_name": "Arameair/Thought-Processor", "private": true})
			return
		}
		w.WriteHeader(404)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func startApp(t *testing.T, dir string, gh github.Config) *harness {
	t.Helper()
	return startAppWith(t, dir, gh, nil)
}

func startAppWith(t *testing.T, dir string, gh github.Config, tweak func(*Options)) *harness {
	t.Helper()
	github.SetPollScaleForTests(20 * time.Millisecond)
	opt := Options{Port: 0, DataDir: dir, GitHub: gh, Version: "test", Branch: "test-branch", WatchEvery: 200 * time.Millisecond}
	if tweak != nil {
		tweak(&opt)
	}
	a, err := New(opt)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &harness{t: t, a: a, cancel: cancel, done: make(chan error, 1)}
	ready := make(chan string, 1)
	go func() { h.done <- a.Run(ctx, ln, func(l string) { ready <- l }) }()
	h.link = <-ready
	h.base = "http://127.0.0.1:" + strconv.Itoa(a.Port)
	jar, _ := cookiejar.New(nil)
	h.c = &http.Client{Jar: jar, Timeout: 5 * time.Second}
	return h
}

func (h *harness) stop() {
	h.cancel()
	select {
	case <-h.done:
	case <-time.After(5 * time.Second):
		h.t.Fatal("Identity did not stop")
	}
}

func (h *harness) do(method, path, body string, hdr map[string]string) (int, string) {
	req, _ := http.NewRequest(method, h.base+path, strings.NewReader(body))
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", h.base)
		if h.csrf != "" {
			req.Header.Set("X-Identity-CSRF", h.csrf)
		}
	}
	for k, v := range hdr {
		if k == "Host" {
			req.Host = v
			continue
		}
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := h.c.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (h *harness) signIn() {
	key := h.link[strings.Index(h.link, "#key=")+5:]
	code, body := h.do("POST", "/api/session", `{"key":"`+key+`"}`, nil)
	if code != 200 {
		h.t.Fatalf("sign-in %d %s", code, body)
	}
	var r map[string]string
	json.Unmarshal([]byte(body), &r)
	h.csrf = r["csrf"]
}

func (h *harness) state() map[string]any {
	code, body := h.do("GET", "/api/state", "", nil)
	if code != 200 {
		h.t.Fatalf("state %d %s", code, body)
	}
	var m map[string]any
	json.Unmarshal([]byte(body), &m)
	return m
}

func TestSecurityBoundaries(t *testing.T) {
	h := startApp(t, t.TempDir(), github.Config{WebBase: github.RealWeb, APIBase: github.RealAPI})
	defer h.stop()

	if code, body := h.do("GET", "/api/health", "", nil); code != 200 || !strings.Contains(body, `"app":"identity"`) || strings.Contains(body, "system_id") {
		t.Fatalf("health %d %s", code, body)
	}
	if code, _ := h.do("GET", "/api/state", "", nil); code != 401 {
		t.Fatal("private state readable without session")
	}
	if code, _ := h.do("GET", "/api/diagnostics", "", nil); code != 401 {
		t.Fatal("diagnostics readable without session")
	}
	if code, _ := h.do("POST", "/api/session", `{"key":"wrong"}`, nil); code != 401 {
		t.Fatal("wrong key accepted")
	}
	key := h.link[strings.Index(h.link, "#key=")+5:]
	if code, _ := h.do("POST", "/api/session", `{"key":"`+key+`"}`, map[string]string{"Origin": "http://evil.example"}); code != 403 {
		t.Fatal("cross-origin sign-in accepted")
	}
	h.signIn()
	if code, _ := h.do("POST", "/api/processor/address", `{"address":"127.0.0.1:7860"}`, map[string]string{"X-Identity-CSRF": ""}); code != 403 {
		t.Fatal("mutation without CSRF accepted")
	}
	if code, _ := h.do("POST", "/api/processor/address", `{"address":"127.0.0.1:7860"}`, map[string]string{"Origin": "http://evil.example"}); code != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
	if code, _ := h.do("GET", "/api/state", "", map[string]string{"Host": "attacker.example:" + strconv.Itoa(h.a.Port)}); code != http.StatusMisdirectedRequest {
		t.Fatal("DNS-rebinding host accepted")
	}
	if code, _ := h.do("POST", "/api/processor/address", `{"address":"10.0.0.5:7860"}`, nil); code != 400 {
		t.Fatal("remote processor address accepted")
	}
	st := h.state()
	if st["github"].(map[string]any)["status"] != "not_configured" {
		t.Fatalf("missing Client ID not reported: %v", st["github"])
	}
	if code, _ := h.do("POST", "/api/github/start", `{}`, nil); code != 502 {
		t.Fatal("sign-in started without Client ID")
	}
	if code, _ := h.do("POST", "/api/github/own-app", `{"client_id":"not-an-id"}`, nil); code != 400 {
		t.Fatal("bad client id accepted")
	}
	if code, _ := h.do("POST", "/api/drafts", `{"feature":"thought-processor"}`, nil); code != 409 {
		t.Fatal("draft without GitHub accepted")
	}
}

func TestFirstUseFlowFixtureRestartAndDiagnostics(t *testing.T) {
	dir := t.TempDir()
	gh := ghFixture(t)
	// Labelled Processor stand-in that answers like the real one's public health.
	tp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"app":"thought-processor","protocol":"thought-processor.diagnostic/0-experimental","version":"0.1.0-experiment"}`))
	}))
	defer tp.Close()
	tpAddr := strings.TrimPrefix(tp.URL, "http://")
	cfg := github.Config{ClientID: "IvFIXTURE0001", ClientIDSource: "test", WebBase: gh.URL, APIBase: gh.URL}

	h := startApp(t, dir, cfg)
	h.signIn()
	systemID := h.a.Store.Profile.SystemID

	// GitHub device flow against the fixture.
	if code, body := h.do("POST", "/api/github/start", `{}`, nil); code != 200 || !strings.Contains(body, "QZXW-8812") || !strings.Contains(body, "TEST FIXTURE") {
		t.Fatalf("start %d %s", code, body)
	}
	// While waiting, diagnostics must not contain the user code.
	_, diagWaiting := h.do("GET", "/api/diagnostics", "", nil)
	if strings.Contains(diagWaiting, "QZXW-8812") || strings.Contains(diagWaiting, "fixdevicecode") {
		t.Fatal("device/user code leaked into diagnostics")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		g := h.state()["github"].(map[string]any)
		if g["status"] == "verified" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("never verified: %v", g)
		}
		time.Sleep(30 * time.Millisecond)
	}
	st := h.state()
	feats := st["features"].([]any)
	if feats[0].(map[string]any)["avail_kind"] != "available" || feats[1].(map[string]any)["avail_kind"] != "unavailable" {
		t.Fatalf("availability from evidence: %v", feats)
	}

	// Local draft, duplicate refused as duplicate (not a second record).
	_, b1 := h.do("POST", "/api/drafts", `{"feature":"thought-processor"}`, nil)
	_, b2 := h.do("POST", "/api/drafts", `{"feature":"thought-processor"}`, nil)
	if !strings.Contains(b1, `"created":true`) || !strings.Contains(b2, `"created":false`) || !strings.Contains(b1, "NOT SUBMITTED") {
		t.Fatalf("drafts: %s / %s", b1, b2)
	}
	if strings.Contains(b1, `"grant":"GRANTED`) {
		t.Fatal("grant claimed")
	}

	// Processor discovery (stand-in) and setup completion.
	if code, body := h.do("POST", "/api/processor/address", `{"address":"`+tpAddr+`"}`, nil); code != 200 || !strings.Contains(body, "DETECTED — SELF-REPORTED, UNVERIFIED") {
		t.Fatalf("processor %d %s", code, body)
	}
	h.do("POST", "/api/setup/complete", `{}`, nil)

	// Inject canaries the way they could realistically arrive: an error detail.
	h.a.Log.Error("test", "canary", "", "x", "leak? http://127.0.0.1:7860/#code=abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef Authorization: Bearer CANARYbearer99 Cookie: identity_b_session=CANARYcookie", nil)
	_, diag := h.do("GET", "/api/diagnostics", "", nil)
	for _, bad := range []string{"ghu_APPFIXTURE", "QZXW-8812", "fixdevicecode", h.csrf, h.link[strings.Index(h.link, "#key=")+5:],
		"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef", "CANARYbearer", "CANARYcookie"} {
		if strings.Contains(diag, bad) {
			t.Fatalf("diagnostics leaked %q", bad)
		}
	}
	for _, want := range []string{"fixture-owner", "TEST FIXTURE", "DETECTED — SELF-REPORTED, UNVERIFIED", "NOT SUBMITTED", "branch=test-branch", "draft.saved", "req_", "[not implemented"} {
		if !strings.Contains(diag, want) {
			t.Fatalf("diagnostics missing %q:\n%s", want, diag)
		}
	}
	lastID := h.a.Log.Stats().LastID
	h.stop()

	// Raw log file on disk must not contain secrets either.
	raw, _ := os.ReadFile(filepath.Join(dir, "logs", "identity-events.jsonl"))
	if strings.Contains(string(raw), "ghu_APPFIXTURE") || strings.Contains(string(raw), "QZXW-8812") || strings.Contains(string(raw), "CANARYbearer") {
		t.Fatal("secret on disk")
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", "open-link.txt")); !os.IsNotExist(err) {
		t.Fatal("private open link left behind after stop")
	}

	// Restart on the same profile.
	h2 := startApp(t, dir, cfg)
	defer h2.stop()
	h2.signIn()
	st2 := h2.state()
	app2 := st2["app"].(map[string]any)
	if app2["system_id"] != systemID || app2["run_id"] == h.a.RunID {
		t.Fatalf("restart identity: %v", app2)
	}
	if st2["setup"].(map[string]any)["completed"] != true {
		t.Fatal("setup state lost")
	}
	if g := st2["github"].(map[string]any); g["status"] != "signed_out" || st2["last_account"].(map[string]any)["login"] != "fixture-owner" {
		t.Fatalf("after restart GitHub must be signed out with history only: %v %v", g, st2["last_account"])
	}
	if st2["processor"].(map[string]any)["address"] != tpAddr {
		t.Fatal("processor address lost")
	}
	_, logs := h2.do("GET", "/api/logs?after=0", "", nil)
	if !strings.Contains(logs, h.a.RunID) || !strings.Contains(logs, "draft.saved") {
		t.Fatal("previous run's log not restored")
	}
	if h2.a.Log.Stats().LastID <= lastID {
		t.Fatal("log ids not continued")
	}
	if len(h2.a.Feats.All()) != 1 {
		t.Fatal("draft lost across restart")
	}
}

func TestProfilesAreIsolatedAndLocked(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	cfg := github.Config{WebBase: github.RealWeb, APIBase: github.RealAPI}
	h1 := startApp(t, d1, cfg)
	defer h1.stop()
	if _, err := New(Options{Port: 0, DataDir: d1, GitHub: cfg}); err == nil {
		t.Fatal("second Identity on the same profile was allowed")
	}
	h2 := startApp(t, d2, cfg)
	defer h2.stop()
	if h1.a.Store.Profile.SystemID == h2.a.Store.Profile.SystemID {
		t.Fatal("profiles share identity")
	}
}

func TestStopFromUIStopsOnlyIdentity(t *testing.T) {
	h := startApp(t, t.TempDir(), github.Config{WebBase: github.RealWeb, APIBase: github.RealAPI})
	h.signIn()
	if code, _ := h.do("POST", "/api/shutdown", `{}`, nil); code != 200 {
		t.Fatal("shutdown refused")
	}
	select {
	case err := <-h.done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not stop")
	}
}

func TestOwnAppGuideSwitchesAndReverts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IDENTITY_GITHUB_CLIENT_ID", "")
	builtin := github.Config{ClientID: "Iv23liBUILTIN0001", ClientIDSource: "built into this build", ClientIDKind: "built-in", WebBase: github.RealWeb, APIBase: github.RealAPI}
	github.SetPollScaleForTests(20 * time.Millisecond)
	a, err := New(Options{Port: 0, DataDir: dir, GitHub: builtin, DefaultApp: AppChoice{ID: builtin.ClientID, Source: builtin.ClientIDSource, Kind: "built-in"}})
	if err != nil {
		t.Fatal(err)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ctx, cancel := context.WithCancel(context.Background())
	h := &harness{t: t, a: a, cancel: cancel, done: make(chan error, 1)}
	ready := make(chan string, 1)
	go func() { h.done <- a.Run(ctx, ln, func(l string) { ready <- l }) }()
	h.link = <-ready
	h.base = "http://127.0.0.1:" + strconv.Itoa(a.Port)
	jar, _ := cookiejar.New(nil)
	h.c = &http.Client{Jar: jar, Timeout: 5 * time.Second}
	defer h.stop()
	h.signIn()

	if g := h.state()["github"].(map[string]any); g["client_id_kind"] != "built-in" || g["status"] != "signed_out" {
		t.Fatalf("built-in app not ready: %v", g)
	}
	if code, _ := h.do("POST", "/api/github/own-app", `{"client_id":"Iv23liOWNAPP00001"}`, nil); code != 200 {
		t.Fatal("own app refused")
	}
	if g := h.state()["github"].(map[string]any); g["client_id_kind"] != "own" {
		t.Fatalf("own app not in use: %v", g)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "github-app.json")); !strings.Contains(string(b), "Iv23liOWNAPP00001") {
		t.Fatal("own app not saved in profile")
	}
	if code, _ := h.do("POST", "/api/github/own-app/remove", `{}`, nil); code != 200 {
		t.Fatal("remove refused")
	}
	if g := h.state()["github"].(map[string]any); g["client_id_kind"] != "built-in" || g["client_id_hint"] != "Iv23…0001" {
		t.Fatalf("did not return to built-in app: %v", g)
	}
	if _, err := os.Stat(filepath.Join(dir, "github-app.json")); !os.IsNotExist(err) {
		t.Fatal("own app file left behind")
	}
}
