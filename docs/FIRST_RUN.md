# First run — a trustworthy local foundation

**Status: target secure first-run authority and recovery model, not yet implemented.** Updated 2026-10-25. A separate [October 25 experimental Go candidate](TRY_THE_PROTOTYPE.md) has a guided local welcome/GitHub/features/Processor UI, but it is **not** proof of secure administrator bootstrap, registered service identity or feature grants. See [current evidence](STATUS.md) and [architecture](ARCHITECTURE.md).

## Principle

> **Identity owns the organization and authority of the system. Each feature owns its actual work.**

Identity should be usable **alone** on a clean machine. Installing Thought Processor, Thought Engine, Memory, Chat-Jimmy, TASS, GitHub or Project Controller must not be a prerequisite for healthy first-run setup.

## Proposed first-run journey

| Stage | What the administrator sees | What must be true before showing success |
| --- | --- | --- |
| 1. Welcome | **Create a new installation** or **restore an existing installation** | A local writable, selected Identity data root can be established without overwriting existing records |
| 2. Administration | Create/confirm the **administrator** and local security boundary | Stable administrator record associated with this installation; appropriate operating-system authentication and key protection design tested |
| 3. Local foundation | **Installation ID**, **Identity service ID**, storage overview | Three different identifiers where appropriate; durable atomic storage initialized; fail closed on corrupt or inaccessible authority records |
| 4. Storage | Inspect **Identity registry** and future **feature data roots** | Paths are visible; feature roots isolated; no data inserted into git checkouts or untrusted shared folders |
| 5. Registration readiness | **Ready for feature registration** (if authenticated registration is truly implemented) | A service can verify its own identity and safely enroll/reconnect; discovery alone is insufficient |
| 6. Dashboard | **Identity ready locally — 0 optional features** | Local records survive restart and UI reports the real state; absent optional services are not errors |
| 7. Optional next steps | Add feature, connect account, review backups, open Thought Processor if available | Every optional action clearly explains approval, actual side effects and unavailable functionality |

**Do not show this as working functionality before it is verified.** For present experiments, show honest incomplete states rather than a simulated "all ready" dashboard.

## Identity records are distinct

- **Installation ID:** the deployment of the ecosystem on this machine/profile. A restore should avoid accidentally creating an unrelated duplicate; cross-device transfer and identity migration require a designed policy.
- **Administrator ID:** the person/actor exercising administrative authority. Identity records do not by themselves authenticate a user.
- **Identity service ID:** the durable identity of this running software service. Every launch creates a separate **runtime/instance ID**; multiple concurrently running copies must be distinguishable.
- **Feature/service ID:** durable identity for a separately installed component; preserved across restarts and version changes, with permission checks for ownership/reassignment.
- **Entity ID:** a companion, business, person, project or other entity using authorized features. An entity is not automatically a service process.

## What happens on subsequent launches?

1. Find the selected existing, durable storage root and verify its integrity.
2. Restore the same installation and administrator records without silently re-enrolling another identity.
3. Load the feature registry and recorded permissions; **do not** treat a saved permission label as a verified live grant.
4. Reconcile feature instances with real authenticated status; show stopped/offline without removing the durable registration.
5. Never start independent services, make model calls, or write to GitHub merely because Identity was opened.

## Background service — OPEN decision

Owner-selected direction: other features should self-register with Identity so the catalog can update without rewriting Processor.

**Not yet decided:** whether Identity should have an opt-in lightweight OS background service, a manually launched service, or a hybrid with local activation. A prototype should not silently install a startup task. If background startup is chosen later, explain it, permit disabling it and preserve the ability to run Identity manually for development.

Discovery and registration may need to queue/retry when Identity is not running; design idempotence rather than allowing features to self-authorize while offline.

## Failure and restore UX

- **New empty install:** legitimate healthy state, not missing Thought Engine.
- **Permission or disk failure:** show the exact failure, make no partial "created" claim.
- **Existing non-empty directory:** offer inspect/restore instead of silently overwriting it.
- **Lost/corrupt identity state:** recover from an authorized backup, preserve identifiers and revoke/reissue compromised credentials as necessary. No unsupported automatic overwrite.
- **Identity offline:** independent features continue only work they can perform safely under valid existing grants, with no new privileged enrollment.
- **Missing account login:** show optional **Not connected**; local identity remains usable.
- **Backup, restore, deletion:** provide explicit, separately confirmed procedures; deletion must not be sold as routine cleanup when it could destroy the authorization root.

## Minimum owner acceptance test

Test on an isolated fresh OS user/profile: first create, restart, inspect exact IDs/paths, backup/restore, read-only health, second feature enrollment attempt, revoked/denied enrollment, Identity-offline behavior and recovery. Record actual builds, full SHA, OS, test fixtures vs real authentication, storage boundaries, failures and rollback.

Related: [Storage](STORAGE.md), [Security](SECURITY.md), [Roadmap](ROADMAP.md), [Reviewer questions](REVIEW.md).