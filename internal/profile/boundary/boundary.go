// Package boundary is P0 of D2 (threat model B): a READ-ONLY, fail-closed check
// of whether a separately protected Identity profile is isolated, at the file
// and ACL level, from processes running as the owner's normal Windows account.
//
// It changes nothing: no ACL, owner, account, service, file or grant is
// created or modified. It is not wired into either Identity service yet.
//
// Two halves, both needed for an acceptance verdict:
//
//   - Check (run AS the protected Identity account): the effective process
//     identity is the designated service account. Every object in the
//     protected root, and every ancestor directory up to the volume root, is
//     inspected through an Inspector (real Windows security descriptors, or a
//     fake in tests). Only the service account, SYSTEM, TrustedInstaller and
//     Administrators may hold rights inside the root. No other principal,
//     including the normal user, any unknown SID and every broad group, may
//     read, write, delete, re-ACL, take ownership, or replace or retarget any
//     part of the path. Reparse points, hard links, redirected final paths,
//     non-fixed volumes, WSL/UNC paths, null DACLs, unprotected (inheriting)
//     root DACLs, untrusted inheritable ACEs and unknown ACE types fail.
//   - Probe (run AS the normal user, Windows only, see probe_windows.go): its
//     token is a standard user with no linked administrator token, and real
//     open/create attempts against the protected paths are denied.
//   - ProbeWSL (same account, Windows only): the same caller gate as Probe,
//     then read/write/list/create attempts through ONE wslpath translation in
//     the default WSL distro. Only an explicit per-attempt denial on a target
//     the Windows API confirms exists counts; absence, errors and silence are
//     UNVERIFIED, and the tested distro and mount are reported (probe_eval.go).
//
// Administrators are trusted writers because the owner put administrative
// host compromise outside this guarantee. That is only meaningful when the
// normal account is NOT an administrator (UAC is not a security boundary),
// which only Probe can establish. Check therefore never claims more than
// "file/ACL boundary holds for these paths at this instant".
package boundary

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Verdict is the overall result. Only Isolated passes; everything else fails closed.
type Verdict string

const (
	Isolated    Verdict = "ISOLATED"     // every check ran and passed (file/ACL scope only)
	NotIsolated Verdict = "NOT_ISOLATED" // a concrete violation was found
	Unverified  Verdict = "UNVERIFIED"   // something could not be determined
	Unsupported Verdict = "UNSUPPORTED"  // no reliable OS query on this platform
)

// Code identifies one finding.
type Code string

const (
	CPolicy          Code = "POLICY_INVALID"
	CPath            Code = "PATH_REJECTED"
	CProcessQuery    Code = "PROCESS_IDENTITY_UNKNOWN"
	CProcessWrong    Code = "PROCESS_NOT_SERVICE_ACCOUNT"
	CProcessIsNormal Code = "PROCESS_IS_NORMAL_USER"
	CInspect         Code = "OBJECT_UNREADABLE"
	CDescriptor      Code = "DESCRIPTOR_INCOMPLETE"
	CNullDACL        Code = "NULL_DACL"
	CUnknownACE      Code = "UNKNOWN_ACE_TYPE"
	COwner           Code = "UNTRUSTED_OWNER"
	CAccess          Code = "UNTRUSTED_ACCESS"
	CInheritable     Code = "UNTRUSTED_INHERITABLE_ACE"
	CRootInherits    Code = "ROOT_DACL_NOT_PROTECTED"
	CReplace         Code = "ANCESTOR_REPLACEABLE"
	CReparse         Code = "REPARSE_POINT"
	CRedirect        Code = "PATH_REDIRECTED"
	CHardLink        Code = "HARD_LINKED"
	CVolume          Code = "VOLUME_NOT_LOCAL_FIXED"
	CRequired        Code = "REQUIRED_FILE_MISSING"
	CTooMany         Code = "TREE_TOO_LARGE"
	CUnsupported     Code = "PLATFORM_UNSUPPORTED"
	CNotDir          Code = "ROOT_NOT_DIRECTORY"
)

