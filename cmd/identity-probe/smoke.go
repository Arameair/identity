package main

// identity-probe smoke: starts the REAL identity executable as separate
// processes on throwaway profiles and runs the two-process test that a
// Thought Processor will depend on. Works the same on Windows and Linux.
//
// Reading the pairing code from Identity's console output stands in for a
// person reading it off the Identity window. Real GitHub is not used.

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var smokeCode = regexp.MustCompile(`\b([A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4})\b`)

type smokeRun struct {
	exe, work, data string
	port            int
	results         []string
	failed          int
	live            []*proc
}

type proc struct {
	cmd    *exec.Cmd
	log    string
	done   chan error
	exited chan struct{}
}

func (s *smokeRun) record(ok bool, step, detail string) {
	tag := "PASS"
	if !ok {
		tag = "FAIL"
		s.failed++
	}
	line := fmt.Sprintf("%s  %-48s %s", tag, step, detail)
	s.results = append(s.results, line)
	fmt.Println("STEP " + line)
}

func (s *smokeRun) start(name, data string, port int) (*proc, error) {
	logPath := filepath.Join(s.work, name+".console.txt")
	f, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(s.exe, "-port", strconv.Itoa(port), "-data", data, "-managed-by", "identity-probe-smoke")
	cmd.Stdout, cmd.Stderr = f, f
	// Keep test locators out of the person's real locator folder.
	cmd.Env = append(os.Environ(), "IDENTITY_LOCATOR_DIR="+filepath.Join(s.work, "locators"))
	if err := cmd.Start(); err != nil {
		f.Close()
		return nil, err
	}
	p := &proc{cmd: cmd, log: logPath, done: make(chan error, 1), exited: make(chan struct{})}
	go func() { err := cmd.Wait(); f.Close(); close(p.exited); p.done <- err }()
	s.live = append(s.live, p)
	return p, nil
}

func (s *smokeRun) startReady(name string) (*proc, error) {
	p, err := s.start(name, s.data, s.port)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 100; i++ {
		if st, h, err := plain("GET", "/api/health", nil); err == nil && st == 200 && h["app"] == "identity" {
			return p, nil
		}
		select {
		case err := <-p.done:
			b, _ := os.ReadFile(p.log)
			return nil, fmt.Errorf("identity exited at start (%v): %s", err, strings.TrimSpace(string(b)))
		case <-time.After(100 * time.Millisecond):
		}
	}
	p.cmd.Process.Kill()
	return nil, errors.New("identity did not answer within 10 s")
}

func (p *proc) console() string { b, _ := os.ReadFile(p.log); return string(b) }

func (p *proc) waitExit(d time.Duration) (bool, error) {
	select {
	case err := <-p.done:
		return true, err
	case <-time.After(d):
		return false, nil
	}
}

