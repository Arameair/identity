package boundary

// Platform-independent halves of the normal-user probes (Probe, ProbeWSL).
//
// The Windows files only gather OS facts (token fields, open results, the
// output of one WSL shell run). Every decision about those facts is made here,
// so the fail-closed rules are covered by deterministic tests on every OS.

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// ProbeReport is the normal-user result.
type ProbeReport struct {
	Verdict  Verdict    `json:"verdict"`
	UserSID  string     `json:"user_sid"`
	Attempts []Attempt  `json:"attempts"`
	Findings []Finding  `json:"findings"`
	Scope    string     `json:"scope"`
	WSL      *WSLTested `json:"wsl,omitempty"` // ProbeWSL only: exactly what was exercised
}

// Attempt is one access attempt.
type Attempt struct {
	Path   string `json:"path"`
	Right  string `json:"right"`
	Result string `json:"result"` // DENIED | ALLOWED | ABSENT | ERROR <detail> | UNOBSERVED
}

func (r *ProbeReport) add(violation bool, c Code, p, f string, a ...any) {
	r.Findings = append(r.Findings, Finding{Code: c, Path: p, Detail: fmt.Sprintf(f, a...), Violation: violation})
}

// Probe codes.
const (
	CNormalIsAdmin  Code = "NORMAL_USER_IS_ADMIN"
	CPrivileged     Code = "NORMAL_USER_PRIVILEGED"
	CProbeAllowed   Code = "NORMAL_USER_ALLOWED"
	CProbeError     Code = "PROBE_ERROR"
	CProbeIsService Code = "PROBE_RUNNING_AS_SERVICE"
	CWSLAllowed     Code = "WSL_ACCESS_ALLOWED"
	CWSLUnverified  Code = "WSL_UNVERIFIED"
	CWSLAbsent      Code = "WSL_TARGET_ABSENT"      // the WSL view did not find a target
	CWSLError       Code = "WSL_ATTEMPT_ERROR"      // an attempt failed for a reason other than access denied
	CWSLUnobserved  Code = "WSL_ATTEMPT_UNOBSERVED" // an expected attempt produced no (or an ambiguous) result
	CWSLExistence   Code = "WSL_TARGET_EXISTENCE"   // Windows did not confirm the target exists
	CWSLScope       Code = "WSL_SCOPE_UNVERIFIED"   // distro / mount of the tested translation not established
)

// ---- caller identity (F1) ---------------------------------------------------------------

// CallerToken is what the OS reported about the probing process's token.
type CallerToken struct {
	UserSID     string
	QueryErrors []string // token fields that could not be read; each makes the result UNVERIFIED
	Elevated    bool     // TokenElevation != 0
	LinkedAdmin bool     // TokenElevationType == Limited (UAC split token)
	AdminGroup  string   // "" | "enabled" | "deny-only (UAC-filtered)"
	Privileges  []string // dangerousPrivileges present on the token (enabled or not)
}

// evaluateCaller is the shared caller gate for Probe and ProbeWSL. ok=false
// means the attempts must not run at all: the result would not be evidence
// about the designated normal user.
func evaluateCaller(p Policy, t CallerToken) (fs []Finding, ok bool) {
	add := func(v bool, c Code, f string, a ...any) {
		fs = append(fs, Finding{Code: c, Detail: fmt.Sprintf(f, a...), Violation: v})
	}
	switch {
	case t.UserSID == "":
		add(false, CProcessQuery, "token user unknown")
		return fs, false
	case strings.EqualFold(t.UserSID, p.ServiceSID):
		add(false, CProbeIsService, "the probe is running as the protected service account; run it as the normal user")
		return fs, false
	case !strings.EqualFold(t.UserSID, p.NormalUserSID):
		add(false, CProcessWrong, "the probe is running as %s, not the designated normal user %s", t.UserSID, p.NormalUserSID)
		return fs, false
	}
	for _, e := range t.QueryErrors {
		add(false, CProcessQuery, "%s", e)
	}
	if t.Elevated {
		add(true, CNormalIsAdmin, "the normal user's token is elevated")
	}
	if t.LinkedAdmin {
		add(true, CNormalIsAdmin, "the normal user has a linked administrator token (UAC split token); UAC is not a security boundary")
	}
	if t.AdminGroup != "" {
		add(true, CNormalIsAdmin, "the normal user is a member of Administrators (%s)", t.AdminGroup)
	}
	for _, name := range t.Privileges {
		add(true, CPrivileged, "the normal user's token holds %s", name)
	}
	return fs, true
}

