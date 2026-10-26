package app

// API v1 — what Thought Processor (the normal human interface, owner decision
// C01 2026-10-26) uses to work with this independently running Identity.
//
// Every route here is program-to-program: a request carrying a browser
// Origin is refused. Apart from /api/v1/pair/request and /api/v1/pair, every
// route needs the pairing bearer token, and every answer is signed:
//
//	X-Identity-Proof: hex(HMAC-SHA256(token, "identity-v1\n" + nonce + "\n" + body))
//
// where body is the exact response bytes and nonce is the caller's optional
// X-Processor-Nonce header (send a fresh one to rule out replayed answers).
// Processor checks the proof and that X-Identity-System-ID equals the ID it
// saved when pairing; a different value means a different Identity
// installation answered (wrong service).
//
// What this API decides and what it doesn't (be honest in the UI):
//   - Pairing proves Processor talks to the Identity the operator approved.
//     It is NOT a grant to any feature or data (C05).
//   - Identities start with no access (C03). Grants are not implemented in
//     this alpha, so /authorize always answers DENIED, never cached (C06).
//   - Install eligibility for Thought Engine (rule repo-visibility-v0) is
//     only permission to download/install — not to run it or read data (C02).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/entities"
	"github.com/Arameair/identity/internal/features"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/pairing"
	"github.com/Arameair/identity/internal/profile"
)

const (
	APIVersion       = "identity/1-alpha"
	pairRequestGap   = 3 * time.Second
	ProofHeader      = "X-Identity-Proof"
	SystemIDHeader   = "X-Identity-System-ID"
	NonceHeader      = "X-Processor-Nonce"
	ShutdownConfirm  = "shut-down-identity"
	DecisionDenied   = "DENIED"
	notTrackedByIdty = "UNKNOWN_TO_IDENTITY"
)

// ProofCanonical is what the HMAC in X-Identity-Proof covers.
func ProofCanonical(nonce string, body []byte) string {
	return "identity-v1\n" + nonce + "\n" + string(body)
}

type pairThrottle struct {
	mu   sync.Mutex
	last time.Time
}

// signed writes v as JSON and signs the exact bytes for the paired caller.
func (a *App) signed(w http.ResponseWriter, r *http.Request, code int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		a.fail(w, r, 500, "api", "api.encode_failed", err.Error())
		return
	}
	nonce := clip(r.Header.Get(NonceHeader), 128)
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set(SystemIDHeader, a.Store.Profile.SystemID)
	h.Set(ProofHeader, pairing.Proof(bearer(r), ProofCanonical(nonce, b)))
	w.WriteHeader(code)
	w.Write(b)
}

// signedFail is a refusal on a paired route, still signed so Processor can
// tell "Identity said no" apart from "something else answered".
func (a *App) signedFail(w http.ResponseWriter, r *http.Request, code int, event, msg string) {
	a.Log.Add("warn", "api", event, reqID(r), "refused", msg, map[string]string{"http": strconv.Itoa(code), "path": r.URL.Path, "processor": clientFrom(r).ProcessorID})
	a.signed(w, r, code, map[string]string{"error": msg, "request_id": reqID(r)})
}

