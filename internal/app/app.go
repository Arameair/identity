// Package app is the one Identity application: it wires the profile,
// activity log, GitHub sign-in, feature catalog/drafts, Processor discovery,
// diagnostics and the local web UI together.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/discovery"
	"github.com/Arameair/identity/internal/entities"
	"github.com/Arameair/identity/internal/features"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/logbook"
	"github.com/Arameair/identity/internal/pairing"
	"github.com/Arameair/identity/internal/profile"
	"github.com/Arameair/identity/internal/publisher"
	"github.com/Arameair/identity/internal/redact"
)

const (
	Product   = "identity"
	Protocol  = "identity/1-alpha"
	Prototype = "alpha candidate"
	Publisher = publisher.Name
)

// NotImplemented is shown in the UI and diagnostics so nothing missing is
// mistaken for something working.
var NotImplemented = []string{
	"Feature/source grants: every authorization answer is DENIED (identities start with no access)",
	"Receiver-side enforcement in Thought Engine or other features",
	"Feature service registration with Identity (only Thought Processor pairs)",
	"Secure administrator bootstrap / per-person admin sign-in (a paired Processor acts for the operator who approved the pairing)",
	"Publisher access-request submission service (drafts stay local, NOT SUBMITTED)",
	"Persistent GitHub sign-in (the token is memory-only; restart = sign in again)",
	"Signed releases / signed publisher catalog / OS-protected token storage",
	"Process supervision: Identity stops itself on request; it never starts or stops other programs",
}

// AppChoice is one GitHub App sign-in can use (public Client ID only).
type AppChoice struct {
	ID, Source, Kind string
}

type Options struct {
	DefaultApp    AppChoice // what "use the publisher's app again" returns to
	Port          int
	DataDir       string
	GitHub        github.Config
	ProcessorAddr string // optional preset from -processor flag
	Version       string
	Branch        string // from -ldflags, else "UNKNOWN"
	WatchEvery    time.Duration
	// ManagedBy is set when another program (normally Thought Processor)
	// started Identity and owns its lifecycle (flag -managed-by). Empty =
	// started by a person or another tool (EXTERNAL).
	ManagedBy string
	// Announce shows lines in Identity's own console window (pairing codes).
	Announce func(lines []string)
}

type BuildInfo struct {
	Version   string `json:"version"`
	Branch    string `json:"branch"`
	Revision  string `json:"revision"`
	Modified  string `json:"modified"`
	VCSTime   string `json:"vcs_time"`
	GoVersion string `json:"go_version"`
}

func readBuild(version, branch string) BuildInfo {
	b := BuildInfo{Version: version, Branch: branch, Revision: "UNKNOWN", Modified: "UNKNOWN", VCSTime: "UNKNOWN", GoVersion: runtime.Version()}
	if b.Branch == "" {
		b.Branch = "UNKNOWN"
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				b.Revision = s.Value
			case "vcs.modified":
				b.Modified = s.Value
			case "vcs.time":
				b.VCSTime = s.Value
			}
		}
	}
	return b
}

type App struct {
	opt      Options
	Store    *profile.Store
	Log      *logbook.Book
	GH       *github.Client
	Feats    *features.Store
	Pair     *pairing.Store
	Ents     *entities.Store
	pairGap  pairThrottle
	access   accessCache
	Watch    *discovery.Watcher
	Sess     *Sessions
	Build    BuildInfo
	RunID    string
	Started  time.Time
	Port     int
	shutdown chan struct{}
	once     sync.Once
	stopWhy  string
}

// New opens the profile (taking its lock) and the activity log.
func New(opt Options) (*App, error) {
	st, err := profile.Open(opt.DataDir)
	if err != nil {
		return nil, err
	}
	a := &App{opt: opt, Store: st, RunID: "run_" + profile.RandHex(8), Started: time.Now(), Port: opt.Port,
		Build: readBuild(opt.Version, opt.Branch), shutdown: make(chan struct{})}
	a.Log, err = logbook.Open(st.Root+string(os.PathSeparator)+"logs", a.RunID)
	if err != nil {
		st.Close()
		return nil, err
	}
	a.Sess = NewSessions()
	redact.Register("operator_key", a.Sess.Key())

	a.GH = github.New(opt.GitHub, features.Repos(), a.onGitHubEvent)
	a.Feats = features.Open(st.Root)
	a.Pair = pairing.Open(st.Root)
	a.Ents = entities.Open(st.Root)

	addr := st.Settings().ProcessorAddress
	if opt.ProcessorAddr != "" {
		if p, err := discovery.ParseAddress(opt.ProcessorAddr, opt.Port); err == nil {
			addr = p
		}
	}
	a.Watch = discovery.NewWatcher(addr, a.onProcessorChange)
	return a, nil
}

func (a *App) onGitHubEvent(level, event, result, detail string) {
	a.Log.Add(level, "github", event, "", result, detail, map[string]string{"mode": a.opt.GitHub.Mode()})
	if event == "github.verified" {
		v := a.GH.View()
		if v.Account != nil {
			acct := v.Account
			if err := a.Store.UpdateSettings(func(s *profile.Settings) {
				s.LastAccount = &profile.VerifiedAccount{Login: acct.Login, ID: acct.ID, Name: acct.Name, ProfileURL: acct.ProfileURL, VerifiedAt: acct.VerifiedAt, Mode: v.Mode}
			}); err != nil {
				a.Log.Error("profile", "settings.save_failed", "", "error", err.Error(), nil)
			}
		}
	}
}

