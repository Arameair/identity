package boundary

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	driveRE    = regexp.MustCompile(`^[A-Za-z]:\\`)
	reservedRE = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[0-9¹²³]|lpt[0-9¹²³]|conin\$|conout\$)(\..*)?$`)
)

// SplitWindowsPath validates a protected-root path and returns it in canonical
// form plus its ancestors, nearest first, ending with the volume root.
//
// Only plain drive-absolute paths on a local drive are accepted. Rejected, fail
// closed: relative paths; UNC and device paths (\\server\…, \\?\…, \\.\…);
// WSL paths (\\wsl$\…, \\wsl.localhost\…, /mnt/c/…), because a profile inside a
// WSL distro lives in a virtual disk that the normal user owns; alternate data
// streams; reserved device names; "." / ".." components; components ending in
// a dot or space (Windows silently strips them, so two spellings name one file).
func SplitWindowsPath(p string) (string, []string, error) {
	if p == "" {
		return "", nil, errors.New("empty path")
	}
	lp := strings.ToLower(strings.ReplaceAll(p, "/", `\`))
	switch {
	case strings.HasPrefix(p, "/"):
		return "", nil, fmt.Errorf("%q is a POSIX/WSL path; the protected root must be a Windows drive path", p)
	case strings.HasPrefix(lp, `\\wsl$`), strings.HasPrefix(lp, `\\wsl.localhost`):
		return "", nil, fmt.Errorf("%q is inside a WSL distribution (its virtual disk belongs to the normal user)", p)
	case strings.HasPrefix(lp, `\\`):
		return "", nil, fmt.Errorf("%q is a UNC or device path; only local drive paths are accepted", p)
	case !driveRE.MatchString(strings.ReplaceAll(p, "/", `\`)):
		return "", nil, fmt.Errorf("%q is not an absolute drive path (X:\\…)", p)
	}
	q := strings.ReplaceAll(p, "/", `\`)
	drive := strings.ToUpper(q[:1]) + `:\`
	rest := strings.TrimRight(q[3:], `\`)
	if rest == "" {
		return "", nil, errors.New("the protected root cannot be a volume root")
	}
	parts := strings.Split(rest, `\`)
	for _, c := range parts {
		switch {
		case c == "":
			return "", nil, fmt.Errorf("%q contains an empty component", p)
		case c == "." || c == "..":
			return "", nil, fmt.Errorf("%q contains a relative component", p)
		case strings.ContainsAny(c, `:*?"<>|`):
			return "", nil, fmt.Errorf("component %q contains a stream separator or wildcard", c)
		case strings.HasSuffix(c, ".") || strings.HasSuffix(c, " ") || strings.HasPrefix(c, " "):
			return "", nil, fmt.Errorf("component %q has a leading space or trailing dot/space", c)
		case reservedRE.MatchString(c):
			return "", nil, fmt.Errorf("component %q is a reserved device name", c)
		}
		for _, r := range c {
			if r < 0x20 {
				return "", nil, fmt.Errorf("component %q contains a control character", c)
			}
		}
	}
	root := drive + strings.Join(parts, `\`)
	var anc []string
	for i := len(parts) - 1; i >= 1; i-- {
		anc = append(anc, drive+strings.Join(parts[:i], `\`))
	}
	anc = append(anc, drive)
	return root, anc, nil
}

// samePath compares an OS-reported final path with the requested path.
func samePath(final, want string) bool {
	f := final
	if strings.HasPrefix(f, `\\?\`) && !strings.HasPrefix(strings.ToUpper(f), `\\?\UNC\`) {
		f = f[4:]
	}
	f = strings.TrimRight(f, `\`)
	return strings.EqualFold(f, strings.TrimRight(want, `\`))
}
