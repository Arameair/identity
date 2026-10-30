package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Arameair/identity/internal/github"
)

// -no-open-link-file: the private troubleshooting link is never written to
// disk (and a stale copy from an older run is removed); the default is unchanged.
func TestNoOpenLinkFile(t *testing.T) {
	gh := github.Config{WebBase: "http://127.0.0.1:1", APIBase: "http://127.0.0.1:1"}
	dir := t.TempDir()
	h := startApp(t, dir, gh)
	if _, err := os.Stat(filepath.Join(dir, "runtime", "open-link.txt")); err != nil {
		t.Fatal("default behaviour changed: open-link.txt missing")
	}
	h.stop()
	h.a.Store.Close()

	dir2 := t.TempDir()
	os.MkdirAll(filepath.Join(dir2, "runtime"), 0o700)
	os.WriteFile(filepath.Join(dir2, "runtime", "open-link.txt"), []byte("stale\n"), 0o600)
	h2 := startAppWith(t, dir2, gh, func(o *Options) { o.NoOpenLinkFile = true })
	defer func() { h2.stop(); h2.a.Store.Close() }()
	if _, err := os.Stat(filepath.Join(dir2, "runtime", "open-link.txt")); err == nil {
		t.Fatal("-no-open-link-file still left open-link.txt on disk")
	}
	if !strings.Contains(h2.link, "#key=") {
		t.Fatal("link no longer announced to the caller")
	}
	filepath.WalkDir(dir2, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, _ := os.ReadFile(p); strings.Contains(string(b), h2.link[strings.Index(h2.link, "#key=")+5:]) {
				t.Errorf("private link key found on disk in %s", p)
			}
		}
		return nil
	})
}
