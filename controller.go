package main

// Controller discovery, startup consent and the SIMULATED launcher.
//
// The simulated launcher never starts a process. It sends one start request
// (with a fresh request_id) to a Thought Controller *host that is already
// running on this computer*, which then opens the controller service for this
// installation using the IDs Identity issued. If no host is running, the
// launcher says so and stops: there is no real installer or launcher yet.
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const tcProtocol = "tc-sim-handshake/0.1"

// suggestedTarget is the Thought Controller build this prototype's handoff
// suggests pinning. It is only used if the owner approves it on screen.
var suggestedTarget = Target{
	Repo:   "Arameair/Thought-Controller",
	Branch: "experiment/first-user-dashboard-20261023",
	Commit: "a6b9dfa668bc0c8fd34af65832964abfae697125",
	Label:  "TEST / PINNED LOCAL ARTIFACT (no published approved release exists)",
}

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ---------- talking to the host ----------

type Health struct {
	App            string `json:"app"`
	Protocol       string `json:"protocol"`
	Simulated      bool   `json:"simulated"`
	Authentication string `json:"authentication"`
	Mode           string `json:"mode"`
	Build          struct {
		ReferenceRepo string `json:"reference_repo"`
		Branch        string `json:"branch"`
		Commit        string `json:"commit"`
		Version       string `json:"version"`
		Provenance    string `json:"provenance"`
	} `json:"build"`
	Host struct {
		PID     int    `json:"pid"`
		Port    string `json:"port"`
		DataDir string `json:"data_dir"`
	} `json:"host"`
	Service struct {
		State          string `json:"state"`
		ServiceID      string `json:"service_id"`
		InstallationID string `json:"installation_id"`
		AdminID        string `json:"admin_id"`
		StartRequestID string `json:"start_request_id"`
		Entities       int    `json:"entities"`
	} `json:"service"`
	Worker struct {
		State  string `json:"state"`
		Reason string `json:"reason"`
	} `json:"worker"`
}

var tcClient = &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// validAddress: loopback host:port only. Identity never calls anything that
// isn't on this computer.
func validAddress(addr string) (string, error) {
	addr = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://"))
	addr = strings.TrimSuffix(addr, "/")
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("%w: use host:port, e.g. 127.0.0.1:7797", ErrInvalid)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return "", fmt.Errorf("%w: only addresses on this computer (127.0.0.1 / localhost) are allowed", ErrInvalid)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("%w: port must be 1–65535", ErrInvalid)
	}
	return net.JoinHostPort(host, port), nil
}

func probeHealth(addr string) (*Health, error) {
	res, err := tcClient.Get("http://" + addr + "/api/health")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var h Health
	if res.StatusCode != 200 || json.Unmarshal(b, &h) != nil {
		return &Health{}, nil // something answered, but not a Thought Controller health reply
	}
	return &h, nil
}

type StartRequest struct {
	RequestID      string      `json:"request_id"`
	InstallationID string      `json:"installation_id"`
	ServiceID      string      `json:"service_id"`
	AdminID        string      `json:"admin_id"`
	ApprovedCommit string      `json:"approved_commit"`
	Launcher       string      `json:"launcher"`
	ConsentID      string      `json:"consent_id"`
	Entities       []EntityRef `json:"entities"`
}

type EntityRef struct {
	EntityID    string `json:"entity_id"`
	EntityType  string `json:"entity_type"`
	DisplayName string `json:"display_name"`
}

type StartReply struct {
	Result        string            `json:"result"`
	Reason        string            `json:"reason"`
	Error         string            `json:"error"`
	Simulated     bool              `json:"simulated"`
	Registrations map[string]string `json:"registrations"`
}

func postJSON(addr, path string, body any) (*http.Response, []byte, error) {
	b, _ := json.Marshal(body)
	res, err := tcClient.Post("http://"+addr+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	return res, raw, nil
}

func requestStart(addr string, req StartRequest) (StartReply, int, error) {
	res, raw, err := postJSON(addr, "/api/sim/lifecycle/start", req)
	if err != nil {
		return StartReply{}, 0, err
	}
	var r StartReply
	json.Unmarshal(raw, &r)
	return r, res.StatusCode, nil
}

func requestRegister(addr string, body any) error {
	res, raw, err := postJSON(addr, "/api/sim/register", body)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, trimTo(string(raw), 200))
	}
	return nil
}