func (a *App) apiV1Routes(m *http.ServeMux) {
	// Step 1 of pairing, started from Thought Processor: Identity shows a
	// one-time code in ITS OWN console window (and troubleshooting page).
	// The person types it into Processor. Nothing secret is returned here.
	m.HandleFunc("POST /api/v1/pair/request", a.wrap(machine, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			ProcessorID string `json:"processor_id"`
			Label       string `json:"processor_label"`
		}
		if decode(r, &b) != nil || !pairing.ValidProcessorID(b.ProcessorID) {
			a.fail(w, r, 400, "pairing", "pairing.request_bad", "expected {processor_id (4–64 letters, digits, . _ -), processor_label}")
			return
		}
		a.pairGap.mu.Lock()
		wait := pairRequestGap - time.Since(a.pairGap.last)
		if wait > 0 {
			a.pairGap.mu.Unlock()
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			a.fail(w, r, 429, "pairing", "pairing.request_too_fast", "a pairing code was just shown; wait a few seconds")
			return
		}
		a.pairGap.last = time.Now()
		a.pairGap.mu.Unlock()
		code, exp, err := a.Pair.NewCode()
		if err != nil {
			a.fail(w, r, 500, "pairing", "pairing.code_failed", err.Error())
			return
		}
		label := clip(strings.TrimSpace(b.Label), 80)
		if label == "" {
			label = "Thought Processor"
		}
		a.announce(code, exp, b.ProcessorID, label)
		a.Log.Info("pairing", "pairing.requested", reqID(r), "code_shown", "a program asked to pair; a one-time code is shown in Identity's window only (valid 5 minutes, never logged)",
			map[string]string{"processor_id": clip(b.ProcessorID, 64), "label": label})
		writeJSON(w, 202, map[string]any{
			"status":     "CODE_SHOWN_IN_IDENTITY",
			"message":    "Identity is showing a one-time pairing code in its own window. Type that code into Thought Processor.",
			"expires_at": exp.UTC(),
			"next":       "POST /api/v1/pair {code, processor_id, processor_label}",
		})
	}))

	m.HandleFunc("POST /api/v1/unpair", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		c := clientFrom(r)
		resp := map[string]any{"unpaired": true, "processor_id": c.ProcessorID,
			"note": "this token no longer works; pairing again needs a new code from Identity's window"}
		a.signed(w, r, 200, resp) // sign with the token while it is still valid
		if _, err := a.Pair.Unpair(c.ProcessorID); err != nil {
			a.Log.Error("pairing", "pairing.unpair_failed", reqID(r), "error", err.Error(), nil)
			return
		}
		a.Log.Info("pairing", "pairing.unpaired", reqID(r), "removed", "Thought Processor "+c.ProcessorID+" unpaired itself; its token no longer works", nil)
	}))

	m.HandleFunc("GET /api/v1/status", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		a.signed(w, r, 200, a.statusV1(r))
	}))

	// ---- identities ----
	m.HandleFunc("GET /api/v1/identities", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		if e := a.Ents.LoadError(); e != "" {
			a.signedFail(w, r, 503, "identities.unavailable", e)
			return
		}
		list := a.Ents.List(r.URL.Query().Get("archived") == "1")
		a.signed(w, r, 200, map[string]any{"identities": list, "count": len(list), "kinds": entities.Kinds,
			"note": "every identity starts with no feature or source access; grants are not implemented in this alpha"})
	}))
	m.HandleFunc("POST /api/v1/identities", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Name, Kind string }
		if decode(r, &b) != nil {
			a.signedFail(w, r, 400, "identities.bad_request", "expected {name, kind}")
			return
		}
		e, err := a.Ents.Create(b.Name, b.Kind, "processor:"+clientFrom(r).ProcessorID)
		if err != nil {
			a.signedFail(w, r, entityErrCode(err), "identities.create_refused", err.Error())
			return
		}
		a.Log.Info("identities", "identity.created", reqID(r), "created", "identity "+e.ID+" ("+e.Kind+") created with no access", map[string]string{"processor": clientFrom(r).ProcessorID})
		a.signed(w, r, 201, e)
	}))
	m.HandleFunc("GET /api/v1/identities/{id}", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		e, ok := a.Ents.Get(r.PathValue("id"))
		if !ok {
			a.signedFail(w, r, 404, "identities.not_found", entities.ErrNotFound.Error())
			return
		}
		a.signed(w, r, 200, e)
	}))
	ent := func(event string, fn func(id string, r *http.Request) (entities.Entity, error)) http.HandlerFunc {
		return a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
			e, err := fn(r.PathValue("id"), r)
			if err != nil {
				a.signedFail(w, r, entityErrCode(err), "identities."+event+"_refused", err.Error())
				return
			}
			a.Log.Info("identities", "identity."+event, reqID(r), "ok", "identity "+e.ID+" "+event, map[string]string{"processor": clientFrom(r).ProcessorID})
			a.signed(w, r, 200, e)
		})
	}
	m.HandleFunc("POST /api/v1/identities/{id}/rename", ent("renamed", func(id string, r *http.Request) (entities.Entity, error) {
		var b struct{ Name string }
		decode(r, &b)
		return a.Ents.Rename(id, b.Name)
	}))
	m.HandleFunc("POST /api/v1/identities/{id}/archive", ent("archived", func(id string, r *http.Request) (entities.Entity, error) { return a.Ents.Archive(id) }))
	m.HandleFunc("POST /api/v1/identities/{id}/restore", ent("restored", func(id string, r *http.Request) (entities.Entity, error) { return a.Ents.Restore(id) }))

	// ---- features ----
	m.HandleFunc("GET /api/v1/features", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		a.signed(w, r, 200, map[string]any{"features": a.featureStates(r.Context(), nil, clientFrom(r)), "legend": featureLegend, "catalog_note": a.catalogNote()})
	}))
	m.HandleFunc("GET /api/v1/identities/{id}/features", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		e, ok := a.Ents.Get(r.PathValue("id"))
		if !ok {
			a.signedFail(w, r, 404, "identities.not_found", entities.ErrNotFound.Error())
			return
		}
		a.signed(w, r, 200, map[string]any{"identity": e, "features": a.featureStates(r.Context(), &e, clientFrom(r)), "legend": featureLegend, "catalog_note": a.catalogNote()})
	}))

	// Generic per-feature install routes. Processor should use these rather
	// than the older Engine-specific paths, so a second feature needs no
	// Processor change. Thought Engine (rule repo-visibility-v0) is the only
	// install adapter so far; other features answer NO_INSTALL_ADAPTER.
	m.HandleFunc("GET /api/v1/features/{id}/eligibility", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := a.findVisible(r.Context(), id); !ok {
			a.signedFail(w, r, 404, "features.unknown", "Identity doesn't know a feature called "+clip(id, 40))
			return
		}
		for _, f := range a.featureStates(r.Context(), nil, clientFrom(r)) {
			if f["feature"] == id {
				a.signed(w, r, 200, map[string]any{"feature": id, "eligible_to_install": f["eligible_to_install"], "install_adapter": installAdapter(id)})
				return
			}
		}
	}))
	// The feature contract (thought.feature/1): everything Processor needs to
	// install the feature without feature-specific code.
	m.HandleFunc("GET /api/v1/features/{id}/contract", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		f, ok := a.findVisible(r.Context(), r.PathValue("id"))
		if !ok {
			a.signedFail(w, r, 404, "features.unknown", "Identity doesn't know a feature called "+clip(r.PathValue("id"), 40))
			return
		}
		a.signed(w, r, 200, features.ContractFor(f))
	}))
	m.HandleFunc("GET /api/v1/features/{id}/release", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		if a.needAdapter(w, r) {
			a.engineRelease(w, r)
		}
	}))
	m.HandleFunc("GET /api/v1/features/{id}/release/download", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		if a.needAdapter(w, r) {
			a.engineDownload(w, r)
		}
	}))

	// ---- authorization (default deny, never cached) ----
	m.HandleFunc("POST /api/v1/authorize", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			IdentityID string `json:"identity_id"`
			Feature    string `json:"feature"`
			Operation  string `json:"operation"`
			Source     string `json:"source"`
		}
		if decode(r, &b) != nil || b.Feature == "" || b.Operation == "" {
			a.signedFail(w, r, 400, "authorize.bad_request", "expected {identity_id, feature, operation, source?}")
			return
		}
		a.signed(w, r, 200, a.authorize(r, b.IdentityID, b.Feature, b.Operation, b.Source))
	}))

	// ---- GitHub sign-in, driven from Processor ----
	m.HandleFunc("GET /api/v1/github", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		a.signed(w, r, 200, a.githubV1())
	}))
	m.HandleFunc("POST /api/v1/github/connect", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		if v := a.GH.View(); v.Status == github.Verified {
			a.signed(w, r, 200, a.githubV1())
			return
		}
		if err := a.GH.Start(); err != nil {
			a.signedFail(w, r, 502, "github.connect_failed", err.Error())
			return
		}
		a.Log.Info("github", "github.connect_from_processor", reqID(r), "waiting", "Thought Processor "+clientFrom(r).ProcessorID+" started GitHub sign-in; the person approves it on github.com", nil)
		a.signed(w, r, 200, a.githubV1())
	}))
	m.HandleFunc("POST /api/v1/github/cancel", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		a.GH.Cancel()
		a.signed(w, r, 200, a.githubV1())
	}))
	m.HandleFunc("POST /api/v1/github/disconnect", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		a.GH.Disconnect()
		a.signed(w, r, 200, a.githubV1())
	}))

	// ---- lifecycle (C04): Processor's explicit-exit "Shut down" choice ----
	m.HandleFunc("POST /api/v1/shutdown", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Confirm string `json:"confirm"`
			Reason  string `json:"reason"`
		}
		decode(r, &b)
		if b.Confirm != ShutdownConfirm {
			a.signedFail(w, r, 400, "lifecycle.shutdown_unconfirmed", `send {"confirm":"`+ShutdownConfirm+`"} only after the person chose to shut down`)
			return
		}
		c := clientFrom(r)
		reason := "Thought Processor " + c.ProcessorID + " asked Identity to stop (" + orDash(clip(b.Reason, 120)) + ")"
		a.Log.Info("lifecycle", "identity.stop_requested", reqID(r), "stopping", reason, map[string]string{"managed_by": orDash(a.opt.ManagedBy)})
		a.signed(w, r, 202, map[string]any{"stopping": true, "pid": os.Getpid(), "run_id": a.RunID,
			"verify": "the process with this pid exits within a few seconds and runtime/service.json is removed"})
		go func() { time.Sleep(200 * time.Millisecond); a.requestShutdownBecause(reason) }()
	}))
}

