package app

// Engine access: the small local API a paired Thought Processor uses to ask
// "may this person install Thought Engine?", to read the latest release, and
// to download it. Rule repo-visibility-v0 (approved by the owner 2026-10-25):
// allowed = the person's verified GitHub account can see Arameair/Thought-engine.
//
// The GitHub token never leaves this process: Identity downloads the release
// itself and streams the bytes to Processor.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/features"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/pairing"
	"github.com/Arameair/identity/internal/profile"
)

const (
	EngineFeature = "thought-engine"
	AccessRule    = "repo-visibility-v0"
	accessFresh   = 60 * time.Second
	MaxAssetBytes = 200 << 20
)

// Access states.
const (
	Allowed      = "ALLOWED"
	NotAllowed   = "NOT_ALLOWED"
	NotSignedIn  = "NOT_SIGNED_IN"
	SignInExpire = "SIGN_IN_EXPIRED"
	CheckFailed  = "CHECK_FAILED"
)

type DecisionAccount struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

type Decision struct {
	Feature          string           `json:"feature"`
	State            string           `json:"state"`
	Allowed          bool             `json:"allowed"`
	Reason           string           `json:"reason"`
	NextStep         string           `json:"next_step"`
	Hint             string           `json:"hint,omitempty"` // e.g. owner_app_not_installed
	Account          *DecisionAccount `json:"account,omitempty"`
	Rule             string           `json:"rule"`
	CheckedAt        time.Time        `json:"checked_at"`
	DecisionID       string           `json:"decision_id"`
	IdentitySystemID string           `json:"identity_system_id"`
	Proof            string           `json:"proof,omitempty"`
}

func (d Decision) canonical() string {
	var id int64
	if d.Account != nil {
		id = d.Account.ID
	}
	return strings.Join([]string{"access", d.DecisionID, d.State, strconv.FormatInt(id, 10),
		d.CheckedAt.UTC().Format(time.RFC3339Nano), d.Rule, d.IdentitySystemID}, "|")
}

type accessCache struct {
	mu   sync.Mutex
	last *Decision
}

func engineRepo() string {
	if f, ok := features.Find(EngineFeature); ok {
		return f.Repo
	}
	return "Arameair/Thought-engine"
}

