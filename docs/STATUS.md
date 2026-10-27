# Identity — implementation status and evidence ledger

**Reviewed October 27, 2026.** These are **distinct**: implemented code, tests reported by a developer, observed owner operation, design agreement, and release approval. This is not a security certification.

## Snapshot

**P0 Alpha is executable. P1 is executable in an experiment. Neither is a generally released or production-hardened identity service.**

- **P0:** Owner-promoted baseline `alpha` at [`c8c0a54ab333077ccbfbd86c5952fb8a00b55dc1`](https://github.com/Arameair/identity/commit/c8c0a54ab333077ccbfbd86c5952fb8a00b55dc1), including local Identity, GitHub account flow, authenticated Processor pairing and `identity/2` signed requests/responses. [P0 context and promotion record](https://github.com/Arameair/Thought-Processor/issues/19).
- **P1:** [`experiment/p1-cleanup-claude-20261026`](https://github.com/Arameair/identity/tree/experiment/p1-cleanup-claude-20261026) builds on Alpha and contains administrator enrollment, action-bound approvals, persistent installation IDs, scoped grants, per-request tickets and receiver enforcement for one **pinned deterministic fixture**. **Not promoted.**
- **P1 test evidence:** [Claude's October 26 cleanup report](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/P1_CLEANUP_REPORT_20261026.md) records a clean-clone **81/81 integration** and **15/15 adversarial** checks, plus P0 regression and UI checks. These are *reported* tests, not a claim that the owner has repeated them on Windows/WSL or that a security audit occurred.
- **User acceptance:** Owner **has not yet run or accepted the P1 cleanup prototype**. Real Fast AI Worker and Thought Engine have **not** been proven as Identity-authorized receiving features.

## Branch map

| Branch | Purpose and currently supported claim | Status |
| --- | --- | --- |
| [`main`](../README.md) | Public-facing project documentation, decisions and case study. **No Go executable source in this branch.** | Documentation lane; not a runtime release. |
| [`alpha`](https://github.com/Arameair/identity/tree/alpha) | Owner-promoted P0 runnable baseline, SHA above. | Working development baseline; **not stable or public-production certified**. |
| [`experiment/p1-feature-foundation-claude-20261026`](https://github.com/Arameair/identity/tree/experiment/p1-feature-foundation-claude-20261026) | First P1 integrated identity/grant prototype. | Historical experiment. |
| [`experiment/p1-cleanup-claude-20261026`](https://github.com/Arameair/identity/tree/experiment/p1-cleanup-claude-20261026) | Later hardened P1 candidate after adversarial review. | **Next owner trial**; no promotion approved. |
| [`alpha-candidate-20261026`](https://github.com/Arameair/identity/tree/alpha-candidate-20261026) | Earlier standalone/paired candidate and engine-install demo. | Earlier experimental lane; do not confuse with current Alpha/P1. |
| [`alpha-candidate-20261025`](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) | Earlier Identity-first local UI and account-flow prototype. | Historical UX candidate. |
| `stable` | Owner-accepted release snapshot. | No accepted release identified in the reviewed evidence. |

**Source of truth:** Verify exact `git rev-parse HEAD`, matching Processor branch, test logs and human verdict before reusing a candidate. An inherited README may describe an older branch.

## Functional coverage

| Capability | P0 Alpha | P1 cleanup experiment | Remaining boundary |
| --- | --- | --- | --- |
| Durable Identity profile, local entities and independent service | **Code present; P0 trial evidence** | Retained | Production backup/recovery and OS hardening not complete. |
| GitHub device sign-in | P0 implementation; owner trial observations on earlier candidate | Required at initial Admin enrollment | Real fresh P1 onboarding/negative-path owner test still pending. |
| Signed Processor–Identity pairing | **Implemented in `identity/2`** | Retained and hardened | Does **not** grant cross-feature access. |
| Original Admin Identity and exact-action approval | Not a P0 general feature grant | **Implemented for P1** | Passphrase security and recovery limitations. |
| Distinct feature installation IDs, default zero grants | Not established in P0 | **Implemented for trusted fixture** | Third-party provenance and OS sandboxing absent. |
| Identity-issued grants, per-request tickets, live checks, revocation | P0 legacy authorize denies | **Implemented/tested with deterministic fixture** | Engine and Fast AI Worker integration **not demonstrated**. |
| Autonomous recommendations and identity creation under confirmed design | Direction recorded | Generic identity creation exists; recommendation cognition **not implemented** | Per-identity capability rules and orchestration require integration testing. |
| Consumer source/data protections, Memory, real AI worker | Not established | Not covered by fixture proof | Must be integrated and tested separately. |
| End-user installer and signed public release | Not claimed | Not claimed | No release approval or broad OS acceptance. |

## Known risks and current decisions

1. **No OS isolation:** same-user processes may reach local files/ports or credentials. P1's pins are not a hostile-code sandbox.
2. **Admin protection:** current local passphrase is not Windows Hello or a production recovery mechanism.
3. **Publisher authenticity and source permissions:** signed release provenance, real information-source grants and independent third-party installation remain incomplete.
4. **Legacy Engine/Worker routes:** older Engine local access and Worker history/read paths require canonical Identity checks before real use. See [Identity #15](https://github.com/Arameair/identity/issues/15), [Engine #3](https://github.com/Arameair/Thought-engine/issues/3), [Worker #3](https://github.com/Arameair/fast-ai-worker/issues/3).
5. **October 27 approved design:** Identity alone authorizes protected work; Processor is the UI/control point; Engine references Identity's canonical IDs and owns thought data; identity-scoped grants enable routine autonomous operations, but feature suggestions and availability do not confer permissions. See [shared decisions](https://github.com/Arameair/Thought-Processor/issues/20).

## Next verification gate

Follow the [matched P1 trial](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/START_HERE.md) with fresh disposable profiles. Record platform, exact commits, GitHub real vs fixture mode, Admin enrollment, persistent installation ID, denial before permission, allow after permission, revocation, Identity outage, restart, redacted logs, problems and **owner KEEP / REVISE / REJECT**.

Do not promote P1 or call it production-ready merely because the developer's tests passed.

[Project overview](../README.md) · [Architecture](ARCHITECTURE.md) · [Engineering case study](PROJECT_JOURNEY.md) · [Security](SECURITY.md) · [Owner decisions](DECISIONS.md).
