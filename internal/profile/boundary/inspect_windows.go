//go:build windows

package boundary

// Real Windows inspector. Read-only: every object is opened with
// READ_CONTROL | FILE_READ_ATTRIBUTES and FILE_FLAG_OPEN_REPARSE_POINT, so a
// junction, symbolic link or mount point is reported as itself and never
// followed. Security descriptors come from GetSecurityInfo on that handle,
// so the descriptor, attributes and final path describe the same object.
//
// Status: compiled (GOOS=windows go vet/build). Behaviour on a real Windows 11
// host is UNVERIFIED until windows_test.go runs there.

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	advapi32                      = syscall.NewLazyDLL("advapi32.dll")
	kernel32                      = syscall.NewLazyDLL("kernel32.dll")
	procGetSecurityInfo           = advapi32.NewProc("GetSecurityInfo")
	procGetSecurityDescriptorCtl  = advapi32.NewProc("GetSecurityDescriptorControl")
	procGetAce                    = advapi32.NewProc("GetAce")
	procGetFinalPathNameByHandleW = kernel32.NewProc("GetFinalPathNameByHandleW")
	procGetDriveTypeW             = kernel32.NewProc("GetDriveTypeW")
)

const (
	seFileObject           = 1
	ownerSecurityInfo      = 0x1
	daclSecurityInfo       = 0x4
	seDACLPresent          = 0x0004
	seDACLProtected        = 0x1000
	fileReadAttributes     = 0x80
	fileFlagBackupSemantic = 0x02000000
	fileFlagOpenReparse    = 0x00200000
	fileAttrReparse        = 0x400
	fileAttrDirectory      = 0x10
	driveFixed             = 3
)

// windowsInspector implements Inspector with the Win32 security API.
type windowsInspector struct{}

// NewInspector returns the OS inspector for this platform.
func NewInspector() Inspector { return windowsInspector{} }

func (windowsInspector) Process() (Process, error) {
	t, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return Process{}, fmt.Errorf("open process token: %w", err)
	}
	defer t.Close()
	u, err := t.GetTokenUser()
	if err != nil {
		return Process{}, fmt.Errorf("token user: %w", err)
	}
	s, err := u.User.Sid.String()
	if err != nil {
		return Process{}, fmt.Errorf("token user SID: %w", err)
	}
	return Process{UserSID: s}, nil
}

func openNoFollow(p string, access uint32) (syscall.Handle, error) {
	u, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	return syscall.CreateFile(u, access, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, fileFlagBackupSemantic|fileFlagOpenReparse, 0)
}

func (windowsInspector) Inspect(p string) (Object, error) {
	h, err := openNoFollow(p, ReadControl|fileReadAttributes)
	if err != nil {
		return Object{}, fmt.Errorf("open %s: %w", p, err)
	}
	defer syscall.CloseHandle(h)
	o := Object{Path: p}

	var fi syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &fi); err != nil {
		return Object{}, fmt.Errorf("file information %s: %w", p, err)
	}
	o.IsDir = fi.FileAttributes&fileAttrDirectory != 0
	o.Reparse = fi.FileAttributes&fileAttrReparse != 0
	o.LinkCount = fi.NumberOfLinks
	o.VolumeID = fi.VolumeSerialNumber

	buf := make([]uint16, 1024)
	n, _, e := procGetFinalPathNameByHandleW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if n == 0 || int(n) >= len(buf) {
		return Object{}, fmt.Errorf("final path %s: %v", p, e)
	}
	o.FinalPath = syscall.UTF16ToString(buf[:n])

	o.Volume = VolumeUnknown
	if fp := strings.TrimPrefix(o.FinalPath, `\\?\`); len(fp) >= 3 && fp[1] == ':' && fp[2] == '\\' {
		if r, err := syscall.UTF16PtrFromString(fp[:3]); err == nil {
			if dt, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(r))); dt == driveFixed {
				o.Volume = VolumeFixed
			} else {
				o.Volume = VolumeOther
			}
		}
	} else {
		o.Volume = VolumeOther // \\?\UNC\… or \\?\Volume{…}
	}

	d, err := descriptorOf(h)
	if err != nil {
		return Object{}, fmt.Errorf("security descriptor %s: %w", p, err)
	}
	o.Descriptor = d
	return o, nil
}

func descriptorOf(h syscall.Handle) (*Descriptor, error) {
	var owner, dacl, sd unsafe.Pointer // filled by GetSecurityInfo; sd owns the memory (LocalFree)
	rc, _, _ := procGetSecurityInfo.Call(uintptr(h), seFileObject, ownerSecurityInfo|daclSecurityInfo,
		uintptr(unsafe.Pointer(&owner)), 0, uintptr(unsafe.Pointer(&dacl)), 0, uintptr(unsafe.Pointer(&sd)))
	if rc != 0 {
		return nil, syscall.Errno(rc)
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(sd)))
	var ctl uint16
	var rev uint32
	if ok, _, e := procGetSecurityDescriptorCtl.Call(uintptr(sd), uintptr(unsafe.Pointer(&ctl)), uintptr(unsafe.Pointer(&rev))); ok == 0 {
		return nil, fmt.Errorf("descriptor control: %v", e)
	}
	if owner == nil {
		return nil, errors.New("no owner in descriptor")
	}
	ownerSID, err := (*syscall.SID)(owner).String()
	if err != nil {
		return nil, err
	}
	d := &Descriptor{Owner: ownerSID, DACLPresent: ctl&seDACLPresent != 0 && dacl != nil, DACLProtected: ctl&seDACLProtected != 0}
	if !d.DACLPresent {
		return d, nil
	}
	count := *(*uint16)(unsafe.Add(dacl, 4)) // ACL.AceCount
	for i := 0; i < int(count); i++ {
		var ace unsafe.Pointer
		if ok, _, e := procGetAce.Call(uintptr(dacl), uintptr(i), uintptr(unsafe.Pointer(&ace))); ok == 0 || ace == nil {
			return nil, fmt.Errorf("ACE %d: %v", i, e)
		}
		a := ACE{Type: *(*uint8)(ace), Flags: *(*uint8)(unsafe.Add(ace, 1))}
		if a.Type == AceAllowed || a.Type == AceDenied {
			a.Mask = *(*uint32)(unsafe.Add(ace, 4))
			s, err := (*syscall.SID)(unsafe.Add(ace, 8)).String()
			if err != nil {
				return nil, fmt.Errorf("ACE %d SID: %w", i, err)
			}
			a.SID = s
		}
		d.DACL = append(d.DACL, a)
	}
	return d, nil
}

func (windowsInspector) List(dir string) ([]string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out, nil
}
