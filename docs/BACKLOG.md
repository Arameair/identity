# Identity — active work and archived backlog

**Historical triage snapshot — October 26, 2026:** Issues #9, #14, #15 and #19 were the active list at that point; 10 older issues and 5 pull requests were closed without promotion. **Newer tracking issues have since been created; this is not an assertion of today's open-issue count.** Refer to the [live issues list](https://github.com/Arameair/identity/issues) and [status ledger](STATUS.md). Closures are backlog organization, not owner acceptance.

## Active issues (one place to start)

| Issue | Purpose | Current boundary |
| --- | --- | --- |
| [#9 — Account and GitHub integration](https://github.com/Arameair/identity/issues/9) | Finish verified provider connection and separately approved repository/account operations. | GitHub login is not a general feature entitlement; normal user interactions eventually belong in Processor. |
| [#14 — Architecture review](https://github.com/Arameair/identity/issues/14) | Decide lifecycle, trust bootstrap, feature relationships, storage, source approvals and unresolved design conflicts. | Cross-repo architecture discussion; proposals are not granted permissions. |
| [#15 — Real authorization enforcement](https://github.com/Arameair/identity/issues/15) | Design and prove actual authenticated source → Thought Engine grant checks, revocation and outage handling. | Install permission, runtime data access and Engine-local toggles are different things. |
| [#19 — Earlier candidate/owner trial](https://github.com/Arameair/identity/issues/19) | Preserve the October 25 owner trial and user friction findings. | Historical baseline; P1 uses newer matched branches. |
| [#20 — Alpha promotion gate](https://github.com/Arameair/identity/issues/20) | Track exact candidate, reproducible proof, security boundaries and owner verdict before promotion. | P0 is owner-promoted; P1 cleanup is a separate unpromoted experiment. |

## Closed historical issues (history retained)

All ten issues in this section are **confirmed closed as `not_planned`**. They describe historical prototypes, duplicated work or a launch model superseded by the **Processor-first human interface** decision. **None was marked `completed`; unresolved requirements remain on the four active tracking issues.**

- **#1, #5, #6 and #16:** Identity-first wizard, automatic Thought Controller launch, or Identity-led "Start Experience" flows. Preserve for research; use [current first-run direction](FIRST_RUN.md) and active #14 for any new lifecycle decision.
- **#2 and #10:** earlier local-card / GitHub-CLI experiments and acceptance checklists; preserve test evidence. Follow the current #19 candidate and do not infer that the original owner tests passed.
- **#3:** umbrella "future gates" referencing the legacy Thought Controller and per-entity repo assumptions. Its remaining legitimate questions are covered by #9, #14 and #15.
- **#8:** historical creation of the separate Thought repository; current engine selection and cognitive feature design belong in the appropriate project and cross-repo architecture review.
- **#17 and #18:** October 25 GitHub polling failure and sign-in/shipping trial notes. The candidate reports fixes, but unresolved real-user regression and release acceptance remain under #19.

## Closed historical pull requests

**All five are confirmed closed without merge**, with source branches and review histories preserved: [#4](https://github.com/Arameair/identity/pull/4), [#7](https://github.com/Arameair/identity/pull/7), [#11](https://github.com/Arameair/identity/pull/11), [#12](https://github.com/Arameair/identity/pull/12), and [#13](https://github.com/Arameair/identity/pull/13). They were earlier experiments, not approved changes to `alpha`, `stable` or `main`.

## Cross-repo decisions that must stay consistent

**Confirmed owner direction:** Thought Processor is the primary **system-wide user/admin interface**; Identity is the identity, security and permission authority; a receiving feature must independently **enforce** an Identity decision. Each feature owns its processing/data. Identity and Processor must be independently operable. **No source is allowed by default**: selecting a feature or connecting an account does not turn on every log, game or conversation feed.

**Still open for product readiness:** OS-backed Admin authentication/recovery, hostile-code isolation, general publisher provenance and install entitlement, real source grants and Engine/Worker receiver enforcement, in-flight revocation, lifecycle supervision and a canonical Engine migration. **Now prototyped in P0/P1 rather than absent:** authenticated Processor pairing, one Admin record, limited per-feature registration and scoped grants for the pinned fixture. The October 26 [Engine installation fixture demo](https://github.com/Arameair/identity/blob/experiment/engine-access-pairing-20261026/docs/ENGINE_INSTALL_DEMO_20261026.md) tests an interim **repo-visibility-v0** rule but does **not** establish a final policy or a real GitHub/Windows user test.

**Design provenance:** The October 25 [owner alignment note](https://github.com/Arameair/identity/blob/experiment/dynamic-feature-observation-design-20261025/docs/OWNER_DESIGN_ALIGNMENT_2026-10-25.md) discusses per-source default-deny, source-owned Sparks, a shared Engine, separate Memory, and Attention/Decay as processing mechanics. The old Spark → Trace → Thought code is historical implementation, not proof the new design is running. Do not discard these distinctions when reviewing older issues.

## Working rules

- New **functional experiments start from owner-controlled `alpha`**; reference or explicitly port previous candidate code and record provenance. Experiments, candidate branches, `main` docs and GitHub issues do not authorize automatic promotion.
- Keep historical branches and closed issues for audit rather than deleting evidence. Prefer one active tracking issue per outcome; link follow-up work there.
- For any implementation claim, record branch/full SHA, operating system, real versus fixture services, tests, error cases, and the owner's decision.
- Keep [status](STATUS.md), [decisions](DECISIONS.md), [architecture](ARCHITECTURE.md), and [security](SECURITY.md) as the primary source documents, instead of copying entire specifications into new issues.