// Finding is one result line. Violation=true means NOT_ISOLATED; false means UNVERIFIED.
type Finding struct {
	Code      Code   `json:"code"`
	Path      string `json:"path,omitempty"`
	SID       string `json:"sid,omitempty"`
	Detail    string `json:"detail"`
	Violation bool   `json:"violation"`
}

// NotCovered is part of every report so that a pass is never read as more than it is.
var NotCovered = []string{
	"localhost / named-pipe / IPC endpoints and who may call them",
	"authorization endpoint behaviour (approvals, grants, tickets, bound decisions)",
	"control of the running Identity process (OpenProcess, debugging, injection, termination)",
	"client-server pinning between Thought Processor / Core / Engine and Identity",
	"key management, signing keys, verification anchors",
	"revocation, rollback and backup/restore of authority records",
	"memory privacy of the running service",
	"cross-entity storage confidentiality (each entity's own data and grants)",
	"D2 Trust writer, attestations and Option B batches (not implemented by P0)",
	"the owner's installed Identity revision and real account/ACL configuration",
	"time-of-check/time-of-use: results hold only at the instant of the check",
	"normal account is a standard user (UAC is not a boundary): only the normal-user Probe / ProbeWSL check this",
	"the instance locator under the normal user's LOCALAPPDATA (a locator, not a trust signal)",
}

// Well-known SIDs.
const (
	SIDSystem         = "S-1-5-18"
	SIDAdministrators = "S-1-5-32-544"
	SIDTrustedInstall = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	SIDCreatorOwner   = "S-1-3-0"
	SIDCreatorGroup   = "S-1-3-1"
	SIDOwnerRights    = "S-1-3-4"
	SIDEveryone       = "S-1-1-0"
	SIDAuthenticated  = "S-1-5-11"
	SIDUsers          = "S-1-5-32-545"
	SIDLocalService   = "S-1-5-19"
	SIDNetworkService = "S-1-5-20"
	SIDInteractive    = "S-1-5-4"
)

var sidRE = regexp.MustCompile(`^S-1-[0-9]+(-[0-9]+)+$`)

// Access rights (winnt.h). Generic bits are mapped before evaluation.
const (
	ReadData        = 0x0001 // FILE_READ_DATA / FILE_LIST_DIRECTORY
	WriteData       = 0x0002 // FILE_WRITE_DATA / FILE_ADD_FILE
	AppendData      = 0x0004 // FILE_APPEND_DATA / FILE_ADD_SUBDIRECTORY
	ReadEA          = 0x0008
	WriteEA         = 0x0010
	Execute         = 0x0020 // FILE_EXECUTE / FILE_TRAVERSE
	DeleteChild     = 0x0040
	ReadAttributes  = 0x0080
	WriteAttributes = 0x0100
	Delete          = 0x00010000
	ReadControl     = 0x00020000
	WriteDAC        = 0x00040000
	WriteOwner      = 0x00080000
	Synchronize     = 0x00100000
	GenericAll      = 0x10000000
	GenericExecute  = 0x20000000
	GenericWrite    = 0x40000000
	GenericRead     = 0x80000000

	fileAllAccess = 0x001F01FF
	fileGenRead   = ReadControl | ReadData | ReadAttributes | ReadEA | Synchronize
	fileGenWrite  = ReadControl | WriteData | WriteAttributes | WriteEA | AppendData | Synchronize
	fileGenExec   = ReadControl | ReadAttributes | Execute | Synchronize

	// harmless: rights that reveal nothing and change nothing.
	harmless = Synchronize | ReadAttributes | ReadControl
	// replaceRights: rights on an ancestor that allow replacing or retargeting the protected path.
	replaceRights = Delete | DeleteChild | WriteDAC | WriteOwner
)

// MapGeneric expands generic bits the way the file object type does.
func MapGeneric(m uint32) uint32 {
	if m&GenericAll != 0 {
		m |= fileAllAccess
	}
	if m&GenericRead != 0 {
		m |= fileGenRead
	}
	if m&GenericWrite != 0 {
		m |= fileGenWrite
	}
	if m&GenericExecute != 0 {
		m |= fileGenExec
	}
	return m &^ (GenericAll | GenericRead | GenericWrite | GenericExecute)
}

