package boundary

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- fake OS ---------------------------------------------------------------------------

const (
	svc    = "S-1-5-80-1111111111-2222222222-3333333333-4444444444-555555555" // NT SERVICE\Identity (virtual account)
	normal = "S-1-5-21-1000000001-2000000002-3000000003-1001"                 // the owner's everyday account
	other  = "S-1-5-21-1000000001-2000000002-3000000003-1002"                 // some other local account
	full   = 0x001F01FF
	rx     = 0x001200A9 // read & execute
	modify = 0x001301BF

	adminRoot  = `C:\ProgramData\Arameair\IdentityProtected`
	entityRoot = `C:\ProgramData\Arameair\TassIdentity`
)

type fakeOS struct {
	proc    Process
	procErr error
	objs    map[string]*Object // lower-case path → object
	kids    map[string][]string
	inspErr map[string]error
	listErr map[string]error
	listed  []string
}

func (f *fakeOS) Process() (Process, error) { return f.proc, f.procErr }
func (f *fakeOS) Inspect(p string) (Object, error) {
	if err := f.inspErr[strings.ToLower(p)]; err != nil {
		return Object{}, err
	}
	o, ok := f.objs[strings.ToLower(p)]
	if !ok {
		return Object{}, errors.New("The system cannot find the file specified.")
	}
	c := *o
	if o.Descriptor != nil {
		d := *o.Descriptor
		d.DACL = append([]ACE(nil), o.Descriptor.DACL...)
		c.Descriptor = &d
	}
	return c, nil
}
func (f *fakeOS) List(d string) ([]string, error) {
	f.listed = append(f.listed, d)
	if err := f.listErr[strings.ToLower(d)]; err != nil {
		return nil, err
	}
	return f.kids[strings.ToLower(d)], nil
}

