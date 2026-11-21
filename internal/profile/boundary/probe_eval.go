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
	MountPoint string   `json:"mount_point"` // longest /proc/mounts mount point containing Path
	FSType     string   `json:"fs_type"`
	Source     string   `json:"source"`
	NotCovered []string `json:"not_covered"`
}

// WSLNotCovered is reported with every ProbeWSL result.
var WSLNotCovered = []string{
	"every WSL distribution other than the one named in wsl.distro (only the default distro is run)",
	"every path other than the single wslpath translation in wsl.path (other drvfs mounts, custom automount roots, bind mounts, wsl --mount disks)",
	`\\wsl$ and \\wsl.localhost access from Windows into a distro`,
	"delete, re-ACL and take-ownership from WSL (the native Probe attempts these)",
	"which Windows token drvfs used: assumed to be the launching (validated) user's token, not independently observed",
	"a WSL instance already running under a different (e.g. elevated) Windows token",
}

// wslFSTypes are the mount types through which /mnt/<drive> reaches NTFS.
var wslFSTypes = map[string]bool{"9p": true, "drvfs": true, "virtiofs": true}

// wslRights are the attempts the script makes, per target kind.
var (
	wslRootRights = []string{"list", "create"}
	wslFileRights = []string{"read", "write"}
)

// wslScript runs inside WSL (sh) with $1 = Windows root, then the relative
// files. It prints exactly one "R <index> <right> <STATUS> [detail]" line per
// attempt; index 0 is the root, i is the i-th file. Silence never means
// denied: an attempt without its line is UNOBSERVED. It never creates a file:
// writes open with O_WRONLY and no O_CREAT/O_TRUNC (dd conv=notrunc,nocreat,
// count=0); the one create is a mkdir that is removed again.
const wslScript = `export LC_ALL=C
echo BEGIN
r=$(wslpath -u "$1" 2>&1) || { echo "NOPATH $(printf %s "$r" | tr '\n\r' '  ' | cut -c1-160)"; echo DONE; exit 0; }
case "$r" in /*) ;; *) echo "NOPATH not an absolute path: $(printf %s "$r" | tr '\n\r' '  ' | cut -c1-160)"; echo DONE; exit 0;; esac
shift
echo "DISTRO ${WSL_DISTRO_NAME:-}"
echo "WSLPATH $r"
awk -v p="$r" '{ m=$2; gsub(/\\040/, " ", m); if ((m == "/" || p == m || index(p, m "/") == 1) && length(m) > length(b)) { b = m; t = $3; s = $1 } } END { if (b != "") print "MOUNT " t " " s " " b }' /proc/mounts 2>/dev/null
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
		case "MOUNT":
			f := strings.SplitN(rest, " ", 3)
			if len(f) == 3 {
				tested.FSType, tested.Source, tested.MountPoint = f[0], f[1], f[2]
			}
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
	switch {
	case tested.Path == "":
		add(false, CWSLScope, root, "no wslpath translation reported")
	case tested.MountPoint == "" || tested.MountPoint == "/":
		add(false, CWSLScope, root, "%s is not under a Windows drive mount (mount point %q)", tested.Path, tested.MountPoint)
	case !wslFSTypes[tested.FSType]:
		add(false, CWSLScope, root, "%s is on a %q mount, not drvfs/9p: it may not reach the Windows object", tested.Path, tested.FSType)
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
