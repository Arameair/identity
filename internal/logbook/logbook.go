// Package logbook is Identity's persistent activity log.
//
// Every entry is redacted before it is written, appended as one JSON line to
// <profile>/logs/identity-events.jsonl, and kept in a bounded in-memory ring
// for the live UI. On start the tail of the file is reloaded so the log
// survives restarts. The file rotates at MaxBytes, keeping Keep old copies.
package logbook

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Arameair/identity/internal/redact"
)

const (
	FileName = "identity-events.jsonl"
	MaxBytes = 1 << 20 // 1 MiB per file
	Keep     = 3       // identity-events.jsonl.1 … .3
	RingSize = 1000
)

type Entry struct {
	ID        int64             `json:"id"`
	Time      time.Time         `json:"ts"`    // UTC
	Local     string            `json:"local"` // same instant with the machine's zone
	RunID     string            `json:"run_id"`
	Level     string            `json:"level"` // info | warn | error
	Component string            `json:"component"`
	Event     string            `json:"event"`
	RequestID string            `json:"request_id,omitempty"`
	Result    string            `json:"result,omitempty"`
	Detail    string            `json:"detail,omitempty"`
	State     map[string]string `json:"state,omitempty"`
}

type Stats struct {
	Path            string `json:"path"`
	InMemory        int    `json:"in_memory"`
	LoadedFromDisk  int    `json:"loaded_from_disk"`
	UnreadableLines int    `json:"unreadable_lines"`
	WriteErrors     int    `json:"write_errors"`
	LastWriteError  string `json:"last_write_error,omitempty"`
	Rotations       int    `json:"rotations"`
	LastID          int64  `json:"last_id"`
}

type Book struct {
	mu     sync.Mutex
	dir    string
	path   string
	runID  string
	ring   []Entry
	lastID int64
	stats  Stats
	now    func() time.Time
}

// Open creates the log directory if needed and reloads recent history.
func Open(dir, runID string) (*Book, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("log folder: %w", err)
	}
	b := &Book{dir: dir, path: filepath.Join(dir, FileName), runID: runID, now: time.Now}
	b.stats.Path = b.path
	b.reload()
	return b, nil
}

func (b *Book) reload() {
	// Older rotated file first, then current, so order is preserved.
	for _, p := range []string{b.path + ".1", b.path} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), 64<<10)
		for sc.Scan() {
			var e Entry
			if err := json.Unmarshal(sc.Bytes(), &e); err != nil || e.ID <= 0 {
				b.stats.UnreadableLines++
				continue
			}
			b.push(e)
			b.stats.LoadedFromDisk++
			if e.ID > b.lastID {
				b.lastID = e.ID
			}
		}
		if sc.Err() != nil {
			b.stats.UnreadableLines++
		}
		f.Close()
	}
	b.stats.LoadedFromDisk = len(b.ring)
}

func (b *Book) push(e Entry) {
	b.ring = append(b.ring, e)
	if len(b.ring) > RingSize {
		b.ring = append([]Entry(nil), b.ring[len(b.ring)-RingSize:]...)
	}
}

// Add records an event. Strings and state values are redacted first.
// It never panics and never blocks on a failing disk: the entry stays in
// memory and the failure is counted and visible in diagnostics.
func (b *Book) Add(level, component, event, requestID, result, detail string, state map[string]string) Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now()
	b.lastID++
	e := Entry{
		ID: b.lastID, Time: t.UTC(), Local: t.Format("2006-01-02 15:04:05 MST -07:00"), RunID: b.runID,
		Level: level, Component: component, Event: event, RequestID: requestID,
		Result: redact.String(result), Detail: redact.String(detail), State: redact.Map(state),
	}
	b.push(e)
	if err := b.write(e); err != nil {
		b.stats.WriteErrors++
		b.stats.LastWriteError = redact.String(err.Error())
	}
	return e
}

func (b *Book) Info(component, event, requestID, result, detail string, state map[string]string) Entry {
	return b.Add("info", component, event, requestID, result, detail, state)
}
func (b *Book) Warn(component, event, requestID, result, detail string, state map[string]string) Entry {
	return b.Add("warn", component, event, requestID, result, detail, state)
}
func (b *Book) Error(component, event, requestID, result, detail string, state map[string]string) Entry {
	return b.Add("error", component, event, requestID, result, detail, state)
}

func (b *Book) write(e Entry) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(b.path); err == nil && fi.Size()+int64(len(line))+1 > MaxBytes {
		if err := b.rotate(); err != nil {
			return fmt.Errorf("rotate: %w", err)
		}
	}
	f, err := os.OpenFile(b.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	cerr := f.Close()
	return errors.Join(werr, cerr)
}

func (b *Book) rotate() error {
	os.Remove(fmt.Sprintf("%s.%d", b.path, Keep))
	for i := Keep - 1; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", b.path, i), fmt.Sprintf("%s.%d", b.path, i+1))
	}
	if err := os.Rename(b.path, b.path+".1"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b.stats.Rotations++
	return nil
}

// After returns up to limit entries with ID > after, oldest first.
func (b *Book) After(after int64, limit int) []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []Entry{}
	for _, e := range b.ring {
		if e.ID > after {
			out = append(out, e)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// Recent returns the last n entries, optionally only warn/error.
func (b *Book) Recent(n int, problemsOnly bool) []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []Entry{}
	for i := len(b.ring) - 1; i >= 0 && len(out) < n; i-- {
		e := b.ring[i]
		if problemsOnly && e.Level == "info" {
			continue
		}
		out = append(out, e)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (b *Book) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.stats
	s.InMemory = len(b.ring)
	s.LastID = b.lastID
	return s
}

// Line renders one entry as a single readable text line.
func Line(e Entry) string {
	s := fmt.Sprintf("%s  %-5s %-10s %-28s", e.Local, e.Level, e.Component, e.Event)
	if e.RequestID != "" {
		s += " " + e.RequestID
	}
	if e.Result != "" {
		s += " result=" + e.Result
	}
	if e.Detail != "" {
		s += " — " + e.Detail
	}
	if len(e.State) > 0 {
		b, _ := json.Marshal(e.State)
		s += " " + string(b)
	}
	return s
}

// CopyFile is used by tests to inspect the raw file.
func (b *Book) ReadRaw() (string, error) {
	f, err := os.Open(b.path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	return string(raw), err
}