// EngineAccess decides, using a cached answer up to a minute old unless fresh.
func (a *App) EngineAccess(ctx context.Context, fresh bool) Decision {
	gv := a.GH.View()
	a.access.mu.Lock()
	if !fresh && a.access.last != nil && time.Since(a.access.last.CheckedAt) < accessFresh &&
		gv.Status == github.Verified && gv.Account != nil && a.access.last.Account != nil && a.access.last.Account.ID == gv.Account.ID {
		d := *a.access.last
		a.access.mu.Unlock()
		return d
	}
	a.access.mu.Unlock()

	repo := engineRepo()
	owner := strings.SplitN(repo, "/", 2)[0]
	d := Decision{Feature: EngineFeature, Rule: AccessRule, CheckedAt: time.Now().UTC(),
		DecisionID: "dec_" + profile.RandHex(8), IdentitySystemID: a.Store.Profile.SystemID}
	switch {
	case gv.Status == github.Expired:
		d.State, d.Reason, d.NextStep = SignInExpire, "Your GitHub sign-in in Identity expired.", "Connect GitHub from Thought Processor (Identity → Connect GitHub)."
	case gv.Status != github.Verified:
		d.State = NotSignedIn
		d.Reason = "Identity isn't signed in to GitHub right now. It forgets the sign-in when it restarts."
		d.NextStep = "Connect GitHub from Thought Processor (Identity → Connect GitHub)."
	default:
		cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		rc, acct, err := a.GH.CheckRepo(cctx, repo)
		cancel()
		if acct != nil {
			d.Account = &DecisionAccount{Login: acct.Login, ID: acct.ID}
		}
		switch {
		case errors.Is(err, github.ErrNotSignedIn):
			d.State, d.Reason, d.NextStep = NotSignedIn, "Identity isn't signed in to GitHub right now.", "Connect GitHub from Thought Processor (Identity → Connect GitHub)."
		case errors.Is(err, github.ErrSignInRejected):
			d.State, d.Reason, d.NextStep = SignInExpire, "GitHub no longer accepts Identity's sign-in (it expired or was revoked).", "Connect GitHub from Thought Processor (Identity → Connect GitHub)."
		case rc.Result == "VISIBLE":
			d.State, d.Allowed = Allowed, true
			d.Reason = fmt.Sprintf("Your GitHub account %s can see %s.", acct.Login, repo)
			d.NextStep = "Install Thought Engine from Thought Processor."
		case rc.Result == "NOT VISIBLE":
			d.State = NotAllowed
			d.Reason = fmt.Sprintf("Your GitHub account %s can't see %s, so it isn't allowed to install Thought Engine.", acct.Login, repo)
			d.NextStep = fmt.Sprintf("Ask %s to give %s access to the %s repository, then check again.", owner, acct.Login, strings.SplitN(repo, "/", 2)[1])
			if strings.EqualFold(acct.Login, owner) {
				actx, acancel := context.WithTimeout(ctx, 15*time.Second)
				sees, aerr := a.GH.AppSeesRepo(actx, repo)
				acancel()
				if aerr == nil && !sees {
					d.Hint = "owner_app_not_installed"
					d.Reason = fmt.Sprintf("You own %s, but Identity's GitHub App isn't installed on it, so GitHub won't show it to Identity.", repo)
					d.NextStep = "On GitHub open the Identity app → Install App → Configure → Repository access → add " + strings.SplitN(repo, "/", 2)[1] + ", then check again."
				}
			}
		default:
			d.State = CheckFailed
			d.Reason = fmt.Sprintf("Identity couldn't ask GitHub right now (HTTP %d).", rc.HTTP)
			d.NextStep = "Check the internet connection, then check again."
		}
	}
	d.Allowed = d.State == Allowed
	a.access.mu.Lock()
	cp := d
	a.access.last = &cp
	a.access.mu.Unlock()
	login := "-"
	if d.Account != nil {
		login = d.Account.Login
	}
	level := "info"
	if d.State == CheckFailed {
		level = "warn"
	}
	a.Log.Add(level, "access", "engine.access_decided", "", d.State, d.Reason, map[string]string{
		"decision_id": d.DecisionID, "rule": d.Rule, "account": login, "hint": orDash(d.Hint)})
	return d
}

// LastAccess is the most recent decision (for the UI and diagnostics).
func (a *App) LastAccess() *Decision {
	a.access.mu.Lock()
	defer a.access.mu.Unlock()
	if a.access.last == nil {
		return nil
	}
	d := *a.access.last
	return &d
}

var platformRE = regexp.MustCompile(`^(windows|linux|darwin)-(amd64|arm64)$`)

func assetName(platform string) string {
	n := "thought-engine-" + platform
	if strings.HasPrefix(platform, "windows-") {
		n += ".exe"
	}
	return n
}

type ReleaseInfo struct {
	DecisionID  string    `json:"decision_id"`
	Tag         string    `json:"tag"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Platform    string    `json:"platform"`
	Asset       string    `json:"asset"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	Proof       string    `json:"proof,omitempty"`
	assetID     int64
}

func (ri ReleaseInfo) canonical() string {
	return strings.Join([]string{"release", ri.DecisionID, ri.Tag, ri.Asset, ri.SHA256, strconv.FormatInt(ri.Size, 10)}, "|")
}

var sumLine = regexp.MustCompile(`^([0-9a-fA-F]{64})\s+\*?(\S+)\s*$`)

