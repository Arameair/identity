# Roadmap — first establish the foundation, then integrate features

**Reviewed:** 2026-10-25. Milestones are proposed **owner acceptance gates**, not delivery commitments or claims that the capabilities work today. The most immediate evidence gate is a **clean Windows owner run** of the October 25 Identity candidate with real GitHub sign-in (or recorded denial), independently running Processor, diagnostics and restart results. See [trial guide](TRY_THE_PROTOTYPE.md) and [status](STATUS.md).

| Gate | Demonstration | Required evidence | Current position |
| --- | --- | --- | --- |
| 0 — Documented direction | Clear governing principle, responsibilities, first-run UX, storage and honest current state | Linked decisions, review and source provenance | Documentation on Main |
| 1 — Durable local authority | Create, save, archive and restore local IDs; distinguish installation/admin/service/entity IDs | Fresh profile + restart + unchanged IDs + recovery failure tests | Local ID and guided UI code exist in separate experiments; secure administrator bootstrap remains unimplemented |
| 2 — Clean storage boundaries | Data roots centrally organized and separately owned | Actual path/permission checks, experiment isolation, backup/restore and disk-failure behavior | **Design only** |
| 3 — Safe optional account association | Verify existing GitHub CLI access without storing its token; read-only repo reference | Real CLI/provider state; no repo mutation; revoke/no-login paths | Earlier `gh` check and newer GitHub App device-flow code exist in experiments; complete real Windows acceptance is pending |
| 4 — Secure service registration | Feature starts independently, authenticates enrollment, registers stable service and current instance ID | Two-service test, denial/replay, restart and stale instance results | **Design only**; current port discovery is unauthenticated |
| 5 — Scoped feature authorization | Allow one narrow operation to one service, deny others and revoke safely | Real receiving-service enforcement, expired/revoked and Identity-offline checks | **Design only** |
| 6 — Cross-feature inspection | Thought Processor lists registered apps, shows true status/flow and open original UI where supported | Real service IDs, version/process facts, correlated logs; no hard-coded app count | Planned in separate repo |
| 7 — Installation and recovery acceptance | New-user installation, safe backups, selected startup lifecycle and approved releases | Real clean machine/OS, test evidence, rollback and explicit owner decision | Planned |

**Step order is an architectural recommendation, not authorization to rewrite or deploy other repositories.** Small local GitHub verification can remain a useful parallel experiment, but it must not be mislabeled as complete Identity-based OAuth. Do not add Thought Engine or Memory requirements to bootstrap gates.

## Next experiment to evaluate

First **finish testing** the existing [October 25 Alpha candidate](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) with a real clean-machine report, instead of restarting the implementation from scratch. For subsequent functional work, start from current owner-approved `alpha` and build an independently runnable, **local** Identity foundation that proves durable installation/admin/service/entity records and reports trustworthy states. Provide storage allocation references for one test feature without moving another feature's raw logs. Test restart, corrupt record, two concurrent instances and non-admin permission. Keep optional GitHub/discovery tests separate until correct scopes are implemented.

Once that is accepted, test authenticated service enrollment followed by Thought Processor discovery as the next independent integration. Full secret storage and cross-service grants must be security-reviewed before any product-grade claims.

## Testing protocol

Record starting Alpha SHA, experimental SHA, environment/OS, exact endpoints, local path or profile, process state, real or fixture dependencies, measured timings, failed/denied cases, identity persistence, source-owned storage, logs and rollback. Keep **PASSED WITH FIXTURES**, **LOCAL PROCESS VERIFIED**, **END-TO-END VERIFIED**, and **OWNER ACCEPTED** distinct.

**The owner alone promotes.** Do not auto-merge Alpha or Stable, or treat a documentation change to Main as an accepted executable.

Related: [First run](FIRST_RUN.md), [Storage](STORAGE.md), [Decisions](DECISIONS.md), [Status](STATUS.md), [Experiment report](EXPERIMENT_TEMPLATE.md).