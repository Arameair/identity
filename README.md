# Identity

**The identity, organization and authorization foundation for independent software features.**

Identity gives people, companions, projects, businesses and services a persistent place in a modular software ecosystem—without requiring any of them to have a chatbot, an AI model, a thought engine or a memory system.

> **Identity owns the organization and authority of the system. Each feature owns its actual work.**

**Project stage:** Active design and experimental implementation. This `main` branch contains **documentation only**. There is no owner-approved production release or verified multi-service security implementation. [See evidence and limitations](docs/STATUS.md).

## The idea

An installation needs to know **who or what exists**, **who owns it**, **which features are registered**, **where each feature may keep data**, and **what connections are authorized**. Identity is the common authority for those decisions, not the system through which every log, thought or message must flow.

```text
                    Owner / administrator
                            |
                            v
                      +-----------+
                      | Identity  |
                      +-----------+
             IDs / registry / accounts / grants
                      /       |       \
                     /        |        \
                    v         v         v
                  TASS    Thought Engine   Other features
                   |           |               |
             owned data    owned data       owned data
                   +-----------+---------------+
                         direct authorized
                         service connections

  Thought Processor: separate feature discovery and inspection surface.
  Memory: optional independent feature, not required for thoughts.
```

**Independence is a requirement:** A feature can be developed, installed, tested, updated, disabled or restarted without requiring every other feature to follow. Identity must work even when the optional feature count is **zero**.

## What Identity does and does not do

| Identity is responsible for | Individual features remain responsible for |
| --- | --- |
| Durable installation, administrator, entity and service IDs | Their own executable lifecycle and internal behavior |
| Registry of available features and authenticated instances | Their own databases, event streams and logs |
| Account references, scoped permissions, grant/revocation policy | Enforcing grants at their own API boundaries |
| Organizing storage ownership, paths and approved access | Creating, interpreting, retaining and deleting their own content |
| Recording accurate identity, connection and authorization status | Running models, producing thoughts and retrieving memories |

Identity is **not** a thought engine, memory database, general-purpose process launcher, or mandatory proxy for every service request.

## First run on a new computer

The intended experience is deliberately small:

1. **Create or restore** an installation and administrator using durable, separate identifiers.
2. **Initialize protected local authority records** and visible, organized storage ownership.
3. **Open Identity alone**, with a truthful dashboard: local status, connected accounts and registered features.
4. **Add independent features when wanted.** Compatible features can request registration with Identity; discovery is never automatic permission.
5. **Connect GitHub or other providers optionally**, through separately approved authorization—not as a condition of creating a local identity.

Read the [first-run proposal](docs/FIRST_RUN.md), [storage model](docs/STORAGE.md), and [architecture](docs/ARCHITECTURE.md). Whether Identity should launch as an opt-in lightweight background service remains [open for independent review](https://github.com/Arameair/identity/issues/14).

## What exists today

A standalone **Go Identity manager exists on experimental branches**, with local ID cards, durable storage, reversible archive/restore, read-only verification using an existing GitHub CLI session, and limited local service discovery. Those implementations are **not merged into `main`**, and their documented tests are not proof of a production-grade identity or grant provider.

| Lane | Purpose | Current interpretation |
| --- | --- | --- |
| [Alpha](https://github.com/Arameair/identity/tree/alpha) | Owner-controlled starting point for new experiments | Documentation-only baseline; no production service |
| [Experiments](https://github.com/Arameair/identity/branches) | Isolated prototypes and design comparisons | Not approved merely because they compile |
| Stable | Known-good owner-accepted snapshot | No verified Stable branch as of 2026-10-24 |
| **Main** | Welcoming documentation and later approved releases | Documentation only |

For reproducible candidate builds, limitations and the exact ledger, see [Status](docs/STATUS.md). **New functional experiments begin from Alpha; promotion is always the owner's decision.**

## Documentation

[**Start with the documentation index →**](docs/README.md)

| Topic | Guide |
| --- | --- |
| Direction and system boundaries | [Vision](docs/VISION.md) · [Architecture](docs/ARCHITECTURE.md) |
| Getting started and storing data | [First run](docs/FIRST_RUN.md) · [Storage](docs/STORAGE.md) |
| Trust and service contracts | [Security](docs/SECURITY.md) · [Interfaces](docs/INTERFACES.md) |
| Project progress | [Status](docs/STATUS.md) · [Roadmap](docs/ROADMAP.md) · [Decisions](docs/DECISIONS.md) |
| Development and review | [Workflow](docs/WORKFLOW.md) · [AI collaboration](docs/AI_COLLABORATION.md) · [Independent review](docs/REVIEW.md) |

## Open to critique, including from Grok

Independent perspectives are welcome. The [end-user design notes](docs/END_USER_DESIGN_NOTES.md) remain available in their original form, and [review issue #14](https://github.com/Arameair/identity/issues/14) explicitly requests counterarguments about new-machine setup, storage isolation, service registration and background operation.

**Reviews are input, not automatic decisions.** The human owner chooses direction; real tests determine whether an implementation works. A GitHub issue does not itself contact an external model. See [Review guide](docs/REVIEW.md).

## AI collaboration and verification

This is a **human-directed project developed with AI assistance**, including architectural review, documentation, experiments and testing. AI-created claims are not independently verified merely because they appear in a commit. Contributors should keep attribution, implemented behavior, test evidence and owner approval separate. See [AI collaboration and verification](docs/AI_COLLABORATION.md).

No repository, even a private one, should contain passwords, provider tokens, session cookies or other secrets. No port-open indicator, green status label or local ID card alone proves a privileged connection.

---

**Design principle:** Centralize *identity and authority*. Decentralize *execution and data ownership*.
