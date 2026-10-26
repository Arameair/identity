package app

// Engine access API tests. GitHub here is a LOCAL TEST FIXTURE: it can sign
// in as different accounts and serves a fake Thought-engine release.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/pairing"
)

type engFixture struct {
	srv *httptest.Server
	mu  sync.Mutex
	// who the next device sign-in becomes, and what they can see
	login     string
	visible   map[string]bool // login -> can see Thought-engine
	installed bool            // app installed on Thought-engine (for owner hint)
	asset     []byte
	sums      string
	uploader  string // who uploaded the asset (approved: github-actions[bot])
	polls     int
}

func newEngFixture(t *testing.T) *engFixture {
	f := &engFixture{login: "Arameair", visible: map[string]bool{"Arameair": true}, installed: true, uploader: "github-actions[bot]",
		asset: []byte("FAKE THOUGHT ENGINE BINARY — test fixture only\n")}
	sum := sha256.Sum256(f.asset)
	f.sums = hex.EncodeToString(sum[:]) + "  thought-engine-windows-amd64.exe\n" + strings.Repeat("0", 64) + "  thought-engine-linux-amd64\n"
	token := func(r *http.Request) string {
		return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ghu_FX_")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.polls = 0
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"device_code": "dc", "user_code": "ENGF-0001", "verification_uri": f.srv.URL + "/login/device", "expires_in": 60, "interval": 5})
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.polls++
		if f.polls < 2 {
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "ghu_FX_" + f.login + "_padpadpadpadpadpad", "expires_in": 28800})
	})
	who := func(r *http.Request) string { return strings.TrimSuffix(token(r), "_padpadpadpadpadpad") }
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		l := who(r)
		id := int64(1000)
		if l != "Arameair" {
			id = 2000
		}
		json.NewEncoder(w).Encode(map[string]any{"login": l, "id": id})
	})
	mux.HandleFunc("GET /user/installations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"installations": []map[string]any{{"id": 5, "app_id": 1, "app_slug": "identity-fixture", "account": map[string]string{"login": "Arameair"}, "repository_selection": "selected"}}})
	})
	mux.HandleFunc("GET /user/installations/5/repositories", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		repos := []map[string]string{{"full_name": "Arameair/Thought-Processor"}}
		if f.installed {
			repos = append(repos, map[string]string{"full_name": "Arameair/Thought-engine"})
		}
		json.NewEncoder(w).Encode(map[string]any{"repositories": repos})
	})
	mux.HandleFunc("GET /repos/Arameair/Thought-Processor", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"full_name": "Arameair/Thought-Processor", "private": false})
	})
	canSee := func(r *http.Request) bool { f.mu.Lock(); defer f.mu.Unlock(); return f.visible[who(r)] }
	mux.HandleFunc("GET /repos/Arameair/Thought-engine", func(w http.ResponseWriter, r *http.Request) {
		if !canSee(r) {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"full_name": "Arameair/Thought-engine", "private": true})
	})
	mux.HandleFunc("GET /repos/Arameair/Thought-engine/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if !canSee(r) {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"tag_name": "engine-v0.1.0-fixture", "name": "fixture", "published_at": time.Now().UTC(),
			"html_url": "https://github.com/Arameair/Thought-engine/releases/tag/engine-v0.1.0-fixture", "author": map[string]string{"login": "github-actions[bot]"},
			"assets": []map[string]any{{"id": 11, "name": "thought-engine-windows-amd64.exe", "size": len(f.asset), "uploader": map[string]string{"login": f.uploader}},
				{"id": 12, "name": "SHA256SUMS", "size": len(f.sums), "uploader": map[string]string{"login": "github-actions[bot]"}}}})
	})
	mux.HandleFunc("GET /repos/Arameair/Thought-engine/releases/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !canSee(r) || r.Header.Get("Accept") != "application/octet-stream" {
			w.WriteHeader(404)
			return
		}
		// Like GitHub: redirect to storage on another host.
		http.Redirect(w, r, f.srv.URL+"/storage/"+r.PathValue("id"), http.StatusFound)
	})
	mux.HandleFunc("GET /storage/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" && strings.HasPrefix(r.Host, "storage.") {
			t.Error("token forwarded to storage host")
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.PathValue("id") == "11" {
			w.Write(f.asset)
		} else {
			w.Write([]byte(f.sums))
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// machine calls Identity like Thought Processor does: no browser Origin.
func (h *harness) machine(method, path, body, token string) (int, http.Header, []byte) {
	req, _ := http.NewRequest(method, h.base+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		signTest(req, token, body)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

func (h *harness) pairProcessor(id string) string {
	h.t.Helper()
	h.do("POST", "/api/pairing/code", `{}`, nil)
	code := h.state()["pairing"].(map[string]any)["pending_code"].(string)
	st, _, b := h.machine("POST", "/api/v1/pair", fmt.Sprintf(`{"code":%q,"processor_id":%q,"processor_label":"test processor"}`, code, id), "")
	if st != 200 {
		h.t.Fatalf("pair %d %s", st, b)
	}
	var r map[string]string
	json.Unmarshal(b, &r)
	testPIDs.Store(r["secret"], id)
	return r["secret"]
}

func (h *harness) githubSignIn() {
	h.t.Helper()
	h.do("POST", "/api/github/start", `{}`, nil)
	deadline := time.Now().Add(5 * time.Second)
	for h.state()["github"].(map[string]any)["status"] != "verified" {
		if time.Now().After(deadline) {
			h.t.Fatalf("sign-in never completed: %v", h.state()["github"])
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func decision(t *testing.T, b []byte, token string) Decision {
	var d Decision
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("decision: %v %s", err, b)
	}
	if d.Proof != pairing.Proof(respKeyOf(token), d.canonical()) {
		t.Fatalf("decision proof does not verify: %+v", d)
	}
	return d
}

func TestEngineAccessPairingAllowDenyDownload(t *testing.T) {
	dir := t.TempDir()
	f := newEngFixture(t)
	cfg := github.Config{ClientID: "IvFIXTURE0001", ClientIDSource: "test", ClientIDKind: "built-in", WebBase: f.srv.URL, APIBase: f.srv.URL}
	h := startApp(t, dir, cfg)
	h.signIn()

	// Pairing: browser-origin and wrong-code attempts refused.
	h.do("POST", "/api/pairing/code", `{}`, nil)
	code := h.state()["pairing"].(map[string]any)["pending_code"].(string)
	if st, _ := h.do("POST", "/api/v1/pair", fmt.Sprintf(`{"code":%q,"processor_id":"tp_x1"}`, code), nil); st != 403 {
		t.Fatalf("pairing from a web page accepted: %d", st)
	}
	if st, _, _ := h.machine("POST", "/api/v1/pair", `{"code":"AAAA-BBBB","processor_id":"tp_x1"}`, ""); st != 401 {
		t.Fatal("wrong code accepted")
	}
	h.do("POST", "/api/pairing/cancel", `{}`, nil)
	token := h.pairProcessor("tp_test0001")

	if st, _, _ := h.machine("GET", "/api/v1/features/thought-engine/access", "", ""); st != 401 {
		t.Fatal("access without token")
	}
	if st, _, _ := h.machine("GET", "/api/v1/features/thought-engine/access", "", strings.Repeat("b", 64)); st != 401 {
		t.Fatal("access with wrong token")
	}
	// Private feature: before sign-in it is hidden (same 404 as an unknown feature).
	if st, _, _ := h.machine("GET", "/api/v1/features/thought-engine/access", "", token); st != 404 {
		t.Fatalf("before sign-in: %d", st)
	}

	// Owner with the app installed on the repo: ALLOWED, release + download verify.
	h.githubSignIn()
	_, _, b := h.machine("GET", "/api/v1/features/thought-engine/access?fresh=1", "", token)
	if d := decision(t, b, token); d.State != Allowed || d.Account.Login != "Arameair" {
		t.Fatalf("owner: %+v", d)
	}
	st, _, b := h.machine("GET", "/api/v1/features/thought-engine/release?platform=windows-amd64", "", token)
	var ri ReleaseInfo
	json.Unmarshal(b, &ri)
	if st != 200 || ri.Tag != "engine-v0.1.0-fixture" || ri.Proof != pairing.Proof(respKeyOf(token), ri.canonical()) {
		t.Fatalf("release %d %s", st, b)
	}
	st, hdr, body := h.machine("GET", "/api/v1/features/thought-engine/release/download?platform=windows-amd64&tag="+ri.Tag, "", token)
	sum := sha256.Sum256(body)
	if st != 200 || hex.EncodeToString(sum[:]) != ri.SHA256 || hdr.Get("X-Release-Tag") != ri.Tag {
		t.Fatalf("download %d sha %x want %s", st, sum, ri.SHA256)
	}
	if st, _, _ := h.machine("GET", "/api/v1/features/thought-engine/release?platform=windows-amd64;rm", "", token); st != 400 {
		t.Fatal("bad platform accepted")
	}
	// Generic route and contract give the same release, with its source checks.
	st, _, b = h.machine("GET", "/api/v1/features/thought-engine/release?platform=windows-amd64", "", token)
	var gri ReleaseInfo
	json.Unmarshal(b, &gri)
	if st != 200 || gri.SHA256 != ri.SHA256 || gri.Source == nil || gri.Source.AssetUploader != "github-actions[bot]" || len(gri.Source.Checks) < 4 {
		t.Fatalf("generic release %d %s", st, b)
	}
	if st, _, b := h.machine("GET", "/api/v1/features/thought-engine/contract", "", token); st != 200 || !strings.Contains(string(b), `"tag_prefix":"engine-v"`) {
		t.Fatalf("contract %d %s", st, b)
	}
	// Approved source: a file uploaded by a person (not the release workflow) is refused.
	f.mu.Lock()
	f.uploader = "someone-else"
	f.mu.Unlock()
	if st, _, b := h.machine("GET", "/api/v1/features/thought-engine/release?platform=windows-amd64", "", token); st != 422 || !strings.Contains(string(b), "UNAPPROVED_SOURCE") {
		t.Fatalf("unapproved uploader %d %s", st, b)
	}
	if st, _, _ := h.machine("GET", "/api/v1/features/thought-engine/release/download?platform=windows-amd64", "", token); st != 422 {
		t.Fatalf("download from unapproved source: %d", st)
	}
	f.mu.Lock()
	f.uploader = "github-actions[bot]"
	f.mu.Unlock()

	// Owner, app NOT installed on the repo: specific hint.
	f.mu.Lock()
	f.visible["Arameair"], f.installed = false, false
	f.mu.Unlock()
	h.a.EngineAccess(context.Background(), true)
	if d := h.a.LastAccess(); d.State != NotAllowed || d.Hint != "owner_app_not_installed" {
		t.Fatalf("owner without app install: %+v", d)
	}
	if st, _, _ := h.machine("GET", "/api/v1/features/thought-engine/access", "", token); st != 404 {
		t.Fatalf("not-allowed account still sees the private feature: %d", st)
	}

	// A second account that can't see the repo: NOT_ALLOWED, no release, no download.
	h.do("POST", "/api/github/disconnect", `{}`, nil)
	f.mu.Lock()
	f.login = "brother-account"
	f.mu.Unlock()
	h.githubSignIn()
	h.a.EngineAccess(context.Background(), true)
	if d := h.a.LastAccess(); d.State != NotAllowed || !strings.Contains(d.Reason, "brother-account") || d.Hint != "" {
		t.Fatalf("second account: %+v", d)
	}
	for _, path := range []string{"/api/v1/features/thought-engine/release?platform=windows-amd64", "/api/v1/features/thought-engine/release/download?platform=windows-amd64", "/api/v1/features/thought-engine/contract"} {
		if st, _, _ := h.machine("GET", path, "", token); st != 404 {
			t.Fatalf("%s answered a not-allowed account: %d", path, st)
		}
	}

	// Diagnostics never contain the token or a pairing code.
	h.do("POST", "/api/pairing/code", `{}`, nil)
	pending := h.state()["pairing"].(map[string]any)["pending_code"].(string)
	_, diag := h.do("GET", "/api/diagnostics", "", nil)
	if strings.Contains(diag, token) || strings.Contains(diag, pending) || !strings.Contains(diag, "paired_processors=1") {
		t.Fatalf("diagnostics leak or missing pairing:\n%s", diag)
	}

	// Restart: pairing survives, GitHub sign-in does not.
	h.stop()
	h2 := startApp(t, dir, cfg)
	defer h2.stop()
	if st, _, _ := h2.machine("GET", "/api/v1/features/thought-engine/access", "", token); st != 404 {
		t.Fatalf("after restart (signed out → hidden): %d", st)
	}
	// Unpair from Identity's window: token stops working.
	h2.signIn()
	h2.do("POST", "/api/pairing/unpair", `{"processor_id":"tp_test0001"}`, nil)
	if st, _, _ := h2.machine("GET", "/api/v1/features/thought-engine/access", "", token); st != 401 {
		t.Fatal("token works after unpair")
	}
}
