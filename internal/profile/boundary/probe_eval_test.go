package boundary

import (
	"fmt"
	"strings"
	"testing"
)

func codes(fs []Finding) map[Code]bool {
	m := map[Code]bool{}
	for _, f := range fs {
		m[f.Code] = true
	}
	return m
}

func dumpF(fs []Finding) string {
	var b strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&b, "\n  %s v=%v %s %s", f.Code, f.Violation, f.Path, f.Detail)
	}
	return b.String()
}

// ---- F1: the caller gate shared by Probe and ProbeWSL ----------------------------------

func TestEvaluateCallerGate(t *testing.T) {
	p := AdminProfile(adminRoot, svc, normal)
	clean := CallerToken{UserSID: normal}
	cases := []struct {
		name    string
		tok     CallerToken
		ok      bool
		want    Verdict
		mustHas Code
	}{
		{"designated standard user", clean, true, Isolated, ""},
		{"designated user, SID case differs", CallerToken{UserSID: strings.ToLower(normal)}, true, Isolated, ""},
		{"unknown token user", CallerToken{}, false, Unverified, CProcessQuery},
		{"running as the service account", CallerToken{UserSID: svc}, false, Unverified, CProbeIsService},
		{"running as some other user", CallerToken{UserSID: other}, false, Unverified, CProcessWrong},
		{"running as SYSTEM", CallerToken{UserSID: SIDSystem}, false, Unverified, CProcessWrong},
		{"elevated", CallerToken{UserSID: normal, Elevated: true}, true, NotIsolated, CNormalIsAdmin},
		{"linked admin (UAC split)", CallerToken{UserSID: normal, LinkedAdmin: true}, true, NotIsolated, CNormalIsAdmin},
		{"Administrators enabled", CallerToken{UserSID: normal, AdminGroup: "enabled"}, true, NotIsolated, CNormalIsAdmin},
		{"Administrators deny-only", CallerToken{UserSID: normal, AdminGroup: "deny-only (UAC-filtered)"}, true, NotIsolated, CNormalIsAdmin},
		{"backup privilege", CallerToken{UserSID: normal, Privileges: []string{"SeBackupPrivilege"}}, true, NotIsolated, CPrivileged},
		{"token field unreadable", CallerToken{UserSID: normal, QueryErrors: []string{"elevation: x"}}, true, Unverified, CProcessQuery},
	}
	for _, c := range cases {
		fs, ok := evaluateCaller(p, c.tok)
		if ok != c.ok || verdictOf(fs) != c.want || (c.mustHas != "" && !codes(fs)[c.mustHas]) {
			t.Errorf("%s: ok=%v verdict=%s, want ok=%v %s %s:%s", c.name, ok, verdictOf(fs), c.ok, c.want, c.mustHas, dumpF(fs))
		}
	}
}

// ---- F2/F3: evaluation of the WSL script output ----------------------------------------

func testTargets(exists Existence) []WSLTarget {
	t, err := wslTargets(entityRoot, EntityProfile(entityRoot, svc, normal), EntityAuthorityFiles)
	if err != nil {
		panic(err)
	}
	for i := range t {
		t[i].Exists = exists
	}
	return t
}

// wslOut builds script output; status(i, right) gives each attempt's line ("" = omit it).
func wslOut(header string, n int, status func(i int, right string) string) string {
	var b strings.Builder
	b.WriteString("BEGIN\n" + header)
	for i := 0; i < n; i++ {
		rights := wslFileRights
		if i == 0 {
			rights = wslRootRights
		}
		for _, r := range rights {
			if s := status(i, r); s != "" {
				fmt.Fprintf(&b, "R %d %s %s\n", i, r, s)
			}
		}
	}
	b.WriteString("DONE\n")
	return b.String()
}

const goodHeader = "DISTRO Ubuntu-24.04\nWSLPATH /mnt/c/ProgramData/Arameair/TassIdentity\nMOUNT 9p C:\\134 /mnt/c\n"

func allDenied(int, string) string { return "DENIED ls: cannot open directory: Permission denied" }

