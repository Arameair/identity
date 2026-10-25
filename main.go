// Identity — home-trial Prototype B (Claude): first-use experience.
//
// One standalone executable. No Project Controller, no gh CLI needed.
//
//	identity-b.exe [-port 7798] [-data <profile folder>] [-processor 127.0.0.1:7860] [-no-browser]
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
	"strconv"
	"strings"
	"syscall"

	"github.com/Arameair/identity/internal/app"
	"github.com/Arameair/identity/internal/github"
	"github.com/Arameair/identity/internal/publisher"
)

const (
	DefaultPort = 7798 // Prototype A (GPT-6) uses 7797; Thought Processor 7860
	appVersion  = "0.2.0-home-trial-b"
)

// Set with: go build -ldflags "-X main.buildBranch=experiment/identity-home-claude-20261025"
var buildBranch = ""

func defaultData() string {
	if v := os.Getenv("IDENTITY_B_DATA"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", "identity-home-claude-b")
	}
	return filepath.Join(home, ".identity", "home-claude-b")
}

func defaultPort() int {
	if v, err := strconv.Atoi(os.Getenv("IDENTITY_B_PORT")); err == nil && v > 0 {
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
	noBrowser := flag.Bool("no-browser", false, "do not open a browser; just print the private link")
	testWeb := flag.String("test-github-web", "", "TEST FIXTURE ONLY: replaces https://github.com")
	testAPI := flag.String("test-github-api", "", "TEST FIXTURE ONLY: replaces https://api.github.com")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("identity", appVersion, "prototype B", buildBranch)
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
	a, err := app.New(app.Options{Port: *port, DataDir: *data, GitHub: cfg, DefaultApp: def, ProcessorAddr: *proc, Version: appVersion, Branch: branch})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Identity could not start:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = a.Run(ctx, nil, func(link string) {
		fmt.Printf("Identity — Prototype B (Claude) %s  commit %s\n", appVersion, short(a.Build.Revision))
		fmt.Printf("  profile : %s\n", *data)
		fmt.Printf("  run     : %s   pid %d   port %d (loopback only)\n", a.RunID, os.Getpid(), a.Port)
		if cfg.Mode() != "real" {
			fmt.Printf("  GITHUB  : TEST FIXTURE at %s — not real GitHub\n", cfg.WebBase)
		} else if cfg.ClientID == "" {
			fmt.Println("  github  : this build has no publisher GitHub App built in — Identity can guide you through using your own")
		} else {
			fmt.Println("  github  : " + cfg.ClientIDSource)
		}
		fmt.Printf("  open    : %s\n", link)
		fmt.Println("            (private link — do not paste it anywhere; it changes every start and is")
		fmt.Printf("             also in %s)\n", a.Store.OpenLinkPath())
		fmt.Println("  stop    : Ctrl+C here, close this window, or Stop Identity in the page. Only Identity stops.")
		if !*noBrowser {
			if err := openBrowser(link); err != nil {
				fmt.Println("  (browser did not open automatically; copy the link above into your browser)")
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
