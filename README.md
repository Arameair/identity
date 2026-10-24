# Identity

**A durable identity, trust and relationship foundation for independent software features.**

Identity is the proposed system-level authority that knows **which participants exist, what they can do, how they relate to one another, and what they are permitted to access**. Participants can be companions, people, businesses, projects **and independently developed features themselves**.

> **Project status — 24 October 2026:** **Design and active experiments; not a production-ready identity/security service.** The `main` branch is documentation. Separate experimental branches contain Go-based local identity prototypes and read-only local service discovery. An authenticated cross-service pairing/grant system has **not** been verified. [Evidence and limitations](docs/STATUS.md).

## Purpose

We want an extensible system in which TASS, Memory, Thought Engine, ScreenBridge, AI workers and future capabilities each have an identity and operate independently. Identity supplies the enduring facts and permission decisions that allow these components to collaborate **without bundling them into one application**.

Identity is the **heart of the system** in the sense of *system identity, trust, relationship and policy knowledge*. It is **not** a substitute for the cognitive Thought Engine, the Memory service or the Thought Processor's operational dashboard.

## System architecture

```text
                   IDENTITY
      participants · trusted relationships
         scoped grants · permissions
                    |
        +-----------+-----------+
        |                       |
 THOUGHT PROCESSOR         INDEPENDENT FEATURES
 discover · connect         Thought Engine · Memory
 status · diagnostics        Vision · Hearing · Workers
        |                       |
        +---- verified links ---+
                     |
     Direct, authorized feature operations
       (no mandatory inference proxy)
```

**Important distinctions:** A feature's persistent identity, its advertised capability, the software installation, and the currently running process are *different things*. Discovery is not service authentication, pairing is not a feature grant, and enabling a feature is not proof it is running.

[Read the Identity-first architecture](docs/IDENTITY_FIRST_MODEL.md) · [Technical architecture](docs/ARCHITECTURE.md)

## Responsibilities and boundaries

| Identity **owns or should own** | Identity **does not own** |
| --- | --- |
| Durable entity and feature identities | Thoughts, prompts, model responses or raw screen captures |
| Trusted service pairing and revocation | The lifecycle of all independently running features |
| Account associations and protected authorization | Forwarding every AI inference request |
| Scope- and entity-specific capability grants | Declaring a mere detected port to be trusted |
| Identity relationships and potentially stable purpose metadata | Replacing the owner's preferred development tools |

The trust system is **planned**, not yet operational. Even after Identity issues permission, each receiving feature must validate the authorization it accepts.

**Storage and registration:** Identity is intended to organize ownership, feature registration and approved storage locations; **each independently running feature keeps its own logs, data and processing state**. Features request trusted registration with Identity; Thought Processor discovers approved catalog entries and reconciles them with the instances actually running. The exact protocol remains unimplemented.

## Current implementation evidence

The unapproved [controller-discovery experiment](https://github.com/Arameair/identity/tree/experiment/identity-controller-port-discovery-20261023) builds on earlier standalone Go prototypes with local persistent identity cards, reversible archive/restore, existing GitHub CLI session verification and read-only local service discovery.

These are **experimental implementations**, not a released secure identity authority. A GitHub CLI sign-in is not Identity-managed OAuth; local port discovery is not pairing; and experimental ability to create an identity card does not prove cross-application grant enforcement. Consult the [status ledger](docs/STATUS.md) for evidence and acceptance gaps.

## Documentation

| Start here | Purpose |
| --- | --- |
| **[Identity-first system model](docs/IDENTITY_FIRST_MODEL.md)** | Current vision, identity-vs-feature-vs-runtime definitions, choices still open |
| [Architecture](docs/ARCHITECTURE.md) | Responsibility boundaries, discovery and proposed data/control flow |
| [First run](docs/FIRST_RUN.md) | Clean installation and admin bootstrap proposals |
| [Storage and ownership](docs/STORAGE.md) | Central organization, independently owned feature data |
| [Independent reviews](docs/REVIEW.md) | Contrasting design opinions and unresolved tradeoffs |
| [Security](docs/SECURITY.md) | Accounts, secrets, authentication and present limitations |
| [Interface proposals](docs/INTERFACES.md) | Draft identity, grant and service contracts—not live APIs |
| [Status and evidence](docs/STATUS.md) | Known branches, test reports, verified vs unverified behavior |
| [Roadmap](docs/ROADMAP.md) | Small, independently testable build gates |
| [Decisions and open questions](docs/DECISIONS.md) | Owner direction versus unsettled design choices |
| [Development workflow](docs/WORKFLOW.md) | Alpha, Stable, Main, experiments and explicit promotions |
| [AI collaboration](docs/AI_COLLABORATION.md) | Authorship, disclosure and verification |

The [original extended design foundation](https://github.com/Arameair/identity/tree/experiment/design-foundation-20261023) and [recent Identity/Processor integration notes](https://github.com/Arameair/identity/tree/experiment/processor-identity-connection-notes-20261023) remain available as historical proposals. They were **not merged into executable Alpha** by this documentation cleanup.

## Design questions and independent review

Our working **Identity–Experience hypothesis** asks whether stable identity purpose together with selective retrieved memory improves the consistency and usefulness of a companion over time. It is **unproven** and independent of Identity's practical value for access control.

- [Hypothesis, experiment matrix and Grok critique](https://github.com/Arameair/Thought-Processor/blob/main/docs/IDENTITY_EXPERIENCE_RESEARCH.md)
- [Open architecture review — Grok and other perspectives welcome](https://github.com/Arameair/Thought-Processor/issues/7)

Unsettled: hierarchical vs relationship-based feature ownership, changing goals/self-model, first-admin recovery, secure grant format, per-install vs per-feature identities, and whether every feature requires a private GitHub repo.

## Development and releases

| Branch | Meaning |
| --- | --- |
| `main` | This documentation / eventual explicitly approved public release |
| `alpha` | Owner-controlled development baseline |
| `stable` | Reserved for an owner-accepted verified snapshot; **no stable branch was present at the latest status review** |
| `experiment/*` | Isolated work, **always branched from Alpha**; evidence first, promotion by explicit owner action only |

This cleanup changes **documentation on main only**, not Alpha or running software. The owner's [Project Controller](https://github.com/Arameair/Project-controller) can help develop and test versions but is **not required** to install or use Identity.

---

**AI collaboration disclosure:** This is human-directed work developed with AI assistance, including ChatGPT and owner-supplied critical feedback attributed to Grok. All AI opinions are review inputs, not guarantees of correctness. [Read the disclosure](docs/AI_COLLABORATION.md).

**Security:** Never commit passwords, tokens, private keys or secret session materials to any repo, including private identity repos. A successful HTTP response is not proof of authorization.
