# Roadmap — first establish the foundation, then integrate features

**Reviewed:** 2026-10-27. These are **owner acceptance gates**, not delivery commitments. Thought Processor is the human-facing interface; Identity is the permission authority. **P0 is promoted to Alpha, while the P1 cleanup build is experimental.** The next evidence gate is the owner's P1 trial of real Identity + Processor processes with a pinned deterministic test feature. See [P1 trial guide](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/START_HERE.md) and [status](STATUS.md).

| Gate | Demonstration | Required evidence | Current position |
| --- | --- | --- | --- |
| 0 — Documented direction | Clear governing principle, responsibilities, first-run UX, storage and honest current state | Linked decisions, review and source provenance | Documentation on Main |
| 1 — Durable local authority | Create, save, archive and restore local IDs; distinguish installation/admin/service/entity IDs | Fresh profile + restart + unchanged IDs + recovery failure tests | **P0 baseline exists; P1 experimental Admin enrollment and per-feature IDs**. Production recovery remains unimplemented. |
| 2 — Clean storage boundaries | Data roots centrally organized and separately owned | Actual path/permission checks, experiment isolation, backup/restore and disk-failure behavior | **Design only** |
| 3 — Safe optional account association | Verify existing GitHub CLI access without storing its token; read-only repo reference | Real CLI/provider state; no repo mutation; revoke/no-login paths | Earlier `gh` check and newer GitHub App device-flow code exist in experiments; complete real Windows acceptance is pending |
| 4 — Secure service registration | Feature starts independently, authenticates enrollment, registers stable service and current instance ID | Two-service test, denial/replay, restart and stale instance results | **P1 trusted-fixture prototype code and reported tests**; real third-party provenance/registration still incomplete. |
| 5 — Scoped feature authorization | Allow one narrow operation to one service, deny others and revoke safely | Real receiving-service enforcement, expired/revoked and Identity-offline checks | **P1 fixture implementation tested in developer report**; Engine/Worker integration and owner validation outstanding. |
| 6 — Processor-first human administration | **Thought Processor** hosts first-run, identity/account views, feature access and true status; Identity responds as the authority service | Real authenticated Processor-to-Identity call, verified IDs and authorization states, correlated errors; no direct Identity UI required for normal use | **Not implemented end-to-end** |
| 7 — Installation and recovery acceptance | New-user installation, safe backups, selected startup lifecycle and approved releases | Real clean machine/OS, test evidence, rollback and explicit owner decision | Planned |

**Step order is an architectural recommendation, not authorization to rewrite or deploy other repositories.** Small local GitHub verification can remain a useful parallel experiment, but it must not be mislabeled as complete Identity-based OAuth. Do not add Thought Engine or Memory requirements to bootstrap gates.

## Next experiment to evaluate

First **owner-test** the current [P1 cleanup experiment](https://github.com/Arameair/identity/tree/experiment/p1-cleanup-claude-20261026) with its matching Processor branch, real Admin onboarding and fail-closed grant checks; retain the October 25 candidate as historical UX evidence. Do not promote that UX as an approved product interface. The next system-level acceptance gate is a clean-machine **Processor-first** path with an authenticated Identity status/management contract. For subsequent functional work, start from current owner-approved `alpha` and build an independently runnable, **local** Identity foundation that proves durable installation/admin/service/entity records and reports trustworthy states. Provide storage allocation references for one test feature without moving another feature's raw logs. Test restart, corrupt record, two concurrent instances and non-admin permission. Keep optional GitHub/discovery tests separate until correct scopes are implemented.

Once that is accepted, test authenticated service enrollment followed by Thought Processor discovery as the next independent integration. Full secret storage and cross-service grants must be security-reviewed before any product-grade claims.

## Testing protocol

Record starting Alpha SHA, experimental SHA, environment/OS, exact endpoints, local path or profile, process state, real or fixture dependencies, measured timings, failed/denied cases, identity persistence, source-owned storage, logs and rollback. Keep **PASSED WITH FIXTURES**, **LOCAL PROCESS VERIFIED**, **END-TO-END VERIFIED**, and **OWNER ACCEPTED** distinct.

**The owner alone promotes.** Do not auto-merge Alpha or Stable, or treat a documentation change to Main as an accepted executable.

Related: [First run](FIRST_RUN.md), [Storage](STORAGE.md), [Decisions](DECISIONS.md), [Status](STATUS.md), [Experiment report](EXPERIMENT_TEMPLATE.md).