//go:build !windows

package boundary

// Runs the real WSL probe script (wslScript) under the local POSIX sh with a
// stub wslpath, so the shell half of F2 is exercised deterministically without
// WSL. This is NOT WSL/drvfs evidence: the mount is the test machine's own
// filesystem, so every result here also carries WSL_SCOPE_UNVERIFIED.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type scriptRun struct {
	out    string
	att    []Attempt
	fs     []Finding
	tested WSLTested
}

// runScript runs wslScript with wslpath translating to posixRoot (or failing when posixRoot == "").
func runScript(t *testing.T, posixRoot string, files []string) scriptRun {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	bin := t.TempDir()
	stub := "#!/bin/sh\necho \"wslpath: $1: No such file or directory\" >&2\nexit 1\n"
	if posixRoot != "" {
		stub = "#!/bin/sh\n[ \"$1\" = -u ] || exit 2\nprintf '%s\\n' \"$WSLPATH_TARGET\"\n"
	}
	if err := os.WriteFile(filepath.Join(bin, "wslpath"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	p := EntityProfile(entityRoot, svc, normal)
	targets, err := wslTargets(entityRoot, p, files)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-c", wslScript, "probe", entityRoot}
	for i := range targets {
		targets[i].Exists = ExistsYes
		if i > 0 {
			args = append(args, targets[i].Rel)
		}
	}
	cmd := exec.Command(sh, args...)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WSLPATH_TARGET="+posixRoot, "WSL_DISTRO_NAME=TestDistro")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	att, fs, tested := evaluateWSL(string(out), targets)
	return scriptRun{string(out), att, fs, tested}
}

func result(r scriptRun, path, right string) string {
	for _, a := range r.att {
		if strings.EqualFold(a.Path, path) && a.Right == "wsl "+right {
			return strings.SplitN(a.Result, " ", 2)[0]
		}
	}
	return "MISSING"
}

func entityFiles(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	d := t.TempDir()
	content := map[string][]byte{}
	for _, f := range EntityAuthorityFiles {
		b := []byte("original " + f + "\n")
		content[f] = b
		if err := os.WriteFile(filepath.Join(d, f), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return d, content
}

func assertUntouched(t *testing.T, d string, content map[string][]byte) {
	t.Helper()
	ents, _ := os.ReadDir(d)
	if len(ents) != len(content) {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("probe changed the directory listing: %v", names)
	}
	for f, want := range content {
		got, err := os.ReadFile(filepath.Join(d, f))
		if err != nil || string(got) != string(want) {
			t.Fatalf("probe changed %s: %q %v", f, got, err)
		}
	}
}

// Accessible files are ALLOWED (a violation) and the probe changes nothing.
func TestWSLScriptAllowedIsViolationAndNonDestructive(t *testing.T) {
	d, content := entityFiles(t)
	r := runScript(t, d, EntityAuthorityFiles)
	if verdictOf(r.fs) != NotIsolated || !codes(r.fs)[CWSLAllowed] {
		t.Fatalf("verdict %s:%s\n%s", verdictOf(r.fs), dumpF(r.fs), r.out)
	}
	for _, right := range []string{"list", "create"} {
		if got := result(r, entityRoot, right); got != "ALLOWED" {
			t.Errorf("root %s: %s\n%s", right, got, r.out)
		}
	}
	if got := result(r, entityRoot+`\entity.json`, "write"); got != "ALLOWED" {
		t.Errorf("write: %s\n%s", got, r.out)
	}
	if !codes(r.fs)[CWSLScope] || r.tested.Distro != "TestDistro" || r.tested.Path != d {
		t.Errorf("the tested translation must be reported, and a non-drvfs mount is UNVERIFIED: %+v%s", r.tested, dumpF(r.fs))
	}
	assertUntouched(t, d, content)
}

// F2: a missing file is ABSENT (UNVERIFIED), never denied, and is not created.
func TestWSLScriptMissingFileIsUnverifiedAndNotCreated(t *testing.T) {
	d, content := entityFiles(t)
	os.Remove(filepath.Join(d, "credential"))
	delete(content, "credential")
	r := runScript(t, d, EntityAuthorityFiles)
	for _, right := range []string{"read", "write"} {
		if got := result(r, entityRoot+`\credential`, right); got != "ABSENT" {
			t.Errorf("credential %s: %s\n%s", right, got, r.out)
		}
	}
	if !codes(r.fs)[CWSLAbsent] {
		t.Errorf("missing file must be reported:%s", dumpF(r.fs))
	}
	assertUntouched(t, d, content)
}

// F2: a missing or unmounted root (wslpath succeeds, nothing is there) is ABSENT, never denied.
func TestWSLScriptMissingRootIsUnverified(t *testing.T) {
	r := runScript(t, filepath.Join(t.TempDir(), "not-mounted", "TassIdentity"), EntityAuthorityFiles)
	if verdictOf(r.fs) != Unverified || !codes(r.fs)[CWSLAbsent] {
		t.Fatalf("verdict %s:%s\n%s", verdictOf(r.fs), dumpF(r.fs), r.out)
	}
	for _, a := range r.att {
		if a.Result == "DENIED" || a.Result == "ALLOWED" {
			t.Errorf("missing root produced %+v", a)
		}
	}
}

// F2: a failed wslpath translation is UNVERIFIED with no attempts.
func TestWSLScriptFailedWslpathIsUnverified(t *testing.T) {
	r := runScript(t, "", EntityAuthorityFiles)
	if verdictOf(r.fs) != Unverified || !codes(r.fs)[CWSLUnverified] || len(r.att) != 0 || !strings.Contains(r.out, "NOPATH") {
		t.Fatalf("verdict %s:%s\n%s", verdictOf(r.fs), dumpF(r.fs), r.out)
	}
}

// F2: denied directory traversal yields explicit DENIED lines (and, here, only
// the scope finding, because the test filesystem is not drvfs). Root ignores
// mode bits, so this case needs a non-root user (it runs on hosted Ubuntu).
func TestWSLScriptDeniedTraversalIsExplicitlyDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits are not enforced (run as a normal user to exercise denial)")
	}
	d, _ := entityFiles(t)
	if err := os.Chmod(d, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(d, 0o700)
	r := runScript(t, d, EntityAuthorityFiles)
	for _, a := range r.att {
		if a.Result != "DENIED" {
			t.Errorf("%+v\n%s", a, r.out)
		}
	}
	for _, f := range r.fs {
		if f.Code != CWSLScope {
			t.Errorf("unexpected finding %s %s", f.Code, f.Detail)
		}
	}
	if verdictOf(r.fs) != Unverified {
		t.Errorf("a non-drvfs mount must keep the result UNVERIFIED: %s", verdictOf(r.fs))
	}
}
