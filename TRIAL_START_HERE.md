# Identity — standalone first-run / feature request prototype

**Status: experimental and incomplete. Do not promote.**

This experiment is branched from `experiment/identity-setup-wizard-20261024`. It keeps that older wizard unchanged under `setup-wizard/` and introduces a separate **standalone** prototype under [`standalone/`](standalone/). Ordinary users do not need Project Controller.

## Current test

- Start `identity.exe` (Windows) or `./identity` (Linux), built from `standalone/` or obtained as an experimental test artifact.
- Enter the **public** GitHub App Client ID on first use (future publisher-signed builds should embed it).
- Authorize the user's real GitHub account using device flow. This proves the logged-in GitHub identity, but **does not yet prove association with an authorized Identity repository or grant entitlement**.
- Show fixed experimental catalog from publisher-labeled `Arameair/identity`: Thought Processor, Thought Engine.
- Save a local, deduplicated **access request DRAFT**, clearly marked **not submitted**. Export it for manual review.
- No actual feature installation, no remote request service, no publisher approval, no entitlement and no Engine authorization.

## Verification status

- Locally compiled a standalone variant on Linux with Go 1.23.2. Unit tests, vet, Linux build and Windows cross-compile succeeded.
- The local standalone variant passed an API-level smoke test using a fake GitHub device flow and /user endpoint, including one saved request and duplicate prevention.
- A browser-driven test of the local app was **blocked by the test environment's administrator browser policy**; do not claim a browser click-through passed.
- The *repository branch's* GitHub Actions run `40100082236` ended in failure **without step summaries or downloadable logs**, so repository build status is **UNVERIFIED**, not a demonstrated source error or passing run. Continue verification on the owner's Windows environment.
- **No real GitHub login, actual publisher delivery or Windows executable run was verified by the assistant.**

## Owner acceptance plan

1. On your computer, start this experiment **as a standalone app**, not through Project Controller.
2. Confirm Identity opens its own window. Set the public Client ID.
3. Use GitHub's official device authorization. Confirm the *actual* account is displayed, not a TEST MODE identity.
4. Check the two feature cards and save Thought Processor request. It must show `SAVED LOCALLY — NOT SUBMITTED`, not `APPROVED`.
5. Restart and reconnect; verify local request history persists without duplicate records, no access granted.
6. Capture sanitized startup logs and owner UI notes; **do not share startup session fragment or GitHub code/token**.

Before any customer release: signed publisher identity, user+repository binding enforcement, real publisher-hosted request/approval service, revocation, audited distribution and safely scoped installer.

**Made with AI assistance.** Owner verdict pending. No Alpha/Stable/Main changes.