func requestStop(addr, installationID string) error {
	res, raw, err := postJSON(addr, "/api/sim/lifecycle/stop", map[string]string{"installation_id": installationID, "reason": "administrator pressed Stop in Identity"})
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		var e struct {
			Error string `json:"error"`
		}
		json.Unmarshal(raw, &e)
		return errors.New(firstSet(e.Error, fmt.Sprintf("HTTP %d", res.StatusCode)))
	}
	return nil
}

type ScanHit struct {
	Address      string `json:"address"`
	Branch       string `json:"branch"`
	Commit       string `json:"commit"`
	ServiceState string `json:"service_state"`
	BoundTo      string `json:"bound_installation_id,omitempty"`
}

// scanLocal looks for Thought Controller hosts on this computer's usual
// Project Controller / standalone ports. Read-only GET /api/health only.
func scanLocal(selfPort string) []ScanHit {
	var mu sync.Mutex
	var hits []ScanHit
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)
	quick := &http.Client{Timeout: 400 * time.Millisecond}
	for p := 7778; p <= 7899; p++ {
		port := strconv.Itoa(p)
		if port == selfPort {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res, err := quick.Get("http://127.0.0.1:" + port + "/api/health")
			if err != nil {
				return
			}
			defer res.Body.Close()
			var h Health
			if json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&h) != nil || h.App != "thought-controller" {
				return
			}
			mu.Lock()
			hits = append(hits, ScanHit{Address: "127.0.0.1:" + port, Branch: h.Build.Branch, Commit: h.Build.Commit, ServiceState: h.Service.State, BoundTo: h.Service.InstallationID})
			mu.Unlock()
		}()
	}
	wg.Wait()
	return hits
}

// ---------- classification ----------

type CheckResult struct {
	State      string    `json:"state"` // NOT_CONFIGURED | NOT_FOUND | INCOMPATIBLE | VERSION_MISMATCH | WRONG_INSTANCE | NOT_RUNNING | VERIFIED
	Label      string    `json:"label"`
	Detail     string    `json:"detail"`
	Address    string    `json:"address"`
	CheckedAt  time.Time `json:"checked_at"`
	ElapsedMS  int64     `json:"elapsed_ms"`
	Trigger    string    `json:"trigger"`
	Health     *Health   `json:"health,omitempty"`
	Attempt    *Attempt  `json:"attempt,omitempty"`
	ConsentMsg string    `json:"consent_note,omitempty"`
	CanOpen    bool      `json:"can_open"`
	OpenURL    string    `json:"open_url,omitempty"`
}

func classify(addr string, h *Health, probeErr error, inst *Installation, target *Target) CheckResult {
	r := CheckResult{Address: addr, Health: h}
	switch {
	case addr == "":
		r.State, r.Label, r.Detail = "NOT_CONFIGURED", "No controller address", "Enter the address Thought Controller runs on (Project Controller shows its port)."
	case probeErr != nil:
		r.State, r.Label = "NOT_FOUND", "NOT FOUND"
		r.Detail = "Nothing answered at " + addr + ". Thought Controller is not running there."
		r.Health = nil
	case h.App != "thought-controller":
		r.State, r.Label, r.Detail = "INCOMPATIBLE", "INCOMPATIBLE — not a Thought Controller", "Something answered at "+addr+", but it is not a Thought Controller. It is not trusted just because it answered."
	case h.Protocol != tcProtocol:
		r.State, r.Label, r.Detail = "INCOMPATIBLE", "INCOMPATIBLE protocol", fmt.Sprintf("This Thought Controller speaks %q; this Identity understands %q.", h.Protocol, tcProtocol)
	case h.Service.State == "ready" && (h.Service.InstallationID != inst.InstallationID || h.Service.ServiceID != inst.ControllerServiceID || h.Service.AdminID != inst.AdminID):
		r.State, r.Label = "WRONG_INSTANCE", "WRONG INSTANCE — belongs to another installation"
		r.Detail = "A controller service is running at " + addr + ", but it is bound to installation " + h.Service.InstallationID + " (service " + h.Service.ServiceID + "), not this one. Identity will not use or take it over. Stop it from its own profile or from the Thought Controller page."
	case target != nil && h.Build.Commit != target.Commit:
		r.State, r.Label = "VERSION_MISMATCH", "VERSION MISMATCH — not the approved build"
		r.Detail = fmt.Sprintf("The Thought Controller at %s runs commit %s (%s); the approved target is %s. Identity won't start or use it. Pin this build only if you choose to (Change target).", addr, h.Build.Commit, h.Build.Branch, target.Commit)
	case h.Service.State == "ready":
		r.State, r.Label, r.CanOpen = "VERIFIED", "RUNNING · VERIFIED (simulated handshake)", true
		r.Detail = "Bound to this installation and service ID; build matches the pinned target. Verification is a SIMULATED, unauthenticated handshake."
		r.OpenURL = "http://" + addr + "/?installation=" + inst.InstallationID
	case target == nil:
		r.State, r.Label = "NOT_RUNNING", "NOT RUNNING for this installation"
		r.Detail = "A Thought Controller host answered at " + addr + " (build " + h.Build.Commit + "), but no controller service is started for this installation. No build is pinned yet."
		if shaRe.MatchString(suggestedTarget.Commit) && h.Build.Commit != suggestedTarget.Commit {
			r.Detail += " Note: this host does NOT run the suggested build, so approving the suggestion would not start it."
		}
	default:
		r.State, r.Label = "NOT_RUNNING", "NOT RUNNING for this installation"
		r.Detail = "A Thought Controller host answered at " + addr + " (the pinned build), but no controller service is started for this installation."
	}
	return r
}