// ACE types and flags.
const (
	AceAllowed = 0x0
	AceDenied  = 0x1
	// Object / callback / conditional types (5, 6, 9, 10, 11, 12, …) are not
	// evaluated: allowed variants are UNVERIFIED; denied variants are ignored.
	AceDeniedObject         = 0x6
	AceDeniedCallback       = 0xA
	AceDeniedCallbackObject = 0xC

	FlagObjectInherit    = 0x01
	FlagContainerInherit = 0x02
	FlagInheritOnly      = 0x08
	FlagInherited        = 0x10
)

// ACE is one access control entry as the OS reports it.
type ACE struct {
	Type  uint8
	Flags uint8
	Mask  uint32
	SID   string
}

// Descriptor is the owner and DACL of one object.
type Descriptor struct {
	Owner         string
	DACLPresent   bool // false = NULL DACL = everyone full control
	DACLProtected bool // SE_DACL_PROTECTED: does not inherit from the parent
	DACL          []ACE
}

// Volume classification.
type VolumeKind string

const (
	VolumeFixed   VolumeKind = "FIXED"
	VolumeOther   VolumeKind = "OTHER" // remote, removable, RAM disk, CD …
	VolumeUnknown VolumeKind = "UNKNOWN"
)

// Object is what the Inspector reports about one path, opened WITHOUT following reparse points.
type Object struct {
	Path       string
	IsDir      bool
	Reparse    bool
	LinkCount  uint32
	FinalPath  string // OS-resolved final path of the opened handle
	Volume     VolumeKind
	VolumeID   uint32
	Descriptor *Descriptor
}

// Process is the effective identity of the calling process.
type Process struct {
	UserSID string
}

// Inspector is the only OS dependency. Implementations must not follow
// reparse points when opening an object, and must report errors, not guess.
type Inspector interface {
	Process() (Process, error)
	Inspect(path string) (Object, error)
	List(dir string) ([]string, error) // names of direct children
}

// ErrUnsupported is returned by inspectors that cannot query security descriptors.
var ErrUnsupported = errors.New("protected-profile boundary checks are only implemented for Windows")

// Policy names the protected root and the principals.
type Policy struct {
	Root          string   // absolute Windows path of the protected profile root
	ServiceSID    string   // the dedicated Identity account (S-1-5-21-… local user or S-1-5-80-… virtual service account)
	NormalUserSID string   // the owner's everyday account (S-1-5-21-…)
	Required      []string // files (relative, forward or back slashes) that must exist in Root
	MaxObjects    int      // tree size bound (default 10000)
}

// AdminProfile is the policy for the Admin-approval path (main.go / internal/app / internal/authority).
func AdminProfile(root, serviceSID, normalSID string) Policy {
	return Policy{Root: root, ServiceSID: serviceSID, NormalUserSID: normalSID, Required: []string{"profile.json"}}
}

// EntityProfile is the policy for the TASS entity path (cmd/tass-identity / internal/entity).
func EntityProfile(root, serviceSID, normalSID string) Policy {
	return Policy{Root: root, ServiceSID: serviceSID, NormalUserSID: normalSID, Required: []string{"entity.json", "credential"}}
}

// AdminAuthorityFiles and EntityAuthorityFiles are the sensitive files of each
// path at c505340 (for the Probe and for reports). Check walks the whole root
// regardless, so files created later are covered by the directory checks.
var (
	AdminAuthorityFiles = []string{"profile.json", "settings.json", "admin.json", "admin-throttle.json", "approvals.json",
		"grants.json", "installations.json", "installation-keys.json", "installation-nonces.json", "paired-clients.json",
		"paired-nonces.json", "identities.json", "drafts.json", "github-app.json", "logs/identity-events.jsonl", "runtime/service.json"}
	EntityAuthorityFiles = []string{"entity.json", "credential", "decisions.jsonl"}
)

