// Package github connects a person's GitHub account through the publisher's
// registered GitHub App using GitHub's official device flow, then reads back
// — with that person's own token — who they are and which publisher app
// installations and feature repositories GitHub says they can see.
//
// Adapted from identity experiment/identity-standalone-access-trial-20261024
// (standalone/github.go). Changes: verification of installations and feature
// repositories, event callbacks for the activity log, redaction registration,
// explicit TEST FIXTURE labelling.
//
// The access token lives in this process's memory only. Nothing here is a
// feature entitlement: repository visibility is reported as visibility.
package github

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

	"github.com/Arameair/identity/internal/redact"
)

const (
	RealWeb = "https://github.com"
	RealAPI = "https://api.github.com"
)

type Config struct {
	ClientID       string // public Client ID of the publisher's GitHub App (not a secret)
	ClientIDSource string // where it came from, for diagnostics
	WebBase        string
	APIBase        string
}

func (c Config) IsReal() bool { return c.WebBase == RealWeb && c.APIBase == RealAPI }

// Mode labels every view so a fixture can never be mistaken for real GitHub.
func (c Config) Mode() string {
	if c.IsReal() {
		return "real"
	}
	return "TEST FIXTURE"
}

// Statuses.
const (
	NotConfigured = "not_configured"
	SignedOut     = "signed_out"
	Waiting       = "waiting"
	Verified      = "verified"
	Denied        = "denied"
	Expired       = "expired"
	Failed        = "error"
)

type Installation struct {
	ID                  int64  `json:"id"`
	Account             string `json:"account"`
	AppSlug             string `json:"app_slug"`
	AppID               int64  `json:"app_id"`
	RepositorySelection string `json:"repository_selection"`
}

type RepoCheck struct {
	Repo      string    `json:"repo"`
	Result    string    `json:"result"` // VISIBLE | NOT VISIBLE | CHECK FAILED
	Private   bool      `json:"private,omitempty"`
	HTTP      int       `json:"http,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type Account struct {
	Login      string    `json:"login"`
	ID         int64     `json:"id"`
	Name       string    `json:"name,omitempty"`
	Type       string    `json:"type,omitempty"`
	ProfileURL string    `json:"profile_url,omitempty"`
	VerifiedAt time.Time `json:"verified_at"`
}

// Publisher is what GitHub itself reports about the publisher app as seen by
// this account. InstallationsResult explains when it could not be read.
type Publisher struct {
	InstallationsResult string         `json:"installations_result"` // READ | NOT AVAILABLE | CHECK FAILED
	InstallationsDetail string         `json:"installations_detail,omitempty"`
	Installations       []Installation `json:"installations"`
	Repos               []RepoCheck    `json:"repos"`
	CheckedAt           time.Time      `json:"checked_at"`
}

type View struct {
	Status         string     `json:"status"`
	Mode           string     `json:"mode"`
	ClientIDSet    bool       `json:"client_id_set"`
	ClientIDSource string     `json:"client_id_source,omitempty"`
	ClientIDHint   string     `json:"client_id_hint,omitempty"`
	UserCode       string     `json:"user_code,omitempty"` // shown to the person only while waiting; never logged
	VerifyURL      string     `json:"verify_url,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Account        *Account   `json:"account,omitempty"`
	Publisher      *Publisher `json:"publisher,omitempty"`
	TokenExpiresAt *time.Time `json:"token_expires_at,omitempty"`
	Message        string     `json:"message,omitempty"`
}

// EventFunc receives sign-in state changes. Details never contain secrets.
type EventFunc func(level, event, result, detail string)

type Client struct {
	http    *http.Client
	onEvent EventFunc
	repos   []string

	mu          sync.Mutex
	cfg         Config
	status      string
	userCode    string
	deviceCode  string
	verifyURL   string
	expires     time.Time
	token       string
	tokenExpiry time.Time
	account     *Account
	publisher   *Publisher
	message     string
	cancel      context.CancelFunc
}

func New(cfg Config, featureRepos []string, onEvent EventFunc) *Client {
	if onEvent == nil {
		onEvent = func(string, string, string, string) {}
	}
	c := &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}, onEvent: onEvent, repos: featureRepos, status: SignedOut}
	if cfg.ClientID == "" {
		c.status = NotConfigured
	}
	return c
}

func hint(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:4] + "…" + id[len(id)-4:]
}