// ---- WSL attempts (F2, F3) --------------------------------------------------------------

// Existence is the Windows-side (native API, same validated token) observation
// of whether a target exists. Only ExistsYes lets a WSL denial count.
type Existence int

const (
	ExistsUnknown Existence = iota
	ExistsYes
	ExistsNo
)

// WSLTarget is one path the WSL probe attempts. Index 0 is the root.
type WSLTarget struct {
	Path      string // Windows path, for the report
	Rel       string // relative to the root ("" for the root)
	Exists    Existence
	ExistsErr string
}

// WSLTested records exactly which WSL translation was exercised. Nothing
// outside it is covered by an ISOLATED ProbeWSL.
type WSLTested struct {
	Distro     string   `json:"distro"`      // $WSL_DISTRO_NAME of the default distro
	Path       string   `json:"path"`        // wslpath -u translation of the root
	Drive      string   `json:"drive"`       // wslpath -u translation of the root's drive (X:\)
	MountPoint string   `json:"mount_point"` // the drive's mount point (= Drive) in /proc/self/mountinfo
	MountRoot  string   `json:"mount_root"`  // the mounted subtree of the source; must be "/"
	MountDev   string   `json:"mount_dev"`   // major:minor of that mount
	FSType     string   `json:"fs_type"`
	Source     string   `json:"source"`
	NotCovered []string `json:"not_covered"`
}

// WSLNotCovered is reported with every ProbeWSL result.
var WSLNotCovered = []string{
	"every WSL distribution other than the one named in wsl.distro (only the default distro is run)",
	"every path other than the single wslpath translation in wsl.path (other drvfs mounts of the drive, custom automount roots, wsl --mount disks); bind, overlay and nested mounts on that path are detected and make the result UNVERIFIED",
	`\\wsl$ and \\wsl.localhost access from Windows into a distro`,
	"delete, re-ACL and take-ownership from WSL (the native Probe attempts these)",
	"which Windows token drvfs used: assumed to be the launching (validated) user's token, not independently observed",
	"a WSL instance already running under a different (e.g. elevated) Windows token",
}

// wslFSTypes are the mount types through which /mnt/<drive> reaches NTFS.
// virtiofs is deliberately absent: its source names a tag, not a drive, so the
// drive identity below cannot be checked and the result would be UNVERIFIED anyway.
var wslFSTypes = map[string]bool{"9p": true, "drvfs": true}

// wslRights are the attempts the script makes, per target kind.
var (
	wslRootRights = []string{"list", "create"}
	wslFileRights = []string{"read", "write"}
)

