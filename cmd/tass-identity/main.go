// tass-identity: Identity for one TASS installation. It creates this installation's own entity,
// unlocks it with the owner's passphrase, and answers other services' "may I?" questions.
//
//	tass-identity                  serve (default 0.0.0.0:8796 in a container; publish it on 127.0.0.1 only)
//	tass-identity -healthcheck     container healthcheck
//
// Environment:
//
//	IDENTITY_DATA          entity.json, credential, decisions.jsonl          (default ~/.tass-identity)
//	IDENTITY_SERVICE_KEYS  file with "service key" lines (core, engine)      (required to authorize anything)
//	IDENTITY_LISTEN        listen address                                   (default 127.0.0.1:8796)
//	IDENTITY_UNLOCK_HOURS  how long an unlock lasts                         (default 12)
//	IDENTITY_CORE_URL      owner-facing local Core link (loopback only)     (default 127.0.0.1:8795)
//	IDENTITY_GITHUB_CLIENT_ID optional public GitHub App Client ID override
//
// Owner routes (the page, create, unlock, lock) answer only requests addressed to localhost/127.0.0.1,
// so other containers on the internal network can't use them. Service routes need a service key.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"html"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"strings"
	"time"

	"github.com/Arameair/identity/internal/entity"
	githubauth "github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/publisher"
)

//go:embed page.html
var page embed.FS

