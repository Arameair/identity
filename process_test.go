//go:build !windows

package main

// End to end with the real program: Play (start), create, Stop (SIGTERM),
// Play again, same identity; a second instance on its own data folder; a
// second instance on the SAME folder is refused.
import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "identity")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func freePort(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
}

type proc struct {
	cmd  *exec.Cmd
	port string
	log  *strings.Builder
	done chan error
}

func start(t *testing.T, bin, pcData, branch, port string) *proc {
	t.Helper()
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "PC_DATA="+pcData, "PC_BRANCH="+branch, "PORT="+port, "IDENTITY_DATA=")
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	p := &proc{cmd: cmd, port: port, log: &strings.Builder{}, done: make(chan error, 1)}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			p.log.WriteString(sc.Text() + "\n")
		}
		p.done <- cmd.Wait()
	}()
	for i := 0; i < 100; i++ {
		if r, err := http.Get("http://127.0.0.1:" + port + "/api/status"); err == nil {
			r.Body.Close()
			return p
		}
		select {
		case err := <-p.done:
			t.Fatalf("exited early: %v\n%s", err, p.log)
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatalf("never came up\n%s", p.log)
	return nil
}

func (p *proc) stop(t *testing.T) {
	p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not stop within 5 s")
	}
	if !strings.Contains(p.log.String(), "stopped cleanly") {
		t.Fatalf("no clean stop:\n%s", p.log)
	}
}

func create(t *testing.T, port, name, typ string) map[string]any {
	body := fmt.Sprintf(`{"display_name":%q,"entity_type":%q}`, name, typ)
	r, err := http.Post("http://127.0.0.1:"+port+"/api/identities", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	json.NewDecoder(r.Body).Decode(&out)
	if r.StatusCode != 201 {
		t.Fatalf("create %s: %d %v", name, r.StatusCode, out)
	}
	return out["identity"].(map[string]any)
}

func list(t *testing.T, port string) []map[string]any {
	r, err := http.Get("http://127.0.0.1:" + port + "/api/identities")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out struct{ Identities []map[string]any }
	json.NewDecoder(r.Body).Decode(&out)
	return out.Identities
}

func TestRealProcessStopAndRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the binary")
	}
	bin := buildBinary(t)
	pcA, pcB := t.TempDir(), t.TempDir()
	port := freePort(t)

	// A + B
	p := start(t, bin, pcA, "experiment/identity-local-v1-20261023", port)
	tass := create(t, port, "TASS", "ai_companion")
	// a second copy on the same data folder is refused
	clash := exec.Command(bin)
	clash.Env = append(os.Environ(), "PC_DATA="+pcA, "PORT="+freePort(t), "IDENTITY_DATA=")
	if out, err := clash.CombinedOutput(); err == nil || !strings.Contains(string(out), "already using data folder") {
		t.Fatalf("same-folder second instance: %v %s", err, out)
	}
	p.stop(t)
	if _, err := os.Stat(filepath.Join(pcA, "identity", "identity.lock")); !os.IsNotExist(err) {
		t.Fatal("lock left behind after Stop")
	}
	if c, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 200*time.Millisecond); err == nil {
		c.Close()
		t.Fatal("port still open after Stop")
	}

	// C: Play again — same TASS
	p = start(t, bin, pcA, "experiment/identity-local-v1-20261023", port)
	got := list(t, port)
	if len(got) != 1 || got[0]["id"] != tass["id"] || got[0]["display_name"] != "TASS" || got[0]["created_at"] != tass["created_at"] || got[0]["entity_type"] != "ai_companion" {
		t.Fatalf("restore: %v vs %v", got, tass)
	}
	if !strings.Contains(p.log.String(), "loaded 1 saved identity") || !strings.Contains(p.log.String(), tass["id"].(string)) {
		t.Fatalf("restore not logged:\n%s", p.log)
	}
	// D
	biz := create(t, port, "Example Workshop", "business")
	if biz["id"] == tass["id"] || len(list(t, port)) != 2 {
		t.Fatal("business identity")
	}

	// G: another instance with its own data folder, running at the same time
	portB := freePort(t)
	q := start(t, bin, pcB, "experiment/other", portB)
	create(t, portB, "Other Test", "project")
	if len(list(t, portB)) != 1 || len(list(t, port)) != 2 {
		t.Fatal("instances shared data")
	}
	q.stop(t)
	if len(list(t, port)) != 2 {
		t.Fatal("stopping B affected A")
	}
	p.stop(t)
	p = start(t, bin, pcA, "experiment/identity-local-v1-20261023", port)
	if got := list(t, port); len(got) != 2 || got[0]["id"] != tass["id"] || got[1]["id"] != biz["id"] {
		t.Fatalf("final restore: %v", got)
	}
	p.stop(t)
	t.Logf("A log:\n%s", p.log)
}
