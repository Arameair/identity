# Architecture — Identity as the authority, features as independent owners

**Owner-selected principle (2026-10-24):**

> **Identity owns the organization and authority of the system. Each feature owns its actual work.**

**Status · October 27, 2026:** The owner-promoted P0 Alpha implements authenticated Processor–Identity pairing, while the unpromoted P1 cleanup experiment implements scoped grants and live checks **for a deterministic test fixture**. The complete multi-feature, cross-service authorization design remains unverified; see [current evidence](STATUS.md). Thought Processor is the normal human interface; Identity owns authority.

## Human interface versus service authority

**The person or administrator opens Thought Processor.** Thought Processor presents the system: setup, identities, account connections, available features, requested approvals, health, logs and diagnostics. It requests authoritative information and operations from **Identity** via an authenticated interface implemented in P0 for pairing and extended in the P1 experiment for limited grants. Identity does not become a second normal user dashboard. A developer-only direct Identity diagnostics/recovery path is acceptable if clearly scoped.

**Important:** Thought Processor is **the control surface**, not a credential vault or the authority that issues grants. Identity must run independently and receiving features must enforce granted permissions. P0 demonstrates authenticated pairing, and P1 provides an experimental Admin enrollment and operation-specific approval flow. Neither constitutes production-grade bootstrap, recovery, or universal feature enforcement; merely reversing a discovery arrow or embedding a page cannot substitute for them. Processor is not a mandatory data-path proxy.

## Three different layers

```text
                     PERSON / ADMINISTRATOR
                              |
                     sees and interacts with
                              v
  +----------------------------------------------------------+
  | THOUGHT PROCESSOR — primary human/admin control surface  |
  | setup | identities | features | approvals | diagnostics   |
  +--------------------+----------------------+--------------+
                       |                      |
     authenticated     |                      | status / UI
     management        v                      v
  +--------------------------------+   +---------------------+
  | IDENTITY — authority service   |   | INDEPENDENT FEATURES |
  | durable IDs | relationships    |   | Thought Engine, TASS |
  | registrations | scoped grants  |   | Memory, others       |
  +----------------+---------------+   +----------+----------+
                   |                              |
             authorization                 own execution,
             decisions (planned)           logs and data
                   +-------------> receiving features enforce
                                  valid permissions locally
```

This diagram shows **intended responsibilities**. P0 implements authenticated Processor pairing; P1 demonstrates receiver-enforced grants for one explicitly trusted local test fixture. Engine, Fast AI Worker and general third-party feature enforcement are **not verified**. Data does not have to route through Processor or Identity.

Features may send data directly to one another when the receiving feature enforces valid authorization. Processor is the **human control point**, not a mandatory relay for independent feature work.

## Responsibility matrix

| Component | Owns | Does not own |
| --- | --- | --- |
| **Identity** | Stable installation, administrator, entity and service IDs; ownership; account authorization references; registration/grant policies; storage-location governance and audit decisions | Every service's content, AI reasoning, model calls, process execution or raw log transit |
| **Thought Processor** | **Primary human and administrator UI**; first-run/restore presentation, identity and account management views, user consent/approval screens, feature catalog, live diagnostics, authorized service interaction | Canonical identity authority, credential vault, independent feature execution, direct access to ungranted resources, or pretending discovery is authentication |
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

- The **Identity service** must work with zero optional cognitive features and without GitHub sign-in; normal human interaction happens through **Thought Processor** when available. No separate Identity browser dashboard is required for product onboarding. [First-run proposal](FIRST_RUN.md).
- An optional feature can be stopped without deleting its durable service registration or corrupting another feature.
- Closing a browser tab is not consent to stop services. On an **explicit application exit**, the owner-selected design asks whether to shut down managed workspace processes (default) or keep them running; only system-owned processes may be stopped through an approved lifecycle controller.
- If Identity is unavailable, the selected v1 policy is to **fail closed** for new protected operations and queued protected dispatch. Existing local history viewing and standalone non-sensitive health checks may continue; no cached grant/offline grace is authorized.
- If a service proposes a direction for additional observation, it cannot create a new grant or start an uncontrolled program.
- Experimental `PC_DATA` and branch checkouts must stay distinct from durable production data.
- Backups, migrations, restore, recovery and credential storage must be tested before the system is described as a secure multi-service provider.

## Security reality

Historical GitHub login experiments include an existing `gh` CLI session and a GitHub App device flow. GitHub login alone is **not** a feature grant. P0 `identity/2` pairing is distinct from unverified port discovery, and P1 shows grants only with a pinned deterministic fixture. Loopback binding and browser session checks do not create OS process isolation or universal feature authorization. See [Security](SECURITY.md), [Interfaces](INTERFACES.md) and [Status](STATUS.md).

## Sources of direction and unresolved questions

- [Decisions](DECISIONS.md) distinguishes accepted architecture from proposed lifecycle details.
- [Independent review](REVIEW.md) preserves contrasting views, including [end-user notes](archive/END_USER_DESIGN_NOTES_20261025.md).
- [Thought Processor connection direction](https://github.com/Arameair/Thought-Processor/blob/experiment/identity-authority-registration-20261024/docs/discussions/2026-10-24-identity-authority-registration.md).
- [Thought-directed waves hypothesis](https://github.com/Arameair/Thought-engine/blob/experiment/thought-engine-v1-20261023/docs/THOUGHT_DIRECTED_WAVES_THEORY_2026-10-24.md).

**Do not treat links to experimental work as evidence of promotion or an implemented feature.**