func (c Consent) validFor(t *Target) bool {
	return c.Status == "approved" && t != nil && c.TargetFingerprint == t.fingerprint()
}

// ---------- check (+ automatic start under saved consent) ----------

var checkMu sync.Mutex // one check/launch at a time: no duplicate start requests

// Check probes the controller and, when allowStart is true and consent B is
// valid for the pinned target, sends exactly ONE start request.
func (a *App) Check(trigger string, allowStart bool) (CheckResult, error) {
	checkMu.Lock()
	defer checkMu.Unlock()
	a.mu.Lock()
	p := a.p
	if p.Inst == nil {
		a.mu.Unlock()
		return CheckResult{}, fmt.Errorf("%w: create the installation first", ErrInvalid)
	}
	inst := *p.Inst
	cf := p.Controller
	managed := p.Store.ByRole("managed")
	a.mu.Unlock()

	t0 := time.Now()
	h, err := a.probe(cf.Address)
	res := classify(cf.Address, h, err, &inst, cf.Target)
	valid := cf.Consent.validFor(cf.Target)
	switch {
	case cf.Consent.Status == "denied":
		res.ConsentMsg = "Automatic startup was declined. Identity works without it."
	case cf.Consent.Status == "revoked":
		res.ConsentMsg = "Automatic startup was revoked."
	case cf.Target == nil:
		res.ConsentMsg = "No target build is pinned. Approve the suggested test build or pin one yourself."
	case cf.Consent.Status == "approved" && !valid:
		res.ConsentMsg = "Saved approval was for a different build — the target changed, so approval is needed again."
	case valid:
		res.ConsentMsg = "Automatic startup approved for this exact build."
	case cf.Consent.Status == "denied":
		res.ConsentMsg = "Automatic startup was declined. Identity works without it."
	case cf.Consent.Status == "revoked":
		res.ConsentMsg = "Automatic startup was revoked."
	default:
		res.ConsentMsg = "Automatic startup hasn't been decided yet."
	}
	var att *Attempt
	if allowStart && valid && res.State == "NOT_RUNNING" {
		att = &Attempt{At: time.Now().UTC(), Trigger: trigger}
		rid, _ := newUUID()
		att.RequestID = "start-" + rid
		refs := make([]EntityRef, 0, len(managed))
		for _, m := range managed {
			refs = append(refs, EntityRef{EntityID: m.ID, EntityType: m.EntityType, DisplayName: m.DisplayName})
		}
		ls := time.Now()
		reply, code, serr := a.start(cf.Address, StartRequest{RequestID: att.RequestID, InstallationID: inst.InstallationID,
			ServiceID: inst.ControllerServiceID, AdminID: inst.AdminID, ApprovedCommit: cf.Target.Commit,
			Launcher:  launcherID + " (SIMULATED — asks an already-running host; never launches a process)",
			ConsentID: cf.Consent.ConsentID, Entities: refs})
		att.ElapsedMS = time.Since(ls).Milliseconds()
		switch {
		case serr != nil:
			att.Result, att.Reason = "error", "start request failed: "+serr.Error()
		case code == 200:
			att.Result, att.Reason = reply.Result, reply.Reason
		default:
			att.Result, att.Reason = firstSet(reply.Result, "error"), firstSet(reply.Reason, reply.Error, fmt.Sprintf("HTTP %d", code))
		}
		h, err = a.probe(cf.Address)
		res = classify(cf.Address, h, err, &inst, cf.Target)
		res.ConsentMsg = "Automatic startup approved for this exact build — used now without asking again."
		res.Attempt = att
	} else if valid && res.State == "NOT_FOUND" {
		res.ConsentMsg = "Startup is approved, but the simulated launcher can only ask a Thought Controller host that is already running. Play Thought Controller (Project Controller or standalone), then Check again. A real launcher isn't built yet."
	}
	res.CheckedAt, res.ElapsedMS, res.Trigger = time.Now().UTC(), time.Since(t0).Milliseconds(), trigger

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.p != p { // profile switched while checking: don't write into the wrong profile
		return res, nil
	}
	next := p.Controller
	next.Counters.Checks++
	if att != nil {
		next.Counters.StartRequests++
		switch att.Result {
		case "started":
			next.Counters.Started++
		case "already_running", "duplicate_request":
			next.Counters.Reused++
		case "refused":
			next.Counters.Refused++
		default:
			next.Counters.Errors++
		}
		next.Attempts = append(append([]Attempt(nil), next.Attempts...), *att)
		if len(next.Attempts) > 50 {
			next.Attempts = next.Attempts[len(next.Attempts)-50:]
		}
		p.Events.Add("launch", fmt.Sprintf("SIMULATED start request %s (%s): %s — %s (%d ms)", att.RequestID, trigger, att.Result, att.Reason, att.ElapsedMS), inst.InstallationID)
	}
	if err := writeJSONAtomic(filepath.Join(p.Dir, "controller.json"), next); err != nil {
		p.Events.Add("error", "controller state NOT saved: "+err.Error(), "")
	}
	p.Controller = next
	p.LastCheck = &res
	p.Events.Add("check", fmt.Sprintf("controller check (%s) at %s: %s (%d ms)", trigger, firstSet(cf.Address, "(no address)"), res.State, res.ElapsedMS), "")
	if res.State == "VERIFIED" && len(managed) > 0 && h != nil && h.Service.Entities < len(managed) {
		go a.registerNew(managed) // identities created while the controller was down
	}
	return res, nil
}

