package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// GitHub sign-in uses GitHub's official device flow: GitHub shows the person
// a short code page, they approve in their own browser, and Identity receives
// an access token. The token stays in this process's memory only.

type GitHubConfig struct {
	ClientID string // public identifier of the owner's registered GitHub app
	Scope    string // "" for a GitHub App; "repo" if an OAuth App is used
	WebBase  string // https://github.com
	APIBase  string // https://api.github.com
}

func (c GitHubConfig) IsReal() bool {
	return c.WebBase == "https://github.com" && c.APIBase == "https://api.github.com"
}

type GitHub struct {
	cfg  GitHubConfig
	http *http.Client

	mu          sync.Mutex
	status      string // not_configured | signed_out | waiting | connected | denied | expired | error
	userCode    string
	verifyURL   string
	expires     time.Time
	token       string
	login       string
	userID      int64
	message     string
	cancel      context.CancelFunc
	connectedAt time.Time
}

func NewGitHub(cfg GitHubConfig) *GitHub {
	g := &GitHub{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}, status: "signed_out"}
	if cfg.ClientID == "" {
		g.status = "not_configured"
	}
	return g
}

type GitHubView struct {
	Status      string    `json:"status"`
	UserCode    string    `json:"user_code,omitempty"`
	VerifyURL   string    `json:"verify_url,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Login       string    `json:"login,omitempty"`
	ConnectedAt time.Time `json:"connected_at,omitempty"`
	Message     string    `json:"message,omitempty"`
	TestMode    bool      `json:"test_mode"`
}

func (g *GitHub) View() GitHubView {
	g.mu.Lock()
	defer g.mu.Unlock()
	v := GitHubView{Status: g.status, Login: g.login, Message: g.message, TestMode: !g.cfg.IsReal()}
	if g.status == "waiting" {
		v.UserCode, v.VerifyURL, v.ExpiresAt = g.userCode, g.verifyURL, g.expires
	}
	if g.status == "connected" {
		v.ConnectedAt = g.connectedAt
	}
	return v
}

func (g *GitHub) Token() (string, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.token, g.login
}

func (g *GitHub) postForm(ctx context.Context, path string, form url.Values, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.WebBase+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub answered %d", resp.StatusCode)
	}
	return json.Unmarshal(b, out)
}

// Start begins sign-in. Calling it while already waiting returns the same code.
func (g *GitHub) Start() error {
	g.mu.Lock()
	if g.cfg.ClientID == "" {
		g.mu.Unlock()
		return errors.New("not_configured")
	}
	if g.status == "waiting" && time.Now().Before(g.expires) {
		g.mu.Unlock()
		return nil
	}
	g.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var dc struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
		ErrorDesc       string `json:"error_description"`
	}
	form := url.Values{"client_id": {g.cfg.ClientID}}
	if g.cfg.Scope != "" {
		form.Set("scope", g.cfg.Scope)
	}
	if err := g.postForm(ctx, "/login/device/code", form, &dc); err != nil {
		g.fail("error", err.Error())
		return err
	}
	if dc.Error != "" || dc.DeviceCode == "" {
		msg := "GitHub refused to start sign-in"
		if dc.Error == "device_flow_disabled" {
			msg = "GitHub sign-in isn't switched on for this app yet (Device Flow is disabled in the app's settings)."
		} else if dc.Error == "unauthorized_client" || dc.Error == "incorrect_client_credentials" {
			msg = "GitHub doesn't recognise this app's Client ID."
		}
		g.fail("error", msg)
		return errors.New(msg)
	}
	// Only ever send the person to GitHub's own page.
	if !strings.HasPrefix(dc.VerificationURI, g.cfg.WebBase+"/") {
		g.fail("error", "GitHub returned an unexpected sign-in address; refusing to open it")
		return errors.New("bad verification uri")
	}
	interval := time.Duration(max(dc.Interval, 5)) * time.Second
	pctx, pcancel := context.WithTimeout(context.Background(), time.Duration(dc.ExpiresIn)*time.Second)
	g.mu.Lock()
	if g.cancel != nil {
		g.cancel()
	}
	g.status, g.userCode, g.verifyURL, g.message = "waiting", dc.UserCode, dc.VerificationURI, ""
	g.expires = time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	g.cancel = pcancel
	g.mu.Unlock()
	go g.poll(pctx, dc.DeviceCode, interval)
	return nil
}

func (g *GitHub) poll(ctx context.Context, deviceCode string, interval time.Duration) {
	t := time.NewTimer(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			g.mu.Lock()
			if g.status == "waiting" {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					g.status, g.message = "expired", "The code expired before it was approved."
				} else {
					g.status, g.message = "signed_out", ""
				}
			}
			g.mu.Unlock()
			return
		case <-t.C:
		}
		var tr struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			Interval    int    `json:"interval"`
		}
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := g.postForm(rctx, "/login/oauth/access_token", url.Values{"client_id": {g.cfg.ClientID},
			"device_code": {deviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &tr)
		cancel()
		switch {
		case err != nil:
			// network blip: keep waiting, GitHub's code is still valid
		case tr.AccessToken != "":
			g.finish(tr.AccessToken)
			return
		case tr.Error == "authorization_pending":
		case tr.Error == "slow_down":
			interval += 5 * time.Second
		case tr.Error == "access_denied":
			g.fail("denied", "You chose Cancel on GitHub, so nothing was connected.")
			return
		case tr.Error == "expired_token":
			g.fail("expired", "The code expired before it was approved.")
			return
		default:
			g.fail("error", "GitHub sign-in failed ("+tr.Error+").")
			return
		}
		t.Reset(interval)
	}
}

func (g *GitHub) finish(token string) {
	// Verify the account with the token we just received.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var u struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	}
	st, err := g.api(ctx, token, "/user", &u)
	if err != nil || st != 200 || u.Login == "" {
		g.fail("error", "GitHub approved sign-in, but the account couldn't be read back.")
		return
	}
	g.mu.Lock()
	g.status, g.token, g.login, g.userID, g.message = "connected", token, u.Login, u.ID, ""
	g.userCode, g.verifyURL, g.connectedAt = "", "", time.Now().UTC()
	g.mu.Unlock()
}

func (g *GitHub) fail(status, msg string) {
	g.mu.Lock()
	g.status, g.message, g.userCode, g.verifyURL = status, msg, "", ""
	g.mu.Unlock()
}

func (g *GitHub) Cancel() {
	g.mu.Lock()
	c := g.cancel
	g.cancel = nil
	if g.status == "waiting" {
		g.status, g.userCode, g.verifyURL = "signed_out", "", ""
	}
	g.mu.Unlock()
	if c != nil {
		c()
	}
}

func (g *GitHub) Disconnect() {
	g.Cancel()
	g.mu.Lock()
	g.token, g.login, g.userID, g.message = "", "", 0, ""
	if g.cfg.ClientID == "" {
		g.status = "not_configured"
	} else {
		g.status = "signed_out"
	}
	g.mu.Unlock()
}

func (g *GitHub) api(ctx context.Context, token, path string, out any) (int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, g.cfg.APIBase+path, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode == 200 && out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

// FindResult is step 3's evidence, all read from GitHub with the person's token.
type FindResult struct {
	Status     string    `json:"status"` // found | no_access | no_branch | error
	Repo       string    `json:"repo"`
	Private    bool      `json:"private"`
	Branch     string    `json:"branch"`
	Commit     string    `json:"commit,omitempty"`
	CommitDate time.Time `json:"commit_date,omitempty"`
	Message    string    `json:"message,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
	AsLogin    string    `json:"as_login"`
}

