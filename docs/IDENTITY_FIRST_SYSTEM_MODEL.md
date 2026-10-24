# Identity-first system model — the foundation behind Thought Processor

**Date:** 2026-10-24  
**Status:** Owner clarification + technical interpretations for discussion. **Not implemented or owner-accepted as a detailed protocol.**  
**Provenance:** Notes drafted with AI collaboration from the owner's architecture discussion.  
**Branch:** `experiment/processor-identity-connection-notes-20261023` (originally branched from `alpha` @ `8d08fec6629dc9cc35ea2266556572b86e5499d7`).  
**Related:** [Identity ↔ Thought Processor integration](THOUGHT_PROCESSOR_INTEGRATION.md), [Trust and bootstrap](IDENTITY_PROCESSOR_TRUST_AND_BOOTSTRAP.md), and [Thought Processor design decisions](https://github.com/Arameair/Thought-Processor/blob/experiment/six-design-decisions-20261023/docs/discussions/2026-10-23-six-design-decisions.md).

## The owner's key clarification

> **Identity is the heart of the overall architecture. Every independent feature will have its own identity. Identity acts as the “brains” behind Thought Processor.**

This is a stronger role than a human-account manager or a registry only for companions/businesses. **Identity is the authority for identifying and describing all participants**, including features and the applications that manage them; understanding relationships between them; and determining what is trusted and permitted. **Thought Processor consults these identities and relationships to organize safe connections and make the system observable.**

This does **not** mean that Identity runs every feature, proxies inference, stores all feature data, or becomes the cognitive Thought Engine. Treat "brains" as the system's authoritative identity/relationship/policy knowledge **until the owner further defines the boundary**.

## Participants that need identities (owner direction)

- **Companions or actors**: e.g. TASS, people, businesses, projects, agents.
- **Independent features/services**: e.g. Thought Engine, Memory, Hearing/Discord, Vision/ScreenBridge, communication bridges, replaceable AI workers.
- **Foundational components**: Thought Processor itself should be representable and distinguishable as a participant; Identity's own application/installation also needs a safe bootstrap identity strategy.

**A feature identity is not the same thing as the companion it serves.** A companion can be authorized to use a feature without owning that feature's entire installation, its credentials, or the private data of other companions.

### Terms to keep distinct

| Concept | Example | Stable across |
| --- | --- | --- |
| **Entity identity** | TASS's permanent identifier | Restarts, code updates, changing display name, relocation after approved restore |
| **Feature/service identity** | A particular authorized Memory service or component identity | Service restarts; exact scope of persistence/installation still to decide |
| **Software type / capability** | “memory”, “vision”, “thinking”, “inference” | Multiple independently made implementations; capability name is *not* a trusted identity |
| **Application installation** | The code/config that offers Memory on a particular machine | Its own upgrades; not necessarily equivalent to service identity |
| **Running instance / session** | Memory process at local port 7790 during run #3 | **Not** stable across restarts, address or process ID changes |
| **Identity-owned relationship** | TASS authorized to use Memory service M with scope `memory.read` | Only while the owner's relationship and grant remain valid |

These distinctions are a **recommended modeling discipline**, not an agreed database schema. A single feature may have multiple installs or instances; decide whether each needs its own durable service identity before implementation. A repository URL is source metadata and **not** proof of service identity.

## Proposed high-level model

```text
                         IDENTITY
            authoritative registry / relationships
                trust, grants, revocation
                     /          \
                    /            \
          TASS identity     Memory service identity
                   \          /
                 authorized use relationship
                            |
                     Thought Processor
            discovery / connection / health /
                 operator diagnostics
                            |
          fast direct application-to-application
          operations after authorization is checked
```

**Conceptual only.** Do not interpret the arrows as mandatory routing through Thought Processor, or every operation synchronously calling Identity.

## Boundary between Identity and Thought Processor

| Identity is authoritative for | Thought Processor is responsible for |
| --- | --- |
| Durable entity and feature identities, trusted service credentials | Finding *running* service instances through agreed discovery |
| Service types and declared capabilities, including compatibility metadata or references | Inspecting advertised capabilities and protocol compatibility |
| Identity-to-identity relationships, ownership/delegation and scoped policy | Presenting who is connected, requested vs approved vs active |
| One-time pairing decisions and trust revocation | Executing or assisting approved connection/reconnection workflows |
| Granting access for an entity to use a feature, respecting per-feature privacy | Correlating bounded logs, events and activity; live application status/cards |
| Durable identity configuration and recovery policy | Version/test evidence **without** owning code deployment or feature lifecycle |

**Boundary requiring further owner clarification:** Does Identity only hold the trusted graph of identities/capabilities/grants, or should it also decide which specific compatible feature instance to select when several are available? This determines how much connection/routing choice belongs to Thought Processor. **Do not declare this decided.**

## Relationship design is OPEN (do not assume an answer)

The owner has established that **every feature has its own identity**, but has **not yet accepted** either of the following relationship policies:

- **A — Ownership hierarchy:** A companion owns one or more feature identities. Easy to reason about initially, but restrictive for shared services and independent feature distribution.
- **B — Independent identities with explicit relationships:** TASS, Memory and Thought Engine each have durable identities; Identity can grant TASS access to particular services without transferring ownership. More flexible for multi-tenant features, but requires more careful permission isolation and relationship modeling.

**Option B was recommended during discussion, not yet chosen by the owner.** It must remain a question for a subsequent conversation, not be introduced as if approved.

Also open: Is a feature identity created by its developer, its installer, its end user, or some combination? Can the same distributed feature be installed for multiple owners? Are identities attached to installations, service principals or both? Does each feature identity need a separate GitHub repository, or only selected entities? None of these has been decided.

## How the existing connection choices fit

**Already owner-selected at a *behavioral* level:**
1. Every participating application uses a **small common connection core** and advertises only the optional capabilities it supports.
2. **Hybrid discovery**: same-OS local registry/record plus configured address or manual fallback across other machines/VMs.
3. New apps can appear automatically **but discovery is not proof of trust**.
4. **Identity handles one-time trusted pairing**, with cryptographically verifiable reconnection; a discovered port/name alone cannot impersonate a service.
5. **Identity authorizes** scoped feature access; *receiving services enforce* scope/audience/entity validity, avoiding Identity in every fast-path inference call.
6. **Short-lived grants** can carry an already-approved connection through a brief Identity outage; expiry stops protected operations, not unrelated applications.
7. **Hybrid status**: significant pushes plus occasional health verification. Status checks do not trigger AI inference.
8. **Distributed logging** plus common trace IDs enables Processor to inspect failures without hoarding private logs.
9. Thought Processor is **not dependent** on the owner's personal Project Controller; every service, including Identity, remains independently runnable.

**Still open:** exact wire protocol/SDK, ownership graph, grant and trust credential format, whether service identities are global or per-install, registrar bootstrap, cross-machine recovery, routing/instance selection responsibility and default data retention.

## Example to validate (not evidence of working software)

- TASS has a durable identity **TASS-1**.
- An independently developed Memory component has a distinct trusted identity **MEMORY-1**, advertises capability `memory`, and runs as transient instance **run-42**.
- Identity stores an owner-approved, scoped relationship enabling **TASS-1** to read/write *its own* namespace in MEMORY-1; another entity has no implicit access.
- Thought Processor discovers run-42, verifies or queries trusted status, shows the Memory application's **real** status/UI when expanded, and maps its relation to TASS.
- A request for TASS uses direct authorized service-to-service communication. Memory checks the request against scoped authorization. Logs are retained locally and can be correlated by ID.
- Memory restarts at a new port, becoming **run-43**; identity, trust and TASS's approved relationship persist as appropriate, but **runtime reachability must be re-verified**.
- If Identity goes offline, approved short-lived authorization does not renew indefinitely; sensitive operations pause when it expires.

Names and identifiers above are illustrative only; this is **not** a working integration or selected data format.

## Design questions for the next Thought Processor conversation

1. If Identity is the graph/authority for the system, **what minimal answers should Thought Processor request** to build its running connection map and dynamic app cards?
2. How should Processor combine **Identity's durable, authorized relationships** with **live discovery facts** without confusing them?
3. If multiple compatible Memory workers appear, **who selects which one TASS uses**—Identity's policy, Processor's resolver, or TASS/Thought Engine?
4. How does the system avoid a circular startup dependency when **Identity itself is a registered, independently running participant**?
5. What is the absolute smallest integration test proving separate **feature identity**, **runtime identity**, **pairing**, **authorization** and **fast direct use** without Project Controller?

**No code changes, no test success, and no branch promotions are asserted by this document.**