// wslScript runs inside WSL (sh) with $1 = Windows root, $2 = its drive
// root (X:\), then the relative files. Before the attempts it reports the
// evidence verifyWSLMount needs to tie the path to the Windows drive: both
// translations, their canonical (symlink-free) forms, st_dev of the drive
// mount point and of the root's parent, whether the root is a symlink, and
// every /proc/self/mountinfo line. It prints exactly one "R <index> <right> <STATUS> [detail]" line per
// attempt; index 0 is the root, i is the i-th file. Silence never means
// denied: an attempt without its line is UNOBSERVED. It never creates a file:
// writes open with O_WRONLY and no O_CREAT/O_TRUNC (dd conv=notrunc,nocreat,
// count=0); the one create is a mkdir that is removed again.
const wslScript = `export LC_ALL=C
echo BEGIN
r=$(wslpath -u "$1" 2>&1) || { echo "NOPATH $(printf %s "$r" | tr '\n\r' '  ' | cut -c1-160)"; echo DONE; exit 0; }
case "$r" in /*) ;; *) echo "NOPATH not an absolute path: $(printf %s "$r" | tr '\n\r' '  ' | cut -c1-160)"; echo DONE; exit 0;; esac
dr=$(wslpath -u "$2" 2>&1) || { echo "NOPATH drive: $(printf %s "$dr" | tr '\n\r' '  ' | cut -c1-160)"; echo DONE; exit 0; }
shift 2
echo "DISTRO ${WSL_DISTRO_NAME:-}"
echo "WSLPATH $r"
echo "DRIVE $dr"
pd=$(dirname -- "$r")
echo "REALDRIVE $(cd -P -- "$dr" 2>/dev/null && pwd -P)"
echo "REALPARENT $(cd -P -- "$pd" 2>/dev/null && pwd -P)"
echo "DEVDRIVE $(stat -c %d -- "$dr" 2>/dev/null)"
echo "DEVPARENT $(stat -c %d -- "$pd" 2>/dev/null)"
if [ -L "$r" ]; then echo ROOTLINK; fi
if [ -r /proc/self/mountinfo ]; then while IFS= read -r l; do echo "MI $l"; done < /proc/self/mountinfo; fi
cls() { case "$1" in *"Permission denied"*|*"Operation not permitted"*) echo DENIED;; *"No such file or directory"*|*"Not a directory"*) echo ABSENT;; *) echo ERROR;; esac; }
res() { if [ "$3" = 0 ]; then echo "R $1 $2 ALLOWED"; else echo "R $1 $2 $(cls "$4") $(printf %s "$4" | tr '\n\r' '  ' | cut -c1-160)"; fi; }
e=$(ls -a -- "$r" 2>&1 >/dev/null); res 0 list $? "$e"
d="$r/.wsl-probe-$$"
e=$(mkdir -- "$d" 2>&1); c=$?; [ $c = 0 ] && rmdir -- "$d"; res 0 create $c "$e"
n=0
for f in "$@"; do n=$((n+1)); p="$r/$f"
  e=$(dd if="$p" of=/dev/null bs=1 count=1 status=none 2>&1); res $n read $? "$e"
  e=$(dd if=/dev/null of="$p" conv=notrunc,nocreat bs=1 count=0 status=none 2>&1); res $n write $? "$e"
done
echo DONE`

