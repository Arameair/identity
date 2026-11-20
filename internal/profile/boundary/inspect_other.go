//go:build !windows

package boundary

// On every non-Windows platform the boundary cannot be verified: Check
// reports UNSUPPORTED. This includes Linux inside WSL, where /mnt/c access
// runs with the Windows user's token and NTFS ACLs are not visible as such.

type unsupportedInspector struct{}

// NewInspector returns the OS inspector for this platform.
func NewInspector() Inspector { return unsupportedInspector{} }

func (unsupportedInspector) Process() (Process, error)      { return Process{}, ErrUnsupported }
func (unsupportedInspector) Inspect(string) (Object, error) { return Object{}, ErrUnsupported }
func (unsupportedInspector) List(string) ([]string, error)  { return nil, ErrUnsupported }
