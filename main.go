// Identity — the independent identity and permission service behind Thought
// Processor. It runs in the background; people use it through Thought
// Processor. Its own web page is a troubleshooting view only.
//
// One standalone executable. No Project Controller, no gh CLI needed.
//
//	identity.exe [-port 7790] [-data <profile folder>] [-open] [-managed-by <who>]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Arameair/identity/internal/app"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/publisher"
)

const (
	// 7790 avoids Thought-engine (7798), Thought Processor (7860) and the
	// 2026-10-25 trial prototypes (7797, 7798).
	DefaultPort = 7790
	appVersion  = "0.4.0-alpha"
)

// Optional: go build -ldflags "-X main.buildBranch=<branch>"; otherwise PC_BRANCH, else UNKNOWN.
var buildBranch = ""

func defaultData() string {
	if v := os.Getenv("IDENTITY_DATA"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", "identity-data")
	}
	return filepath.Join(home, ".identity", "default")
}

// defaultLocatorDir is the per-user folder of running-Identity locators:
// %LOCALAPPDATA%\Arameair\Identity\instances on Windows. IDENTITY_LOCATOR_DIR
// overrides it; "-" turns it off.
func defaultLocatorDir() string {
	if v := os.Getenv("IDENTITY_LOCATOR_DIR"); v != "" {
		if v == "-" {
			return ""
		}
		return v
	}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "Arameair", "Identity", "instances")
		}
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "arameair-identity", "instances")
	}
	return ""
}

func defaultPort() int {
	if v, err := strconv.Atoi(os.Getenv("IDENTITY_PORT")); err == nil && v > 0 {
		return v
	}
	return DefaultPort
}

// clientID picks which GitHub App sign-in uses. Normal users get the
// publisher's app built into this binary. Order:
//  1. env IDENTITY_GITHUB_CLIENT_ID             (developer override)
//  2. <profile>/github-app.json                 (the person's own app, set up with the guide in Identity)
//  3. github-app.json beside the exe / cwd      (developer override)
//  4. publisher.GitHubAppClientID               (built in — the normal case)
//
// def is the same without step 2: what "switch back" returns to.
func clientID(dataDir string) (cur, def app.AppChoice) {
	def = builtinOrDev()
	if v := os.Getenv("IDENTITY_GITHUB_CLIENT_ID"); strings.TrimSpace(v) != "" {
		return def, def
	}
	p := filepath.Join(dataDir, "github-app.json")
	if id := readClientIDFile(p); id != "" {
		return app.AppChoice{ID: id, Source: "your own GitHub App (set up in Identity)", Kind: "own"}, def
	}
	return def, def
}

func builtinOrDev() app.AppChoice {
	if id := strings.TrimSpace(os.Getenv("IDENTITY_GITHUB_CLIENT_ID")); id != "" {
		return app.AppChoice{ID: id, Source: "developer override: environment IDENTITY_GITHUB_CLIENT_ID", Kind: "developer"}
	}
	var paths []string
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "github-app.json"))
	}
	paths = append(paths, "github-app.json")
	for _, p := range paths {
		if id := readClientIDFile(p); id != "" {
			return app.AppChoice{ID: id, Source: "developer override: " + p, Kind: "developer"}
		}
	}
	if id := strings.TrimSpace(publisher.GitHubAppClientID); github.ValidClientID(id) {
		return app.AppChoice{ID: id, Source: "built into this build (" + publisher.Name + "'s GitHub App)", Kind: "built-in"}
	}
	return app.AppChoice{}
}

func readClientIDFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var m map[string]string
	if json.Unmarshal(b, &m) == nil && github.ValidClientID(strings.TrimSpace(m["client_id"])) {
		return strings.TrimSpace(m["client_id"])
	}
	return ""
}

func openBrowser(link string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	case "darwin":
		cmd = exec.Command("open", link)
	default:
		cmd = exec.Command("xdg-open", link)
	}
	return cmd.Start()
}

