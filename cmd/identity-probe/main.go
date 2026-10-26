// identity-probe is a small TEST CLIENT that talks to Identity's API v1 the
// way Thought Processor should. It is not Thought Processor and has no UI.
//
// It exists so the owner (or an AI operator such as a Grok bot) can test this
// Identity alpha candidate without waiting for Processor, and as working
// reference code for the Processor team.
//
//	identity-probe pair       ask Identity for a pairing code, type it in, save the pairing
//	identity-probe check      run the read checks and print PASS/FAIL (exit 1 on any FAIL)
//	identity-probe create     create an identity:  -name Tass -kind companion
//	identity-probe github     start GitHub sign-in through Identity and wait for approval
//	identity-probe shutdown   ask Identity to stop (as if the person chose Shut down) and verify it exited
//	identity-probe unpair     remove this probe's pairing
//	identity-probe smoke      start the real identity binary itself and run the whole
//	                          two-process test (pairing, restart, both start orders,
//	                          wrong service, shutdown, unpair) on throwaway profiles
//
// Flags (all commands): -url http://127.0.0.1:7790  -state <file>
// The pairing token is kept only in the state file (0600) and never printed.
package main

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type pairingFile struct {
	BaseURL          string    `json:"base_url"`
	ProcessorID      string    `json:"processor_id"`
	Token            string    `json:"token"`
	IdentitySystemID string    `json:"identity_system_id"`
	PairedAt         time.Time `json:"paired_at"`
}

var (
	baseURL   string
	statePath string
	client    = &http.Client{Timeout: 20 * time.Second}
	fails     int
)

func defaultState() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "identity-probe-pairing.json"
	}
	return filepath.Join(home, ".identity-probe", "pairing.json")
}

// newFlags lets -url and -state go before or after the command.
func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(&baseURL, "url", baseURL, "Identity address")
	fs.StringVar(&statePath, "state", statePath, "pairing file")
	return fs
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func pass(what, detail string) { fmt.Printf("PASS  %-34s %s\n", what, detail) }
func fail(what, detail string) { fails++; fmt.Printf("FAIL  %-34s %s\n", what, detail) }
func info(what, detail string) { fmt.Printf("INFO  %-34s %s\n", what, detail) }

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

// locatorDir mirrors identity's per-user locator folder.
func locatorDir() string {
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

type locator struct {
	BaseURL  string `json:"base_url"`
	SystemID string `json:"identity_system_id"`
	PID      int    `json:"pid"`
	Profile  string `json:"profile"`
}

// locators lists running-Identity locator files. They only say where to
// look; trust still comes from the pairing proof.
func locators() []locator {
	dir := locatorDir()
	if dir == "" {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(dir, "idn_*.json"))
	var out []locator
	for _, f := range files {
		var l locator
		if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &l) == nil && l.BaseURL != "" {
			out = append(out, l)
		}
	}
	return out
}

func load() pairingFile {
	baseURL = strings.TrimRight(baseURL, "/")
	b, err := os.ReadFile(statePath)
	if err != nil {
		die("not paired yet (no %s). Run:  identity-probe pair", statePath)
	}
	var p pairingFile
	if json.Unmarshal(b, &p) != nil || len(p.Token) != 64 {
		die("%s is damaged; delete it and pair again", statePath)
	}
	explicit := baseURL != ""
	if !explicit {
		baseURL = p.BaseURL
		// Identity may have moved port: find THIS installation by its ID.
		if _, _, err := plain("GET", "/api/health", nil); err != nil {
			for _, l := range locators() {
				if l.SystemID == p.IdentitySystemID && l.BaseURL != p.BaseURL {
					info("locator", "paired Identity "+l.SystemID+" is now at "+l.BaseURL)
					baseURL = l.BaseURL
				}
			}
		}
	}
	return p
}

func save(p pairingFile) {
	os.MkdirAll(filepath.Dir(statePath), 0o700)
	b, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(statePath, b, 0o600); err != nil {
		die("could not save pairing: %v", err)
	}
}

// plain calls an unauthenticated route.
func plain(method, path string, body any) (int, map[string]any, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, baseURL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m, nil
}

var errProof = errors.New("answer is NOT signed by the paired Identity")
var errWrongIdentity = errors.New("a DIFFERENT Identity installation answered (wrong service)")

// hm is HMAC-SHA256 as hex.
func hm(key []byte, parts ...string) string {
	m := hmac.New(sha256.New, key)
	for _, x := range parts {
		m.Write([]byte(x))
	}
	return hex.EncodeToString(m.Sum(nil))
}

