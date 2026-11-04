package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/authority"
	"github.com/Arameair/identity/internal/entity"
)

func init() { authority.SetIterationsForTests(1000) }

const engineKey = "0123456789abcdef0123456789abcdef-engine"

func req(t *testing.T, h http.Handler, method, host, path string, hdr map[string]string, body any) (int, map[string]any) {
	t.Helper()
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, "http://"+host+path, bytes.NewReader(b))
	r.Host = host
	r.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestOwnerRoutesAndServiceRoutes(t *testing.T) {
	st, _ := entity.Open(t.TempDir(), time.Hour)
	h := routes(st, map[string]string{engineKey: "engine"}, "pagetoken")
	page := map[string]string{"X-Identity-Page": "pagetoken"}
	create := map[string]string{"name": "Nova", "passphrase": "correct horse battery"}
	// from another container (Host identity:8796): owner routes refused
	if c, _ := req(t, h, "POST", "identity:8796", "/v1/entity", page, create); c != 403 {
		t.Fatalf("create from the network: %d", c)
	}
	// without the page token: refused
	if c, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", nil, create); c != 403 {
		t.Fatalf("create without page token: %d", c)
	}
	// services can't authorize anything before an entity exists
	auth := map[string]string{"Authorization": "Bearer " + engineKey}
	if _, d := req(t, h, "POST", "identity:8796", "/v1/authorize", auth, map[string]string{"action": "model.request"}); d["allow"] != false {
		t.Fatalf("no entity: %v", d)
	}
	if c, d := req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", page, create); c != 201 || d["state"] != "UNLOCKED" {
		t.Fatalf("create: %d %v", c, d)
	}
	if c, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", page, create); c != 409 {
		t.Fatal("second create must be 409")
	}
	if c, _ := req(t, h, "POST", "identity:8796", "/v1/authorize", nil, map[string]string{"action": "model.request"}); c != 401 {
		t.Fatal("authorize without a service key")
	}
	_, d := req(t, h, "POST", "identity:8796", "/v1/authorize", auth, map[string]string{"action": "model.request", "detail": "test"})
	if d["allow"] != true || d["service"] != "engine" || !strings.HasPrefix(d["decision_id"].(string), "dec_") {
		t.Fatalf("authorize: %v", d)
	}
	_, s := req(t, h, "GET", "identity:8796", "/v1/state", nil, nil)
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "verifier") || strings.Contains(string(b), "credential") {
		t.Fatalf("state leaks secrets: %s", b)
	}
	req(t, h, "POST", "127.0.0.1:8796", "/v1/lock", page, nil)
	if _, d := req(t, h, "POST", "identity:8796", "/v1/authorize", auth, map[string]string{"action": "model.request"}); d["allow"] != false {
		t.Fatal("locked must deny")
	}
}

func TestLearningGrantsEndpointRequiresOwnerProof(t *testing.T) {
	st, _ := entity.Open(t.TempDir(), time.Hour)
	h := routes(st, map[string]string{engineKey: "core"}, "page-secret")
	ownerHeader := map[string]string{"X-Identity-Page": "page-secret"}
	body := map[string]string{"passphrase":"correct horse battery"}
	if code, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/learning", ownerHeader, body); code != 403 {
		t.Fatalf("nonexistent entity should be refused: %d", code)
	}
	if code, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/entity", ownerHeader,
		map[string]string{"name":"TASS", "passphrase":"correct horse battery"}); code != 201 { t.Fatal("create") }
	if code, _ := req(t, h, "POST", "identity:8796", "/v1/grants/learning", ownerHeader, body); code != 403 { t.Fatal("not owner host") }
	if code, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/learning", nil, body); code != 403 { t.Fatal("CSRF token missing") }
	if code, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/learning", ownerHeader,
		map[string]string{"passphrase":"wrong password"}); code != 403 { t.Fatal("incorrect passphrase") }
	if code, response := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/learning", ownerHeader, body); code != 200 || response["state"] != "UNLOCKED" {
		t.Fatalf("grant: %d %v", code, response)
	}
	_, a := req(t, h, "POST", "identity:8796", "/v1/authorize",
		map[string]string{"Authorization":"Bearer "+engineKey}, map[string]string{"action":"lesson.learn"})
	if a["allow"] != true { t.Fatalf("grant not usable: %v", a) }
	req(t, h, "POST", "127.0.0.1:8796", "/v1/lock", ownerHeader, nil)
	if code, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/grants/learning", ownerHeader, body); code != 403 { t.Fatal("locked entity") }
}

// The learning card must say what changes (including that github.sync can write the
// entity's document), report its own result next to the button, and keep the
// unrelated Issue #51 reference approval clearly separate from TASS's lessons.
func TestLearningCardExplainsChangeAndSeparatesIssue51(t *testing.T) {
	b, err := page.ReadFile("page.html")
	if err != nil {
		t.Fatal(err)
	}
	p := string(b)
	for _, want := range []string{
		`id="grantMsg"`, `id="grantDone"`, "This can write his document to that repository",
		"Nothing was sent to Identity", "Identity refused it: ", "never stored",
		"Separate developer test (not TASS's lessons)",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("learning card is missing %q", want)
		}
	}
	if strings.Contains(p, `input.value = "";
  if (!passphrase)`) {
		t.Error("the passphrase must not be cleared before it is checked/sent")
	}
}
