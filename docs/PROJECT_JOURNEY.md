# Identity — engineering case study

**Stage:** active prototype development · **Updated:** 2026-10-25 · **Source:** [Arameair/identity](https://github.com/Arameair/identity)

## Problem

A collection of independently developed applications—Thought Processor, Thought Engine, companion interfaces and future features—needs a consistent way to know **who an entity is, which capabilities exist, which accounts it is associated with, and which operations it may perform**. Hard-coding identities and permissions into every application would make the system harder to extend, audit, recover and hand to another user.

Identity is an exploration of a reusable, local-first trust and organization layer. It is not itself the AI reasoning engine, memory store, or required process supervisor.

## Constraints that shaped the work

- **Independent components:** each feature starts, stops, logs, and retains its own data without a mandatory monolithic controller.
- **New-machine onboarding:** a person should be able to try Identity on a computer without the author's Project Controller or personal GitHub credentials.
- **Visible truth:** detected services, self-reported identities, authenticated services and authorized operations must have different status labels.
- **Security boundaries:** local secrets and device codes must not be placed in GitHub commits, diagnostic copies or ordinary feature logs.
- **Evidence-driven iteration:** multiple experimental approaches compete, and only the owner explicitly approves promotion from Alpha to Stable/Main.

## Architecture and decisions

Identity is responsible for **durable identity records, associations, service registration concepts, access decisions and feature-owned storage references**. The Thought Processor is a separate operational interface that can show available features and runtime status. Individual features retain their own processing, logs and storage.

The distinction between **identity**, **installation**, **runtime instance**, **account authentication**, **feature availability**, and **permission to execute** is deliberate. A local HTTP response or visible GitHub repository must never silently become an authorization grant.

See [Identity-first model](IDENTITY_FIRST_MODEL.md), [Architecture](ARCHITECTURE.md), [Storage](STORAGE.md), and [Security](SECURITY.md).

## Iterations and evidence

| Period / lane | Work investigated | Evidence and caveat |
| --- | --- | --- |
| Oct 23 — local prototypes | Local identity records, reversible archival, GitHub CLI account checks and port discovery | Implementations exist on separate `experiment/*` branches; they do not establish a verified secure trust system |
| Oct 24 — system boundary | Identity-first startup, independent features, organized but feature-owned storage, proposed grants and registration | Architecture and acceptance documents; much of the contract remains **design only** |
| Oct 25 — Prototype B | Standalone Go application, local monitor/logging, mocked GitHub device flow, read-only Processor detection, process independence | Developer Linux runtime and fixture testing documented in [Prototype B evidence](https://github.com/Arameair/identity/blob/experiment/identity-home-claude-20261025/docs/HOME_TRIAL_EVIDENCE_CLAUDE.md); owner Windows and real GitHub results were outstanding in that report |
| Oct 25 — Alpha candidate | [`alpha-candidate-20261025`](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) adds a publisher-app configuration, diagnostic visibility and fixes for sign-in polling, stale codes, UI state and port conflicts | Fixes are **documented by the candidate**, not independently verified here; owner acceptance and promotion remain pending |

The core recent testing challenge is not simply to make two pages render. It is to demonstrate that **two actual programs can communicate without confusing discovery with trust, leaking credentials, or taking control of one another's lifecycle**.

## What a reviewer can inspect

1. **Runnable source and test organization** — [October 25 candidate](https://github.com/Arameair/identity/tree/alpha-candidate-20261025): Go entrypoint, internal application, GitHub device flow, local discovery, logbook, profiles, redaction, tests and Windows build script.
2. **Reproducible operator procedure** — [Candidate runbook](https://github.com/Arameair/identity/blob/alpha-candidate-20261025/AGENTS.md) and [new-user trial guide](TRY_THE_PROTOTYPE.md).
3. **Measured and unmeasured results separated** — [Prototype B evidence](https://github.com/Arameair/identity/blob/experiment/identity-home-claude-20261025/docs/HOME_TRIAL_EVIDENCE_CLAUDE.md), [home-trial acceptance](https://github.com/Arameair/identity/blob/experiment/identity-consolidated-home-trial-20261025/docs/HOME_TRIAL_ACCEPTANCE_2026-10-25.md), and [status ledger](STATUS.md).
4. **Reasoned tradeoffs and open decisions** — [Decision register](DECISIONS.md), [independent reviews](REVIEW.md), [roadmap](ROADMAP.md) and [branch workflow](WORKFLOW.md).
5. **Transparent collaboration** — [AI contribution and verification policy](AI_COLLABORATION.md).

## Engineering practices demonstrated or actively exercised

- Go CLI/server prototypes and local HTTP interfaces.
- Explicit separation of authentication, service discovery and authorization.
- Device-flow integration design with no embedded client secret.
- Separate process lifecycles, persistent local profiles and observable diagnostics.
- Redaction tests and fixture-vs-live labeling.
- Versioned experiments, comparison criteria, rollback instructions and human promotion gates.

These describe the **work evidenced by the repositories**, not a claim of production-grade security or complete feature authorization.

## Gaps and next acceptance point

**Not delivered as a verified service:** authenticated Identity-to-Processor pairing, durable permission issuance and receiving-service enforcement, secure administrator recovery, user feature-request submission, and a published general release. The `main` branch contains documentation only; its design narrative must not be mistaken for executable capability.

The next useful milestone is a **real clean Windows install** on a separate user profile: build from the candidate, complete/decline real GitHub device sign-in, run Processor alongside it, verify both stop/restart independently, capture redacted diagnostics and record the owner's pass/fail decision. Only after evidence and explicit approval should executable code be promoted.

## Authorship and scope

This project is human-directed and developed in collaboration with AI assistants. Documentation, architecture critique and some experimental implementations were created with AI assistance and reviewed through human-led iteration. Attribution does not imply endorsement or independent verification by any AI provider.

**Read next:** [README](../README.md) · [Try the prototype](TRY_THE_PROTOTYPE.md) · [Status](STATUS.md) · [AI collaboration](AI_COLLABORATION.md)