func (a *App) onProcessorChange(prev, cur discovery.Result) {
	level := "info"
	if prev.State == discovery.Detected && cur.State != discovery.Detected {
		level = "warn"
	}
	if cur.State == discovery.WrongService {
		level = "warn"
	}
	from := prev.State
	if from == "" {
		from = "unknown"
	}
	a.Log.Add(level, "processor", "processor.state_changed", "", cur.State, cur.Note, map[string]string{
		"address": cur.Address, "from": from, "claimed_app": cur.ClaimedApp, "claimed_version": cur.ClaimedVersion,
		"claimed_protocol": cur.ClaimedProtocol, "elapsed_ms": strconv.FormatInt(cur.ElapsedMS, 10)})
}

// Run serves until ctx is cancelled or the UI asks to stop. It returns after
// a clean shutdown. ln may be nil, in which case Run listens on 127.0.0.1:Port.
func (a *App) Run(ctx context.Context, ln net.Listener, ready func(link string)) error {
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(a.opt.Port)))
		if err != nil {
			a.Log.Error("lifecycle", "identity.listen_failed", "", "error", fmt.Sprintf("port %d: %v", a.opt.Port, err), nil)
			return fmt.Errorf("cannot listen on 127.0.0.1:%d — is another program (or another Identity) using it? Choose another with -port. (%v)", a.opt.Port, err)
		}
	}
	a.Port = ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}

	link := fmt.Sprintf("http://127.0.0.1:%d/#key=%s", a.Port, a.Sess.Key())
	if err := a.Store.SaveOpenLink(link); err != nil {
		a.Log.Warn("lifecycle", "identity.open_link_not_saved", "", "warn", err.Error(), nil)
	}
	v := a.GH.View()
	a.Log.Info("lifecycle", "identity.started", "", "running", "Identity is serving on loopback only", map[string]string{
		"pid": strconv.Itoa(os.Getpid()), "port": strconv.Itoa(a.Port), "profile": profile.DisplayPath(a.Store.Root),
		"system_id": a.Store.Profile.SystemID, "profile_created": strconv.FormatBool(a.Store.Created),
		"version": a.Build.Version, "revision": a.Build.Revision, "branch": a.Build.Branch,
		"github_mode": v.Mode, "github_status": v.Status, "client_id_source": orDash(v.ClientIDSource),
		"processor_address": orDash(a.Watch.Address()), "log_loaded": strconv.Itoa(a.Log.Stats().LoadedFromDisk),
		"log_unreadable_lines": strconv.Itoa(a.Log.Stats().UnreadableLines)})
	if e := a.Feats.LoadError(); e != "" {
		a.Log.Error("features", "drafts.load_failed", "", "disabled", e, nil)
	}
	if e := a.Ents.LoadError(); e != "" {
		a.Log.Error("identities", "identities.load_failed", "", "disabled", e, nil)
	}
	if err := a.Store.SaveService(a.serviceRecord()); err != nil {
		a.Log.Warn("lifecycle", "identity.service_file_not_saved", "", "warn", err.Error(), nil)
	}

	wctx, wcancel := context.WithCancel(ctx)
	defer wcancel()
	every := a.opt.WatchEvery
	if every <= 0 {
		every = 10 * time.Second
	}
	go a.Watch.Run(wctx, every)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	if ready != nil {
		ready(link)
	}
	var reason string
	select {
	case <-ctx.Done():
		reason = "signal from the console (Ctrl+C / window closed / terminate)"
	case <-a.shutdown:
		reason = a.stopWhy
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			a.Log.Error("lifecycle", "identity.server_failed", "", "error", err.Error(), nil)
			a.Store.Close()
			return err
		}
	}
	a.Log.Info("lifecycle", "identity.stopping", "", "stopping", reason+"; only Identity stops — other programs are not touched", map[string]string{"uptime_s": strconv.Itoa(int(time.Since(a.Started).Seconds()))})
	a.GH.Disconnect()
	sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.Shutdown(sctx)
	a.Store.Close()
	return nil
}

// RequestShutdown asks Run to stop (used by the UI's Stop Identity button).
func (a *App) RequestShutdown() {
	a.requestShutdownBecause("Stop Identity pressed in the troubleshooting page")
}

func (a *App) requestShutdownBecause(why string) {
	a.once.Do(func() { a.stopWhy = why; close(a.shutdown) })
}

// serviceRecord is written to runtime/service.json so Thought Processor can
// find Identity whichever starts first. A locator only — never trust.
func (a *App) serviceRecord() map[string]any {
	mb := a.opt.ManagedBy
	if mb == "" {
		mb = "external"
	}
	return map[string]any{"app": Product, "api": APIVersion, "protocol": Protocol, "version": a.Build.Version,
		"base_url": "http://127.0.0.1:" + strconv.Itoa(a.Port), "port": a.Port, "pid": os.Getpid(), "run_id": a.RunID,
		"started_at": a.Started.UTC(), "managed_by": mb,
		"note": "locator only — not proof of identity. Verify the pairing proof (X-Identity-Proof) before trusting answers."}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
