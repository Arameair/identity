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

// goodHeader is complete, consistent mount evidence for the root through the
// whole-drive 9p automount of C: at /mnt/c (device 0:70 → st_dev 70), as WSL 2
// reports it. Every F3 negative below perturbs exactly one part of it.
const (
	goodRoot   = "/mnt/c/ProgramData/Arameair/TassIdentity"
	rootMI     = "MI 1 0 8:32 / / rw,relatime - ext4 /dev/sdc rw\n"
	driveMI    = "MI 70 1 0:70 / /mnt/c rw,noatime - 9p C:\\134 rw,aname=drvfs;path=C:\\;uid=1000;gid=1000\n"
	goodHead0  = "DISTRO Ubuntu-24.04\nWSLPATH " + goodRoot + "\nDRIVE /mnt/c/\nREALDRIVE /mnt/c\nREALPARENT /mnt/c/ProgramData/Arameair\n"
	goodDev    = "DEVDRIVE 70\nDEVPARENT 70\n"
	goodHeader = goodHead0 + goodDev + rootMI + driveMI
)

// withMI appends mountinfo lines (mounted after the drive) to goodHeader-style evidence.
func withMI(extra ...string) string { return goodHeader + strings.Join(extra, "") }

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
		{"WSL 1 drvfs automount accepted", ExistsYes, wslOut(goodHead0+goodDev+rootMI+"MI 70 1 0:70 / /mnt/c rw - drvfs C:\\134 rw,case=off\n", n, allDenied), Isolated, ""},
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
		// F3: the translation that was tested must be named and must be the Windows object.
		{"distro not named", ExistsYes, wslOut(strings.Replace(goodHeader, "DISTRO Ubuntu-24.04", "DISTRO", 1), n, allDenied), Unverified, CWSLScope},
		// Codex audit 7060488246, blocking 1 (exact regression): the root's WSL path is
		// under a nested 9p/drvfs mount (here another subtree of C: bound over the
		// root's parent; a bind shares the drive's device, so st_dev matches) and every attempt
		// says DENIED. That is evidence about the overlaid object: never ISOLATED.
		{"nested 9p bind of another C: subtree over the root's parent", ExistsYes, wslOut(goodHeader+
			"MI 91 70 0:70 /Users/u/decoy /mnt/c/ProgramData/Arameair rw - 9p C:\\134 rw,aname=drvfs;path=C:\\\n", n, allDenied), Unverified, CWSLScope},
		{"nested drvfs of another drive at the root itself", ExistsYes, wslOut(withMI("MI 92 70 0:92 / "+goodRoot+" rw - drvfs D:\\134 rw\n"), n, allDenied), Unverified, CWSLScope},
		{"nested 9p mount, st_dev still claims the drive", ExistsYes, wslOut(withMI("MI 93 70 0:93 / /mnt/c/ProgramData rw - 9p C:\\134 rw,aname=drvfs;path=C:\\\n"), n, allDenied), Unverified, CWSLScope},
		{"bind mount over one authority file", ExistsYes, wslOut(withMI("MI 94 70 0:70 /Users/u/x.json "+goodRoot+"/entity.json rw - 9p C:\\134 rw\n"), n, allDenied), Unverified, CWSLScope},
		{"overlay on the root", ExistsYes, wslOut(withMI("MI 95 70 0:95 / "+goodRoot+" rw - overlay overlay rw,lowerdir=/a,upperdir=/b\n"), n, allDenied), Unverified, CWSLScope},
		{"second mount stacked on /mnt/c", ExistsYes, wslOut(withMI("MI 96 1 0:96 / /mnt/c rw - 9p C:\\134 rw,aname=drvfs;path=C:\\\n"), n, allDenied), Unverified, CWSLScope},
		{"/mnt/c is a bind of a C: subtree", ExistsYes, wslOut(goodHead0+goodDev+rootMI+"MI 70 1 0:70 /Users/u /mnt/c rw - 9p C:\\134 rw,aname=drvfs;path=C:\\\n", n, allDenied), Unverified, CWSLScope},
		{"/mnt/c is drive D:", ExistsYes, wslOut(goodHead0+goodDev+rootMI+"MI 70 1 0:70 / /mnt/c rw - 9p D:\\134 rw,aname=drvfs;path=D:\\\n", n, allDenied), Unverified, CWSLScope},
		{"/mnt/c is a drvfs mount of C:\\Other", ExistsYes, wslOut(goodHead0+goodDev+rootMI+"MI 70 1 0:70 / /mnt/c rw - drvfs C:\\134Other rw\n", n, allDenied), Unverified, CWSLScope},
		{"9p path= option names a subdirectory", ExistsYes, wslOut(goodHead0+goodDev+rootMI+"MI 70 1 0:70 / /mnt/c rw - 9p C:\\134 rw,aname=drvfs;path=C:\\Other;uid=1000\n", n, allDenied), Unverified, CWSLScope},
		{"something mounted over /mnt hides /mnt/c (st_dev differs)", ExistsYes, wslOut(goodHead0+"DEVDRIVE 99\nDEVPARENT 99\n"+rootMI+driveMI+"MI 99 1 0:99 / /mnt rw - tmpfs tmpfs rw\n", n, allDenied), Unverified, CWSLScope},
		{"root's parent on another device", ExistsYes, wslOut(goodHead0+"DEVDRIVE 70\nDEVPARENT 71\n"+rootMI+driveMI, n, allDenied), Unverified, CWSLScope},
		{"st_dev not reported", ExistsYes, wslOut(goodHead0+"DEVDRIVE\nDEVPARENT\n"+rootMI+driveMI, n, allDenied), Unverified, CWSLScope},
		{"drive mount point is a symlink", ExistsYes, wslOut(strings.Replace(goodHeader, "REALDRIVE /mnt/c", "REALDRIVE /srv/fake-c", 1), n, allDenied), Unverified, CWSLScope},
		{"a parent component is a symlink", ExistsYes, wslOut(strings.Replace(goodHeader, "REALPARENT /mnt/c/ProgramData/Arameair", "REALPARENT /mnt/c/Users/u/decoy", 1), n, allDenied), Unverified, CWSLScope},
		{"root is a symlink", ExistsYes, wslOut(goodHeader+"ROOTLINK\n", n, allDenied), Unverified, CWSLScope},
		{"no mountinfo", ExistsYes, wslOut(goodHead0+goodDev, n, allDenied), Unverified, CWSLScope},
		{"unparseable mountinfo", ExistsYes, wslOut(withMI("MI garbage\n"), n, allDenied), Unverified, CWSLScope},
		{"duplicate evidence line", ExistsYes, wslOut(goodHeader+"DRIVE /mnt/d\n", n, allDenied), Unverified, CWSLUnobserved},
		{"virtiofs (source is a tag, not a drive)", ExistsYes, wslOut(goodHead0+goodDev+rootMI+"MI 70 1 0:70 / /mnt/c rw - virtiofs drvfsC rw\n", n, allDenied), Unverified, CWSLScope},
		{"tmpfs at /mnt/c", ExistsYes, wslOut(goodHead0+goodDev+rootMI+"MI 70 1 0:70 / /mnt/c rw - tmpfs C:\\134 rw\n", n, allDenied), Unverified, CWSLScope},
		{"root on the distro's own ext4", ExistsYes, wslOut("DISTRO Ubuntu\nWSLPATH /home/u/x\nDRIVE /mnt/c\nREALDRIVE /mnt/c\nREALPARENT /home/u\n"+goodDev+rootMI+driveMI, n, allDenied), Unverified, CWSLScope},
		{"drive translates to /", ExistsYes, wslOut(strings.Replace(goodHeader, "DRIVE /mnt/c/", "DRIVE /", 1), n, allDenied), Unverified, CWSLScope},
		{"unrelated mounts elsewhere are fine", ExistsYes, wslOut(withMI("MI 97 1 0:97 / /mnt/d rw - 9p D:\\134 rw\n", "MI 98 70 0:98 / /mnt/c/Users rw - 9p C:\\134 rw\n"), n, allDenied), Isolated, ""},
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

