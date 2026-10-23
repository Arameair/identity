# Technical boundaries and prototype assignments — Identity owns bootstrap

**DESIGN / planned implementation only.** [Start Here](BOOTSTRAP_START_HERE_2026-10-23.md) · [Acceptance matrix](NEW_ADMIN_TEST_MATRIX_2026-10-23.md) · [Existing local Identity V1](https://github.com/Arameair/identity/tree/experiment/identity-local-v1-20261023) · [Thought Controller](https://github.com/Arameair/Thought-Controller)

## Fixed architectural decisions

1. **First program:** user starts Identity. It creates/restores a **stable administrator principal** and an **installation ID**. Administrator, installation, controller-service identity, and individual managed identities are different records; no reused UUIDs.
2. **Controller dependency:** Identity checks for Thought Controller each launch; it is the primary dashboard and thought engine. Identity's standalone functions still work if controller missing.
3. **Saved opt-in (option B):** first missing-controller startup asks consent once. Later missing-controller startups may automatically request the **same trusted approved launch target**, subject to revocation and version/source changes.
4. **GitHub check:** after controller discovery/request unless authentication is required first to obtain a private approved build. Account check does not itself authorize repo writes.
5. **One framework, independent admins:** Thought Controller originates from the reference implementation but runs with an **installation-specific system identity**, independent entity data and fork/update options.
6. **Project Controller isn't Thought Controller.** It (or a separate approved installer/launcher) owns process lifecycle. Identity never silently starts uncontrolled child processes. A new-user distribution cannot assume Project Controller is already installed: implement a standalone supported launcher path or show an explicit prerequisite.
7. **Thought Controller main view:** browse thoughts by owner identity (Netflix/Prime-style sections); developer can inspect details/logs. Do not build a competing daily-dashboard application.

## Durable model (proposed; field/API names NOT yet stable)

| Record | Example fields | Owner |
| --- | --- | --- |
| installation | installation_id, created_at, schema, local data-root | Identity |
| administrator | entity_id, type=administrator, display name, owner/role proof | Identity |
| controller-service | entity_id, type=system_service, installation_id, reference_origin, approved_version+SHA, runtime locator, protocol compatibility | Identity registry, verified with running Thought Controller |
| managed identity | entity_id, type=companion/business/project/other, display name, repo association (pending until approved), capability references | Identity |
| launcher consent | approved version/source/signature/target, lifecycle provider, grant time/status, revocation metadata | Identity protected local policy |
| GitHub connection | provider account reference, checked scopes, expiry/revocation, secure credential reference | Identity with OS-backed credential storage, not in GitHub |

**On upgrade:** keep identifiers, relationships and data; migrate schemas only with backup and owner review. Different admin installs never inherit reference creator's IDs, sessions, private repos or tokens. Thought Controller itself can appear under 'System Services', not as a normal person with compulsory thoughts.

## Suggested internal contracts (negotiable; do not claim implemented)

**Identity → trusted launcher:** Discover(status), RequestStart(request_id, pinned_artifact_identity, approved_consent_reference), VerifyStarted(service_id, process/endpoint), Stop only under separate explicit admin action. Authentication, exact target allowlist, idempotent job IDs, ownership checks, deduplicated start, failure/timeout states; no arbitrary command execution or raw shell supplied by a model.

**Identity ↔ Thought Controller:** Read-only service health/handshake returns service identity, installation binding, protocol version, instance version/SHA, readiness and registered features. Authenticated idempotent RegisterEntity(entity_id, entity_type, owner_installation_id) returns registered/already_registered/denied/unsupported. Entity selection and thought reads require a verified entity scope. Capability requests return limited handles or approval-required; no provider credential values.

**Identity → GitHub:** Check existing authorization with provider-supported non-mutating verification; when absent, open official login/signup/consent. Scope new private-repo creation as a distinct user-approved operation. Persist provider tokens only in protected OS store; do not put tokens in GitHub (even private), Thought Controller prompts, logs, or downloaded manifests. Authentication is per administrator, not inherited from reference creator.

## Implementation gates — don't combine the whole system in one patch

**P0 local foundation (existing):** Test local Identity create/save/restore V1 in owner's Project Controller first; report owner PASS/FAIL. No Alpha promotion based only on sandbox tests.

**P1 Identity bootstrap mock and UI:** Extend an owner-approved baseline in a new experiment branch. Create fresh administrator/installation and controller-service records, auto-start consent/revoke UI, verify discovered service identity, show startup state machine with a **fake bounded launcher / fake controller** only. GitHub card reports MOCK, NOT CONNECTED, CONNECTED or REAUTH REQUIRED honestly. Run fresh/restart/denial tests without external side effects.

**P2 Thought Controller service interface and dashboard:** Separate Thought Controller team's experiment: verified local handshake and idempotent registry with at least two isolated managed entity thought collections; new-user empty state and browsable tiles. Keep existing thought parser/safety and evidence. Mock model acceptable for structure; label simulation.

**P3 Real lifecycle path:** Negotiate with Project Controller team for an authenticated narrow lifecycle adapter OR build a separate user-approved standalone launcher. Test one **pinned authorized** Thought Controller artifact, no duplicates, no arbitrary commands, clean stop/restart, rollback. If a required release doesn't yet exist, show **BLOCKED** and test pinned local build, not latest.

**P4 GitHub account and private repo:** Provider-approved login with protected credentials and explicit scopes, private repository create/select/verify, nonsecret manifest per entity, no automatic external writes on startup, revoked/offline tests. May be needed earlier only for private-source download.

**P5 Owner acceptance:** Run full clean-user test on a separate install/profile (see acceptance matrix). Record precise SHAs, actual call counts, process owners, repo provenance, status of real vs mock paths, and approval history. Only after owner verification consider promotion.

**Ordering nuance:** P1 and P2 can be built independently against mock peers, but integration must test *real* behavior and protocol compatibility. P3 launcher dependencies require separate approval by the lifecycle owner. P4 must not block basic offline Identity use.

## Out of scope for these first prototypes

No continuous autonomous thoughts at boot; no implicit Chat-Jimmy calls; no arbitrary code changes, self-approval of permissions, real public distribution, remote admin control plane, forced repository updates, shared global identity data, or multi-tenant security claims without isolation tests.

## Branch and evidence policy

Each coding team starts from current **owner-approved Alpha** in its own repository, creates a separate experimental branch, and reads these design notes by link. Do not start from an unpromoted experiment without explicit owner approval. The current tested local Identity experiment and Thought Controller repair experiment are **not** Alpha; review integration strategy explicitly. No auto-merge/promotion. Each team hands back a runnable prototype, exact full SHA, tests PASS/FAIL/BLOCKED/UNVERIFIED, diagnostic copy and rollback procedure.