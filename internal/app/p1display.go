package app

// What Identity's approval page shows (Grok M3). Everything here comes from
// Identity's own records — the pairing store, installations, identities and
// the Admin record — never from text the requesting program supplied in the
// request. Raw IDs and digests go under "advanced".

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Arameair/identity/internal/authority"
)

// principalInfo describes a principal ("processor:tp_…" / "installation:ins_…")
// from Identity's records. verified=false means Identity has no record of it.
func (a *App) principalInfo(p string) (label, kind string, pairedAt *time.Time, verified bool) {
	k, id, _ := strings.Cut(p, ":")
	switch k {
	case "processor":
		if c, ok := a.Pair.Lookup(id); ok {
			t := c.PairedAt
			return c.Label, "paired program", &t, true
		}
		return "unknown program (no longer paired)", "paired program", nil, false
	case "installation":
		if in, err := a.Installs.Get(id); err == nil {
			t := in.CreatedAt
			return in.Name + " (installed feature)", "installed feature", &t, true
		}
		return "unknown feature installation", "installed feature", nil, false
	}
	return "unknown", "unknown", nil, false
}

// principalLabel is a short human label plus the ID, for summaries/logs.
func (a *App) principalLabel(p string) string {
	l, _, _, ok := a.principalInfo(p)
	if !ok {
		return p + " (not known to Identity)"
	}
	return "\"" + l + "\" (" + p + ")"
}

func (a *App) approvalDisplay(ap authority.Approval) map[string]any {
	label, kind, pairedAt, verified := a.principalInfo(ap.RequestedBy)
	req := map[string]any{"label": label, "kind": kind, "verified": verified, "principal": ap.RequestedBy,
		"label_source": "Identity's pairing record (the name shown next to the pairing code when you paired it) — not text from this request"}
	if pairedAt != nil {
		req["paired_at"] = pairedAt.UTC()
	}
	adm, admErr := a.Admin.Get()
	switch {
	case ap.Kind == authority.KindEnrollAdmin:
		req["is_primary"] = true
		req["note"] = "This program will be recorded as your primary Thought Processor."
	case admErr != nil:
	case adm.PrimaryRequester == "":
		req["warning"] = "Identity does not know which paired program is your primary Thought Processor (your Admin record predates this check). Approve only if you started this request yourself."
	case adm.PrimaryRequester != ap.RequestedBy:
		pl, _, _, _ := a.principalInfo(adm.PrimaryRequester)
		req["is_primary"] = false
		req["warning"] = "This request does NOT come from your primary Thought Processor (\"" + pl + "\"). It comes from \"" + label + "\". Approve only if you started it yourself in that program."
	default:
		req["is_primary"] = true
	}
	if !verified {
		req["warning"] = "Identity has no pairing record for the program that made this request. Deny it."
	}

	d := map[string]any{"requester": req, "information_sources": "none — P1 never grants an information source"}
	adv := map[string]any{"approval_id": ap.ID, "action_sha256": ap.Digest, "requested_by": ap.RequestedBy}
	feature := func(id string) map[string]any {
		in, err := a.Installs.Get(id)
		if err != nil {
			return map[string]any{"name": "unknown installation", "installation_id": id}
		}
		return map[string]any{"name": in.Name, "feature_id": in.FeatureID, "version": in.Version, "state": in.State,
			"trust_basis": orDash(in.TrustBasis), "installation_id": in.ID}
	}
	capDesc := func(id, c string) string {
		if in, err := a.Installs.Get(id); err == nil {
			for _, x := range in.Capabilities {
				if x.ID == c {
					return x.Description
				}
			}
		}
		return ""
	}
	switch ap.Kind {
	case authority.KindEnrollAdmin:
		d["headline"] = "Establish this installation's Admin Identity"
	case authority.KindInstall:
		var r authority.InstallRequest
		json.Unmarshal(ap.Action, &r)
		caps := []map[string]string{}
		for _, c := range r.Capabilities {
			caps = append(caps, map[string]string{"id": c.ID, "description": c.Description})
		}
		d["headline"] = "Install the feature \"" + r.Name + "\" " + r.Version
		d["destination"] = map[string]any{"name": r.Name, "feature_id": r.FeatureID, "version": r.Version,
			"trust_basis": orDash(a.pins.Basis(r.ArtifactSHA256)), "publisher": orDash(r.PublisherClaim) + " (claimed by the package, NOT verified)"}
		d["capabilities_declared"] = caps
		d["effect"] = "Creates one installation identity with ZERO grants and ZERO information sources. Nothing can use it until you approve a separate grant."
		adv["artifact_sha256"], adv["manifest_sha256"], adv["source"] = r.ArtifactSHA256, r.ManifestSHA256, r.SourceKind+": "+r.SourceRef
	case authority.KindGrant:
		var g authority.GrantRequest
		json.Unmarshal(ap.Action, &g)
		gl, _, _, _ := a.principalInfo(g.Requester)
		subj := map[string]any{"identity_id": g.Subject}
		if e, ok := a.Ents.Get(g.Subject); ok {
			subj["name"], subj["kind"] = e.Name, e.Kind
		}
		dest := feature(g.Destination)
		d["headline"] = "Allow \"" + gl + "\" to ask \"" + dest["name"].(string) + "\" to " + g.Capability
		d["who_gets_access"] = map[string]any{"label": gl, "principal": g.Requester}
		d["destination"] = dest
		d["capability"] = map[string]any{"id": g.Capability, "description": capDesc(g.Destination, g.Capability)}
		d["subject"] = subj
		if g.ExpiresAt != nil {
			d["expiry"] = map[string]any{"expires_at": g.ExpiresAt.UTC(), "until_revoked": false}
		} else {
			d["expiry"] = map[string]any{"until_revoked": true, "requires_confirmation": true,
				"warning": "This grant would NEVER expire. Approving it needs the extra confirmation box."}
		}
	case authority.KindUninstall:
		var act struct {
			ID string `json:"installation_id"`
		}
		json.Unmarshal(ap.Action, &act)
		f := feature(act.ID)
		d["headline"] = "Uninstall \"" + f["name"].(string) + "\""
		d["destination"] = f
		d["effect"] = "Revokes all its grants and removes its credential. The ID is kept as a tombstone and never reused."
	case authority.KindRevokeGrant:
		var act struct {
			ID     string `json:"grant_id"`
			Reason string `json:"reason"`
		}
		json.Unmarshal(ap.Action, &act)
		d["headline"] = "Revoke a permission (Admin emergency revocation)"
		if g, err := a.Grants.Get(act.ID); err == nil {
			gl, _, _, _ := a.principalInfo(g.Requester)
			d["who_gets_access"] = map[string]any{"label": gl, "principal": g.Requester}
			d["destination"] = feature(g.Destination)
			d["capability"] = map[string]any{"id": g.Capability, "description": capDesc(g.Destination, g.Capability)}
			d["grant_status"] = g.Status(time.Now())
		}
		d["reason"] = orDash(act.Reason)
		d["effect"] = "Only removes access; nothing is granted."
		adv["grant_id"] = act.ID
	}
	d["advanced"] = adv
	return d
}
