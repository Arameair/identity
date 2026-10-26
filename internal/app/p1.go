package app

// P1 — original Admin Identity, Admin approvals, feature installation
// identities, scoped grants, request tickets and the live decision check.
//
// Rules this file enforces (owner decisions #20, Identity #14/#15):
//   - GitHub sign-in is an external login alias. The Admin Identity is a
//     durable LOCAL record, established once, with explicit confirmation and
//     a local authenticator. Switching GitHub accounts never transfers it.
//   - Pairing authenticates Thought Processor (a program), not a person.
//     Elevated actions (register an installation, grant a capability,
//     uninstall) need a fresh Admin approval bound to the exact action,
//     made in Identity's own page with the Admin authenticator.
//   - Every installed copy gets its own durable installation ID with ZERO grants.
//   - The receiving feature enforces: it verifies a ticket only Identity can
//     sign for it and asks Identity live before any work. No cached allows.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/authority"
	"github.com/Arameair/identity/internal/entities"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/pairing"
	"github.com/Arameair/identity/internal/profile"
	"github.com/Arameair/identity/internal/redact"
)

const (
	InstallationIDHeader = "X-Installation-ID"
	FeatureAPI           = "thought.feature-api/1"
	P1Features           = "p1: admin-identity, admin-approvals, installation-identities, scoped-grants, tickets, live-check"
	approvalCookie       = "identity_approval"
	approvalMethod       = "local passphrase (" + authority.AuthenticatorKind + ")"
)

var sha256HexRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ---- approval page sessions (scoped to ONE approval) ----

type approvalSess struct {
	approvalID string
	csrf       string
	expires    time.Time
}

type approvalSessions struct {
	mu sync.Mutex
	m  map[string]approvalSess
}

func (s *approvalSessions) open(approvalID string, exp time.Time) (id, csrf string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.m {
		if now.After(v.expires) {
			delete(s.m, k)
		}
	}
	id, csrf = profile.RandHex(32), profile.RandHex(24)
	s.m[id] = approvalSess{approvalID: approvalID, csrf: csrf, expires: exp}
	redact.Register("approval_cookie", id)
	redact.Register("approval_csrf", csrf)
	return id, csrf
}