func (c *Client) View() View {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireTokenLocked()
	v := View{Status: c.status, Mode: c.cfg.Mode(), ClientIDSet: c.cfg.ClientID != "", ClientIDSource: c.cfg.ClientIDSource,
		ClientIDHint: hint(c.cfg.ClientID), Message: c.message}
	if c.status == Waiting {
		e := c.expires
		v.UserCode, v.VerifyURL, v.ExpiresAt = c.userCode, c.verifyURL, &e
	}
	if c.status == Verified {
		if c.account != nil {
			a := *c.account
			v.Account = &a
		}
		if c.publisher != nil {
			p := *c.publisher
			v.Publisher = &p
		}
		if !c.tokenExpiry.IsZero() {
			t := c.tokenExpiry
			v.TokenExpiresAt = &t
		}
	}
	return v
}

func (c *Client) expireTokenLocked() {
	if c.status == Verified && !c.tokenExpiry.IsZero() && time.Now().After(c.tokenExpiry) {
		redact.Forget(c.token)
		c.status, c.token, c.message = Expired, "", "GitHub's sign-in for this session expired. Connect again."
		go c.onEvent("warn", "github.session_expired", "expired", "token lifetime reached; signed out of this session")
	}
}

// SetClientID configures the public Client ID if none is set yet.
func (c *Client) SetClientID(id, source string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.ClientID != "" {
		return errors.New("a GitHub App Client ID is already configured; nothing changed")
	}
	c.cfg.ClientID, c.cfg.ClientIDSource, c.status, c.message = id, source, SignedOut, ""
	return nil
}

// ValidClientID checks the public Client ID format of GitHub Apps (Iv…).
func ValidClientID(id string) bool {
	return strings.HasPrefix(id, "Iv") && len(id) >= 10 && len(id) <= 64 && !strings.ContainsAny(id, " \t\r\n/\\\"'")
}

// postForm sends GitHub's OAuth parameters form-encoded, as GitHub documents
// for the device flow.
func (c *Client) postForm(ctx context.Context, path string, form url.Values, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.WebBase+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Arameair-Identity-PrototypeB")
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("couldn't reach GitHub (network error)")
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode != 200 {
		// GitHub sometimes reports OAuth errors with a non-200 status and a JSON error body.
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &e) == nil && e.Error != "" {
			return json.Unmarshal(b, out)
		}
		return fmt.Errorf("GitHub answered HTTP %d", resp.StatusCode)
	}
	return json.Unmarshal(b, out)
}

// Start begins device-flow sign-in. While already waiting it is a no-op.
func (c *Client) Start() error {
	c.mu.Lock()
	if c.cfg.ClientID == "" {
		c.mu.Unlock()
		return errors.New("no GitHub App Client ID is configured yet")
	}
	if c.status == Waiting && time.Now().Before(c.expires) {
		c.mu.Unlock()
		return nil
	}
	cfg := c.cfg
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var dc struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
	}
	c.onEvent("info", "github.signin_start", "requested", "asking "+cfg.Mode()+" GitHub for a device sign-in code")
	if err := c.postForm(ctx, "/login/device/code", url.Values{"client_id": {cfg.ClientID}}, &dc); err != nil {
		c.fail(Failed, "GitHub sign-in could not start: "+err.Error())
		return err
	}
	if dc.Error != "" || dc.DeviceCode == "" {
		msg := "GitHub refused to start sign-in (" + dc.Error + ")."
		switch dc.Error {
		case "device_flow_disabled":
			msg = "GitHub sign-in isn't switched on for this app (Device Flow is disabled in the GitHub App settings)."
		case "unauthorized_client", "incorrect_client_credentials", "Not Found":
			msg = "GitHub doesn't recognise this app's Client ID."
		}
		c.fail(Failed, msg)
		return errors.New(msg)
	}
	if !strings.HasPrefix(dc.VerificationURI, cfg.WebBase+"/") {
		c.fail(Failed, "GitHub returned an unexpected sign-in address; refusing to use it.")
		return errors.New("unexpected verification address")
	}
	if dc.ExpiresIn <= 0 {
		dc.ExpiresIn = 900
	}
	redact.Register("github_device_code", dc.DeviceCode)
	redact.Register("github_user_code", dc.UserCode)
	interval := time.Duration(max(dc.Interval, 5)) * time.Second
	pctx, pcancel := context.WithTimeout(context.Background(), time.Duration(dc.ExpiresIn)*time.Second)
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.status, c.userCode, c.deviceCode, c.verifyURL, c.message = Waiting, dc.UserCode, dc.DeviceCode, dc.VerificationURI, ""
	c.expires = time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	c.cancel = pcancel
	c.mu.Unlock()
	c.onEvent("info", "github.signin_waiting", "waiting", fmt.Sprintf("code issued (not logged); waiting up to %ds for approval on %s", dc.ExpiresIn, dc.VerificationURI))
	go c.poll(pctx, dc.DeviceCode, interval)
	return nil
}