func (a *App) startupCheck() {
	a.mu.Lock()
	has := a.p.Inst != nil
	a.mu.Unlock()
	if has {
		go a.Check("Identity startup", true)
	}
}

// registerNew tells a VERIFIED controller about new managed identities.
func (a *App) registerNew(ids []Identity) {
	if len(ids) == 0 {
		return
	}
	a.mu.Lock()
	p := a.p
	if p.Inst == nil || p.LastCheck == nil || p.LastCheck.State != "VERIFIED" {
		a.mu.Unlock()
		return // registered with the full list on the next controller start
	}
	addr, inst := p.Controller.Address, *p.Inst
	a.mu.Unlock()
	refs := make([]EntityRef, 0, len(ids))
	for _, m := range ids {
		refs = append(refs, EntityRef{EntityID: m.ID, EntityType: m.EntityType, DisplayName: m.DisplayName})
	}
	err := a.reg(addr, map[string]any{"installation_id": inst.InstallationID, "service_id": inst.ControllerServiceID, "entities": refs})
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.p == p {
		if err != nil {
			p.Events.Add("error", "SIMULATED registration with the controller failed: "+err.Error()+" (will be sent again on the next controller start)", "")
		} else {
			p.Events.Add("register", fmt.Sprintf("SIMULATED registration of %d identit%s with the running controller", len(refs), map[bool]string{true: "y", false: "ies"}[len(refs) == 1]), "")
		}
	}
}

// ---------- owner decisions ----------

func (a *App) saveController(next ControllerFile) error {
	if err := writeJSONAtomic(filepath.Join(a.p.Dir, "controller.json"), next); err != nil {
		return fmt.Errorf("NOT saved: %w", err)
	}
	a.p.Controller = next
	return nil
}

