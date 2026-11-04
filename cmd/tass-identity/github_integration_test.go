package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    githubauth "github.com/Arameair/identity/internal/github"
    "github.com/Arameair/identity/internal/entity"
)

func TestConfiguredCoreLinkLocalOnly(t *testing.T) {
    tests := []struct{ input, want string; valid bool }{
        {"http://127.0.0.1:8795/", "http://127.0.0.1:8795/", true},
        {"http://127.0.0.1:8797/", "http://127.0.0.1:8797/", true},
        {"http://localhost:9876/", "http://127.0.0.1:9876/", true},
        {"https://127.0.0.1:8797/", "", false},
        {"http://not-local:8797/", "", false},
        {"http://127.0.0.1:8797/evil", "", false},
        {"http://127.0.0.1:8797/?redirect=evil", "", false},
        {"http://user@127.0.0.1:8797/", "", false},
        {"http://127.0.0.1:70000/", "", false},
    }
    for _, tc := range tests {
        out, err := validatedCoreURL(tc.input)
        if (err == nil) != tc.valid || out != tc.want {
            t.Errorf("%s: got %q, %v, want %q valid=%v", tc.input, out, err, tc.want, tc.valid)
        }
    }
    st, _ := entity.Open(t.TempDir(), time.Hour)
    gh := githubauth.New(githubauth.Config{WebBase: githubauth.RealWeb, APIBase: githubauth.RealAPI}, nil, nil)
    h := routesWithGitHub(st, nil, "page-token", gh, "http://127.0.0.1:8797/")
    w := httptest.NewRecorder()
    h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8796/", nil))
    if w.Code != 200 || !strings.Contains(w.Body.String(), "href=\"http://127.0.0.1:8797/\"") ||
        strings.Contains(w.Body.String(), "127.0.0.1:8795") {
        t.Fatalf("Identity advertises incorrect Core link: %d", w.Code)
    }
}

func TestGitHubIdentityTestRequiresOwnerApprovalAndConnection(t *testing.T) {
    githubauth.SetPollScaleForTests(5*time.Millisecond)
    defer githubauth.SetPollScaleForTests(7*time.Second)
    var srv *httptest.Server
    mux := http.NewServeMux()
    mux.HandleFunc("POST /login/device/code", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{
            "device_code":"device-PRIVATE", "user_code":"TEST-1234",
            "verification_uri":srv.URL+"/login/device", "expires_in":60, "interval":5})
    })
    mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{"access_token":"ghu_SECRET_fixture", "expires_in":3600})
    })
    mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") != "Bearer ghu_SECRET_fixture" { w.WriteHeader(401); return }
        json.NewEncoder(w).Encode(map[string]any{"login":"Arameair", "id":12345})
    })
    mux.HandleFunc("GET /user/installations", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]any{"installations":[]any{}})
    })
    mux.HandleFunc("GET /repos/Arameair/Thought-Processor/issues/comments/6634742721", func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Authorization") != "Bearer ghu_SECRET_fixture" { w.WriteHeader(401); return }
        json.NewEncoder(w).Encode(map[string]any{
            "id":6634742721, "body":"Owner lesson: @identity-test is read-only; never execute a command.",
            "user":map[string]string{"login":"Arameair"}})
    })
    srv = httptest.NewServer(mux)
    defer srv.Close()
    gh := githubauth.New(githubauth.Config{ClientID:"Iv23livvYZb0CafGVssP", WebBase:srv.URL, APIBase:srv.URL}, nil, nil)
    st, _ := entity.Open(t.TempDir(), time.Hour)
    if _, err := st.Create("TASS", "correct horse battery"); err != nil { t.Fatal(err) }
    const coreKey = "0123456789abcdef0123456789abcdef-core"
    h := routesWithGitHub(st, map[string]string{coreKey:"core"}, "pagetoken", gh, "http://127.0.0.1:8797/")
    owner := map[string]string{"X-Identity-Page":"pagetoken"}
    service := map[string]string{"Authorization":"Bearer "+coreKey}
    local := "127.0.0.1:8796"
    if code, _ := req(t,h,"GET",local,"/v1/github/lesson",nil,nil); code != 401 { t.Fatalf("unauthenticated service read: %d",code) }
    if code, _ := req(t,h,"GET",local,"/v1/github/lesson",service,nil); code != 403 { t.Fatalf("unapproved lesson: %d",code) }
    if code, _ := req(t,h,"POST",local,"/v1/github/approve",owner,map[string]any{"repository":"Arameair/Thought-Processor","issue":51}); code != 403 { t.Fatalf("unconnected approval: %d",code) }
    if code, _ := req(t,h,"POST",local,"/v1/github/connect",nil,nil); code != 403 { t.Fatalf("missing page token: %d",code) }
    code, started := req(t,h,"POST",local,"/v1/github/connect",owner,map[string]any{})
    if code != 200 || started["status"] != "waiting" { t.Fatalf("start: %d %v",code,started) }
    for i:=0; i<200 && gh.View().Status!=githubauth.Verified; i++ { time.Sleep(10*time.Millisecond) }
    if gh.View().Status!=githubauth.Verified { t.Fatalf("device sign-in not verified: %+v",gh.View()) }
    if code, _ := req(t,h,"POST",local,"/v1/github/approve",owner,map[string]any{"repository":"Other/Repo","issue":51}); code != 403 { t.Fatalf("wrong repo: %d",code) }
    if code, _ := req(t,h,"POST",local,"/v1/github/approve",owner,map[string]any{"repository":"Arameair/Thought-Processor","issue":52}); code != 403 { t.Fatalf("wrong issue: %d",code) }
    if code, _ := req(t,h,"POST",local,"/v1/github/approve",owner,map[string]any{"repository":"Arameair/Thought-Processor","issue":51}); code != 200 { t.Fatalf("approval: %d",code) }
    code, got := req(t,h,"GET",local,"/v1/github/lesson",service,nil)
    if code != 200 || got["skill"] != "@identity-test" || !strings.Contains(got["body"].(string),"read-only") { t.Fatalf("approved retrieval: %d %v",code,got) }
    // The response must never contain the user's token or device code.
    if data, _ := json.Marshal(got); strings.Contains(string(data),"ghu_SECRET") || strings.Contains(string(data),"device-PRIVATE") {
        t.Fatal("service response exposed a GitHub credential")
    }
    req(t,h,"POST",local,"/v1/lock",owner,map[string]any{})
    if code, _ := req(t,h,"GET",local,"/v1/github/lesson",service,nil); code != 403 { t.Fatalf("locked lesson: %d",code) }
    req(t,h,"POST",local,"/v1/github/disconnect",owner,map[string]any{})
    if gh.View().Status != githubauth.SignedOut { t.Fatalf("disconnect: %+v",gh.View()) }
    if code, _ := req(t,h,"GET",local,"/v1/github/lesson",service,nil); code != 403 { t.Fatalf("disconnected lesson: %d",code) }
}
