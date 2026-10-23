package main

// Test profiles. Every profile is a complete, independent installation:
//
//	<data>/active-profile.json
//	<data>/profiles/<profile>/installation.json   installation, administrator and controller-service IDs
//	<data>/profiles/<profile>/identities/<id>.json one file per identity (ported V1 store)
//	<data>/profiles/<profile>/controller.json      address, pinned target, startup consent, launch attempts
//	<data>/profiles/<profile>/github.json          GitHub card (and the MOCK test selector)
//	<data>/profiles/<profile>/events.jsonl         what happened
//
// "Test as New User" makes a brand-new empty profile and switches to it, so
// the owner can walk the first-run experience again without touching any
// earlier profile.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var profileRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,59}$`)

// Installation is issued once per profile. The three IDs always differ.
type Installation struct {
	SchemaVersion       int        `json:"schema_version"`
	InstallationID      string     `json:"installation_id"`
	AdminID             string     `json:"admin_id"`
	ControllerServiceID string     `json:"controller_service_id"`
	CreatedAt           time.Time  `json:"created_at"`
	Profile             string     `json:"profile"`
	ReferenceRepo       string     `json:"reference_repo"`
	SetupStep           int        `json:"setup_step"`
	SetupCompletedAt    *time.Time `json:"setup_completed_at,omitempty"`
}

// Target is the one controller build the owner pinned. There is no
// published approved release, so this is always a TEST pin.
type Target struct {
	Repo     string    `json:"repo"`
	Branch   string    `json:"branch"`
	Commit   string    `json:"commit"`
	Label    string    `json:"label"`
	ChosenAt time.Time `json:"chosen_at"`
	ChosenBy string    `json:"chosen_by"`
}

const launcherID = "identity-simulated-launcher/0.1"

func (t *Target) fingerprint() string {
	if t == nil {
		return ""
	}
	h := sha256.Sum256([]byte(t.Repo + "\x00" + t.Commit + "\x00" + launcherID))
	return hex.EncodeToString(h[:8])
}

// Consent is option B: one saved permission to request startup of exactly
// this target through this launcher on later launches.
type Consent struct {
	Status            string     `json:"status"` // none | approved | denied | revoked
	ConsentID         string     `json:"consent_id,omitempty"`
	TargetFingerprint string     `json:"target_fingerprint,omitempty"`
	TargetCommit      string     `json:"target_commit,omitempty"`
	Launcher          string     `json:"launcher,omitempty"`
	Scope             string     `json:"scope,omitempty"`
	DecidedAt         *time.Time `json:"decided_at,omitempty"`
}

type Attempt struct {
	RequestID string    `json:"request_id"`
	At        time.Time `json:"at"`
	Trigger   string    `json:"trigger"`
	Result    string    `json:"result"`
	Reason    string    `json:"reason"`
	ElapsedMS int64     `json:"elapsed_ms"`
}

type ControllerCounters struct {
	Checks        int `json:"checks"`
	StartRequests int `json:"start_requests"`
	Started       int `json:"started"`
	Reused        int `json:"reused"`
	Refused       int `json:"refused"`
	Errors        int `json:"errors"`
}

type ControllerFile struct {
	SchemaVersion int                `json:"schema_version"`
	Address       string             `json:"address"`
	Target        *Target            `json:"target,omitempty"`
	Consent       Consent            `json:"consent"`
	Attempts      []Attempt          `json:"attempts"`
	Counters      ControllerCounters `json:"counters"`
}

type GitHubFile struct {
	SchemaVersion  int    `json:"schema_version"`
	SimulatedState string `json:"simulated_state"` // "" (real status) | mock_connected | mock_offline | mock_access_required
}

// Profile is the open state of one test profile.
type Profile struct {
	Name       string
	Dir        string
	Inst       *Installation
	Store      *Store
	Controller ControllerFile
	GitHub     GitHubFile
	GitHubCheck *GitHubCLIResult // runtime-only external CLI session check, never grants authorization
	Events     *EventLog
	LastCheck  *CheckResult
}

type App struct {
	mu       sync.Mutex // guards profile switching and all profile state
	cfg      Config
	p        *Profile
	started  time.Time
	probe    func(addr string) (*Health, error) // replaced in tests
	start    func(addr string, req StartRequest) (StartReply, int, error)
	reg      func(addr string, body any) error
	stop     func(addr, installationID string) error
	scan     func(selfPort string) []ScanHit
	now      func() time.Time
	failNext bool
}

