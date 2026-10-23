package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeGit(top, sha, head string) func(...string) (string, error) {
	return func(a ...string) (string, error) {
		switch strings.Join(a, " ") {
		case "rev-parse --show-toplevel":
			if top == "" {
				return "", errors.New("not a repo")
			}
			return top, nil
		case "rev-parse HEAD":
			return sha, nil
		case "rev-parse --abbrev-ref HEAD":
			return head, nil
		}
		return "", errors.New("?")
	}
}

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestConfigProjectControllerDetached(t *testing.T) {
	pc := t.TempDir()
	c, err := resolveConfig("", envOf(map[string]string{"PC_BRANCH": "experiment/identity-local-v1-20261023", "PC_DATA": pc, "PORT": "7811"}),
		fakeGit("/work/checkout", "abc123def4567890", "HEAD"), "/home/u")
	if err != nil {
		t.Fatal(err)
	}
	if c.Branch != "experiment/identity-local-v1-20261023" || c.BranchFrom != "PC_BRANCH" || !c.Detached || c.Commit != "abc123def4567890" ||
		c.DataDir != filepath.Join(pc, "identity") || c.DataSource != "PC_DATA" || c.Port != "7811" {
		t.Fatalf("%+v", c)
	}
}

func TestConfigOverridesAndDefaults(t *testing.T) {
	g := fakeGit("/work/checkout", "abc123def4567890", "my-branch")
	flagDir, envDir, pc := t.TempDir(), t.TempDir(), t.TempDir()
	all := map[string]string{"IDENTITY_DATA": envDir, "PC_DATA": pc}
	if c, _ := resolveConfig(flagDir, envOf(all), g, "/h"); c.DataDir != flagDir || c.DataSource != "-data flag" {
		t.Fatalf("flag wins: %+v", c)
	}
	if c, _ := resolveConfig("", envOf(all), g, "/h"); c.DataDir != envDir || c.DataSource != "IDENTITY_DATA" {
		t.Fatalf("env next: %+v", c)
	}
	home := t.TempDir()
	if c, _ := resolveConfig("", envOf(nil), g, home); c.DataDir != filepath.Join(home, ".identity-test", "my-branch") || c.Branch != "my-branch" || c.BranchFrom != "git" || c.Port != "7796" {
		t.Fatalf("standalone: %+v", c)
	}
	// detached with no PC_BRANCH: folder per commit, never a shared "default"
	c, _ := resolveConfig("", envOf(nil), fakeGit("/work/checkout", "abc123def4567890", "HEAD"), home)
	if c.Branch != "unknown" || c.DataDir != filepath.Join(home, ".identity-test", "commit-abc123def456") {
		t.Fatalf("detached standalone: %+v", c)
	}
	// branch names with slashes stay one folder
	c, _ = resolveConfig("", envOf(map[string]string{"PC_BRANCH": "experiment/a/../b"}), g, home)
	if filepath.Dir(c.DataDir) != filepath.Join(home, ".identity-test") {
		t.Fatalf("branch name escaped the test folder: %s", c.DataDir)
	}
}

func TestConfigRefusesDataInsideCheckout(t *testing.T) {
	top := t.TempDir()
	_, err := resolveConfig(filepath.Join(top, "data"), envOf(nil), fakeGit(top, "x", "b"), "/h")
	if err == nil || !strings.Contains(err.Error(), "inside the code checkout") {
		t.Fatalf("got %v", err)
	}
	if _, err := resolveConfig(top+"-sibling", envOf(nil), fakeGit(top, "x", "b"), "/h"); err != nil {
		t.Fatalf("sibling folder is fine: %v", err)
	}
}

func TestLockOneAppPerDataFolder(t *testing.T) {
	dir := t.TempDir()
	l1, err := acquireLock(dir, "1")
	if err != nil {
		t.Fatal(err)
	}
	// simulate a different live process holding it
	os.WriteFile(filepath.Join(dir, "identity.lock"), []byte("1 9999\n"), 0o600) // pid 1 is always alive
	if _, err := acquireLock(dir, "2"); err == nil || !strings.Contains(err.Error(), "already using data folder") {
		t.Fatalf("second app on same folder: %v", err)
	}
	// a stale lock (dead pid) is taken over
	os.WriteFile(filepath.Join(dir, "identity.lock"), []byte("999999 1\n"), 0o600)
	l2, err := acquireLock(dir, "2")
	if err != nil {
		t.Fatalf("stale lock: %v", err)
	}
	l2.release()
	l1.release()
	if _, err := os.Stat(filepath.Join(dir, "identity.lock")); !os.IsNotExist(err) {
		t.Fatal("release must remove the lock")
	}
	other, err := acquireLock(t.TempDir(), "3")
	if err != nil {
		t.Fatal("a different folder is independent")
	}
	other.release()
}