// Decide records approve / deny / revoke. Approving with no target pinned
// pins the on-screen suggested test build — and only that.
func (a *App) Decide(decision string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.p
	if p.Inst == nil {
		return fmt.Errorf("%w: create the installation first", ErrInvalid)
	}
	next := p.Controller
	now := a.now().UTC()
	switch decision {
	case "approve":
		if next.Target == nil {
			if !shaRe.MatchString(suggestedTarget.Commit) {
				return fmt.Errorf("%w: no target build is pinned and this prototype has no suggested build — pin one first (Change target)", ErrInvalid)
			}
			t := suggestedTarget
			t.ChosenAt, t.ChosenBy = now, "owner approved the prototype's suggested test build"
			next.Target = &t
		}
		cid, _ := newUUID()
		next.Consent = Consent{Status: "approved", ConsentID: cid, TargetFingerprint: next.Target.fingerprint(), TargetCommit: next.Target.Commit,
			Launcher: launcherID, DecidedAt: &now,
			Scope: "Request startup of THIS controller build only, through the simulated launcher, when it isn't running. Does not approve model calls, GitHub access, code updates, messages or any other program."}
	case "deny":
		next.Consent = Consent{Status: "denied", DecidedAt: &now}
	case "revoke":
		if next.Consent.Status != "approved" {
			return fmt.Errorf("%w: there is no approval to revoke", ErrInvalid)
		}
		next.Consent = Consent{Status: "revoked", DecidedAt: &now, TargetCommit: next.Consent.TargetCommit}
	default:
		return fmt.Errorf("%w: decision must be approve, deny or revoke", ErrInvalid)
	}
	if err := a.saveController(next); err != nil {
		return err
	}
	p.Events.Add("consent", "automatic controller startup "+map[string]string{"approve": "APPROVED for commit " + next.Consent.TargetCommit + " (consent " + next.Consent.ConsentID + ")", "deny": "DECLINED — nothing will be started", "revoke": "REVOKED — nothing will be started until approved again"}[decision], p.Inst.InstallationID)
	return nil
}

func (a *App) SetAddress(addr string) error {
	addr, err := validAddress(addr)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.p.Controller
	next.Address = addr
	if err := a.saveController(next); err != nil {
		return err
	}
	a.p.LastCheck = nil
	a.p.Events.Add("controller", "controller address set to "+addr, "")
	return nil
}

// SetTarget pins a build. useRunning pins whatever answers at the address —
// an explicit owner choice, shown before it is made.
func (a *App) SetTarget(branch, commit string, useRunning bool) (*Target, error) {
	a.mu.Lock()
	addr := a.p.Controller.Address
	a.mu.Unlock()
	if useRunning {
		h, err := a.probe(addr)
		if err != nil || h.App != "thought-controller" {
			return nil, fmt.Errorf("%w: no Thought Controller answered at %s", ErrInvalid, addr)
		}
		branch, commit = h.Build.Branch, h.Build.Commit
	}
	commit = strings.ToLower(strings.TrimSpace(commit))
	if !shaRe.MatchString(commit) {
		return nil, fmt.Errorf("%w: commit must be a full 40-character SHA", ErrInvalid)
	}
	branch = trimTo(branch, 120)
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.p.Controller
	t := Target{Repo: suggestedTarget.Repo, Branch: firstSet(branch, "(not given)"), Commit: commit, Label: suggestedTarget.Label,
		ChosenAt: a.now().UTC(), ChosenBy: map[bool]string{true: "owner pinned the build answering at " + addr, false: "owner entered the build"}[useRunning]}
	next.Target = &t
	if err := a.saveController(next); err != nil {
		return nil, err
	}
	msg := "pinned target build " + commit + " (" + t.Branch + ")"
	if next.Consent.Status == "approved" && !next.Consent.validFor(&t) {
		msg += " — saved approval was for another build, so automatic startup needs approval again"
	}
	a.p.Events.Add("target", msg, "")
	return &t, nil
}

func (a *App) StopController() error {
	a.mu.Lock()
	p := a.p
	if p.Inst == nil {
		a.mu.Unlock()
		return fmt.Errorf("%w: no installation", ErrInvalid)
	}
	addr, iid := p.Controller.Address, p.Inst.InstallationID
	a.mu.Unlock()
	err := a.stop(addr, iid)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		p.Events.Add("error", "SIMULATED stop request failed: "+err.Error(), "")
		return err
	}
	p.LastCheck = nil
	p.Events.Add("launch", "SIMULATED stop: asked the host to stop this installation's controller service (no process was ended)", iid)
	return nil
}
