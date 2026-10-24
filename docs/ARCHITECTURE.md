# Architecture — Identity as the authority, features as independent owners

**Owner-selected principle (2026-10-24):**

> **Identity owns the organization and authority of the system. Each feature owns its actual work.**

**Status:** Architecture and integration direction, **not implemented multi-service authorization**. The current experimental Go manager creates local records and detects some services without authenticating them. [Current status](STATUS.md).

## Three different layers

```text
+-------------------------------------------------------------+
| Identity: identity and authority                             |
| Durable IDs | ownership | account references | storage      |
| locations/policy | registered services | scoped grants       |
+----------------------+--------------------------------------+
                       |
       discover, enroll, approve and verify (control plane)
                       |
+----------------------v--------------------------------------+
| Thought Processor: optional cross-feature control surface   |
| service cards | connection map | real logs/flow/metrics     |
| original feature UI | truthful status and diagnostics       |
+-------------+---------------------+-------------------------+
              :                     :
       observations/status   diagnostics/status (not all data)
              :                     :
  +-----------v--------+   +--------v------------+
  | TASS / ScreenBridge|-->| Thought Engine      |
  | feature-owned logs |   | feature-owned waves |
  +--------------------+   +----------+----------+
                                     |
                                     v
                                AI worker
   Optional Memory is an independent source of authorized recall.
```

Arrows represent *possible authorized connections*, **not already implemented APIs** and not mandatory relays. Features can send source data to each other directly once the receiving feature enforces appropriate authorization. Processor is not required for their ongoing work.

## Responsibility matrix

| Component | Owns | Does not own |
| --- | --- | --- |
| **Identity** | Stable installation, administrator, entity and service IDs; ownership; account authorization references; registration/grant policies; storage-location governance and audit decisions | Every service's content, AI reasoning, model calls, process execution or raw log transit |
| **Thought Processor** | Catalog-based discovery of compatible running services; authorized connection assistance; inspection, truthful status cards, developer diagnostics | Canonical identity authority, credential vault, automatic deployment, thought content or every log |
| **Thought Engine** | Optional observation processing, candidate thoughts, directed waves, temporary working context, source evidence, its own records/worker calls | Issuing identity-wide permission or treating thoughts as memory |
| **Memory** | Optional, separately enabled persistence and recall of historical experience | Becoming mandatory for any thought or identity |
| **Source features** | Their own data collection, logs, domain functionality, lifecycle and permission enforcement | Automatic access to any other entity or feature |
| **Project Controller** | Separate developer-only branch builds, Play/Stop, inspection and test context | User Identity authority or production dependency |
| **External providers** | Their own accounts and native access control | Identity's local records or implicit grant to every feature |

## IDs, ownership and running instances

Do not collapse distinct concepts:
- **Installation ID:** durable local deployment/profile.
- **Administrator ID:** authenticated administrative principal as defined by a future OS-aware security design.
- **Entity ID:** durable person/companion/business/project identity.
- **Service ID:** durable identity of an installed registered feature, including the Identity service itself.
- **Instance/run ID:** one execution of a service; can change on every start or run in parallel.
- **Account connection:** provider-specific authorization distinct from an entity record.
- **Capability grant:** scoped authorization for a caller to perform a named operation on resources for a specific entity/service, with expiry and revocation rules.

A source repository, announced port, version string or self-reported UUID alone does not establish service authenticity.

## Registration and interoperability

1. Feature starts **independently**. It may request registration with Identity using its durable service identity, authenticated enrollment/installation association and bounded capability description.
2. Identity checks ownership/trust and records the service, version, protocol, authorized endpoints and runtime instance information.
3. Thought Processor discovers approved catalog entries and can display a live card for each actual app, with fallback for unavailable embedded UI.
4. An identity-scoped, resource-scoped permission is explicitly approved; the **receiving service** must enforce it. Discovery alone cannot bypass a grant.
5. After approval, services may exchange data directly; the Processor may inspect source-owned logs/metrics on demand with appropriate scope.
6. Restart, revocation, incompatible versions, Identity unavailability or source downtime must have distinct truthful states.

**Lifecycle choice OPEN:** Whether Identity itself runs as an opt-in lightweight background service for always-available registration, manual-on-demand, or hybrid. Do not install an unattended service without an explicit, tested consent path.

## Storage is centrally organized and independently owned

Identity stores the canonical ownership/location records and enforces policy. Each feature stores its **own** events, logs, database, thought traces or memories within approved isolated areas.

```text
Selected storage root / registered storage locations
  identity/       canonical authority and audit
  features/tass/  TASS-owned data
  features/thought-engine/  Thought-owned data
  features/memory/         Memory-owned data, when installed
```

This is a **logical example**, not a mandatory physical directory or a claim that the existing prototype provisions it. Do not route high-volume logs through Identity's registry and do not commit them to source-code repositories. Multiple roots/volumes and OS/VM boundaries must be supported by policy and tests. Read [storage detail](STORAGE.md).

## Clean machine and failure behavior

- Identity must be usable with **zero optional features** and without GitHub sign-in. [First-run proposal](FIRST_RUN.md).
- An optional feature can be stopped without deleting its durable service registration or corrupting another feature.
- Closing Processor does not stop source services or revoke their previously valid grants.
- If Identity is unavailable, no **new** privilege should be granted. Previously authorized independent work may continue only under a tested, bounded offline policy.
- If a service proposes a direction for additional observation, it cannot create a new grant or start an uncontrolled program.
- Experimental `PC_DATA` and branch checkouts must stay distinct from durable production data.
- Backups, migrations, restore, recovery and credential storage must be tested before the system is described as a secure multi-service provider.

## Security reality

The present prototype uses an already authenticated GitHub CLI session for read-only verification; this **does not** make Identity an OAuth broker. Local port checks are discovery, **not** authenticated service registration. Loopback and HTTP Origin checks do not create a trusted channel by themselves. See [Security](SECURITY.md), [Interfaces](INTERFACES.md) and [Status](STATUS.md).

## Sources of direction and unresolved questions

- [Decisions](DECISIONS.md) distinguishes accepted architecture from proposed lifecycle details.
- [Independent review](REVIEW.md) preserves contrasting views, including [end-user notes](END_USER_DESIGN_NOTES.md).
- [Thought Processor connection direction](https://github.com/Arameair/Thought-Processor/blob/experiment/identity-authority-registration-20261024/docs/discussions/2026-10-24-identity-authority-registration.md).
- [Thought-directed waves hypothesis](https://github.com/Arameair/Thought-engine/blob/experiment/thought-engine-v1-20261023/docs/THOUGHT_DIRECTED_WAVES_THEORY_2026-10-24.md).

**Do not treat links to experimental work as evidence of promotion or an implemented feature.**
