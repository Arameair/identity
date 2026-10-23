package main

// GitHub CLI is an optional, READ-ONLY probe of the machine's existing
// official GitHub authentication. It is NOT a per-installation OAuth grant.
// Identity never reads or stores the underlying token.
//
// This is a deliberately small intermediate step before the real account
// broker. A success here means "GitHub CLI can access /user" and nothing more.
import (
    "context"
    "errors"
    "os"
    "os/exec"
    "regexp"
    "strings"
    "time"
)

type GitHubCLIResult struct {
    State string `json:"state"`
    Account string `json:"account,omitempty"`
    Source string `json:"source"`
    Detail string `json:"detail"`
    Next string `json:"next"`
    CheckedAt time.Time `json:"checked_at"`
    ElapsedMS int64 `json:"elapsed_ms"`
    PerIdentity bool `json:"per_identity"`
    IdentityLinked bool `json:"identity_linked"`
}

var githubLoginName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// stripGitHubEnvironment prevents a hidden GH_TOKEN/GITHUB_TOKEN from making
// the check appear to be a user's interactive GitHub CLI login. A user's
// normal GitHub CLI credential store is still available to gh itself.
func stripGitHubEnvironment(env []string) []string {
    blocked := map[string]bool{
        "GH_TOKEN": true, "GITHUB_TOKEN": true,
        "GH_ENTERPRISE_TOKEN": true, "GITHUB_ENTERPRISE_TOKEN": true,
        "GH_HOST": true, "GITHUB_HOST": true,
    }
    out := make([]string, 0, len(env)+3)
    for _, value := range env {
        name, _, ok := strings.Cut(value, "=")
        if ok && blocked[strings.ToUpper(name)] { continue }
        out = append(out, value)
    }
    return append(out, "GH_HOST=github.com", "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1")
}

// checkGitHubCLI makes exactly one read-only API call via the official gh CLI.
// It has an 8-second timeout, never prints CLI stderr (which may contain
// sensitive environment details) and only exposes a validated login name.
func checkGitHubCLI() (result GitHubCLIResult) {
    started := time.Now()
    result = GitHubCLIResult{
        State: "NOT CHECKED", Source: "Official GitHub CLI (gh)",
        Detail: "Identity has no linked GitHub account. This checks only an existing GitHub CLI session on this machine.",
        Next: "The check cannot create repositories, grant permissions, or connect a profile.",
        CheckedAt: started.UTC(), PerIdentity: false, IdentityLinked: false,
    }
    defer func(){result.ElapsedMS = time.Since(started).Milliseconds()}()
    path, err := exec.LookPath("gh")
    if err != nil {
        result.State = "CLI NOT INSTALLED"
        result.Detail = "The official GitHub CLI was not found in this app's environment. No GitHub request was made."
        result.Next = "Install GitHub CLI from https://cli.github.com/ if you want to try the optional machine-session check."
        return result
    }
    ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, path, "api", "user", "--hostname", "github.com", "--jq", ".login")
    cmd.Env = stripGitHubEnvironment(os.Environ())
    output, err := cmd.Output()
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        result.State = "CHECK TIMED OUT"
        result.Detail = "The read-only GitHub session verification did not finish. No account was linked."
        result.Next = "Check connectivity and try again."
        return result
    }
    if err != nil {
        result.State = "CLI NOT AUTHENTICATED OR UNAVAILABLE"
        result.Detail = "GitHub CLI could not verify an account using its own existing credential store. Identity did not read a token."
        result.Next = "If GitHub CLI is installed, run 'gh auth login --hostname github.com --web' in your own terminal, then click Check again."
        return result
    }
    login := strings.TrimSpace(string(output))
    if !githubLoginName.MatchString(login) {
        result.State = "UNVERIFIED RESPONSE"
        result.Detail = "GitHub CLI returned unexpected account data. Identity has not linked an account."
        result.Next = "Check the official GitHub CLI installation before retrying."
        return result
    }
    result.State = "VERIFIED EXTERNAL CLI SESSION"
    result.Account = login
    result.Detail = "The GitHub API verified the displayed account through the machine's GitHub CLI session. Identity has not imported its token or bound this account to a System ID Card."
    result.Next = "A later, separately approved OAuth/account-broker step is required to link this account safely to an Identity installation."
    return result
}

func (a *App) VerifyGitHubCLI() GitHubCLIResult {
    a.mu.Lock()
    profile := a.p
    a.mu.Unlock()
    check := checkGitHubCLI() // external process; do not hold App lock
    a.mu.Lock()
    defer a.mu.Unlock()
    if a.p == profile {
        profile.GitHubCheck = &check // runtime status only; no credential persisted
        profile.Events.Add("github", "read-only official GitHub CLI check: "+check.State+" (no token imported or account linked)", "")
    }
    return check
}
