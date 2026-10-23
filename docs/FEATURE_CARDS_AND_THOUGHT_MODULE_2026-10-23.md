# Identity as an ID card and feature controller — owner design, 2026-10-23

**Decision update; planned architecture, not implemented.** This supersedes earlier suggestions that every entity automatically thinks, or that Thought Controller itself is responsible for account ownership. See [new-user setup](BOOTSTRAP_START_HERE_2026-10-23.md), [first-run plan](NEW_ADMIN_BOOTSTRAP_2026-10-23.md), and [Thought Controller's complementary design](https://github.com/Arameair/Thought-Controller/blob/experiment/multi-identity-controller-design-20261023/docs/THOUGHT_FEATURE_BOUNDARY_2026-10-23.md).

## One way to explain the system

**Every entity has an ID card. The card says who it is, what it owns and what it can do. Features provide capabilities, but owning an identity alone does not give it those capabilities.**

Identity is the **central authority** for permanent IDs, identity cards, ownership, approved accounts, GitHub access, feature installation, feature grants, and configuration references. Features do not each build independent login managers or retrieve raw GitHub tokens.

### What an ID card contains

A minimal card holds stable entity ID, display name, type (system / administrator / service / companion / business / other), installation scope, owner/relationships, creation timestamp, schema version and a list of configured feature references. The card can later hold descriptive nonsecret information and pointers to related data.

An ID card also indicates:
- **Accounts**: provider names, which account is linked, permission state and secure reference — NEVER password, token or session cookie.
- **GitHub**: approved per-entity private repository association, verified ownership/visibility and nonsecret configuration manifest.
- **Features**: name, version, installed/available vs enabled/disabled, scoped grant status, data location/health, approval and revocation information.
- **Relationships**: owns/is owned by/authorized to act for, without implicit privilege transfer.
- **History**: configuration and permissions changes/auditable decisions, not high-frequency thought processing.

An entity's GitHub repository can store its approved **nonsecret blueprint/ID-card manifest**, but the local Identity database is authoritative for current grants and runtime state. On first creation, GitHub may be **PENDING**; do not block local identity creation. Secrets are kept in the OS-protected store. Thought traces remain in the Thought module's own fast local storage.

## The system identity and the first feature

On installation, Identity generates a **new local System ID Card** (not the repository's code identity and not the original author's personal ID) and separate administrator identity. Identity registers **Thought** as an available system feature. The actual Thought service/module has a stable service identity, installation binding, independently verified version/source and local storage. Thought Controller is the operational UI for this feature, not the sole owner of thinking, GitHub or credentials.

System has capability **available/installed** by default only when installed and verified. Individual entities like TASS or a business **cannot produce sparks, traces or sustained thoughts until Thought Processing is specifically enabled and authorized on their ID card**. A business can exist indefinitely with no thinking enabled. An administrator can revoke the capability while leaving the entity, its GitHub repo and approved stored history intact.

### One-card enablement flow — desired end-user behavior

Owner opens TASS → **Features** → **Enable Thought Processing**.

1. Identity checks whether Thought is installed, version-compatible, healthy and from an approved source. If missing, it offers a specific version and asks approval before installing or starting it.
2. Identity explains scope and effects (this grants Thought permission to process events for *TASS only*), shows storage/worker implications and obtains administrator approval.
3. Identity records one durable, auditable scoped grant tied to TASS's permanent ID, the Thought feature identity, permission set, allowed data/worker limits and policy expiry/revocation.
4. Identity sends a versioned registration/configuration request to Thought using a verified authenticated channel; duplicate enable requests MUST be idempotent.
5. Thought checks the grant on its own boundary before accepting data or processing a spark. Identity owns authorization decisions, but feature services independently enforce them. No shared master credential and no blanket trust merely because something runs on localhost.
6. TASS's card changes to **Enabled / Ready**, **Enabled / Waiting for Thought**, or **Denied / Error** based on actual verification. An available feature is NOT the same as an enabled or running one.
7. Thought Controller displays the TASS profile and its thought collection only if authorized. Empty state is legitimate. Disabling/revoking stops new processing for TASS without destroying its identity or silently erasing saved data.

**One click from the user's perspective; several explicit, verifiable operations internally.** Never bypass approval, provider restrictions, install trust or feature-specific permission checks to make the UI look simple.

## A reusable feature system

Thought is the FIRST feature, not a special hardcoded exception. A future Memory, Screen Perception, Discord or GitHub feature should be discoverable and explicitly enabled in a similarly consistent way. Identity stores the common feature manifest/grant; modules keep their own implementation, metrics and bounded domain data. GitHub account management stays centralized in Identity and uses scoped actions, not handing broad tokens to every feature.

## What should be tested now

- Create a clean test System card, TASS card and Business card; all different stable IDs.
- Show Thought **available** on System, but **disabled** on TASS and Business by default.
- Select **Enable Thought Processing** for TASS; in first test, use a clearly MOCK Thought adapter and test exactly one scoped grant/registration. No real GitHub writes.
- TASS gains permission to send a simulated observation and receive one spark/trace/thought progression; Business is rejected at server/service boundary while feature disabled.
- Restart and confirm ID cards, grant, feature state and TASS's thought history persist; disabling TASS halts new processing but doesn't erase approved history.
- Enable Thought for Business separately and verify completely isolated thought data and permissions.
- No hidden AI model calls or automatic authorization simply because feature is preinstalled.

### Provenance and branches

The local Identity prototype, Thought Controller repair, and new-user bootstrap designs are **unpromoted experiments**. Claude may already be building from earlier notes. Team must READ this new decision and **report conflicts rather than silently rebuilding everything or modifying Alpha**. Start future experiments from current owner-approved Alpha, or request owner approval to port exact commits. The Thought implementation goes in its OWN proposed **Arameair/Thought** private repository, not as an inseparable embedded part of Thought Controller. See [new repo design handoff](THOUGHT_REPOSITORY_PLAN_2026-10-23.md).