// evaluateWSL turns the script output into attempts and findings. Every
// expected attempt must be observed exactly once; only DENIED on a target that
// Windows confirmed exists counts as denied. ABSENT, ERROR, missing, duplicate
// or unknown lines, a failed translation, an unnamed distro and a non-drvfs
// mount are all UNVERIFIED; ALLOWED is a violation.
func evaluateWSL(out string, targets []WSLTarget) (att []Attempt, fs []Finding, tested WSLTested) {
	tested.NotCovered = append([]string(nil), WSLNotCovered...)
	root := ""
	if len(targets) > 0 {
		root = targets[0].Path
	}
	add := func(v bool, c Code, p, f string, a ...any) {
		fs = append(fs, Finding{Code: c, Path: p, Detail: fmt.Sprintf(f, a...), Violation: v})
	}
	if len(targets) == 0 || targets[0].Rel != "" {
		add(false, CWSLUnverified, "", "no root target")
		return
	}
	for _, t := range targets {
		switch t.Exists {
		case ExistsNo:
			add(false, CWSLExistence, t.Path, "Windows reports the target does not exist; a WSL denial on it would not be evidence")
		case ExistsUnknown:
			add(false, CWSLExistence, t.Path, "Windows could not confirm the target exists (%s)", t.ExistsErr)
		}
	}

	var lines []string
	for _, l := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) < 2 || lines[0] != "BEGIN" || lines[len(lines)-1] != "DONE" {
		add(false, CWSLUnverified, root, "WSL probe output incomplete: %.200q", out)
		return
	}
	type key struct {
		i     int
		right string
	}
	seen := map[key]string{}
	dup := map[key]bool{}
	ev := wslMountEvidence{fields: map[string]string{}}
	for _, l := range lines[1 : len(lines)-1] {
		head, rest, _ := strings.Cut(l, " ")
		switch head {
		case "NOPATH":
			add(false, CWSLUnverified, root, "wslpath could not translate the root: %s", rest)
			return
		case "DISTRO":
			tested.Distro = rest
		case "WSLPATH":
			tested.Path = rest
		case "DRIVE", "REALDRIVE", "REALPARENT", "DEVDRIVE", "DEVPARENT", "ROOTLINK":
			if _, ok := ev.fields[head]; ok {
				add(false, CWSLUnobserved, root, "duplicate %s line", head)
			}
			ev.fields[head] = rest
		case "MI":
			m, err := parseMountInfo(rest)
			if err != nil {
				add(false, CWSLScope, root, "unparseable mountinfo line %q: %v", rest, err)
				continue
			}
			ev.mounts = append(ev.mounts, m)
		case "R":
			f := strings.SplitN(rest, " ", 4)
			if len(f) < 3 {
				add(false, CWSLUnobserved, root, "malformed result line %q", l)
				continue
			}
			i, err := strconv.Atoi(f[0])
			if err != nil || i < 0 || i >= len(targets) {
				add(false, CWSLUnobserved, root, "result for an unknown target %q", l)
				continue
			}
			k := key{i, f[1]}
			if _, ok := seen[k]; ok {
				dup[k] = true
			}
			v := f[2]
			if len(f) == 4 {
				v += " " + f[3]
			}
			seen[k] = v
		default:
			add(false, CWSLUnobserved, root, "unexpected output line %q", l)
		}
	}
	if tested.Distro == "" {
		add(false, CWSLScope, root, "the WSL distribution that ran the probe is not named ($WSL_DISTRO_NAME empty)")
	}
	if tested.Path == "" {
		add(false, CWSLScope, root, "no wslpath translation reported")
	} else {
		ev.path = tested.Path
		for _, problem := range verifyWSLMount(&tested, ev, root[0], wslFSTypes) {
			add(false, CWSLScope, root, "WSL path not proven to be the Windows object: %s", problem)
		}
	}

	expected := map[key]bool{}
	for i, t := range targets {
		rights := wslFileRights
		if i == 0 {
			rights = wslRootRights
		}
		for _, right := range rights {
			k := key{i, right}
			expected[k] = true
			a := Attempt{Path: t.Path, Right: "wsl " + right}
			v, ok := seen[k]
			status, detail, _ := strings.Cut(v, " ")
			switch {
			case !ok:
				a.Result = "UNOBSERVED"
				add(false, CWSLUnobserved, t.Path, "wsl %s: no result reported", right)
			case dup[k]:
				a.Result = "UNOBSERVED"
				add(false, CWSLUnobserved, t.Path, "wsl %s: more than one result reported", right)
			case status == "ALLOWED":
				a.Result = "ALLOWED"
				add(true, CWSLAllowed, t.Path, "from WSL (%s): %s allowed", tested.Path, right)
			case status == "DENIED":
				a.Result = "DENIED" // counts only together with Exists == ExistsYes (flagged above otherwise)
			case status == "ABSENT":
				a.Result = "ABSENT"
				add(false, CWSLAbsent, t.Path, "wsl %s: the WSL view does not find the target (%s); wrong distro, mount or translation, or the target is missing", right, detail)
			case status == "ERROR":
				a.Result = "ERROR " + detail
				add(false, CWSLError, t.Path, "wsl %s: %s", right, detail)
			default:
				a.Result = "UNOBSERVED"
				add(false, CWSLUnobserved, t.Path, "wsl %s: unknown result %q", right, v)
			}
			att = append(att, a)
		}
	}
	for k := range seen {
		if !expected[k] {
			add(false, CWSLUnobserved, root, "result for an attempt that was not requested: %d %s", k.i, k.right)
		}
	}
	return
}

// ---- WSL mount identity (F3) -----------------------------------------------------------

type mountInfo struct {
	dev, root, point, fstype, source, superOpts string
}

type wslMountEvidence struct {
	path   string
	fields map[string]string // DRIVE, REALDRIVE, REALPARENT, DEVDRIVE, DEVPARENT, ROOTLINK
	mounts []mountInfo
}