func main() {
	port := flag.Int("port", defaultPort(), "loopback port for Identity (0 = pick a free one)")
	data := flag.String("data", defaultData(), "private profile folder for this Identity")
	proc := flag.String("processor", "", "optional Thought Processor address to watch, e.g. 127.0.0.1:7860")
	open := flag.Bool("open", false, "also open the troubleshooting page in a browser (not needed for normal use)")
	flag.Bool("no-browser", true, "kept for older scripts; Identity no longer opens a browser unless -open is given")
	managedBy := flag.String("managed-by", os.Getenv("IDENTITY_MANAGED_BY"), "set by the program that started Identity and owns its lifecycle, e.g. thought-processor:tp_123")
	testWeb := flag.String("test-github-web", "", "TEST FIXTURE ONLY: replaces https://github.com")
	testAPI := flag.String("test-github-api", "", "TEST FIXTURE ONLY: replaces https://api.github.com")
	version := flag.Bool("version", false, "print version and exit")
	var pins multiFlag
	flag.Var(&pins, "trust-fixture-artifact", "pin one more exact feature artifact SHA-256 that may be registered (repeatable; the reviewed P1 fixture is built in)")
	flag.Parse()

	if *version {
		rev := "UNKNOWN"
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, st := range bi.Settings {
				if st.Key == "vcs.revision" {
					rev = st.Value
				}
			}
		}
		fmt.Println("identity", appVersion, "alpha candidate", buildBranch, "commit", rev)
		return
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintln(os.Stderr, "invalid -port")
		os.Exit(2)
	}
	cfg := github.Config{WebBase: github.RealWeb, APIBase: github.RealAPI}
	if *testWeb != "" || *testAPI != "" {
		if *testWeb == "" || *testAPI == "" {
			fmt.Fprintln(os.Stderr, "both -test-github-web and -test-github-api are required for fixture mode")
			os.Exit(2)
		}
		cfg.WebBase, cfg.APIBase = strings.TrimRight(*testWeb, "/"), strings.TrimRight(*testAPI, "/")
	}
	abs, err := filepath.Abs(*data)
	if err == nil {
		*data = abs
	}
	cur, def := clientID(*data)
	cfg.ClientID, cfg.ClientIDSource, cfg.ClientIDKind = cur.ID, cur.Source, cur.Kind

	branch := buildBranch
	if branch == "" {
		branch = os.Getenv("PC_BRANCH") // set by Project Controller
	}
	var outMu sync.Mutex
	announce := func(lines []string) {
		outMu.Lock()
		defer outMu.Unlock()
		fmt.Println(strings.Join(lines, "\n"))
	}
	a, err := app.New(app.Options{Port: *port, DataDir: *data, GitHub: cfg, DefaultApp: def, ProcessorAddr: *proc, Version: appVersion, Branch: branch,
		ManagedBy: strings.TrimSpace(*managedBy), Announce: announce, LocatorDir: defaultLocatorDir(), TrustArtifacts: pins})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Identity could not start:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = a.Run(ctx, nil, func(link string) {
		outMu.Lock()
		defer outMu.Unlock()
		fmt.Printf("Identity %s  branch %s  commit %s\n", appVersion, a.Build.Branch, short(a.Build.Revision))
		fmt.Println("  Identity is running in the background. Use it through Thought Processor —")
		fmt.Println("  you don't need to open anything here. Keep this window open (minimise it is fine).")
		fmt.Println()
		fmt.Printf("  profile : %s\n", *data)
		fmt.Printf("  run     : %s   pid %d   port %d (loopback only)   api %s\n", a.RunID, os.Getpid(), a.Port, app.APIVersion)
		if p := a.LocatorPath(); p != "" {
			fmt.Printf("  locator : %s\n", p)
		}
		if *managedBy != "" {
			fmt.Printf("  started : by %s (it may stop Identity when you choose Shut down)\n", *managedBy)
		}
		if cfg.Mode() != "real" {
			fmt.Printf("  GITHUB  : TEST FIXTURE at %s — not real GitHub\n", cfg.WebBase)
		} else if cfg.ClientID == "" {
			fmt.Println("  github  : this build has no publisher GitHub App built in")
		} else {
			fmt.Println("  github  : " + cfg.ClientIDSource)
		}
		for _, d := range pins {
			fmt.Printf("  trust   : operator-pinned feature artifact %s… (-trust-fixture-artifact)\n", strings.ToLower(strings.TrimSpace(d))[:min(12, len(strings.TrimSpace(d)))])
		}
		if n := len(a.Pair.Clients()); n > 0 {
			fmt.Printf("  paired  : %d Thought Processor(s) — they reconnect on their own\n", n)
		} else {
			fmt.Println("  paired  : none yet — in Thought Processor choose Connect Identity; a code will appear HERE")
		}
		fmt.Println("  stop    : Ctrl+C here, or Shut down from Thought Processor. Only Identity stops.")
		fmt.Println()
		fmt.Println("  troubleshooting page (developers / support only — private, do not share):")
		fmt.Printf("    %s\n", link)
		if *open {
			if err := openBrowser(link); err != nil {
				fmt.Println("    (browser did not open automatically; copy the link above)")
			}
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Identity stopped with an error:", err)
		os.Exit(1)
	}
	fmt.Println("Identity stopped.")
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }
