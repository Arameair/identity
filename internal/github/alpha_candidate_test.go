package github

// Candidate regressions use local HTTP fixtures. They do not prove that a
// real GitHub account or publisher app will authorize successfully.
import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestCandidatePendingHasActionableMessageWithoutAssumingCause(t *testing.T) {
    SetPollScaleForTests(15 * time.Millisecond)
    defer setPollHook(func(d time.Duration) time.Duration { return d })
    mux := http.NewServeMux()
    var server *httptest.Server
    mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "device_code": "pending-device-123", "user_code": "PEND-1234",
            "verification_uri": server.URL + "/login/device", "expires_in": 60, "interval": 5,
        })
    })
    mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
    })
    server = httptest.NewServer(mux)
    defer server.Close()

    c := New(Config{ClientID: "IvFIXTURE0001", WebBase: server.URL, APIBase: server.URL}, nil, nil)
    if err := c.Start(); err != nil { t.Fatal(err) }
    defer c.Cancel()
    deadline := time.Now().Add(2 * time.Second)
    for time.Now().Before(deadline) {
        v := c.View()
        if v.LastCheckedAt != nil {
            if v.WaitingSince == nil { t.Fatal("missing wait start time") }
            if v.Status != Waiting || v.Account != nil {
                t.Fatalf("a pending reply cannot verify an account: %+v", v)
            }
            if !strings.Contains(v.PollNote, "GitHub tab") || !strings.Contains(v.PollNote, "cannot read") {
                t.Fatalf("pending reply missing actionable guidance: %q", v.PollNote)
            }
            return
        }
        time.Sleep(10 * time.Millisecond)
    }
    t.Fatal("fixture never reached pending state")
}

func TestCandidateLateOAuthApprovalCannotResurrectCancelledSignIn(t *testing.T) {
    SetPollScaleForTests(15 * time.Millisecond)
    defer setPollHook(func(d time.Duration) time.Duration { return d })
    entered := make(chan struct{})
    release := make(chan struct{})
    mux := http.NewServeMux()
    var server *httptest.Server
    mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "device_code": "late-device-123", "user_code": "LATE-1234",
            "verification_uri": server.URL + "/login/device", "expires_in": 60, "interval": 5,
        })
    })
    mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "access_token": "ghu_lateFixture1234567890", "expires_in": 28800,
        })
    })
    mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
        close(entered)
        <-release
        json.NewEncoder(w).Encode(map[string]any{"login": "old-account", "id": 123})
    })
    mux.HandleFunc("GET /user/installations", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{"installations": []any{}})
    })
    server = httptest.NewServer(mux)
    defer server.Close()

    c := New(Config{ClientID: "IvFIXTURE0001", WebBase: server.URL, APIBase: server.URL}, nil, nil)
    if err := c.Start(); err != nil { t.Fatal(err) }
    select {
    case <-entered:
    case <-time.After(2 * time.Second):
        close(release)
        t.Fatal("fixture did not begin reading account")
    }
    c.Cancel()
    if v := c.View(); v.Status != SignedOut { t.Fatalf("cancel must sign out: %+v", v) }
    close(release)
    // Give the delayed fixture response time to reach the guarded state write.
    time.Sleep(150 * time.Millisecond)
    if v := c.View(); v.Status != SignedOut || v.Account != nil {
        t.Fatalf("old sign-in resurrected after cancel: %+v", v)
    }
}