func openApp(cfg Config) (*App, error) {
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "profiles"), 0o700); err != nil {
		return nil, fmt.Errorf("can't create profiles folder: %w", err)
	}
	a := &App{cfg: cfg, started: time.Now(), now: time.Now}
	a.probe, a.start, a.reg, a.stop, a.scan = probeHealth, requestStart, requestRegister, requestStop, scanLocal
	name := "test-profile-1"
	var ap struct {
		Profile string `json:"profile"`
	}
	if err := readJSONStrict(filepath.Join(cfg.DataDir, "active-profile.json"), &ap); err == nil && profileRe.MatchString(ap.Profile) {
		name = ap.Profile
	}
	if err := a.openProfileLocked(name); err != nil {
		return nil, err
	}
	a.log("start", fmt.Sprintf("Identity %s on http://127.0.0.1:%s/ (pid %d) · branch %s (%s%s) · commit %s · data %s (%s) · profile %s",
		appVersion, cfg.Port, os.Getpid(), cfg.Branch, cfg.BranchFrom, map[bool]string{true: ", detached checkout", false: ""}[cfg.Detached],
		cfg.Commit, cfg.DataDir, cfg.DataSource, name), "")
	return a, nil
}

func (a *App) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.p != nil {
		a.p.Events.Close()
	}
}

func (a *App) log(kind, detail, id string) {
	if a.p != nil {
		a.p.Events.Add(kind, detail, id)
	}
}

// openProfileLocked opens (creating if needed) a profile and makes it active.
func (a *App) openProfileLocked(name string) error {
	if !profileRe.MatchString(name) {
		return fmt.Errorf("%w: profile names use a-z, 0-9, dot, dash and underscore", ErrInvalid)
	}
	dir := filepath.Join(a.cfg.DataDir, "profiles", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("can't create profile folder: %w", err)
	}
	p := &Profile{Name: name, Dir: dir, Events: openEventLog(filepath.Join(dir, "events.jsonl"), name)}
	var inst Installation
	switch err := readJSONStrict(filepath.Join(dir, "installation.json"), &inst); {
	case err == nil:
		if !uuidRe.MatchString(inst.InstallationID) || !uuidRe.MatchString(inst.AdminID) || !uuidRe.MatchString(inst.ControllerServiceID) {
			p.Events.Close()
			return fmt.Errorf("installation.json in profile %s has invalid IDs — left untouched; use another profile", name)
		}
		p.Inst = &inst
	case errors.Is(err, os.ErrNotExist):
	default:
		p.Events.Close()
		return fmt.Errorf("installation.json in profile %s can't be read (%v) — left untouched; use another profile", name, err)
	}
	iid := ""
	if p.Inst != nil {
		iid = p.Inst.InstallationID
	}
	st, err := openStore(dir, iid)
	if err != nil {
		p.Events.Close()
		return err
	}
	p.Store = st
	p.Controller = ControllerFile{SchemaVersion: 1, Address: "127.0.0.1:7797", Consent: Consent{Status: "none"}, Attempts: []Attempt{}}
	if err := readJSONStrict(filepath.Join(dir, "controller.json"), &p.Controller); err != nil && !errors.Is(err, os.ErrNotExist) {
		p.Events.Add("problem", "controller.json can't be read ("+err.Error()+"); using defaults in memory, file left untouched", "")
	}
	p.GitHub = GitHubFile{SchemaVersion: 1}
	if err := readJSONStrict(filepath.Join(dir, "github.json"), &p.GitHub); err != nil && !errors.Is(err, os.ErrNotExist) {
		p.Events.Add("problem", "github.json can't be read ("+err.Error()+"); showing real status, file left untouched", "")
	}
	if err := writeJSONAtomic(filepath.Join(a.cfg.DataDir, "active-profile.json"), map[string]string{"profile": name}); err != nil {
		p.Events.Add("error", "active profile choice NOT saved: "+err.Error(), "")
	}
	if a.p != nil {
		a.p.Events.Add("profile", "switched away to profile "+name, "")
		a.p.Events.Close()
	}
	a.p = p
	if p.Inst == nil {
		p.Events.Add("profile", "opened profile with NO installation yet (first-run welcome)", "")
	} else {
		p.Events.Add("restore", fmt.Sprintf("restored installation %s · administrator %s · controller service %s · %d managed identit%s · startup consent %s",
			p.Inst.InstallationID, p.Inst.AdminID, p.Inst.ControllerServiceID, len(st.ByRole("managed")),
			map[bool]string{true: "y", false: "ies"}[len(st.ByRole("managed")) == 1], p.Controller.Consent.Status), p.Inst.InstallationID)
	}
	for _, pr := range st.Problems() {
		p.Events.Add("problem", "could not load "+pr.File+": "+pr.Reason+" (file left untouched)", "")
	}
	return nil
}

