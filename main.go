// Identity Manager, Gate 1: create, save, list/open and restore local test
// identities. No accounts, no network calls, no other components.
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

const appVersion = "identity-local-v1"

// Config is everything the app knows about how it was started.
type Config struct {
	DataDir     string `json:"data_dir"`
	DataSource  string `json:"data_source"` // flag | IDENTITY_DATA | PC_DATA | standalone default
	Branch      string `json:"branch"`
	BranchFrom  string `json:"branch_source"` // PC_BRANCH | git | unknown
	Detached    bool   `json:"detached"`
	Commit      string `json:"commit"`
	Port        string `json:"port"`
	Version     string `json:"version"`
	CheckoutDir string `json:"checkout_dir,omitempty"`
}

// resolveConfig works out the data folder and build identity. Order:
// -data flag, IDENTITY_DATA, Project Controller's PC_DATA, then a local
// test folder per branch (or per commit when detached).
func resolveConfig(flagData string, env func(string) string, git func(...string) (string, error), home string) (Config, error) {
	c := Config{Version: appVersion, Port: firstSet(env("PORT"), "7796")}
	if top, err := git("rev-parse", "--show-toplevel"); err == nil {
		c.CheckoutDir = top
	}
	c.Commit = "unknown"
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
	case env("IDENTITY_DATA") != "":
		c.DataDir, c.DataSource = env("IDENTITY_DATA"), "IDENTITY_DATA"
	case env("PC_DATA") != "":
		c.DataDir, c.DataSource = filepath.Join(env("PC_DATA"), "identity"), "PC_DATA"
	default:
		key := c.Branch
		if key == "unknown" {
			key = "commit-" + short(c.Commit)
		}
		c.DataDir, c.DataSource = filepath.Join(home, ".identity-test", safeName(key)), "standalone default (per branch)"
	}
	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return c, err
	}
	c.DataDir = abs
	if c.CheckoutDir != "" && within(abs, c.CheckoutDir) {
		return c, fmt.Errorf("data folder %s is inside the code checkout %s — identity data must live outside the repository; set -data, IDENTITY_DATA or PC_DATA", abs, c.CheckoutDir)
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

// ---------- events ----------

type Event struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"` // start | restore | create | repeat | error | problem | stop | test-tool
	Detail string    `json:"detail"`
	ID     string    `json:"id,omitempty"`
}

type EventLog struct {
	mu   sync.Mutex
	list []Event
	f    *os.File
	err  string
}

func openEventLog(dir string) *EventLog {
	l := &EventLog{}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		l.err = "event log not writable: " + err.Error()
	} else {
		l.f = f
	}
	return l
}

// Add records an event in memory, in events.jsonl and on stdout (the
// Project Controller log). Names are kept out of the log lines.
func (l *EventLog) Add(kind, detail, id string) {
	e := Event{At: time.Now().UTC(), Kind: kind, Detail: detail, ID: id}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, e)
	if len(l.list) > 100 {
		l.list = l.list[len(l.list)-100:]
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
	fmt.Printf("%s  %-9s %s%s\n", time.Now().Format("15:04:05"), kind, detail, idPart)
}

func (l *EventLog) Recent(n int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.list) > n {
		return append([]Event(nil), l.list[len(l.list)-n:]...)
	}
	return append([]Event(nil), l.list...)
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
	dataFlag := flag.String("data", "", "data folder (overrides IDENTITY_DATA and PC_DATA)")
	flag.Parse()
	home, _ := os.UserHomeDir()
	cfg, err := resolveConfig(*dataFlag, os.Getenv, gitCmd, home)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if err := run(cfg, nil); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run serves until SIGINT/SIGTERM (or until stop is closed, in tests).
func run(cfg Config, stop <-chan struct{}) error {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("can't create data folder %s: %w", cfg.DataDir, err)
	}
	lock, err := acquireLock(cfg.DataDir, cfg.Port)
	if err != nil {
		return err
	}
	defer lock.release()

	events := openEventLog(cfg.DataDir)
	defer events.Close()
	store, err := openStore(cfg.DataDir)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+cfg.Port)
	if err != nil {
		return fmt.Errorf("can't listen on 127.0.0.1:%s: %w", cfg.Port, err)
	}
	events.Add("start", fmt.Sprintf("Identity %s on http://127.0.0.1:%s/ (pid %d) · branch %s (%s%s) · commit %s · data %s (%s)",
		appVersion, cfg.Port, os.Getpid(), cfg.Branch, cfg.BranchFrom, map[bool]string{true: ", detached checkout", false: ""}[cfg.Detached],
		cfg.Commit, cfg.DataDir, cfg.DataSource), "")
	list := store.List()
	events.Add("restore", fmt.Sprintf("loaded %d saved identit%s from disk", len(list), map[bool]string{true: "y", false: "ies"}[len(list) == 1]), "")
	for _, it := range list {
		events.Add("restore", "restored "+typeLabel(it.EntityType)+" created "+it.CreatedAt.Format(time.RFC3339), it.ID)
	}
	for _, p := range store.Problems() {
		events.Add("problem", "could not load "+p.File+": "+p.Reason+" (file left untouched)", "")
	}

	s := &server{cfg: cfg, store: store, events: events, started: time.Now()}
	srv := &http.Server{Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

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
	events.Add("stop", "stopped cleanly ("+reason+"); all saves were already on disk", "")
	return nil
}

func firstSet(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
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