// Report is the result of Check.
type Report struct {
	Verdict    Verdict   `json:"verdict"`
	Root       string    `json:"root"`
	ProcessSID string    `json:"process_sid,omitempty"`
	Objects    int       `json:"objects_checked"`
	Ancestors  int       `json:"ancestors_checked"`
	Findings   []Finding `json:"findings"`
	Scope      string    `json:"scope"`
	NotCovered []string  `json:"not_covered"`
}

// Passed is true only for ISOLATED.
func (r Report) Passed() bool { return r.Verdict == Isolated }

func (r *Report) add(violation bool, c Code, p, sid, f string, a ...any) {
	r.Findings = append(r.Findings, Finding{Code: c, Path: p, SID: sid, Detail: fmt.Sprintf(f, a...), Violation: violation})
}

const scope = "FILE/ACL BOUNDARY ONLY, at one instant: the protected root and its ancestors, as reported by the OS. " +
	"Not a D2 Trust, IPC, process-control or installed-system claim."

// trusted reports whether sid may hold rights inside the protected root.
func (p Policy) trusted(sid string) bool {
	switch strings.ToUpper(sid) {
	case strings.ToUpper(p.ServiceSID), SIDSystem, SIDAdministrators, strings.ToUpper(SIDTrustedInstall):
		return true
	}
	return false
}

func validPrincipal(sid string, allowVirtual bool) error {
	s := strings.ToUpper(sid)
	if !sidRE.MatchString(s) {
		return fmt.Errorf("%q is not a SID string", sid)
	}
	switch {
	case strings.HasPrefix(s, "S-1-5-21-") && strings.Count(s, "-") >= 7:
		return nil // domain or local account
	case allowVirtual && strings.HasPrefix(s, "S-1-5-80-") && s != strings.ToUpper(SIDTrustedInstall):
		return nil // NT SERVICE\<name> virtual account (exclusive to one service)
	}
	return fmt.Errorf("%s is a shared or well-known principal, not a dedicated account", sid)
}

