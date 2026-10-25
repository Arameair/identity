# Identity documentation

**Updated:** October 25, 2026 · [Project home](../README.md) · [Prototype status](STATUS.md)

**Guiding principle:** Identity organizes authority and relationships. Each independent feature owns its actual work and data. This repository documents both a **longer-term identity/security architecture** and a **working-in-progress local Go prototype**. Do not confuse the two.

## New to the project?

| Start here | What you will learn |
| --- | --- |
| [Main project overview](../README.md) | Problem, system boundaries, current progress and where to navigate |
| **[Try the experimental application](TRY_THE_PROTOTYPE.md)** | Git / Go / PowerShell walkthrough for a Windows tester; optional Processor pairing, expected states and evidence |
| **[Engineering case study](PROJECT_JOURNEY.md)** | Problem statement, constraints, design tradeoffs, iterations, results and outstanding gaps |
| [Status and branch ledger](STATUS.md) | Precisely what has source code, fixtures, real-runtime evidence or owner acceptance |

## Architecture and decisions

| Document | Purpose |
| --- | --- |
| [Vision](VISION.md) | Why Identity is separate from thought, memory and other optional services |
| [Identity-first model](IDENTITY_FIRST_MODEL.md) | Entity, feature, service and runtime distinctions |
| [Architecture](ARCHITECTURE.md) | Identity/Processor/feature responsibilities and conceptual integration flow |
| [First-run target](FIRST_RUN.md) | Planned secure administrator, restore and zero-feature experience; **not all implemented** |
| [Storage governance](STORAGE.md) | Centrally organized references, independently owned feature data |
| [Security](SECURITY.md) | Actual prototype limitations, credentials, authentication and missing grant enforcement |
| [Proposed interfaces](INTERFACES.md) | Forward-looking service enrollment and authorization contracts, **not live APIs** |
| [Decision register](DECISIONS.md) | Owner-selected architectural direction and unresolved choices |
| [Independent review](REVIEW.md) | Contrasting opinions and review methods |

## Engineering workflow and evidence

| Document | Purpose |
| --- | --- |
| [Roadmap](ROADMAP.md) | Small acceptance gates and measurable next step |
| [Workflow](WORKFLOW.md) | Start functional experiments from Alpha; owner controls promotion |
| [Experiment template](EXPERIMENT_TEMPLATE.md) | Version, environment, run evidence, failure, side effects and rollback report |
| [AI collaboration](AI_COLLABORATION.md) | Honest authorship and source/test verification |
| [Original user-design critique](END_USER_DESIGN_NOTES.md) | Historical independent proposal, not silently promoted into a requirement |

## Primary source and trial evidence

- [October 25 Identity Alpha candidate](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) — runnable candidate with code, tests and its own [operator runbook](https://github.com/Arameair/identity/blob/alpha-candidate-20261025/AGENTS.md). **Promotion pending.**
- [Claude Prototype B evidence](https://github.com/Arameair/identity/blob/experiment/identity-home-claude-20261025/docs/HOME_TRIAL_EVIDENCE_CLAUDE.md) — development-Linux process tests, separately marked GitHub fixtures and explicit missing owner Windows test.
- [Owner home-trial acceptance card](https://github.com/Arameair/identity/blob/experiment/identity-consolidated-home-trial-20261025/docs/HOME_TRIAL_ACCEPTANCE_2026-10-25.md) — what must be observed before calling the work accepted.
- [Open first-run / storage review](https://github.com/Arameair/identity/issues/14) — record critique and alternatives without treating comments as decisions.

**Branch note:** `main` currently contains documentation rather than the prototype executable. Functional work belongs in isolated branches based on `alpha`; no documentation update approves code or moves a release lane.