func (s *approvalSessions) lookup(r *http.Request) (approvalSess, bool) {
	c, err := r.Cookie(approvalCookie)
	if err != nil || len(c.Value) != 64 {
		return approvalSess{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[c.Value]
	if !ok || time.Now().After(v.expires) {
		delete(s.m, c.Value)
		return approvalSess{}, false
	}
	return v, true
}

// ---- decision audit (recent, in memory; every decision is also in the persistent log) ----

type decisionLog struct {
	mu   sync.Mutex
	list []map[string]any
}

func (d *decisionLog) add(m map[string]any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.list = append(d.list, m)
	if len(d.list) > 300 {
		d.list = d.list[len(d.list)-300:]
	}
}

func (d *decisionLog) forInstallation(id string, n int) []map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := []map[string]any{}
	for i := len(d.list) - 1; i >= 0 && len(out) < n; i-- {
		if id == "" || d.list[i]["destination_installation_id"] == id || d.list[i]["caller"] == "installation:"+id {
			out = append(out, d.list[i])
		}
	}
	return out
}

func (a *App) decide(r *http.Request, event, decision, reason string, f map[string]string) string {
	id := "dec_" + profile.RandHex(8)
	m := map[string]any{"decision_id": id, "decision": decision, "reason": reason, "at": time.Now().UTC(), "event": event,
		"caller": principalOf(r), "request_id": reqID(r)}
	for k, v := range f {
		m[k] = v
	}
	a.decisions.add(m)
	lf := map[string]string{"decision_id": id, "caller": principalOf(r)}
	for k, v := range f {
		lf[k] = v
	}
	level := "info"
	if decision != "ALLOW" && decision != "ALLOW_TICKET" {
		level = "warn"
	}
	a.Log.Add(level, "authority", event, reqID(r), decision, orDash(reason), lf)
	return id
}

// ---- helpers ----

func (a *App) approvalURL(ap authority.Approval, token string) string {
	return "http://127.0.0.1:" + strconv.Itoa(a.Port) + "/approve#a=" + ap.ID + "&t=" + token
}

func (a *App) announceApproval(ap authority.Approval, url string) {
	redact.Register("approval_link", url)
	if a.opt.Announce == nil {
		return
	}
	lines := []string{"", "  +-- ADMIN APPROVAL REQUEST " + ap.ID + " (" + ap.Kind + ") from " + ap.RequestedBy}
	for _, s := range ap.Summary {
		lines = append(lines, "  |   "+s)
	}
	lines = append(lines, "  |", "  |   Approve or deny it in Identity's own page (your Admin passphrase is typed THERE, never in Processor):",
		"  |   "+url, "  |   Valid until "+ap.ExpiresAt.Local().Format("15:04:05")+". If you didn't ask for this, ignore it.", "  +--", "")
	a.opt.Announce(lines)
}

func (a *App) newApproval(w http.ResponseWriter, r *http.Request, kind string, action any, summary []string) {
	ap, token, err := a.Approvals.Create(kind, action, summary, principalOf(r))
	if err != nil {
		code := 503
		if errors.Is(err, authority.ErrTooManyPending) {
			code = 429
		}
		a.signedFail(w, r, code, "approval.create_failed", err.Error())
		return
	}
	url := a.approvalURL(ap, token)
	a.announceApproval(ap, url)
	a.Log.Info("approval", "approval.requested", reqID(r), "PENDING", ap.Kind+" requested by "+ap.RequestedBy+"; waiting for the Admin in Identity's own page",
		map[string]string{"approval_id": ap.ID, "action_sha256": ap.Digest})
	a.signed(w, r, 202, map[string]any{"approval": ap.View(time.Now()), "approval_url": url,
		"next": "The Admin opens approval_url (Identity's own page), checks the details and approves with the Admin passphrase. Then POST the commit route with {approval_id}.",
		"note": "Asking for an approval grants nothing. Processor cannot approve it."})
}

// beginApproval is the ONE guard every elevated commit route goes through.
func (a *App) beginApproval(w http.ResponseWriter, r *http.Request, kind string) (authority.Approval, authority.Admin, bool, bool) {
	var b struct {
		ApprovalID string `json:"approval_id"`
	}
	decode(r, &b)
	if !authority.ValidApprovalID(b.ApprovalID) {
		a.decide(r, "elevated.refused", "DENIED", "ADMIN_APPROVAL_REQUIRED", map[string]string{"kind": kind})
		a.signed(w, r, 403, map[string]any{"error": "ADMIN_APPROVAL_REQUIRED", "request_id": reqID(r),
			"message": "This is an elevated action. It needs an approval_id that the Admin approved in Identity's own page. Pairing alone is not Admin authority."})
		return authority.Approval{}, authority.Admin{}, false, false
	}
	adm, err := a.Admin.Get()
	if err != nil {
		a.signedFail(w, r, 409, "elevated.no_admin", err.Error())
		return authority.Approval{}, authority.Admin{}, false, false
	}
	ap, fresh, err := a.Approvals.Begin(b.ApprovalID, kind, principalOf(r))
	if err == nil && ap.DecidedBy != adm.AdminID {
		err = errors.New("that approval was not made by this installation's Admin Identity")
	}
	if err != nil {
		code := 403
		if errors.Is(err, authority.ErrApprovalsDamaged) {
			code = 503
		}
		a.decide(r, "elevated.refused", "DENIED", err.Error(), map[string]string{"kind": kind, "approval_id": b.ApprovalID})
		a.signed(w, r, code, map[string]any{"error": "APPROVAL_NOT_USABLE", "message": err.Error(), "request_id": reqID(r)})
		return authority.Approval{}, authority.Admin{}, false, false
	}
	return ap, adm, fresh, true
}

func entityProblem(e entities.Entity, ok bool) string {
	if !ok {
		return "no identity with that ID"
	}
	if e.ArchivedAt != nil {
		return "that identity is archived"
	}
	return ""
}

func (a *App) adminView() map[string]any {
	st, pub, why := a.Admin.State()
	gv := a.GH.View()
	out := map[string]any{"state": st,
		"meaning": "The Admin Identity is this installation's original administrator: a durable LOCAL record. GitHub is only the external sign-in bound to it. Pairing does not make Processor an Admin.",
		"authenticator_options": []map[string]string{
			{"kind": authority.AuthenticatorKind, "status": "AVAILABLE", "note": "Linux/WSL fallback: passphrase typed only into Identity's own page; PBKDF2-HMAC-SHA256 verifier stored 0600"},
			{"kind": "platform-authenticator (Windows Hello / passkey)", "status": "BLOCKED", "note": "not implemented or tested in P1"},
		},
		"recovery": "DEFERRED (owner decision): losing the Admin passphrase makes this TEST profile unrecoverable; no fallback owner is ever created",
	}
	if why != "" {
		out["error"] = why
	}
	if pub != nil {
		out["admin"] = pub
	}
	if gv.Status == github.Verified && gv.Account != nil {
		cur := map[string]any{"id": gv.Account.ID, "login": gv.Account.Login, "mode": gv.Mode}
		if pub != nil {
			cur["is_admin_binding"] = gv.Account.ID == pub.GitHub.ID
			if gv.Account.ID != pub.GitHub.ID {
				cur["note"] = "This signed-in GitHub account is NOT the account bound to the Admin Identity. It gets no Admin authority."
			}
		}
		out["current_github"] = cur
	} else {
		out["current_github"] = map[string]any{"state": strings.ToUpper(gv.Status)}
	}
	return out
}

func (a *App) installationView(in authority.Installation) map[string]any {
	now := time.Now()
	grants := []map[string]any{}
	for _, g := range a.Grants.List() {
		if g.Destination == in.ID || g.Requester == "installation:"+in.ID {
			grants = append(grants, map[string]any{"grant": g, "status": g.Status(now)})
		}
	}
	active := 0
	for _, g := range grants {
		if g["status"] == "ACTIVE" {
			active++
		}
	}
	return map[string]any{"installation": in, "grants": grants, "active_grants": active,
		"information_sources": []string{}, "information_sources_note": "none: P1 never grants an information source",
		"credential_registered": a.InstKeys.Has(in.ID)}
}

// ---- routes ----

func (a *App) p1Routes(m *routeMux) {
	// Admin Identity
	m.HandleFunc("GET /api/v1/admin", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		a.signed(w, r, 200, a.adminView())
	}))
	m.HandleFunc("POST /api/v1/admin/enrollment", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		st, _, why := a.Admin.State()
		switch st {
		case "ENROLLED":
			a.decide(r, "admin.enroll_refused", "DENIED", "ALREADY_ENROLLED", nil)
			a.signed(w, r, 409, map[string]any{"error": "ALREADY_ENROLLED", "message": authority.ErrAdminExists.Error(), "admin": a.adminView()})
			return
		case "DAMAGED":
			a.signedFail(w, r, 503, "admin.damaged", why)
			return
		}
		gv := a.GH.View()
		if gv.Status != github.Verified || gv.Account == nil {
			a.signed(w, r, 409, map[string]any{"error": "GITHUB_NOT_VERIFIED", "message": "Sign in to GitHub first. The Admin Identity is established right after a verified GitHub sign-in, with your explicit confirmation."})
			return
		}
		acct := gv.Account
		action := map[string]any{"system_id": a.Store.Profile.SystemID, "github_id": acct.ID, "github_login": acct.Login,
			"github_mode": gv.Mode, "github_verified_at": acct.VerifiedAt}
		summary := []string{"Establish this installation's Admin Identity (once; it cannot be transferred by signing in again).",
			"Bind GitHub account " + acct.Login + " (id " + strconv.FormatInt(acct.ID, 10) + ", " + gv.Mode + ") as its sign-in alias.",
			"Choose the local Admin passphrase used to approve elevated actions, also offline."}
		a.newApproval(w, r, authority.KindEnrollAdmin, action, summary)
	}))

	m.HandleFunc("GET /api/v1/approvals/{id}", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		ap, ok := a.Approvals.Get(r.PathValue("id"))
		if !ok || ap.RequestedBy != principalOf(r) {
			a.signedFail(w, r, 404, "approval.not_found", "no such approval for this program")
			return
		}
		a.signed(w, r, 200, ap.View(time.Now()))
	}))

	// Installations
	m.HandleFunc("POST /api/v1/installations/requests", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		var req authority.InstallRequest
		if decode(r, &req) != nil {
			a.signedFail(w, r, 400, "install.bad_request", "expected an install request")
			return
		}
		req.RequestedBy = principalOf(r)
		if err := req.Validate(a.pins); err != nil {
			code := 400
			if errors.Is(err, authority.ErrUntrusted) || errors.Is(err, authority.ErrNotPinned) {
				code = 403
			}
			a.decide(r, "install.request_refused", "DENIED", err.Error(), map[string]string{"feature": clip(req.FeatureID, 48)})
			a.signed(w, r, code, map[string]any{"error": "INSTALL_REQUEST_REFUSED", "message": err.Error(), "request_id": reqID(r)})
			return
		}
		if _, err := a.Admin.Get(); err != nil {
			a.signedFail(w, r, 409, "install.no_admin", err.Error())
			return
		}
		caps := []string{}
		for _, c := range req.Capabilities {
			caps = append(caps, c.ID)
		}
		summary := []string{"Register a NEW installation identity for feature " + req.FeatureID + " " + req.Version + " (" + req.Name + ").",
			"Trust: " + a.pins.Basis(req.ArtifactSHA256) + " — exact artifact pinned (the package's own trust label is not evidence). Source " + req.SourceKind + ": " + req.SourceRef,
			"Publisher (claimed, NOT verified): " + orDash(req.PublisherClaim),
			"Artifact SHA-256: " + req.ArtifactSHA256,
			"Declares capabilities: " + orDash(strings.Join(caps, ", ")) + ". It will get ZERO grants and ZERO information sources."}
		a.newApproval(w, r, authority.KindInstall, req, summary)
	}))
	m.HandleFunc("POST /api/v1/installations", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		ap, adm, fresh, ok := a.beginApproval(w, r, authority.KindInstall)
		if !ok {
			return
		}
		var req authority.InstallRequest
		json.Unmarshal(ap.Action, &req)
		basis := a.pins.Basis(req.ArtifactSHA256)
		if basis == "" { // pins can change across restarts: re-check at commit
			a.Approvals.Finish(ap.ID, "", "FAILED: artifact no longer pinned")
			a.decide(r, "install.refused", "DENIED", "ARTIFACT_NOT_PINNED", map[string]string{"feature": clip(req.FeatureID, 48)})
			a.signed(w, r, 403, map[string]any{"error": "INSTALL_REQUEST_REFUSED", "message": authority.ErrNotPinned.Error(), "request_id": reqID(r)})
			return
		}
		in, created, err := a.Installs.Reserve(req, ap.ID, adm.AdminID, basis)
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 409, "install.reserve_failed", err.Error())
			return
		}
		resp := map[string]any{"installation": in, "created": created, "approval_reused": !fresh}
		switch in.State {
		case authority.InsPending:
			// Issue (or, on a retry, rotate) the credential. Only the newest works.
			secret, err := a.InstKeys.EnrollWithTicketKey(in.ID, in.FeatureID)
			if err != nil {
				a.signedFail(w, r, 503, "install.credential_failed", "the installation credential could not be saved: "+err.Error())
				return
			}
			in, err = a.Installs.Update(in.ID, func(x *authority.Installation) error { x.CredentialGen++; return nil })
			if err != nil {
				a.signedFail(w, r, 503, "install.save_failed", err.Error())
				return
			}
			resp["installation"] = in
			resp["credential"] = map[string]any{"installation_id": in.ID, "secret": secret, "identity_system_id": a.Store.Profile.SystemID,
				"generation": in.CredentialGen, "api": APIVersion,
				"note": "shown once; store it 0600 for the feature, then the FEATURE confirms with POST /api/v1/installation/confirm signed with it"}
		default:
			resp["credential"] = nil
			resp["note"] = "this installation is already " + in.State + "; no new credential is issued"
		}
		a.Approvals.Finish(ap.ID, in.ID, "RESERVED "+in.State)
		a.decide(r, "install.registered", "ALLOW", "", map[string]string{"installation_id": in.ID, "feature": in.FeatureID,
			"approval_id": ap.ID, "created": strconv.FormatBool(created)})
		a.signed(w, r, 200, resp)
	}))
	m.HandleFunc("GET /api/v1/installations", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		if e := a.Installs.LoadError(); e != "" {
			a.signedFail(w, r, 503, "installations.unavailable", e)
			return
		}
		out := []map[string]any{}
		for _, in := range a.Installs.List() {
			out = append(out, a.installationView(in))
		}
		a.signed(w, r, 200, map[string]any{"installations": out, "count": len(out)})
	}))
	m.HandleFunc("GET /api/v1/installations/{id}", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		in, err := a.Installs.Get(r.PathValue("id"))
		if err != nil {
			a.signedFail(w, r, 404, "installations.not_found", err.Error())
			return
		}
		v := a.installationView(in)
		v["decisions"] = a.decisions.forInstallation(in.ID, 50)
		a.signed(w, r, 200, v)
	}))
	m.HandleFunc("POST /api/v1/installations/{id}/abandon", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		in, err := a.Installs.Update(id, func(x *authority.Installation) error {
			if x.RequestedBy != principalOf(r) {
				return errors.New("only the program that requested this installation can abandon it")
			}
			if x.State != authority.InsPending {
				return errors.New("only an unconfirmed (" + authority.InsPending + ") registration can be abandoned; use uninstall")
			}
			t := time.Now().UTC()
			x.State, x.EndedAt, x.EndedBy = authority.InsAbandoned, &t, principalOf(r)
			return nil
		})
		if err != nil {
			a.signedFail(w, r, 409, "install.abandon_refused", err.Error())
			return
		}
		a.InstKeys.Unpair(id)
		a.Log.Info("install", "install.abandoned", reqID(r), in.State, "unconfirmed installation "+id+" abandoned; its credential was removed", nil)
		a.signed(w, r, 200, map[string]any{"installation": in})
	}))
	m.HandleFunc("POST /api/v1/installations/{id}/uninstall/requests", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		in, err := a.Installs.Get(r.PathValue("id"))
		if err != nil {
			a.signedFail(w, r, 404, "installations.not_found", err.Error())
			return
		}
		if in.State != authority.InsInstalled && in.State != authority.InsPending {
			a.signedFail(w, r, 409, "uninstall.refused", "installation is already "+in.State)
			return
		}
		a.newApproval(w, r, authority.KindUninstall, map[string]string{"installation_id": in.ID, "feature_id": in.FeatureID},
			[]string{"Uninstall installation " + in.ID + " (" + in.FeatureID + " " + in.Version + ").",
				"All its grants are revoked and its credential removed. A tombstone keeps the ID; it is never reused."})
	}))
	m.HandleFunc("POST /api/v1/installations/{id}/uninstall", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		ap, _, fresh, ok := a.beginApproval(w, r, authority.KindUninstall)
		if !ok {
			return
		}
		var act struct {
			ID string `json:"installation_id"`
		}
		json.Unmarshal(ap.Action, &act)
		if act.ID != r.PathValue("id") {
			a.Approvals.Finish(ap.ID, "", "FAILED: approval is for "+act.ID)
			a.signedFail(w, r, 403, "uninstall.wrong_target", "that approval is for a different installation")
			return
		}
		if !fresh {
			in, _ := a.Installs.Get(act.ID)
			a.signed(w, r, 200, map[string]any{"installation": in, "approval_reused": true})
			return
		}
		n, err := a.Grants.RevokeAllFor(act.ID, "admin-approval:"+ap.ID, "installation uninstalled")
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 503, "uninstall.revoke_failed", err.Error())
			return
		}
		if _, err := a.InstKeys.Unpair(act.ID); err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 503, "uninstall.credential_failed", err.Error())
			return
		}
		in, err := a.Installs.Update(act.ID, func(x *authority.Installation) error {
			t := time.Now().UTC()
			x.State, x.EndedAt, x.EndedBy = authority.InsUninstalled, &t, "admin-approval:"+ap.ID
			return nil
		})
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 503, "uninstall.save_failed", err.Error())
			return
		}
		a.Approvals.Finish(ap.ID, in.ID, "UNINSTALLED")
		a.decide(r, "install.uninstalled", "ALLOW", "", map[string]string{"installation_id": in.ID, "grants_revoked": strconv.Itoa(n), "approval_id": ap.ID})
		a.signed(w, r, 200, map[string]any{"installation": in, "grants_revoked": n})
	}))

	// Grants
	m.HandleFunc("POST /api/v1/grants/requests", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Requester        string `json:"requester"`
			Destination      string `json:"destination_installation_id"`
			Subject          string `json:"subject_identity_id"`
			Capability       string `json:"capability"`
			ExpiresInSeconds int64  `json:"expires_in_seconds"`
			UntilRevoked     bool   `json:"until_revoked"`
		}
		if decode(r, &b) != nil || (b.UntilRevoked && b.ExpiresInSeconds != 0) {
			a.signedFail(w, r, 400, "grant.bad_request", "expected {destination_installation_id, subject_identity_id, capability, expires_in_seconds? | until_revoked?, requester?}")
			return
		}
		if _, err := a.Admin.Get(); err != nil {
			a.signedFail(w, r, 409, "grant.no_admin", err.Error())
			return
		}
		requester := principalOf(r)
		if b.Requester != "" && b.Requester != requester {
			// Another requester may be named only if it is a registered, installed feature.
			id, ok := strings.CutPrefix(b.Requester, "installation:")
			ri, err := a.Installs.Get(id)
			if !ok || err != nil || ri.State != authority.InsInstalled {
				a.signedFail(w, r, 400, "grant.bad_requester", "requester must be this Processor or an installed feature (installation:ins_…)")
				return
			}
			requester = b.Requester
		}
		dest, err := a.Installs.Get(b.Destination)
		if err != nil || dest.State != authority.InsInstalled {
			a.signedFail(w, r, 400, "grant.bad_destination", "destination must be an INSTALLED installation")
			return
		}
		if !dest.HasCapability(b.Capability) {
			a.signedFail(w, r, 400, "grant.bad_capability", "that installation does not declare capability "+clip(b.Capability, 64))
			return
		}
		e, ok := a.Ents.Get(b.Subject)
		if p := entityProblem(e, ok); p != "" {
			a.signedFail(w, r, 400, "grant.bad_subject", p)
			return
		}
		// Default: a finite expiry (Grok M3). "Until revoked" only when asked
		// for explicitly AND confirmed by the Admin with an extra tick.
		var exp *time.Time
		expTxt := "NO EXPIRY — until revoked (the Admin must confirm this separately)"
		if !b.UntilRevoked {
			secs := b.ExpiresInSeconds
			if secs == 0 {
				secs = int64(authority.DefaultGrantTTL / time.Second)
			}
			if secs < 5 || secs > 30*24*3600 {
				a.signedFail(w, r, 400, "grant.bad_expiry", "expires_in_seconds must be 5 … 2592000 (omit it for the 1-hour default; use until_revoked:true for no expiry)")
				return
			}
			t := time.Now().Add(time.Duration(secs) * time.Second).UTC().Truncate(time.Second)
			exp, expTxt = &t, "expires "+t.Local().Format("2006-01-02 15:04:05")
			if b.ExpiresInSeconds == 0 {
				expTxt += " (default: 1 hour)"
			}
		}
		req := authority.GrantRequest{Requester: requester, Destination: dest.ID, Subject: e.ID, Capability: b.Capability,
			SourceScope: "none", ExpiresAt: exp, UntilRevoked: b.UntilRevoked, RequestedBy: principalOf(r)}
		a.newApproval(w, r, authority.KindGrant, req, []string{
			"Allow " + a.principalLabel(requester) + " to ask " + dest.Name + " (" + dest.FeatureID + ", installation " + dest.ID + ")",
			"to perform ONE capability: " + b.Capability,
			"for identity " + e.Name + " (" + e.ID + ", " + e.Kind + "). Information sources: none. " + expTxt + "."})
	}))
	m.HandleFunc("POST /api/v1/grants", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		ap, adm, fresh, ok := a.beginApproval(w, r, authority.KindGrant)
		if !ok {
			return
		}
		if !fresh {
			g, err := a.Grants.Get(ap.Result)
			if err != nil {
				a.signedFail(w, r, 409, "grant.reused_missing", err.Error())
				return
			}
			a.signed(w, r, 200, map[string]any{"grant": g, "status": g.Status(time.Now()), "approval_reused": true})
			return
		}
		var req authority.GrantRequest
		json.Unmarshal(ap.Action, &req)
		// Re-validate: the world may have changed since the approval.
		dest, err := a.Installs.Get(req.Destination)
		e, eok := a.Ents.Get(req.Subject)
		switch {
		case err != nil || dest.State != authority.InsInstalled:
			err = errors.New("destination is no longer installed")
		case entityProblem(e, eok) != "":
			err = errors.New("subject identity is no longer available")
		case req.ExpiresAt != nil && !time.Now().Before(*req.ExpiresAt):
			err = errors.New("the approved expiry has already passed")
		case req.ExpiresAt == nil && !(req.UntilRevoked && ap.ConfirmedUntilRevoked):
			err = errors.New("a grant without expiry needs the Admin's explicit 'until revoked' confirmation in Identity's page")
		}
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 409, "grant.revalidate_failed", err.Error())
			return
		}
		g, err := a.Grants.Create(req, ap.ID, adm.AdminID)
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 503, "grant.save_failed", err.Error())
			return
		}
		a.Approvals.Finish(ap.ID, g.ID, "GRANTED")
		a.decide(r, "grant.created", "ALLOW", "", map[string]string{"grant_id": g.ID, "destination_installation_id": g.Destination,
			"subject": g.Subject, "capability": g.Capability, "requester": g.Requester, "approval_id": ap.ID, "admin_id": adm.AdminID})
		a.signed(w, r, 201, map[string]any{"grant": g, "status": g.Status(time.Now())})
	}))
	m.HandleFunc("GET /api/v1/grants", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		if e := a.Grants.LoadError(); e != "" {
			a.signedFail(w, r, 503, "grants.unavailable", e)
			return
		}
		ins := r.URL.Query().Get("installation")
		now := time.Now()
		out := []map[string]any{}
		for _, g := range a.Grants.List() {
			if ins == "" || g.Destination == ins || g.Requester == "installation:"+ins {
				out = append(out, map[string]any{"grant": g, "status": g.Status(now)})
			}
		}
		a.signed(w, r, 200, map[string]any{"grants": out})
	}))
	// Revocation (Grok M2). Revoking narrows access, so the parties to a grant
	// may do it without an approval: the requester it names, the program that
	// asked for it, or the destination feature (installation route). Anyone
	// else — e.g. an unrelated paired program — is refused and must ask the
	// Admin (emergency path below). Every revocation records who and when.
	selfRevoke := func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Reason string `json:"reason"`
		}
		decode(r, &b)
		g, err := a.Grants.Get(r.PathValue("id"))
		if err == nil && !g.MayRevoke(principalOf(r)) {
			a.decide(r, "grant.revoke_refused", "DENIED", "REVOKE_NOT_PERMITTED", map[string]string{"grant_id": g.ID, "destination_installation_id": g.Destination})
			a.signed(w, r, 403, map[string]any{"error": "REVOKE_NOT_PERMITTED", "request_id": reqID(r),
				"message": "Only the program the grant was made for, the program that asked for it, or the receiving feature can revoke it directly. Anyone else must ask the Admin: POST /api/v1/grants/{id}/revoke/requests."})
			return
		}
		if err == nil {
			g, err = a.Grants.Revoke(g.ID, principalOf(r), b.Reason)
		}
		if err != nil {
			code := 503
			if errors.Is(err, authority.ErrNoGrant) {
				code = 404
			}
			a.signedFail(w, r, code, "grant.revoke_failed", err.Error())
			return
		}
		a.decide(r, "grant.revoked", "REVOKED", orDash(b.Reason), map[string]string{"grant_id": g.ID, "destination_installation_id": g.Destination, "revoked_by": g.RevokedBy})
		a.signed(w, r, 200, map[string]any{"grant": g, "status": g.Status(time.Now())})
	}
	m.HandleFunc("POST /api/v1/grants/{id}/revoke", a.wrap(paired, selfRevoke))
	m.HandleFunc("POST /api/v1/installation/grants/{id}/revoke", a.wrap(installation, selfRevoke))
	m.HandleFunc("POST /api/v1/grants/{id}/revoke/requests", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Reason string `json:"reason"`
		}
		decode(r, &b)
		g, err := a.Grants.Get(r.PathValue("id"))
		if err != nil {
			a.signedFail(w, r, 404, "grant.not_found", err.Error())
			return
		}
		if st := g.Status(time.Now()); st != "ACTIVE" {
			a.signedFail(w, r, 409, "grant.not_active", "that grant is already "+st)
			return
		}
		if _, err := a.Admin.Get(); err != nil {
			a.signedFail(w, r, 409, "grant.no_admin", err.Error())
			return
		}
		a.newApproval(w, r, authority.KindRevokeGrant, map[string]string{"grant_id": g.ID, "reason": clip(b.Reason, 120)}, []string{
			"Revoke (end) grant " + g.ID + ": " + a.principalLabel(g.Requester) + " may no longer ask installation " + g.Destination + " for " + g.Capability + ".",
			"This only removes access. Asked for by " + a.principalLabel(principalOf(r)) + "."})
	}))
	m.HandleFunc("POST /api/v1/grants/{id}/admin-revoke", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		ap, adm, fresh, ok := a.beginApproval(w, r, authority.KindRevokeGrant)
		if !ok {
			return
		}
		var act struct {
			ID     string `json:"grant_id"`
			Reason string `json:"reason"`
		}
		json.Unmarshal(ap.Action, &act)
		if act.ID != r.PathValue("id") {
			a.Approvals.Finish(ap.ID, "", "FAILED: approval is for "+act.ID)
			a.signedFail(w, r, 403, "revoke.wrong_target", "that approval is for a different grant")
			return
		}
		by := "admin:" + adm.AdminID + " (approval " + ap.ID + ")"
		g, err := a.Grants.Revoke(act.ID, by, act.Reason)
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 503, "grant.revoke_failed", err.Error())
			return
		}
		a.Approvals.Finish(ap.ID, g.ID, "REVOKED")
		a.decide(r, "grant.revoked", "REVOKED", orDash(act.Reason), map[string]string{"grant_id": g.ID, "destination_installation_id": g.Destination,
			"revoked_by": g.RevokedBy, "approval_id": ap.ID, "admin_id": adm.AdminID})
		a.signed(w, r, 200, map[string]any{"grant": g, "status": g.Status(time.Now()), "approval_reused": !fresh})
	}))

	// Tickets: an authenticated requester asks for a one-request ticket.
	ticket := func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Destination string `json:"destination_installation_id"`
			Capability  string `json:"capability"`
			Subject     string `json:"subject_identity_id"`
			Digest      string `json:"request_sha256"`
		}
		if decode(r, &b) != nil || !sha256HexRE.MatchString(b.Digest) {
			a.signedFail(w, r, 400, "ticket.bad_request", "expected {destination_installation_id, capability, subject_identity_id, request_sha256}")
			return
		}
		f := map[string]string{"destination_installation_id": clip(b.Destination, 40), "capability": clip(b.Capability, 64),
			"subject": clip(b.Subject, 40), "requester": principalOf(r)}
		deny := func(code int, reason string) {
			id := a.decide(r, "ticket.denied", "DENIED", reason, f)
			a.signed(w, r, code, map[string]any{"decision": "DENIED", "reason": reason, "decision_id": id, "request_id": reqID(r),
				"meaning": "no protected work may happen; nothing was started"})
		}
		if e := a.Grants.LoadError(); e != "" {
			deny(503, "AUTHORITY_DATA_UNAVAILABLE")
			return
		}
		dest, err := a.Installs.Get(b.Destination)
		if err != nil || dest.State != authority.InsInstalled {
			deny(403, authority.DenyDestination)
			return
		}
		if a.pins.Basis(dest.ArtifactSHA256) == "" {
			deny(403, "ARTIFACT_NOT_PINNED")
			return
		}
		e, ok := a.Ents.Get(b.Subject)
		if entityProblem(e, ok) != "" {
			deny(403, authority.DenySubject)
			return
		}
		g, why := a.Grants.FindActive(principalOf(r), dest.ID, e.ID, b.Capability)
		if why != "" {
			deny(403, why)
			return
		}
		key, ok := a.InstKeys.TicketKeyFor(dest.ID)
		if !ok {
			deny(403, "DESTINATION_HAS_NO_CREDENTIAL")
			return
		}
		tk, p, err := a.Grants.Issue(g, a.Store.Profile.SystemID, b.Digest, key)
		if err != nil {
			deny(503, err.Error())
			return
		}
		f["grant_id"], f["ticket_id"] = g.ID, p.TicketID
		id := a.decide(r, "ticket.issued", "ALLOW_TICKET", "", f)
		a.signed(w, r, 200, map[string]any{"decision": "ALLOW_TICKET", "decision_id": id, "ticket": tk, "ticket_id": p.TicketID,
			"grant_id": g.ID, "grant_version": g.Version, "expires_at": time.Unix(p.ExpiresAt, 0).UTC(),
			"meaning": "one request, this exact body, this recipient only; the recipient must still ask Identity before doing anything"})
	}
	m.HandleFunc("POST /api/v1/tickets", a.wrap(paired, ticket))
	m.HandleFunc("POST /api/v1/installation/tickets", a.wrap(installation, ticket))

	m.HandleFunc("GET /api/v1/decisions", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		a.signed(w, r, 200, map[string]any{"decisions": a.decisions.forInstallation(r.URL.Query().Get("installation"), 100),
			"note": "recent decisions since Identity started; every decision is also in Identity's persistent log"})
	}))

	// ---- called by feature installations (their own credential) ----
	m.HandleFunc("POST /api/v1/installation/confirm", a.wrap(installation, func(w http.ResponseWriter, r *http.Request) {
		id := clientFrom(r).ProcessorID
		in, err := a.Installs.Update(id, func(x *authority.Installation) error {
			switch x.State {
			case authority.InsInstalled:
				return nil
			case authority.InsPending:
				t := time.Now().UTC()
				x.State, x.ConfirmedAt = authority.InsInstalled, &t
				return nil
			}
			return errors.New("installation is " + x.State)
		})
		if err != nil {
			a.signedFail(w, r, 409, "install.confirm_refused", err.Error())
			return
		}
		a.Log.Info("install", "install.confirmed", reqID(r), in.State, "installation "+id+" proved it holds its credential; it has ZERO grants", nil)
		a.signed(w, r, 200, map[string]any{"installation": in})
	}))
	m.HandleFunc("GET /api/v1/installation/self", a.wrap(installation, func(w http.ResponseWriter, r *http.Request) {
		in, err := a.Installs.Get(clientFrom(r).ProcessorID)
		if err != nil {
			a.signedFail(w, r, 404, "install.self_missing", err.Error())
			return
		}
		a.signed(w, r, 200, a.installationView(in))
	}))
	// The live decision. The caller is the RECEIVING feature, authenticated by
	// its own credential; it reports the ticket it holds. ALLOW consumes the
	// ticket. Anything else — including any storage problem — is not ALLOW.
	m.HandleFunc("POST /api/v1/authz/check", a.wrap(installation, func(w http.ResponseWriter, r *http.Request) {
		var c authority.CheckRequest
		caller := clientFrom(r).ProcessorID
		f := map[string]string{"destination_installation_id": caller}
		deny := func(code int, reason string) {
			id := a.decide(r, "authz.checked", "DENIED", reason, f)
			a.signed(w, r, code, map[string]any{"decision": "DENIED", "reason": reason, "decision_id": id, "checked_at": time.Now().UTC(), "cache": "none"})
		}
		if decode(r, &c) != nil {
			deny(400, "BAD_REQUEST")
			return
		}
		f["ticket_id"], f["grant_id"], f["capability"], f["subject"], f["requester"] = clip(c.TicketID, 40), clip(c.GrantID, 40), clip(c.Capability, 64), clip(c.Subject, 40), clip(c.Requester, 80)
		in, err := a.Installs.Get(caller)
		if err != nil || in.State != authority.InsInstalled {
			deny(403, authority.DenyDestination)
			return
		}
		if a.pins.Basis(in.ArtifactSHA256) == "" {
			deny(403, "ARTIFACT_NOT_PINNED")
			return
		}
		e, ok := a.Ents.Get(c.Subject)
		if entityProblem(e, ok) != "" {
			deny(403, authority.DenySubject)
			return
		}
		g, why := a.Grants.Check(caller, c)
		if why == "" && g.Destination != caller {
			why = authority.DenyWrongAudience
		}
		if why != "" {
			deny(403, why)
			return
		}
		id := a.decide(r, "authz.checked", "ALLOW", "", f)
		a.signed(w, r, 200, map[string]any{"decision": "ALLOW", "decision_id": id, "grant_id": g.ID, "grant_version": g.Version,
			"capability": g.Capability, "subject_identity_id": g.Subject, "requester": g.Requester, "checked_at": time.Now().UTC(),
			"cache": "none — ask again for every request"})
	}))

	// ---- Identity's own approval page (the Admin types the passphrase HERE) ----
	m.HandleFunc("GET /approve", a.wrap(public, func(w http.ResponseWriter, r *http.Request) {
		b, err := webFS.ReadFile("web/approve.html")
		if err != nil {
			http.Error(w, "missing", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	}))
	m.HandleFunc("GET /approve.js", a.wrap(public, func(w http.ResponseWriter, r *http.Request) {
		b, _ := webFS.ReadFile("web/approve.js")
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Write(b)
	}))
	m.HandleFunc("POST /api/approval/open", a.wrap(approvalPage, func(w http.ResponseWriter, r *http.Request) {
		var b struct{ ID, Token string }
		decode(r, &b)
		ap, err := a.Approvals.OpenView(b.ID, b.Token)
		if err != nil {
			a.fail(w, r, 401, "approval", "approval.link_refused", err.Error())
			return
		}
		id, csrf := a.apSess.open(ap.ID, ap.ExpiresAt)
		http.SetCookie(w, &http.Cookie{Name: approvalCookie, Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		a.Log.Info("approval", "approval.page_opened", reqID(r), "viewing", "the approval page for "+ap.ID+" was opened (one-time link used)", nil)
		writeJSON(w, 200, map[string]any{"csrf": csrf, "approval": ap.View(time.Now()), "display": a.approvalDisplay(ap), "admin": a.adminView(), "system_id": a.Store.Profile.SystemID})
	}))
	m.HandleFunc("GET /api/approval/view", a.wrap(approvalPage, func(w http.ResponseWriter, r *http.Request) {
		s, ok := a.apSess.lookup(r)
		if !ok {
			writeJSON(w, 401, map[string]string{"error": "This approval page has expired. Ask again from Thought Processor."})
			return
		}
		ap, _ := a.Approvals.Get(s.approvalID)
		writeJSON(w, 200, map[string]any{"approval": ap.View(time.Now()), "display": a.approvalDisplay(ap), "admin": a.adminView(), "system_id": a.Store.Profile.SystemID})
	}))
	m.HandleFunc("POST /api/approval/decide", a.wrap(approvalPage, a.handleDecide))
}

// handleDecide: the Admin's decision, made in Identity's own page.
func (a *App) handleDecide(w http.ResponseWriter, r *http.Request) {
	s, ok := a.apSess.lookup(r)
	if !ok || r.Header.Get("X-Identity-CSRF") != s.csrf {
		a.fail(w, r, 401, "approval", "approval.session_refused", "This approval page has expired or was not opened with its one-time link.")
		return
	}
	var b struct {
		Decision    string `json:"decision"` // approve | deny
		Passphrase  string `json:"passphrase"`
		Confirm     string `json:"passphrase_confirm"`
		Acknowledge bool   `json:"acknowledge"`
		// ConfirmUntilRevoked: the extra, deliberate tick for a grant with no expiry.
		ConfirmUntilRevoked bool `json:"confirm_until_revoked"`
	}
	if decode(r, &b) != nil || (b.Decision != "approve" && b.Decision != "deny") {
		a.fail(w, r, 400, "approval", "approval.bad_request", "decision must be approve or deny")
		return
	}
	ap, found := a.Approvals.Get(s.approvalID)
	if !found {
		a.fail(w, r, 404, "approval", "approval.missing", "approval not found")
		return
	}
	done := func(ap authority.Approval) {
		writeJSON(w, 200, map[string]any{"approval": ap.View(time.Now()), "display": a.approvalDisplay(ap), "admin": a.adminView()})
	}
	if b.Decision == "deny" {
		// Denying needs no passphrase: it can only make things safer.
		ap2, err := a.Approvals.Decide(ap.ID, false, "", "deny without authentication")
		if err != nil {
			a.fail(w, r, 409, "approval", "approval.decide_failed", err.Error())
			return
		}
		a.Log.Info("approval", "approval.denied", reqID(r), authority.Denied, ap.Kind+" "+ap.ID+" denied in Identity's page", nil)
		done(ap2)
		return
	}
	if ap.Kind == authority.KindEnrollAdmin {
		var act struct {
			GitHubID    int64     `json:"github_id"`
			GitHubLogin string    `json:"github_login"`
			GitHubMode  string    `json:"github_mode"`
			VerifiedAt  time.Time `json:"github_verified_at"`
		}
		json.Unmarshal(ap.Action, &act)
		gv := a.GH.View()
		switch {
		case !b.Acknowledge:
			a.fail(w, r, 400, "approval", "admin.enroll_unconfirmed", "Tick the confirmation: this establishes the Admin Identity for this installation.")
			return
		case b.Passphrase != b.Confirm:
			a.fail(w, r, 400, "approval", "admin.enroll_mismatch", "The two passphrases are different.")
			return
		case gv.Status != github.Verified || gv.Account == nil || gv.Account.ID != act.GitHubID:
			a.fail(w, r, 409, "approval", "admin.enroll_account_changed", "The GitHub account signed in now is not the one this request was made for. Ask again.")
			return
		}
		if ap.State != authority.Pending || !time.Now().Before(ap.ExpiresAt) {
			a.fail(w, r, 409, "approval", "admin.enroll_stale", "This request is no longer pending.")
			return
		}
		adm, err := a.Admin.Enroll(a.Store.Profile.SystemID, ap.ID, authority.GitHubBinding{ID: act.GitHubID, Login: act.GitHubLogin,
			Mode: act.GitHubMode, VerifiedAt: act.VerifiedAt, ProofMethod: "GitHub device-flow sign-in verified by Identity (" + act.GitHubMode + ")"}, b.Passphrase, ap.RequestedBy)
		if err != nil {
			code := 400
			if errors.Is(err, authority.ErrAdminExists) {
				code = 409
			} else if errors.Is(err, authority.ErrAdminDamaged) {
				code = 503
			}
			a.fail(w, r, code, "approval", "admin.enroll_refused", err.Error())
			return
		}
		if _, err := a.Approvals.Decide(ap.ID, true, adm.AdminID, approvalMethod); err == nil {
			if _, _, err := a.Approvals.Begin(ap.ID, ap.Kind, ap.RequestedBy); err == nil {
				a.Approvals.Finish(ap.ID, adm.AdminID, "ENROLLED")
			}
		}
		a.decide(r, "admin.enrolled", "ALLOW", "", map[string]string{"admin_id": adm.AdminID, "github_id": strconv.FormatInt(act.GitHubID, 10), "approval_id": ap.ID})
		ap, _ = a.Approvals.Get(ap.ID)
		done(ap)
		return
	}
	untilRevoked := false
	if ap.Kind == authority.KindGrant {
		var gr authority.GrantRequest
		json.Unmarshal(ap.Action, &gr)
		untilRevoked = gr.ExpiresAt == nil
		if untilRevoked && !b.ConfirmUntilRevoked {
			a.fail(w, r, 400, "approval", "grant.until_revoked_unconfirmed",
				"This grant has NO expiry. To approve it, also tick 'I understand this grant does not expire'. Otherwise deny it and ask for a grant with an expiry.")
			return
		}
	}
	adm, err := a.Admin.Authenticate(b.Passphrase)
	if err != nil {
		code := 401
		switch {
		case errors.Is(err, authority.ErrLockedOut):
			code = 429
		case errors.Is(err, authority.ErrAdminDamaged), errors.Is(err, authority.ErrThrottleDamage):
			code = 503
		case errors.Is(err, authority.ErrNoAdmin):
			code = 409
		}
		a.Log.Warn("approval", "approval.auth_failed", reqID(r), "refused", "Admin authentication failed for "+ap.ID+": "+err.Error(), nil)
		a.fail(w, r, code, "approval", "approval.auth_refused", err.Error())
		return
	}
	ap2, err := a.Approvals.Decide(ap.ID, true, adm.AdminID, approvalMethod, untilRevoked)
	if err != nil {
		a.fail(w, r, 409, "approval", "approval.decide_failed", err.Error())
		return
	}
	a.Log.Info("approval", "approval.approved", reqID(r), authority.Approved, ap.Kind+" "+ap.ID+" approved by Admin "+adm.AdminID+" ("+approvalMethod+")",
		map[string]string{"action_sha256": ap.Digest})
	done(ap2)
}

// RequestDigest is the SHA-256 a ticket binds to (exported for tests).
func RequestDigest(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

var _ = pairing.TicketKey
