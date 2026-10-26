package app

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Arameair/identity/internal/discovery"
	"github.com/Arameair/identity/internal/features"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/logbook"
	"github.com/Arameair/identity/internal/pairing"
	"github.com/Arameair/identity/internal/profile"
)

//go:embed web/*
var webFS embed.FS

type ctxKey int

const (
	reqIDKey  ctxKey = 1
	clientKey ctxKey = 2
)

func reqID(r *http.Request) string {
	if v, ok := r.Context().Value(reqIDKey).(string); ok {
		return v
	}
	return ""
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (a *App) fail(w http.ResponseWriter, r *http.Request, code int, component, event, msg string) {
	level := "warn"
	if code >= 500 {
		level = "error"
	}
	a.Log.Add(level, component, event, reqID(r), "refused", msg, map[string]string{"http": strconv.Itoa(code), "path": r.URL.Path})
	writeJSON(w, code, map[string]string{"error": msg, "request_id": reqID(r)})
}

type access int

const (
	public access = iota
	bootstrap
	private
	mutate
	machine      // program-to-program: no browser Origin allowed, JSON, no session
	paired       // machine + valid identity/2 signature from a paired Processor
	installation // machine + valid identity/2 signature from a registered feature installation (separate key registry)
	approvalPage // Identity's own approval page: Origin + JSON + approval-scoped cookie + CSRF
)

// Handler returns the full HTTP handler with host, origin, session and CSRF checks.
func (a *App) Handler() http.Handler {
	a.routes = nil
	m := &routeMux{http.NewServeMux(), a}
	static := func(name, ctype string) http.HandlerFunc {
		b, err := webFS.ReadFile("web/" + name)
		return func(w http.ResponseWriter, r *http.Request) {
			if err != nil {
				http.Error(w, "asset missing", 500)
				return
			}
			w.Header().Set("Content-Type", ctype)
			w.Write(b)
		}
	}
	m.HandleFunc("GET /{$}", a.wrap(public, static("index.html", "text/html; charset=utf-8")))
	m.HandleFunc("GET /app.js", a.wrap(public, static("app.js", "text/javascript; charset=utf-8")))
	m.HandleFunc("GET /app.css", a.wrap(public, static("app.css", "text/css; charset=utf-8")))

	// Public and minimal: lets Thought Processor (or anyone on this computer)
	// see what answers here. Self-reported; not authentication.
	m.HandleFunc("GET /api/health", a.wrap(public, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"app": Product, "version": a.Build.Version, "protocol": Protocol, "api": APIVersion,
			"note": "self-reported; not authentication. Thought Processor: pair via POST /api/v1/pair/request"})
	}))

	m.HandleFunc("POST /api/session", a.wrap(bootstrap, a.handleSignIn))
	m.HandleFunc("GET /api/session", a.wrap(private, func(w http.ResponseWriter, r *http.Request) {
		rec, _ := a.Sess.Lookup(r)
		writeJSON(w, 200, map[string]any{"signed_in": true, "csrf": rec.csrf})
	}))
	m.HandleFunc("GET /api/state", a.wrap(private, func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, a.State()) }))
	m.HandleFunc("GET /api/logs", a.wrap(private, func(w http.ResponseWriter, r *http.Request) {
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		writeJSON(w, 200, map[string]any{"entries": a.Log.After(after, 500), "stats": a.Log.Stats()})
	}))
	m.HandleFunc("GET /api/diagnostics", a.wrap(private, func(w http.ResponseWriter, r *http.Request) {
		text := a.Diagnostics(reqID(r))
		a.Log.Info("diagnostics", "diagnostics.generated", reqID(r), "ok", "Copy Diagnostics report built ("+strconv.Itoa(len(text))+" bytes, redacted)", nil)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, text)
	}))

	m.HandleFunc("POST /api/diagnostics/result", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			OK     bool   `json:"ok"`
			Method string `json:"method"`
			Error  string `json:"error"`
		}
		decode(r, &b)
		if b.OK {
			a.Log.Info("diagnostics", "diagnostics.copied", reqID(r), "copied", "report placed on the clipboard via "+clip(b.Method, 30), nil)
		} else {
			a.Log.Warn("diagnostics", "diagnostics.copy_failed", reqID(r), "failed", "browser could not copy ("+clip(b.Error, 120)+"); report shown for manual copy", nil)
		}
		writeJSON(w, 200, map[string]bool{"logged": true})
	}))
	m.HandleFunc("POST /api/github/own-app", a.wrap(mutate, a.handleOwnApp))
	m.HandleFunc("POST /api/github/own-app/remove", a.wrap(mutate, a.handleOwnAppRemove))
	m.HandleFunc("POST /api/setup/complete", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		if err := a.Store.UpdateSettings(func(s *profile.Settings) { s.SetupCompletedAt = &now; s.WizardRequested = false }); err != nil {
			a.fail(w, r, 500, "setup", "setup.save_failed", "could not save setup progress: "+err.Error())
			return
		}
		a.Log.Info("setup", "setup.completed", reqID(r), "monitoring", "setup finished; switching to the monitoring view", a.stateSummary())
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/setup/reopen", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		if err := a.Store.UpdateSettings(func(s *profile.Settings) { s.SetupCompletedAt = nil; s.WizardRequested = true }); err != nil {
			a.fail(w, r, 500, "setup", "setup.save_failed", "could not save: "+err.Error())
			return
		}
		a.Log.Info("setup", "setup.reopened", reqID(r), "setup", "setup flow opened again by the operator", nil)
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/github/start", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		if err := a.GH.Start(); err != nil {
			writeJSON(w, 502, map[string]any{"error": err.Error(), "request_id": reqID(r), "state": a.State()})
			return
		}
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/github/cancel", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		a.GH.Cancel()
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/github/disconnect", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		a.GH.Disconnect()
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/github/recheck", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		if err := a.GH.Recheck(); err != nil {
			a.fail(w, r, 409, "github", "github.recheck_failed", err.Error())
			return
		}
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/processor/address", a.wrap(mutate, a.handleProcessorAddress))
	m.HandleFunc("POST /api/processor/check", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		res := a.Watch.CheckNow(ctx)
		a.Log.Info("processor", "processor.checked", reqID(r), res.State, "manual read-only check of "+orDash(res.Address)+" ("+strconv.FormatInt(res.ElapsedMS, 10)+" ms)", nil)
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/processor/opened", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		cur := a.Watch.Current()
		a.Log.Info("processor", "processor.open_clicked", reqID(r), cur.State, "operator opened Thought Processor's own page at "+orDash(cur.OpenURL)+"; Identity sent no code or credentials — Processor asks for its own operator code", nil)
		writeJSON(w, 200, map[string]bool{"logged": true})
	}))
	m.HandleFunc("POST /api/drafts", a.wrap(mutate, a.handleDraft))
	m.HandleFunc("POST /api/drafts/withdraw", a.wrap(mutate, a.handleWithdraw))
	a.engineRoutes(m)
	a.apiV1Routes(m)
	a.p1Routes(m)
	m.HandleFunc("POST /api/shutdown", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		a.Log.Info("lifecycle", "identity.stop_requested", reqID(r), "stopping", "Stop Identity pressed", nil)
		writeJSON(w, 200, map[string]string{"stopping": "identity only"})
		go func() { time.Sleep(200 * time.Millisecond); a.RequestShutdown() }()
	}))
	return a.guard(m)
}

