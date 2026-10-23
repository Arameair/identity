package main

// TestFirstUserEndToEnd runs the REAL Identity and Thought Controller programs
// together, the way Project Controller starts them (PORT, PC_BRANCH, PC_DATA),
// and walks the new-administrator flow through their HTTP APIs.
//
// It needs a Thought Controller checkout:
//
//	THOUGHT_CONTROLLER_SRC=/path/to/Thought-Controller go test -run FirstUser -v
//
// Without that variable it is skipped.
import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type proc struct {
	cmd *exec.Cmd
	out *bytes.Buffer
}

func startProc(t *testing.T, dir, bin, port, branch, data string) *proc {
	t.Helper()
	cmd := exec.Command(bin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PORT="+port, "PC_BRANCH="+branch, "PC_DATA="+data, "PC_NO_BROWSER=1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if r, err := http.Get("http://127.0.0.1:" + port + "/"); err == nil {
			r.Body.Close()
			return &proc{cmd, &out}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s did not start:\n%s", bin, out.String())
	return nil
}

func (p *proc) stop() {
	p.cmd.Process.Signal(syscall.SIGTERM)
	p.cmd.Wait()
}

func getJSON(t *testing.T, url string) map[string]any {
	t.Helper()
	r, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var m map[string]any
	json.NewDecoder(r.Body).Decode(&m)
	return m
}

func postJSON2(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	r, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	raw, _ := io.ReadAll(r.Body)
	var m map[string]any
	json.Unmarshal(raw, &m)
	return r.StatusCode, m
}

func dig(m map[string]any, path ...string) any {
	var v any = m
	for _, k := range path {
		mm, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = mm[k]
	}
	return v
}

func TestFirstUserEndToEnd(t *testing.T) {
	src := os.Getenv("THOUGHT_CONTROLLER_SRC")
	if src == "" {
		t.Skip("set THOUGHT_CONTROLLER_SRC to a Thought Controller checkout to run the two-program test")
	}
	if runtime.GOOS == "windows" {
		t.Skip("uses SIGTERM")
	}
	tmp := t.TempDir()
	idBin, tcBin := filepath.Join(tmp, "identity"), filepath.Join(tmp, "thought-controller")
	if out, err := exec.Command("go", "build", "-o", idBin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build identity: %v %s", err, out)
	}
	b := exec.Command("go", "build", "-o", tcBin, ".")
	b.Dir = src
	if out, err := b.CombinedOutput(); err != nil {
		t.Fatalf("build thought controller: %v %s", err, out)
	}
	tcCommit, _ := exec.Command("git", "-C", src, "rev-parse", "HEAD").Output()
	idPort, tcPort := freeTestPort(t), freeTestPort(t)
	ID, TC := "http://127.0.0.1:"+idPort, "http://127.0.0.1:"+tcPort
	idData, tcData := filepath.Join(tmp, "id.data"), filepath.Join(tmp, "tc.data")
	wd, _ := os.Getwd()

	idp := startProc(t, wd, idBin, idPort, "experiment/e2e-identity", idData)
	defer func() { idp.stop() }()
	if st := getJSON(t, ID+"/api/status"); st["installation"] != nil {
		t.Fatal("N1: new profile must have no installation")
	}
	code, r := postJSON2(t, ID+"/api/install", map[string]string{"admin_name": "E2E admin"})
	if code != 201 {
		t.Fatalf("install: %d %v", code, r)
	}
	inst := r["installation"].(map[string]any)
	postJSON2(t, ID+"/api/identities/starter", map[string]any{})
	postJSON2(t, ID+"/api/controller/address", map[string]string{"address": "127.0.0.1:" + tcPort})
	_, c := postJSON2(t, ID+"/api/controller/check", map[string]any{"trigger": "e2e", "allow_start": true})
	if c["state"] != "NOT_FOUND" {
		t.Fatalf("N3: expected NOT_FOUND, got %v", c["state"])
	}

	tcp := startProc(t, src, tcBin, tcPort, "experiment/e2e-tc", tcData)
	defer func() { tcp.stop() }()
	// Pin exactly the build under test (an explicit owner choice in the UI).
	if code, r := postJSON2(t, ID+"/api/controller/target", map[string]any{"branch": "e2e", "commit": strings.TrimSpace(string(tcCommit))}); code != 200 {
		t.Fatalf("pin: %d %v", code, r)
	}
	_, c = postJSON2(t, ID+"/api/controller/check", map[string]any{"trigger": "e2e", "allow_start": true})
	if c["state"] != "NOT_RUNNING" || c["attempt"] != nil {
		t.Fatalf("no consent yet: %v", c)
	}
	_, r = postJSON2(t, ID+"/api/controller/consent", map[string]string{"decision": "approve"})
	if dig(r, "check", "state") != "VERIFIED" || dig(r, "check", "attempt", "result") != "started" {
		t.Fatalf("N5: approve should start once and verify: %v", r)
	}
	h := getJSON(t, TC+"/api/health")
	if dig(h, "service", "service_id") != inst["controller_service_id"] || dig(h, "counters", "start_requests") != 1.0 || dig(h, "counters", "model_calls") != 0.0 {
		t.Fatalf("controller should be bound to Identity's service ID after one request: %v", h)
	}
	ws := getJSON(t, TC+"/api/workspace")
	ents := dig(ws, "workspace", "entities").([]any)
	if len(ents) != 2 {
		t.Fatalf("TASS and Example Workshop should be registered: %v", ents)
	}
	tass := ents[0].(map[string]any)["entity_id"].(string)
	if code, _ := postJSON2(t, TC+"/api/entities/"+tass+"/thoughts", map[string]string{"title": "e2e thought", "state": "sleeping"}); code != 201 {
		t.Fatal("create thought")
	}

	// Stop everything; start again (controller first, like a normal Play order).
	idp.stop()
	tcp.stop()
	tcp = startProc(t, src, tcBin, tcPort, "experiment/e2e-tc", tcData)
	idp = startProc(t, wd, idBin, idPort, "experiment/e2e-identity", idData)
	var st map[string]any
	for i := 0; i < 40; i++ { // Identity's single startup check runs in the background
		st = getJSON(t, ID+"/api/status")
		if dig(st, "controller", "last_check", "state") == "VERIFIED" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if dig(st, "installation", "installation_id") != inst["installation_id"] || dig(st, "installation", "controller_service_id") != inst["controller_service_id"] {
		t.Fatal("N2/N15: IDs changed across restart")
	}
	if dig(st, "controller", "last_check", "state") != "VERIFIED" || dig(st, "controller", "consent_valid") != true {
		t.Fatalf("N6: saved consent should restart the controller without asking: %v", dig(st, "controller", "last_check"))
	}
	h = getJSON(t, TC+"/api/health")
	if dig(h, "counters", "start_requests") != 1.0 {
		t.Fatalf("N6: exactly one start request after restart, got %v", dig(h, "counters"))
	}
	r2 := getJSON(t, TC+"/api/entities/"+tass+"/thoughts")
	if l := r2["thoughts"].([]any); len(l) != 1 || l[0].(map[string]any)["title"] != "e2e thought" {
		t.Fatal("T9: thought not restored after restart")
	}
	// Identity restarts while the controller keeps running: reuse, no request.
	idp.stop()
	idp = startProc(t, wd, idBin, idPort, "experiment/e2e-identity", idData)
	time.Sleep(500 * time.Millisecond)
	if h = getJSON(t, TC+"/api/health"); dig(h, "counters", "start_requests") != 1.0 {
		t.Fatal("N7: running controller must be reused without a start request")
	}

	// Second installation: fresh profile, different IDs, wrong-instance until A stops.
	_, r = postJSON2(t, ID+"/api/profiles/new", map[string]any{})
	_, r = postJSON2(t, ID+"/api/install", map[string]string{"admin_name": "Second"})
	instB := r["installation"].(map[string]any)
	if instB["installation_id"] == inst["installation_id"] || instB["controller_service_id"] == inst["controller_service_id"] {
		t.Fatal("N16: second installation must get new IDs")
	}
	postJSON2(t, ID+"/api/identities/starter", map[string]any{})
	postJSON2(t, ID+"/api/controller/address", map[string]string{"address": "127.0.0.1:" + tcPort})
	_, c = postJSON2(t, ID+"/api/controller/check", map[string]any{"trigger": "e2e", "allow_start": true})
	if c["state"] != "WRONG_INSTANCE" {
		t.Fatalf("N8: expected WRONG_INSTANCE, got %v", c["state"])
	}
	postJSON2(t, TC+"/api/sim/lifecycle/stop", map[string]string{"installation_id": inst["installation_id"].(string)})
	postJSON2(t, ID+"/api/controller/target", map[string]any{"use_running": true})
	_, r = postJSON2(t, ID+"/api/controller/consent", map[string]string{"decision": "approve"})
	if dig(r, "check", "state") != "VERIFIED" {
		t.Fatalf("B start: %v", r)
	}
	ws = getJSON(t, TC+"/api/workspace")
	for _, e := range dig(ws, "workspace", "entities").([]any) {
		em := e.(map[string]any)
		if em["entity_id"] == tass || em["total"] != 0.0 {
			t.Fatal("T12: second installation must not see A's identities or thoughts")
		}
	}
	diag, _ := http.Get(ID + "/api/diagnostics")
	db, _ := io.ReadAll(diag.Body)
	diag.Body.Close()
	if strings.Contains(string(db), "E2E admin") || !strings.Contains(string(db), instB["controller_service_id"].(string)) {
		t.Fatal("N18: diagnostics must carry IDs and no names")
	}
}

func freeTestPort(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	a := l.Addr().String()
	return a[strings.LastIndex(a, ":")+1:]
}
