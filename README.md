# Identity — authorization and data ownership

**Current status (November 6, 2026):** Identity is the canonical authority for installation/entity identities, scoped grants, lock/unlock and revocation. This public `main` remains a documentation entry, **not a shipped production identity provider**. Historical P0/P1 alpha test results remain valid only for their recorded commit and environment.

## Current Go TASS integration

The integrated candidate [Thought-Processor PR #88](https://github.com/Arameair/Thought-Processor/pull/88) pins Identity `7da7f7c5bbecc42d3a7e9316a2037243eda10b88`, the [junction grant candidate PR #40](https://github.com/Arameair/identity/pull/40). It is experimental; no new security boundary is production-certified and the owner has not accepted the full combined local stack. [Current cross-repo source and release map](https://github.com/Arameair/Thought-Processor/blob/docs/current-go-stack-index-20261106/core/docs/CURRENT_GO_STACK_20261106.md).

- **Identity** owns canonical entity IDs, authenticated service identities, explicit narrowly-scoped capability grants and access decisions.
- **Thought Engine** owns thought/cycle execution and must check permissions at ingress and point of use. A matching display name is **not** an identity proof. [Thought-engine #17](https://github.com/Arameair/Thought-engine/issues/17) remains open.
- **Thought Processor/Core** handles human control, supervised learning and Cognition/Development UI; it cannot mint grants from a model, lesson, GitHub comment or a UI toggle.
- **TASS** owns its descriptive mind and approved lesson source; optional GitHub sync is not a substitute for security or ownership.
- **KYSSA / future companions:** separate independent user-controlled data and permissions are a design goal, **not yet a validated two-install privacy property**. [#30](https://github.com/Arameair/identity/issues/30) has the isolation and synthetic test plan; no real KYSSA account or private data must be modified without her owner's consent.

## Known gaps and required tests

[Identity #15](https://github.com/Arameair/identity/issues/15) covers authenticating observations and checking grants before queued dispatch; [Thought Processor #47](https://github.com/Arameair/Thought-Processor/issues/47) documents limitations of localhost page tokens. Separate file permissions are **not** the same as encrypted storage. A new installation must use its own IDs, secrets, storage and optional GitHub account; two entities with identical names must not share records. Revoked/locked/unavailable Identity must fail closed for protected actions.

**Current acceptance:** [Core #89](https://github.com/Arameair/Thought-Processor/issues/89) proves one real authorized operation and Cognition trace first; then [Core #90](https://github.com/Arameair/Thought-Processor/issues/90) verifies portable synthetic independent installations. There is **no authorization to deploy or grant broader abilities** here.

## Historical documentation and source code

Older P0/P1 admin-enrollment, separate launcher, experimental pairing and independent Feature/Fast Worker designs can be inspected via [the archived Git history](https://github.com/Arameair/identity/commits/main/) and [docs](docs/README.md). They should **not** be used as current startup instructions or assumptions about the Go one-app candidate. The public documentation-only `main` is distinct from experimental runnable code.

Release policy: branch/PR review, exact pin, tests and owner approval before merge; documentation edits do not constitute release acceptance.
