# Identity documentation

[Project home](../README.md) · [Current status](STATUS.md) · [Open design review](REVIEW.md)

Identity's governing principle:

> **Identity owns the organization and authority of the system. Each feature owns its actual work.**

## Understand the project

| Read | Why |
| --- | --- |
| [Vision](VISION.md) | Who Identity serves, and why it stays independent of optional features |
| [Architecture](ARCHITECTURE.md) | What Identity owns versus Thought Processor, Thought Engine, Memory and other features |
| [First-run experience](FIRST_RUN.md) | How a new installation and administrator should be initialized |
| [Storage ownership](STORAGE.md) | Clean, isolated feature data managed through Identity-owned references |
| [Security](SECURITY.md) | Permission and credential boundaries; current security gaps |
| [Interfaces](INTERFACES.md) | Existing limitations and proposed service/grant contracts |

## Plan and verify work

| Read | Why |
| --- | --- |
| [Status and branch ledger](STATUS.md) | What code is present, what's verified, what's still hypothetical |
| [Decision register](DECISIONS.md) | Approved direction versus open questions |
| [Roadmap](ROADMAP.md) | Owner-gated acceptance sequence, not promises of finished software |
| [Branch workflow](WORKFLOW.md) | Alpha → experiment → owner decision → Stable / Main |
| [Experiment template](EXPERIMENT_TEMPLATE.md) | Required evidence, failures and rollback |
| [AI collaboration](AI_COLLABORATION.md) | Human decision authority and AI assistance disclosure |

## Independent perspectives

- [End-user design notes](END_USER_DESIGN_NOTES.md) — original outside-user recommendations, retained even where they differ from the evolving design.
- [Independent review guide](REVIEW.md) and [open review issue #14](https://github.com/Arameair/identity/issues/14) — invite criticism, including Grok's perspective, with explicit attribution and testable alternatives.

**Important:** `main` is the documentation/release-facing lane, not evidence that its experimental integrations have been implemented or accepted. Start new functional experiments from the current `alpha` and ask the owner before promotion.