// releaseFor finds the asset for platform in the latest release and its
// checksum from the release's SHA256SUMS file.
func (a *App) releaseFor(ctx context.Context, platform string) (ReleaseInfo, int, string) {
	repo := engineRepo()
	rel, err := a.GH.LatestRelease(ctx, repo)
	switch {
	case errors.Is(err, github.ErrNoRelease):
		return ReleaseInfo{}, 404, "Thought Engine has no published version yet. " + strings.SplitN(repo, "/", 2)[0] + " needs to publish a release."
	case err != nil:
		return ReleaseInfo{}, 502, "Identity couldn't read the release from GitHub: " + err.Error()
	}
	want := assetName(platform)
	ri := ReleaseInfo{Tag: rel.Tag, Name: rel.Name, PublishedAt: rel.PublishedAt, Platform: platform, Asset: want}
	var sumsID int64
	for _, as := range rel.Assets {
		if as.Name == want {
			ri.assetID, ri.Size = as.ID, as.Size
		}
		if as.Name == "SHA256SUMS" {
			sumsID = as.ID
		}
	}
	if ri.assetID == 0 {
		return ri, 404, fmt.Sprintf("Release %s has no %s file for this computer (%s).", rel.Tag, want, platform)
	}
	if sumsID == 0 {
		return ri, 422, fmt.Sprintf("Release %s has no SHA256SUMS file, so its download can't be checked. Nothing will be installed.", rel.Tag)
	}
	if ri.Size <= 0 || ri.Size > MaxAssetBytes {
		return ri, 422, fmt.Sprintf("Release file size %d is outside the allowed range.", ri.Size)
	}
	body, _, err := a.GH.OpenAsset(ctx, repo, sumsID)
	if err != nil {
		return ri, 502, "Identity couldn't read SHA256SUMS: " + err.Error()
	}
	defer body.Close()
	sc := bufio.NewScanner(io.LimitReader(body, 64<<10))
	for sc.Scan() {
		if m := sumLine.FindStringSubmatch(sc.Text()); m != nil && m[2] == want {
			ri.SHA256 = strings.ToLower(m[1])
		}
	}
	if ri.SHA256 == "" {
		return ri, 422, fmt.Sprintf("SHA256SUMS in %s has no line for %s. Nothing will be installed.", rel.Tag, want)
	}
	return ri, 200, ""
}

func clientFrom(r *http.Request) pairing.PublicClient {
	c, _ := r.Context().Value(clientKey).(pairing.PublicClient)
	return c
}

