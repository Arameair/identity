// Package redact removes credentials and one-time links from any text that
// may leave the process: log lines, Copy Diagnostics, error messages.
//
// Two layers:
//  1. Exact secrets registered at runtime (operator key, session cookie,
//     CSRF token, GitHub access token, device code, user code) are replaced
//     wherever they appear.
//  2. Pattern rules catch the same kinds of secret even if they were never
//     registered (for example a token pasted into an error message).
package redact

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

const Mask = "[REDACTED]"

var (
	mu      sync.RWMutex
	secrets = map[string]string{} // secret -> label
)

// Register adds an exact secret. Very short values are ignored so that
// ordinary words are never masked.
func Register(label, secret string) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 6 {
		return
	}
	mu.Lock()
	secrets[secret] = label
	mu.Unlock()
}

// Forget removes an exact secret (pattern rules still apply).
func Forget(secret string) {
	mu.Lock()
	delete(secrets, strings.TrimSpace(secret))
	mu.Unlock()
}

type rule struct {
	re   *regexp.Regexp
	repl string
}

var rules = []rule{
	// Private key blocks.
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "[REDACTED PRIVATE KEY]"},
	// One-time links: #key=… (Identity) and #code=… (Thought Processor), any other fragment secret.
	{regexp.MustCompile(`(?i)#(key|code|token|access_token|session)=[^\s"'<>&]*`), "#$1=" + Mask},
	// Query-string secrets.
	{regexp.MustCompile(`(?i)([?&](?:code|key|token|access_token|refresh_token|device_code|user_code|client_secret|state)=)[^\s"'<>&#]*`), "${1}" + Mask},
	// Authorization headers / bearer values.
	{regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)("?)[^\r\n"]+`), "${1}${2}" + Mask},
	{regexp.MustCompile(`(?i)\b(bearer|token)\s+[A-Za-z0-9_\-\.=]{8,}`), "$1 " + Mask},
	// Cookie headers.
	{regexp.MustCompile(`(?i)((?:set-)?cookie\s*[:=]\s*)[^\r\n]+`), "${1}" + Mask},
	{regexp.MustCompile(`(?i)\b(identity_b_session|tp_session|identity_trial)=[^\s;"']+`), "$1=" + Mask},
	// GitHub token formats (classic, fine-grained, app user/server/refresh).
	{regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9_]{16,})`), Mask},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`), Mask},
	// JSON / form fields that hold secrets.
	{regexp.MustCompile(`(?i)("?(?:access_token|refresh_token|device_code|user_code|client_secret|password|operator_code|operator_key|csrf|cookie|secret)"?\s*[:=]\s*"?)[^\s",}&]+`), "${1}" + Mask},
	// GitHub device-flow user codes look like ABCD-1234.
	{regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`), Mask},
	// Long random hex (operator keys, cookies). 40-hex git SHAs are kept on purpose.
	{regexp.MustCompile(`\b[a-fA-F0-9]{48,}\b`), Mask},
}

// String returns s with all known secrets removed.
func String(s string) string {
	if s == "" {
		return s
	}
	mu.RLock()
	list := make([]string, 0, len(secrets))
	for k := range secrets {
		list = append(list, k)
	}
	mu.RUnlock()
	// Longest first so a secret that contains another is fully masked.
	sort.Slice(list, func(i, j int) bool { return len(list[i]) > len(list[j]) })
	for _, k := range list {
		s = strings.ReplaceAll(s, k, Mask)
	}
	for _, r := range rules {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}

// Map redacts every value of a string map (keys are kept).
func Map(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = String(v)
	}
	return out
}
