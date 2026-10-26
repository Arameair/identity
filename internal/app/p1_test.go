package app

// P1 tests: a real Identity HTTP server on a real loopback port. GitHub is a
// LOCAL TEST FIXTURE that can sign in as two different accounts. The "Admin"
// is this test typing the passphrase into Identity's approval page over HTTP.

import (
	"encoding/json"
	"io"
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

	"github.com/Arameair/identity/internal/authority"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/pairing"
	"github.com/Arameair/identity/internal/profile"
)

func init() { authority.SetIterationsForTests(1000) }

const adminPass = "correct horse battery staple"

type ghSwitch struct {
	srv   *httptest.Server
	user  atomic.Value
	polls atomic.Int32
}

func newGHSwitch(t *testing.T) *ghSwitch {
	g := &ghSwitch{}
	g.user.Store("owner-a")
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
		g.polls.Store(0)
		json.NewEncoder(w).Encode(map[string]any{"device_code": "fixdev", "user_code": "FIXT-0001", "verification_uri": g.srv.URL + "/login/device", "expires_in": 60, "interval": 5})
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if g.polls.Add(1) < 2 {
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "ghu_FIXTURE_" + g.user.Load().(string), "expires_in": 28800})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		u := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ghu_FIXTURE_")
		id := map[string]int{"owner-a": 1001, "other-b": 2002}[u]
		json.NewEncoder(w).Encode(map[string]any{"login": u, "id": id})
	})
	mux.HandleFunc("GET /user/installations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"installations": []any{}})
	})
	mux.HandleFunc("GET /repos/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

func (g *ghSwitch) cfg() github.Config {
	return github.Config{ClientID: "IvFIXTURE0001", ClientIDSource: "test", ClientIDKind: "built-in", WebBase: g.srv.URL, APIBase: g.srv.URL}
}

func (h *harness) githubAs(g *ghSwitch, user string) {
	h.t.Helper()
	h.do("POST", "/api/github/disconnect", `{}`, nil)
	g.user.Store(user)
	h.githubSignIn()
}

// approve plays the Admin in Identity's own page: open the one-time link,
// then decide. Returns the decide status and body.
func (h *harness) approve(link string, body map[string]any) (int, map[string]any) {
	h.t.Helper()
	frag := link[strings.Index(link, "#")+1:]
	var id, tok string
	for _, kv := range strings.Split(frag, "&") {
		if v, ok := strings.CutPrefix(kv, "a="); ok {
			id = v
		}
		if v, ok := strings.CutPrefix(kv, "t="); ok {
			tok = v
		}
	}
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	post := func(path string, v any, csrf string) (int, map[string]any) {
		b, _ := json.Marshal(v)
		req, _ := http.NewRequest("POST", h.base+path, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", h.base)
		if csrf != "" {
			req.Header.Set("X-Identity-CSRF", csrf)
		}
		resp, err := c.Do(req)
		if err != nil {
			h.t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var m map[string]any
		json.Unmarshal(raw, &m)
		return resp.StatusCode, m
	}
	st, m := post("/api/approval/open", map[string]string{"id": id, "token": tok}, "")
	h.lastView = m
	if st != 200 {
		return st, m
	}
	return post("/api/approval/decide", body, m["csrf"].(string))
}

func approveBody(pass string) map[string]any {
	return map[string]any{"decision": "approve", "passphrase": pass}
}

func enrollBody(pass string) map[string]any {
	return map[string]any{"decision": "approve", "passphrase": pass, "passphrase_confirm": pass, "acknowledge": true}
}

// inst calls an installation route signed with an installation credential and
// verifies Identity's signed answer.
func (h *harness) inst(method, path, body, id, secret string) (int, map[string]any) {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.base+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rk, sk := pairing.DeriveKeys(secret)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := profile.RandHex(12)
	req.Header.Set(InstallationIDHeader, id)
	req.Header.Set(TimeHeader, ts)
	req.Header.Set(NonceHeader, nonce)
	req.Header.Set(AuthHeader, pairing.RequestMAC(rk, method, req.URL.RequestURI(), ts, nonce, []byte(body)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	json.Unmarshal(raw, &m)
	if resp.StatusCode != 401 && resp.StatusCode != 503 {
		if pairing.ResponseProof(sk, nonce, resp.StatusCode, raw) != resp.Header.Get(ProofHeader) {
			h.t.Fatalf("%s %s: installation answer not signed (%d %s)", method, path, resp.StatusCode, raw)
		}
	}
	return resp.StatusCode, m
}

func jsonOf(v any) string { b, _ := json.Marshal(v); return string(b) }

type p1env struct {
	h      *harness
	con    *console
	g      *ghSwitch
	tokA   string
	tokB   string
	ent    string
	other  string
	admin  string
	insID  string
	insSec string
	dir    string
}

// setup: Identity + GitHub fixture, two paired Processors, admin enrolled,
// two identities.
func setupP1(t *testing.T) *p1env {
	e := &p1env{dir: t.TempDir(), g: newGHSwitch(t)}
	e.h, e.con = startV1(t, e.dir, e.g.cfg())
	e.h.signIn()
	e.tokA, _ = e.h.pairV1(e.con, "tp_aaaa")
	time.Sleep(pairRequestGap)
	e.tokB, _ = e.h.pairV1(e.con, "tp_bbbb")
	e.h.githubAs(e.g, "owner-a")
	st, m := e.h.v1("POST", "/api/v1/admin/enrollment", `{}`, e.tokA)
	if st != 202 {
		t.Fatalf("enrollment request %d %v", st, m)
	}
	if st, m := e.h.approve(m["approval_url"].(string), enrollBody(adminPass)); st != 200 {
		t.Fatalf("enroll %d %v", st, m)
	}
	_, a := e.h.v1("GET", "/api/v1/admin", "", e.tokA)
	e.admin = a["admin"].(map[string]any)["admin_id"].(string)
	_, x := e.h.v1("POST", "/api/v1/identities", `{"name":"Tass","kind":"companion"}`, e.tokA)
	e.ent = x["id"].(string)
	_, y := e.h.v1("POST", "/api/v1/identities", `{"name":"Other","kind":"person"}`, e.tokA)
	e.other = y["id"].(string)
	return e
}

func installReq(key string) map[string]any {
	return map[string]any{"feature_id": "p1-deterministic-worker", "name": "P1 Deterministic Worker (TEST FIXTURE)", "version": "0.1.0",
		"api": FeatureAPI, "publisher_claim": "Arameair P1 test harness", "source_kind": "local-trusted-package", "source_ref": "testfeatures/deterministic-worker",
		"trust_class": authority.TrustLocalFixture, "artifact_sha256": strings.Repeat("a", 64), "manifest_sha256": strings.Repeat("b", 64),
		"install_key": key, "capabilities": []map[string]string{{"id": "worker.test.synthetic", "description": "synthetic", "data_class": "SYNTHETIC"}}}
}

// install runs request → Admin approval → commit → confirm; returns id, secret.
func (e *p1env) install(t *testing.T, key string) (string, string) {
	st, m := e.h.v1("POST", "/api/v1/installations/requests", jsonOf(installReq(key)), e.tokA)
	if st != 202 {
		t.Fatalf("install request %d %v", st, m)
	}
	apID := m["approval"].(map[string]any)["id"].(string)
	if st, _ := e.h.approve(m["approval_url"].(string), approveBody(adminPass)); st != 200 {
		t.Fatalf("approve install %d", st)
	}
	st, c := e.h.v1("POST", "/api/v1/installations", `{"approval_id":"`+apID+`"}`, e.tokA)
	if st != 200 {
		t.Fatalf("install commit %d %v", st, c)
	}
	in := c["installation"].(map[string]any)
	cred := c["credential"].(map[string]any)
	id, sec := in["installation_id"].(string), cred["secret"].(string)
	if st, m := e.h.inst("POST", "/api/v1/installation/confirm", `{}`, id, sec); st != 200 {
		t.Fatalf("confirm %d %v", st, m)
	}
	return id, sec
}

func (e *p1env) grant(t *testing.T, tok, dest, subject, capability string, expires int) string {
	body := jsonOf(map[string]any{"destination_installation_id": dest, "subject_identity_id": subject, "capability": capability, "expires_in_seconds": expires})
	st, m := e.h.v1("POST", "/api/v1/grants/requests", body, tok)
	if st != 202 {
		t.Fatalf("grant request %d %v", st, m)
	}
	apID := m["approval"].(map[string]any)["id"].(string)
	if st, _ := e.h.approve(m["approval_url"].(string), approveBody(adminPass)); st != 200 {
		t.Fatalf("approve grant %d", st)
	}
	st, g := e.h.v1("POST", "/api/v1/grants", `{"approval_id":"`+apID+`"}`, tok)
	if st != 201 {
		t.Fatalf("grant commit %d %v", st, g)
	}
	return g["grant"].(map[string]any)["grant_id"].(string)
}

func (e *p1env) ticket(tok, dest, subject, capability, digest string) (int, map[string]any) {
	return e.h.v1("POST", "/api/v1/tickets", jsonOf(map[string]string{"destination_installation_id": dest, "subject_identity_id": subject,
		"capability": capability, "request_sha256": digest}), tok)
}

func (e *p1env) check(insID, secret string, tk map[string]any, requester, capability, subject, digest string) (int, map[string]any) {
	return e.h.inst("POST", "/api/v1/authz/check", jsonOf(map[string]string{"ticket_id": tk["ticket_id"].(string), "grant_id": tk["grant_id"].(string),
		"requester": requester, "capability": capability, "subject_identity_id": subject, "request_sha256": digest}), insID, secret)
}

func TestP1AdminEnrollmentOnceNoTakeover(t *testing.T) {
	g := newGHSwitch(t)
	dir := t.TempDir()
	h, con := startV1(t, dir, g.cfg())
	h.signIn()
	tok, _ := h.pairV1(con, "tp_aaaa")

	if _, a := h.v1("GET", "/api/v1/admin", "", tok); a["state"] != "NOT_ENROLLED" {
		t.Fatal(a)
	}
	if st, m := h.v1("POST", "/api/v1/admin/enrollment", `{}`, tok); st != 409 || m["error"] != "GITHUB_NOT_VERIFIED" {
		t.Fatalf("enrollment without GitHub: %d %v", st, m)
	}
	h.githubAs(g, "owner-a")
	_, m := h.v1("POST", "/api/v1/admin/enrollment", `{}`, tok)
	link := m["approval_url"].(string)
	// The pairing key can't approve: the decide route needs the page session.
	if st, _, _ := h.machine("POST", "/api/approval/decide", `{"decision":"approve","passphrase":"x"}`, tok); st != 403 && st != 401 {
		t.Fatalf("processor reached decide: %d", st)
	}
	if st, _ := h.approve(link, map[string]any{"decision": "approve", "passphrase": "short", "passphrase_confirm": "short", "acknowledge": true}); st != 400 {
		t.Fatal("weak passphrase accepted")
	}
	// The one-time link is spent; a new request gives a new link.
	if st, _ := h.approve(link, enrollBody(adminPass)); st != 401 {
		t.Fatal("approval link reused")
	}
	_, m = h.v1("POST", "/api/v1/admin/enrollment", `{}`, tok)
	link2 := m["approval_url"].(string)
	_, m = h.v1("POST", "/api/v1/admin/enrollment", `{}`, tok) // a racing second request
	link3 := m["approval_url"].(string)
	if st, r := h.approve(link2, map[string]any{"decision": "approve", "passphrase": adminPass, "passphrase_confirm": adminPass}); st != 400 {
		t.Fatalf("enrollment without explicit confirmation: %d %v", st, r)
	}
	_, m = h.v1("POST", "/api/v1/admin/enrollment", `{}`, tok)
	if st, r := h.approve(m["approval_url"].(string), enrollBody(adminPass)); st != 200 {
		t.Fatalf("enroll %d %v", st, r)
	}
	_, a := h.v1("GET", "/api/v1/admin", "", tok)
	adm := a["admin"].(map[string]any)
	if a["state"] != "ENROLLED" || adm["github_binding"].(map[string]any)["id"].(float64) != 1001 {
		t.Fatal(a)
	}
	if st, _ := h.approve(link3, enrollBody("another strong passphrase")); st != 409 {
		t.Fatalf("second racing enrollment succeeded: %d", st)
	}
	// Account switch: the other account gets no Admin authority and cannot enroll.
	h.githubAs(g, "other-b")
	_, a2 := h.v1("GET", "/api/v1/admin", "", tok)
	if a2["admin"].(map[string]any)["admin_id"] != adm["admin_id"] || a2["current_github"].(map[string]any)["is_admin_binding"] != false {
		t.Fatalf("account switch changed admin: %v", a2)
	}
	if st, m := h.v1("POST", "/api/v1/admin/enrollment", `{}`, tok); st != 409 || m["error"] != "ALREADY_ENROLLED" {
		t.Fatalf("re-enrollment after switch: %d %v", st, m)
	}
	// Restart: same Admin.
	h.stop()
	h, _ = startV1(t, dir, g.cfg())
	_, a3 := h.v1("GET", "/api/v1/admin", "", tok)
	if a3["admin"].(map[string]any)["admin_id"] != adm["admin_id"] {
		t.Fatal("admin changed across restart")
	}
	// Corrupted admin record: never treated as "not enrolled".
	h.stop()
	os.WriteFile(filepath.Join(dir, "admin.json"), []byte("{damaged"), 0o600)
	h, _ = startV1(t, dir, g.cfg())
	h.signIn()
	h.githubAs(g, "other-b")
	if _, a4 := h.v1("GET", "/api/v1/admin", "", tok); a4["state"] != "DAMAGED" {
		t.Fatal(a4)
	}
	if st, _ := h.v1("POST", "/api/v1/admin/enrollment", `{}`, tok); st != 503 {
		t.Fatalf("enrollment over damaged admin record: %d", st)
	}
	h.stop()
}

func TestP1ElevatedRoutesRequireAdminApproval(t *testing.T) {
	e := setupP1(t)
	defer e.h.stop()
	h := e.h
	// Inventory: every route classified, elevated ones marked.
	for _, r := range h.a.Routes() {
		if strings.HasPrefix(r.Guard, "UNCLASSIFIED") {
			t.Errorf("route %s has no documented guard", r.Pattern)
		}
	}
	seen := map[string]bool{}
	for _, r := range h.a.Routes() {
		seen[r.Pattern] = true
	}
	for p := range RouteGuards {
		if !seen[p] {
			t.Errorf("RouteGuards lists %s, which is not registered", p)
		}
	}
	for _, p := range ElevatedRoutes {
		if RouteGuards[p] != gElevated {
			t.Errorf("%s not marked elevated", p)
		}
	}
	// Paired Processor without Admin approval: denied, whatever it claims.
	for _, path := range []string{"/api/v1/installations", "/api/v1/grants", "/api/v1/installations/ins_00000000000000000000/uninstall",
		"/api/v1/grants/gnt_00000000000000000000/admin-revoke"} {
		for _, body := range []string{`{}`, `{"approved":true}`, `{"approval_id":"apr_00000000000000000000"}`} {
			st, m := h.v1("POST", path, body, e.tokA)
			if st != 403 && st != 409 {
				t.Fatalf("%s %s: %d %v", path, body, st, m)
			}
		}
	}
	// Pending (not yet approved) approval: denied.
	_, m := h.v1("POST", "/api/v1/installations/requests", jsonOf(installReq("00112233445566778899aabbccddeeff")), e.tokA)
	apID := m["approval"].(map[string]any)["id"].(string)
	link := m["approval_url"].(string)
	if st, _ := h.v1("POST", "/api/v1/installations", `{"approval_id":"`+apID+`"}`, e.tokA); st != 403 {
		t.Fatal("pending approval accepted")
	}
	// Wrong passphrase: still pending.
	if st, _ := h.approve(link, approveBody("not the admin passphrase")); st != 401 {
		t.Fatal("wrong passphrase approved")
	}
	_, m = h.v1("POST", "/api/v1/installations/requests", jsonOf(installReq("00112233445566778899aabbccddeeff")), e.tokA)
	apID, link = m["approval"].(map[string]any)["id"].(string), m["approval_url"].(string)
	if st, _ := h.approve(link, approveBody(adminPass)); st != 200 {
		t.Fatal("approve failed")
	}
	// Another paired Processor can't use it; the wrong route kind can't either.
	if st, _ := h.v1("POST", "/api/v1/installations", `{"approval_id":"`+apID+`"}`, e.tokB); st != 403 {
		t.Fatal("other processor used the approval")
	}
	if st, _ := h.v1("POST", "/api/v1/grants", `{"approval_id":"`+apID+`"}`, e.tokA); st != 403 {
		t.Fatal("install approval used for a grant")
	}
	// An installation credential can't call Processor routes, and vice versa.
	st, c := h.v1("POST", "/api/v1/installations", `{"approval_id":"`+apID+`"}`, e.tokA)
	if st != 200 {
		t.Fatalf("commit %d %v", st, c)
	}
	insID := c["installation"].(map[string]any)["installation_id"].(string)
	sec := c["credential"].(map[string]any)["secret"].(string)
	testPIDs.Store(sec, insID)
	if st, _ := h.v1("GET", "/api/v1/status", "", sec); st != 401 {
		t.Fatal("installation credential accepted on a Processor route")
	}
	if st, _ := h.inst("GET", "/api/v1/installation/self", "", "tp_aaaa", e.tokA); st != 401 {
		t.Fatal("processor key accepted on an installation route")
	}
	// Idempotent retry of the same approval: same installation, rotated
	// credential (old one stops working), never a second ID.
	st, c2 := h.v1("POST", "/api/v1/installations", `{"approval_id":"`+apID+`"}`, e.tokA)
	if st != 200 || c2["installation"].(map[string]any)["installation_id"] != insID || c2["created"] != false {
		t.Fatalf("retry: %d %v", st, c2)
	}
	sec2 := c2["credential"].(map[string]any)["secret"].(string)
	if st, _ := h.inst("POST", "/api/v1/installation/confirm", `{}`, insID, sec); st != 401 {
		t.Fatal("rotated-out credential still works")
	}
	if st, _ := h.inst("POST", "/api/v1/installation/confirm", `{}`, insID, sec2); st != 200 {
		t.Fatal("confirm failed")
	}
	if st, c3 := h.v1("POST", "/api/v1/installations", `{"approval_id":"`+apID+`"}`, e.tokA); st != 200 || c3["credential"] != nil {
		t.Fatalf("after confirm a retry must not issue a credential: %v", c3)
	}
	_, l := h.v1("GET", "/api/v1/installations", "", e.tokA)
	if l["count"].(float64) != 1 {
		t.Fatalf("duplicate installations: %v", l)
	}
	// Untrusted trust class: blocked before any approval.
	bad := installReq("ffeeddccbbaa99887766554433221100")
	bad["trust_class"] = "THIRD_PARTY_RELEASE"
	if st, m := h.v1("POST", "/api/v1/installations/requests", jsonOf(bad), e.tokA); st != 403 || !strings.Contains(m["message"].(string), "UNTRUSTED_EXECUTION_BLOCKED") {
		t.Fatalf("untrusted install request: %d %v", st, m)
	}
}

func TestP1GrantTicketCheckRevokeOutageRestart(t *testing.T) {
	e := setupP1(t)
	h := e.h
	ins1, sec1 := e.install(t, "00112233445566778899aabbccddeeff")
	ins2, sec2 := e.install(t, "ffeeddccbbaa99887766554433221100")
	if ins1 == ins2 {
		t.Fatal("two copies share an installation ID")
	}
	const capW = "worker.test.synthetic"
	dig := RequestDigest([]byte(`{"request_id":"r1","input":"hello"}`))
	_, self := h.inst("GET", "/api/v1/installation/self", "", ins1, sec1)
	if self["active_grants"].(float64) != 0 || len(self["information_sources"].([]any)) != 0 {
		t.Fatal("new installation must start with zero grants and sources")
	}
	// Before any grant: DENIED, no ticket.
	if st, m := e.ticket(e.tokA, ins1, e.ent, capW, dig); st != 403 || m["reason"] != authority.DenyNoGrant || m["ticket"] != nil {
		t.Fatalf("ticket before grant: %d %v", st, m)
	}
	// Unknown capability can't even be requested.
	if st, _ := h.v1("POST", "/api/v1/grants/requests", jsonOf(map[string]any{"destination_installation_id": ins1, "subject_identity_id": e.ent, "capability": "worker.test.other"}), e.tokA); st != 400 {
		t.Fatal("undeclared capability requested")
	}
	gid := e.grant(t, e.tokA, ins1, e.ent, capW, 0)

	st, tk := e.ticket(e.tokA, ins1, e.ent, capW, dig)
	if st != 200 || tk["decision"] != "ALLOW_TICKET" {
		t.Fatalf("ticket %d %v", st, tk)
	}
	if p, err := authority.ParseTicket(tk["ticket"].(string), pairing.TicketKey(sec1)); err != nil || p.Audience != ins1 || p.Digest != dig {
		t.Fatalf("ticket does not verify with the recipient's key: %v", err)
	}
	if _, err := authority.ParseTicket(tk["ticket"].(string), pairing.TicketKey(sec2)); err == nil {
		t.Fatal("ticket verifies for another installation")
	}
	// Wrong recipient, wrong digest, wrong requester claim: DENIED.
	if st, m := e.check(ins2, sec2, tk, "processor:tp_aaaa", capW, e.ent, dig); st != 403 || m["reason"] != authority.DenyWrongAudience {
		t.Fatalf("wrong recipient: %d %v", st, m)
	}
	if st, m := e.check(ins1, sec1, tk, "processor:tp_aaaa", capW, e.ent, strings.Repeat("0", 64)); st != 403 || m["reason"] != authority.DenyMismatch {
		t.Fatalf("wrong digest: %d %v", st, m)
	}
	if st, m := e.check(ins1, sec1, tk, "processor:tp_bbbb", capW, e.ent, dig); st != 403 || m["reason"] != authority.DenyMismatch {
		t.Fatalf("spoofed requester: %d %v", st, m)
	}
	// The right one: ALLOW, once.
	if st, m := e.check(ins1, sec1, tk, "processor:tp_aaaa", capW, e.ent, dig); st != 200 || m["decision"] != "ALLOW" || m["grant_id"] != gid {
		t.Fatalf("allow: %d %v", st, m)
	}
	if st, m := e.check(ins1, sec1, tk, "processor:tp_aaaa", capW, e.ent, dig); st != 403 || m["reason"] != authority.DenyTicketReplayed {
		t.Fatalf("replay: %d %v", st, m)
	}
	// Other requester, other subject, other recipient, other capability: no ticket.
	for _, c := range []struct{ tok, dest, sub, capb string }{
		{e.tokB, ins1, e.ent, capW}, {e.tokA, ins1, e.other, capW}, {e.tokA, ins2, e.ent, capW}, {e.tokA, ins1, e.ent, "worker.test.other"},
	} {
		if st, m := e.ticket(c.tok, c.dest, c.sub, c.capb, dig); st != 403 || m["ticket"] != nil {
			t.Fatalf("out-of-scope ticket issued: %+v %d %v", c, st, m)
		}
	}
	// Ticket outstanding when the grant is revoked: dies with it.
	_, tk2 := e.ticket(e.tokA, ins1, e.ent, capW, dig)
	if st, _ := h.v1("POST", "/api/v1/grants/"+gid+"/revoke", `{"reason":"test"}`, e.tokA); st != 200 {
		t.Fatal("revoke failed")
	}
	if st, m := e.check(ins1, sec1, tk2, "processor:tp_aaaa", capW, e.ent, dig); st != 403 || m["decision"] == "ALLOW" {
		t.Fatalf("ticket survived revocation: %v", m)
	}
	if st, m := e.ticket(e.tokA, ins1, e.ent, capW, dig); st != 403 || m["reason"] != authority.DenyRevoked {
		t.Fatalf("after revoke: %d %v", st, m)
	}
	// Expiring grant.
	e.grant(t, e.tokA, ins1, e.ent, capW, 5)
	if st, _ := e.ticket(e.tokA, ins1, e.ent, capW, dig); st != 200 {
		t.Fatal("fresh expiring grant should work")
	}
	h.a.Grants.Now = func() time.Time { return time.Now().Add(6 * time.Second) }
	if st, m := e.ticket(e.tokA, ins1, e.ent, capW, dig); st != 403 || m["reason"] != authority.DenyExpired {
		t.Fatalf("expired grant: %d %v", st, m)
	}
	h.a.Grants.Now = time.Now
	// A live grant again, a ticket, then Identity restarts: grants persist,
	// tickets don't (fail closed), the installation keeps its ID and key.
	gid3 := e.grant(t, e.tokA, ins1, e.ent, capW, 0)
	_, tk3 := e.ticket(e.tokA, ins1, e.ent, capW, dig)
	h.stop()
	h, _ = startV1(t, e.dir, e.g.cfg())
	e.h = h
	if st, m := e.check(ins1, sec1, tk3, "processor:tp_aaaa", capW, e.ent, dig); st != 403 || m["reason"] != authority.DenyTicketUnknown {
		t.Fatalf("ticket survived restart: %d %v", st, m)
	}
	_, tk4 := e.ticket(e.tokA, ins1, e.ent, capW, dig)
	// (the 5-second grant is still active in real time, so either live grant may answer)
	if st, m := e.check(ins1, sec1, tk4, "processor:tp_aaaa", capW, e.ent, dig); st != 200 || m["grant_id"] != tk4["grant_id"] || gid3 == "" {
		t.Fatalf("after restart: %d %v", st, m)
	}
	_, l := h.v1("GET", "/api/v1/installations", "", e.tokA)
	if l["count"].(float64) != 2 {
		t.Fatalf("installations after restart: %v", l)
	}
	// Archived subject: denied even with a grant.
	h.v1("POST", "/api/v1/identities/"+e.ent+"/archive", `{}`, e.tokA)
	if st, m := e.ticket(e.tokA, ins1, e.ent, capW, dig); st != 403 || m["reason"] != authority.DenySubject {
		t.Fatalf("archived subject: %d %v", st, m)
	}
	h.v1("POST", "/api/v1/identities/"+e.ent+"/restore", `{}`, e.tokA)
	// Uninstall (elevated): grants revoked, credential gone, tombstone kept.
	_, m := h.v1("POST", "/api/v1/installations/"+ins1+"/uninstall/requests", `{}`, e.tokA)
	apID := m["approval"].(map[string]any)["id"].(string)
	h.approve(m["approval_url"].(string), approveBody(adminPass))
	if st, _ := h.v1("POST", "/api/v1/installations/"+ins2+"/uninstall", `{"approval_id":"`+apID+`"}`, e.tokA); st != 403 {
		t.Fatal("uninstall approval used on another installation")
	}
	_, m = h.v1("POST", "/api/v1/installations/"+ins1+"/uninstall/requests", `{}`, e.tokA)
	apID = m["approval"].(map[string]any)["id"].(string)
	h.approve(m["approval_url"].(string), approveBody(adminPass))
	st, u := h.v1("POST", "/api/v1/installations/"+ins1+"/uninstall", `{"approval_id":"`+apID+`"}`, e.tokA)
	if st != 200 || u["installation"].(map[string]any)["state"] != authority.InsUninstalled || u["grants_revoked"].(float64) < 1 {
		t.Fatalf("uninstall: %d %v", st, u)
	}
	if st, _ := h.inst("GET", "/api/v1/installation/self", "", ins1, sec1); st != 401 {
		t.Fatal("uninstalled credential still works")
	}
	if st, m := e.ticket(e.tokA, ins1, e.ent, capW, dig); st != 403 || m["reason"] != authority.DenyDestination {
		t.Fatalf("ticket for uninstalled: %v", m)
	}
	// Grants storage damaged: fail closed, not allow.
	h.stop()
	os.WriteFile(filepath.Join(e.dir, "grants.json"), []byte("{bad"), 0o600)
	h, _ = startV1(t, e.dir, e.g.cfg())
	defer h.stop()
	e.h = h
	if st, m := e.ticket(e.tokA, ins2, e.ent, capW, dig); st != 503 || m["decision"] != "DENIED" {
		t.Fatalf("damaged grants: %d %v", st, m)
	}
}

// Grok M2 / M3 / L2 (P1 cleanup).
func TestP1CleanupRevocationDisplayExpiryPairing(t *testing.T) {
	e := setupP1(t)
	h := e.h
	defer func() { e.h.stop() }()
	ins1, sec1 := e.install(t, "00112233445566778899aabbccddeeff")
	ins2, sec2 := e.install(t, "ffeeddccbbaa99887766554433221100")
	const capW = "worker.test.synthetic"
	status := func(gid string) (string, map[string]any) {
		_, l := h.v1("GET", "/api/v1/grants", "", e.tokA)
		for _, x := range l["grants"].([]any) {
			m := x.(map[string]any)
			if g := m["grant"].(map[string]any); g["grant_id"] == gid {
				return m["status"].(string), g
			}
		}
		return "", nil
	}

	// M3: no expiry requested → finite 1-hour default; the primary
	// Processor's request shows its paired label and no warning.
	gid := e.grant(t, e.tokA, ins1, e.ent, capW, 0)
	disp := h.lastView["display"].(map[string]any)
	req := disp["requester"].(map[string]any)
	if req["label"] != "Test Processor" || req["verified"] != true || req["is_primary"] != true || req["warning"] != nil {
		t.Fatalf("primary requester display: %v", req)
	}
	if disp["advanced"].(map[string]any)["action_sha256"] == nil || disp["capability"] == nil || disp["subject"] == nil {
		t.Fatalf("display missing parts: %v", disp)
	}
	_, g := status(gid)
	exp, err := time.Parse(time.RFC3339, g["expires_at"].(string))
	if err != nil || exp.Sub(time.Now()) < 55*time.Minute || exp.Sub(time.Now()) > 61*time.Minute {
		t.Fatalf("default expiry: %v %v", g["expires_at"], err)
	}

	// M2: an unrelated paired program and an unrelated installation cannot revoke.
	if st, m := h.v1("POST", "/api/v1/grants/"+gid+"/revoke", `{"reason":"not mine"}`, e.tokB); st != 403 || m["error"] != "REVOKE_NOT_PERMITTED" {
		t.Fatalf("B revoked A's grant: %d %v", st, m)
	}
	if st, _ := h.inst("POST", "/api/v1/installation/grants/"+gid+"/revoke", `{}`, ins2, sec2); st != 403 {
		t.Fatalf("unrelated installation revoked: %d", st)
	}
	if s, _ := status(gid); s != "ACTIVE" {
		t.Fatalf("grant changed after refused revokes: %s", s)
	}
	// Admin emergency path, requested by B: approval page warns B is not primary.
	st, m := h.v1("POST", "/api/v1/grants/"+gid+"/revoke/requests", `{"reason":"emergency"}`, e.tokB)
	if st != 202 {
		t.Fatalf("admin revoke request %d %v", st, m)
	}
	apID := m["approval"].(map[string]any)["id"].(string)
	if st, _ := h.v1("POST", "/api/v1/grants/"+gid+"/admin-revoke", `{"approval_id":"`+apID+`"}`, e.tokB); st != 403 {
		t.Fatal("admin revoke without approval")
	}
	if st, _ := h.approve(m["approval_url"].(string), approveBody(adminPass)); st != 200 {
		t.Fatal("approve admin revoke")
	}
	if w := h.lastView["display"].(map[string]any)["requester"].(map[string]any)["warning"]; w == nil || !strings.Contains(w.(string), "NOT come from your primary") {
		t.Fatalf("no warning for non-primary requester: %v", h.lastView["display"])
	}
	st, r := h.v1("POST", "/api/v1/grants/"+gid+"/admin-revoke", `{"approval_id":"`+apID+`"}`, e.tokB)
	if st != 200 || !strings.HasPrefix(r["grant"].(map[string]any)["revoked_by"].(string), "admin:"+e.admin) {
		t.Fatalf("admin revoke: %d %v", st, r)
	}
	// The destination feature may revoke grants naming it; revoked_by is recorded.
	gid2 := e.grant(t, e.tokA, ins1, e.ent, capW, 600)
	st, r = h.inst("POST", "/api/v1/installation/grants/"+gid2+"/revoke", `{"reason":"feature ended it"}`, ins1, sec1)
	if st != 200 || r["grant"].(map[string]any)["revoked_by"] != "installation:"+ins1 {
		t.Fatalf("destination revoke: %d %v", st, r)
	}
	// The requester may revoke its own grant.
	gid3 := e.grant(t, e.tokA, ins1, e.ent, capW, 600)
	if st, r := h.v1("POST", "/api/v1/grants/"+gid3+"/revoke", `{}`, e.tokA); st != 200 || r["grant"].(map[string]any)["revoked_by"] != "processor:tp_aaaa" {
		t.Fatalf("own revoke: %d %v", st, r)
	}

	// M3: "until revoked" needs the extra confirmation on Identity's page.
	st, m = h.v1("POST", "/api/v1/grants/requests", jsonOf(map[string]any{"destination_installation_id": ins1, "subject_identity_id": e.ent,
		"capability": capW, "until_revoked": true}), e.tokA)
	if st != 202 {
		t.Fatalf("until_revoked request %d %v", st, m)
	}
	apID, link := m["approval"].(map[string]any)["id"].(string), m["approval_url"].(string)
	if st, _ := h.approve(link, approveBody(adminPass)); st != 400 {
		t.Fatalf("until-revoked approved without confirmation: %d", st)
	}
	if st, _ := h.v1("POST", "/api/v1/grants", `{"approval_id":"`+apID+`"}`, e.tokA); st != 403 {
		t.Fatal("unconfirmed until-revoked grant committed")
	}
	// (the one-time link is spent; a fresh request, approved with the tick)
	_, m = h.v1("POST", "/api/v1/grants/requests", jsonOf(map[string]any{"destination_installation_id": ins1, "subject_identity_id": e.ent,
		"capability": capW, "until_revoked": true}), e.tokA)
	apID, link = m["approval"].(map[string]any)["id"].(string), m["approval_url"].(string)
	body := approveBody(adminPass)
	body["confirm_until_revoked"] = true
	if st, _ := h.approve(link, body); st != 200 {
		t.Fatal("confirmed until-revoked refused")
	}
	st, r = h.v1("POST", "/api/v1/grants", `{"approval_id":"`+apID+`"}`, e.tokA)
	if gg := r["grant"].(map[string]any); st != 201 || gg["expires_at"] != nil || gg["until_revoked"] != true {
		t.Fatalf("until-revoked commit %d %v", st, r)
	}

	// L2: a pairing code works only for the program it was shown for; a wrong
	// program's attempt neither uses it nor counts toward the lockout.
	time.Sleep(pairRequestGap)
	if st, _, b := h.machine("POST", "/api/v1/pair/request", `{"processor_id":"tp_cccc","processor_label":"Program C"}`, ""); st != 202 {
		t.Fatalf("pair request %d %s", st, b)
	}
	code := e.con.lastCode(t)
	for i := 0; i < 6; i++ {
		if st, _, _ := h.machine("POST", "/api/v1/pair", `{"code":"`+code+`","processor_id":"tp_dddd","processor_label":"x"}`, ""); st != 403 {
			t.Fatalf("code used by another program: %d", st)
		}
	}
	st, _, b := h.machine("POST", "/api/v1/pair", `{"code":"`+code+`","processor_id":"tp_cccc","processor_label":"a different label"}`, "")
	if st != 200 {
		t.Fatalf("rightful program could not pair: %d %s", st, b)
	}
	if c, ok := h.a.Pair.Lookup("tp_cccc"); !ok || c.Label != "Program C" {
		t.Fatalf("paired label must be the one shown next to the code: %+v", c)
	}
}