// unescapeMount decodes the \NNN octal escapes of /proc/self/mountinfo.
func unescapeMount(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseMountInfo parses one proc(5) mountinfo line:
// id parent major:minor root mountpoint options [optional...] - fstype source superoptions
func parseMountInfo(l string) (mountInfo, error) {
	f := strings.Fields(l)
	sep := -1
	for i := 6; i < len(f); i++ {
		if f[i] == "-" {
			sep = i
			break
		}
	}
	if sep < 0 || sep+2 >= len(f) {
		return mountInfo{}, fmt.Errorf("too few fields")
	}
	m := mountInfo{dev: f[2], root: unescapeMount(f[3]), point: unescapeMount(f[4]),
		fstype: f[sep+1], source: unescapeMount(f[sep+2])}
	if sep+3 < len(f) {
		m.superOpts = unescapeMount(f[sep+3])
	}
	if !strings.Contains(m.dev, ":") || !strings.HasPrefix(m.point, "/") {
		return mountInfo{}, fmt.Errorf("bad device or mount point")
	}
	return m, nil
}

// userDev converts "major:minor" to the st_dev value stat -c %d prints (glibc makedev).
func userDev(mm string) (uint64, bool) {
	a, b, ok := strings.Cut(mm, ":")
	ma, e1 := strconv.ParseUint(a, 10, 32)
	mi, e2 := strconv.ParseUint(b, 10, 32)
	if !ok || e1 != nil || e2 != nil {
		return 0, false
	}
	return (ma&0xfffff000)<<32 | (ma&0xfff)<<8 | (mi&0xffffff00)<<12 | (mi & 0xff), true
}

func trimSlash(p string) string {
	if len(p) > 1 {
		return strings.TrimRight(p, "/")
	}
	return p
}

// isDriveRoot reports whether s names the root of the given drive ("C:", "C:\", "C:/").
func isDriveRoot(s string, drive byte) bool {
	s = strings.TrimRight(s, `\/`)
	return len(s) == 2 && s[1] == ':' && strings.EqualFold(s[:1], string(drive))
}

// verifyWSLMount returns every reason the WSL path cannot be shown to be the
// intended Windows object. An empty result requires ALL of:
//   - the drive translates to a canonical (symlink-free) mount point D, not "/";
//   - the root's WSL path is strictly under D, its parent resolves to itself
//     (no symlinked component) and the root itself is not a symlink;
//   - exactly one mountinfo entry at D: an accepted type, mounting the WHOLE
//     source (mount root "/"), whose source (and any path= option) is this
//     drive's root;
//   - st_dev of D and of the root's parent both equal that entry's device, so
//     the path really resolves through it (not a mount hiding D, not a
//     same-named directory elsewhere);
//   - no other mount anywhere between D and the root, at the root, or below it
//     (bind, overlay or nested drvfs/9p mounts of any source).
func verifyWSLMount(t *WSLTested, ev wslMountEvidence, drive byte, fsOK map[string]bool) (problems []string) {
	bad := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	r := ev.path
	d := trimSlash(ev.fields["DRIVE"])
	t.Drive = d
	if d == "" || d == "/" || !strings.HasPrefix(d, "/") {
		bad("drive %c: translates to %q, not a drive mount point", drive, d)
		return
	}
	if !strings.HasPrefix(r, d+"/") {
		bad("%s is not under the drive mount point %s", r, d)
	}
	if got := ev.fields["REALDRIVE"]; got != d {
		bad("drive mount point %s resolves to %q (missing or symlinked)", d, got)
	}
	if got := ev.fields["REALPARENT"]; got != path.Dir(r) {
		bad("the root's parent %s resolves to %q (missing, untraversable or symlinked)", path.Dir(r), got)
	}
	if _, ok := ev.fields["ROOTLINK"]; ok {
		bad("%s is a symbolic link in the WSL view", r)
	}
	if len(ev.mounts) == 0 {
		bad("no mountinfo reported")
		return
	}
	var at []mountInfo
	for _, m := range ev.mounts {
		p := trimSlash(m.point)
		switch {
		case p == d:
			at = append(at, m)
		case strings.HasPrefix(p, d+"/") && (p == r || strings.HasPrefix(r, p+"/") || strings.HasPrefix(p, r+"/")):
			bad("another mount at %s (%s %s, mounted subtree %s) lies on the path: bind/overlay/nested mount", p, m.fstype, m.source, m.root)
		}
	}
	if len(at) != 1 {
		bad("expected exactly one mount at %s, found %d (none, or stacked mounts)", d, len(at))
		return
	}
	m := at[0]
	t.MountPoint, t.MountRoot, t.MountDev, t.FSType, t.Source = trimSlash(m.point), m.root, m.dev, m.fstype, m.source
	if !fsOK[m.fstype] {
		bad("%s is a %q mount, not drvfs/9p", d, m.fstype)
	}
	if m.root != "/" {
		bad("%s mounts the subtree %q of its source, not the whole drive (bind mount)", d, m.root)
	}
	if !isDriveRoot(m.source, drive) {
		bad("%s is mounted from %q, not drive %c:", d, m.source, drive)
	}
	for _, o := range strings.FieldsFunc(m.superOpts, func(c rune) bool { return c == ',' || c == ';' }) {
		if v, ok := strings.CutPrefix(o, "path="); ok && !isDriveRoot(v, drive) {
			bad("%s is mounted from path=%q, not drive %c:", d, v, drive)
		}
	}
	want, ok := userDev(m.dev)
	if !ok {
		bad("unparseable device %q", m.dev)
		return
	}
	for _, k := range []string{"DEVDRIVE", "DEVPARENT"} {
		if got, err := strconv.ParseUint(ev.fields[k], 10, 64); err != nil || got != want {
			bad("%s st_dev %q is not the drive mount's device %s", k, ev.fields[k], m.dev)
		}
	}
	return
}

// ---- ProbeWSL core (OS access injected) -------------------------------------------------

// wslRunner is ProbeWSL's only OS access, injected so the gating order is testable.
type wslRunner struct {
	token    func() CallerToken
	exists   func(path string) (Existence, string)
	lookPath func() error
	run      func(args []string) ([]byte, error)
}

// probeWSL is ProbeWSL with the OS injected. WSL is launched only when the
// policy and file list are valid, the caller is the designated normal user,
// and the token is fully known and clean (no query error, not elevated, no
// linked admin token, not in Administrators, no ACL-bypass privilege).
func probeWSL(p Policy, files []string, env wslRunner) (r ProbeReport) {
	r = ProbeReport{Scope: "WSL ACCESS ATTEMPTS through ONE wslpath translation of the protected root, in the DEFAULT WSL distro only (exactly what is in wsl). " + scope}
	defer func() { r.Verdict = verdictOf(r.Findings) }()
	root, _, err := SplitWindowsPath(p.Root)
	if err != nil {
		r.add(true, CPath, p.Root, "%v", err)
		return r
	}
	if err := p.validate(); err != nil {
		r.add(false, CPolicy, "", "%v", err)
		return r
	}
	targets, err := wslTargets(root, p, files)
	if err != nil {
		r.add(false, CPolicy, "", "%v", err)
		return r
	}
	tok := env.token()
	r.UserSID = tok.UserSID
	fs, ok := evaluateCaller(p, tok)
	r.Findings = append(r.Findings, fs...)
	if !ok || len(fs) > 0 {
		r.add(false, CWSLUnverified, root, "WSL not launched: the caller token is not a fully known, clean standard-user token")
		return r
	}
	for i := range targets {
		targets[i].Exists, targets[i].ExistsErr = env.exists(targets[i].Path)
	}
	if err := env.lookPath(); err != nil {
		r.add(false, CWSLUnverified, root, "wsl.exe not available: WSL access UNVERIFIED")
		return r
	}
	args := []string{"-e", "sh", "-c", wslScript, "probe", root, root[:3]}
	for _, t := range targets[1:] {
		args = append(args, t.Rel)
	}
	out, err := env.run(args)
	text := strings.ReplaceAll(string(out), "\x00", "") // wsl.exe may emit UTF-16 diagnostics
	if err != nil {
		r.add(false, CWSLUnverified, root, "wsl.exe failed (%v): %.200q", err, text)
		return r
	}
	att, wfs, tested := evaluateWSL(text, targets)
	r.Attempts, r.WSL = att, &tested
	r.Findings = append(r.Findings, wfs...)
	return r
}

// wslTargets lists the root and every file (listed files plus Required,
// deduplicated). Each file must be a plain relative path inside the root.
func wslTargets(root string, p Policy, files []string) ([]WSLTarget, error) {
	t := []WSLTarget{{Path: root}}
	seen := map[string]bool{}
	for _, f := range append(append([]string(nil), files...), p.Required...) {
		rel := strings.ReplaceAll(f, `\`, "/")
		if rel == "" || path.IsAbs(rel) || path.Clean(rel) != rel || rel == "." || rel == ".." ||
			strings.HasPrefix(rel, "../") || strings.ContainsAny(rel, ":\x00\n\r") {
			return nil, fmt.Errorf("file %q must be a plain relative path inside the root", f)
		}
		if seen[strings.ToLower(rel)] {
			continue
		}
		seen[strings.ToLower(rel)] = true
		t = append(t, WSLTarget{Path: root + `\` + strings.ReplaceAll(rel, "/", `\`), Rel: rel})
	}
	return t, nil
}