func (f *fakeOS) put(p string, dir bool, owner string, protected bool, aces ...ACE) *Object {
	o := &Object{Path: p, IsDir: dir, LinkCount: 1, FinalPath: `\\?\` + p, Volume: VolumeFixed, VolumeID: 7,
		Descriptor: &Descriptor{Owner: owner, DACLPresent: true, DACLProtected: protected, DACL: aces}}
	if dir {
		o.LinkCount = 0
	}
	f.objs[strings.ToLower(p)] = o
	if i := strings.LastIndex(p, `\`); i > 2 {
		parent := strings.ToLower(p[:i])
		f.kids[parent] = append(f.kids[parent], p[i+1:])
	}
	return o
}

func (f *fakeOS) get(p string) *Object { return f.objs[strings.ToLower(p)] }

func allow(sid string, mask uint32, flags uint8) ACE {
	return ACE{Type: AceAllowed, Flags: flags, Mask: mask, SID: sid}
}

// world builds a realistic, correctly protected layout under C:\ProgramData,
// with stock Windows ACLs on C:\ and C:\ProgramData.
func world() *fakeOS {
	f := &fakeOS{proc: Process{UserSID: svc}, objs: map[string]*Object{}, kids: map[string][]string{},
		inspErr: map[string]error{}, listErr: map[string]error{}}
	oici, io := uint8(FlagObjectInherit|FlagContainerInherit), uint8(FlagInheritOnly|FlagObjectInherit|FlagContainerInherit)
	f.put(`C:\`, true, SIDTrustedInstall, true,
		allow(SIDAdministrators, full, oici), allow(SIDSystem, full, oici), allow(SIDUsers, rx, oici),
		allow(SIDAuthenticated, AppendData, 0), allow(SIDAuthenticated, modify, io), allow(SIDCreatorOwner, GenericAll, io))
	f.put(`C:\ProgramData`, true, SIDSystem, true,
		allow(SIDSystem, full, oici), allow(SIDAdministrators, full, oici), allow(SIDCreatorOwner, GenericAll, io),
		allow(SIDUsers, rx, oici), allow(SIDUsers, WriteData|AppendData|WriteEA|WriteAttributes, FlagContainerInherit))
	f.put(`C:\ProgramData\Arameair`, true, SIDAdministrators, false,
		allow(SIDSystem, full, oici|FlagInherited), allow(SIDAdministrators, full, oici|FlagInherited),
		allow(SIDUsers, rx, oici|FlagInherited), allow(SIDUsers, WriteData|AppendData|WriteEA|WriteAttributes, FlagContainerInherit|FlagInherited))
	for _, root := range []string{adminRoot, entityRoot} {
		f.put(root, true, svc, true, allow(svc, full, oici), allow(SIDSystem, full, oici), allow(SIDAdministrators, full, oici))
	}
	inh := func() []ACE {
		return []ACE{allow(svc, full, FlagInherited), allow(SIDSystem, full, FlagInherited), allow(SIDAdministrators, full, FlagInherited)}
	}
	inhDir := func() []ACE {
		return []ACE{allow(svc, full, oici|FlagInherited), allow(SIDSystem, full, oici|FlagInherited), allow(SIDAdministrators, full, oici|FlagInherited)}
	}
	for _, n := range []string{"profile.json", "settings.json", "admin.json", "admin-throttle.json", "approvals.json", "grants.json", "paired-clients.json"} {
		f.put(adminRoot+`\`+n, false, svc, false, inh()...)
	}
	f.put(adminRoot+`\runtime`, true, svc, false, inhDir()...)
	f.put(adminRoot+`\runtime\service.json`, false, svc, false, inh()...)
	f.put(adminRoot+`\logs`, true, svc, false, inhDir()...)
	f.put(adminRoot+`\logs\identity-events.jsonl`, false, svc, false, inh()...)
	for _, n := range []string{"entity.json", "credential", "decisions.jsonl"} {
		f.put(entityRoot+`\`+n, false, svc, false, inh()...)
	}
	return f
}

func adminPolicy() Policy  { return AdminProfile(adminRoot, svc, normal) }
func entityPolicy() Policy { return EntityProfile(entityRoot, svc, normal) }

func has(r Report, c Code) bool {
	for _, f := range r.Findings {
		if f.Code == c {
			return true
		}
	}
	return false
}

func dump(r Report) string {
	var b strings.Builder
	for _, f := range r.Findings {
		b.WriteString("\n  " + string(f.Code) + " " + f.Path + " " + f.SID + " " + f.Detail)
	}
	return b.String()
}

// ---- tests -----------------------------------------------------------------------------

func TestGoodProtectedProfilesAreIsolated(t *testing.T) {
	for name, p := range map[string]Policy{"admin": adminPolicy(), "entity": entityPolicy()} {
		f := world()
		r := Check(f, p)
		if r.Verdict != Isolated || !r.Passed() {
			t.Fatalf("%s: verdict %s, want ISOLATED:%s", name, r.Verdict, dump(r))
		}
		if r.Ancestors != 3 || r.Objects < 4 {
			t.Fatalf("%s: checked %d ancestors and %d objects", name, r.Ancestors, r.Objects)
		}
		if r.Scope == "" || len(r.NotCovered) < 10 {
			t.Fatalf("%s: a pass must carry its scope and the not-covered list", name)
		}
	}
}

// Every negative mutation must fail closed with the stated verdict and code.
func TestNegativeBoundaryCases(t *testing.T) {
	type tc struct {
		name   string
		policy func() Policy
		mutate func(f *fakeOS)
		want   Verdict
		code   Code
	}
	file := func(n string) string { return adminRoot + `\` + n }
	addACE := func(p string, a ACE) func(*fakeOS) {
		return func(f *fakeOS) { o := f.get(p); o.Descriptor.DACL = append(o.Descriptor.DACL, a) }
	}
	cases := []tc{
		// file tampering / unsafe ACLs
		{"normal user may write admin.json", adminPolicy, addACE(file("admin.json"), allow(normal, WriteData, 0)), NotIsolated, CAccess},
		{"normal user may write approvals.json (forged approval)", adminPolicy, addACE(file("approvals.json"), allow(normal, modify, 0)), NotIsolated, CAccess},
		{"normal user may delete admin-throttle.json (throttle reset)", adminPolicy, addACE(file("admin-throttle.json"), allow(normal, Delete, 0)), NotIsolated, CAccess},
		{"normal user may edit entity.json grants", entityPolicy, addACE(entityRoot+`\entity.json`, allow(normal, WriteData|AppendData, 0)), NotIsolated, CAccess},
		{"normal user may read the entity credential", entityPolicy, addACE(entityRoot+`\credential`, allow(normal, ReadData, 0)), NotIsolated, CAccess},
		{"normal user may read pairing keys", adminPolicy, addACE(file("paired-clients.json"), allow(normal, rx, 0)), NotIsolated, CAccess},
		{"normal user may re-ACL a file", adminPolicy, addACE(file("grants.json"), allow(normal, WriteDAC, 0)), NotIsolated, CAccess},
		{"normal user may take ownership", adminPolicy, addACE(file("grants.json"), allow(normal, WriteOwner, 0)), NotIsolated, CAccess},
		{"GENERIC_WRITE is mapped, not ignored", adminPolicy, addACE(file("admin.json"), allow(normal, GenericWrite, 0)), NotIsolated, CAccess},
		{"GENERIC_ALL on root", adminPolicy, addACE(adminRoot, allow(normal, GenericAll, 0)), NotIsolated, CAccess},
		{"Everyone on root", adminPolicy, addACE(adminRoot, allow(SIDEveryone, rx, 0)), NotIsolated, CAccess},
		{"Authenticated Users may add files to root", adminPolicy, addACE(adminRoot, allow(SIDAuthenticated, WriteData, 0)), NotIsolated, CAccess},
		{"Users on runtime dir", adminPolicy, addACE(file("runtime"), allow(SIDUsers, modify, 0)), NotIsolated, CAccess},
		{"Interactive on logs", adminPolicy, addACE(file(`logs\identity-events.jsonl`), allow(SIDInteractive, WriteData, 0)), NotIsolated, CAccess},
		{"deny ACE never rescues an allow", adminPolicy, func(f *fakeOS) {
			o := f.get(file("admin.json"))
			o.Descriptor.DACL = append([]ACE{{Type: AceDenied, Mask: full, SID: normal}}, append(o.Descriptor.DACL, allow(normal, WriteData, 0))...)
		}, NotIsolated, CAccess},
		{"permissive INHERITED ace counts", adminPolicy, addACE(file("settings.json"), allow(SIDUsers, modify, FlagInherited)), NotIsolated, CAccess},
		{"inheritable ace exposes future files", adminPolicy, addACE(adminRoot, allow(SIDUsers, modify, FlagInheritOnly|FlagObjectInherit)), NotIsolated, CInheritable},
		{"root DACL inherits from parent", adminPolicy, func(f *fakeOS) { f.get(adminRoot).Descriptor.DACLProtected = false }, NotIsolated, CRootInherits},
		{"NULL DACL", adminPolicy, func(f *fakeOS) { f.get(file("grants.json")).Descriptor.DACLPresent = false }, NotIsolated, CNullDACL},
		// incorrect security principals
		{"file owned by the normal user", adminPolicy, func(f *fakeOS) { f.get(file("admin.json")).Descriptor.Owner = normal }, NotIsolated, COwner},
		{"root owned by another account", adminPolicy, func(f *fakeOS) { f.get(adminRoot).Descriptor.Owner = other }, NotIsolated, COwner},
		{"unknown account may write", adminPolicy, addACE(file("admin.json"), allow(other, WriteData, 0)), NotIsolated, CAccess},
		{"LocalService (shared) may write", adminPolicy, addACE(file("admin.json"), allow(SIDLocalService, WriteData, 0)), NotIsolated, CAccess},
		{"process runs as the normal user", adminPolicy, func(f *fakeOS) { f.proc.UserSID = normal }, NotIsolated, CProcessIsNormal},
		{"process runs as another account", adminPolicy, func(f *fakeOS) { f.proc.UserSID = other }, NotIsolated, CProcessWrong},
		{"process identity unknown", adminPolicy, func(f *fakeOS) { f.procErr = errors.New("token unavailable") }, Unverified, CProcessQuery},
		{"service and normal user are the same", func() Policy { return AdminProfile(adminRoot, normal, normal) }, nil, Unverified, CPolicy},
		{"service account is Users", func() Policy { return AdminProfile(adminRoot, SIDUsers, normal) }, nil, Unverified, CPolicy},
		{"service account is LocalService", func() Policy { return AdminProfile(adminRoot, SIDLocalService, normal) }, nil, Unverified, CPolicy},
		{"service account is TrustedInstaller", func() Policy { return AdminProfile(adminRoot, SIDTrustedInstall, normal) }, nil, Unverified, CPolicy},
		{"normal user given as Administrators", func() Policy { return AdminProfile(adminRoot, svc, SIDAdministrators) }, nil, Unverified, CPolicy},
		{"normal user not a SID", func() Policy { return AdminProfile(adminRoot, svc, "Dustin") }, nil, Unverified, CPolicy},
		// parent-directory replacement
		{"normal user may delete an ancestor", adminPolicy, addACE(`C:\ProgramData\Arameair`, allow(normal, Delete, 0)), NotIsolated, CReplace},
		{"Users may delete children of ProgramData", adminPolicy, addACE(`C:\ProgramData`, allow(SIDUsers, DeleteChild, 0)), NotIsolated, CReplace},
		{"normal user may delete-child in root's parent", adminPolicy, addACE(`C:\ProgramData\Arameair`, allow(normal, DeleteChild, 0)), NotIsolated, CReplace},
		{"normal user may re-ACL an ancestor", adminPolicy, addACE(`C:\ProgramData\Arameair`, allow(normal, WriteDAC, 0)), NotIsolated, CReplace},
		{"ancestor owned by the normal user", adminPolicy, func(f *fakeOS) { f.get(`C:\ProgramData\Arameair`).Descriptor.Owner = normal }, NotIsolated, COwner},
		{"ancestor generic-all for Authenticated Users", adminPolicy, addACE(`C:\ProgramData\Arameair`, allow(SIDAuthenticated, GenericAll, 0)), NotIsolated, CReplace},
		// link / reparse substitution and redirection
		{"root is a junction", adminPolicy, func(f *fakeOS) { f.get(adminRoot).Reparse = true }, NotIsolated, CReparse},
		{"ancestor is a junction", adminPolicy, func(f *fakeOS) { f.get(`C:\ProgramData\Arameair`).Reparse = true }, NotIsolated, CReparse},
		{"authority file is a symlink", adminPolicy, func(f *fakeOS) { f.get(file("admin.json")).Reparse = true }, NotIsolated, CReparse},
		{"root final path redirected (subst / mount / short name)", adminPolicy, func(f *fakeOS) { f.get(adminRoot).FinalPath = `\\?\D:\Elsewhere` }, NotIsolated, CRedirect},
		{"root final path unknown", adminPolicy, func(f *fakeOS) { f.get(adminRoot).FinalPath = "" }, Unverified, CRedirect},
		{"authority file hard-linked", adminPolicy, func(f *fakeOS) { f.get(file("approvals.json")).LinkCount = 2 }, NotIsolated, CHardLink},
		{"file on another volume", adminPolicy, func(f *fakeOS) { f.get(file("grants.json")).VolumeID = 9 }, NotIsolated, CVolume},
		{"root on a network/removable volume", adminPolicy, func(f *fakeOS) { f.get(adminRoot).Volume = VolumeOther }, NotIsolated, CVolume},
		// missing / ambiguous descriptors
		{"descriptor missing", adminPolicy, func(f *fakeOS) { f.get(file("admin.json")).Descriptor = nil }, Unverified, CDescriptor},
		{"owner missing", adminPolicy, func(f *fakeOS) { f.get(file("admin.json")).Descriptor.Owner = "" }, Unverified, CDescriptor},
		{"owner not a SID", adminPolicy, func(f *fakeOS) { f.get(file("admin.json")).Descriptor.Owner = `NT AUTHORITY\SYSTEM` }, Unverified, CDescriptor},
		{"ACE principal not a SID", adminPolicy, addACE(file("admin.json"), allow("Everyone", WriteData, 0)), Unverified, CDescriptor},
		{"callback (conditional) allow ACE", adminPolicy, addACE(file("admin.json"), ACE{Type: 9, Mask: full}), Unverified, CUnknownACE},
		{"object allow ACE", adminPolicy, addACE(adminRoot, ACE{Type: 5, Mask: full}), Unverified, CUnknownACE},
		{"creator-owner applied to an existing object", adminPolicy, addACE(file("admin.json"), allow(SIDCreatorOwner, full, 0)), Unverified, CUnknownACE},
		{"object unreadable", adminPolicy, func(f *fakeOS) { f.inspErr[strings.ToLower(file("admin.json"))] = errors.New("Access is denied.") }, Unverified, CInspect},
		{"ancestor unreadable", adminPolicy, func(f *fakeOS) { f.inspErr[`c:\programdata`] = errors.New("Access is denied.") }, Unverified, CInspect},
		{"directory unlistable", adminPolicy, func(f *fakeOS) { f.listErr[strings.ToLower(file("runtime"))] = errors.New("Access is denied.") }, Unverified, CInspect},
		{"required authority file missing", entityPolicy, func(f *fakeOS) {
			delete(f.objs, strings.ToLower(entityRoot+`\credential`))
			f.kids[strings.ToLower(entityRoot)] = []string{"entity.json", "decisions.jsonl"}
		}, Unverified, CRequired},
		{"root is a file", adminPolicy, func(f *fakeOS) { f.get(adminRoot).IsDir = false }, NotIsolated, CNotDir},
		{"tree too large", func() Policy { p := adminPolicy(); p.MaxObjects = 3; return p }, nil, Unverified, CTooMany},
		{"violation dominates unverified", adminPolicy, func(f *fakeOS) {
			f.get(file("admin.json")).Descriptor = nil
			o := f.get(file("grants.json"))
			o.Descriptor.DACL = append(o.Descriptor.DACL, allow(normal, WriteData, 0))
		}, NotIsolated, CAccess},
		// paths (WSL, UNC, device, aliasing)
		{"WSL distro path", func() Policy { return AdminProfile(`\\wsl$\Ubuntu\home\d\.identity`, svc, normal) }, nil, NotIsolated, CPath},
		{"WSL localhost path", func() Policy { return AdminProfile(`\\wsl.localhost\Ubuntu\srv\identity`, svc, normal) }, nil, NotIsolated, CPath},
		{"WSL mount path", func() Policy { return AdminProfile(`/mnt/c/ProgramData/Arameair/IdentityProtected`, svc, normal) }, nil, NotIsolated, CPath},
		{"UNC share", func() Policy { return AdminProfile(`\\nas\share\identity`, svc, normal) }, nil, NotIsolated, CPath},
		{"device namespace", func() Policy { return AdminProfile(`\\?\C:\ProgramData\Arameair\IdentityProtected`, svc, normal) }, nil, NotIsolated, CPath},
		{"relative path", func() Policy { return AdminProfile(`identity-data`, svc, normal) }, nil, NotIsolated, CPath},
		{"volume root", func() Policy { return AdminProfile(`C:\`, svc, normal) }, nil, NotIsolated, CPath},
		{"dot-dot", func() Policy { return AdminProfile(`C:\ProgramData\..\Users\d\.identity`, svc, normal) }, nil, NotIsolated, CPath},
		{"trailing dot alias", func() Policy { return AdminProfile(`C:\ProgramData\Arameair\IdentityProtected.`, svc, normal) }, nil, NotIsolated, CPath},
		{"alternate data stream", func() Policy { return AdminProfile(`C:\ProgramData\Arameair\IdentityProtected:x`, svc, normal) }, nil, NotIsolated, CPath},
		{"reserved device name", func() Policy { return AdminProfile(`C:\ProgramData\NUL\identity`, svc, normal) }, nil, NotIsolated, CPath},
		{"required path escapes root", func() Policy { p := adminPolicy(); p.Required = []string{`..\other\admin.json`}; return p }, nil, Unverified, CPolicy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := world()
			if c.mutate != nil {
				c.mutate(f)
			}
			r := Check(f, c.policy())
			if r.Verdict != c.want || !has(r, c.code) || r.Passed() {
				t.Fatalf("verdict %s (want %s), want code %s:%s", r.Verdict, c.want, c.code, dump(r))
			}
		})
	}
}

// A profile in the normal user's own home is never isolated: its ancestors are hers.
func TestProfileUnderNormalUserHomeIsNotIsolated(t *testing.T) {
	f := world()
	home := `C:\Users\Dustin`
	f.put(`C:\Users`, true, SIDSystem, true, allow(SIDSystem, full, 3), allow(SIDAdministrators, full, 3), allow(SIDUsers, rx, 3))
	f.put(home, true, normal, true, allow(normal, full, 3), allow(SIDSystem, full, 3), allow(SIDAdministrators, full, 3))
	root := home + `\.identity`
	f.put(root, true, svc, true, allow(svc, full, 3), allow(SIDSystem, full, 3))
	f.put(root+`\profile.json`, false, svc, false, allow(svc, full, FlagInherited))
	r := Check(f, AdminProfile(root, svc, normal))
	if r.Verdict != NotIsolated || !has(r, COwner) || !has(r, CReplace) {
		t.Fatalf("want NOT_ISOLATED with owner + replace findings:%s", dump(r))
	}
}

// Reparse directories are reported and never descended into.
func TestNeverDescendsThroughReparsePoint(t *testing.T) {
	f := world()
	f.get(adminRoot + `\runtime`).Reparse = true
	r := Check(f, adminPolicy())
	for _, d := range f.listed {
		if strings.EqualFold(d, adminRoot+`\runtime`) {
			t.Fatal("listed the contents of a reparse point")
		}
	}
	if r.Verdict != NotIsolated || !has(r, CReparse) {
		t.Fatalf("want reparse violation:%s", dump(r))
	}
}

// SIDs compare case-insensitively; CREATOR OWNER templates and Administrators are accepted.
func TestTrustedPrincipalsAndCase(t *testing.T) {
	f := world()
	f.proc.UserSID = strings.ToLower(svc)
	r := Check(f, AdminProfile(adminRoot, strings.ToLower(svc), normal))
	if r.Verdict != Isolated {
		t.Fatalf("lower-case SIDs:%s", dump(r))
	}
	ok := false
	for _, n := range r.NotCovered {
		ok = ok || strings.Contains(n, "UAC is not a boundary")
	}
	if !ok {
		t.Fatal("Administrators are trusted, so the report must say the normal-user/UAC condition is not covered")
	}
}

// The authority-file lists must name files that the two real paths actually use.
func TestAuthorityFileListsMatchSource(t *testing.T) {
	src := func(globs ...string) string {
		var b strings.Builder
		for _, g := range globs {
			ms, _ := filepath.Glob(g)
			for _, m := range ms {
				if strings.HasSuffix(m, "_test.go") {
					continue
				}
				x, _ := os.ReadFile(m)
				b.Write(x)
			}
		}
		return b.String()
	}
	admin := src("../*.go", "../../authority/*.go", "../../pairing/*.go", "../../app/*.go", "../../logbook/*.go", "../../entities/*.go", "../../features/*.go", "../../github/*.go", "../../../main.go")
	entity := src("../../entity/*.go", "../../../cmd/tass-identity/*.go")
	for _, n := range AdminAuthorityFiles {
		base := n[strings.LastIndex(n, "/")+1:]
		if !strings.Contains(admin, `"`+base+`"`) {
			t.Errorf("Admin path does not reference %q", base)
		}
	}
	for _, n := range EntityAuthorityFiles {
		if !strings.Contains(entity, `"`+n+`"`) {
			t.Errorf("TASS entity path does not reference %q", n)
		}
	}
}

func TestSplitWindowsPathAncestors(t *testing.T) {
	root, anc, err := SplitWindowsPath(`c:/ProgramData/Arameair/IdentityProtected/`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`C:\ProgramData\Arameair`, `C:\ProgramData`, `C:\`}
	if root != `C:\ProgramData\Arameair\IdentityProtected` || strings.Join(anc, "|") != strings.Join(want, "|") {
		t.Fatalf("got %s %v", root, anc)
	}
}