func (s *smokeRun) shutdownViaAPI(p *proc, pf pairingFile, reason string) bool {
	st, _, err := call(pf, "POST", "/api/v1/shutdown", map[string]string{"confirm": "shut-down-identity", "reason": reason})
	if err != nil || st != 202 {
		return false
	}
	exited, _ := p.waitExit(10 * time.Second)
	return exited
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func cmdSmoke(args []string) {
	fs := flag.NewFlagSet("smoke", flag.ExitOnError)
	exe := fs.String("identity", "", "path to the identity executable to test (default: identity[.exe] next to this probe, else in the current folder)")
	work := fs.String("work", "", "folder for throwaway profiles, console captures and the report (default: a new temp folder)")
	port := fs.Int("port", 7891, "port for the test Identity (not 7790, so a real Identity can keep running)")
	fs.Parse(args)

	if *exe == "" {
		name := "identity"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		cands := []string{name}
		if self, err := os.Executable(); err == nil {
			cands = append([]string{filepath.Join(filepath.Dir(self), name)}, cands...)
		}
		for _, c := range cands {
			if fileExists(c) {
				*exe = c
				break
			}
		}
	}
	if *exe == "" || !fileExists(*exe) {
		die("identity executable not found; build it first (go build -o identity.exe .) or pass -identity <path>")
	}
	abs, _ := filepath.Abs(*exe)
	if *work == "" {
		d, err := os.MkdirTemp("", "identity-smoke-")
		if err != nil {
			die("%v", err)
		}
		*work = d
	}
	os.MkdirAll(*work, 0o700)
	s := &smokeRun{exe: abs, work: *work, data: filepath.Join(*work, "profile"), port: *port}
	baseURL = "http://127.0.0.1:" + strconv.Itoa(*port)
	statePath = filepath.Join(*work, "probe-pairing.json")
	ver, _ := exec.Command(abs, "-version").Output()
	fmt.Printf("identity-probe smoke\n  identity : %s (%s)\n  work     : %s\n  port     : %d\n\n", abs, strings.TrimSpace(string(ver)), *work, *port)
	if _, _, err := plain("GET", "/api/health", nil); err == nil {
		die("something already answers on port %d; pick another with -port", *port)
	}
	var cur *proc
	must := func(p *proc, err error) *proc {
		if err != nil {
			s.record(false, "start Identity", err.Error())
			s.report(string(ver))
		}
		return p
	}

	// 1. Background service mode.
	cur = must(s.startReady("run1"))
	con := cur.console()
	s.record(strings.Contains(con, "Use it through Thought Processor"), "1 starts as a background service", "console tells the person to use Thought Processor; no browser opened")
	svc := filepath.Join(s.data, "runtime", "service.json")
	b, _ := os.ReadFile(svc)
	s.record(strings.Contains(string(b), `"pid": `+strconv.Itoa(cur.cmd.Process.Pid)), "1 runtime/service.json names this process", "locator for either start order (not trust)")

	// 2. Pairing started by the program; code only in Identity's window.
	st, r, err := plain("POST", "/api/v1/pair/request", map[string]string{"processor_id": "smoke_probe", "processor_label": "identity-probe smoke"})
	time.Sleep(300 * time.Millisecond)
	m := smokeCode.FindAllStringSubmatch(cur.console(), -1)
	code := ""
	if len(m) > 0 {
		code = m[len(m)-1][1]
	}
	s.record(err == nil && st == 202 && code != "" && !strings.Contains(fmt.Sprint(r), code), "2 pair request shows code only in Identity window", fmt.Sprintf("HTTP %d", st))
	st, r, err = plain("POST", "/api/v1/pair", map[string]string{"code": code, "processor_id": "smoke_probe", "processor_label": "identity-probe smoke"})
	pf := pairingFile{BaseURL: baseURL, ProcessorID: "smoke_probe", Token: str(r, "token"), IdentitySystemID: str(r, "identity_system_id"), PairedAt: time.Now().UTC()}
	s.record(err == nil && st == 200 && len(pf.Token) == 64, "2 pairing completes", "installation "+pf.IdentitySystemID)
	save(pf)
	evlog := filepath.Join(s.data, "logs", "identity-events.jsonl")
	lb, _ := os.ReadFile(evlog)
	s.record(code != "" && !strings.Contains(string(lb), code) && !strings.Contains(string(lb), pf.Token), "2 code and token never logged", "")

	// 3. Identity records and read checks.
	st, e, err := call(pf, "POST", "/api/v1/identities", map[string]string{"name": "Smoke Tester", "kind": "person"})
	entID := str(e, "id")
	s.record(err == nil && st == 201 && fmt.Sprint(e["authorized_sources"]) == "[]", "3 create identity (starts with no access)", entID)
	s.record(runCheck(pf, 0) == 0, "3 signed status / identities / features / authorize=DENIED", "see output above")

	// 4. Shut down on request (person chose Shut down at exit) — clean stop.
	ok := s.shutdownViaAPI(cur, pf, "smoke: person chose Shut down")
	con = cur.console()
	s.record(ok && strings.Contains(con, "Identity stopped."), "4 confirmed shutdown → process exits cleanly", fmt.Sprintf("pid %d", cur.cmd.Process.Pid))
	s.record(!fileExists(svc), "4 service.json removed on clean stop", "")
	lb, _ = os.ReadFile(evlog)
	s.record(strings.Contains(string(lb), "smoke: person chose Shut down"), "4 stop reason in Identity's log", "")
	cur = nil

	// 5. Restart → reconnect with saved pairing, records kept.
	cur = must(s.startReady("run2"))
	st, l, err := call(pf, "GET", "/api/v1/identities", nil)
	s.record(err == nil && st == 200 && strings.Contains(fmt.Sprint(l), entID), "5 restart: reconnects without new code, identity kept", "")
	s.record(strings.Contains(cur.console(), "paired  : 1"), "5 console shows existing pairing", "")

	// 6. Processor-first: client waits while Identity is down, then connects.
	s.shutdownViaAPI(cur, pf, "smoke: stop before processor-first test")
	cur = nil
	started := make(chan *proc, 1)
	go func() {
		time.Sleep(5 * time.Second)
		p, err := s.startReady("run3")
		if err != nil {
			fmt.Println("start failed:", err)
		}
		started <- p
	}()
	rc := runCheck(pf, 40*time.Second)
	cur = <-started
	s.record(rc == 0 && cur != nil, "6 Processor-first: waits, then verified connection", "client started first; Identity 5 s later")

	// 7. Identity down → AUTHORITY UNAVAILABLE (fail closed).
	s.shutdownViaAPI(cur, pf, "smoke: stop for outage test")
	cur = nil
	s.record(runCheck(pf, 0) == 2, "7 Identity down → AUTHORITY UNAVAILABLE", "a real Processor must block protected actions")

	// 8. A different Identity installation on the same port = wrong service.
	other, err := s.start("other", filepath.Join(s.work, "other-profile"), s.port)
	if err == nil {
		time.Sleep(time.Second)
		st, _, _ := call(pf, "GET", "/api/v1/status", nil)
		s.record(st == 401, "8 other installation refuses our pairing", fmt.Sprintf("HTTP %d", st))
		other.cmd.Process.Kill()
		other.waitExit(5 * time.Second)
	} else {
		s.record(false, "8 other installation", err.Error())
	}

	// 9. Second Identity on the same profile is refused (lock).
	cur = must(s.startReady("run4"))
	dup, err := s.start("duplicate", s.data, s.port+1)
	if err == nil {
		exited, _ := dup.waitExit(5 * time.Second)
		s.record(exited && strings.Contains(dup.console(), "profile in use"), "9 second Identity on same profile refused", "")
		if !exited {
			dup.cmd.Process.Kill()
		}
	}

	// 10. Unpair → token refused.
	st, _, err = call(pf, "POST", "/api/v1/unpair", map[string]string{})
	st2, _, _ := call(pf, "GET", "/api/v1/status", nil)
	s.record(err == nil && st == 200 && st2 == 401, "10 unpair → old token refused", "")

	// 11. Crash-style stop (window closed / killed), then start again.
	cur.cmd.Process.Kill()
	cur.waitExit(5 * time.Second)
	cur = nil
	stale := fileExists(svc)
	p, err := s.startReady("run5")
	s.record(err == nil, "11 starts again after a crash-style stop", fmt.Sprintf("stale service.json present before restart: %v (replaced on start)", stale))
	if p != nil {
		b, _ = os.ReadFile(svc)
		s.record(strings.Contains(string(b), `"pid": `+strconv.Itoa(p.cmd.Process.Pid)), "11 service.json now names the new process", "")
		cur = p
	}
	s.report(string(ver))
}

func (s *smokeRun) report(ver string) {
	for _, p := range s.live { // never leave test processes behind
		select {
		case <-p.exited:
		default:
			p.cmd.Process.Kill()
			<-p.exited
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "IDENTITY SMOKE REPORT  %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "identity: %s\nos/arch: %s/%s\nGitHub: not used (fixture-free; real sign-in is a separate, human step)\n\n", strings.TrimSpace(ver), runtime.GOOS, runtime.GOARCH)
	for _, r := range s.results {
		b.WriteString(r + "\n")
	}
	// Leave nothing private in the work folder: mask the troubleshooting
	// link and pairing codes in console captures, delete the probe pairing.
	key := regexp.MustCompile(`#key=[0-9a-f]+`)
	if caps, _ := filepath.Glob(filepath.Join(s.work, "*.console.txt")); caps != nil {
		for _, c := range caps {
			if b, err := os.ReadFile(c); err == nil {
				b = key.ReplaceAll(b, []byte("#key=<removed>"))
				b = smokeCode.ReplaceAll(b, []byte("XXXX-XXXX"))
				os.WriteFile(c, b, 0o600)
			}
		}
	}
	os.Remove(statePath)
	// Killed (crash-style) runs leave their private link behind; remove it.
	if links, _ := filepath.Glob(filepath.Join(s.work, "*", "runtime", "open-link.txt")); links != nil {
		for _, l := range links {
			os.Remove(l)
		}
	}
	result := "PASS"
	if s.failed > 0 {
		result = fmt.Sprintf("FAIL (%d)", s.failed)
	}
	fmt.Fprintf(&b, "\nSMOKE RESULT: %s\n", result)
	path := filepath.Join(s.work, "smoke-report.txt")
	os.WriteFile(path, []byte(b.String()), 0o600)
	fmt.Println()
	fmt.Print(b.String())
	fmt.Println("report saved:", path, "(contains no codes or tokens)")
	if s.failed > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}
