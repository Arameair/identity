package main

import (
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
)

func TestGitHubCLIEnvironmentDoesNotInheritTokens(t *testing.T) {
    e := stripGitHubEnvironment([]string{
        "PATH=/bin", "GH_TOKEN=secret-one", "GITHUB_TOKEN=secret-two",
        "GH_ENTERPRISE_TOKEN=secret-three", "GH_HOST=other.example",
        "GH_CONFIG_DIR=/tmp/example-gh-config",
    })
    joined := strings.Join(e, "\n")
    for _, secret := range []string{"secret-one", "secret-two", "secret-three", "other.example"} {
        if strings.Contains(joined, secret) { t.Fatalf("unexpected credential or host inherited") }
    }
    for _, expected := range []string{"GH_HOST=github.com", "GH_PROMPT_DISABLED=1", "GH_CONFIG_DIR=/tmp/example-gh-config"} {
        if !strings.Contains(joined, expected) { t.Errorf("expected safe env entry %s", expected) }
    }
}

func TestGitHubCLINotInstalledShowsNoAccount(t *testing.T) {
    t.Setenv("PATH", t.TempDir())
    result := checkGitHubCLI()
    if result.State != "CLI NOT INSTALLED" { t.Fatalf("state = %s", result.State) }
    if result.Account != "" || result.IdentityLinked || result.PerIdentity {
        t.Fatal("missing CLI must never mark an account as linked")
    }
}

func TestGitHubCLIMockBinaryProvesStatusIsNotIdentityAuthorization(t *testing.T) {
    if runtime.GOOS == "windows" { t.Skip("test-only shell shim; Windows compile covered separately") }
    temp := t.TempDir()
    script := "#!/bin/sh\nif [ -n \"$GH_TOKEN\" ] || [ -n \"$GITHUB_TOKEN\" ]; then exit 7; fi\nif [ \"$1\" = api ] && [ \"$2\" = user ]; then echo test-user; else exit 3; fi\n"
    if err := os.WriteFile(filepath.Join(temp, "gh"), []byte(script), 0700); err != nil { t.Fatal(err) }
    t.Setenv("PATH", temp)
    t.Setenv("GH_TOKEN", "must-not-pass-to-child")
    r := checkGitHubCLI()
    if r.State != "VERIFIED EXTERNAL CLI SESSION" || r.Account != "test-user" { t.Fatalf("fake CLI result: %+v", r) }
    if r.IdentityLinked || r.PerIdentity { t.Fatal("external CLI session is not an Identity grant") }
    if r.ElapsedMS < 0 || r.CheckedAt.IsZero() { t.Fatal("missing diagnostics") }
}

func TestGitHubLoginNameValidation(t *testing.T) {
    for _, input := range []string{"octocat", "user-123"} {
        if !githubLoginName.MatchString(input) { t.Fatalf("rejected valid test name %s", input) }
    }
    for _, input := range []string{"", "a b", "a\nsecret", "../login", "https://example.com"} {
        if githubLoginName.MatchString(input) { t.Fatalf("accepted unsafe account label %q", input) }
    }
}
