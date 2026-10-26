# First run — Thought Processor is the normal interface (launch order flexible)

**Status:** intended user experience and security acceptance criteria; **not fully implemented**. Reviewed 2026-10-26. Existing experimental Identity candidates still open their *own* setup/monitor browser UI. That behavior is a prototype/debug path, **not the target experience**. See [current status](STATUS.md) and [test instructions](TRY_THE_PROTOTYPE.md).

## One human-facing application

> **People and administrators open and interact with Thought Processor. Identity provides identity, relationships, registration, and authorization behind it. Independent features own their own execution and data.**

**Identity-first describes the authority model, not the executable startup order. The owner confirmed that Identity or Thought Processor can start first; the requirement is a clean, genuinely authenticated state with both available for full system administration.** A user should not need to visit an Identity dashboard to configure accounts, inspect identities, approve features, view service health, or diagnose normal problems. These tasks should be available in **Thought Processor**, which invokes Identity's authenticated interfaces when it needs authoritative operations.

Identity must remain a usable, independently running **service** even when Thought Processor is offline. It is not a mandatory screen, and its startup should not silently launch or terminate Processor. Developer-only direct Identity diagnostics or emergency recovery tools can exist, but they must not become the primary administrative product.

## Process startup and system exit — owner choice

- **Either Identity-first or Processor-first startup is acceptable.** Thought Processor remains the normal human-facing UI when the full system is ready; the underlying launch mechanism is not decided. Missing/untrusted services are accurately blocked, not labelled READY.
- First operator-approved **service pairing** establishes peer authenticity; subsequent restarts reconnect automatically using securely stored scoped credentials. Pairing does not imply permission to access information sources.
- An **explicit Processor application exit** should ask **Shut down managed workspace (selected by default)** or **Keep services running**. A controlled shutdown verifies owned process and child-process exit, reports leftovers, and never kills unrelated services without separate permission. Closing a browser tab is not necessarily a Processor application exit.
- When Identity is unreachable, **new protected actions and queued protected dispatch fail closed**; no offline grace/cached allowance in v1. Independent processes can remain running for safe status and diagnostics.
- These are design requirements; no reliable process-supervisor, secure pairing, or receiver-enforced grant implementation is proven by this page.

See [cross-repository decisions](CROSS_REPO_DECISIONS_2026-10-26.md).

## Proposed first-use journey in Thought Processor

| Step in **Thought Processor** | What the person sees and does | What services must verify before marking success |
| --- | --- | --- |
| 1. Welcome | **Create a new installation** or **restore existing** | Processor can reach the required Identity service or clearly explains it is unavailable; it does **not** claim trust from a port check |
| 2. Establish ownership | Sign in/confirm the appropriate administrator through a deliberate approval flow | Identity establishes durable installation and administrator records through a secure bootstrap protocol; Processor must not become a second canonical authority |
| 3. Identity status | Processor displays Identity's verified identity, connection state and scoped diagnostics | Service identity is authenticated, not just self-reported; IDs and records survive restart |
| 4. Optional GitHub account | Choose whether to connect GitHub and approve provider authorization in the browser | Identity or a defined account component validates the provider result and returns a safe status; Processor never handles a shared master token |
| 5. Discover features | View available vs installed vs connected vs authorized services | Registration, visibility, installation, runtime and grant state are kept distinct |
| 6. Request or approve access | Review the requested operation and scope; accept/deny only where supported | Identity validates authority and records the decision; **each receiving feature enforces it** |
| 7. Monitoring and recovery | One Processor dashboard shows meaningful status, logs and safe diagnostics | No false green states; Identity/feature outages and pending access are accurate and actionable |

The steps may be reached after **either service startup order**. They are **acceptance targets**, not a claim that the current Processor and Identity prototypes support them end-to-end. The intended empty state is: **Processor available, Identity unavailable or ready as verified, zero optional features**—not an error that forces Thought Engine/Memory installation.

## Distinct identity records

- **Installation ID:** durable local deployment/profile, safe to restore without accidental duplication.
- **Administrator ID:** authority-bearing actor; an ID record alone is not authentication.
- **Identity service ID:** durable identity of the authority service, distinct from its **runtime/run ID**.
- **Feature/service ID:** durable registered identity of an independently running component, separate from current endpoint and process.
- **Entity ID:** a companion, person, business or project that uses authorized features; not necessarily a running service.

Processor **displays** these authorized records; Identity remains their authority. Processor may cache display state with freshness labels but must not invent credentials, IDs or grants when disconnected.

## Restart, absence and recovery

1. A user interacts through **Thought Processor** in normal use, regardless of which service process started first. It should make an authenticated attempt to connect to the existing Identity service and show **verified / unavailable / unverified / needs setup** precisely.
2. Identity independently reopens its authorized data profile and validates integrity. Neither component silently rewrites another feature's storage.
3. Processor queries appropriate status, presents persistent identities, feature availability, and any approved permission changes; cached display state must not be confused with current authorization.
4. If Identity is unavailable, Processor displays the problem and offers documented recovery paths. It does **not** grant new permissions, call a claimed endpoint trusted, or require deleting identity records.
5. On **explicit Processor exit**, the person chooses **Shut down managed workspace (default)** or **Keep services running**. For shutdown, the approved process manager must verify that owned services and child processes stop cleanly and report failures. For Keep Running, other independent processes may remain alive; neither choice grants data access. If Identity is down, block new protected work rather than rely on past cached grants.
6. If a developer opens Identity directly, that is an explicit troubleshooting/testing route and must be labelled as such.

## Lifecycle decision remains open

The owner selected **Thought Processor as the human interface** and explicitly allowed **either service startup order**; the safe mechanism for keeping Identity available is still open. An opt-in lightweight background service, manual launch, and a hybrid local activation flow still need comparison and acceptance testing. A first-run implementation must not silently install a privileged startup task or turn Processor into an unauthorized process supervisor.

Service discovery is not trust. Before Processor administers Identity, the two programs need an authenticated, replay-resistant, least-privilege protocol with recovery and offline behavior.

## Current mismatch to resolve

The [October 25 Identity candidate](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) currently starts a standalone Identity browser wizard, shows GitHub and feature actions there, and detects Thought Processor through its public health response. It is a **useful isolated subsystem test**, but the **human interaction direction is inverted** compared with this target architecture. Do not promote that Identity-first UX as the finished system.

The next integration acceptance gate should explicitly demonstrate a **Thought Processor UI** initiating user-visible setup/status while an independent Identity service answers authenticated, read-only requests first, followed by controlled access decisions only when security design and negative tests exist. Use minimal, demonstrable operations; do not declare secure grants complete before receiving services enforce them.

## Minimum acceptance evidence

On a clean OS profile record: exact source branches/SHAs; independent process IDs and ports; reaching the same verified end state using **either startup order**, while using Thought Processor as the normal human interface; Identity unavailable/detected/authenticated distinctions; first-run/admin state provenance; reconnect and restart; rejected/expired/unauthorized calls; no credential leakage in Processor or Identity diagnostics; and an explicit owner decision.

See [Architecture](ARCHITECTURE.md), [Security](SECURITY.md), [Roadmap](ROADMAP.md), [Status](STATUS.md).
