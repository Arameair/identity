package main

// L1/L2 tests. fakeGitHub is a FIXTURE that speaks GitHub's device-flow and
// REST shapes. Passing here proves Identity's handling, NOT a real sign-in.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeGitHub struct {
	mu       sync.Mutex
	outcome  string // approve | deny | expire | disabled
	polls    int
	repoCode int
	srv      *httptest.Server
}

func newFakeGitHub(outcome string) *fakeGitHub {
	f := &fakeGitHub{outcome: outcome, repoCode: 200}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if f.outcome == "disabled" {
			io.WriteString(w, `{"error":"device_flow_disabled"}`)
			return
		}
		if r.Form.Get("client_id") == "" {
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"device_code": "dc", "user_code": "WDJB-MJHT",
			"verification_uri": f.srv.URL + "/login/device", "expires_in": 900, "interval": 0})
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.polls++
		n := f.polls
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case n == 1:
			io.WriteString(w, `{"error":"authorization_pending"}`)
		case f.outcome == "deny":
			io.WriteString(w, `{"error":"access_denied"}`)
		case f.outcome == "expire":
			io.WriteString(w, `{"error":"expired_token"}`)
		default:
			io.WriteString(w, `{"access_token":"gho_FIXTURETOKEN","token_type":"bearer"}`)
		}
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gho_FIXTURETOKEN" {
			w.WriteHeader(401)
			return
		}
		io.WriteString(w, `{"login":"fixture-user","id":42}`)
	})
	mux.HandleFunc("GET /repos/Arameair/Thought-Processor", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(f.repoCode)
		if f.repoCode == 200 {
			io.WriteString(w, `{"full_name":"Arameair/Thought-Processor","private":true}`)
		}
	})
	mux.HandleFunc("GET /repos/Arameair/Thought-Processor/branches/{b}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("b") != defaultBranch {
			w.WriteHeader(404)
			return
		}
		io.WriteString(w, `{"name":"x","commit":{"sha":"7c1a69e70df4e03f940b6bff907f0628735d1a81","commit":{"message":"processor: run under Project-controller\n\nbody","committer":{"date":"2026-10-24T09:36:15Z"}}}}`)
	})
	f.srv = httptest.NewServer(mux)
	return f
}

func (f *fakeGitHub) cfg() GitHubConfig {
	return GitHubConfig{ClientID: "Iv1.fixture", WebBase: f.srv.URL, APIBase: f.srv.URL}
}

