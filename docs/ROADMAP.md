# Development roadmap — experiment in small verified steps

[← Home](../README.md) · [Owner tests](ACCEPTANCE_TESTS.md) · [Coding team instructions](../AGENTS.md)

**Don't build all features at once.** Start from Alpha for each approved experimental prototype, work in a new `experiment/*` branch, and report what is actually observed. Owner approves any Alpha promotion.

## Gate 0: documentation and boundary approval (NOW)

Deliver a clickable repository that a non-programmer can navigate: README, examples, plain explanations, security and module responsibilities, proposed controller registration, a first-run user journey and pass/fail criteria. Discuss open decisions in [Decisions](DECISIONS.md). **No credentials or actual account access.**

## Gate 1: local Identity prototype (FIRST CODE)

- A standalone application visible through Project Controller with independent Play/Stop; simple calm tiles and **Create / Restore / Inspect** screens.
- Generate a stable ID once. Store an entity locally, close app, change to another code branch *in a separate test data sandbox*, and show how an approved release restores the same ID.
- Show pending GitHub and controller statuses. Use mock GitHub and fake Thought Controller adapters for all external operations.
- Safe copyable diagnostics: exact app branch/SHA, data store location, entity ID short form, action and result, no secret values.
- Tests for duplicate IDs, corrupt storage, restart, user cancel, mock offline, no unnecessary side effects or orphan processes.
- **Do not mark setup complete** without the required GitHub repository; permit an honest locally saved **pending setup**.

## Gate 2: real GitHub setup (AFTER Gate 1 security)

- Guided official GitHub account signup if needed, or authorize existing account using approved flow.
- Explicit authorization scopes, encrypted OS-backed credential storage, revoked/expired/denied states; never ask for passwords/MFA or paste access tokens in UI.
- Explicit **Create PRIVATE repo** or **Use suitable existing PRIVATE repo**, verify privacy/ownership/write access, write a **nonsecret schema-versioned manifest** only after preview/approval.
- Restore entity from repo on a second device; reauthorize local credentials; no accidental identity re-creation.
- Test first with a disposable private test repo. Failure/revoke paths are as important as success.

## Gate 3: Thought Controller handshake (AFTER Thought Controller's owner test)

- Coordinate a small compatible experimental registration interface with Thought Controller team. Test health, authentication, idempotent register and reconnect using a fake controller first.
- Under Project Controller, discover already-running Thought Controller; do not start/stop it independently.
- Standalone mode: show saved user approval for optional auto-start if controller is missing. Verify readiness and registration after start; never spawn duplicate processes.
- Register an entity, stop/restart either app, reconnect same ID, and show configured/authorized/reachable/verified states.
- A missing or incompatible interface must show **unsupported/pending**, not silently pass.

## Gate 4: first optional puzzle piece (LATER)

Use a mock Memory module, then an independent real Memory module: request `memory.primary`, guide setup/authorization, save durable nonsecret locator, connect, verify, revoke, replace without losing the entity. This tests the reusable registry concept **without building all possible services**.

## Explicitly not included in the first prototype

- AI personality definitions or “consciousness” systems.
- A full Discord/GitHub agent with broad writes.
- Multi-entity thought concurrency, remote exposed control APIs, arbitrary module auto-installation, agent-initiated account creation, autonomous authorization or code modifications.
- Auto-merging a branch, copying private chat history into GitHub or storing any real token in the repository.

**Stop/go rule:** if a step is FAIL or BLOCKED, capture exact context, repair it in a new experiment and retest. **Do not promote** based only on a build or mock-only response.

[Next: acceptance tests →](ACCEPTANCE_TESTS.md)
