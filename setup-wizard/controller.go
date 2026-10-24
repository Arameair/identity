package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Controller talks to Project-controller's launch-request handoff. Identity
// can ask Controller to prepare a branch and read that request's progress.
// It cannot approve: the owner clicks "Review & allow" in Controller.
type Controller struct {
	base string // http://127.0.0.1:7777
	http *http.Client

	mu        sync.Mutex
	reqID     string
	secret    string
	expectSHA string // commit GitHub showed in step 3
	last      LaunchView
	busy      bool
}

func NewController(base string) *Controller {
	return &Controller{base: base, http: &http.Client{Timeout: 10 * time.Second,
		Transport: &http.Transport{Proxy: nil}}}
}

type LaunchView struct {
	Phase         string    `json:"phase"` // idle | controller_missing | controller_old | requesting | downloading | waiting_for_approval | building | starting | running | failed | stopped | gone | error
	Commit        string    `json:"commit,omitempty"`
	CommitMatch   *bool     `json:"commit_match,omitempty"`
	Port          int       `json:"port,omitempty"`
	URL           string    `json:"url,omitempty"`
	Error         string    `json:"error,omitempty"`
	Log           []string  `json:"log,omitempty"`
	CheckedAt     time.Time `json:"checked_at,omitempty"`
	ControllerURL string    `json:"controller_url"`
	signInURL     string
}

var codeRe = regexp.MustCompile(`code=[0-9a-f]{16,}`)

// sanitized hides the one-time sign-in code from anything shown as a log.
func (v LaunchView) sanitized() LaunchView {
	out := v
	out.Log = make([]string, len(v.Log))
	for i, l := range v.Log {
		out.Log[i] = codeRe.ReplaceAllString(l, "code=<hidden>")
	}
	return out
}

func (c *Controller) View() LaunchView {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.last.sanitized()
	v.ControllerURL = c.base + "/"
	if v.Phase == "" {
		v.Phase = "idle"
	}
	return v
}

func (c *Controller) SignInURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last.signInURL
}

func (c *Controller) set(v LaunchView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = v
}

// Available reports whether Controller is running and supports launch requests.
func (c *Controller) Available(ctx context.Context) (string, string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/launch-requests-pending", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return "controller_missing", "Project Controller isn't running at " + c.base + ". Open Project Controller, then click Try again."
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		return "controller_old", "Project Controller is running, but this version of it can't take requests from Identity yet. It needs the launch-request update (Project-controller branch experiment/identity-launch-request-20261024)."
	}
	return "ok", ""
}

// Request asks Controller to prepare the branch. A second call while a
// request is active reuses it (no duplicate panes or downloads).
func (c *Controller) Request(ctx context.Context, repo, branch, expectSHA string, retry bool) error {
	c.mu.Lock()
	if c.busy {
		c.mu.Unlock()
		return nil
	}
	active := c.reqID != "" && !retry
	switch c.last.Phase {
	case "failed", "stopped", "gone", "error", "controller_missing", "controller_old":
		active = false
	}
	if active {
		c.mu.Unlock()
		return nil
	}
	c.busy = true
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.busy = false; c.mu.Unlock() }()

	if st, msg := c.Available(ctx); st != "ok" {
		c.set(LaunchView{Phase: st, Error: msg, CheckedAt: time.Now().UTC()})
		return nil
	}
	c.set(LaunchView{Phase: "requesting", CheckedAt: time.Now().UTC()})
	body, _ := json.Marshal(map[string]string{"requester": "Identity", "repo": repo, "branch": branch})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/launch-requests", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		c.set(LaunchView{Phase: "controller_missing", Error: "Project Controller stopped answering. Open it, then click Try again.", CheckedAt: time.Now().UTC()})
		return nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode != 201 {
		c.set(LaunchView{Phase: "error", Error: "Project Controller couldn't prepare Thought Processor: " + strings.TrimSpace(string(b)), CheckedAt: time.Now().UTC()})
		return nil
	}
	var out struct{ ID, Secret string }
	if json.Unmarshal(b, &out) != nil || out.ID == "" || out.Secret == "" {
		c.set(LaunchView{Phase: "error", Error: "Project Controller gave an unreadable answer.", CheckedAt: time.Now().UTC()})
		return nil
	}
	c.mu.Lock()
	c.reqID, c.secret, c.expectSHA = out.ID, out.Secret, expectSHA
	c.mu.Unlock()
	c.Refresh(ctx)
	return nil
}

var openLineRe = regexp.MustCompile(`open\s*:\s*(http://127\.0\.0\.1:(\d+)/#code=[0-9a-f]{16,})`)

// Refresh reads the request's progress from Controller.
func (c *Controller) Refresh(ctx context.Context) {
	c.mu.Lock()
	id, secret, expect := c.reqID, c.secret, c.expectSHA
	c.mu.Unlock()
	if id == "" {
		return
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/launch-requests/"+id, nil)
	req.Header.Set("X-Launch-Request-Secret", secret)
	resp, err := c.http.Do(req)
	if err != nil {
		prev := c.View()
		c.set(LaunchView{Phase: "controller_missing", Error: "Project Controller stopped answering (it may have been closed). Open it, then click Try again.",
			Log: prev.Log, CheckedAt: time.Now().UTC()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		c.set(LaunchView{Phase: "gone", Error: "Project Controller no longer knows this request (it may have restarted). Click Try again.", CheckedAt: time.Now().UTC()})
		return
	}
	var r struct {
		Stage  string   `json:"stage"`
		Commit string   `json:"commit"`
		Port   int      `json:"port"`
		URL    string   `json:"url"`
		Error  string   `json:"error"`
		Log    []string `json:"log"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&r); err != nil {
		c.set(LaunchView{Phase: "error", Error: "Project Controller gave an unreadable answer.", CheckedAt: time.Now().UTC()})
		return
	}
	v := LaunchView{Phase: r.Stage, Commit: r.Commit, Port: r.Port, Error: r.Error, Log: r.Log, CheckedAt: time.Now().UTC()}
	if r.Port > 0 && strings.HasPrefix(r.URL, fmt.Sprintf("http://127.0.0.1:%d/", r.Port)) {
		v.URL = r.URL
	}
	if r.Commit != "" && expect != "" {
		m := r.Commit == expect
		v.CommitMatch = &m
	}
	// The sign-in link Processor printed for THIS run, on THIS port.
	if r.Port > 0 {
		for i := len(r.Log) - 1; i >= 0; i-- {
			if m := openLineRe.FindStringSubmatch(r.Log[i]); m != nil && m[2] == fmt.Sprint(r.Port) {
				v.signInURL = m[1]
				break
			}
		}
	}
	c.set(v)
}

func (c *Controller) Active() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reqID != ""
}