func entityErrCode(err error) int {
	switch {
	case errors.Is(err, entities.ErrNotFound):
		return 404
	case errors.Is(err, entities.ErrDupName), errors.Is(err, entities.ErrArchived):
		return 409
	case errors.Is(err, entities.ErrName), errors.Is(err, entities.ErrKind):
		return 400
	}
	return 503
}

// announce prints the pairing code where only someone at this computer,
// looking at Identity's own window, can see it.
func (a *App) announce(code string, exp time.Time, processorID, label string) {
	if a.opt.Announce == nil {
		return
	}
	a.opt.Announce([]string{
		"",
		"  +-- PAIRING REQUEST from " + label + " (" + processorID + ")",
		"  |   If you just pressed Connect in Thought Processor, type this code there:",
		"  |",
		"  |          " + code,
		"  |",
		"  |   Valid until " + exp.Local().Format("15:04:05") + ". If you didn't ask for this, ignore it.",
		"  +--",
		"",
	})
}

func (a *App) statusV1(r *http.Request) map[string]any {
	c := clientFrom(r)
	active, archived := a.Ents.Counts()
	lifecycle := map[string]any{"pid": os.Getpid(), "run_id": a.RunID, "port": a.Port,
		"started_at": a.Started.UTC(), "uptime_s": int(time.Since(a.Started).Seconds()),
		"stop": "POST /api/v1/shutdown (only after the person chooses Shut down)"}
	if a.opt.ManagedBy != "" {
		lifecycle["started_by"] = a.opt.ManagedBy
		lifecycle["ownership"] = "MANAGED"
	} else {
		lifecycle["started_by"] = "external (a person or another tool started it)"
		lifecycle["ownership"] = "EXTERNAL"
	}
	var last any
	if d := a.LastAccess(); d != nil {
		last = map[string]any{"state": d.State, "rule": d.Rule, "checked_at": d.CheckedAt, "decision_id": d.DecisionID}
	}
	return map[string]any{
		"app": Product, "api": APIVersion, "protocol": Protocol,
		"state":              "READY",
		"identity_system_id": a.Store.Profile.SystemID,
		"installation": map[string]any{"system_id": a.Store.Profile.SystemID, "created_at": a.Store.Profile.CreatedAt,
			"profile": profile.DisplayPath(a.Store.Root)},
		"build":     a.Build,
		"lifecycle": lifecycle,
		"you": map[string]any{"processor_id": c.ProcessorID, "label": c.Label, "paired_at": c.PairedAt,
			"meaning": "you are a paired program; pairing proves who you talk to and grants no feature or data access"},
		"github":     a.githubV1(),
		"identities": map[string]int{"active": active, "archived": archived},
		"authority": map[string]any{
			"grants":        "NONE — grants are not implemented in this alpha; every /api/v1/authorize answer is DENIED",
			"default":       "deny (identities start with no access)",
			"cache_policy":  "do not cache decisions; if Identity can't be reached, block protected actions and show AUTHORITY UNAVAILABLE",
			"engine_access": last,
		},
		"not_implemented": NotImplemented,
		"checked_at":      time.Now().UTC(),
	}
}

