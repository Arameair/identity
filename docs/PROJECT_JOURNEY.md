# Identity — engineering case study

**Stage:** working P0 baseline and experimental P1 · **Updated:** 2026-10-27 · **Source:** [Arameair/identity](https://github.com/Arameair/identity)

## Problem

A collection of independently developed applications—Thought Processor, Thought Engine, companion interfaces and future features—needs a consistent way to know **who an entity is, which capabilities exist, which accounts it is associated with, and which operations it may perform**. Hard-coding identities and permissions into every application would make the system harder to extend, audit, recover and hand to another user.

Identity is an exploration of a reusable, local-first trust and organization layer. It is not itself the AI reasoning engine, memory store, or required process supervisor.

## Constraints that shaped the work

- **Independent components:** each feature starts, stops, logs, and retains its own data without a mandatory monolithic controller.
- **New-machine onboarding:** a person should use **Thought Processor as the primary UI**, with Identity providing the independently running identity/authorization backend. Neither should require the author's Project Controller or personal credentials.
- **Visible truth:** detected services, self-reported identities, authenticated services and authorized operations must have different status labels.
- **Security boundaries:** local secrets and device codes must not be placed in GitHub commits, diagnostic copies or ordinary feature logs.
- **Evidence-driven iteration:** multiple experimental approaches compete, and only the owner explicitly approves promotion from Alpha to Stable/Main.

## Architecture and decisions

Identity is responsible for **durable identity records, associations, service registration concepts, access decisions and feature-owned storage references**. **Thought Processor is the intended main application for people and administrators**, providing first-run, account/identity management, feature choices, approval prompts and operational diagnostics. The authenticated P0 pairing and experimental P1 grant flow request Identity's decisions rather than inventing grants; full real-feature integration still needs testing. Individual features retain their own processing, logs and storage.

The distinction between **identity**, **installation**, **runtime instance**, **account authentication**, **feature availability**, and **permission to execute** is deliberate. A local HTTP response or visible GitHub repository must never silently become an authorization grant.

See [Identity-first model](IDENTITY_FIRST_MODEL.md), [Architecture](ARCHITECTURE.md), [Storage](STORAGE.md), and [Security](SECURITY.md).

## Iterations and evidence

| Period / lane | Work investigated | Evidence and caveat |
| --- | --- | --- |
| Oct 23 — local prototypes | Local identity records, reversible archival, GitHub CLI account checks and port discovery | Implementations exist on separate `experiment/*` branches; they do not establish a verified secure trust system |
| Oct 24 — system boundary | Earlier Identity-first setup proposals, independent features, organized but feature-owned storage, proposed grants and registration | Historical UX proposals are superseded by the **Oct 25 Processor-first user interface decision**; much of the security contract remains **design only** |
| Oct 25 — Prototype B | Standalone Go application, local monitor/logging, mocked GitHub device flow, read-only Processor detection, process independence | Developer Linux runtime and fixture testing documented in [Prototype B evidence](https://github.com/Arameair/identity/blob/experiment/identity-home-claude-20261025/docs/HOME_TRIAL_EVIDENCE_CLAUDE.md); owner Windows and real GitHub results were outstanding in that report |
| Oct 25 — Alpha candidate | [`alpha-candidate-20261025`](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) adds publisher-app configuration, diagnostics and fixes for sign-in polling, stale codes, UI state and port conflicts | Historical experimental lane; candidate claims require test evidence |
| Oct 26 — P0 Alpha | [`alpha` at `c8c0a54`](https://github.com/Arameair/identity/tree/alpha) establishes the owner-promoted working Identity/Processor pairing baseline | Signed `identity/2` exchanges and P0 owner trial; **not** a feature-grant or production release |
| Oct 26 — P1 build and adversarial cleanup | [P1 cleanup Identity code](https://github.com/Arameair/identity/tree/experiment/p1-cleanup-claude-20261026) adds Admin, grant, installation, approval and live check mechanics | [Claude's cleanup report](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/P1_CLEANUP_REPORT_20261026.md) reports fresh-clone 81/81 integration and 15/15 adversarial tests against a pinned fixture, following independent QA; owner acceptance pending |
| Oct 27 — integration design review | Identity-only canonical IDs; Processor coordinates; Engine/Worker must independently enforce Identity decisions; recommendations do not confer authority | [Confirmed cross-repo decisions](https://github.com/Arameair/Thought-Processor/issues/20); not implemented by the actual Engine/Worker yet |

**Design correction (Oct 25):** Early prototypes inverted the presentation boundary: users were asked to open Identity and detect Thought Processor. The intended product is the opposite: **open Processor and work through it; Identity serves verified authority data behind the UI**. The old wizard remains a legitimate backend diagnostic experiment, but is not the desired final onboarding design.

The core recent testing challenge is not simply to make two pages render. It is to demonstrate that **two actual programs can communicate without confusing discovery with trust, leaking credentials, or taking control of one another's lifecycle**.

## What a reviewer can inspect

1. **Runnable source and test organization** — [P0 Alpha](https://github.com/Arameair/identity/tree/alpha) and [P1 cleanup experiment](https://github.com/Arameair/identity/tree/experiment/p1-cleanup-claude-20261026): Go server, real authenticated pairing, authority records, tests and developer runbooks.
2. **Reproducible operator procedure** — [matched P1 START HERE](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/START_HERE.md) and [older historical trial](TRY_THE_PROTOTYPE.md).
3. **Reported and owner-observed results separated** — [P1 developer test report](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/P1_CLEANUP_REPORT_20261026.md), [adversarial review](https://github.com/Arameair/Thought-Processor/issues/22), [historical home trials](https://github.com/Arameair/identity/issues/18), and [status ledger](STATUS.md).
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

**Delivered in experimental scope:** P0 authenticated Processor–Identity pairing in Alpha; P1 Admin/installation identity, grants and live enforcement against a pinned deterministic fixture. **Not delivered or owner-accepted as a complete secure service:** production administrator recovery, OS isolation, general third-party trust, real Engine/Worker enforcement, secure new-user distribution and a published release. The `main` branch contains documentation only; the executable branches are separate.

The next useful milestone is the **owner's hands-on P1 cleanup test** with matched experimental branches and disposable profiles: establish the Admin Identity through GitHub, install the pinned fixture with no grants, exercise deny → grant → allow → revoke → deny, take Identity offline, and check durable IDs after restart. Then integrate the actual Fast AI Worker and Thought Engine with authenticated receiving-service enforcement. Only after evidence and explicit owner approval should executable code be promoted.

## Authorship and scope

This project is human-directed and developed in collaboration with AI assistants. Documentation, architecture critique and some experimental implementations were created with AI assistance and reviewed through human-led iteration. Attribution does not imply endorsement or independent verification by any AI provider.

**Read next:** [README](../README.md) · [P1 trial](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/START_HERE.md) · [Status](STATUS.md) · [Credits](CREDITS.md) · [AI collaboration](AI_COLLABORATION.md)
