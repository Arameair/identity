package main

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed web/index.html web/app.js web/app.css
var webFS embed.FS

type App struct {
	port   int
	code   string // per-run bootstrap code in the opened link's #fragment
	gh     *GitHub
	ctl    *Controller
	repo   string
	branch string

	mu       sync.Mutex
	sessions map[string]string // session id -> csrf
	find     *FindResult
	finding  bool
	proc     ProcView
	started  time.Time
	done     chan struct{}
}

// ProcView is step 5's evidence: Thought Processor answering on the port
// Project-controller reported.
type ProcView struct {
	Status    string    `json:"status"` // not_started | checking | running | not_answering | wrong_app
	URL       string    `json:"url,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CanOpen   bool      `json:"can_open"`
}

func NewApp(port int, controller string, cfg GitHubConfig) *App {
	return &App{port: port, code: randHex(24), gh: NewGitHub(cfg), ctl: NewController(controller),
		repo: defaultRepo, branch: defaultBranch, sessions: map[string]string{}, started: time.Now().UTC(),
		proc: ProcView{Status: "not_started"}, done: make(chan struct{})}
}

func (a *App) stop() { close(a.done) }

// loop keeps step 4/5 live while Thought Processor is being started.
func (a *App) loop() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-t.C:
		}
		if !a.ctl.Active() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		a.ctl.Refresh(ctx)
		a.checkProcessor(ctx)
		cancel()
	}
}

var procClient = &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func (a *App) checkProcessor(ctx context.Context) {
	lv := a.ctl.View()
	if lv.Phase != "running" || lv.URL == "" {
		a.mu.Lock()
		if lv.Phase == "failed" || lv.Phase == "stopped" || lv.Phase == "gone" {
			a.proc = ProcView{Status: "not_started"}
		} else if a.proc.Status != "not_started" {
			a.proc.Status, a.proc.CanOpen = "checking", false
		}
		a.mu.Unlock()
		return
	}
	v := ProcView{URL: lv.URL, CheckedAt: time.Now().UTC()}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(lv.URL, "/")+"/api/health", nil)
	resp, err := procClient.Do(req)
	switch {
	case err != nil:
		v.Status, v.Detail = "not_answering", "Project Controller says it is running, but it isn't answering yet."
	default:
		var h struct{ App string }
		json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&h)
		resp.Body.Close()
		if resp.StatusCode == 200 && h.App == "thought-processor" {
			v.Status = "running"
			v.CanOpen = a.ctl.SignInURL() != ""
			if !v.CanOpen {
				v.Detail = "It's running, but its sign-in link wasn't found in Project Controller's log."
			}
		} else {
			v.Status, v.Detail = "wrong_app", "Something answered on that port, but it isn't Thought Processor."
		}
	}
	a.mu.Lock()
	a.proc = v
	a.mu.Unlock()
}

// ---------- state ----------

func (a *App) State() map[string]any {
	gh := a.gh.View()
	lv := a.ctl.View()
	a.mu.Lock()
	find, proc, finding := a.find, a.proc, a.finding
	a.mu.Unlock()
	// The current step is derived from evidence, never stored as a claim.
	step := 2
	switch {
	case gh.Status != "connected":
		step = 2
	case find == nil || find.Status != "found" || find.AsLogin != gh.Login:
		step = 3
	case proc.Status == "running":
		step = 5
	default:
		step = 4
	}
	rev, mod := revision()
	return map[string]any{
		"step": step, "github": gh, "find": find, "finding": finding, "launch": lv, "processor": proc,
		"target": map[string]string{"repo": a.repo, "branch": a.branch},
		"about":  map[string]any{"version": appVersion, "commit": rev, "modified": mod, "test_mode": gh.TestMode, "port": a.port},
	}
}

// ---------- HTTP ----------

func secure(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	h.Set("X-Frame-Options", "DENY")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (a *App) hostOK(r *http.Request) bool {
	p := fmt.Sprint(a.port)
	return r.Host == "127.0.0.1:"+p || r.Host == "localhost:"+p
}

func (a *App) session(r *http.Request) (string, bool) {
	c, err := r.Cookie("id_session")
	if err != nil {
		return "", false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	csrf, ok := a.sessions[c.Value]
	return csrf, ok
}

// wrap: kind "page" (public static), "read" (session), "act" (session+origin+csrf+json)
func (a *App) wrap(kind string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secure(w)
		if !a.hostOK(r) {
			http.Error(w, "Identity only answers on its own address", http.StatusMisdirectedRequest)
			return
		}
		if kind == "act" || kind == "signin" {
			if r.Header.Get("Origin") != "http://"+r.Host || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeJSON(w, 403, map[string]string{"error": "This action must come from the Identity window."})
				return
			}
		}
		if kind == "read" || kind == "act" {
			csrf, ok := a.session(r)
			if !ok {
				writeJSON(w, 401, map[string]string{"error": "signed_out"})
				return
			}
			if kind == "act" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Identity-CSRF")), []byte(csrf)) != 1 {
				writeJSON(w, 403, map[string]string{"error": "Please reload the Identity window."})
				return
			}
		}
		h(w, r)
	}
}

func (a *App) Routes() *http.ServeMux {
	m := http.NewServeMux()
	static := func(name, ctype string) http.HandlerFunc {
		b, _ := webFS.ReadFile("web/" + name)
		return func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", ctype); w.Write(b) }
	}
	m.HandleFunc("GET /{$}", a.wrap("page", static("index.html", "text/html; charset=utf-8")))
	m.HandleFunc("GET /app.js", a.wrap("page", static("app.js", "text/javascript; charset=utf-8")))
	m.HandleFunc("GET /app.css", a.wrap("page", static("app.css", "text/css; charset=utf-8")))

	m.HandleFunc("POST /api/session", a.wrap("signin", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ K string }
		json.NewDecoder(io.LimitReader(r.Body, 512)).Decode(&in)
		if subtle.ConstantTimeCompare([]byte(in.K), []byte(a.code)) != 1 {
			writeJSON(w, 401, map[string]string{"error": "This link is from an earlier start of Identity."})
			return
		}
		id, csrf := randHex(32), randHex(32)
		a.mu.Lock()
		a.sessions[id] = csrf
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "id_session", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		writeJSON(w, 200, map[string]string{"csrf": csrf})
	}))
	m.HandleFunc("GET /api/session", a.wrap("read", func(w http.ResponseWriter, r *http.Request) {
		csrf, _ := a.session(r)
		writeJSON(w, 200, map[string]string{"csrf": csrf})
	}))
	m.HandleFunc("GET /api/state", a.wrap("read", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, a.State()) }))

	m.HandleFunc("POST /api/github/start", a.wrap("act", func(w http.ResponseWriter, r *http.Request) {
		a.gh.Start() // outcome is reported through state
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/github/cancel", a.wrap("act", func(w http.ResponseWriter, r *http.Request) {
		a.gh.Cancel()
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/github/disconnect", a.wrap("act", func(w http.ResponseWriter, r *http.Request) {
		a.gh.Disconnect()
		a.mu.Lock()
		a.find = nil
		a.mu.Unlock()
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/find", a.wrap("act", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		if a.finding {
			a.mu.Unlock()
			writeJSON(w, 200, a.State())
			return
		}
		a.finding = true
		a.mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		res := a.gh.Find(ctx, a.repo, a.branch)
		cancel()
		a.mu.Lock()
		a.find, a.finding = &res, false
		a.mu.Unlock()
		writeJSON(w, 200, a.State())
	}))
	launch := func(retry bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			a.mu.Lock()
			find := a.find
			a.mu.Unlock()
			if find == nil || find.Status != "found" {
				writeJSON(w, 409, map[string]string{"error": "Find Thought Processor first."})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			a.ctl.Request(ctx, a.repo, a.branch, find.Commit, retry)
			cancel()
			writeJSON(w, 200, a.State())
		}
	}
	m.HandleFunc("POST /api/start", a.wrap("act", launch(false)))
	m.HandleFunc("POST /api/retry", a.wrap("act", launch(true)))
	// The sign-in link is handed over only on the person's click, never in state.
	m.HandleFunc("POST /api/open", a.wrap("act", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		ok := a.proc.Status == "running" && a.proc.CanOpen
		a.mu.Unlock()
		link := a.ctl.SignInURL()
		if !ok || link == "" {
			writeJSON(w, 409, map[string]string{"error": "Thought Processor isn't ready to open yet."})
			return
		}
		writeJSON(w, 200, map[string]string{"url": link})
	}))
	return m
}
