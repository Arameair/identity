// Identity — first new-administrator prototype.
//
// Identity is the first program a new administrator opens. This prototype
// walks a clean TEST profile through: create (or restore) the administrator
// and installation, check for Thought Controller, ask once for permission to
// start one pinned controller build automatically, start it through a
// clearly labeled SIMULATED launcher, show GitHub status honestly, and open
// the simulated Thought Controller dashboard.
//
// It never launches a process, never contacts GitHub or any other network
// service, never stores credentials and never calls an AI model. Its only
// outbound requests go to a Thought Controller address on this computer
// (loopback only) that the owner can see and change.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const appVersion = "identity-first-admin-v0.1"

// Config is everything the app knows about how it was started.
type Config struct {
	DataDir     string `json:"data_dir"`
	DataSource  string `json:"data_source"`
	Branch      string `json:"branch"`
	BranchFrom  string `json:"branch_source"`
	Detached    bool   `json:"detached"`
	Commit      string `json:"commit"`
	Port        string `json:"port"`
	Version     string `json:"version"`
	CheckoutDir string `json:"checkout_dir,omitempty"`
}

// resolveConfig works out the data folder and build identity. Order: -data
// flag, IDENTITY_FIRST_ADMIN_DATA, Project Controller's PC_DATA, then a
// per-branch test folder. The names deliberately differ from the local V1
// experiment (IDENTITY_DATA, ~/.identity-test, $PC_DATA/identity) so this
// prototype can never open or change that experiment's identities.
func resolveConfig(flagData string, env func(string) string, git func(...string) (string, error), home string) (Config, error) {
	c := Config{Version: appVersion, Port: firstSet(env("PORT"), "7796"), Commit: "unknown"}
	if top, err := git("rev-parse", "--show-toplevel"); err == nil {
		c.CheckoutDir = top
	}
	if sha, err := git("rev-parse", "HEAD"); err == nil {
		c.Commit = sha
	}
	head, _ := git("rev-parse", "--abbrev-ref", "HEAD")
	c.Detached = head == "HEAD"
	switch {
	case env("PC_BRANCH") != "":
		c.Branch, c.BranchFrom = env("PC_BRANCH"), "PC_BRANCH"
	case head != "" && head != "HEAD":
		c.Branch, c.BranchFrom = head, "git"
	default:
		c.Branch, c.BranchFrom = "unknown", "unknown"
	}
	switch {
	case flagData != "":
		c.DataDir, c.DataSource = flagData, "-data flag"
	case env("IDENTITY_FIRST_ADMIN_DATA") != "":
		c.DataDir, c.DataSource = env("IDENTITY_FIRST_ADMIN_DATA"), "IDENTITY_FIRST_ADMIN_DATA"
	case env("PC_DATA") != "":
		c.DataDir, c.DataSource = filepath.Join(env("PC_DATA"), "identity-first-admin"), "PC_DATA"
	default:
		key := c.Branch
		if key == "unknown" {
			key = "commit-" + short(c.Commit)
		}
		c.DataDir, c.DataSource = filepath.Join(home, ".identity-first-admin-test", safeName(key)), "standalone default (per branch)"
	}
	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return c, err
	}
	c.DataDir = abs
	if c.CheckoutDir != "" && within(abs, c.CheckoutDir) {
		return c, fmt.Errorf("data folder %s is inside the code checkout %s — identity data must live outside the repository; set -data, IDENTITY_FIRST_ADMIN_DATA or PC_DATA", abs, c.CheckoutDir)
	}
	return c, nil
}

func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func gitCmd(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// ---------- events (ported from V1 main.go, one log per profile) ----------

type Event struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Detail string    `json:"detail"`
	ID     string    `json:"id,omitempty"`
}

type EventLog struct {
	mu   sync.Mutex
	list []Event
	f    *os.File
	err  string
	tag  string
}

func openEventLog(path, tag string) *EventLog {
	l := &EventLog{tag: tag}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		l.err = "event log not writable: " + err.Error()
	} else {
		l.f = f
	}
	return l
}

// Add records an event in memory, in events.jsonl and on stdout (the Project
// Controller log). Display names are kept out of log lines.
func (l *EventLog) Add(kind, detail, id string) {
	e := Event{At: time.Now().UTC(), Kind: kind, Detail: detail, ID: id}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, e)
	if len(l.list) > 200 {
		l.list = l.list[len(l.list)-200:]
	}
	if l.f != nil {
		b, _ := json.Marshal(e)
		if _, err := l.f.Write(append(b, '\n')); err != nil {
			l.err = "event log write failed: " + err.Error()
		}
	}
	idPart := ""
	if id != "" {
		idPart = " id=" + id
	}
	fmt.Printf("%s  [%s] %-10s %s%s\n", time.Now().Format("15:04:05"), l.tag, kind, detail, idPart)
}

func (l *EventLog) Recent(n int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.list) > n {
		return append([]Event{}, l.list[len(l.list)-n:]...)
	}
	return append([]Event{}, l.list...)
}

func (l *EventLog) Err() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

func (l *EventLog) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		l.f.Close()
		l.f = nil
	}
}

// ---------- main ----------

func main() {
	dataFlag := flag.String("data", "", "data folder (overrides IDENTITY_FIRST_ADMIN_DATA and PC_DATA)")
	flag.Parse()
	home, _ := os.UserHomeDir()
	cfg, err := resolveConfig(*dataFlag, os.Getenv, gitCmd, home)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if err := run(cfg, nil, nil); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run serves until SIGINT/SIGTERM (or until stop is closed, in tests).
func run(cfg Config, stop <-chan struct{}, ready chan<- string) error {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("can't create data folder %s: %w", cfg.DataDir, err)
	}
	lock, err := acquireLock(cfg.DataDir, cfg.Port)
	if err != nil {
		return err
	}
	defer lock.release()
	ln, err := net.Listen("tcp", "127.0.0.1:"+cfg.Port)
	if err != nil {
		return fmt.Errorf("can't listen on 127.0.0.1:%s: %w", cfg.Port, err)
	}
	cfg.Port = fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
	app, err := openApp(cfg)
	if err != nil {
		ln.Close()
		return err
	}
	defer app.Close()
	app.startupCheck()

	s := &server{app: app}
	srv := &http.Server{Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	if ready != nil {
		ready <- "127.0.0.1:" + cfg.Port
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	reason := ""
	select {
	case v := <-sig:
		reason = "signal " + v.String()
	case <-stop:
		reason = "stop requested"
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			reason = "server error: " + err.Error()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	app.log("stop", "Identity stopped cleanly ("+reason+"); all saves were already on disk; it never started any process, so none are left behind", "")
	return nil
}

func firstSet(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func short(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}

func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 || b.String() == "." || b.String() == ".." {
		return "default"
	}
	return b.String()
}
