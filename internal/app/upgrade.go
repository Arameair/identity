package app

// EXPERIMENTAL identity.installation-upgrade/1 routes (see
// internal/authority/upgrade.go). Not deployed; needs owner/security review.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Arameair/identity/internal/authority"
)

func (a *App) upgradeRoutes(m *routeMux) {
	m.HandleFunc("POST /api/v1/installations/{id}/upgrade/requests", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		cur, err := a.Installs.Get(r.PathValue("id"))
		if err != nil {
			a.signedFail(w, r, 404, "installations.not_found", err.Error())
			return
		}
		var u authority.UpgradeRequest
		if decode(r, &u) != nil {
			a.signedFail(w, r, 400, "upgrade.bad_request", "expected {from_artifact_sha256, to:{…}, data_migration}")
			return
		}
		u.RequestedBy = principalOf(r)
		if err := u.Validate(cur, a.pins); err != nil {
			code := 400
			switch {
			case errors.Is(err, authority.ErrUpgradeOwner), errors.Is(err, authority.ErrNotPinned), errors.Is(err, authority.ErrUntrusted),
				errors.Is(err, authority.ErrUpgradeIrreversible):
				code = 403
			case errors.Is(err, authority.ErrUpgradeStale), errors.Is(err, authority.ErrUpgradeState):
				code = 409
			}
			a.decide(r, "upgrade.request_refused", "DENIED", err.Error(), map[string]string{"installation_id": cur.ID})
			a.signed(w, r, code, map[string]any{"error": "UPGRADE_REQUEST_REFUSED", "message": err.Error(), "request_id": reqID(r)})
			return
		}
		if _, err := a.Admin.Get(); err != nil {
			a.signedFail(w, r, 409, "upgrade.no_admin", err.Error())
			return
		}
		added, removed, kept := authority.CapabilityDiff(cur.Capabilities, u.To.Capabilities)
		ent := u.Entitlement
		if ent == "" {
			ent = "NOT_REQUIRED"
		}
		summary := []string{
			"Upgrade installation " + cur.ID + " (" + cur.FeatureID + ") from " + cur.Version + " to " + u.To.Version + ". Same installation ID and credential.",
			"Trust: " + a.pins.Basis(u.To.ArtifactSHA256) + " — exact new artifact pinned. From " + cur.ArtifactSHA256[:12] + "… to " + u.To.ArtifactSHA256[:12] + "…",
			"Grants kept for: " + orDash(strings.Join(kept, ", ")) + ". Grants REVOKED for dropped capabilities: " + orDash(strings.Join(removed, ", ")) + ".",
			"New capabilities (NO grant; each needs its own approval): " + orDash(strings.Join(added, ", ")) + ".",
			"Data migration: " + u.DataMigration + ". Security fix: " + map[bool]string{true: "yes", false: "no"}[u.SecurityFix] + ". Entitlement (informational, grants nothing): " + ent + ".",
			"The requester may roll back to " + cur.Version + " within 30 minutes if the new version fails its checks; revoked grants are not restored.",
		}
		a.newApproval(w, r, authority.KindUpgrade, u, summary)
	}))

	m.HandleFunc("POST /api/v1/installations/{id}/upgrade", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		ap, _, fresh, ok := a.beginApproval(w, r, authority.KindUpgrade)
		if !ok {
			return
		}
		var u authority.UpgradeRequest
		json.Unmarshal(ap.Action, &u)
		if u.InstallationID != r.PathValue("id") {
			a.Approvals.Finish(ap.ID, "", "FAILED: approval is for "+u.InstallationID)
			a.signedFail(w, r, 403, "upgrade.wrong_target", "that approval is for a different installation")
			return
		}
		if !fresh {
			in, _ := a.Installs.Get(u.InstallationID)
			a.signed(w, r, 200, map[string]any{"installation": in, "approval_reused": true})
			return
		}
		cur, err := a.Installs.Get(u.InstallationID)
		if err == nil {
			err = u.Validate(cur, a.pins) // pins, state and from-artifact re-checked at commit
		}
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.decide(r, "upgrade.refused", "DENIED", err.Error(), map[string]string{"installation_id": u.InstallationID, "approval_id": ap.ID})
			a.signed(w, r, 409, map[string]any{"error": "UPGRADE_REFUSED", "message": err.Error(), "request_id": reqID(r)})
			return
		}
		_, removed, _ := authority.CapabilityDiff(cur.Capabilities, u.To.Capabilities)
		revoked, err := a.Grants.RevokeCapabilities(cur.ID, removed, "admin-approval:"+ap.ID, "capability dropped by upgrade to "+u.To.Version)
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 503, "upgrade.revoke_failed", err.Error())
			return
		}
		in, err := a.Installs.ApplyUpgrade(u, ap.ID, a.pins.Basis(u.To.ArtifactSHA256), revoked)
		if err != nil {
			a.Approvals.Finish(ap.ID, "", "FAILED: "+err.Error())
			a.signedFail(w, r, 409, "upgrade.save_failed", err.Error())
			return
		}
		a.Approvals.Finish(ap.ID, in.ID, "UPGRADED "+u.FromArtifactSHA256[:12]+"->"+u.To.ArtifactSHA256[:12])
		a.decide(r, "install.upgraded", "ALLOW", "", map[string]string{"installation_id": in.ID, "approval_id": ap.ID,
			"from": u.FromArtifactSHA256[:12], "to": u.To.ArtifactSHA256[:12], "grants_revoked": strings.Join(revoked, ",")})
		a.signed(w, r, 200, map[string]any{"installation": in, "grants_revoked": revoked})
	}))

	m.HandleFunc("POST /api/v1/installations/{id}/upgrade/rollback", a.wrap(paired, func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			ApprovalID string `json:"approval_id"`
			Reason     string `json:"reason"`
		}
		decode(r, &b)
		id := r.PathValue("id")
		in, err := a.Installs.RollbackUpgrade(id, b.ApprovalID, principalOf(r), b.Reason, a.pins)
		if err != nil {
			a.decide(r, "upgrade.rollback_refused", "DENIED", err.Error(), map[string]string{"installation_id": clip(id, 32)})
			a.signed(w, r, 403, map[string]any{"error": "ROLLBACK_REFUSED", "message": err.Error(), "request_id": reqID(r)})
			return
		}
		a.decide(r, "install.upgrade_rolled_back", "ALLOW", "", map[string]string{"installation_id": in.ID, "approval_id": b.ApprovalID,
			"restored": in.ArtifactSHA256[:12]})
		a.signed(w, r, 200, map[string]any{"installation": in})
	}))
}
