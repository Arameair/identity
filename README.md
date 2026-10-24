# Identity

**A persistent identity and access foundation for independent software features.**

Welcome to **Identity** — the project for defining **who or what an entity is**, preserving that identity over time, and controlling which external accounts and optional capabilities it may use.

An entity might be an AI companion, person, business, project, or system service. Identity is intended to support all of them **without requiring a personality, thinking engine, or any particular feature**.

## AI collaboration and authorship

**Identity is developed through collaboration between its human owner and AI assistants.** The owner determines the goals, priorities, requirements, and whether changes are accepted. AI assistants support architecture discussions, documentation, experimental code, testing, and review.

AI-produced documentation or code is **not independently verified merely because it is committed**. Claims of working behavior must be supported by actual tests and observed results.

See [AI collaboration and verification](docs/AI_COLLABORATION.md).

> **Current status — ACTIVE EXPERIMENTATION (2026-10-23).** This `main` branch contains **project documentation, not a production-ready application**. Runnable Go code exists on unapproved experimental branches. Local identity management and GitHub CLI verification have experimental implementations, while official in-app provider authorization, cross-service feature grants, and production-grade security remain unimplemented. See [current status and evidence](docs/STATUS.md).

## What Identity is responsible for

- **Identity cards:** Give different kinds of entities durable, unique IDs and non-secret records that survive restarts.
- **Account management:** Provide a controlled home for authorized external connections, beginning with GitHub.
- **Access and permissions:** Define and eventually broker limited actions for a specific identity, with consent and revocation.
- **Optional features:** Record which capabilities are available, enabled, and authorized for each identity independently.
- **Continuity:** Preserve identity and configuration as applications, models, branches, and machines change.

**Identity is not a chatbot, thought engine, personality system, memory database, or general-purpose process controller.** Specialized components remain independent and enforce their own authorized boundaries.

## How it fits together

```text
               Owner / administrator
                        |
                        v
                    Identity
          stable IDs / accounts / grants
                        |
           +------------+------------+
           |            |            |
         TASS        Business      Service
      companion      identity     identity
           |            |            |
           +-- only explicitly approved features --+
                        |
                 Thought / Memory /
                 Communication / etc.
```

This is the **target architecture**, not a claim that all integrations are already connected.

**Example:** TASS can exist as an identity and later receive an approved Thought Processing grant. A separate business identity remains valid without Thought Processing. Turning off Thought must not stop an unrelated feature or erase the identity.

## What's available today

The [controller-discovery experimental branch](https://github.com/Arameair/identity/tree/experiment/identity-controller-port-discovery-20261023) contains a standalone Go application with:

- Local identity creation, persistence, reversible archiving and restoration.
- Verification of an existing **official GitHub CLI** login and read-only association with an existing private repository.
- Read-only discovery of a local Thought Controller service, without taking over its process.

**Important limits:** These are experimental capabilities. The GitHub CLI session is not Identity-managed OAuth or a credential broker for other applications. A discovered service is **not** an authenticated service. The experiment does not create, modify or delete repositories, issue real feature grants, or implement a secure service handshake. Owner acceptance results are tracked separately.

## Start here

| Document | What it explains |
| --- | --- |
| [Current state and branch ledger](docs/STATUS.md) | Which branches and capabilities exist, and what was actually verified? |
| [Vision and principles](docs/VISION.md) | Why Identity exists, what it should offer, and what it should not do |
| [Architecture and ownership](docs/ARCHITECTURE.md) | How Identity relates to Thought, Thought Processor and independent features |
| [Accounts and security](docs/SECURITY.md) | Current GitHub CLI approach, intended provider authorization, risks and limits |
| [Interfaces and contracts](docs/INTERFACES.md) | Proposed identity, grant, account, and service integration boundaries |
| [Roadmap and acceptance gates](docs/ROADMAP.md) | The next independently testable milestones |
| [Decisions and open questions](docs/DECISIONS.md) | Owner direction, proposals requiring proof, and unresolved choices |
| [Branch and promotion workflow](docs/WORKFLOW.md) | How an experiment starts from Alpha and becomes an approved version |
| [Experiment report template](docs/EXPERIMENT_TEMPLATE.md) | Required commits, tests, failures, evidence and rollback |
| [AI collaboration and verification](docs/AI_COLLABORATION.md) | Human direction, AI assistance, attribution and verification standards |

For the original comprehensive design notes, see the [design foundation](https://github.com/Arameair/identity/tree/experiment/design-foundation-20261023). Those documents are proposals, **not automatically approved or implemented behavior**.

## Branch policy

| Branch | Purpose | Approval |
| --- | --- | --- |
| `main` | Professional project introduction and, later, explicitly approved releases | Human owner |
| `stable` | Known-good, owner-approved snapshot **when one exists** | Human owner; no automatic update |
| `alpha` | Owner-controlled integration baseline | Human owner |
| `experiment/<topic>-YYYYMMDD` | Isolated prototypes, **always based on the current Alpha** | Tested and compared before any promotion |

As of this review, **there is no `stable` branch**. Alpha remains the owner-controlled baseline. Improvements to this README or these notes do not promote experimental code.

Every functional experiment should preserve its starting Alpha SHA, resulting full SHA, test results, runtime evidence, limitations, rollback details and explicit owner decision. Use a GitHub issue for the goal.

## Related work

- [Thought](https://github.com/Arameair/Thought) and [Thought-engine](https://github.com/Arameair/Thought-engine) — separate experimental approaches to thought processing; no canonical integration is implied.
- [Thought-Processor](https://github.com/Arameair/Thought-Processor) — independent operator control point for the optional Thought Processing feature.
- [Thought-Controller](https://github.com/Arameair/Thought-Controller) — related dashboard and controller experiments.
- [Project Controller](https://github.com/Arameair/Project-controller) — independent developer tool for building, starting, stopping, and observing project versions.

**Owner acceptance rule:** A successful build, green test, or HTTP 200 only proves what was directly tested. Do not claim an account is securely connected, a feature is authorized, or a separate service exchanged real data without matching evidence.

**Security baseline:** Never commit credentials or secrets to any identity repository; never mistake a private GitHub repo, a local port, or a browser UI state for authorization.