// keys derives the request and response keys from the pairing secret
// (API identity/2). The secret itself is never sent.
func keys(secret string) (req, resp []byte) {
	r, _ := hex.DecodeString(hm([]byte(secret), "identity/2 request key"))
	s, _ := hex.DecodeString(hm([]byte(secret), "identity/2 response key"))
	return r, s
}

// call is a paired request (API identity/2). The request is signed with the
// request key (method, path, time, nonce, body hash); the answer must carry a
// proof made with the separate response key over our nonce, the status and
// the exact body, plus the installation ID we paired with. This is the check
// Thought Processor does too.
func call(p pairingFile, method, path string, body any) (int, map[string]any, error) {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, baseURL+path, bytes.NewReader(b))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rk, sk := keys(p.Token)
	nonce := randHex(12)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha256.Sum256(b)
	req.Header.Set("X-Processor-ID", p.ProcessorID)
	req.Header.Set("X-Identity-Time", ts)
	req.Header.Set("X-Identity-Nonce", nonce)
	req.Header.Set("X-Identity-Auth", hm(rk, "identity/2 request\n"+method+"\n"+req.URL.RequestURI()+"\n"+ts+"\n"+nonce+"\n"+hex.EncodeToString(sum[:])))
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	json.Unmarshal(raw, &m)
	if resp.StatusCode == 401 {
		return 401, m, nil // not paired with this Identity (or unpaired)
	}
	want := hm(sk, "identity/2 response\n"+nonce+"\n"+strconv.Itoa(resp.StatusCode)+"\n", string(raw))
	if !hmac.Equal([]byte(want), []byte(resp.Header.Get("X-Identity-Proof"))) {
		return resp.StatusCode, m, errProof
	}
	if resp.Header.Get("X-Identity-System-ID") != p.IdentitySystemID {
		return resp.StatusCode, m, errWrongIdentity
	}
	return resp.StatusCode, m, nil
}

func str(m map[string]any, keys ...string) string {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = mm[k]
	}
	if cur == nil {
		return ""
	}
	return fmt.Sprint(cur)
}

func cmdPair(args []string) {
	fs := newFlags("pair")
	code := fs.String("code", "", "the code shown in Identity's window (asked for if empty)")
	label := fs.String("label", "identity-probe (test client — not Thought Processor)", "name Identity shows for this client")
	fs.Parse(args)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		var live []locator
		for _, l := range locators() {
			baseURL = l.BaseURL
			if st, h, err := plain("GET", "/api/health", nil); err == nil && st == 200 && h["app"] == "identity" {
				live = append(live, l)
			}
		}
		baseURL = ""
		switch len(live) {
		case 1:
			baseURL = live[0].BaseURL
			fmt.Printf("Found Identity %s at %s (profile %s)\n", live[0].SystemID, live[0].BaseURL, live[0].Profile)
		case 0:
			baseURL = "http://127.0.0.1:7790"
		default:
			fmt.Println("More than one Identity is running on this computer:")
			for _, l := range live {
				fmt.Printf("  %s  %s  profile %s\n", l.SystemID, l.BaseURL, l.Profile)
			}
			die("choose one with -url")
		}
	}
	st, h, err := plain("GET", "/api/health", nil)
	if err != nil || st != 200 || h["app"] != "identity" {
		die("Identity is not answering at %s (%v). Start identity.exe first.", baseURL, err)
	}
	id := "probe_" + randHex(6)
	if *code == "" {
		st, r, err := plain("POST", "/api/v1/pair/request", map[string]string{"processor_id": id, "processor_label": *label})
		if err != nil || st != 202 {
			die("pairing request refused (%d %v %v)", st, r, err)
		}
		fmt.Println("Identity is now showing a one-time code in ITS OWN console window.")
		fmt.Print("Type that code here and press Enter: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		*code = strings.TrimSpace(line)
	}
	st, r, err := plain("POST", "/api/v1/pair", map[string]string{"code": *code, "processor_id": id, "processor_label": *label})
	if err != nil || st != 200 {
		die("pairing failed: %d %s", st, str(r, "error"))
	}
	p := pairingFile{BaseURL: baseURL, ProcessorID: id, Token: str(r, "secret"), IdentitySystemID: str(r, "identity_system_id"), PairedAt: time.Now().UTC()}
	save(p)
	fmt.Printf("PAIRED  as %s with Identity installation %s (pairing saved to %s; token not shown)\n", id, p.IdentitySystemID, statePath)
}