func (p Policy) validate() error {
	if err := validPrincipal(p.ServiceSID, true); err != nil {
		return fmt.Errorf("service account: %w", err)
	}
	if err := validPrincipal(p.NormalUserSID, false); err != nil {
		return fmt.Errorf("normal user: %w", err)
	}
	if strings.EqualFold(p.ServiceSID, p.NormalUserSID) {
		return errors.New("the service account and the normal user are the same principal")
	}
	for _, r := range p.Required {
		c := path.Clean(strings.ReplaceAll(r, `\`, "/"))
		if c == "." || strings.HasPrefix(c, "../") || c == ".." || path.IsAbs(c) || strings.Contains(c, ":") {
			return fmt.Errorf("required file %q must be a relative path inside the root", r)
		}
	}
	return nil
}

// Check runs the service-side boundary check. It never modifies anything.
func Check(in Inspector, p Policy) (r Report) {
	r = Report{Root: p.Root, Scope: scope, NotCovered: append([]string(nil), NotCovered...)}
	defer func() { r.Verdict = verdictOf(r.Findings) }()
	if p.MaxObjects <= 0 {
		p.MaxObjects = 10000
	}
	if err := p.validate(); err != nil {
		r.add(false, CPolicy, "", "", "%v", err)
		return r
	}
	root, ancestors, err := SplitWindowsPath(p.Root)
	if err != nil {
		r.add(true, CPath, p.Root, "", "%v", err)
		return r
	}
	r.Root = root

	proc, err := in.Process()
	switch {
	case errors.Is(err, ErrUnsupported):
		r.add(false, CUnsupported, "", "", "%v", err)
		return r
	case err != nil:
		r.add(false, CProcessQuery, "", "", "%v", err)
		return r
	}
	r.ProcessSID = proc.UserSID
	switch {
	case strings.EqualFold(proc.UserSID, p.NormalUserSID):
		r.add(true, CProcessIsNormal, "", proc.UserSID, "this check is running as the normal user, not as the protected Identity account")
		return r
	case !strings.EqualFold(proc.UserSID, p.ServiceSID):
		r.add(true, CProcessWrong, "", proc.UserSID, "effective process user is %s; the designated Identity account is %s", proc.UserSID, p.ServiceSID)
		return r
	}

	// The root itself.
	ro, ok := inspect(in, &r, root)
	if !ok {
		return r
	}
	if !ro.IsDir {
		r.add(true, CNotDir, root, "", "the protected root is not a directory")
		return r
	}
	if ro.FinalPath == "" {
		r.add(false, CRedirect, root, "", "the OS did not report a final path for the root")
	} else if !samePath(ro.FinalPath, root) {
		r.add(true, CRedirect, root, "", "the root resolves to %s (subst, mount point, junction or short-name redirection)", ro.FinalPath)
	}
	if d := ro.Descriptor; d != nil && d.DACLPresent && !d.DACLProtected {
		r.add(true, CRootInherits, root, "", "the root DACL inherits from its parent; it must be protected (inheritance disabled)")
	}

	// Ancestors: must not be replaceable or retargetable by anyone untrusted.
	for _, a := range ancestors {
		ao, ok := inspect(in, &r, a)
		if !ok {
			continue
		}
		r.Ancestors++
		checkCommon(&r, p, ao, ro.VolumeID)
		checkACL(&r, p, ao, replaceRights, false)
	}

	// The whole tree under the root.
	seen := map[string]bool{}
	var walk func(o Object)
	walk = func(o Object) {
		if r.Objects >= p.MaxObjects {
			return
		}
		r.Objects++
		checkCommon(&r, p, o, ro.VolumeID)
		checkACL(&r, p, o, ^uint32(harmless), true)
		if !o.IsDir || o.Reparse {
			return // never descend through a reparse point
		}
		names, err := in.List(o.Path)
		if err != nil {
			r.add(false, CInspect, o.Path, "", "cannot list directory: %v", err)
			return
		}
		sort.Strings(names)
		for _, n := range names {
			if n == "" || n == "." || n == ".." || strings.ContainsAny(n, `\/:`) {
				r.add(true, CPath, o.Path, "", "unexpected directory entry name %q", n)
				continue
			}
			if r.Objects >= p.MaxObjects {
				r.add(false, CTooMany, root, "", "more than %d objects; the rest were not checked", p.MaxObjects)
				return
			}
			cp := o.Path + `\` + n
			seen[strings.ToLower(cp)] = true
			if co, ok := inspect(in, &r, cp); ok {
				walk(co)
			}
		}
	}
	walk(ro)

	for _, req := range p.Required {
		cp := root + `\` + strings.ReplaceAll(path.Clean(strings.ReplaceAll(req, `\`, "/")), "/", `\`)
		if !seen[strings.ToLower(cp)] {
			r.add(false, CRequired, cp, "", "required authority file is missing; this is not the expected profile")
		}
	}
	return r
}

func inspect(in Inspector, r *Report, p string) (Object, bool) {
	o, err := in.Inspect(p)
	if err != nil {
		r.add(false, CInspect, p, "", "%v", err)
		return Object{}, false
	}
	if o.Path == "" {
		o.Path = p
	}
	return o, true
}

func checkCommon(r *Report, p Policy, o Object, rootVolume uint32) {
	if o.Reparse {
		r.add(true, CReparse, o.Path, "", "reparse point (junction, symbolic link, mount point or cloud placeholder) in the protected path")
	}
	if !o.IsDir && o.LinkCount != 1 {
		if o.LinkCount == 0 {
			r.add(false, CHardLink, o.Path, "", "link count unknown")
		} else {
			r.add(true, CHardLink, o.Path, "", "file has %d hard links; another name may live outside the boundary", o.LinkCount)
		}
	}
	switch o.Volume {
	case VolumeFixed:
		if o.VolumeID != rootVolume {
			r.add(true, CVolume, o.Path, "", "object is on a different volume than the root")
		}
	case VolumeUnknown, "":
		r.add(false, CVolume, o.Path, "", "volume type unknown")
	default:
		r.add(true, CVolume, o.Path, "", "object is not on a local fixed volume (%s)", o.Volume)
	}
}

// checkACL fails any untrusted principal that holds a right in forbidden on o.
// inTree adds the protected-tree rules (inheritable ACEs).
func checkACL(r *Report, p Policy, o Object, forbidden uint32, inTree bool) {
	d := o.Descriptor
	if d == nil || d.Owner == "" {
		r.add(false, CDescriptor, o.Path, "", "owner or DACL could not be read")
		return
	}
	if !sidRE.MatchString(strings.ToUpper(d.Owner)) {
		r.add(false, CDescriptor, o.Path, d.Owner, "owner is not a SID")
		return
	}
	if !p.trusted(d.Owner) {
		r.add(true, COwner, o.Path, d.Owner, "owned by an untrusted principal (an owner can always rewrite the DACL)")
	}
	if !d.DACLPresent {
		r.add(true, CNullDACL, o.Path, "", "NULL DACL: everyone has full control")
		return
	}
	for _, a := range d.DACL {
		switch a.Type {
		case AceAllowed:
		case AceDenied, AceDeniedObject, AceDeniedCallback, AceDeniedCallbackObject:
			continue // deny ACEs are never credited: a pass must not depend on ACE order
		default:
			r.add(false, CUnknownACE, o.Path, a.SID, "ACE type %d is not evaluated (object/callback/conditional)", a.Type)
			continue
		}
		sid := strings.ToUpper(a.SID)
		if !sidRE.MatchString(sid) {
			r.add(false, CDescriptor, o.Path, a.SID, "ACE principal is not a SID")
			continue
		}
		mask := MapGeneric(a.Mask)
		applies := a.Flags&FlagInheritOnly == 0
		inheritable := o.IsDir && a.Flags&(FlagObjectInherit|FlagContainerInherit) != 0
		switch sid {
		case SIDCreatorOwner, SIDCreatorGroup:
			// Placeholders: only meaningful as inherit-only templates; the
			// creator must already have been allowed to create (checked separately).
			if applies {
				r.add(false, CUnknownACE, o.Path, a.SID, "creator placeholder applied to an existing object")
			}
			continue
		case SIDOwnerRights:
			if p.trusted(d.Owner) {
				continue
			}
		}
		if p.trusted(sid) {
			continue
		}
		who := "untrusted principal"
		if strings.EqualFold(sid, p.NormalUserSID) {
			who = "the normal user"
		}
		if applies && mask&forbidden != 0 {
			c := CAccess
			if !inTree {
				c = CReplace
			}
			r.add(true, c, o.Path, a.SID, "%s holds 0x%08x (%s)", who, mask&forbidden, describe(mask&forbidden))
		}
		if inTree && inheritable && mask&^uint32(harmless) != 0 {
			r.add(true, CInheritable, o.Path, a.SID, "%s receives 0x%08x on objects created here later", who, mask&^uint32(harmless))
		}
	}
}

func describe(m uint32) string {
	names := []struct {
		b uint32
		n string
	}{{ReadData, "read/list"}, {WriteData, "write/add-file"}, {AppendData, "append/add-subdir"}, {ReadEA, "read-ea"},
		{WriteEA, "write-ea"}, {Execute, "execute/traverse"}, {DeleteChild, "delete-child"}, {WriteAttributes, "write-attributes"},
		{Delete, "delete"}, {WriteDAC, "write-dac"}, {WriteOwner, "take-ownership"}}
	var out []string
	for _, x := range names {
		if m&x.b != 0 {
			out = append(out, x.n)
		}
	}
	if len(out) == 0 {
		return "other"
	}
	return strings.Join(out, ",")
}

func verdictOf(fs []Finding) Verdict {
	v := Isolated
	for _, f := range fs {
		switch {
		case f.Code == CUnsupported:
			return Unsupported
		case f.Violation:
			v = NotIsolated
		case v == Isolated:
			v = Unverified
		}
	}
	return v
}