// guard: request ID, security headers, Host check (DNS-rebinding defence).
func (a *App) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := "req_" + profile.RandHex(6)
		r = r.WithContext(context.WithValue(r.Context(), reqIDKey, id))
		h := w.Header()
		h.Set("X-Request-ID", id)
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		p := strconv.Itoa(a.Port)
		if r.Host != "127.0.0.1:"+p && r.Host != "localhost:"+p && r.Host != "[::1]:"+p {
			a.Log.Warn("web", "web.wrong_host", id, "refused", "request with unexpected Host header refused", map[string]string{"host": clip(r.Host, 60)})
			http.Error(w, "wrong host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) wrap(level access, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		if level == public {
			h(w, r)
			return
		}
		if level == machine || level == paired || level == installation {
			// These routes are for Thought Processor (a program), never for a web page.
			if r.Header.Get("Origin") != "" {
				a.fail(w, r, 403, "pairing", "pairing.browser_refused", "this endpoint is for a paired program, not a web page")
				return
			}
			if level == paired || level == installation {
				// API v2: the pairing secret is never sent. The request is
				// signed (see pairing.RequestMAC); the answer will be too.
				body, err := io.ReadAll(r.Body)
				if err != nil {
					a.fail(w, r, 413, "pairing", "pairing.body_refused", "request body too large")
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				// Processors and feature installations are verified against
				// SEPARATE registries: one can never pass as the other.
				store, idHeader, kind := a.Pair, ProcessorIDHeader, "processor"
				if level == installation {
					store, idHeader, kind = a.InstKeys, InstallationIDHeader, "installation"
				}
				c, rk, err := store.Verify(r.Header.Get(idHeader), r.Method, r.URL.RequestURI(),
					r.Header.Get(TimeHeader), r.Header.Get(NonceHeader), r.Header.Get(AuthHeader), body)
				if err != nil {
					if errors.Is(err, pairing.ErrReplayStore) {
						a.fail(w, r, 503, "pairing", "pairing.replay_storage_unavailable", "replay protection unavailable; protected operation blocked")
						return
					}
					a.fail(w, r, 401, "pairing", "pairing.request_refused", err.Error())
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), clientKey, authed{c, rk, r.Header.Get(NonceHeader), kind}))
			}
			h(w, r)
			return
		}
		if level == approvalPage {
			if r.Method == "POST" && (r.Header.Get("Origin") != "http://"+r.Host || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json")) {
				a.fail(w, r, 403, "approval", "approval.origin_refused", "request did not come from Identity's approval page")
				return
			}
			h(w, r)
			return
		}
		if level == bootstrap || level == mutate {
			if r.Header.Get("Origin") != "http://"+r.Host {
				a.fail(w, r, 403, "web", "web.origin_refused", "request did not come from the Identity window")
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				a.fail(w, r, 415, "web", "web.content_type_refused", "JSON body required")
				return
			}
		}
		if level == bootstrap {
			h(w, r)
			return
		}
		rec, ok := a.Sess.Lookup(r)
		if !ok {
			// No log line for plain unauthenticated GETs (a closed tab polling would flood the log).
			if level == mutate {
				a.fail(w, r, 401, "web", "web.session_refused", "not signed in to this Identity window")
				return
			}
			writeJSON(w, 401, map[string]string{"error": "open Identity with the link from its console window", "request_id": reqID(r)})
			return
		}
		if level == mutate && r.Header.Get("X-Identity-CSRF") != rec.csrf {
			a.fail(w, r, 403, "web", "web.csrf_refused", "missing or wrong anti-forgery token")
			return
		}
		h(w, r)
	}
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func (a *App) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Key string `json:"key"`
	}
	decode(r, &b)
	id, csrf, ok, locked := a.Sess.Exchange(b.Key)
	if locked {
		a.fail(w, r, 429, "web", "session.locked", "too many wrong links; wait a minute")
		return
	}
	if !ok {
		a.fail(w, r, 401, "web", "session.refused", "that open link is not valid for this run of Identity (it changes every start)")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	a.Log.Info("web", "session.opened", reqID(r), "signed_in", "an Identity window was opened with this run's private link", map[string]string{"sessions": strconv.Itoa(a.Sess.Count())})
	writeJSON(w, 200, map[string]string{"csrf": csrf})
}

