package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

//go:embed web/index.html
var indexHTML []byte

type server struct {
	cfg     Config
	store   *Store
	events  *EventLog
	started time.Time
}

// Future pieces, shown as plain labels. Nothing here contacts anything.
var futureSlots = []map[string]string{
	{"name": "GitHub repository", "status": "NOT CONFIGURED"},
	{"name": "Thought Controller", "status": "NOT CONNECTED"},
	{"name": "Memory", "status": "NOT CONFIGURED"},
	{"name": "Discord", "status": "NOT CONFIGURED"},
	{"name": "Other modules", "status": "NOT CONFIGURED"},
}

type identityView struct {
	Identity
	TypeLabel  string `json:"type_label"`
	SaveStatus string `json:"save_status"`
}

func view(it Identity) identityView {
	return identityView{Identity: it, TypeLabel: typeLabel(it.EntityType),
		SaveStatus: "Saved locally — read back from disk"}
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.local(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(indexHTML)
	}))
	mux.HandleFunc("GET /api/status", s.local(s.handleStatus))
	mux.HandleFunc("GET /api/identities", s.local(func(w http.ResponseWriter, r *http.Request) {
		list := s.store.List()
		out := make([]identityView, 0, len(list))
		for _, it := range list {
			out = append(out, view(it))
		}
		writeJSON(w, 200, map[string]any{"identities": out, "problems": s.store.Problems()})
	}))
	mux.HandleFunc("GET /api/identities/{id}", s.local(func(w http.ResponseWriter, r *http.Request) {
		it, err := s.store.Get(r.PathValue("id"))
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"identity": view(it), "future": futureSlots})
	}))
	mux.HandleFunc("POST /api/identities", s.write(s.handleCreate))
	mux.HandleFunc("GET /api/diagnostics", s.local(s.handleDiagnostics))

	// Owner test tools for acceptance F. They only touch this app's own test data folder.
	mux.HandleFunc("POST /api/test/fail-next-save", s.write(func(w http.ResponseWriter, r *http.Request) {
		s.store.FailNextSave()
		s.events.Add("test-tool", "the next save will fail on purpose (acceptance F)", "")
		writeJSON(w, 200, map[string]string{"status": "the next Create will fail on purpose"})
	}))
	mux.HandleFunc("POST /api/test/corrupt-file", s.write(func(w http.ResponseWriter, r *http.Request) {
		name, err := s.store.WriteCorruptTestFile()
		if err != nil {
			s.events.Add("error", "couldn't write corrupt test file: "+err.Error(), "")
			writeErr(w, err)
			return
		}
		s.events.Add("test-tool", "wrote deliberately broken file "+name+" (acceptance F)", "")
		writeJSON(w, 200, map[string]string{"file": name})
	}))
	mux.HandleFunc("POST /api/test/remove-corrupt-files", s.write(func(w http.ResponseWriter, r *http.Request) {
		n, err := s.store.RemoveCorruptTestFiles()
		if err != nil {
			writeErr(w, err)
			return
		}
		s.events.Add("test-tool", fmt.Sprintf("removed %d corrupt test file(s)", n), "")
		writeJSON(w, 200, map[string]int{"removed": n})
	}))
	mux.HandleFunc("POST /api/reload", s.write(func(w http.ResponseWriter, r *http.Request) {
		if err := s.store.Load(); err != nil {
			s.events.Add("error", err.Error(), "")
			writeErr(w, err)
			return
		}
		s.events.Add("restore", fmt.Sprintf("reloaded from disk: %d identities, %d problem file(s)", len(s.store.List()), len(s.store.Problems())), "")
		writeJSON(w, 200, map[string]string{"status": "reloaded"})
	}))
	return mux
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DisplayName        string `json:"display_name"`
		EntityType         string `json:"entity_type"`
		RequestID          string `json:"request_id"`
		AllowDuplicateName bool   `json:"allow_duplicate_name"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields() // e.g. a caller can't choose its own "id"
	if err := dec.Decode(&in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid request: " + err.Error()})
		return
	}
	res, err := s.store.Create(in.DisplayName, in.EntityType, in.RequestID, in.AllowDuplicateName)
	if err != nil {
		if !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrDuplicateName) {
			s.events.Add("error", "create failed: "+err.Error(), "")
		}
		writeErr(w, err)
		return
	}
	if res.Repeat {
		s.events.Add("repeat", "same create request received again — returned the existing identity, nothing new created", res.Identity.ID)
	} else {
		s.events.Add("create", "created and saved "+typeLabel(res.Identity.EntityType), res.Identity.ID)
	}
	writeJSON(w, 201, map[string]any{"identity": view(res.Identity), "repeat": res.Repeat})
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"config":            s.cfg,
		"pid":               os.Getpid(),
		"started_at":        s.started.UTC(),
		"count":             len(s.store.List()),
		"problems":          s.store.Problems(),
		"events":            s.events.Recent(20),
		"event_log_error":   s.events.err,
		"fail_next_pending": s.store.FailNextPending(),
		"types":             EntityTypes,
		"future":            futureSlots,
	})
}

// handleDiagnostics: safe to paste anywhere. No names, no file contents, no
// secrets (none are stored). IDs are random test identifiers.
func (s *server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	c := s.cfg
	var b strings.Builder
	fmt.Fprintf(&b, "## Identity diagnostics (%s)\n\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Version: %s · Go %s · %s/%s\n", c.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "- Branch: %s (from %s)%s\n- Commit: %s\n", c.Branch, c.BranchFrom, map[bool]string{true: " · detached checkout", false: ""}[c.Detached], c.Commit)
	fmt.Fprintf(&b, "- Data folder: %s (from %s)\n- Port: %s · pid %d · up %s\n", c.DataDir, c.DataSource, c.Port, os.Getpid(), time.Since(s.started).Round(time.Second))
	list := s.store.List()
	fmt.Fprintf(&b, "- Saved identities: %d\n", len(list))
	for _, it := range list {
		fmt.Fprintf(&b, "  - %s · %s · created %s\n", it.ID, typeLabel(it.EntityType), it.CreatedAt.Format(time.RFC3339))
	}
	probs := s.store.Problems()
	fmt.Fprintf(&b, "- Problem files: %d\n", len(probs))
	for _, p := range probs {
		fmt.Fprintf(&b, "  - %s: %s\n", p.File, p.Reason)
	}
	if s.events.err != "" {
		fmt.Fprintf(&b, "- Event log: %s\n", s.events.err)
	}
	if s.store.FailNextPending() {
		b.WriteString("- Test tool: next save is set to fail\n")
	}
	b.WriteString("- Future pieces: GitHub NOT CONFIGURED · Thought Controller NOT CONNECTED · Memory/Discord/other NOT CONFIGURED (no external requests are made)\n")
	b.WriteString("\n### Recent events\n\n")
	for _, e := range s.events.Recent(15) {
		id := ""
		if e.ID != "" {
			id = " " + e.ID
		}
		fmt.Fprintf(&b, "- %s %s: %s%s\n", e.At.Format("15:04:05"), e.Kind, e.Detail, id)
	}
	b.WriteString("\n_Identity names are left out. No credentials exist in this app._\n")
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(b.String()))
}

// ---------- safety ----------

// local: only requests addressed to this machine's loopback name (blocks
// DNS-rebinding pages). The server also only listens on 127.0.0.1.
func (s *server) local(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" && host != "[::1]" {
			writeJSON(w, 403, map[string]string{"error": "host not allowed"})
			return
		}
		h(w, r)
	}
}

// write: local + same-origin + JSON body (stops other web pages posting forms).
func (s *server) write(h http.HandlerFunc) http.HandlerFunc {
	return s.local(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
			writeJSON(w, 403, map[string]string{"error": "origin not allowed"})
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeJSON(w, 415, map[string]string{"error": "send Content-Type: application/json"})
			return
		}
		h(w, r)
	})
}

func writeErr(w http.ResponseWriter, err error) {
	code := 500
	switch {
	case errors.Is(err, ErrInvalid):
		code = 400
	case errors.Is(err, ErrNotFound):
		code = 404
	case errors.Is(err, ErrDuplicateName):
		code = 409
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
