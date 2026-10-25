package github

// All tests here use a LOCAL TEST FIXTURE standing in for github.com. They
// prove Identity's handling of GitHub's documented responses; they do NOT
// prove a real sign-in with the publisher's registered app.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fixture struct {
	srv     *httptest.Server
	polls   atomic.Int32
	outcome string // "approve" | "deny"
}

func newFixture(t *testing.T, outcome string) *fixture {
	f := &fixture{outcome: outcome}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("client_id") != "IvFIXTURE0001" {
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized_client"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"device_code": "fixture-device-code-123", "user_code": "FIXT-0001",
			"verification_uri": f.srv.URL + "/login/device", "expires_in": 60, "interval": 5})
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		n := f.polls.Add(1)
		switch {
		case n < 2:
			json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
		case f.outcome == "deny":
			json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"access_token": "ghu_FIXTUREtokenFIXTUREtoken0001", "expires_in": 28800})
		}
	})
	auth := func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer ghu_FIXTUREtokenFIXTUREtoken0001"
	}
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"login": "fixture-owner", "id": 4242, "name": "Fixture Owner", "type": "User", "html_url": "https://github.com/fixture-owner"})
	})
	mux.HandleFunc("GET /user/installations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"installations": []map[string]any{{"id": 77, "app_id": 9, "app_slug": "identity-fixture-app",
			"repository_selection": "selected", "account": map[string]string{"login": "Arameair"}}}})
	})
	mux.HandleFunc("GET /repos/Arameair/Thought-Processor", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"full_name": "Arameair/Thought-Processor", "private": true})
	})
	mux.HandleFunc("GET /repos/Arameair/Thought-engine", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func waitStatus(t *testing.T, c *Client, want string) View {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if v := c.View(); v.Status == want {
			return v
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("status never became %s: %+v", want, c.View())
	return View{}
}

func TestFixtureApproveFlow(t *testing.T) {
	pollInterval = func(time.Duration) time.Duration { return 20 * time.Millisecond }
	f := newFixture(t, "approve")
	var events []string
	var evMu sync.Mutex
	c := New(Config{ClientID: "IvFIXTURE0001", WebBase: f.srv.URL, APIBase: f.srv.URL}, []string{"Arameair/Thought-Processor", "Arameair/Thought-engine"},
		func(l, e, r, d string) { evMu.Lock(); events = append(events, e+" "+d); evMu.Unlock() })
	if c.View().Mode != "TEST FIXTURE" {
		t.Fatal("fixture not labelled")
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	if v := c.View(); v.Status != Waiting || v.UserCode != "FIXT-0001" {
		t.Fatalf("waiting view %+v", v)
	}
	v := waitStatus(t, c, Verified)
	if v.Account.Login != "fixture-owner" || v.Account.ID != 4242 {
		t.Fatalf("account %+v", v.Account)
	}
	if v.Publisher.InstallationsResult != "READ" || v.Publisher.Installations[0].Account != "Arameair" {
		t.Fatalf("publisher %+v", v.Publisher)
	}
	if r, _ := v.RepoVisible("Arameair/Thought-Processor"); r.Result != "VISIBLE" {
		t.Fatalf("processor repo %+v", r)
	}
	if r, _ := v.RepoVisible("Arameair/Thought-engine"); r.Result != "NOT VISIBLE" {
		t.Fatalf("engine repo %+v", r)
	}
	if v.UserCode != "" || v.TokenExpiresAt == nil {
		t.Fatalf("verified view leaked code or missed expiry: %+v", v)
	}
	evMu.Lock()
	seen := append([]string(nil), events...)
	evMu.Unlock()
	for _, e := range seen {
		if strings.Contains(e, "FIXT-0001") || strings.Contains(e, "ghu_") || strings.Contains(e, "fixture-device-code") {
			t.Fatalf("secret in event: %s", e)
		}
	}
	c.Disconnect()
	if v := c.View(); v.Status != SignedOut || v.Account != nil {
		t.Fatalf("disconnect %+v", v)
	}
}

func TestFixtureDeniedIsHonest(t *testing.T) {
	pollInterval = func(time.Duration) time.Duration { return 20 * time.Millisecond }
	f := newFixture(t, "deny")
	c := New(Config{ClientID: "IvFIXTURE0001", WebBase: f.srv.URL, APIBase: f.srv.URL}, nil, nil)
	c.Start()
	v := waitStatus(t, c, Denied)
	if v.Account != nil || v.Publisher != nil {
		t.Fatal("denied flow produced an account")
	}
}

func TestUnknownClientIDRefused(t *testing.T) {
	f := newFixture(t, "approve")
	c := New(Config{ClientID: "IvWRONG00001", WebBase: f.srv.URL, APIBase: f.srv.URL}, nil, nil)
	if err := c.Start(); err == nil {
		t.Fatal("unknown client id accepted")
	}
	if v := c.View(); v.Status != Failed || !strings.Contains(v.Message, "Client ID") {
		t.Fatalf("view %+v", v)
	}
}

func TestNoClientIDIsNotConfigured(t *testing.T) {
	c := New(Config{WebBase: RealWeb, APIBase: RealAPI}, nil, nil)
	if v := c.View(); v.Status != NotConfigured || v.Mode != "real" {
		t.Fatalf("view %+v", v)
	}
	if err := c.Start(); err == nil {
		t.Fatal("start without client id succeeded")
	}
	if ValidClientID("abc") || !ValidClientID("Iv23liABCDEFGH") {
		t.Fatal("client id validation")
	}
}