// handleOwnApp: the person followed Identity's guide, registered their own
// GitHub App and pasted its public Client ID. Saved in the profile; it
// replaces the built-in app for this profile until removed.
func (a *App) handleOwnApp(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ClientID string `json:"client_id"`
	}
	decode(r, &b)
	id := strings.TrimSpace(b.ClientID)
	if !github.ValidClientID(id) {
		a.fail(w, r, 400, "setup", "github.own_app_invalid", "that doesn't look like a GitHub App Client ID — it starts with Iv and is on the app's General page (not the client secret)")
		return
	}
	if os.Getenv("IDENTITY_GITHUB_CLIENT_ID") != "" {
		a.fail(w, r, 409, "setup", "github.own_app_blocked", "a developer override (IDENTITY_GITHUB_CLIENT_ID) is set for this run; remove it to use your own app")
		return
	}
	path := filepath.Join(a.Store.Root, "github-app.json")
	if err := profile.WriteJSON(path, map[string]string{"client_id": id}); err != nil {
		a.fail(w, r, 500, "setup", "github.own_app_not_saved", "could not save: "+err.Error())
		return
	}
	a.GH.UseApp(id, "your own GitHub App (set up in Identity)", "own")
	a.Log.Info("github", "github.own_app_set", reqID(r), "saved", "this profile now signs in with the person's own GitHub App (public Client ID saved in the profile; not a secret)", nil)
	writeJSON(w, 200, a.State())
}

func (a *App) handleOwnAppRemove(w http.ResponseWriter, r *http.Request) {
	if err := os.Remove(filepath.Join(a.Store.Root, "github-app.json")); err != nil && !os.IsNotExist(err) {
		a.fail(w, r, 500, "setup", "github.own_app_not_removed", "could not remove: "+err.Error())
		return
	}
	d := a.opt.DefaultApp
	a.GH.UseApp(d.ID, d.Source, d.Kind)
	what := "no built-in app in this build"
	if d.ID != "" {
		what = d.Source
	}
	a.Log.Info("github", "github.own_app_removed", reqID(r), "removed", "own GitHub App removed from this profile; now using "+what, nil)
	writeJSON(w, 200, a.State())
}

