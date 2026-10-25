# Design decisions and unresolved questions

**Reviewed:** 2026-10-25. Keep **owner-selected direction**, **independent review**, and **tested behavior** separate. This register is not proof that any new cross-service integrations work.

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

## Pending architecture choices

| ID | Question | Current direction | Status |
| --- | --- | --- | --- |
| I-01 | Should Identity launch as a background service? | Compare opt-in lightweight service, manual UI/service and hybrid; no automatic installation | **OPEN** |
| I-02 | How does **Processor-led** first-run establish administrator authority and protect keys across OSes? | Processor presents the flow; Identity validates durable trust using an authenticated protocol; verify OS credential facilities before any success claim | **OPEN** |
| I-03 | How do unknown features prove enrollment/service identity? | Authenticated claim + administrator approval, not just port discovery | **OPEN** |
| I-04 | What is the feature-to-feature authorization and revocation contract? | Receiver enforces scoped grants, expiry/revocation and bounded offline policy | **OPEN** |
| I-05 | How are storage roots, encryption, backups and cross-OS migrations governed? | Identity-owned location references, isolated feature storage, no code checkout content | **OPEN** |
| I-06 | When and by whom may a GitHub repository be created? | Explicit scope approval, no automatic writes or deletions | **OPEN** |
| I-07 | How do owner-approved services restart and reconnect without Processor? | Independent lifecycle and accurate status; installer/launcher details undecided | **OPEN** |
| I-08 | How to migrate non-production experiments into durable identities? | Preserve existing IDs and backups with explicit owner actions | **OPEN** |
| I-09 | What constitutes a reproducible Alpha/Stable/Main acceptance? | Real tests and owner evidence, not model claims | **OPEN** |

## Unresolved reviewer input

The [end-user notes](END_USER_DESIGN_NOTES.md) recommend *one folder, one process* and **no background service** for the first usable product. This is useful for a minimalist prototype but is **not accepted as a universal ecosystem constraint**. Separately owned storage **is** owner-confirmed. The background service versus manual startup choice remains open and is specifically [under independent review](https://github.com/Arameair/identity/issues/14).

The outside-user notes also propose a Thought Engine readiness section on the default **Identity** dashboard. That UI placement is superseded by **D-08**: if useful, the feature's status belongs in **Thought Processor**. This does not approve installing or requiring Thought Engine.

## Evidence and decision policy

When resolving an open item, record the date, options, proponent/reviewer, human decision, a reasoned trade-off, test results and any rollback plan. Preserve superseded advice rather than deleting it. New notes do not create an implied merge, release or permission grant.

Related: [Architecture](ARCHITECTURE.md), [First run](FIRST_RUN.md), [Storage](STORAGE.md), [Review](REVIEW.md), [Status](STATUS.md).