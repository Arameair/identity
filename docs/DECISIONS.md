# Design decisions and unresolved questions

**Reviewed:** 2026-10-26. Keep **owner-selected direction**, **independent review**, and **tested behavior** separate. This register is not proof that any new cross-service integrations work.

## Confirmed owner direction

| ID | Decision | Meaning | Evidence/status |
| --- | --- | --- | --- |
| D-01 | **Identity owns the organization and authority of the system. Each feature owns its actual work.** | Identity provides durable identities, ownership, accounts, storage governance and grants; no universal data plane | Owner selected in discussion, design only |
| D-02 | **Central organization, separate feature-owned storage.** | Identity knows where authorized data belongs; TASS, Thought Engine, Memory, etc. keep and process their own data | Owner confirmed 2026-10-24; not implemented |
| D-03 | **Independent feature registration.** | Independently running features request registration with Identity; Thought Processor discovers the approved catalog | Owner confirmed direction; authenticated protocol not implemented |
| D-04 | **No forced Thought or Memory.** | A person, business, service or companion can have an identity without cognitive features; a memory may trigger a thought but a thought is not memory | Owner confirmed |
| D-05 | **Manual promotion.** | New functional experiments start from Alpha; owner reviews and explicitly promotes; experiments cannot silently update Alpha/Stable/Main | Established workflow |
| D-06 | **Truthful diagnostic states.** | Installed, discovered, authenticated, authorized, enabled, ready, processed and persisted are distinct | Established principle |
| D-07 | **Open independent review.** | Invite Grok and other reviewers to challenge decisions; feedback stays attributed and unapproved until owner decides | Requested 2026-10-24; [Issue #14](https://github.com/Arameair/identity/issues/14) |
| D-08 | **Thought Processor is the human and administrator interface; Identity is the service authority behind it.** | First run, accounts, permissions, feature catalog, statuses and normal diagnostics belong in Thought Processor. Identity owns authoritative decisions and records, not a parallel user dashboard. Direct Identity tools may serve development/recovery only. | Owner correction 2026-10-25; **accepted architecture direction, integration not implemented** |

## October 26 conflict resolutions — owner confirmed

The authoritative cross-repository register is [Cross-repo decisions](CROSS_REPO_DECISIONS_2026-10-26.md). The following refine the older D-01–D-08 entries without claiming implementation:

- **D-09 — Flexible first-launch order:** Identity or Processor may start first. Both must reach the same authenticated, clean end state. Thought Processor remains the normal human/admin **interface**, regardless of process startup order.
- **D-10 — Distribution is not authorization:** Verified GitHub repository access may determine prototype **obtain/install eligibility** only. Identity grants separately govern which identity can operate a feature and access each particular source/action; receiving features enforce. Sources start with **zero grants**.
- **D-11 — Exit with consent:** Explicit Processor application exit asks **Shut down managed workspace (default)** or **Keep services running**. Track, gracefully stop and verify processes the system owns; no killing unrelated services, no automatic shutdown on a browser tab close. Crash recovery and exact lifecycle runner are open.
- **D-12 — Pair once; reconnect securely:** First connection requires operator-approved pairing; approved instances may automatically reconnect on later starts with securely stored scoped credentials and explicit revoke/unpair. Pairing does **not** confer permission to consume data.
- **D-13 — Offline policy v1: fail closed:** If Identity is absent or cannot validate a required grant, receiving features block new protected operations and queued protected dispatch. No cached allows or temporary offline grants in the initial prototype. A service can remain alive while its protected work is blocked.
- **D-14 — Memory later:** Memory is a wholly separate optional future feature. No Memory build or contract is within this first Processor prototype.
- **D-15 — Sequence:** Identity concept → Thought Processor system-wide UX now → Thought Engine next. No permanent selection between the `Thought` and `Thought-engine` experiments has been made.

**Evidence status:** These are owner-approved **design decisions only**. The code, authentication protocol, runtime tests and owner Alpha acceptance are not established by this list.

## Pending architecture choices

| ID | Question | Current direction | Status |
| --- | --- | --- | --- |
| I-01 | Which mechanism launches or keeps Identity available? | **Launch order is flexible (D-09).** Manual, supervised, background or hybrid startup still need a safe process-owner/recovery design | **OPEN mechanism; order settled** |
| I-02 | How does first-run establish administrator authority and protect keys across OSes? | **Either startup order is allowed**; Processor presents the normal human flow and Identity validates trust using a real authenticated contract | **OPEN implementation; order settled** |
| I-03 | How do unknown features prove enrollment/service identity? | Authenticated claim + administrator approval, not just port discovery | **OPEN** |
| I-04 | What is the feature-to-feature authorization and revocation contract? | Receiver enforces scoped grants and revocation; **v1 fails closed with no offline grace**. Exact verification and queued/in-flight revocation semantics still open | **OPEN protocol; outage policy settled** |
| I-05 | How are storage roots, encryption, backups and cross-OS migrations governed? | Identity-owned location references, isolated feature storage, no code checkout content | **OPEN** |
| I-06 | When and by whom may a GitHub repository be created? | Explicit scope approval, no automatic writes or deletions | **OPEN** |
| I-07 | How do owner-approved services start, stop and recover? | On explicit Processor exit ask whether to shut down managed workspace (**default**) or keep it running. Verify no orphaned owned processes; external services remain separate. Runner implementation unresolved | **OPEN mechanism; UX policy settled** |
| I-08 | How to migrate non-production experiments into durable identities? | Preserve existing IDs and backups with explicit owner actions | **OPEN** |
| I-09 | What constitutes a reproducible Alpha/Stable/Main acceptance? | Real tests and owner evidence, not model claims | **OPEN** |

## Unresolved reviewer input

The [end-user notes](archive/END_USER_DESIGN_NOTES_20261025.md) recommend *one folder, one process* and **no background service** for the first usable product. This is useful for a minimalist prototype but is **not accepted as a universal ecosystem constraint**. Separately owned storage **is** owner-confirmed. The background service versus manual startup choice remains open and is specifically [under independent review](https://github.com/Arameair/identity/issues/14).

The outside-user notes also propose a Thought Engine readiness section on the default **Identity** dashboard. That UI placement is superseded by **D-08**: if useful, the feature's status belongs in **Thought Processor**. This does not approve installing or requiring Thought Engine.

## Evidence and decision policy

When resolving an open item, record the date, options, proponent/reviewer, human decision, a reasoned trade-off, test results and any rollback plan. Preserve superseded advice rather than deleting it. New notes do not create an implied merge, release or permission grant.

Related: [Architecture](ARCHITECTURE.md), [First run](FIRST_RUN.md), [Storage](STORAGE.md), [Review](REVIEW.md), [Status](STATUS.md).