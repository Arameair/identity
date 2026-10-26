package app

// Route inventory (Identity #14: "Every protected route is inventoried, with
// one documented enforcement guard"). Every pattern registered on the mux is
// recorded; TestRouteInventory fails if a route has no entry here, or if an
// entry names a route that no longer exists. The guard text is what the
// route's wrap() level and handler actually enforce.

import (
	"net/http"
	"sort"
)

const (
	gPublic       = "PUBLIC — static page/asset or self-reported health; no data, no action"
	gWebSession   = "LOCAL OPERATOR SESSION — Identity troubleshooting page: private per-run link cookie + CSRF + Origin + Host"
	gWebBootstrap = "LOCAL OPERATOR BOOTSTRAP — exchanges this run's private link for a session (throttled)"
	gMachine      = "MACHINE — no browser Origin; pairing code / request handshake only"
	gPaired       = "PAIRED ROUTINE — identity/2 signed request from a paired Processor; signed response; no Admin authority"
	gElevated     = "ELEVATED — paired Processor AND a consumed Admin approval (kind-, requester-, expiry- and digest-bound; made with the Admin authenticator in Identity's own page)"
	gInstallation = "INSTALLATION — identity/2 signed request with a feature installation's own credential (separate registry); signed response"
	gApprovalPage = "APPROVAL PAGE — one-time link → approval-scoped cookie + CSRF + Origin; the Admin passphrase is checked here"
)

// RouteGuards documents every route. Elevated routes must use beginApproval.
var RouteGuards = map[string]string{
	"GET /{$}":    gPublic,
	"GET /app.js": gPublic, "GET /app.css": gPublic, "GET /api/health": gPublic,
	"GET /approve": gPublic, "GET /approve.js": gPublic,

	"POST /api/session": gWebBootstrap,
	"GET /api/session":  gWebSession, "GET /api/state": gWebSession, "GET /api/logs": gWebSession, "GET /api/diagnostics": gWebSession,
	"POST /api/diagnostics/result": gWebSession, "POST /api/github/own-app": gWebSession, "POST /api/github/own-app/remove": gWebSession,
	"POST /api/setup/complete": gWebSession, "POST /api/setup/reopen": gWebSession, "POST /api/github/start": gWebSession,
	"POST /api/github/cancel": gWebSession, "POST /api/github/disconnect": gWebSession, "POST /api/github/recheck": gWebSession,
	"POST /api/processor/address": gWebSession, "POST /api/processor/check": gWebSession, "POST /api/processor/opened": gWebSession,
	"POST /api/drafts": gWebSession, "POST /api/drafts/withdraw": gWebSession, "POST /api/shutdown": gWebSession,
	"POST /api/pairing/code": gWebSession, "POST /api/pairing/cancel": gWebSession, "POST /api/pairing/unpair": gWebSession,
	"POST /api/access/check": gWebSession,

	"POST /api/v1/pair/request": gMachine, "POST /api/v1/pair": gMachine,

	"POST /api/v1/unpair": gPaired, "GET /api/v1/status": gPaired, "GET /api/v1/identities": gPaired, "POST /api/v1/identities": gPaired,
	"GET /api/v1/identities/{id}": gPaired, "POST /api/v1/identities/{id}/rename": gPaired, "POST /api/v1/identities/{id}/archive": gPaired,
	"POST /api/v1/identities/{id}/restore": gPaired, "GET /api/v1/features": gPaired, "GET /api/v1/identities/{id}/features": gPaired,
	"GET /api/v1/features/{id}/eligibility": gPaired, "GET /api/v1/features/{id}/contract": gPaired, "GET /api/v1/features/{id}/release": gPaired,
	"GET /api/v1/features/{id}/release/download": gPaired, "POST /api/v1/authorize": gPaired, "GET /api/v1/github": gPaired,
	"POST /api/v1/github/connect": gPaired, "POST /api/v1/github/cancel": gPaired, "POST /api/v1/github/disconnect": gPaired,
	"POST /api/v1/shutdown": gPaired, "GET /api/v1/features/thought-engine/access": gPaired, "GET /api/v1/features/thought-engine/release": gPaired,
	"GET /api/v1/features/thought-engine/release/download": gPaired,

	// P1
	"GET /api/v1/admin": gPaired, "POST /api/v1/admin/enrollment": gPaired + " (creates a request only; enrollment completes in the approval page)",
	"GET /api/v1/approvals/{id}":                         gPaired + " (only the requester's own approvals)",
	"POST /api/v1/installations/requests":                gPaired + " (creates a request only)",
	"POST /api/v1/installations":                         gElevated,
	"GET /api/v1/installations":                          gPaired,
	"GET /api/v1/installations/{id}":                     gPaired,
	"POST /api/v1/installations/{id}/abandon":            gPaired + " (requester's own unconfirmed registration only)",
	"POST /api/v1/installations/{id}/uninstall/requests": gPaired + " (creates a request only)",
	"POST /api/v1/installations/{id}/uninstall":          gElevated,
	"POST /api/v1/grants/requests":                       gPaired + " (creates a request only)",
	"POST /api/v1/grants":                                gElevated,
	"GET /api/v1/grants":                                 gPaired,
	"POST /api/v1/grants/{id}/revoke":                    gPaired + " (narrows access; persisted before success)",
	"POST /api/v1/tickets":                               gPaired + " (issues a ticket ONLY under an active grant for this exact requester/recipient/subject/capability)",
	"GET /api/v1/decisions":                              gPaired,
	"POST /api/v1/installation/confirm":                  gInstallation,
	"GET /api/v1/installation/self":                      gInstallation,
	"POST /api/v1/authz/check":                           gInstallation + " (the live decision; consumes the ticket)",
	"POST /api/v1/installation/tickets":                  gInstallation + " (installation as requester; same grant rule)",
	"POST /api/approval/open":                            gApprovalPage,
	"GET /api/approval/view":                             gApprovalPage,
	"POST /api/approval/decide":                          gApprovalPage,
}

// ElevatedRoutes must refuse without a usable Admin approval.
var ElevatedRoutes = []string{"POST /api/v1/installations", "POST /api/v1/installations/{id}/uninstall", "POST /api/v1/grants"}

type routeEntry struct {
	Pattern string `json:"route"`
	Guard   string `json:"guard"`
}

// routeMux records every registered pattern.
type routeMux struct {
	*http.ServeMux
	a *App
}

func (m *routeMux) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	g, ok := RouteGuards[pattern]
	if !ok {
		g = "UNCLASSIFIED — this is a bug (see routes.go)"
	}
	m.a.routes = append(m.a.routes, routeEntry{pattern, g})
	m.ServeMux.HandleFunc(pattern, h)
}

// Routes returns the inventory, sorted.
func (a *App) Routes() []routeEntry {
	out := append([]routeEntry(nil), a.routes...)
	sort.Slice(out, func(i, j int) bool { return out[i].Pattern < out[j].Pattern })
	return out
}