var version = "dev"

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func main() {
	hc := flag.Bool("healthcheck", false, "container healthcheck")
	flag.Parse()
	listen := env("IDENTITY_LISTEN", "127.0.0.1:8796")
	if *hc {
		addr := strings.Replace(listen, "0.0.0.0", "127.0.0.1", 1)
		r, err := (&http.Client{Timeout: 4 * time.Second}).Get("http://" + addr + "/v1/health")
		if err != nil || r.StatusCode != 200 {
			fmt.Println("not answering:", err)
			os.Exit(1)
		}
		return
	}
	log.SetFlags(log.Ltime)
	home, _ := os.UserHomeDir()
	hours, _ := strconv.ParseFloat(env("IDENTITY_UNLOCK_HOURS", "12"), 64)
	if hours <= 0 {
		hours = 12
	}
	st, err := entity.Open(env("IDENTITY_DATA", filepath.Join(home, ".tass-identity")), time.Duration(hours*float64(time.Hour)))
	if err != nil {
		log.Fatal(err)
	}
	keys, err := loadKeys(os.Getenv("IDENTITY_SERVICE_KEYS"))
	if err != nil {
		log.Fatalf("service keys: %v", err)
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	s := st.State()
	log.Printf("tass-identity %s on %s · %s · %d service key(s)", version, listen, s.State, len(keys))
	clientID := env("IDENTITY_GITHUB_CLIENT_ID", publisher.GitHubAppClientID)
	if clientID != "" && !githubauth.ValidClientID(clientID) {
		log.Fatal("invalid public GitHub App Client ID")
	}
	coreURL, err := validatedCoreURL(env("IDENTITY_CORE_URL", "http://127.0.0.1:8795/"))
	if err != nil {
		log.Fatal(err)
	}
	gh := githubauth.New(githubauth.Config{ClientID: clientID, ClientIDKind: "built-in",
		ClientIDSource: "publisher", WebBase: githubauth.RealWeb, APIBase: githubauth.RealAPI}, nil, nil)
	log.Fatal((&http.Server{Handler: routesWithGitHub(st, keys, randHex(32), gh, coreURL), ReadHeaderTimeout: 10 * time.Second}).Serve(ln))
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// loadKeys reads "service key" lines. Keys shorter than 32 characters are refused.
func loadKeys(path string) (map[string]string, error) {
	out := map[string]string{}
	if path == "" {
		return out, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if len(f) != 2 || len(f[1]) < 32 {
			return nil, fmt.Errorf("each line must be: <service> <key of 32+ characters>")
		}
		out[f[1]] = f[0]
	}
	return out, nil
}

func localHost(r *http.Request) bool {
	h, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		h = r.Host
	}
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// validatedCoreURL forbids arbitrary targets and browser redirects in the owner's Identity page.
func validatedCoreURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") ||
		(u.Path != "" && u.Path != "/") {
		return "", errors.New("IDENTITY_CORE_URL must be a local http://127.0.0.1:PORT/ address")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("IDENTITY_CORE_URL requires a valid explicit local port")
	}
	return "http://127.0.0.1:" + strconv.Itoa(port) + "/", nil
}

func routes(st *entity.Store, keys map[string]string, pageToken string) http.Handler {
	return routesWithGitHub(st, keys, pageToken, githubauth.New(githubauth.Config{
		ClientID: publisher.GitHubAppClientID, WebBase: githubauth.RealWeb, APIBase: githubauth.RealAPI,
	}, nil, nil), "http://127.0.0.1:8795/")
}

// The GitHub session and approval are memory-only and belong to this exact Identity process.
func routesWithGitHub(st *entity.Store, keys map[string]string, pageToken string, gh *githubauth.Client, coreURL string) http.Handler {
	mux := http.NewServeMux()
	var approved struct {
		sync.Mutex
		value bool
		generation uint64 // invalidates in-flight fetch on connect/disconnect/lock
	}
	owner := func(h http.HandlerFunc) http.HandlerFunc { // the owner's page only: localhost + this page's token
		return func(w http.ResponseWriter, r *http.Request) {
			if !localHost(r) {
				writeJSON(w, 403, map[string]string{"error": "owner actions only from this computer's own page"})
				return
			}
			if r.Method == http.MethodPost {
				if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Identity-Page")), []byte(pageToken)) != 1 {
					writeJSON(w, 403, map[string]string{"error": "reload Identity's page"})
					return
				}
				if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
					writeJSON(w, 415, map[string]string{"error": "JSON only"})
					return
				}
			}
			h(w, r)
		}
	}
	service := func(h func(w http.ResponseWriter, r *http.Request, svc string)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			k := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			for key, name := range keys {
				if subtle.ConstantTimeCompare([]byte(k), []byte(key)) == 1 {
					h(w, r, name)
					return
				}
			}
			writeJSON(w, 401, map[string]string{"error": "missing or wrong service key"})
		}
	}
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"service": "tass-identity", "version": version, "state": st.State().State})
	})
	mux.HandleFunc("GET /v1/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, st.State()) })
	mux.HandleFunc("GET /{$}", owner(func(w http.ResponseWriter, r *http.Request) {
		b, _ := page.ReadFile("page.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; frame-ancestors 'none'")
		body := strings.Replace(string(b), "__PAGE_TOKEN__", pageToken, 1)
		body = strings.ReplaceAll(body, "__CORE_URL__", html.EscapeString(coreURL))
		w.Write([]byte(body))
	}))
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad request"})
			return false
		}
		return true
	}
	mux.HandleFunc("POST /v1/entity", owner(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name       string `json:"name"`
			Passphrase string `json:"passphrase"`
		}
		if !decode(w, r, &in) {
			return
		}
		s, err := st.Create(in.Name, in.Passphrase)
		if errors.Is(err, entity.ErrExists) {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		log.Printf("entity created: %s (%s)", s.Entity.EntityID, s.Entity.Name)
		writeJSON(w, 201, s)
	}))
	mux.HandleFunc("POST /v1/unlock", owner(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Passphrase string `json:"passphrase"`
		}
		if !decode(w, r, &in) {
			return
		}
		s, err := st.Unlock(in.Passphrase)
		switch {
		case errors.Is(err, entity.ErrThrottled):
			writeJSON(w, 429, map[string]string{"error": err.Error()})
		case err != nil:
			log.Printf("unlock refused: %v", err)
			writeJSON(w, 403, map[string]string{"error": err.Error()})
		default:
			log.Printf("unlocked %s until %s", s.Entity.EntityID, s.UnlockedUntil.Format(time.RFC3339))
			writeJSON(w, 200, s)
		}
	}))
	mux.HandleFunc("POST /v1/lock", owner(func(w http.ResponseWriter, r *http.Request) {
		st.Lock()
		approved.Lock()
		approved.value = false
		approved.generation++
		approved.Unlock()
		log.Printf("locked by the owner")
		writeJSON(w, 200, st.State())
	}))
	// Owner's GitHub control plane: no GitHub token ever reaches the page or Core.
	mux.HandleFunc("GET /v1/github/status", owner(func(w http.ResponseWriter, r *http.Request) {
		approved.Lock()
		ok := approved.value
		approved.Unlock()
		writeJSON(w, 200, map[string]any{"github": gh.View(), "approved": ok,
			"repository": "Arameair/Thought-Processor", "issue": 51, "skill": "@identity-test"})
	}))
	mux.HandleFunc("POST /v1/github/connect", owner(func(w http.ResponseWriter, r *http.Request) {
		if st.State().State != entity.Unlocked {
			writeJSON(w, 403, map[string]string{"error": "unlock your entity first"})
			return
		}
		approved.Lock()
		approved.value = false
		approved.generation++
		approved.Unlock()
		if err := gh.Start(); err != nil {
			writeJSON(w, 503, map[string]string{"error": "GitHub sign-in did not start; check the status"})
			return
		}
		writeJSON(w, 200, gh.View())
	}))
	mux.HandleFunc("POST /v1/github/disconnect", owner(func(w http.ResponseWriter, r *http.Request) {
		approved.Lock()
		approved.value = false
		approved.generation++
		gh.Disconnect() // memory-only token and pending device authorization cleared
		approved.Unlock()
		writeJSON(w, 200, map[string]string{"status": "signed_out"})
	}))
	mux.HandleFunc("POST /v1/github/approve", owner(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Repository string `json:"repository"`
			Issue int `json:"issue"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Repository != "Arameair/Thought-Processor" || in.Issue != 51 {
			writeJSON(w, 403, map[string]string{"error": "only the #51 @identity-test pilot is allowlisted"})
			return
		}
		if st.State().State != entity.Unlocked || gh.View().Status != githubauth.Verified {
			writeJSON(w, 403, map[string]string{"error": "unlock TASS and connect GitHub first"})
			return
		}
		approved.Lock()
		generation := approved.generation
		approved.Unlock()
		if _, err := gh.ReadIdentityTestLesson(); err != nil {
			writeJSON(w, 502, map[string]string{"error": "the lesson could not be read with this GitHub connection"})
			return
		}
		if st.State().State != entity.Unlocked {
			writeJSON(w, 403, map[string]string{"error": "Identity locked during approval"})
			return
		}
		approved.Lock()
		if approved.generation != generation || gh.View().Status != githubauth.Verified || st.State().State != entity.Unlocked {
			approved.Unlock()
			writeJSON(w, 403, map[string]string{"error": "GitHub session or Identity changed during approval"})
			return
		}
		approved.value = true
		approved.Unlock()
		writeJSON(w, 200, map[string]any{"approved": true, "repository": in.Repository, "issue": in.Issue, "skill": "@identity-test"})
	}))
	// This is a read-only service-key endpoint, not a generic GitHub proxy.
	mux.HandleFunc("GET /v1/github/lesson", service(func(w http.ResponseWriter, r *http.Request, svc string) {
		if svc != "core" || st.State().State != entity.Unlocked {
			writeJSON(w, 403, map[string]string{"error": "Identity authorization required"})
			return
		}
		approved.Lock()
		ok := approved.value
		generation := approved.generation
		approved.Unlock()
		if !ok || gh.View().Status != githubauth.Verified {
			writeJSON(w, 403, map[string]string{"error": "owner has not approved a connected GitHub lesson"})
			return
		}
		body, err := gh.ReadIdentityTestLesson()
		if err != nil {
			writeJSON(w, 502, map[string]string{"error": "approved GitHub lesson unavailable"})
			return
		}
		approved.Lock()
		stillApproved := approved.value && approved.generation == generation && gh.View().Status == githubauth.Verified
		approved.Unlock()
		if st.State().State != entity.Unlocked || !stillApproved {
			writeJSON(w, 403, map[string]string{"error": "Identity or GitHub permission changed during fetch"})
			return
		}
		writeJSON(w, 200, map[string]any{"skill": "@identity-test", "repository": "Arameair/Thought-Processor",
			"issue": 51, "comment_id": 6634742721, "source": "https://github.com/Arameair/Thought-Processor/issues/51#issuecomment-6634742721",
			"mode": "reference_only", "body": body})
	}))
	mux.HandleFunc("POST /v1/authorize", service(func(w http.ResponseWriter, r *http.Request, svc string) {
		var in struct {
			Action   string `json:"action"`
			EntityID string `json:"entity_id"`
			Detail   string `json:"detail"`
		}
		if !decode(w, r, &in) || in.Action == "" {
			if in.Action == "" {
				writeJSON(w, 400, map[string]string{"error": "action is required"})
			}
			return
		}
		d := st.Authorize(svc, in.Action, in.Detail, in.EntityID)
		s := st.State()
		resp := map[string]any{"allow": d.Allow, "decision_id": d.ID, "reason": d.Reason, "entity_id": d.EntityID, "service": svc}
		if s.Entity != nil {
			resp["entity_name"] = s.Entity.Name
		}
		if s.UnlockedUntil != nil && d.Allow {
			resp["valid_until"] = s.UnlockedUntil
		}
		writeJSON(w, 200, resp)
	}))
	return mux
}