func bearer(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func (a *App) engineRoutes(m *http.ServeMux) {
	// ---- for Thought Processor (program to program) ----
	m.HandleFunc("POST /api/v1/pair", a.wrap(machine, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Code        string `json:"code"`
			ProcessorID string `json:"processor_id"`
			Label       string `json:"processor_label"`
		}
		if err := decode(r, &b); err != nil {
			a.fail(w, r, 400, "pairing", "pairing.bad_request", "expected {code, processor_id, processor_label}")
			return
		}
		token, err := a.Pair.Pair(b.Code, b.ProcessorID, b.Label)
		if err != nil {
			code := 401
			if errors.Is(err, pairing.ErrLocked) {
				code = 429
			} else if errors.Is(err, pairing.ErrBadRequest) {
				code = 400
			}
			a.fail(w, r, code, "pairing", "pairing.refused", err.Error())
			return
		}
		a.Log.Info("pairing", "pairing.paired", reqID(r), "paired", "Thought Processor "+clip(b.ProcessorID, 64)+" paired with this Identity (token kept only as a hash)", map[string]string{"label": clip(b.Label, 80)})
		writeJSON(w, 200, map[string]any{"token": token, "identity_system_id": a.Store.Profile.SystemID,
			"identity_version": a.Build.Version, "note": "keep this token private; it is shown once"})
	}))
	m.HandleFunc("GET /api/v1/features/thought-engine/access", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		d := a.EngineAccess(r.Context(), r.URL.Query().Get("fresh") == "1")
		d.Proof = pairing.Proof(bearer(r), d.canonical())
		a.Log.Info("access", "engine.access_answered", reqID(r), d.State, "answered Thought Processor "+clientFrom(r).ProcessorID, map[string]string{"decision_id": d.DecisionID})
		writeJSON(w, 200, d)
	}))
	m.HandleFunc("GET /api/v1/features/thought-engine/release", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		platform := r.URL.Query().Get("platform")
		if !platformRE.MatchString(platform) {
			a.fail(w, r, 400, "access", "engine.release_bad_platform", "platform must look like windows-amd64")
			return
		}
		d := a.EngineAccess(r.Context(), false)
		if !d.Allowed {
			d.Proof = pairing.Proof(bearer(r), d.canonical())
			writeJSON(w, 403, map[string]any{"error": "not_allowed", "decision": d})
			return
		}
		ri, code, msg := a.releaseFor(r.Context(), platform)
		if code != 200 {
			a.fail(w, r, code, "access", "engine.release_unavailable", msg)
			return
		}
		ri.DecisionID = d.DecisionID
		ri.Proof = pairing.Proof(bearer(r), ri.canonical())
		a.Log.Info("access", "engine.release_described", reqID(r), "ok", "release "+ri.Tag+" "+ri.Asset+" described to "+clientFrom(r).ProcessorID, map[string]string{"sha256": ri.SHA256, "size": strconv.FormatInt(ri.Size, 10), "decision_id": d.DecisionID})
		writeJSON(w, 200, ri)
	}))
	m.HandleFunc("GET /api/v1/features/thought-engine/release/download", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		platform := r.URL.Query().Get("platform")
		if !platformRE.MatchString(platform) {
			a.fail(w, r, 400, "access", "engine.release_bad_platform", "platform must look like windows-amd64")
			return
		}
		d := a.EngineAccess(r.Context(), false)
		if !d.Allowed {
			a.fail(w, r, 403, "access", "engine.download_refused", d.Reason)
			return
		}
		ri, code, msg := a.releaseFor(r.Context(), platform)
		if code != 200 {
			a.fail(w, r, code, "access", "engine.release_unavailable", msg)
			return
		}
		if want := r.URL.Query().Get("tag"); want != "" && want != ri.Tag {
			a.fail(w, r, 409, "access", "engine.release_changed", "the latest release changed from "+clip(want, 40)+" to "+ri.Tag+"; check again")
			return
		}
		body, _, err := a.GH.OpenAsset(r.Context(), engineRepo(), ri.assetID)
		if err != nil {
			a.fail(w, r, 502, "access", "engine.download_failed", err.Error())
			return
		}
		defer body.Close()
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(15 * time.Minute))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(ri.Size, 10))
		w.Header().Set("X-Release-Tag", ri.Tag)
		w.Header().Set("X-Asset-Name", ri.Asset)
		t0 := time.Now()
		n, err := io.Copy(w, io.LimitReader(body, ri.Size+1))
		res, lvl := "streamed", "info"
		if err != nil || n != ri.Size {
			res, lvl = "incomplete", "warn"
		}
		a.Log.Add(lvl, "access", "engine.download_streamed", reqID(r), res, fmt.Sprintf("%s %s: %d of %d bytes in %s to %s", ri.Tag, ri.Asset, n, ri.Size, time.Since(t0).Round(time.Millisecond), clientFrom(r).ProcessorID), map[string]string{"decision_id": d.DecisionID})
	}))

	// ---- for the person, in Identity's own window ----
	m.HandleFunc("POST /api/pairing/code", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		if _, _, err := a.Pair.NewCode(); err != nil {
			a.fail(w, r, 500, "pairing", "pairing.code_failed", err.Error())
			return
		}
		a.Log.Info("pairing", "pairing.code_created", reqID(r), "waiting", "one-time pairing code shown in Identity's window (valid 5 minutes, not logged)", nil)
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/pairing/cancel", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		a.Pair.CancelCode()
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/pairing/unpair", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			ProcessorID string `json:"processor_id"`
		}
		decode(r, &b)
		ok, err := a.Pair.Unpair(b.ProcessorID)
		if err != nil {
			a.fail(w, r, 500, "pairing", "pairing.unpair_failed", err.Error())
			return
		}
		if ok {
			a.Log.Info("pairing", "pairing.unpaired", reqID(r), "removed", "Thought Processor "+clip(b.ProcessorID, 64)+" unpaired; its token no longer works", nil)
		}
		writeJSON(w, 200, a.State())
	}))
	m.HandleFunc("POST /api/access/check", a.wrap(mutate, func(w http.ResponseWriter, r *http.Request) {
		a.EngineAccess(r.Context(), true)
		writeJSON(w, 200, a.State())
	}))
}

// pairingState is what Identity's own window shows.
func (a *App) pairingState() map[string]any {
	code, exp := a.Pair.Pending()
	st := map[string]any{"clients": a.Pair.Clients(), "load_error": a.Pair.LoadError()}
	if code != "" {
		st["pending_code"], st["expires_at"] = code, exp
	}
	return st
}
