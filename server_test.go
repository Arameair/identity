package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	st := mustStore(t, dir)
	ev := openEventLog(dir)
	t.Cleanup(ev.Close)
	s := &server{cfg: Config{DataDir: dir, Branch: "test-branch", Commit: "abc", Version: appVersion}, store: st, events: ev, started: time.Now()}
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return s, ts
}

func postJSON(t *testing.T, url string, body any, hdr ...string) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func getBody(t *testing.T, url string) (int, string) {
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestHTTPCreateOpenAndFutureSlots(t *testing.T) {
	_, ts := newTestServer(t)
	code, out := postJSON(t, ts.URL+"/api/identities", map[string]any{"display_name": "TASS", "entity_type": "ai_companion", "request_id": "r1"})
	if code != 201 {
		t.Fatalf("%d %v", code, out)
	}
	id := out["identity"].(map[string]any)["id"].(string)
	code, body := getBody(t, ts.URL+"/api/identities/"+id)
	if code != 200 || !strings.Contains(body, `"display_name": "TASS"`) || !strings.Contains(body, "AI companion") {
		t.Fatalf("%d %s", code, body)
	}
	for _, want := range []string{`"GitHub repository"`, `"NOT CONFIGURED"`, `"Thought Controller"`, `"NOT CONNECTED"`} {
		if !strings.Contains(body, want) {
			t.Errorf("detail missing %s", want)
		}
	}
	if code, _ := getBody(t, ts.URL+"/api/identities/00000000-0000-4000-8000-000000000000"); code != 404 {
		t.Fatal("unknown id")
	}
	if code, _ := getBody(t, ts.URL+"/api/identities/..%2F..%2Fsecret"); code != 400 {
		t.Fatalf("path-like id: %d", code)
	}
	// repeated click
	_, again := postJSON(t, ts.URL+"/api/identities", map[string]any{"display_name": "TASS", "entity_type": "ai_companion", "request_id": "r1"})
	if again["repeat"] != true || again["identity"].(map[string]any)["id"] != id {
		t.Fatalf("%v", again)
	}
	_, list := getBody(t, ts.URL+"/api/identities")
	if strings.Count(list, `"id":`) != 1 {
		t.Fatalf("repeat created a second identity: %s", list)
	}
}

func TestHTTPRejectsBadRequests(t *testing.T) {
	s, ts := newTestServer(t)
	cases := []struct {
		body any
		want int
	}{
		{map[string]any{"display_name": "", "entity_type": "business"}, 400},
		{map[string]any{"display_name": "X", "entity_type": "dragon"}, 400},
		{map[string]any{"display_name": "X", "entity_type": "business", "id": "11111111-1111-4111-8111-111111111111"}, 400}, // can't choose an ID
		{map[string]any{"display_name": "X", "entity_type": "business", "password": "hunter2"}, 400},
	}
	for _, c := range cases {
		if code, out := postJSON(t, ts.URL+"/api/identities", c.body); code != c.want {
			t.Errorf("%v -> %d %v", c.body, code, out)
		}
	}
	if code, _ := postJSON(t, ts.URL+"/api/identities", map[string]any{"display_name": "X", "entity_type": "business"}, "Origin", "https://evil.example"); code != 403 {
		t.Error("cross-site post")
	}
	req, _ := http.NewRequest("POST", ts.URL+"/api/identities", strings.NewReader("display_name=X&entity_type=business"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 415 {
		t.Error("form post")
	}
	req, _ = http.NewRequest("GET", ts.URL+"/api/identities", nil)
	req.Host = "attacker.example:80"
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 403 {
		t.Error("foreign Host header (DNS rebinding)")
	}
	big := map[string]any{"display_name": strings.Repeat("a", 5000), "entity_type": "business"}
	if code, _ := postJSON(t, ts.URL+"/api/identities", big); code != 400 {
		t.Error("oversized body")
	}
	if r, _ := http.Post(ts.URL+"/api/identities/x", "application/json", nil); r.StatusCode != 405 {
		t.Errorf("wrong method: %d", r.StatusCode)
	}
	if len(s.store.List()) != 0 {
		t.Fatal("rejected requests created identities")
	}
}

func TestHTTPFailureToolAndDiagnostics(t *testing.T) {
	s, ts := newTestServer(t)
	_, out := postJSON(t, ts.URL+"/api/identities", map[string]any{"display_name": "TASS", "entity_type": "ai_companion"})
	tassID := out["identity"].(map[string]any)["id"].(string)
	postJSON(t, ts.URL+"/api/test/fail-next-save", map[string]any{})
	code, out := postJSON(t, ts.URL+"/api/identities", map[string]any{"display_name": "Example Workshop", "entity_type": "business"})
	if code != 500 || !strings.Contains(out["error"].(string), "save failed") {
		t.Fatalf("%d %v", code, out)
	}
	postJSON(t, ts.URL+"/api/test/corrupt-file", map[string]any{})
	_, list := getBody(t, ts.URL+"/api/identities")
	if strings.Count(list, `"id":`) != 1 || !strings.Contains(list, "corrupt-test-") || !strings.Contains(list, tassID) {
		t.Fatalf("%s", list)
	}
	_, diag := getBody(t, ts.URL+"/api/diagnostics")
	for _, want := range []string{"test-branch", "Saved identities: 1", tassID, "Problem files: 1", "NOT CONNECTED", "save failed"} {
		if !strings.Contains(diag, want) {
			t.Errorf("diagnostics missing %q", want)
		}
	}
	if strings.Contains(diag, "TASS") {
		t.Errorf("diagnostics must not include names:\n%s", diag)
	}
	_, status := getBody(t, ts.URL+"/api/status")
	if !strings.Contains(status, `"count": 1`) || !strings.Contains(status, `"kind": "error"`) {
		t.Fatalf("%s", status)
	}
	if n := len(s.store.Problems()); n != 1 {
		t.Fatal(n)
	}
}

func TestPageServed(t *testing.T) {
	_, ts := newTestServer(t)
	code, body := getBody(t, ts.URL+"/")
	if code != 200 || !strings.Contains(body, "IDENTITY MANAGER") || !strings.Contains(body, "Copy Diagnostics") {
		t.Fatal(code)
	}
}
