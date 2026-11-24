package main

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Arameair/identity/internal/entity"
)

// Owner approval interface for github.mailbox.share (fixtures only).

func TestShareRequestRefusesTextTheOwnerCannotSee(t *testing.T) {
	f := ghSetup(t)
	ent := f.st.State().Entity.EntityID
	for i, text := range []string{"right‮to-left", "zero​width", "bell\x07", "carriage\rreturn", "soft­hyphen",
		"line separator"} {
		id := "share_20261124T1200" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "_0123abcd"
		if c, _ := f.raw("POST", "/v1/github/mailbox/share-requests", coreKey, map[string]any{"share_id": id, "repo": "Arameair/TASS",
			"issue": 24, "text": text, "entity_id": ent}); c != 403 {
			t.Fatalf("text %q must be refused (invisible or ambiguous characters), got %d", text, c)
		}
	}
	if c, _ := f.raw("POST", "/v1/github/mailbox/share-requests", coreKey, map[string]any{"share_id": "share_20261124T120099_0123abcd",
		"repo": "Arameair/TASS", "issue": 24, "text": "Zeile 1\n\tZeile 2 — café", "entity_id": ent}); c != 200 {
		t.Fatalf("visible text with newline, tab and accents must be accepted: %d", c)
	}
}

func TestShareApprovalRoutesAreOwnerPageOnly(t *testing.T) {
	f := ghSetup(t)
	ent := f.st.State().Entity.EntityID
	const sid = "share_20261124T120002_0123abcd"
	f.raw("POST", "/v1/github/mailbox/share-requests", coreKey, map[string]any{"share_id": sid, "repo": "Arameair/TASS", "issue": 24, "text": "owner only", "entity_id": ent})
	h := routes(f.st, map[string]string{coreKey: "core"}, "page-secret")
	// From the container network (not localhost): no list, no approval.
	if c, _ := req(t, h, "GET", "identity:8796", "/v1/github/mailbox/share-requests", nil, nil); c != 403 {
		t.Fatalf("list from a non-local host: %d", c)
	}
	// Localhost without the page token (another local program): no list of pending texts.
	if c, out := req(t, h, "GET", "127.0.0.1:8796", "/v1/github/mailbox/share-requests", nil, nil); c != 403 || out["requests"] != nil {
		t.Fatalf("pending texts readable without the page token: %d %v", c, out)
	}
	if c, out := req(t, h, "GET", "127.0.0.1:8796", "/v1/github/mailbox/share-requests", map[string]string{"X-Identity-Page": "wrong"}, nil); c != 403 || out["requests"] != nil {
		t.Fatalf("pending texts readable with a wrong token: %d", c)
	}
	if c, out := req(t, h, "GET", "127.0.0.1:8796", "/v1/github/mailbox/share-requests", map[string]string{"X-Identity-Page": "page-secret"}, nil); c != 200 || len(out["requests"].([]any)) != 1 {
		t.Fatalf("owner page must read it: %d", c)
	}
	// Localhost without the page token (another local program): no approval.
	if c, _ := req(t, h, "POST", "127.0.0.1:8796", "/v1/github/mailbox/share-approve", nil,
		map[string]any{"share_id": sid, "text_sha256": hexOf("owner only"), "passphrase": pass, "approve": true}); c != 403 {
		t.Fatalf("approval without the page token: %d", c)
	}
	if r, _ := f.st.ShareRequestFor("core", sid, ent); r.State != "PENDING" {
		t.Fatalf("request changed: %+v", r)
	}
	// Rejecting needs the page but not the passphrase, and leaves nothing to redeem.
	if c, out := f.owner("POST", "/v1/github/mailbox/share-approve", map[string]any{"share_id": sid, "text_sha256": hexOf("owner only"), "approve": false}); c != 200 || out["state"] != "REJECTED" || out["decision_id"] != nil {
		t.Fatalf("reject: %d %v", c, out)
	}
}

func TestPageShowsShareExactlyAndHashesTheDisplayedText(t *testing.T) {
	st, _ := entity.Open(t.TempDir(), time.Hour)
	h := routes(st, nil, "page-secret")
	r := httptest.NewRequest("GET", "http://127.0.0.1:8796/", nil)
	r.Host = "127.0.0.1:8796"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	page := w.Body.String()
	for _, want := range []string{`id="shareApprovals"`, `"/v1/github/mailbox/share-requests"`, `"/v1/github/mailbox/share-approve"`,
		`"Destination: " + q.repo + " issue #" + q.issue`, `crypto.subtle.digest("SHA-256"`, `sha256Hex(pre.textContent)`,
		`passphrase: approve ? pass.value : ""`} {
		if !strings.Contains(page, want) {
			t.Fatalf("owner page is missing %s", want)
		}
	}
	// The share code never builds HTML from request values.
	block := page[strings.Index(page, "async function refreshShares"):strings.Index(page, "async function decideShare")]
	block += page[strings.Index(page, "async function decideShare"):]
	block = block[:strings.Index(block, "\n}\n")+2]
	if regexp.MustCompile(`innerHTML|insertAdjacentHTML|outerHTML|document\.write`).MatchString(block) {
		t.Fatal("share rendering must use textContent only")
	}
	if strings.Contains(block, "q.text_sha256, passphrase") || strings.Contains(block, "text_sha256: q.text_sha256") {
		t.Fatal("approval must hash the displayed text, not echo the server's hash")
	}
}
