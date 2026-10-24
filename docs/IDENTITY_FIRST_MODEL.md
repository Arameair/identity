# Identity-first architecture

**Design direction · 24 October 2026 · Not an implemented cross-service protocol**

Identity is the **durable identity, trust, relationship and policy authority** for an ecosystem of independent applications and capabilities. The owner clarified that **every independently developed feature should also have an identity**. This is broader than a profile manager for a companion or an external-account broker.

The current main branch is **documentation**, not a production service. Runnable Identity features and experimental discovery exist on separate unapproved branches. See [status](STATUS.md).

## Architectural thesis

- **Identity knows who and what exists.** It preserves durable identities, purpose or capability descriptions, relationships, ownership/trust and authorization decisions.
- **Thought Processor knows what's running.** It discovers live service instances, connects compatible, authorized components, displays their actual UIs where safely possible, and exposes status, diagnostics and version evidence.
- **Thought Engine determines when and how to think.** It consumes approved observations, records Sparks/Traces/Thoughts, assembles bounded context, and calls replaceable workers. It need not send every inference request through Processor or Identity.
- **Memory may preserve selective experience and retrieve it for future reasoning.** The precise boundary between Thought Engine's historical thoughts and a future Memory service is an **open research question**.
- **Every service enforces its own authorization.** Identity is the authority for grants; discovery, pairing and a shared feature label never themselves grant access.

This division gives Identity system-level authority without making it a runtime bottleneck or a cognitive Thought Engine.

### Organization versus storage (additional owner-selected boundary)

**Identity organizes the system and records authorized data locations; individual features own their actual databases, logs, thoughts, memories and work.** A feature can register its independent service identity with Identity; Thought Processor then discovers the approved service catalog and verifies which **running instance** is reachable. Do not funnel feature data through Identity merely because it provides the directory of identities or approved storage locations.

This update aligns with the [current Identity architecture](ARCHITECTURE.md) and [owner decision ledger](DECISIONS.md). Whether Identity runs continuously as an opt-in background service, manually on demand, or in a hybrid mode remains open.

## Four distinct notions of identity

| Concept | Example | Purpose |
| --- | --- | --- |
| **Entity / participant** | TASS, a person, a business | Durable subject of ownership and authorization |
| **Feature or service identity** | A separately trusted Memory component | Durable identity of an independently developed service |
| **Capability type** | `memory`, `vision`, `inference` | Describes what a service offers; not proof of trust |
| **Runtime instance** | The current Memory process on a local port | Transient live endpoint; not a durable identity |

**Source repository, build SHA, service identity, installed application and process instance are different facts.** A claimed repository URL or healthy HTTP port does not authenticate a service.

An Identity *application card* inside Thought Processor represents the real **running Identity application**, not a separate application for each TASS/business record.

## Current owner-selected behavior (still unimplemented)

1. **Small shared connection core** with optional feature-specific capabilities.
2. **Hybrid discovery:** compatible apps self-register through an automatic same-OS local discovery mechanism when practical; configured address or manual connection for remote machines/VMs.
3. **One-time Identity-controlled service pairing**, with credential-verifiable reconnection; untrusted discovery does not equal pairing.
4. **Scoped, entity-specific authorization and feature approval** owned by Identity and enforced by recipient services. Expanded permissions require fresh approval.
5. **Short-lived authorization during temporary Identity outages** for preauthorized connections only, expiring to a safe pause for protected operations; 5–15 minutes is a candidate test range, **not** a committed value.
6. **Hybrid health reporting:** significant changes pushed promptly, occasional verification, stale/unreachable states labeled accurately.
7. **Distributed logs and correlation IDs:** services keep their own logs; Processor can request bounded authorized diagnostics.
8. **Independent lifecycle:** no required Project Controller installation and no hidden process management or mandatory inference proxy.

The protocol, pairing credential scheme, local discovery registry location, exact TTL, and optional SDK strategy **remain open**.

## Outstanding choices—do not represent these as settled

- **Relationship model:** Are feature identities owned hierarchically by a companion, or independent participants connected through explicit relationships? The second was recommended, **not chosen**.
- **Purpose and evolution:** Does Identity retain only stable description/authorization, or also changing goals and a controlled self-model? Which updates require owner approval?
- **Service identity scope:** Created by feature developer, installation, operator or a combination? How are multiple instances / shared services handled?
- **Repository policy:** Does every feature identity need its own private GitHub repo, or only particular entity types? Never treat a repository as a secrets vault.
- **Cognitive relevance:** Does identity-based purpose materially improve Thought Engine decisions beyond ordinary instructions and retrieved memory? **Not established**.
- **Bootstrap/recovery:** How does Identity establish the first administrator and trusted counterpart while operating entirely independently?

## Research question and falsifiable hypothesis

The current working hypothesis is:

> Stable identity/purpose context plus relevant retrieved experience can improve the coherence and contextual usefulness of an otherwise equivalent Thought Engine compared with recent observations alone.

This is a **hypothesis**, not proof of intelligence or a justification for a mandatory dependency. Compare purpose-only, memory-only, combined, and rolling-context baseline configurations with the same observations/model/context budget. Measure delayed recall, contradictions, purpose-relevant decisions and latency. Keep the security controls constant across these cognitive comparisons.

See [the companion hypothesis and Grok critique](https://github.com/Arameair/Thought-Processor/blob/main/docs/IDENTITY_EXPERIENCE_RESEARCH.md) and [open multi-model architecture review](https://github.com/Arameair/Thought-Processor/issues/7).

## Recommended prototype order

1. Prove create/save/restore of a local entity ID and isolate experimental test data.
2. Introduce a minimal independently identifiable service without Project Controller.
3. Demonstrate discovery **without** privileged access; then explicit pairing and one scoped grant with a real allow/deny test.
4. Test independent restarts and bounded authorization expiry.
5. Add Thought Engine/Memory integration only with measured context/retrieval benefits; avoid creating a universal platform before validating the small path.

**The owner approves changes and all Alpha/Stable/Main promotions. These notes do not merge experimental code or establish a working release.**

---
*Authorship: owner-directed design written with AI assistance. Grok feedback is acknowledged as user-supplied, independently unverified model critique; no AI analysis is automatically accepted.*
