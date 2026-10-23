package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

//go:embed web/index.html
var indexHTML []byte

type server struct{ app *App }

type identityView struct {
	Identity
	TypeLabel  string `json:"type_label"`
	SaveStatus string `json:"save_status"`
}

func view(it Identity) identityView {
	return identityView{Identity: it, TypeLabel: typeLabel(it.EntityType), SaveStatus: "Saved locally — read back from disk"}
}

func views(list []Identity) []identityView {
	out := make([]identityView, 0, len(list))
	for _, it := range list {
		out = append(out, view(it))
	}
	return out
}

func (s *server) routes() *http.ServeMux {
	a := s.app
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", local(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(indexHTML)
	}))
	mux.HandleFunc("GET /api/status", local(s.handleStatus))
	mux.HandleFunc("GET /api/diagnostics", local(s.handleDiagnostics))
	// Identity's own minimal health, so a mis-pointed controller address can be
	// tested (it is correctly reported INCOMPATIBLE: not a Thought Controller).
	mux.HandleFunc("GET /api/health", local(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"app": "identity", "version": appVersion})
	}))

	mux.HandleFunc("POST /api/install", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			AdminName string `json:"admin_name"`
		}
		if !decode(w, r, &in) {
			return
		}
		inst, err := a.CreateInstallation(in.AdminName)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"installation": inst})
	}))
	mux.HandleFunc("POST /api/setup", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Step     int  `json:"step"`
			Complete bool `json:"complete"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.SetSetupStep(in.Step, in.Complete); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "saved"})
	}))
	mux.HandleFunc("POST /api/identities", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			DisplayName        string `json:"display_name"`
			EntityType         string `json:"entity_type"`
			RequestID          string `json:"request_id"`
			AllowDuplicateName bool   `json:"allow_duplicate_name"`
		}
		if !decode(w, r, &in) {
			return
		}
		res, err := a.CreateIdentity(in.DisplayName, in.EntityType, in.RequestID, in.AllowDuplicateName)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"identity": view(res.Identity), "repeat": res.Repeat})
	}))
	mux.HandleFunc("POST /api/identities/starter", write(func(w http.ResponseWriter, r *http.Request) {
		made, err := a.StarterIdentities()
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"created": views(made)})
	}))
	mux.HandleFunc("POST /api/controller/check", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Trigger    string `json:"trigger"`
			AllowStart bool   `json:"allow_start"`
		}
		if !decode(w, r, &in) {
			return
		}
		res, err := a.Check(firstSet(trimTo(in.Trigger, 60), "owner check"), in.AllowStart)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, res)
	}))
	mux.HandleFunc("POST /api/controller/consent", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Decision string `json:"decision"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.Decide(in.Decision); err != nil {
			writeErr(w, err)
			return
		}
		var res *CheckResult
		if in.Decision == "approve" {
			c, err := a.Check("approval just given", true)
			if err == nil {
				res = &c
			}
		}
		writeJSON(w, 200, map[string]any{"status": "saved", "check": res})
	}))
	mux.HandleFunc("POST /api/controller/address", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Address string `json:"address"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.SetAddress(in.Address); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "saved"})
	}))
	mux.HandleFunc("POST /api/controller/scan", write(func(w http.ResponseWriter, r *http.Request) {
		hits := a.scan(a.cfg.Port)
		a.mu.Lock()
		a.p.Events.Add("controller", fmt.Sprintf("searched 127.0.0.1 ports 7778–7899 for Thought Controller hosts (read-only health checks): %d found", len(hits)), "")
		a.mu.Unlock()
		writeJSON(w, 200, map[string]any{"hits": hits})
	}))
	mux.HandleFunc("POST /api/controller/target", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Branch     string `json:"branch"`
			Commit     string `json:"commit"`
			UseRunning bool   `json:"use_running"`
		}
		if !decode(w, r, &in) {
			return
		}
		t, err := a.SetTarget(in.Branch, in.Commit, in.UseRunning)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"target": t})
	}))
	mux.HandleFunc("POST /api/controller/stop", write(func(w http.ResponseWriter, r *http.Request) {
		if err := a.StopController(); err != nil {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "stopped (simulated)"})
	}))
	mux.HandleFunc("POST /api/github/simulate", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			State string `json:"state"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.SetGitHubSimulation(in.State); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "saved"})
	}))
	mux.HandleFunc("POST /api/profiles/new", write(func(w http.ResponseWriter, r *http.Request) {
		name, err := a.NewTestProfile()
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 201, map[string]string{"profile": name})
	}))
	mux.HandleFunc("POST /api/profiles/switch", write(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Profile string `json:"profile"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.SwitchProfile(in.Profile); err != nil {
			writeErr(w, err)
			return
		}
		go a.startupCheck()
		writeJSON(w, 200, map[string]string{"profile": in.Profile})
	}))
	mux.HandleFunc("POST /api/test/fail-next-save", write(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		if a.p.Inst == nil {
			a.failNext = true
		} else {
			a.p.Store.FailNextSave()
		}
		a.p.Events.Add("test-tool", "the next save will fail on purpose", "")
		a.mu.Unlock()
		writeJSON(w, 200, map[string]string{"status": "the next create will fail on purpose"})
	}))
	return mux
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	a := s.app
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.p
	out := map[string]any{
		"config":            a.cfg,
		"pid":               os.Getpid(),
		"started_at":        a.started.UTC(),
		"profile":           p.Name,
		"profile_dir":       p.Dir,
		"profiles":          a.profilesLocked(),
		"installation":      p.Inst,
		"types":             EntityTypes,
		"problems":          p.Store.Problems(),
		"events":            p.Events.Recent(40),
		"events_error":      p.Events.Err(),
		"github":            githubStatus(p.GitHub),
		"github_setting":    p.GitHub.SimulatedState,
		"fail_next_pending": a.failNext || p.Store.FailNextPending(),
	}
	if p.Inst != nil {
		out["administrator"] = views(p.Store.ByRole("administrator"))
		out["system_services"] = views(p.Store.ByRole("system_service"))
		out["managed"] = views(p.Store.ByRole("managed"))
	}
	cf := p.Controller
	attempts := append([]Attempt{}, cf.Attempts...)
	if len(attempts) > 10 {
		attempts = attempts[len(attempts)-10:]
	}
	sugg := suggestedTarget
	out["controller"] = map[string]any{
		"address":          cf.Address,
		"target":           cf.Target,
		"suggested_target": map[bool]any{true: &sugg, false: nil}[shaRe.MatchString(sugg.Commit)],
		"consent":          cf.Consent,
		"consent_valid":    cf.Consent.validFor(cf.Target),
		"needs_reapproval": cf.Consent.Status == "approved" && !cf.Consent.validFor(cf.Target),
		"launcher":         launcherID,
		"attempts":         attempts,
		"counters":         cf.Counters,
		"last_check":       p.LastCheck,
	}
	writeJSON(w, 200, out)
}

// handleDiagnostics: safe to paste anywhere. Display names are left out; no
// credentials exist in this app.
func (s *server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	a := s.app
	a.mu.Lock()
	defer a.mu.Unlock()
	c, p := a.cfg, a.p
	var b strings.Builder
	fmt.Fprintf(&b, "## Identity diagnostics (%s)\n\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Version: %s · Go %s · %s/%s\n", c.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "- Branch: %s (from %s)%s\n- Commit: %s\n", c.Branch, c.BranchFrom, map[bool]string{true: " · detached checkout", false: ""}[c.Detached], c.Commit)
	fmt.Fprintf(&b, "- Data folder: %s (from %s) · port %s · pid %d · up %s\n", c.DataDir, c.DataSource, c.Port, os.Getpid(), time.Since(a.started).Round(time.Second))
	fmt.Fprintf(&b, "- Test profile: %s (%s) · profiles in this data folder: %d\n", p.Name, p.Dir, len(a.profilesLocked()))
	if p.Inst == nil {
		b.WriteString("- Installation: NONE yet (first-run welcome)\n")
	} else {
		i := p.Inst
		fmt.Fprintf(&b, "- installation_id %s (created %s)\n- administrator_id %s\n- controller_service_id %s\n", i.InstallationID, i.CreatedAt.Format(time.RFC3339), i.AdminID, i.ControllerServiceID)
		fmt.Fprintf(&b, "- Setup: step %d · completed %v\n", i.SetupStep, i.SetupCompletedAt != nil)
		m := p.Store.ByRole("managed")
		fmt.Fprintf(&b, "- Managed identities: %d\n", len(m))
		for _, it := range m {
			fmt.Fprintf(&b, "  - %s · %s · created %s\n", it.ID, typeLabel(it.EntityType), it.CreatedAt.Format(time.RFC3339))
		}
	}
	cf := p.Controller
	fmt.Fprintf(&b, "- Controller address: %s · launcher %s (SIMULATED — never launches a process)\n", firstSet(cf.Address, "(none)"), launcherID)
	if cf.Target != nil {
		fmt.Fprintf(&b, "- Pinned target: %s · %s · %s · %s · chosen %s by %s\n", cf.Target.Repo, cf.Target.Branch, cf.Target.Commit, cf.Target.Label, cf.Target.ChosenAt.Format(time.RFC3339), cf.Target.ChosenBy)
	} else {
		b.WriteString("- Pinned target: none\n")
	}
	fmt.Fprintf(&b, "- Startup consent: %s · valid for pinned target: %v", cf.Consent.Status, cf.Consent.validFor(cf.Target))
	if cf.Consent.ConsentID != "" {
		fmt.Fprintf(&b, " · consent %s · fingerprint %s · commit %s", cf.Consent.ConsentID, cf.Consent.TargetFingerprint, cf.Consent.TargetCommit)
	}
	if cf.Consent.DecidedAt != nil {
		fmt.Fprintf(&b, " · decided %s", cf.Consent.DecidedAt.Format(time.RFC3339))
	}
	b.WriteString("\n")
	k := cf.Counters
	fmt.Fprintf(&b, "- Controller counters: checks %d · start requests %d · started %d · reused %d · refused %d · errors %d\n", k.Checks, k.StartRequests, k.Started, k.Reused, k.Refused, k.Errors)
	if lc := p.LastCheck; lc != nil {
		fmt.Fprintf(&b, "- Last check: %s (%s) at %s · %d ms · trigger %q\n", lc.State, lc.Label, lc.CheckedAt.Format(time.RFC3339), lc.ElapsedMS, lc.Trigger)
		if h := lc.Health; h != nil && h.App == "thought-controller" {
			fmt.Fprintf(&b, "  - controller build %s @ %s · protocol %s · simulated %v · service %s %s · bound installation %s · worker %s\n",
				h.Build.Branch, h.Build.Commit, h.Protocol, h.Simulated, h.Service.State, h.Service.ServiceID, firstSet(h.Service.InstallationID, "(none)"), h.Worker.State)
		}
	} else {
		b.WriteString("- Last check: none since start or profile switch\n")
	}
	if n := len(cf.Attempts); n > 0 {
		b.WriteString("- Recent start attempts:\n")
		from := 0
		if n > 8 {
			from = n - 8
		}
		for _, at := range cf.Attempts[from:] {
			fmt.Fprintf(&b, "  - %s %s (%s): %s — %s · %d ms\n", at.At.Format(time.RFC3339), at.RequestID, at.Trigger, at.Result, at.Reason, at.ElapsedMS)
		}
	}
	g := githubStatus(p.GitHub)
	fmt.Fprintf(&b, "- GitHub: %s%s · source access %s · GitHub requests made: 0 · tokens stored: 0\n", g.State, map[bool]string{true: " (MOCK test display)", false: ""}[g.Mock], g.SourceAccess)
	probs := p.Store.Problems()
	fmt.Fprintf(&b, "- Problem files: %d\n", len(probs))
	for _, pr := range probs {
		fmt.Fprintf(&b, "  - %s: %s\n", pr.File, pr.Reason)
	}
	if e := p.Events.Err(); e != "" {
		fmt.Fprintf(&b, "- Event log: %s\n", e)
	}
	b.WriteString("\n### Recent events\n\n")
	for _, e := range p.Events.Recent(20) {
		id := ""
		if e.ID != "" {
			id = " " + e.ID
		}
		fmt.Fprintf(&b, "- %s %s: %s%s\n", e.At.Format("15:04:05"), e.Kind, e.Detail, id)
	}
	b.WriteString("\n_Display names are left out. No credentials exist in this app._\n")
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(b.String()))
}

// ---------- safety (ported from V1 server.go) ----------

func local(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" && host != "[::1]" {
			writeJSON(w, 403, map[string]string{"error": "host not allowed"})
			return
		}
		h(w, r)
	}
}

func write(h http.HandlerFunc) http.HandlerFunc {
	return local(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
			writeJSON(w, 403, map[string]string{"error": "origin not allowed"})
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeJSON(w, 415, map[string]string{"error": "send Content-Type: application/json"})
			return
		}
		h(w, r)
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	dec.DisallowUnknownFields() // e.g. a caller can't choose its own IDs
	if err := dec.Decode(v); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return false
	}
	return true
}

func writeErr(w http.ResponseWriter, err error) {
	code := 500
	switch {
	case errors.Is(err, ErrInvalid):
		code = 400
	case errors.Is(err, ErrNotFound):
		code = 404
	case errors.Is(err, ErrDuplicateName):
		code = 409
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
