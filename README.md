# Identity

**An experimental local-first authority for companion identities and permissions.**

Identity separates **who may act** from **the software or AI model proposing an action**. It is designed to issue and verify bounded owner-approved permissions for other services, without storing all of their private thoughts or taking over their execution.

> **Project status — November 9, 2026:** Working research and integration prototypes, **not a production-ready identity provider**. Public `main` is a documentation entry point, not the latest runnable Go service. Newer code, independent reviews and owner-host acceptance are tracked in experimental branches and draft pull requests.

## What Identity is responsible for

- **Canonical identity and ownership:** Track distinct entities and authorized services; a matching name is not proof that two entities are the same.
- **Explicit grants:** Limit permission to a specific requester, operation and target. Authentication or a reachable local port alone is not authorization.
- **Lock, revoke and deny:** Protected work must not proceed when required authority is unavailable or withdrawn.
- **Service enforcement:** The receiving service must verify the decision too. A chat message, model response, GitHub issue, Markdown document or visual graph cannot grant itself permission.

Identity does **not** generate Thoughts, execute skills or choose the companion's goals. Those behaviors belong to other components and require their own execution evidence.

## Current development evidence

The current system experiments combine Identity with a local Go Thought Engine and the [Thought Processor owner interface](https://github.com/Arameair/Thought-Processor). Historical Alpha and P1 experiments include permission and pairing tests, but their results must not be generalized to every newer candidate.

The current supervised-nudge work is documented in [Thought Processor issue #100](https://github.com/Arameair/Thought-Processor/issues/100): it requires owner-bound permission for each proposed nudge and an independently checked Stop operation. **The accepted design is not proof that the full implementation or live behavior passed testing.**

[Owner acceptance #89](https://github.com/Arameair/Thought-Processor/issues/89) and [portability review #90](https://github.com/Arameair/Thought-Processor/issues/90) track the wider end-to-end proof needed for reproducible execution and independent installations.

## Security limitations

This is **not** an Internet-exposed login service, a certified identity platform, or a guarantee of isolation between same-OS-user processes. Localhost browser tokens, local file permissions, and a passphrase are not by themselves complete operating-system isolation or encryption. Multi-companion privacy, recovery, permission revocation under failure and safe owner-machine deployment require further validation.

Avoid putting real credentials, private owner records, prompts or execution journals in public test reports. No README, issue or PR authorizes a live grant or service restart.

## Source, history and review

- [Thought Processor overview](https://github.com/Arameair/Thought-Processor) — human-facing control plane and current milestone.
- [Identity documentation](docs/README.md) — historical designs and scope.
- [November 6 pinned integration snapshot](https://github.com/Arameair/Thought-Processor/blob/af59e7d9213170669eddd4f269ba1f1b50c427e9/core/docs/CURRENT_GO_STACK_20261106.md) — dated reference, **not** a live status feed.
- [Identity documentation review](https://github.com/Arameair/identity/pull/41) — draft for owner review.

Earlier P0/P1 source, tests and decisions remain accessible through Git history. **No stable public installation or production-security claim is made.**
