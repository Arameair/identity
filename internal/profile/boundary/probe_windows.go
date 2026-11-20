//go:build windows

package boundary

// Probe is the normal-user half of the boundary check. Run it AS the owner's
// everyday, unelevated account against a DISPOSABLE protected profile.
//
// It is non-destructive: every attempt opens a handle with one requested
// right and closes it immediately (nothing is written, renamed or re-ACLed).
// The one create attempt uses CREATE_NEW + FILE_FLAG_DELETE_ON_CLOSE, so even
// a wrongly allowed create leaves nothing behind. Only ERROR_ACCESS_DENIED
// counts as denied; any other outcome is UNVERIFIED or a violation.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// ProbeReport is the normal-user result.
type ProbeReport struct {
	Verdict  Verdict   `json:"verdict"`
	UserSID  string    `json:"user_sid"`
	Attempts []Attempt `json:"attempts"`
	Findings []Finding `json:"findings"`
	Scope    string    `json:"scope"`
}

// Attempt is one access attempt.
type Attempt struct {
	Path   string `json:"path"`
	Right  string `json:"right"`
	Result string `json:"result"` // DENIED | ALLOWED | ABSENT | ERROR <errno>
}

const (
	tokenGroups           = 2
	tokenPrivileges       = 3
	tokenElevationType    = 18
	tokenElevation        = 20
	elevationTypeLimited  = 3
	seGroupUseForDenyOnly = 0x10
	errAccessDenied       = syscall.Errno(5)
	fileFlagDeleteOnClose = 0x04000000
)

var procLookupPrivilegeValueW = advapi32.NewProc("LookupPrivilegeValueW")

// dangerousPrivileges let a token bypass file ACLs or take over processes.
var dangerousPrivileges = []string{"SeBackupPrivilege", "SeRestorePrivilege", "SeTakeOwnershipPrivilege",
	"SeDebugPrivilege", "SeSecurityPrivilege", "SeManageVolumePrivilege", "SeTcbPrivilege", "SeLoadDriverPrivilege"}

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
)