// pollInterval lets tests shorten waiting.
var pollInterval = func(d time.Duration) time.Duration { return d }

func (c *Client) poll(ctx context.Context, deviceCode string, interval time.Duration) {
	t := time.NewTimer(pollInterval(interval))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			still := c.status == Waiting && c.deviceCode == deviceCode
			c.mu.Unlock()
			if still {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					c.fail(Expired, "The code expired before it was approved on GitHub.")
				}
			}
			return
		case <-t.C:
		}
		var tr struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
			Error       string `json:"error"`
		}
		c.mu.Lock()
		cid := c.cfg.ClientID
		c.mu.Unlock()
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := c.postForm(rctx, "/login/oauth/access_token", url.Values{"client_id": {cid},
			"device_code": {deviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &tr)
		cancel()
		switch {
		case err != nil:
			if ctx.Err() != nil {
				continue
			}
			// transient network problem: the code is still valid, keep waiting
		case tr.AccessToken != "":
			c.finish(tr.AccessToken, tr.ExpiresIn)
			return
		case tr.Error == "authorization_pending":
		case tr.Error == "slow_down":
			interval += 5 * time.Second
		case tr.Error == "access_denied":
			c.fail(Denied, "Sign-in was cancelled on GitHub, so nothing was connected.")
			return
		case tr.Error == "expired_token":
			c.fail(Expired, "The code expired before it was approved on GitHub.")
			return
		default:
			c.fail(Failed, "GitHub sign-in failed ("+tr.Error+").")
			return
		}
		t.Reset(pollInterval(interval))
	}
}

func (c *Client) finish(token string, expiresIn int) {
	redact.Register("github_access_token", token)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var u struct {
		Login   string `json:"login"`
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Type    string `json:"type"`
		HTMLURL string `json:"html_url"`
	}
	st, err := c.api(ctx, token, "/user", &u)
	if err != nil || st != 200 || u.Login == "" || u.ID == 0 {
		redact.Forget(token)
		c.fail(Failed, fmt.Sprintf("GitHub approved sign-in, but the account could not be read back (HTTP %d).", st))
		return
	}
	acct := &Account{Login: u.Login, ID: u.ID, Name: u.Name, Type: u.Type, ProfileURL: u.HTMLURL, VerifiedAt: time.Now().UTC()}
	pub := c.readPublisher(ctx, token)
	c.mu.Lock()
	c.status, c.token, c.account, c.publisher, c.message = Verified, token, acct, pub, ""
	c.userCode, c.deviceCode, c.verifyURL = "", "", ""
	c.tokenExpiry = time.Time{}
	if expiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	c.mu.Unlock()
	c.onEvent("info", "github.verified", "verified", fmt.Sprintf("GitHub (%s) confirmed account %s (id %d); token kept in memory only", c.cfg.Mode(), acct.Login, acct.ID))
	c.onEvent("info", "github.publisher_read", pub.InstallationsResult, publisherSummary(pub))
}

func publisherSummary(p *Publisher) string {
	parts := []string{fmt.Sprintf("installations %s (%d)", p.InstallationsResult, len(p.Installations))}
	for _, r := range p.Repos {
		parts = append(parts, r.Repo+" "+r.Result)
	}
	return strings.Join(parts, "; ")
}

