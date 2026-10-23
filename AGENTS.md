# Team instructions — read before writing code

[Project guide](README.md) · [Gate plan](docs/ROADMAP.md) · [Owner acceptance checks](docs/ACCEPTANCE_TESTS.md) · [Security](docs/ACCOUNTS_AND_SECURITY.md)

## Assignment

Build a reusable **Identity application**, not a persona manager or a WoW-specific chatbot. Identity creates/restores entity IDs, guides GitHub account/repository setup, manages secure account authorizations and registers entities/modules with Thought Controller. Businesses/people/AI companions must all be valid entity types; personality, thinking and memory are optional.

**Current requested phase: documentation and handoff.** Do not interpret this blueprint as permission to provision real GitHub repos, request actual tokens or change Thought Controller.

## Branch policy

- All implementation experiments **start from latest owner-approved `alpha`**, on a new `experiment/<feature>-<date>` branch. Do not work directly on main, alpha or stable.
- Keep Alpha/main unchanged; no auto-merge/push to Alpha. Owner promotes manually only after a real review.
- If an experiment depends on unpromoted Thought Controller code, name its exact SHA and request owner guidance for the integration strategy; don't pretend it is Alpha.
- Include `marquee.json` or the manifest required by Project Controller; show correct running branch/full SHA and independent Play/Stop.

## First implementation request (Gate 1 only)

1. Implement a small, clean guided dashboard: Create, Restore, Open entity, inspect fields, fake account status and fake controller status.
2. Generate ID once, persist locally and restore after restart. Distinguish **live profile** vs isolated **experiment test profile**, honor Project Controller's `PC_BRANCH` and `PC_DATA` as appropriate while ensuring code branch choice never silently changes a real entity.
3. Write **mock GitHub adapter** and **fake controller** to prove workflow without external side effects. Do not create actual GitHub accounts/repos or use private access tokens in Gate 1.
4. Model nonsecret data and future **capability references**; never store passwords/tokens in checked-in repo or trace. Provide an explicit human approval path for future actions.
5. Show readable errors, status/metrics, **Copy safe diagnostics**, version, storage location, request IDs; separate configured/authorized/reachable/verified, mock/real and PASS/FAIL/UNVERIFIED.
6. Add tests for identity restoration, duplicate prevention, offline/denied states, branch isolation, Reset/Stop/restart, corrupted store, and forbidden cross-entity access.
7. Provide an owner-run acceptance walkthrough in Project Controller with exact branch/SHA, logs, known gaps, clean rollback and no orphan processes.

## Future gates — do NOT combine

- Gate 2: approved real GitHub signup guidance, minimal OAuth/GitHub App permissions, protected OS credential vault, **private repo** creation/selection and secure restore.
- Gate 3: explicitly negotiated and authenticated **versioned** Thought Controller registration/discovery. Existing controller V1 does not have this yet.
- Gate 4: first optional module (`memory.primary`) and owner-approved setup flow.

## Proof and handoff expectations

For every change, report what actually runs, why it's safe, exactly which tests ran, whether data was MOCK/test/live, and all failures/unknowns. A green badge without evidence is not acceptance. **No covert account creation, no raw tokens in GitHub, no model-driven permission self-approval, no automatic branch promotion.**