func cmdCheck(args []string) {
	fs := newFlags("check")
	wait := fs.Duration("wait", 0, "keep trying this long if Identity isn't running yet (tests Processor-first start)")
	fs.Parse(args)
	p := load()
	os.Exit(runCheck(p, *wait))
}

// runCheck returns 0 = PASS, 1 = FAIL, 2 = AUTHORITY UNAVAILABLE.
func runCheck(p pairingFile, wait time.Duration) int {
	fails = 0
	fmt.Printf("identity-probe check  %s  (paired as %s)\n\n", baseURL, p.ProcessorID)

	deadline := time.Now().Add(wait)
	var hst int
	var h map[string]any
	var err error
	for {
		hst, h, err = plain("GET", "/api/health", nil)
		if err == nil || time.Now().After(deadline) {
			break
		}
		info("waiting for Identity", "not running yet — protected actions would be BLOCKED (AUTHORITY UNAVAILABLE)")
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		fail("Identity reachable", "(expected when Identity is stopped) AUTHORITY UNAVAILABLE — no answer at "+baseURL+"; a real Processor must block protected actions now")
		return 2
	}
	if hst == 200 && h["app"] == "identity" {
		pass("public health", fmt.Sprintf("version %s, api %s (self-reported, not trusted)", str(h, "version"), str(h, "api")))
	} else {
		fail("public health", fmt.Sprintf("%d %v", hst, h))
	}

	st, s, err := call(p, "GET", "/api/v1/status", nil)
	switch {
	case err != nil:
		fail("status signed by paired Identity", err.Error())
	case st == 401:
		fail("status signed by paired Identity", "token refused — this Identity no longer knows the probe (unpaired, or a different profile). Pair again.")
	case st == 200 && str(s, "state") == "READY":
		pass("status signed by paired Identity", fmt.Sprintf("installation %s, pid %s, %s", str(s, "identity_system_id"), str(s, "lifecycle", "pid"), str(s, "lifecycle", "ownership")))
		info("github", str(s, "github", "state")+" "+str(s, "github", "account", "login"))
	default:
		fail("status", fmt.Sprintf("%d %v", st, s))
	}
	if st != 200 || err != nil {
		return 1
	}

	st, l, err := call(p, "GET", "/api/v1/identities", nil)
	firstID := ""
	if err == nil && st == 200 {
		ids, _ := l["identities"].([]any)
		pass("list identities", fmt.Sprintf("%d active", len(ids)))
		for _, x := range ids {
			e := x.(map[string]any)
			info("  identity", fmt.Sprintf("%s  %s (%s)  sources=%v features=%v", str(e, "id"), str(e, "name"), str(e, "kind"), e["authorized_sources"], e["enabled_features"]))
			if firstID == "" {
				firstID = str(e, "id")
			}
			if src, _ := e["authorized_sources"].([]any); len(src) != 0 {
				fail("  default no access", str(e, "id")+" has sources — grants are not supposed to exist in this alpha")
			}
		}
	} else {
		fail("list identities", fmt.Sprintf("%d %v %v", st, err, l))
	}

	st, f, err := call(p, "GET", "/api/v1/features", nil)
	if err == nil && st == 200 {
		pass("feature catalog", "")
		for _, x := range f["features"].([]any) {
			e := x.(map[string]any)
			info("  "+str(e, "feature"), "eligible_to_install="+str(e, "eligible_to_install", "state")+"  installed="+str(e, "installed")+"  registered="+str(e, "service_registered"))
		}
	} else {
		fail("feature catalog", fmt.Sprintf("%d %v", st, err))
	}

	target := firstID
	if target == "" {
		target = "ent_00000000000000000000"
	}
	st, d, err := call(p, "POST", "/api/v1/authorize", map[string]string{"identity_id": target, "feature": "thought-engine", "operation": "observe", "source": "discord"})
	if err == nil && st == 200 && str(d, "decision") == "DENIED" && str(d, "cache") == "none" {
		pass("authorize defaults to DENIED", str(d, "state")+" — "+str(d, "reason"))
	} else {
		fail("authorize defaults to DENIED", fmt.Sprintf("%d %v %v", st, err, d))
	}

	// Wrong-service guard: the same request with a made-up token must fail.
	bad := p
	bad.Token = randHex(32)
	if st, _, _ := call(bad, "GET", "/api/v1/status", nil); st == 401 {
		pass("unknown token refused", "")
	} else {
		fail("unknown token refused", fmt.Sprint(st))
	}

	fmt.Println()
	if fails > 0 {
		fmt.Printf("RESULT: FAIL (%d)\n", fails)
		return 1
	}
	fmt.Println("RESULT: PASS")
	return 0
}

