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
	tok := readCallerToken()
	r.UserSID = tok.UserSID
	fs, ok := evaluateCaller(p, tok)
	r.Findings = append(r.Findings, fs...)
	if !ok {
		return r
	}

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

// readCallerToken gathers the current process token's facts. It decides
// nothing: evaluateCaller (probe_eval.go) does, for Probe and ProbeWSL alike.
func readCallerToken() (c CallerToken) {
	t, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		c.QueryErrors = append(c.QueryErrors, fmt.Sprintf("open process token: %v", err))
		return c // UserSID "" → evaluateCaller refuses
	}
	defer t.Close()
	if u, err := t.GetTokenUser(); err == nil {
		c.UserSID, _ = u.User.Sid.String()
	}
	var et, el uint32
	var n uint32
	if err := syscall.GetTokenInformation(t, tokenElevationType, (*byte)(unsafe.Pointer(&et)), 4, &n); err != nil {
		c.QueryErrors = append(c.QueryErrors, fmt.Sprintf("elevation type: %v", err))
	}
	if err := syscall.GetTokenInformation(t, tokenElevation, (*byte)(unsafe.Pointer(&el)), 4, &n); err != nil {
		c.QueryErrors = append(c.QueryErrors, fmt.Sprintf("elevation: %v", err))
	}
	c.Elevated = el != 0
	c.LinkedAdmin = et == elevationTypeLimited
	// Administrators in the group list, enabled or deny-only, means an admin account.
	if buf, err := tokenInfo(t, tokenGroups); err != nil {
		c.QueryErrors = append(c.QueryErrors, fmt.Sprintf("token groups: %v", err))
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
				c.AdminGroup = "enabled"
				if g.Attributes&seGroupUseForDenyOnly != 0 {
					c.AdminGroup = "deny-only (UAC-filtered)"
				}
			}
		}
	}
	// Dangerous privileges, present at all (enabled or not), bypass ACLs.
	if buf, err := tokenInfo(t, tokenPrivileges); err != nil {
		c.QueryErrors = append(c.QueryErrors, fmt.Sprintf("token privileges: %v", err))
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
				c.QueryErrors = append(c.QueryErrors, fmt.Sprintf("cannot look up %s", name))
				continue
			}
			if held[luid] {
				c.Privileges = append(c.Privileges, name)
			}
		}
	}
	return c
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

// windowsExistence opens the path with NO access rights (CreateFile documents
// that this succeeds for metadata queries even where read access would be
// denied), without following reparse points. It is the positive existence
// evidence a WSL denial needs: success = exists; not-found = absent; any other
// result = unknown.
func windowsExistence(p string) (Existence, string) {
	h, err := openNoFollow(p, 0)
	switch {
	case err == nil:
		syscall.CloseHandle(h)
		return ExistsYes, ""
	case errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(err, syscall.ERROR_PATH_NOT_FOUND):
		return ExistsNo, err.Error()
	default:
		return ExistsUnknown, err.Error()
	}
}

// ProbeWSL repeats read/write/list/create attempts from inside WSL, where
// /mnt/<drive> access is served with the launching Windows user's token.
//
// WSL is launched only for the designated normal user with a fully known,
// clean token (no query error, not elevated, no linked admin token, not in
// Administrators, no ACL-bypass privilege); any other caller gets findings and
// no WSL process at all (probeWSL, probe_eval.go). Each target's existence is
// first confirmed through the Windows API; a WSL denial counts only for a
// target that exists, and only an explicit per-attempt "Permission denied" /
// "Operation not permitted" is a denial (absent, errors, silence: UNVERIFIED).
//
// Coverage is ONE translation in the DEFAULT distro, reported in r.WSL, and
// only when the mount evidence ties that path to the whole Windows drive with
// no bind/overlay/nested mount on it (verifyWSLMount); every other distro and
// mount path stays UNVERIFIED (see WSLNotCovered).
// Non-destructive: no file is created, truncated or written; one probe
// directory is created and removed only if creation is (wrongly) allowed.
func ProbeWSL(p Policy, files []string) ProbeReport {
	return probeWSL(p, files, wslRunner{
		token:  readCallerToken,
		exists: windowsExistence,
		lookPath: func() error {
			_, err := exec.LookPath("wsl.exe")
			return err
		},
		run: func(args []string) ([]byte, error) { return exec.Command("wsl.exe", args...).CombinedOutput() },
	})
}