func (a *App) githubV1() map[string]any {
	v := a.GH.View()
	out := map[string]any{"state": strings.ToUpper(v.Status), "mode": v.Mode, "app_source": v.ClientIDSource,
		"token_storage": "memory only — restarting Identity signs GitHub out"}
	if v.Message != "" {
		out["message"] = v.Message
	}
	switch v.Status {
	case github.Waiting:
		out["user_code"] = v.UserCode
		out["verification_uri"] = v.VerifyURL
		out["expires_at"] = v.ExpiresAt
		out["last_checked_at"] = v.LastCheckedAt
		out["github_said"] = v.PollNote
		out["next_step"] = "Show the person this code and link. They enter it on GitHub and press Authorize. Poll GET /api/v1/github every few seconds."
	case github.Verified:
		if v.Account != nil {
			out["account"] = map[string]any{"login": v.Account.Login, "id": v.Account.ID, "verified_at": v.Account.VerifiedAt}
		}
		out["meaning"] = "GitHub confirmed this account to Identity. It is not, by itself, permission to use any feature."
	case github.NotConfigured:
		out["next_step"] = "This build has no GitHub App built in; use the Identity troubleshooting page (Advanced) to set one up."
	default:
		out["next_step"] = "POST /api/v1/github/connect to start sign-in."
	}
	return out
}

