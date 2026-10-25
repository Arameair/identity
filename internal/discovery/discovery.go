// Package discovery looks — read-only — for a Thought Processor at one
// address the owner chose. Rules (same as Processor's own probe.go):
//   - only an explicitly configured loopback address; no scanning
//   - no credentials, cookies or session sent; no control request
//   - only GET /api/health, the public self-description, is read
//   - every field it returns is an UNTRUSTED CLAIM
//
// Discovery itself stays unauthenticated (pairing is separate), so the best
// possible result is DETECTED — SELF-REPORTED, UNVERIFIED. Never "verified".
package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	NotConfigured = "NOT CONFIGURED"
	NotRunning    = "NOT RUNNING"
	NoAnswer      = "NO ANSWER"
	Unrecognized  = "UNRECOGNIZED SERVICE"
	WrongService  = "WRONG SERVICE"
	Detected      = "DETECTED — SELF-REPORTED, UNVERIFIED"

	tcpTimeout    = 800 * time.Millisecond
	healthTimeout = 1500 * time.Millisecond
	StaleAfter    = 30 * time.Second
)

// ParseAddress accepts host:port or http://host:port on this computer only.
// selfPort is refused so Identity cannot be pointed at itself.
func ParseAddress(raw string, selfPort int) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 64 {
		return "", errors.New("address too long")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("not a valid address")
	}
	if u.Scheme != "http" {
		return "", errors.New("only http:// on this computer is supported")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("give only host and port (no user, path, query or #fragment)")
	}
	host, port := u.Hostname(), u.Port()
	switch host {
	case "127.0.0.1", "::1", "localhost":
	default:
		return "", errors.New("only this computer (127.0.0.1, ::1 or localhost) is allowed; other machines are refused")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("an explicit port 1–65535 is required, e.g. 127.0.0.1:7860")
	}
	if selfPort > 0 && n == selfPort {
		return "", errors.New("that is Identity's own port")
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}

type Result struct {
	Address         string    `json:"address"`
	State           string    `json:"state"`
	CheckedAt       time.Time `json:"checked_at"`
	ElapsedMS       int64     `json:"elapsed_ms"`
	HTTPStatus      int       `json:"http_status,omitempty"`
	ClaimedApp      string    `json:"claimed_app,omitempty"`
	ClaimedVersion  string    `json:"claimed_version,omitempty"`
	ClaimedProtocol string    `json:"claimed_protocol,omitempty"`
	Note            string    `json:"note"`
	OpenURL         string    `json:"open_url,omitempty"` // original UI root, no code
	Fresh           string    `json:"freshness"`          // CURRENT | STALE | NEVER CHECKED
}

var client = &http.Client{
	Timeout: healthTimeout,
	Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true,
		DialContext: (&net.Dialer{Timeout: tcpTimeout}).DialContext},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func clean(s string, n int) string {
	var b strings.Builder
	for _, r := range s {
		if b.Len() >= n {
			break
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(".-_/", r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Probe performs one read-only check of addr (already parsed).
func Probe(ctx context.Context, addr string) Result {
	t0 := time.Now()
	r := Result{Address: addr, CheckedAt: t0.UTC(), Fresh: "CURRENT"}
	if addr == "" {
		r.State, r.Note = NotConfigured, "no Thought Processor address has been chosen"
		return r
	}
	c, err := (&net.Dialer{Timeout: tcpTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		r.ElapsedMS = time.Since(t0).Milliseconds()
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			r.State, r.Note = NoAnswer, "no answer within "+tcpTimeout.String()
		} else {
			r.State, r.Note = NotRunning, "nothing is listening on that port"
		}
		return r
	}
	c.Close()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/api/health", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	r.ElapsedMS = time.Since(t0).Milliseconds()
	if err != nil {
		r.State, r.Note = NoAnswer, "the port is open but the public health request failed or timed out"
		return r
	}
	defer resp.Body.Close()
	r.HTTPStatus = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		r.State, r.Note = Unrecognized, "the port is open, but GET /api/health answered HTTP "+strconv.Itoa(resp.StatusCode)
		return r
	}
	var m map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&m); err != nil {
		r.State, r.Note = Unrecognized, "the port is open, but /api/health is not a small JSON object"
		return r
	}
	app, _ := m["app"].(string)
	ver, _ := m["version"].(string)
	proto, _ := m["protocol"].(string)
	r.ClaimedApp, r.ClaimedVersion, r.ClaimedProtocol = clean(app, 40), clean(ver, 40), clean(proto, 60)
	if r.ClaimedApp != "thought-processor" || !strings.HasPrefix(r.ClaimedProtocol, "thought-processor.") {
		r.State = WrongService
		r.Note = "something answered, but it does not describe itself as Thought Processor (app=" + orNone(r.ClaimedApp) + ", protocol=" + orNone(r.ClaimedProtocol) + ")"
		return r
	}
	r.State = Detected
	r.OpenURL = "http://" + addr + "/"
	r.Note = "it describes itself as Thought Processor. That is only a self-claim; what Identity trusts is the pairing below — requests carrying the pairing token, not this address."
	return r
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// Watcher re-probes the configured address on an interval and reports state
// transitions (not every probe) so the log stays readable.
type Watcher struct {
	mu       sync.Mutex
	addr     string
	last     *Result
	onChange func(prev, cur Result)
	kick     chan struct{}
}

func NewWatcher(addr string, onChange func(prev, cur Result)) *Watcher {
	return &Watcher{addr: addr, onChange: onChange, kick: make(chan struct{}, 1)}
}

func (w *Watcher) SetAddress(addr string) {
	w.mu.Lock()
	w.addr, w.last = addr, nil
	w.mu.Unlock()
	w.Kick()
}

func (w *Watcher) Address() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.addr
}

func (w *Watcher) Kick() {
	select {
	case w.kick <- struct{}{}:
	default:
	}
}

// CheckNow probes immediately and returns the result.
func (w *Watcher) CheckNow(ctx context.Context) Result {
	addr := w.Address()
	r := Probe(ctx, addr)
	w.mu.Lock()
	var prev Result
	if w.last != nil {
		prev = *w.last
	}
	changed := w.addr == addr && (w.last == nil || w.last.State != r.State || w.last.ClaimedVersion != r.ClaimedVersion)
	if w.addr == addr {
		w.last = &r
	}
	w.mu.Unlock()
	if changed && w.onChange != nil {
		w.onChange(prev, r)
	}
	return r
}

func (w *Watcher) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		w.CheckNow(pctx)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-w.kick:
		}
	}
}

// Current returns the last result with freshness computed now.
func (w *Watcher) Current() Result {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.last == nil {
		if w.addr == "" {
			return Result{State: NotConfigured, Fresh: "NEVER CHECKED", Note: "no Thought Processor address has been chosen"}
		}
		return Result{Address: w.addr, State: "NOT CHECKED YET", Fresh: "NEVER CHECKED"}
	}
	r := *w.last
	if time.Since(r.CheckedAt) > StaleAfter {
		r.Fresh = "STALE"
		r.OpenURL = ""
	}
	return r
}
