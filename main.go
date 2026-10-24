// Identity is a standalone local identity-card and GitHub account manager.
// It never creates, modifies, or deletes repositories. Removing a card archives
// local state, preserving the original card and any repository association.
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/index.html
var webFS embed.FS

const schema = 1
const maxBody = 8192

var types = map[string]bool{"ai_companion": true, "business": true, "person": true, "project": true, "other": true}
var repoNameRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)
var loginRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
var idRE = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)

type Card struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Repo       string     `json:"repo,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}
type Account struct {
	Login    string    `json:"login"`
	LinkedAt time.Time `json:"linked_at"`
	Provider string    `json:"provider"`
}
type State struct {
	Schema   int      `json:"schema"`
	SystemID string   `json:"system_id"`
	Cards    []Card   `json:"cards"`
	GitHub   *Account `json:"github,omitempty"`
}
type GHStatus struct {
	State        string    `json:"state"`
	Login        string    `json:"login,omitempty"`
	BoundAccount string    `json:"bound_account,omitempty"`
	Detail       string    `json:"detail"`
	CheckedAt    time.Time `json:"checked_at"`
}
type RepoInfo struct {
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	URL      string `json:"html_url"`
}
type App struct {
	mu                            sync.Mutex
	dataDir, branch, commit, port string
	state                         State
	github                        githubOps
	controller                    *ControllerDiscovery
}
type githubOps interface {
	User(context.Context) (string, error)
	Repository(context.Context, string) (RepoInfo, error)
}
type ghCLI struct{}

func uuid() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
func safeName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > 80 {
		return "", errors.New("name must be 1–80 characters")
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return "", errors.New("name contains control characters")
		}
	}
	return s, nil
}
func cleanEnv(env []string) []string {
	block := map[string]bool{"GH_TOKEN": true, "GITHUB_TOKEN": true, "GH_ENTERPRISE_TOKEN": true, "GITHUB_ENTERPRISE_TOKEN": true, "GH_HOST": true, "GITHUB_HOST": true, "GH_PROMPT_DISABLED": true}
	out := make([]string, 0, len(env)+3)
	for _, s := range env {
		k, _, _ := strings.Cut(s, "=")
		if !block[strings.ToUpper(k)] {
			out = append(out, s)
		}
	}
	return append(out, "GH_HOST=github.com", "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1")
}
func ghRun(ctx context.Context, args ...string) ([]byte, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return nil, errors.New("GitHub CLI (gh) is not installed here. Install it from https://cli.github.com/")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = cleanEnv(os.Environ())
	// Never include stderr in an error; it may contain sensitive info.
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("GitHub CLI authentication or network check failed. Run gh auth login --hostname github.com --web in a terminal on this same computer, then retry")
	}
	if len(out) > 32768 {
		return nil, errors.New("GitHub CLI response exceeds size limit")
	}
	return out, nil
}
func (ghCLI) User(ctx context.Context) (string, error) {
	out, err := ghRun(ctx, "api", "user", "--hostname", "github.com", "--jq", ".login")
	if err != nil {
		return "", err
	}
	login := strings.TrimSpace(string(out))
	if !loginRE.MatchString(login) {
		return "", errors.New("GitHub returned an invalid username")
	}
	return login, nil
}
func (ghCLI) Repository(ctx context.Context, repo string) (RepoInfo, error) {
	if !repoNameRE.MatchString(repo) {
		return RepoInfo{}, errors.New("enter an exact repository: owner/name")
	}
	out, err := ghRun(ctx, "api", "repos/"+repo, "--hostname", "github.com", "--jq", "{full_name,private,html_url}")
	if err != nil {
		return RepoInfo{}, err
	}
	var info RepoInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return info, errors.New("invalid GitHub repository response")
	}
	if !strings.EqualFold(info.FullName, repo) || !strings.HasPrefix(info.URL, "https://github.com/") {
		return RepoInfo{}, errors.New("GitHub returned unexpected repository identity")
	}
	return info, nil
}

// acquireLock ensures two copies cannot write the same identity directory.
// A crash may leave a lock; never silently delete it because a live owner
// might still be using the store. Diagnose before manually clearing it.
func acquireLock(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(dir, "identity.lock")
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("identity data folder is locked or unwritable (%s); stop the other instance before retrying: %w", lockPath, err)
	}
	_, _ = fmt.Fprintf(f, "pid=%d started=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	return func() { _ = f.Close(); _ = os.Remove(lockPath) }, nil
}
func gitValue(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
func within(path, parent string) bool {
	if parent == "" || parent == "unknown" {
		return false
	}
	rel, err := filepath.Rel(parent, path)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
func openApp(dir, branch, commit, port string, g githubOps) (*App, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	a := &App{dataDir: abs, branch: branch, commit: commit, port: port, github: g}
	if a.github == nil {
		a.github = ghCLI{}
	}
	path := filepath.Join(abs, "identity.json")
	content, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		id, e := uuid()
		if e != nil {
			return nil, e
		}
		a.state = State{Schema: schema, SystemID: id, Cards: []Card{}}
		if e = a.saveLocked(); e != nil {
			return nil, e
		}
	case err != nil:
		return nil, err
	default:
		if err = json.Unmarshal(content, &a.state); err != nil {
			return nil, fmt.Errorf("cannot read identity store (left unchanged): %w", err)
		}
		if a.state.Schema != schema || !idRE.MatchString(a.state.SystemID) {
			return nil, errors.New("unsupported or corrupt identity store; left unchanged")
		}
		seen := map[string]bool{}
		for _, c := range a.state.Cards {
			if !idRE.MatchString(c.ID) || seen[c.ID] || !types[c.Type] {
				return nil, errors.New("invalid existing identity record; left unchanged")
			}
			seen[c.ID] = true
		}
		if a.state.Cards == nil {
			a.state.Cards = []Card{}
		}
	}
	discovery, err := openControllerDiscovery(abs)
	if err != nil { return nil, err }
	a.controller = discovery
	return a, nil
}
func (a *App) saveLocked() error {
	b, err := json.MarshalIndent(a.state, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(a.dataDir, ".identity-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, filepath.Join(a.dataDir, "identity.json")); err != nil {
		return err
	}
	return nil
}
func (a *App) mutate(fn func(*State) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, _ := json.Marshal(a.state)
	var next State
	_ = json.Unmarshal(b, &next)
	if err := fn(&next); err != nil {
		return err
	}
	prior := a.state
	a.state = next
	if err := a.saveLocked(); err != nil {
		a.state = prior
		return err
	}
	return nil
}
func (a *App) snapshot() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, _ := json.Marshal(a.state)
	var state State
	_ = json.Unmarshal(b, &state)
	sort.Slice(state.Cards, func(i, j int) bool { return state.Cards[i].CreatedAt.Before(state.Cards[j].CreatedAt) })
	return state
}
func (a *App) add(name, typ string) (Card, error) {
	var card Card
	err := a.mutate(func(s *State) error {
		name, e := safeName(name)
		if e != nil {
			return e
		}
		if !types[typ] {
			return errors.New("select a supported identity type")
		}
		for _, c := range s.Cards {
			if c.ArchivedAt == nil && c.Type == typ && strings.EqualFold(c.Name, name) {
				return errors.New("an active identity already uses that name and type")
			}
		}
		id, e := uuid()
		if e != nil {
			return e
		}
		card = Card{ID: id, Name: name, Type: typ, CreatedAt: time.Now().UTC()}
		s.Cards = append(s.Cards, card)
		return nil
	})
	return card, err
}
func (a *App) archive(id string, undo bool) error {
	return a.mutate(func(s *State) error {
		for i := range s.Cards {
			c := &s.Cards[i]
			if c.ID != id {
				continue
			}
			if undo {
				if c.ArchivedAt == nil {
					return errors.New("identity is already active")
				}
				c.ArchivedAt = nil
			} else {
				if c.ArchivedAt != nil {
					return errors.New("identity is already removed")
				}
				now := time.Now().UTC()
				c.ArchivedAt = &now
			}
			return nil
		}
		return errors.New("identity not found")
	})
}
func (a *App) linkRepo(id, repo string) error {
	if !repoNameRE.MatchString(repo) {
		return errors.New("enter owner/name")
	}
	if _, err := a.boundLogin(context.Background()); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := a.github.Repository(ctx, repo)
	if err != nil {
		return err
	}
	if !info.Private {
		return errors.New("only private repositories can be attached to identity cards")
	}
	return a.mutate(func(s *State) error {
		for i := range s.Cards {
			if s.Cards[i].ID == id {
				if s.Cards[i].ArchivedAt != nil {
					return errors.New("restore the identity before attaching a repository")
				}
				s.Cards[i].Repo = info.FullName
				return nil
			}
		}
		return errors.New("identity not found")
	})
}
func (a *App) boundLogin(ctx context.Context) (string, error) {
	a.mu.Lock()
	bound := ""
	if a.state.GitHub != nil {
		bound = a.state.GitHub.Login
	}
	a.mu.Unlock()
	if bound == "" {
		return "", errors.New("connect your GitHub account in Identity first")
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 10*time.Second {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	who, err := a.github.User(ctx)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(bound, who) {
		return "", errors.New("the active GitHub CLI account differs from the one linked to this system. Reconnect deliberately")
	}
	return who, nil
}
func (a *App) connect() (GHStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	who, err := a.github.User(ctx)
	if err != nil {
		return GHStatus{}, err
	}
	err = a.mutate(func(s *State) error {
		if s.GitHub != nil && !strings.EqualFold(s.GitHub.Login, who) {
			return errors.New("different GitHub account already linked; disconnect it first")
		}
		if s.GitHub == nil {
			s.GitHub = &Account{Login: who, LinkedAt: time.Now().UTC(), Provider: "official-gh-cli"}
		}
		return nil
	})
	if err != nil {
		return GHStatus{}, err
	}
	return a.githubStatus(), nil
}
func (a *App) githubStatus() GHStatus {
	a.mu.Lock()
	bound := ""
	if a.state.GitHub != nil {
		bound = a.state.GitHub.Login
	}
	a.mu.Unlock()
	out := GHStatus{CheckedAt: time.Now().UTC(), BoundAccount: bound}
	if bound == "" {
		out.State = "NOT CONNECTED"
		out.Detail = "Sign in with the official GitHub CLI, then select Connect account. No token is ever copied to Identity."
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	who, err := a.github.User(ctx)
	if err != nil {
		out.State = "AUTHENTICATION REQUIRED"
		out.Detail = err.Error()
		return out
	}
	out.Login = who
	if !strings.EqualFold(who, bound) {
		out.State = "ACCOUNT MISMATCH"
		out.Detail = "The active GitHub CLI account does not match the account linked to this system."
		return out
	}
	out.State = "CONNECTED"
	out.Detail = "GitHub verified this account using the official CLI. Identity stores only the account name, not the credential."
	return out
}
func (a *App) disconnect() error {
	return a.mutate(func(s *State) error { s.GitHub = nil; return nil })
}
func (a *App) status() map[string]any {
	return map[string]any{"state": a.snapshot(), "github": a.githubStatus(), "branch": a.branch, "commit": a.commit, "data_dir": a.dataDir, "port": a.port, "mode": "REAL LOCAL RECORDS + REAL GITHUB CLI VERIFICATION", "repository_deletion_supported": false}
}
func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func bad(w http.ResponseWriter, err error) { reply(w, 400, map[string]string{"error": err.Error()}) }
func read(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		bad(w, err)
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		bad(w, errors.New("unexpected extra input"))
		return false
	}
	return true
}
func localOnly(next http.HandlerFunc, write bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			reply(w, 403, map[string]string{"error": "local host only"})
			return
		}
		if write {
			origin := r.Header.Get("Origin")
			if origin != "http://"+r.Host {
				reply(w, 403, map[string]string{"error": "this action requires the local Identity page"})
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				reply(w, 415, map[string]string{"error": "JSON required"})
				return
			}
		}
		next(w, r)
	}
}
func (a *App) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", localOnly(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		file, _ := webFS.ReadFile("web/index.html")
		_, _ = w.Write(file)
	}, false))
	mux.HandleFunc("GET /api/status", localOnly(func(w http.ResponseWriter, r *http.Request) { reply(w, 200, a.status()) }, false))
	mux.HandleFunc("POST /api/identities", localOnly(func(w http.ResponseWriter, r *http.Request) {
		var p struct{ Name, Type string }
		if !read(w, r, &p) {
			return
		}
		c, e := a.add(p.Name, p.Type)
		if e != nil {
			bad(w, e)
			return
		}
		reply(w, 201, c)
	}, true))
	mux.HandleFunc("POST /api/identities/{id}/remove", localOnly(func(w http.ResponseWriter, r *http.Request) {
		if e := a.archive(r.PathValue("id"), false); e != nil {
			bad(w, e)
			return
		}
		reply(w, 200, map[string]string{"status": "REMOVED LOCALLY — GITHUB REPOSITORY UNCHANGED"})
	}, true))
	mux.HandleFunc("POST /api/identities/{id}/restore", localOnly(func(w http.ResponseWriter, r *http.Request) {
		if e := a.archive(r.PathValue("id"), true); e != nil {
			bad(w, e)
			return
		}
		reply(w, 200, map[string]string{"status": "RESTORED"})
	}, true))
	mux.HandleFunc("POST /api/identities/{id}/repo", localOnly(func(w http.ResponseWriter, r *http.Request) {
		var p struct{ Repo string }
		if !read(w, r, &p) {
			return
		}
		if e := a.linkRepo(r.PathValue("id"), p.Repo); e != nil {
			bad(w, e)
			return
		}
		reply(w, 200, map[string]string{"status": "VERIFIED PRIVATE REPOSITORY ASSOCIATION SAVED — NO GITHUB WRITE"})
	}, true))
	mux.HandleFunc("POST /api/github/connect", localOnly(func(w http.ResponseWriter, r *http.Request) {
		out, e := a.connect()
		if e != nil {
			bad(w, e)
			return
		}
		reply(w, 200, out)
	}, true))
	mux.HandleFunc("POST /api/github/disconnect", localOnly(func(w http.ResponseWriter, r *http.Request) {
		if e := a.disconnect(); e != nil {
			bad(w, e)
			return
		}
		reply(w, 200, map[string]string{"status": "ACCOUNT UNLINKED LOCALLY — CLI SESSION UNCHANGED"})
	}, true))
	mux.HandleFunc("GET /api/controller/status",localOnly(func(w http.ResponseWriter,r *http.Request){
		reply(w,200,a.controller.Check())
	},false))
	mux.HandleFunc("POST /api/controller/find",localOnly(func(w http.ResponseWriter,r *http.Request){
		reply(w,200,map[string]any{"controllers":a.controller.Scan(),"scanned_ports":"7778–7899","scope":"127.0.0.1 only"})
	},true))
	mux.HandleFunc("POST /api/controller/port",localOnly(func(w http.ResponseWriter,r *http.Request){
		var p struct{Port int `json:"port"`}
		if !read(w,r,&p){return}
		if err:=a.controller.SetPort(p.Port);err!=nil{bad(w,err);return}
		reply(w,200,a.controller.Check())
	},true))
	mux.HandleFunc("POST /api/github/check", localOnly(func(w http.ResponseWriter, r *http.Request) { reply(w, 200, a.githubStatus()) }, true))
	return mux
}
func main() {
	data := flag.String("data", "", "local identity directory")
	flag.Parse()
	port := os.Getenv("PORT")
	if port == "" {
		port = "7796"
	}
	branch := os.Getenv("PC_BRANCH")
	if branch == "" {
		branch = gitValue("rev-parse", "--abbrev-ref", "HEAD")
	}
	sha := gitValue("rev-parse", "HEAD")
	dir := *data
	if dir == "" {
		dir = os.Getenv("IDENTITY_DATA")
	}
	if dir == "" {
		if pc := os.Getenv("PC_DATA"); pc != "" {
			dir = filepath.Join(pc, "identity-real")
		}
	}
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".identity-real", branch)
	}
	abs, _ := filepath.Abs(dir)
	if within(abs, gitValue("rev-parse", "--show-toplevel")) {
		log.Fatal("data folder must be outside git checkout")
	}
	unlock, err := acquireLock(abs)
	if err != nil {
		log.Fatal(err)
	}
	defer unlock()
	app, err := openApp(dir, branch, sha, port, nil)
	if err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Identity local app on http://127.0.0.1:%s/; GitHub credentials remain owned by gh; no repository deletion feature; OS %s", port, runtime.GOOS)
	srv := &http.Server{Handler: app.handler(), ReadHeaderTimeout: 6 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Print(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