func (g *GitHub) Find(ctx context.Context, repo, branch string) FindResult {
	token, login := g.Token()
	r := FindResult{Repo: repo, Branch: branch, CheckedAt: time.Now().UTC(), AsLogin: login}
	if token == "" {
		r.Status, r.Detail = "error", "Connect GitHub first."
		return r
	}
	var rp struct {
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
	}
	st, err := g.api(ctx, token, "/repos/"+repo, &rp)
	switch {
	case err != nil:
		r.Status, r.Detail = "error", "Couldn't reach GitHub. Check the internet connection and try again."
		return r
	case st == 404 || st == 403:
		r.Status, r.Detail = "no_access", fmt.Sprintf("GitHub says the account %s can't see %s. Ask the owner to give this account access, then try again.", login, repo)
		return r
	case st != 200:
		r.Status, r.Detail = "error", fmt.Sprintf("GitHub answered %d.", st)
		return r
	}
	r.Repo, r.Private = rp.FullName, rp.Private
	var br struct {
		Commit struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message   string `json:"message"`
				Committer struct {
					Date time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
		} `json:"commit"`
	}
	st, err = g.api(ctx, token, "/repos/"+repo+"/branches/"+url.PathEscape(branch), &br)
	switch {
	case err != nil:
		r.Status, r.Detail = "error", "Couldn't reach GitHub. Check the internet connection and try again."
	case st == 404:
		r.Status, r.Detail = "no_branch", "The repository is there, but the version "+branch+" was not found. It may have been renamed or removed."
	case st != 200:
		r.Status, r.Detail = "error", fmt.Sprintf("GitHub answered %d.", st)
	default:
		r.Status, r.Commit, r.CommitDate = "found", br.Commit.SHA, br.Commit.Commit.Committer.Date
		r.Message, _, _ = strings.Cut(br.Commit.Commit.Message, "\n")
		if len(r.Message) > 120 {
			r.Message = r.Message[:120] + "…"
		}
	}
	return r
}