var featureLegend = map[string]string{
	"eligible_to_install":  "may this installation download/install the feature (the feature's access rule, e.g. repo-visibility-v0). Install only — NOT permission to run it or read any data",
	"installed":            "UNKNOWN_TO_IDENTITY — Thought Processor knows what it installed; Identity does not track installs in this alpha",
	"service_registered":   "NOT_REGISTERED — feature services do not register with Identity yet",
	"enabled_for_identity": "NOT_GRANTED until a grant exists; grants are not implemented in this alpha",
	"authorized_sources":   "always empty: no source is allowed by default",
	"running":              "UNKNOWN_TO_IDENTITY — ask the feature or Processor",
}

func (a *App) featureStates(ctx context.Context, e *entities.Entity, c pairing.PublicClient) []map[string]any {
	out := []map[string]any{}
	for _, f := range features.Catalog {
		if !a.canSee(ctx, f) {
			continue // private and this account isn't allowed to see it: leave no trace
		}
		st := map[string]any{"feature": f.ID, "name": f.Name, "repo": f.Repo, "description": f.Description,
			"visibility": f.Visibility, "contract_schema": features.ContractSchema,
			"installed": notTrackedByIdty, "running": notTrackedByIdty, "service_registered": "NOT_REGISTERED"}
		switch {
		case f.ID == "thought-processor":
			st["eligible_to_install"] = map[string]any{"state": "NOT_APPLICABLE", "meaning": "Thought Processor is the interface you are using"}
			st["service_registered"] = "PAIRED"
			st["paired_as"] = c.ProcessorID
		case f.InstallRule:
			d := a.EngineAccess(ctx, false)
			st["eligible_to_install"] = map[string]any{"state": d.State, "rule": d.Rule, "reason": d.Reason, "next_step": d.NextStep,
				"checked_at": d.CheckedAt, "decision_id": d.DecisionID,
				"applies_to": "this installation's verified GitHub account",
				"meaning":    "install only — not permission to run it for an identity, read sources, or share data"}
		default:
			st["eligible_to_install"] = map[string]any{"state": "NO_RULE", "meaning": "no install rule exists for this feature"}
		}
		if e != nil {
			st["enabled_for_identity"] = "NOT_GRANTED"
			st["authorized_sources"] = []string{}
		}
		out = append(out, st)
	}
	return out
}

