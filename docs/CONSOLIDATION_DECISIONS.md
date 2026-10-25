# Consolidation decisions — 2026-10-25
- Base: Identity alpha `8d08fec6629dc9cc35ea2266556572b86e5499d7` (documentation-only). Preserve it.
- `identity-real-manager-20261023`: reuse the core pattern (durable random System ID; cards with archive/restore). Omit the `gh` CLI dependency; no destructive migration of old profiles.
- `identity-standalone-access-trial-20261024/standalone/github.go`: reuse official GitHub App device flow and verify `GET /user`, memory-only tokens, local draft semantics. Its original source blob is reused in root `github.go`. Do not convert drafts into publisher approval claims.
- `identity-setup-wizard-20261024`: defer its Project Controller-coupled wizard; not needed for independent startup.
- `identity-controller-port-discovery-20261023`: keep explicit loopback-only read-only discovery, but target **Thought-Processor**, not legacy Thought-Controller.
- Processor source reference `Thought-Processor:experiment/processor-controller-launch-fix-20261024`, SHA `0ba8bd52facedda1405897e9136d68ab184a2e32`: do not alter; public `GET /api/health` only. A valid self-claim is **DISCOVERED_UNVERIFIED** and never a grant.
- Added: one Go root executable, bounded real local event log, Copy Diagnostics redaction, current status view, read-only Processor probe, private profile A on port 7797; no repo deletion, Engine calls or fake grants.
- NEXT SECURITY GATES: authenticated service identity contract, signed scoped grants, enforcing Engine intake, publisher approval service and signed release provenance. None are supplied by this candidate.
