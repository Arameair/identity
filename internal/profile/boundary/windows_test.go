//go:build windows

package boundary

// Two kinds of Windows tests.
//
//  1. Always-on, UNPRIVILEGED negatives against the real OS. They only create
//     temp directories, a junction and a hard link under t.TempDir(). They never
//     change an ACL, owner, account, service or anything outside the temp dir.
//
//  2. OPT-IN acceptance harness for a DISPOSABLE, separately protected profile
//     that an operator prepared beforehand. It is skipped unless every variable
//     below is set. It is read-only (see Probe) and never configures anything.
//
//     IDENTITY_D2_ACCEPTANCE   service | normal | wsl   (which half to run, as which account)
//     IDENTITY_D2_DISPOSABLE   must be exactly: I-UNDERSTAND-THIS-IS-A-DISPOSABLE-TEST-PROFILE
//     IDENTITY_D2_KIND         admin | entity
//     IDENTITY_D2_ROOT         e.g. C:\ProgramData\Arameair\D2Test\admin
//     IDENTITY_D2_SERVICE_SID  the protected account's SID
//     IDENTITY_D2_NORMAL_SID   the owner-style normal account's SID
//
//     service: run AS the protected account  → Check must be ISOLATED
//     normal:  run AS the unelevated normal user → Probe must be ISOLATED
//     wsl:     run AS the unelevated normal user → ProbeWSL must be ISOLATED

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeOther = "S-1-5-21-1000000001-2000000002-3000000003-1999"

func currentSID(t *testing.T) string {
	t.Helper()
	p, err := NewInspector().Process()
	if err != nil || !strings.HasPrefix(p.UserSID, "S-1-") {
		t.Fatalf("process identity: %+v %v", p, err)
	}
	return p.UserSID
}

func tempProfile(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "profile.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return d
}

func logReport(t *testing.T, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	t.Logf("%s", b)
}

func TestWindowsRealDescriptorParsing(t *testing.T) {
	sys := filepath.Join(os.Getenv("SystemRoot"), "System32")
	o, err := NewInspector().Inspect(sys)
	if err != nil {
		t.Fatal(err)
	}
	if !o.IsDir || o.Reparse || o.Descriptor == nil || !o.Descriptor.DACLPresent || len(o.Descriptor.DACL) == 0 {
		t.Fatalf("unexpected System32 object: %+v", o)
	}
	if !strings.EqualFold(o.Descriptor.Owner, SIDTrustedInstall) {
		t.Logf("note: System32 owner is %s (TrustedInstaller expected on stock Windows)", o.Descriptor.Owner)
	}
	if o.Volume != VolumeFixed || o.FinalPath == "" {
		t.Fatalf("volume %s final %q", o.Volume, o.FinalPath)
	}
}

// A normal temp folder (inherits its DACL; the user owns its ancestors) is never isolated.
func TestWindowsTempProfileIsNotIsolated(t *testing.T) {
	me := currentSID(t)
	if !strings.HasPrefix(me, "S-1-5-21-") {
		t.Skipf("runs as %s, not a local/domain account", me)
	}
	r := Check(NewInspector(), AdminProfile(tempProfile(t), me, fakeOther))
	logReport(t, r)
	if r.Passed() || r.Verdict != NotIsolated || !has(r, CRootInherits) {
		t.Fatalf("verdict %s:%s", r.Verdict, dump(r))
	}
}

// Running the service-side check as the designated normal user is refused.
func TestWindowsCheckAsNormalUserRefused(t *testing.T) {
	me := currentSID(t)
	if !strings.HasPrefix(me, "S-1-5-21-") {
		t.Skipf("runs as %s", me)
	}
	r := Check(NewInspector(), AdminProfile(tempProfile(t), svc, me))
	if r.Verdict != NotIsolated || !has(r, CProcessIsNormal) {
		t.Fatalf("verdict %s:%s", r.Verdict, dump(r))
	}
}

