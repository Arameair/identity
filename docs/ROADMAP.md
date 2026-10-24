# Roadmap — first establish the foundation, then integrate features

**Reviewed:** 2026-10-24. Milestones are proposed **owner acceptance gates**, not delivery commitments or claims that the capabilities work today.

| Gate | Demonstration | Required evidence | Current position |
| --- | --- | --- | --- |
| 0 — Documented direction | Clear governing principle, responsibilities, first-run UX, storage and honest current state | Linked decisions, review and source provenance | Documentation on Main |
| 1 — Durable local authority | Create, save, archive and restore local IDs; distinguish installation/admin/service/entity IDs | Fresh profile + restart + unchanged IDs + recovery failure tests | Basic local ID capability exists experimentally; complete secure bootstrap not implemented |
| 2 — Clean storage boundaries | Data roots centrally organized and separately owned | Actual path/permission checks, experiment isolation, backup/restore and disk-failure behavior | **Design only** |
| 3 — Safe optional account association | Verify existing GitHub CLI access without storing its token; read-only repo reference | Real CLI/provider state; no repo mutation; revoke/no-login paths | Prototype exists; official in-app account broker absent |
| 4 — Secure service registration | Feature starts independently, authenticates enrollment, registers stable service and current instance ID | Two-service test, denial/replay, restart and stale instance results | **Design only**; current port discovery is unauthenticated |
| 5 — Scoped feature authorization | Allow one narrow operation to one service, deny others and revoke safely | Real receiving-service enforcement, expired/revoked and Identity-offline checks | **Design only** |
| 6 — Cross-feature inspection | Thought Processor lists registered apps, shows true status/flow and open original UI where supported | Real service IDs, version/process facts, correlated logs; no hard-coded app count | Planned in separate repo |
| 7 — Installation and recovery acceptance | New-user installation, safe backups, selected startup lifecycle and approved releases | Real clean machine/OS, test evidence, rollback and explicit owner decision | Planned |

**Step order is an architectural recommendation, not authorization to rewrite or deploy other repositories.** Small local GitHub verification can remain a useful parallel experiment, but it must not be mislabeled as complete Identity-based OAuth. Do not add Thought Engine or Memory requirements to bootstrap gates.

## Next experiment to evaluate

Start from current owner-approved `alpha`. Build an independently runnable, **local** Identity foundation that proves durable installation/admin/service/entity records and reports trustworthy states. Provide storage allocation references for one test feature without moving another feature's raw logs. Test restart, corrupt record, two concurrent instances and non-admin permission. Keep optional GitHub/discovery tests separate until correct scopes are implemented.

Once that is accepted, test authenticated service enrollment followed by Thought Processor discovery as the next independent integration. Full secret storage and cross-service grants must be security-reviewed before any product-grade claims.

## Testing protocol

Record starting Alpha SHA, experimental SHA, environment/OS, exact endpoints, local path or profile, process state, real or fixture dependencies, measured timings, failed/denied cases, identity persistence, source-owned storage, logs and rollback. Keep **PASSED WITH FIXTURES**, **LOCAL PROCESS VERIFIED**, **END-TO-END VERIFIED**, and **OWNER ACCEPTED** distinct.

**The owner alone promotes.** Do not auto-merge Alpha or Stable, or treat a documentation change to Main as an accepted executable.

Related: [First run](FIRST_RUN.md), [Storage](STORAGE.md), [Decisions](DECISIONS.md), [Status](STATUS.md), [Experiment report](EXPERIMENT_TEMPLATE.md).