type ProfileInfo struct {
	Name           string     `json:"name"`
	InstallationID string     `json:"installation_id,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	Active         bool       `json:"active"`
}

func (a *App) profilesLocked() []ProfileInfo {
	var out []ProfileInfo
	entries, _ := os.ReadDir(filepath.Join(a.cfg.DataDir, "profiles"))
	for _, e := range entries {
		if !e.IsDir() || !profileRe.MatchString(e.Name()) {
			continue
		}
		pi := ProfileInfo{Name: e.Name(), Active: a.p != nil && a.p.Name == e.Name()}
		var inst Installation
		if readJSONStrict(filepath.Join(a.cfg.DataDir, "profiles", e.Name(), "installation.json"), &inst) == nil {
			pi.InstallationID = inst.InstallationID
			c := inst.CreatedAt
			pi.CreatedAt = &c
		}
		out = append(out, pi)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// NewTestProfile makes a fresh, empty profile and switches to it.
func (a *App) NewTestProfile() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	base := "new-user-" + a.now().Format("20060102-150405")
	name := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(a.cfg.DataDir, "profiles", name)); errors.Is(err, os.ErrNotExist) {
			break
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
	if a.p != nil {
		a.p.Events.Add("profile", "Test as New User: leaving this profile untouched and opening new empty profile "+name, "")
	}
	return name, a.openProfileLocked(name)
}

func (a *App) SwitchProfile(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := os.Stat(filepath.Join(a.cfg.DataDir, "profiles", name)); err != nil || !profileRe.MatchString(name) {
		return fmt.Errorf("%w: no such profile", ErrNotFound)
	}
	return a.openProfileLocked(name)
}

// CreateInstallation issues the installation, administrator and controller
// service IDs together. Repeats (same request) return the same result.
func (a *App) CreateInstallation(adminName string) (*Installation, error) {
	adminName, err := cleanName(adminName)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.p
	if p.Inst != nil {
		return p.Inst, fmt.Errorf("%w: this profile already has installation %s", ErrDuplicateName, p.Inst.InstallationID)
	}
	if a.failNext {
		a.failNext = false
		p.Events.Add("error", "installation NOT created: simulated save failure (owner test tool) — nothing was written", "")
		return nil, errors.New("save failed: simulated failure (owner test tool) — nothing was written")
	}
	ids := map[string]bool{}
	gen := func() (string, error) {
		for {
			id, err := newUUID()
			if err != nil {
				return "", err
			}
			if !ids[id] {
				ids[id] = true
				return id, nil
			}
		}
	}
	iid, err1 := gen()
	aid, err2 := gen()
	sid, err3 := gen()
	if err := errors.Join(err1, err2, err3); err != nil {
		return nil, fmt.Errorf("couldn't generate IDs: %w", err)
	}
	inst := &Installation{SchemaVersion: 1, InstallationID: iid, AdminID: aid, ControllerServiceID: sid,
		CreatedAt: a.now().UTC(), Profile: p.Name, ReferenceRepo: "Arameair/Thought-Controller", SetupStep: 2}
	// Records first, installation.json last: a profile only counts as installed
	// once every part is on disk.
	p.Store.installationID = iid
	if _, err := p.Store.createWithID(aid, adminName, "administrator", "", true); err != nil {
		p.Store.installationID = ""
		return nil, fmt.Errorf("administrator NOT saved: %w", err)
	}
	if _, err := p.Store.createWithID(sid, "Thought Controller", "system_service", "", true); err != nil {
		return nil, fmt.Errorf("controller service identity NOT saved: %w", err)
	}
	if err := writeJSONAtomic(filepath.Join(p.Dir, "installation.json"), inst); err != nil {
		return nil, fmt.Errorf("installation NOT saved: %w", err)
	}
	p.Inst = inst
	p.Events.Add("create", "created installation "+iid+" with administrator "+aid+" and Thought Controller service identity "+sid+" (three distinct IDs)", iid)
	return inst, nil
}

func (a *App) SetSetupStep(step int, complete bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.p
	if p.Inst == nil {
		return fmt.Errorf("%w: no installation yet", ErrInvalid)
	}
	next := *p.Inst
	if step >= 2 && step <= 6 {
		next.SetupStep = step
	}
	if complete && next.SetupCompletedAt == nil {
		t := a.now().UTC()
		next.SetupCompletedAt = &t
	}
	if err := writeJSONAtomic(filepath.Join(p.Dir, "installation.json"), next); err != nil {
		return fmt.Errorf("setup progress NOT saved: %w", err)
	}
	*p.Inst = next
	if complete {
		p.Events.Add("setup", "first-run setup marked complete; later starts open the overview instead of the wizard", p.Inst.InstallationID)
	}
	return nil
}

// StarterIdentities creates TASS and Example Workshop if they don't exist.
func (a *App) StarterIdentities() ([]Identity, error) {
	a.mu.Lock()
	p := a.p
	if p.Inst == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("%w: create the installation first", ErrInvalid)
	}
	var out []Identity
	for _, s := range [][2]string{{"TASS", "ai_companion"}, {"Example Workshop", "business"}} {
		res, err := p.Store.Create(s[0], s[1], "starter-"+s[1], false)
		if errors.Is(err, ErrDuplicateName) {
			continue
		}
		if err != nil {
			a.mu.Unlock()
			return out, err
		}
		if !res.Repeat {
			p.Events.Add("create", "created starter "+typeLabel(s[1]), res.Identity.ID)
			out = append(out, res.Identity)
		}
	}
	a.mu.Unlock()
	a.registerNew(out)
	return out, nil
}

func (a *App) CreateIdentity(name, typ, requestID string, allowDup bool) (CreateResult, error) {
	if roleFor(typ) != "managed" {
		return CreateResult{}, fmt.Errorf("%w: administrator and system-service identities are issued with the installation", ErrInvalid)
	}
	a.mu.Lock()
	p := a.p
	if p.Inst == nil {
		a.mu.Unlock()
		return CreateResult{}, fmt.Errorf("%w: create the installation first", ErrInvalid)
	}
	res, err := p.Store.Create(name, typ, requestID, allowDup)
	if err == nil && !res.Repeat {
		p.Events.Add("create", "created and saved "+typeLabel(typ), res.Identity.ID)
	} else if err == nil {
		p.Events.Add("repeat", "same create request received again — returned the existing identity", res.Identity.ID)
	}
	a.mu.Unlock()
	if err == nil && !res.Repeat {
		a.registerNew([]Identity{res.Identity})
	}
	return res, err
}

// ---------- GitHub Identity account state (unlinked; mock status selector) ----------
// A separate explicit github_cli.go probe may query /user via the official
// machine CLI; it never links the resulting account to this Identity card.

func (a *App) SetGitHubSimulation(state string) error {
	switch state {
	case "", "mock_connected", "mock_offline", "mock_access_required":
	default:
		return fmt.Errorf("%w: unknown GitHub test state", ErrInvalid)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	next := a.p.GitHub
	next.SimulatedState = state
	if err := writeJSONAtomic(filepath.Join(a.p.Dir, "github.json"), next); err != nil {
		return fmt.Errorf("GitHub test setting NOT saved: %w", err)
	}
	a.p.GitHub = next
	a.p.Events.Add("github", "GitHub card test setting: "+firstSet(state, "real status (NOT CONNECTED)")+" — no GitHub request was made", "")
	return nil
}

type GitHubStatus struct {
	State        string `json:"state"`
	Mock         bool   `json:"mock"`
	Summary      string `json:"summary"`
	Next         string `json:"next"`
	SourceAccess string `json:"source_access"`
	SourceDetail string `json:"source_detail"`
	CallsMade    int    `json:"calls_made"`
}

func githubStatus(g GitHubFile) GitHubStatus {
	s := GitHubStatus{
		SourceAccess: "ACCESS REQUIRED",
		SourceDetail: "Arameair/identity and Arameair/Thought-Controller are PRIVATE and have no published approved releases. A new administrator without access to them can't download a build, and this prototype downloads nothing.",
	}
	switch g.SimulatedState {
	case "mock_connected":
		s.State, s.Mock = "MOCK CONNECTED", true
		s.Summary = "Test display only. No account was checked, no token exists and no repository was touched."
		s.Next = "Switch back to Real status to see the true state."
	case "mock_offline":
		s.State, s.Mock = "MOCK OFFLINE", true
		s.Summary = "Test display only: shows how Identity keeps working locally when GitHub can't be reached."
		s.Next = "Local setup continues; nothing waits on GitHub."
	case "mock_access_required":
		s.State, s.Mock = "MOCK ACCESS REQUIRED", true
		s.Summary = "Test display only: shows the state where an account is connected but lacks access to the private source."
		s.Next = "A future build would explain how to request access; nothing is requested now."
	default:
		s.State = "NOT CONNECTED"
		s.Summary = "No GitHub account is linked to this Identity installation. Identity stores no GitHub tokens or passwords. An optional read-only machine GitHub CLI check is separate from this status."
		s.Next = "Connecting through GitHub's official sign-in isn't built yet (planned later, with your approval). You can keep going locally."
	}
	return s
}

// ---------- small helpers ----------

func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmp)
		}
	}()
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, b) {
		return fmt.Errorf("could not verify the save after writing (%v)", err)
	}
	return nil
}

func readJSONStrict(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("not a valid record: %v", err)
	}
	return nil
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
