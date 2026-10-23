# Identity + Thought Controller — first-time administrator journey

**Design proposal, 2026-10-23. Not implemented or approved for Alpha.**
[Owner issue](https://github.com/Arameair/identity/issues) · [New-user tests](NEW_ADMIN_TEST_MATRIX_2026-10-23.md) · [Technical contract](NEW_ADMIN_BOOTSTRAP_2026-10-23.md)

## The product in plain English

A new administrator should launch **Identity** as the single starting point. Identity restores or establishes the administrator and their installation, checks whether the approved **Thought Controller** is running, uses the saved permission to request startup when needed, checks GitHub access, and opens Thought Controller as the main developer workspace.

**Thought Controller is itself a registered system/service identity** with a stable instance ID. It comes from the original reference implementation, but every installation belongs to its own administrator and has independent data, experiments and update decisions. Being a service identity does **not** mean it automatically thinks about itself.

### The first-time screens (future UI; not current behavior)

| Screen | What the new administrator sees | What really happens |
| --- | --- | --- |
| Welcome | **Create my installation** / **Restore an existing installation** | Creates a unique installation ID and administrator identity locally, or loads an existing authorized identity; no GitHub prerequisite merely to start |
| Controller discovery | **Checking Thought Controller** | Finds a known instance by verified local discovery; confirms service identity, compatible protocol, version and actual readiness without sending an AI prompt |
| Controller unavailable | **Start an approved Thought Controller automatically next time?** | On first use, explain source/version and request one-time permission; if approved, save limited launch preference. Request installation/start via authorized launcher; show real progress and errors |
| Controller already running | **Reuse existing controller** | Don't install/start a duplicate. Verify it belongs to this installation and is compatible; otherwise ask, never take it over |
| GitHub | **GitHub connected / Sign in or connect / Continue locally** | Check only verified existing authorization. If absent, guide provider-approved auth or official GitHub signup; handle denial/offline. Don't ask for password/MFA or copy tokens into prompts |
| Identity registration | **My administrator / System Thought Controller** | Create or restore stable, distinct IDs. Register service identity to Identity; controller gets only limited entity references and authorized handles |
| Main dashboard | **Open Thought Controller** | Navigate into identity-organized thought tiles; show no-thoughts empty state honestly. Missing GitHub/worker is shown as pending, not fabricated success |

A user should not need to open five separate apps or know Git commands to finish the supported install path. During first tests, use a fresh **sandbox** (separate profile/data directory), never delete the owner's live data.

## Returning administrator

Open Identity again: restore same administrator ID, installation ID and controller service ID. Find and reuse a compatible running controller. If stopped, **automatically request startup under the prior saved consent**; do not show the consent dialog again unless the approved executable/source/version/trust scope materially changes, the grant expires/is revoked, or no trusted launcher is available. Verify health after request. Re-check GitHub connection without creating repositories or repeating OAuth unnecessarily.

**The remembered permission authorizes controller startup only.** It does not approve model calls, GitHub access, code updates, posting messages or unrelated executables. No hidden retries or uncontrolled background startup loops.

## Who owns what?

| Component | Purpose |
| --- | --- |
| **Identity app** | Bootstrapping, durable installation/admin/entity/service IDs, account setup and authorization, capability discovery, startup consent |
| **Thought Controller** | Identified independent service managing entity-scoped thoughts; main Netflix-like developer dashboard for many identities; development/test metrics |
| **Project Controller or approved local launcher** | Actual process/build/install/Play/Stop ownership; Identity only requests actions through a separately authenticated interface |
| **Reference Thought Controller repo** | Original architecture and intentionally published approved versions, not a central server that owns another user's identities |
| **Each administrator's installation** | Separate thought data, identities, experiments, permissions, backups and change history |

Identity stays useful while Thought Controller or GitHub is down. A missing worker must not block local setup or show artificial thoughts.

## Critical distribution gap: the current repos are PRIVATE

As of this writing [Identity](https://github.com/Arameair/identity) and [Thought Controller](https://github.com/Arameair/Thought-Controller) are **private and have no published releases**. An unknown new administrator therefore cannot download a verified 'latest' release merely by knowing the repo URL.

**Do not solve this by auto-selecting Alpha/main/most-recent commit or storing the owner's GitHub token in the package.** There must first be an explicitly approved source of installable builds (e.g. published verified release artifacts) OR the new administrator must authenticate to an authorized private source before download. If private-source login is necessary, move the GitHub connection step earlier for **obtaining the binary**, then re-check account capabilities after controller startup. A fresh user without permission to the source gets a truthful **ACCESS REQUIRED / NO APPROVED BUILD** state, not an invented successful install.

For developer-only prototypes, a locally provided, owner-chosen pinned artifact and fake lifecycle adapter can validate the wizard. Label this **TEST / PINNED LOCAL ARTIFACT**, not general-public onboarding.

## Reference and independence

Identity records **origin repo or distribution, immutable approved release/version/commit, verification evidence and local service instance ID**. The origin is a provenance reference, not authorization or a command to follow future commits. A developer may keep local modifications/forks, compare upstream approved updates, and opt into changes after backup/compatibility review. No automatic reset, forced upstream merge, or mutation of the original owner's repository.

The identity ID survives software replacements. Keep installation metadata and entity data in a durable user-selected store distinct from per-branch test directories. New experiments default to sandbox storage; adopting a production profile is explicit.

## Exact sequence we need to prove

1. Create fresh sandbox administrator/installation.
2. Verify no running controller. Grant startup permission once for a **known pinned version** through a trusted lifecycle owner.
3. Launch/reuse it; show truthful process, origin, version/SHA, actual health and service identity.
4. Check GitHub access; show authorized, unavailable, unconfigured or needs reauthorization. No GitHub writes unless separately confirmed.
5. Open the Thought Controller identity-organized dashboard; verify entity isolation and persistent data.
6. Restart both. Same IDs and thoughts; no extra processes, signup, grants or repo creation.
7. Test changed controller version, offline mode, denied login, missing private-repo access and invalid/malicious local service.

**No team should claim the full flow is working until these tests pass on a clean installation.** Work is deliberately split into small independently testable prototypes (see [next phases](NEW_ADMIN_BOOTSTRAP_2026-10-23.md)).