func (a *App) authorize(r *http.Request, identityID, feature, operation, source string) map[string]any {
	d := map[string]any{"decision": DecisionDenied, "identity_id": identityID, "feature": feature,
		"operation": clip(operation, 64), "source": clip(source, 64),
		"decision_id": "dec_" + profile.RandHex(8), "checked_at": time.Now().UTC(),
		"cache": "none", "valid_for_seconds": 0,
		"policy": "default deny (C03); ask again before every protected action; if Identity can't be reached, treat as AUTHORITY UNAVAILABLE and block (C06)"}
	e, ok := a.Ents.Get(identityID)
	_, known := a.findVisible(r.Context(), feature)
	switch {
	case !ok:
		d["state"], d["reason"] = "UNKNOWN_IDENTITY", "Identity has no identity with that ID."
	case e.ArchivedAt != nil:
		d["state"], d["reason"] = "IDENTITY_ARCHIVED", "That identity is archived."
	case !known:
		d["state"], d["reason"] = "UNKNOWN_FEATURE", "Identity doesn't know that feature."
	default:
		d["state"], d["reason"] = "NO_GRANT", "No grant allows this. Every identity starts with no access, and grants can't be created in this alpha."
	}
	a.Log.Info("authority", "authorize.decided", reqID(r), DecisionDenied, d["state"].(string)+" for "+clip(feature, 40)+"/"+clip(operation, 40),
		map[string]string{"decision_id": d["decision_id"].(string), "identity_id": clip(identityID, 40), "processor": clientFrom(r).ProcessorID})
	return d
}

func installAdapter(id string) string {
	if id == EngineFeature {
		return "thought-engine: rule " + AccessRule + "; release + SHA256SUMS from the feature's GitHub releases, streamed by Identity"
	}
	return "NONE"
}

// needAdapter answers for features that have no install adapter yet.
func (a *App) needAdapter(w http.ResponseWriter, r *http.Request) bool {
	id := r.PathValue("id")
	if _, ok := a.findVisible(r.Context(), id); !ok {
		a.signedFail(w, r, 404, "features.unknown", "Identity doesn't know a feature called "+clip(id, 40))
		return false
	}
	if id != EngineFeature {
		a.signed(w, r, 404, map[string]any{"error": "NO_INSTALL_ADAPTER", "feature": id,
			"message": "Identity has no install rule for this feature yet", "request_id": reqID(r)})
		return false
	}
	return true
}

// canSee: public features are shown to everyone; a private feature only when
// this installation's verified GitHub account passes the feature's access
// rule (repo-visibility-v0 for Thought Engine). Anyone else gets exactly the
// same answers as for a feature that doesn't exist.
func (a *App) canSee(ctx context.Context, f features.Feature) bool {
	if f.Visibility != features.Private {
		return true
	}
	if !f.InstallRule {
		return false
	}
	return a.EngineAccess(ctx, false).State == Allowed
}

func (a *App) findVisible(ctx context.Context, id string) (features.Feature, bool) {
	f, ok := features.Find(id)
	if !ok || !a.canSee(ctx, f) {
		return features.Feature{}, false
	}
	return f, true
}

// catalogNote tells the person how to see more, without naming hidden features.
func (a *App) catalogNote() string {
	if a.GH.View().Status != github.Verified {
		return "Some features are only shown to approved accounts. Sign in to GitHub to see everything available to you."
	}
	return "Showing every feature your account is approved to see."
}