func (a *App) handleProcessorAddress(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Address string `json:"address"`
	}
	decode(r, &b)
	addr, err := discovery.ParseAddress(b.Address, a.Port)
	if err != nil {
		a.fail(w, r, 400, "processor", "processor.address_refused", err.Error())
		return
	}
	if err := a.Store.UpdateSettings(func(s *profile.Settings) { s.ProcessorAddress = addr }); err != nil {
		a.fail(w, r, 500, "processor", "processor.address_not_saved", "could not save: "+err.Error())
		return
	}
	a.Watch.SetAddress(addr)
	a.Log.Info("processor", "processor.address_set", reqID(r), "saved", "Thought Processor address set to "+orDash(addr)+" (read-only public health checks only)", nil)
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	a.Watch.CheckNow(ctx)
	writeJSON(w, 200, a.State())
}

func (a *App) handleDraft(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Feature string `json:"feature"`
	}
	decode(r, &b)
	v := a.GH.View()
	if v.Status != github.Verified {
		a.fail(w, r, 409, "features", "draft.refused", "connect and verify GitHub in this session first; a draft records which verified account it is for")
		return
	}
	d, created, err := a.Feats.Save(b.Feature, v.Account, v.Mode, a.Store.Profile.SystemID)
	if err != nil {
		a.fail(w, r, 409, "features", "draft.refused", err.Error())
		return
	}
	if created {
		a.Log.Info("features", "draft.saved", reqID(r), "NOT SUBMITTED", "access-request draft for "+d.FeatureID+" saved locally for "+d.GitHubLogin+"; nothing was sent to the publisher", map[string]string{"draft_id": d.ID, "github_mode": d.Mode})
	} else {
		a.Log.Info("features", "draft.duplicate", reqID(r), "NOT SUBMITTED", "a draft for "+d.FeatureID+" already exists; no duplicate created", map[string]string{"draft_id": d.ID})
	}
	writeJSON(w, 200, map[string]any{"draft": d, "created": created, "delivery": features.DraftDelivery, "state": a.State()})
}

func (a *App) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Feature string `json:"feature"`
	}
	decode(r, &b)
	v := a.GH.View()
	if v.Status != github.Verified || v.Account == nil {
		a.fail(w, r, 409, "features", "draft.withdraw_refused", "connect GitHub first")
		return
	}
	ok, err := a.Feats.Withdraw(b.Feature, v.Account.ID, v.Mode)
	if err != nil {
		a.fail(w, r, 500, "features", "draft.withdraw_failed", err.Error())
		return
	}
	if ok {
		a.Log.Info("features", "draft.withdrawn", reqID(r), "deleted", "local draft for "+clip(b.Feature, 40)+" deleted", nil)
	}
	writeJSON(w, 200, a.State())
}

// State is the private JSON the UI renders.
func (a *App) State() map[string]any {
	gv := a.GH.View()
	set := a.Store.Settings()
	return map[string]any{
		"app": map[string]any{
			"name": "Identity", "prototype": Prototype, "build": a.Build, "run_id": a.RunID, "pid": os.Getpid(), "port": a.Port,
			"started_at": a.Started.UTC(), "uptime_s": int(time.Since(a.Started).Seconds()),
			"profile_path": profile.DisplayPath(a.Store.Root), "system_id": a.Store.Profile.SystemID,
			"profile_created_at": a.Store.Profile.CreatedAt, "status": "RUNNING",
		},
		"pairing":          a.pairingState(),
		"identities":       a.Ents.List(true),
		"identities_error": a.Ents.LoadError(),
		"managed_by":       a.opt.ManagedBy,
		"engine_access":    a.LastAccess(),
		"publisher":        map[string]string{"name": Publisher, "evidence": "name embedded in this experimental build — not a signed release"},
		"setup": map[string]any{"completed": set.SetupCompletedAt != nil || !set.WizardRequested, "completed_at": set.SetupCompletedAt,
			"note": "the step-by-step wizard is a developer/troubleshooting path; normal first use is in Thought Processor"},
		"github":          gv,
		"last_account":    set.LastAccount,
		"processor":       a.Watch.Current(),
		"features":        a.Feats.Cards(gv),
		"drafts_error":    a.Feats.LoadError(),
		"log":             a.Log.Stats(),
		"not_implemented": NotImplemented,
		"state_legend": map[string]string{
			"available":     features.AvailVisible,
			"unavailable":   features.AvailHidden,
			"not_requested": features.ReqNone,
			"requested":     "REQUESTED (submitted to publisher) — cannot occur yet: no submission service exists",
			"draft":         features.ReqDraft,
			"granted":       "GRANTED — cannot occur yet: no publisher grant service exists",
		},
	}
}

func (a *App) stateSummary() map[string]string {
	gv := a.GH.View()
	login := "-"
	if gv.Account != nil {
		login = gv.Account.Login
	}
	return map[string]string{"github_status": gv.Status, "github_login": login, "github_mode": gv.Mode,
		"processor": a.Watch.Current().State, "drafts": strconv.Itoa(len(a.Feats.All()))}
}

var _ = logbook.Line