func TestEvaluateWSL(t *testing.T) {
	n := len(testTargets(ExistsYes))
	cases := []struct {
		name   string
		exists Existence
		out    string
		want   Verdict
		code   Code
	}{
		{"all denied on existing targets, drvfs mount, named distro", ExistsYes, wslOut(goodHeader, n, allDenied), Isolated, ""},
		{"virtiofs mount accepted", ExistsYes, wslOut(strings.Replace(goodHeader, "9p", "virtiofs", 1), n, allDenied), Isolated, ""},
		// F2: absence and silence are never denial.
		{"no output at all (old behaviour: ISOLATED)", ExistsYes, "", Unverified, CWSLUnverified},
		{"no attempt lines (old behaviour: ISOLATED)", ExistsYes, wslOut(goodHeader, n, func(int, string) string { return "" }), Unverified, CWSLUnobserved},
		{"missing file seen from WSL", ExistsYes, wslOut(goodHeader, n, func(i int, r string) string {
			if i == 1 {
				return "ABSENT dd: failed to open: No such file or directory"
			}
			return allDenied(i, r)
		}), Unverified, CWSLAbsent},
		{"missing / unmounted root seen from WSL", ExistsYes, wslOut(goodHeader, n, func(int, string) string {
			return "ABSENT ls: cannot access: No such file or directory"
		}), Unverified, CWSLAbsent},
		{"denied, but Windows says the targets do not exist", ExistsNo, wslOut(goodHeader, n, allDenied), Unverified, CWSLExistence},
		{"denied, but Windows existence unknown", ExistsUnknown, wslOut(goodHeader, n, allDenied), Unverified, CWSLExistence},
		{"failed wslpath", ExistsYes, "BEGIN\nNOPATH wslpath: C:\\x: Invalid argument\nDONE\n", Unverified, CWSLUnverified},
		{"other error (I/O)", ExistsYes, wslOut(goodHeader, n, func(i int, r string) string {
			if r == "read" {
				return "ERROR dd: error reading: Input/output error"
			}
			return allDenied(i, r)
		}), Unverified, CWSLError},
		{"one attempt missing", ExistsYes, wslOut(goodHeader, n, func(i int, r string) string {
			if i == n-1 && r == "write" {
				return ""
			}
			return allDenied(i, r)
		}), Unverified, CWSLUnobserved},
		{"duplicate attempt line", ExistsYes, strings.Replace(wslOut(goodHeader, n, allDenied), "DONE", "R 0 list DENIED x\nDONE", 1), Unverified, CWSLUnobserved},
		{"unknown status word", ExistsYes, wslOut(goodHeader, n, func(i int, r string) string { return "MAYBE" }), Unverified, CWSLUnobserved},
		{"attempt for an unknown target", ExistsYes, strings.Replace(wslOut(goodHeader, n, allDenied), "DONE", "R 99 read DENIED\nDONE", 1), Unverified, CWSLUnobserved},
		{"unrequested attempt", ExistsYes, strings.Replace(wslOut(goodHeader, n, allDenied), "DONE", "R 1 list DENIED\nDONE", 1), Unverified, CWSLUnobserved},
		{"no DONE (killed midway)", ExistsYes, strings.Replace(wslOut(goodHeader, n, allDenied), "DONE\n", "", 1), Unverified, CWSLUnverified},
		{"stray output line", ExistsYes, strings.Replace(wslOut(goodHeader, n, allDenied), "BEGIN\n", "BEGIN\nhello\n", 1), Unverified, CWSLUnobserved},
		// F3: the translation that was tested must be named and must reach NTFS.
		{"distro not named", ExistsYes, wslOut(strings.Replace(goodHeader, "DISTRO Ubuntu-24.04", "DISTRO", 1), n, allDenied), Unverified, CWSLScope},
		{"root on the distro's own ext4", ExistsYes, wslOut("DISTRO Ubuntu\nWSLPATH /home/u/x\nMOUNT ext4 /dev/sdc /\n", n, allDenied), Unverified, CWSLScope},
		{"9p, but mounted at / (not a drive mount)", ExistsYes, wslOut("DISTRO Ubuntu\nWSLPATH /x\nMOUNT 9p rootfs /\n", n, allDenied), Unverified, CWSLScope},
		{"tmpfs mount", ExistsYes, wslOut(strings.Replace(goodHeader, "9p", "tmpfs", 1), n, allDenied), Unverified, CWSLScope},
		{"no mount reported", ExistsYes, wslOut("DISTRO Ubuntu\nWSLPATH /mnt/c/x\n", n, allDenied), Unverified, CWSLScope},
		// Allowed stays a violation, whatever else is uncertain.
		{"write allowed", ExistsYes, wslOut(goodHeader, n, func(i int, r string) string {
			if i == 1 && r == "write" {
				return "ALLOWED"
			}
			return allDenied(i, r)
		}), NotIsolated, CWSLAllowed},
		{"allowed and absent mixed", ExistsYes, wslOut(goodHeader, n, func(i int, r string) string {
			if r == "list" {
				return "ALLOWED"
			}
			return "ABSENT No such file or directory"
		}), NotIsolated, CWSLAllowed},
	}
	for _, c := range cases {
		att, fs, tested := evaluateWSL(c.out, testTargets(c.exists))
		if v := verdictOf(fs); v != c.want || (c.code != "" && !codes(fs)[c.code]) {
			t.Errorf("%s: verdict %s, want %s with %s:%s", c.name, v, c.want, c.code, dumpF(fs))
		}
		if len(tested.NotCovered) == 0 {
			t.Errorf("%s: the not-covered list must always be reported", c.name)
		}
		if c.want == Isolated {
			if len(att) != 2+2*(n-1) || tested.Distro == "" || tested.MountPoint != "/mnt/c" || tested.FSType == "" {
				t.Errorf("%s: an ISOLATED result must name every attempt and the exact translation: %d %+v", c.name, len(att), tested)
			}
			for _, a := range att {
				if a.Result != "DENIED" {
					t.Errorf("%s: %+v", c.name, a)
				}
			}
		}
	}
}

func TestWSLTargetsRejectsUnsafeRelativePaths(t *testing.T) {
	for _, f := range []string{"", "/etc/passwd", `C:\x`, "../x", "a/../../x", "a//b", "./a", "a\x00b", "a\nb"} {
		if _, err := wslTargets(adminRoot, AdminProfile(adminRoot, svc, normal), []string{f}); err == nil {
			t.Errorf("%q accepted", f)
		}
	}
	tg, err := wslTargets(adminRoot, AdminProfile(adminRoot, svc, normal), AdminAuthorityFiles)
	if err != nil || len(tg) != 1+len(AdminAuthorityFiles) || tg[0].Rel != "" {
		t.Fatalf("%v %d", err, len(tg))
	}
}