func (c *Client) readPublisher(ctx context.Context, token string) *Publisher {
	p := &Publisher{CheckedAt: time.Now().UTC(), Installations: []Installation{}, Repos: []RepoCheck{}}
	var inst struct {
		Installations []struct {
			ID                  int64  `json:"id"`
			AppID               int64  `json:"app_id"`
			AppSlug             string `json:"app_slug"`
			RepositorySelection string `json:"repository_selection"`
			Account             struct {
				Login string `json:"login"`
			} `json:"account"`
		} `json:"installations"`
	}
	st, err := c.api(ctx, token, "/user/installations?per_page=50", &inst)
	switch {
	case err != nil:
		p.InstallationsResult, p.InstallationsDetail = "CHECK FAILED", "could not reach GitHub"
	case st == 200:
		p.InstallationsResult = "READ"
		for _, i := range inst.Installations {
			p.Installations = append(p.Installations, Installation{ID: i.ID, AppID: i.AppID, AppSlug: i.AppSlug, Account: i.Account.Login, RepositorySelection: i.RepositorySelection})
		}
		if len(p.Installations) == 0 {
			p.InstallationsDetail = "GitHub reports no installation of the publisher app that this account can access"
		}
	default:
		p.InstallationsResult = "NOT AVAILABLE"
		p.InstallationsDetail = fmt.Sprintf("GitHub answered HTTP %d (expected for an OAuth App instead of a GitHub App)", st)
	}
	for _, repo := range c.repos {
		var rp struct {
			FullName string `json:"full_name"`
			Private  bool   `json:"private"`
		}
		rc := RepoCheck{Repo: repo, CheckedAt: time.Now().UTC()}
		st, err := c.api(ctx, token, "/repos/"+repo, &rp)
		rc.HTTP = st
		switch {
		case err != nil:
			rc.Result = "CHECK FAILED"
		case st == 200:
			rc.Result, rc.Private = "VISIBLE", rp.Private
		case st == 404 || st == 403:
			rc.Result = "NOT VISIBLE"
		default:
			rc.Result = "CHECK FAILED"
		}
		p.Repos = append(p.Repos, rc)
	}
	return p
}

// Recheck re-reads account and publisher information with the current token.
func (c *Client) Recheck() error {
	c.mu.Lock()
	token, status := c.token, c.status
	c.mu.Unlock()
	if status != Verified || token == "" {
		return errors.New("not signed in to GitHub in this session")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var u struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	}
	st, err := c.api(ctx, token, "/user", &u)
	if err != nil {
		return errors.New("couldn't reach GitHub")
	}
	if st == 401 {
		redact.Forget(token)
		c.fail(Expired, "GitHub no longer accepts this session's sign-in (revoked or expired). Connect again.")
		return errors.New("token no longer valid")
	}
	if st != 200 {
		return fmt.Errorf("GitHub answered HTTP %d", st)
	}
	pub := c.readPublisher(ctx, token)
	c.mu.Lock()
	if c.account != nil {
		c.account.VerifiedAt = time.Now().UTC()
	}
	c.publisher = pub
	c.mu.Unlock()
	c.onEvent("info", "github.publisher_read", pub.InstallationsResult, publisherSummary(pub))
	return nil
}

func (c *Client) fail(status, msg string) {
	c.mu.Lock()
	redact.Forget(c.userCode)
	redact.Forget(c.deviceCode)
	c.status, c.message, c.userCode, c.deviceCode, c.verifyURL = status, msg, "", "", ""
	c.mu.Unlock()
	level := "warn"
	if status == Failed {
		level = "error"
	}
	c.onEvent(level, "github.signin_"+status, status, msg)
}

func (c *Client) Cancel() {
	c.mu.Lock()
	cn := c.cancel
	c.cancel = nil
	was := c.status
	if c.status == Waiting {
		c.status, c.userCode, c.deviceCode, c.verifyURL = SignedOut, "", "", ""
	}
	c.mu.Unlock()
	if cn != nil {
		cn()
	}
	if was == Waiting {
		c.onEvent("info", "github.signin_cancelled", "cancelled", "sign-in cancelled in Identity")
	}
}

func (c *Client) Disconnect() {
	c.Cancel()
	c.mu.Lock()
	had := c.status == Verified
	redact.Forget(c.token)
	c.token, c.account, c.publisher, c.message, c.tokenExpiry = "", nil, nil, "", time.Time{}
	if c.cfg.ClientID == "" {
		c.status = NotConfigured
	} else {
		c.status = SignedOut
	}
	c.mu.Unlock()
	if had {
		c.onEvent("info", "github.disconnected", "signed_out", "session token discarded from memory")
	}
}

func (c *Client) api(ctx context.Context, token, path string, out any) (int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.APIBase+path, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Arameair-Identity-PrototypeB")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
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

// RepoVisible reports the last checked visibility of repo for the verified account.
func (v View) RepoVisible(repo string) (RepoCheck, bool) {
	if v.Publisher == nil {
		return RepoCheck{}, false
	}
	for _, r := range v.Publisher.Repos {
		if strings.EqualFold(r.Repo, repo) {
			return r, true
		}
	}
	return RepoCheck{}, false
}

// SetPollScaleForTests shortens device-flow polling. TEST USE ONLY.
func SetPollScaleForTests(d time.Duration) {
	pollInterval = func(time.Duration) time.Duration { return d }
}
