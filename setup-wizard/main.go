// Identity setup wizard (experiment).
//
// Five steps from opening Identity to a running Thought Processor:
//
//	1 Welcome  2 Connect GitHub  3 Find Thought Processor
//	4 Start it through Project-controller (owner approves there)  5 Open it
//
// No entities, no Thought Engine, no AI worker, no fake connection states.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	"runtime"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
)

const appVersion = "0.1.0-wizard-experiment"

// The approved version this first experience starts. Changing it is a code
// change the owner reviews, not something a request can choose.
const (
	defaultRepo   = "Arameair/Thought-Processor"
	defaultBranch = "experiment/processor-first-prototype-20261024"
)

// GitHub app identity. A Client ID is public (not a secret). The owner pastes
// theirs into github-app.json next to this program (see docs/SETUP_GITHUB_APP.md).
type appFile struct {
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
}

func revision() (string, bool) {
	rev, modified := "unknown", false
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				rev = s.Value
			}
			if s.Key == "vcs.modified" {
				modified = s.Value == "true"
			}
		}
	}
	return rev, modified
}

func main() {
	home, _ := os.UserHomeDir()
	data := flag.String("data", filepath.Join(home, ".identity-setup"), "folder for Identity's small runtime files")
	port := flag.Int("port", 7790, "loopback port for the Identity window")
	controller := flag.String("controller", "http://127.0.0.1:7777", "Project-controller address on this computer")
	appConfig := flag.String("github-app", "", "path to github-app.json (default: next to the program, then the current folder)")
	noBrowser := flag.Bool("no-browser", false, "don't open the browser automatically; just print the link")
	// Test-only overrides. When used, every screen says TEST MODE.
	ghWeb := flag.String("test-github-web", "", "TEST ONLY: replace https://github.com")
	ghAPI := flag.String("test-github-api", "", "TEST ONLY: replace https://api.github.com")
	flag.Parse()

	cfg := GitHubConfig{WebBase: "https://github.com", APIBase: "https://api.github.com"}
	if *ghWeb != "" || *ghAPI != "" {
		cfg.WebBase, cfg.APIBase = *ghWeb, *ghAPI
	}
	cfg.ClientID, cfg.Scope = os.Getenv("IDENTITY_GITHUB_CLIENT_ID"), ""
	if cfg.ClientID == "" {
		for _, p := range candidatePaths(*appConfig) {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			var af appFile
			if json.Unmarshal(b, &af) == nil {
				cfg.ClientID, cfg.Scope = af.ClientID, af.Scope
			}
			break
		}
	}
	if err := run(*data, *port, *controller, cfg, !*noBrowser, nil, nil); err != nil {
		fmt.Fprintln(os.Stderr, "Identity couldn't start:", err)
		os.Exit(1)
	}
}

func candidatePaths(explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	out := []string{}
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Join(filepath.Dir(exe), "github-app.json"))
	}
	return append(out, "github-app.json")
}

type instanceFile struct {
	PID  int    `json:"pid"`
	Port int    `json:"port"`
	Link string `json:"link"`
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// run serves until interrupted. ready (tests) receives the app.
func run(dataDir string, port int, controller string, cfg GitHubConfig, browser bool, quit <-chan struct{}, ready chan<- *App) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("can't create %s: %w", dataDir, err)
	}
	instPath := filepath.Join(dataDir, "instance.json")
	lk, err := acquireLock(filepath.Join(dataDir, "identity.lock"))
	if err != nil {
		// Already running: bring its window back instead of starting twice.
		if b, e := os.ReadFile(instPath); e == nil {
			var inst instanceFile
			if json.Unmarshal(b, &inst) == nil && inst.Link != "" {
				fmt.Printf("Identity is already running at http://127.0.0.1:%d/ — opening its window.\n", inst.Port)
				if browser {
					openBrowser(inst.Link)
				}
				return nil
			}
		}
		return errors.New("another Identity is already running from " + dataDir)
	}
	defer lk.release()

	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return fmt.Errorf("port %d is used by another program; start Identity with -port and a different number", port)
		}
		return err
	}
	port = ln.Addr().(*net.TCPAddr).Port
	app := NewApp(port, controller, cfg)
	link := fmt.Sprintf("http://127.0.0.1:%d/#k=%s", port, app.code)
	if b, err := json.Marshal(instanceFile{PID: os.Getpid(), Port: port, Link: link}); err == nil {
		writeFile0600(instPath, b)
	}
	defer os.Remove(instPath)

	srv := &http.Server{Handler: app.Routes(), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	go app.loop()

	rev, mod := revision()
	fmt.Printf("Identity setup %s (commit %s%s)\n", appVersion, shortRev(rev), map[bool]string{true: " + local changes"}[mod])
	if !cfg.IsReal() {
		fmt.Println("  TEST MODE: GitHub is replaced by", cfg.WebBase, "and", cfg.APIBase)
	}
	fmt.Printf("  window : http://127.0.0.1:%d/ (opening in your browser)\n", port)
	fmt.Println("  stop   : close this window or press Ctrl+C")
	if browser {
		if err := openBrowser(link); err != nil {
			fmt.Println("  Couldn't open a browser automatically. Run Identity again to reopen its window.")
		}
	}
	if ready != nil {
		ready <- app
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	select {
	case <-sig:
	case <-quit:
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	app.stop()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	fmt.Println("Identity stopped. Thought Processor and Project Controller keep running.")
	return nil
}

func shortRev(r string) string {
	if len(r) > 12 {
		return r[:12]
	}
	return r
}

func writeFile0600(path string, b []byte) {
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, path)
	}
}
