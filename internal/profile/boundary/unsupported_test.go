//go:build !windows

package boundary

import "testing"

// Linux, macOS and Linux-inside-WSL cannot verify the boundary: UNSUPPORTED, never a pass.
func TestNonWindowsIsUnsupportedFailClosed(t *testing.T) {
	r := Check(NewInspector(), AdminProfile(adminRoot, svc, normal))
	if r.Verdict != Unsupported || r.Passed() || !has(r, CUnsupported) {
		t.Fatalf("verdict %s:%s", r.Verdict, dump(r))
	}
	r = Check(NewInspector(), EntityProfile(entityRoot, svc, normal))
	if r.Verdict != Unsupported || r.Passed() {
		t.Fatalf("entity verdict %s", r.Verdict)
	}
}