// Probe tries, as the current (normal) user, every right that would let it
// read, change, delete, re-ACL, take over or replace the protected profile.
// files are relative paths (AdminAuthorityFiles / EntityAuthorityFiles); required must exist.
func Probe(p Policy, files []string) (r ProbeReport) {
	r = ProbeReport{Scope: "NORMAL-USER ACCESS ATTEMPTS against the listed paths, plus this token's admin/privilege state. " + scope}
	defer func() { r.Verdict = verdictOf(r.Findings) }()
	root, ancestors, err := SplitWindowsPath(p.Root)
	if err != nil {
		r.add(true, CPath, p.Root, "%v", err)
		return r
	}
	if err := p.validate(); err != nil {
		r.add(false, CPolicy, "", "%v", err)
		return r
	}
	t, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		r.add(false, CProcessQuery, "", "%v", err)
		return r
	}
	defer t.Close()
	if u, err := t.GetTokenUser(); err == nil {
		r.UserSID, _ = u.User.Sid.String()
	}
	switch {
	case r.UserSID == "":
		r.add(false, CProcessQuery, "", "token user unknown")
		return r
	case strings.EqualFold(r.UserSID, p.ServiceSID):
		r.add(false, CProbeIsService, "", "the probe is running as the protected service account; run it as the normal user")
		return r
	case !strings.EqualFold(r.UserSID, p.NormalUserSID):
		r.add(false, CProcessWrong, "", "the probe is running as %s, not the designated normal user %s", r.UserSID, p.NormalUserSID)
		return r
	}
	checkToken(&r, t)

	try := func(path string, names []string, rights []uint32, required bool) {
		for i, right := range rights {
			h, err := openNoFollow(path, right)
			a := Attempt{Path: path, Right: names[i]}
			switch {
			case err == nil:
				syscall.CloseHandle(h)
				a.Result = "ALLOWED"
				r.add(true, CProbeAllowed, path, "the normal user obtained %s", names[i])
			case errors.Is(err, errAccessDenied):
				a.Result = "DENIED"
			case errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(err, syscall.ERROR_PATH_NOT_FOUND):
				a.Result = "ABSENT"
				if required {
					r.add(false, CRequired, path, "required file not found from the normal user's view")
				}
				r.Attempts = append(r.Attempts, a)
				return
			default:
				a.Result = "ERROR " + err.Error()
				r.add(false, CProbeError, path, "%s: %v", names[i], err)
			}
			r.Attempts = append(r.Attempts, a)
		}
	}
	fileNames := []string{"read", "write", "write-attributes", "delete", "write-dac", "take-ownership"}
	fileRights := []uint32{ReadData, WriteData | AppendData, WriteAttributes, Delete, WriteDAC, WriteOwner}
	dirNames := []string{"list", "add-file", "add-subdir", "delete-child", "delete", "write-dac", "take-ownership"}
	dirRights := []uint32{ReadData, WriteData, AppendData, DeleteChild, Delete, WriteDAC, WriteOwner}
	ancNames := []string{"delete", "delete-child", "write-dac", "take-ownership"}
	ancRights := []uint32{Delete, DeleteChild, WriteDAC, WriteOwner}

	try(root, dirNames, dirRights, true)
	req := map[string]bool{}
	for _, f := range p.Required {
		req[strings.ToLower(strings.ReplaceAll(f, "/", `\`))] = true
	}
	for _, f := range files {
		rel := strings.ReplaceAll(f, "/", `\`)
		try(root+`\`+rel, fileNames, fileRights, req[strings.ToLower(rel)])
	}
	for _, a := range ancestors {
		try(a, ancNames, ancRights, false)
	}

	// Create attempt (delete-on-close: nothing remains even if wrongly allowed).
	b := make([]byte, 6)
	rand.Read(b)
	cp := root + `\.boundary-probe-` + hex.EncodeToString(b)
	if u, err := syscall.UTF16PtrFromString(cp); err == nil {
		h, err := syscall.CreateFile(u, syscall.GENERIC_WRITE, 0, nil, syscall.CREATE_NEW, fileFlagDeleteOnClose, 0)
		a := Attempt{Path: cp, Right: "create-file"}
		switch {
		case err == nil:
			syscall.CloseHandle(h)
			a.Result = "ALLOWED"
			r.add(true, CProbeAllowed, cp, "the normal user created a file in the protected root")
		case errors.Is(err, errAccessDenied):
			a.Result = "DENIED"
		default:
			a.Result = "ERROR " + err.Error()
			r.add(false, CProbeError, cp, "create: %v", err)
		}
		r.Attempts = append(r.Attempts, a)
	}
	return r
}

func checkToken(r *ProbeReport, t syscall.Token) {
	var et, el uint32
	var n uint32
	if err := syscall.GetTokenInformation(t, tokenElevationType, (*byte)(unsafe.Pointer(&et)), 4, &n); err != nil {
		r.add(false, CProcessQuery, "", "elevation type: %v", err)
	}
	if err := syscall.GetTokenInformation(t, tokenElevation, (*byte)(unsafe.Pointer(&el)), 4, &n); err != nil {
		r.add(false, CProcessQuery, "", "elevation: %v", err)
	}
	if el != 0 {
		r.add(true, CNormalIsAdmin, "", "the normal user's token is elevated")
	}
	if et == elevationTypeLimited {
		r.add(true, CNormalIsAdmin, "", "the normal user has a linked administrator token (UAC split token); UAC is not a security boundary")
	}
	// Administrators in the group list, enabled or deny-only, means an admin account.
	if buf, err := tokenInfo(t, tokenGroups); err != nil {
		r.add(false, CProcessQuery, "", "token groups: %v", err)
	} else {
		count := *(*uint32)(unsafe.Pointer(&buf[0]))
		// TOKEN_GROUPS: DWORD count, then SID_AND_ATTRIBUTES {PSID; DWORD} aligned to pointer size.
		step := unsafe.Sizeof(syscall.SIDAndAttributes{})
		// Groups starts after the DWORD count, aligned to the struct's alignment.
		groupsOff := (4 + unsafe.Alignof(syscall.SIDAndAttributes{}) - 1) &^ (unsafe.Alignof(syscall.SIDAndAttributes{}) - 1)
		base := unsafe.Pointer(&buf[groupsOff])
		for i := uintptr(0); i < uintptr(count) && groupsOff+(i+1)*step <= uintptr(len(buf)); i++ {
			g := (*syscall.SIDAndAttributes)(unsafe.Add(base, i*step))
			if s, err := g.Sid.String(); err == nil && s == SIDAdministrators {
				how := "enabled"
				if g.Attributes&seGroupUseForDenyOnly != 0 {
					how = "deny-only (UAC-filtered)"
				}
				r.add(true, CNormalIsAdmin, "", "the normal user is a member of Administrators (%s)", how)
			}
		}
	}
	// Dangerous privileges, present at all (enabled or not), bypass ACLs.
	if buf, err := tokenInfo(t, tokenPrivileges); err != nil {
		r.add(false, CProcessQuery, "", "token privileges: %v", err)
	} else {
		count := *(*uint32)(unsafe.Pointer(&buf[0]))
		held := map[[2]uint32]bool{}
		for i := uintptr(0); i < uintptr(count); i++ {
			off := 4 + i*12 // LUID_AND_ATTRIBUTES {LUID{low,high}; DWORD}
			if int(off+8) > len(buf) {
				break
			}
			held[[2]uint32{*(*uint32)(unsafe.Pointer(&buf[off])), *(*uint32)(unsafe.Pointer(&buf[off+4]))}] = true
		}
		for _, name := range dangerousPrivileges {
			var luid [2]uint32
			u, _ := syscall.UTF16PtrFromString(name)
			if ok, _, _ := procLookupPrivilegeValueW.Call(0, uintptr(unsafe.Pointer(u)), uintptr(unsafe.Pointer(&luid))); ok == 0 {
				r.add(false, CProcessQuery, "", "cannot look up %s", name)
				continue
			}
			if held[luid] {
				r.add(true, CPrivileged, "", "the normal user's token holds %s", name)
			}
		}
	}
}

func tokenInfo(t syscall.Token, class uint32) ([]byte, error) {
	var n uint32
	syscall.GetTokenInformation(t, class, nil, 0, &n)
	if n == 0 {
		return nil, errors.New("no size")
	}
	buf := make([]byte, n)
	if err := syscall.GetTokenInformation(t, class, &buf[0], n, &n); err != nil {
		return nil, err
	}
	return buf, nil
}

// ProbeWSL repeats the essential attempts from inside WSL (the default distro,
// via wsl.exe), where /mnt/<drive> access runs with the launching Windows
// user's token. Non-destructive: it only appends nothing (": >>") to files
// that already exist, reads them, and creates+removes one probe directory.
func ProbeWSL(p Policy, files []string) (r ProbeReport) {
	r = ProbeReport{Scope: "WSL ACCESS ATTEMPTS through the drvfs mount of the protected root. " + scope}
	defer func() { r.Verdict = verdictOf(r.Findings) }()
	root, _, err := SplitWindowsPath(p.Root)
	if err != nil {
		r.add(true, CPath, p.Root, "%v", err)
		return r
	}
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		r.add(false, CWSLUnverified, root, "wsl.exe not available: WSL access UNVERIFIED")
		return r
	}
	script := `r=$(wslpath -u "$1") || { echo NOPATH; exit 0; }; shift
for f in "$@"; do p="$r/$f"
  if [ -e "$p" ]; then
    if ( : >> "$p" ) 2>/dev/null; then echo "WRITABLE $f"; fi
    if cat "$p" >/dev/null 2>&1; then echo "READABLE $f"; fi
  fi
done
d="$r/.wsl-probe-$$"; if mkdir "$d" 2>/dev/null; then rmdir "$d"; echo "CREATABLE ."; fi
if ls "$r" >/dev/null 2>&1; then echo "LISTABLE ."; fi
echo DONE`
	args := append([]string{"-e", "sh", "-c", script, "probe", root}, files...)
	out, err := exec.Command("wsl.exe", args...).CombinedOutput()
	text := strings.ReplaceAll(string(out), "\x00", "") // wsl.exe may emit UTF-16 diagnostics
	if err != nil || !strings.Contains(text, "DONE") || strings.Contains(text, "NOPATH") {
		r.add(false, CWSLUnverified, root, "WSL probe did not complete (%v): %.200q", err, text)
		return r
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		for _, k := range []string{"WRITABLE ", "READABLE ", "CREATABLE ", "LISTABLE "} {
			if strings.HasPrefix(line, k) {
				r.add(true, CWSLAllowed, root, "from WSL: %s", line)
				r.Attempts = append(r.Attempts, Attempt{Path: root, Right: "wsl " + strings.ToLower(strings.TrimSpace(k)), Result: "ALLOWED"})
			}
		}
	}
	if len(r.Attempts) == 0 {
		r.Attempts = append(r.Attempts, Attempt{Path: root, Right: "wsl write/read/create/list", Result: "DENIED"})
	}
	return r
}