func waitStatus(t *testing.T, g *GitHub, want string) GitHubView {
	t.Helper()
	end := time.Now().Add(20 * time.Second)
	for time.Now().Before(end) {
		if v := g.View(); v.Status == want {
			return v
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("never reached %s; last %+v", want, g.View())
	return GitHubView{}
}

func TestDeviceFlowOutcomesFixture(t *testing.T) {
	for _, c := range []struct{ outcome, want string }{{"approve", "connected"}, {"deny", "denied"}, {"expire", "expired"}, {"disabled", "error"}} {
		f := newFakeGitHub(c.outcome)
		g := NewGitHub(f.cfg())
		g.Start()
		if c.outcome != "disabled" {
			v := g.View()
			if v.Status != "waiting" || v.UserCode != "WDJB-MJHT" || !strings.HasPrefix(v.VerifyURL, f.srv.URL+"/") || !v.TestMode {
				t.Fatalf("%s: waiting view %+v", c.outcome, v)
			}
		}
		v := waitStatus(t, g, c.want)
		if c.want == "connected" && v.Login != "fixture-user" {
			t.Fatalf("login %q", v.Login)
		}
		if c.outcome == "disabled" && !strings.Contains(v.Message, "Device Flow") {
			t.Fatalf("disabled message: %q", v.Message)
		}
		f.srv.Close()
	}
}

func TestNotConfiguredAndUnsafeVerifyURL(t *testing.T) {
	g := NewGitHub(GitHubConfig{WebBase: "https://github.com", APIBase: "https://api.github.com"})
	if g.View().Status != "not_configured" || g.Start() == nil {
		t.Fatal("must refuse to sign in without a Client ID")
	}
	// A device-code answer pointing anywhere but GitHub's own site is refused.
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"device_code":"d","user_code":"X","verification_uri":"https://evil.example/login","expires_in":900,"interval":5}`)
	}))
	defer evil.Close()
	g2 := NewGitHub(GitHubConfig{ClientID: "x", WebBase: evil.URL, APIBase: evil.URL})
	if g2.Start() == nil || g2.View().Status != "error" {
		t.Fatalf("unsafe verify URL accepted: %+v", g2.View())
	}
}

func TestFindFixture(t *testing.T) {
	f := newFakeGitHub("approve")
	defer f.srv.Close()
	g := NewGitHub(f.cfg())
	if r := g.Find(context.Background(), defaultRepo, defaultBranch); r.Status != "error" {
		t.Fatalf("find before sign-in: %+v", r)
	}
	g.Start()
	waitStatus(t, g, "connected")
	r := g.Find(context.Background(), defaultRepo, defaultBranch)
	if r.Status != "found" || r.Commit != "7c1a69e70df4e03f940b6bff907f0628735d1a81" || !r.Private || r.AsLogin != "fixture-user" || strings.Contains(r.Message, "body") {
		t.Fatalf("found: %+v", r)
	}
	if r := g.Find(context.Background(), defaultRepo, "nope"); r.Status != "no_branch" {
		t.Fatalf("no branch: %+v", r)
	}
	f.repoCode = 404
	if r := g.Find(context.Background(), defaultRepo, defaultBranch); r.Status != "no_access" || !strings.Contains(r.Detail, "fixture-user") {
		t.Fatalf("no access: %+v", r)
	}
}

func TestControllerMissingAndOld(t *testing.T) {
	c := NewController("http://127.0.0.1:1")
	c.Request(context.Background(), defaultRepo, defaultBranch, "", false)
	if v := c.View(); v.Phase != "controller_missing" {
		t.Fatalf("missing: %+v", v)
	}
	old := httptest.NewServer(http.NotFoundHandler())
	defer old.Close()
	c2 := NewController(old.URL)
	c2.Request(context.Background(), defaultRepo, defaultBranch, "", false)
	if v := c2.View(); v.Phase != "controller_old" {
		t.Fatalf("old: %+v", v)
	}
}

func TestSignInLinkHiddenFromLogs(t *testing.T) {
	v := LaunchView{Log: []string{"  open    : http://127.0.0.1:7778/#code=0123456789abcdef0123456789abcdef"}}
	if s := v.sanitized(); strings.Contains(s.Log[0], "0123456789abcdef") {
		t.Fatalf("code leaked: %s", s.Log[0])
	}
}

func TestWizardGuards(t *testing.T) {
	f := newFakeGitHub("approve")
	defer f.srv.Close()
	quit := make(chan struct{})
	ready := make(chan *App, 1)
	done := make(chan error, 1)
	go func() { done <- run(t.TempDir(), 0, "http://127.0.0.1:1", f.cfg(), false, quit, ready) }()
	app := <-ready
	defer func() { close(quit); <-done }()
	base := "http://127.0.0.1:" + itoa(app.port)
	do := func(method, path, body string, hdr map[string]string) (int, string) {
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		for k, v := range hdr {
			if k == "Host" {
				req.Host = v
			} else {
				req.Header.Set(k, v)
			}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, _ := do("GET", "/api/state", "", nil); code != 401 {
		t.Fatalf("state without session: %d", code)
	}
	if code, _ := do("POST", "/api/session", `{"k":"`+app.code+`"}`, map[string]string{"Origin": "http://evil.example", "Content-Type": "application/json"}); code != 403 {
		t.Fatalf("cross-origin sign-in: %d", code)
	}
	if code, _ := do("GET", "/", "", map[string]string{"Host": "evil.example"}); code != 421 {
		t.Fatalf("rebinding host: %d", code)
	}
	req, _ := http.NewRequest("POST", base+"/api/session", strings.NewReader(`{"k":"`+app.code+`"}`))
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var s struct{ CSRF string }
	json.NewDecoder(resp.Body).Decode(&s)
	resp.Body.Close()
	cookie := resp.Cookies()[0].Value
	hdr := map[string]string{"Origin": base, "Content-Type": "application/json", "Cookie": "id_session=" + cookie}
	if code, _ := do("POST", "/api/github/start", `{}`, hdr); code != 403 {
		t.Fatalf("action without CSRF: %d", code)
	}
	hdr["X-Identity-CSRF"] = s.CSRF
	if code, _ := do("POST", "/api/start", `{}`, hdr); code != 409 {
		t.Fatalf("start before find must be refused: %d", code)
	}
	if code, _ := do("POST", "/api/open", `{}`, hdr); code != 409 {
		t.Fatalf("open before running must be refused: %d", code)
	}
	_, body := do("GET", "/api/state", "", map[string]string{"Cookie": "id_session=" + cookie})
	if !strings.Contains(body, `"test_mode":true`) || strings.Contains(body, "gho_") {
		t.Fatalf("state: %s", body)
	}
	_ = url.Values{}
}

func itoa(n int) string    { return strings.TrimSpace(strings.Replace(jsonNum(n), "\n", "", -1)) }
func jsonNum(n int) string { b, _ := json.Marshal(n); return string(b) }