func cmdCreate(args []string) {
	fs := newFlags("create")
	name := fs.String("name", "", "identity name")
	kind := fs.String("kind", "person", "person | companion | business | project")
	fs.Parse(args)
	p := load()
	st, e, err := call(p, "POST", "/api/v1/identities", map[string]string{"name": *name, "kind": *kind})
	if err != nil || st != 201 {
		die("create refused: %d %s %v", st, str(e, "error"), err)
	}
	fmt.Printf("CREATED %s  %s (%s)  — no access granted\n", str(e, "id"), str(e, "name"), str(e, "kind"))
}

func cmdGitHub(args []string) {
	fs := newFlags("github")
	timeout := fs.Duration("timeout", 10*time.Minute, "how long to wait for the person to approve")
	fs.Parse(args)
	p := load()
	st, g, err := call(p, "POST", "/api/v1/github/connect", map[string]string{})
	if err != nil || st != 200 {
		die("connect refused: %d %s %v", st, str(g, "error"), err)
	}
	if str(g, "state") == "VERIFIED" {
		fmt.Println("Already signed in to GitHub as", str(g, "account", "login"))
		return
	}
	fmt.Println("A PERSON must do this step:")
	fmt.Println("  1. Open", str(g, "verification_uri"))
	fmt.Println("  2. Enter the code:", str(g, "user_code"))
	fmt.Println("  3. Press Continue / Authorize on GitHub.")
	end := time.Now().Add(*timeout)
	last := ""
	for time.Now().Before(end) {
		time.Sleep(3 * time.Second)
		_, g, err = call(p, "GET", "/api/v1/github", nil)
		if err != nil {
			die("Identity stopped answering: %v", err)
		}
		s := str(g, "state")
		if s != last {
			fmt.Println("  GitHub state:", s, str(g, "message"))
			last = s
		}
		switch s {
		case "VERIFIED":
			fmt.Println("SIGNED IN as", str(g, "account", "login"), "— this is not, by itself, permission to use any feature.")
			return
		case "ERROR", "EXPIRED", "DENIED", "SIGNED_OUT":
			die("sign-in did not complete: %s %s", s, str(g, "message"))
		}
	}
	die("timed out waiting for approval")
}

func cmdShutdown(args []string) {
	newFlags("shutdown").Parse(args)
	p := load()
	st, r, err := call(p, "POST", "/api/v1/shutdown", map[string]string{"confirm": "shut-down-identity", "reason": "identity-probe shutdown test"})
	if err != nil || st != 202 {
		die("shutdown refused: %d %s %v", st, str(r, "error"), err)
	}
	fmt.Printf("Identity accepted shutdown (pid %s). Waiting for it to exit…\n", str(r, "pid"))
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		if _, _, err := plain("GET", "/api/health", nil); err != nil {
			fmt.Println("PASS  Identity stopped (port closed). Check its window says \"Identity stopped.\"")
			return
		}
	}
	die("FAIL  Identity still answering 10 s after accepting shutdown")
}

func cmdUnpair(args []string) {
	newFlags("unpair").Parse(args)
	p := load()
	st, _, err := call(p, "POST", "/api/v1/unpair", map[string]string{})
	if err != nil || st != 200 {
		die("unpair failed: %d %v", st, err)
	}
	if st, _, _ := call(p, "GET", "/api/v1/status", nil); st == 401 {
		fmt.Println("PASS  unpaired; the old token is now refused")
	} else {
		die("FAIL  token still works after unpair (%d)", st)
	}
	os.Remove(statePath)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: identity-probe [-url http://127.0.0.1:7790] [-state file] pair|check|create|github|shutdown|unpair|smoke [flags]")
		os.Exit(2)
	}
	g := flag.NewFlagSet("identity-probe", flag.ExitOnError)
	g.StringVar(&baseURL, "url", "", "Identity address (default: from the pairing file, else http://127.0.0.1:7790)")
	g.StringVar(&statePath, "state", defaultState(), "where this probe keeps its pairing (0600)")
	g.Parse(os.Args[1:])
	rest := g.Args()
	if len(rest) == 0 {
		die("missing command")
	}
	switch rest[0] {
	case "pair":
		cmdPair(rest[1:])
	case "check":
		cmdCheck(rest[1:])
	case "create":
		cmdCreate(rest[1:])
	case "github":
		cmdGitHub(rest[1:])
	case "shutdown":
		cmdShutdown(rest[1:])
	case "unpair":
		cmdUnpair(rest[1:])
	case "smoke":
		cmdSmoke(rest[1:])
	default:
		die("unknown command %q", rest[0])
	}
}