// ---- F1 (Codex audit 7060488246, blocking 2): no WSL launch for a rejected caller ------

type fakeWSL struct {
	tok      CallerToken
	launches int
	exists   int
	out      string
}

func (f *fakeWSL) runner() wslRunner {
	return wslRunner{
		token:    func() CallerToken { return f.tok },
		exists:   func(string) (Existence, string) { f.exists++; return ExistsYes, "" },
		lookPath: func() error { return nil },
		run:      func([]string) ([]byte, error) { f.launches++; return []byte(f.out), nil },
	}
}

func TestProbeWSLNeverLaunchesForRejectedCaller(t *testing.T) {
	p := EntityProfile(entityRoot, svc, normal)
	n := len(testTargets(ExistsYes))
	cases := []struct {
		name string
		tok  CallerToken
		want Verdict
		code Code
	}{
		{"token user unknown", CallerToken{}, Unverified, CProcessQuery},
		{"service account", CallerToken{UserSID: svc}, Unverified, CProbeIsService},
		{"another user", CallerToken{UserSID: other}, Unverified, CProcessWrong},
		{"token field unreadable", CallerToken{UserSID: normal, QueryErrors: []string{"token groups: x"}}, Unverified, CProcessQuery},
		{"elevated", CallerToken{UserSID: normal, Elevated: true}, NotIsolated, CNormalIsAdmin},
		{"linked admin token", CallerToken{UserSID: normal, LinkedAdmin: true}, NotIsolated, CNormalIsAdmin},
		{"Administrators enabled", CallerToken{UserSID: normal, AdminGroup: "enabled"}, NotIsolated, CNormalIsAdmin},
		{"Administrators deny-only", CallerToken{UserSID: normal, AdminGroup: "deny-only (UAC-filtered)"}, NotIsolated, CNormalIsAdmin},
		{"ACL-bypass privilege", CallerToken{UserSID: normal, Privileges: []string{"SeRestorePrivilege"}}, NotIsolated, CPrivileged},
		{"linked admin + deny-only (the hosted-runner shape)", CallerToken{UserSID: normal, LinkedAdmin: true, AdminGroup: "deny-only (UAC-filtered)"}, NotIsolated, CNormalIsAdmin},
	}
	for _, c := range cases {
		f := &fakeWSL{tok: c.tok, out: wslOut(goodHeader, n, allDenied)}
		r := probeWSL(p, EntityAuthorityFiles, f.runner())
		if f.launches != 0 || f.exists != 0 || len(r.Attempts) != 0 || r.WSL != nil {
			t.Errorf("%s: WSL launched %d times, %d existence probes, %d attempts", c.name, f.launches, f.exists, len(r.Attempts))
		}
		if r.Verdict != c.want || !hasP(r, c.code) {
			t.Errorf("%s: verdict %s, want %s with %s:%s", c.name, r.Verdict, c.want, c.code, dumpF(r.Findings))
		}
	}
	// Control: the same fake with a clean designated token does launch, exactly once.
	f := &fakeWSL{tok: CallerToken{UserSID: normal}, out: wslOut(goodHeader, n, allDenied)}
	r := probeWSL(p, EntityAuthorityFiles, f.runner())
	if f.launches != 1 || r.Verdict != Isolated || r.WSL == nil || r.WSL.MountRoot != "/" {
		t.Fatalf("clean caller: launches %d verdict %s:%s", f.launches, r.Verdict, dumpF(r.Findings))
	}
	// Invalid policy or file list: no launch either.
	for name, files := range map[string][]string{"bad file": {"../x"}} {
		f := &fakeWSL{tok: CallerToken{UserSID: normal}}
		if r := probeWSL(p, files, f.runner()); f.launches != 0 || r.Verdict == Isolated {
			t.Errorf("%s: launches %d verdict %s", name, f.launches, r.Verdict)
		}
	}
	f = &fakeWSL{tok: CallerToken{UserSID: normal}}
	if r := probeWSL(EntityProfile(entityRoot, svc, svc), nil, f.runner()); f.launches != 0 || r.Verdict == Isolated {
		t.Errorf("invalid policy: launches %d verdict %s", f.launches, r.Verdict)
	}
}

func hasP(r ProbeReport, c Code) bool {
	for _, f := range r.Findings {
		if f.Code == c {
			return true
		}
	}
	return false
}

// st_dev encoding (glibc makedev), checked against values observed with stat -c %d on Linux.
func TestUserDevMatchesKernelStDev(t *testing.T) {
	for mm, want := range map[string]uint64{"0:41": 41, "0:70": 70, "254:0": 65024, "8:32": 2080} {
		if got, ok := userDev(mm); !ok || got != want {
			t.Errorf("%s: %d %v, want %d", mm, got, ok, want)
		}
	}
	for _, bad := range []string{"", "8", "a:b", "8:"} {
		if _, ok := userDev(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}
