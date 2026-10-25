package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Arameair/identity/internal/redact"
)

// ErrNotSignedIn means there is no usable GitHub sign-in in this session.
var ErrNotSignedIn = errors.New("not signed in to GitHub in this session")

// ErrSignInRejected means GitHub no longer accepts this session's token.
var ErrSignInRejected = errors.New("GitHub no longer accepts this sign-in")

func (c *Client) currentToken() (string, *Account, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireTokenLocked()
	if c.status != Verified || c.token == "" || c.account == nil {
		return "", nil, ErrNotSignedIn
	}
	a := *c.account
	return c.token, &a, nil
}

// CheckRepo asks GitHub, with the person's own sign-in, whether they can see
// repo right now, and updates the stored publisher view.
func (c *Client) CheckRepo(ctx context.Context, repo string) (RepoCheck, *Account, error) {
	token, acct, err := c.currentToken()
	if err != nil {
		return RepoCheck{}, nil, err
	}
	var rp struct {
		Private bool `json:"private"`
	}
	st, err := c.api(ctx, token, "/repos/"+repo, &rp)
	rc := RepoCheck{Repo: repo, HTTP: st, CheckedAt: time.Now().UTC()}
	switch {
	case err != nil:
		rc.Result = "CHECK FAILED"
	case st == 200:
		rc.Result, rc.Private = "VISIBLE", rp.Private
	case st == 401:
		c.expireNow("GitHub no longer accepts this session's sign-in (revoked or expired). Connect again.")
		return rc, acct, ErrSignInRejected
	case st == 404 || st == 403:
		rc.Result = "NOT VISIBLE"
	default:
		rc.Result = "CHECK FAILED"
	}
	c.mu.Lock()
	if c.publisher != nil {
		found := false
		for i := range c.publisher.Repos {
			if strings.EqualFold(c.publisher.Repos[i].Repo, repo) {
				c.publisher.Repos[i], found = rc, true
			}
		}
		if !found {
			c.publisher.Repos = append(c.publisher.Repos, rc)
		}
	}
	c.mu.Unlock()
	return rc, acct, nil
}

func (c *Client) expireNow(msg string) {
	c.mu.Lock()
	redact.Forget(c.token)
	c.status, c.token, c.message = Expired, "", msg
	c.mu.Unlock()
	c.onEvent("warn", "github.session_rejected", "expired", msg)
}

// AppSeesRepo reports whether any installation of the sign-in app that this
// user can access includes repo. Used only to explain a NOT VISIBLE result to
// the repo's owner ("the app isn't installed on it").
func (c *Client) AppSeesRepo(ctx context.Context, repo string) (bool, error) {
	token, _, err := c.currentToken()
	if err != nil {
		return false, err
	}
	var inst struct {
		Installations []struct {
			ID int64 `json:"id"`
		} `json:"installations"`
	}
	if st, err := c.api(ctx, token, "/user/installations?per_page=50", &inst); err != nil || st != 200 {
		return false, fmt.Errorf("installations not readable (HTTP %d)", st)
	}
	for _, in := range inst.Installations {
		var rs struct {
			Repositories []struct {
				FullName string `json:"full_name"`
			} `json:"repositories"`
		}
		if st, err := c.api(ctx, token, fmt.Sprintf("/user/installations/%d/repositories?per_page=100", in.ID), &rs); err == nil && st == 200 {
			for _, r := range rs.Repositories {
				if strings.EqualFold(r.FullName, repo) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

type ReleaseAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type Release struct {
	Tag         string         `json:"tag"`
	Name        string         `json:"name"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

// ErrNoRelease means the repository has no published release.
var ErrNoRelease = errors.New("no published release")

// LatestRelease reads the newest published release of repo with the person's
// sign-in. For a private repository the app needs Contents: read.
func (c *Client) LatestRelease(ctx context.Context, repo string) (*Release, error) {
	token, _, err := c.currentToken()
	if err != nil {
		return nil, err
	}
	var r struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	st, err := c.api(ctx, token, "/repos/"+repo+"/releases/latest", &r)
	switch {
	case err != nil:
		return nil, errors.New("couldn't reach GitHub")
	case st == 404:
		return nil, ErrNoRelease
	case st == 401:
		c.expireNow("GitHub no longer accepts this session's sign-in. Connect again.")
		return nil, ErrSignInRejected
	case st == 403:
		return nil, errors.New("GitHub refused to show releases (HTTP 403) — the GitHub App may need Contents: read-only for this private repository")
	case st != 200:
		return nil, fmt.Errorf("GitHub answered HTTP %d", st)
	}
	out := &Release{Tag: r.TagName, Name: r.Name, PublishedAt: r.PublishedAt}
	for _, a := range r.Assets {
		out.Assets = append(out.Assets, ReleaseAsset{ID: a.ID, Name: a.Name, Size: a.Size})
	}
	return out, nil
}

// OpenAsset streams one release asset. GitHub answers with a redirect to its
// file storage; Go drops the Authorization header when following a redirect
// to a different host, so the token is only ever sent to GitHub's API.
func (c *Client) OpenAsset(ctx context.Context, repo string, assetID int64) (io.ReadCloser, int64, error) {
	token, _, err := c.currentToken()
	if err != nil {
		return nil, 0, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases/assets/%d", c.cfg.APIBase, repo, assetID), nil)
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Arameair-Identity")
	req.Header.Set("Authorization", "Bearer "+token)
	dl := &http.Client{Timeout: 10 * time.Minute}
	resp, err := dl.Do(req)
	if err != nil {
		return nil, 0, errors.New("couldn't reach GitHub to download")
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("GitHub answered HTTP %d for the download", resp.StatusCode)
	}
	return resp.Body, resp.ContentLength, nil
}
