# Security and account-management boundaries

**Reviewed:** 2026-10-25. **Security status: experimental.** This project does **not** yet provide a verified identity provider, protected administrator authority, authenticated Identity-to-feature pairing, or enforced multi-service grants. Do not use the prototype to secure sensitive data or production systems.

[Current evidence](STATUS.md) · [Prototype trial](TRY_THE_PROTOTYPE.md) · [Planned contracts](INTERFACES.md)

## Trust principles

- An **identity card** is a record; it does not authenticate a person or program.
- **GitHub account authentication** confirms the account represented by a GitHub response. It does not grant another application's capabilities or repository access.
- **Local service discovery** checks a self-reported endpoint. A matching process name, port or HTTP 200 is not authenticated pairing.
- **Authorization** requires a defined caller, resource, scope, expiration and receiving-service enforcement. Feature visibility or a UI toggle is not a grant.
- Never place passwords, MFA codes, access tokens, session cookies, private opening links or unredacted sensitive logs in GitHub, reports, screenshots or model prompts.

## Two distinct GitHub approaches exist in experiments

### Earlier local manager: existing GitHub CLI session

The [real-manager experiment](https://github.com/Arameair/identity/tree/experiment/identity-real-manager-20261023) uses an existing `gh` login for read-only verification of a GitHub account and repository metadata. It does not issue Identity-controlled GitHub authorization. Its OS/WSL/VM credential handling is inherited from the installed CLI.

### October 25 candidate: GitHub App device flow

The [Alpha candidate](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) includes code for GitHub's device-authorization flow and a publisher's **public** App Client ID, with the login token intended to live in process memory rather than on disk. Its source also implements sign-in lifecycle handling, local account visibility and explicit sign-in status.

The [Prototype B test record](https://github.com/Arameair/identity/blob/experiment/identity-home-claude-20261025/docs/HOME_TRIAL_EVIDENCE_CLAUDE.md) demonstrates a simulated/fixture device flow, but explicitly does **not** establish a completed real GitHub login on the owner's Windows computer. The later candidate documents fixes to `slow_down` handling and stale sign-in codes; **real successful owner acceptance still requires observation**.

A **Client ID is public** and can be compiled into a distributed app. **Client secrets, device codes, access tokens and authentication cookies are not public**. The design should request only provider permissions necessary for each explicitly supported action, including account/installation metadata. It must not imply that access to a GitHub repository grants access to publisher features.

**Current feature requests:** candidate-side drafts are **NOT SUBMITTED**; permission states are **NOT GRANTED**. A future submission, approval, entitlement and revocation service still needs design and receiver-side testing.

## Local HTTP and service connections

The candidate starts a local server bound to `127.0.0.1`, provides local browser access, and probes Processor's health/status endpoint read-only. Local binding, private browser fragments and redaction reduce some exposure, but **none establishes secure mutual authentication for separate services**.

Before cross-service grants are real, test at least: service identity proof, administrative approval, scope validation, replay prevention, expiration, revocation, offline behavior, compatible protocol versions, actor isolation and enforcement in the destination service.

Never forward a local app's private `#key=` or Processor `#code=` fragment. Use the current run's link only in your own local browser. Review diagnostics for private material **even if redaction tests have passed**.

## Data ownership, isolation and recovery

Identity is intended to organize location/ownership references for independently operated features while preserving each feature's own data and logs. Actual OS-level isolation, secure administrator bootstrap, backup/restore and cross-device migration remain **design and acceptance gates**. A proposed folder layout is not proof of filesystem security.

- Prefer distinct profiles for each experiment and application.
- Stop processes before changing or deleting a profile, and back up any data you want to preserve.
- No feature should receive a shared master token by default.
- Identity being stopped must not magically authorize a new service; any permitted offline behavior requires a bounded and tested policy.
- Do not store private user data or secrets in repository checkouts.

## Required before any security or production claim

1. Real provider sign-in tested on the target OS, including denial, failure, cancel and expiration paths.
2. Secret redaction and profile persistence audited with negative tests and reviewed diagnostics.
3. Administrator and service identity protected by an explicit OS-aware trust model.
4. Authenticated enrollment, scoped grants and **destination-side rejection** of unauthorized calls.
5. Documented rollback, revocation, backup/restore, versioning and negative security testing.
6. Human owner sign-off with recorded build SHA and reproducible evidence.

[Roadmap](ROADMAP.md) · [Architecture](ARCHITECTURE.md) · [Storage](STORAGE.md) · [Workflow](WORKFLOW.md)