// A junction substituted for the root is detected (no admin rights needed for mklink /J).
func TestWindowsJunctionRootDetected(t *testing.T) {
	me := currentSID(t)
	if !strings.HasPrefix(me, "S-1-5-21-") {
		t.Skipf("runs as %s", me)
	}
	real := tempProfile(t)
	link := filepath.Join(t.TempDir(), "link")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
		t.Skipf("mklink /J unavailable: %v %s", err, out)
	}
	r := Check(NewInspector(), AdminProfile(link, me, fakeOther))
	logReport(t, r)
	if r.Verdict != NotIsolated || !has(r, CReparse) {
		t.Fatalf("verdict %s:%s", r.Verdict, dump(r))
	}
}

// A second hard link to an authority file is detected.
func TestWindowsHardLinkDetected(t *testing.T) {
	me := currentSID(t)
	if !strings.HasPrefix(me, "S-1-5-21-") {
		t.Skipf("runs as %s", me)
	}
	d := tempProfile(t)
	if err := os.Link(filepath.Join(d, "profile.json"), filepath.Join(t.TempDir(), "alias.json")); err != nil {
		t.Skipf("hard link unavailable: %v", err)
	}
	r := Check(NewInspector(), AdminProfile(d, me, fakeOther))
	if r.Verdict != NotIsolated || !has(r, CHardLink) {
		t.Fatalf("verdict %s:%s", r.Verdict, dump(r))
	}
}

// The normal-user probe against a folder the user owns must find allowed access.
func TestWindowsProbeFindsAccessOnUnprotectedFolder(t *testing.T) {
	me := currentSID(t)
	if !strings.HasPrefix(me, "S-1-5-21-") {
		t.Skipf("runs as %s", me)
	}
	r := Probe(AdminProfile(tempProfile(t), svc, me), []string{"profile.json"})
	logReport(t, r)
	if r.Verdict != NotIsolated {
		t.Fatalf("verdict %s", r.Verdict)
	}
	ok := false
	for _, f := range r.Findings {
		ok = ok || f.Code == CProbeAllowed
	}
	if !ok {
		t.Fatal("expected NORMAL_USER_ALLOWED findings")
	}
	for _, a := range r.Attempts {
		if strings.Contains(a.Path, ".boundary-probe-") {
			if _, err := os.Stat(a.Path); err == nil {
				t.Fatal("probe left its create-attempt file behind")
			}
		}
	}
}

// ---- opt-in acceptance harness ---------------------------------------------------------

func TestAcceptanceDisposableProtectedProfile(t *testing.T) {
	mode := os.Getenv("IDENTITY_D2_ACCEPTANCE")
	if mode == "" {
		t.Skip("opt-in: set IDENTITY_D2_ACCEPTANCE and the other IDENTITY_D2_* variables (see file header)")
	}
	if os.Getenv("IDENTITY_D2_DISPOSABLE") != "I-UNDERSTAND-THIS-IS-A-DISPOSABLE-TEST-PROFILE" {
		t.Fatal("refusing: IDENTITY_D2_DISPOSABLE confirmation missing; never point this at a live profile")
	}
	root, svcSID, normSID := os.Getenv("IDENTITY_D2_ROOT"), os.Getenv("IDENTITY_D2_SERVICE_SID"), os.Getenv("IDENTITY_D2_NORMAL_SID")
	var p Policy
	var files []string
	switch os.Getenv("IDENTITY_D2_KIND") {
	case "admin":
		p, files = AdminProfile(root, svcSID, normSID), AdminAuthorityFiles
	case "entity":
		p, files = EntityProfile(root, svcSID, normSID), EntityAuthorityFiles
	default:
		t.Fatal("IDENTITY_D2_KIND must be admin or entity")
	}
	var verdict Verdict
	switch mode {
	case "service":
		r := Check(NewInspector(), p)
		logReport(t, r)
		verdict = r.Verdict
	case "normal":
		r := Probe(p, files)
		logReport(t, r)
		verdict = r.Verdict
	case "wsl":
		r := ProbeWSL(p, files)
		logReport(t, r)
		verdict = r.Verdict
	default:
		t.Fatal("IDENTITY_D2_ACCEPTANCE must be service, normal or wsl")
	}
	if verdict != Isolated {
		t.Fatalf("%s half: %s (fail closed)", mode, verdict)
	}
	t.Logf("%s half ISOLATED for %s — file/ACL scope only; see not_covered", mode, root)
}
