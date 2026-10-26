# Identity

**Identity is the independent identity and authorization service for the Thought Processor ecosystem.** It maintains durable identities, service relationships, registrations, and permission decisions. Each feature keeps its own execution and data.

> **Status — October 26, 2026:** Active experimental project, **not a released or audited security platform**. `main` is the public documentation entry point. Runnable Go prototypes and their tests are on separate candidate/experiment branches; **none has been promoted to a functional `alpha` or `stable` release**.

## What the system is meant to do

A person opens **[Thought Processor](https://github.com/Arameair/Thought-Processor)** to set up and manage the system. Processor presents identities, accounts, available features, approval requests, connections, and diagnostics. **Identity** makes and records authoritative identity/permission decisions behind that interface. **Independent features** (Thought Engine, Memory, communication tools, etc.) perform their own work and **enforce** permissions applicable to them.

This separation matters:

- **Discovered** does not mean authenticated; **authenticated** does not mean authorized.
- GitHub login or repository visibility does **not automatically grant** every feature.
- Installing a feature is not the same authorization as starting it or granting access to its data.
- Processor closing must not silently shut down Identity or other independently running features.
- Thought Engine and Memory are optional. Identity does not think, run models, or store their data.

[Architecture and responsibilities](docs/ARCHITECTURE.md) · [Recorded decisions and open questions](docs/DECISIONS.md)

## What exists today

| Area | Current evidence |
| --- | --- |
| **Identity candidate** | [October 25 Go candidate](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) includes a standalone local UI, GitHub device sign-in, diagnostics, and read-only Processor detection. Its **Identity-first browser setup is a developer experiment**, not the intended end-user flow. |
| **Pairing / Engine install experiment** | [October 26 experimental branch](https://github.com/Arameair/identity/tree/experiment/engine-access-pairing-20261026) pairs Processor with Identity and tests a **temporary** repository-visibility-based Engine installation rule. [Evidence](https://github.com/Arameair/identity/blob/experiment/engine-access-pairing-20261026/docs/ENGINE_INSTALL_DEMO_20261026.md) uses a **GitHub test fixture**; real GitHub, a published release, and Windows acceptance are unverified. |
| **Production-grade authorization** | **Not established.** Authenticated administrator management through Processor, independent feature enrollment, scoped grants, revocation enforcement, and secure recovery remain open. |
| **Release** | **None approved.** `main` and `alpha` contain documentation, not a runnable product. |

**Do not use experimental pairing or visibility checks as protection for sensitive or production data.** See [security boundaries](docs/SECURITY.md) and [evidence-based status](docs/STATUS.md).

## Start here

- **Understand the project:** [Architecture](docs/ARCHITECTURE.md) and [decisions](docs/DECISIONS.md).
- **Find what was actually tested:** [Status and branch ledger](docs/STATUS.md).
- **See how a new user should interact with it:** [Processor-led first run](docs/FIRST_RUN.md).
- **Run a developer-only candidate:** [Experimental subsystem trial](docs/TRY_THE_PROTOTYPE.md). It is **not** end-user installation guidance.
- **Review the work as a portfolio project:** [Engineering case study](docs/PROJECT_JOURNEY.md).
- **Find remaining reference documents:** [Documentation index](docs/README.md).

## Development rules

`alpha` is the owner-controlled experimental baseline; new functional experiments branch from `alpha`. `stable` is for a verified owner-accepted snapshot, and `main` is the public-facing documentation/release lane. No experimental build advances automatically. **Only the owner explicitly approves promotion.** [Workflow and evidence requirements](docs/WORKFLOW.md).

This is human-directed, AI-assisted work. Design proposals, mock-service passes, executable tests, live GitHub checks, and owner acceptance are distinct. See [collaboration policy](docs/AI_COLLABORATION.md). Never publish device codes, access tokens, cookies, or private local opening URLs.
