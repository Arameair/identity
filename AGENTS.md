# Team instructions — Identity FIRST working prototype

[← Home](README.md) · **[Read the precise first-build specification](docs/FIRST_PROTOTYPE.md)** · [Acceptance](docs/ACCEPTANCE_TESTS.md) · [Roadmap](docs/ROADMAP.md)

## The assignment — ONE step only

Build a small local **Identity Manager** application. It must **create, save, list/open and restore** unique identities across complete app restarts. Work for different entity types (at least AI companion, Business, Person, Project, Other). Identity is **not** personality. Do not build GitHub integration, Thought Controller integration, even mocks for those connections, or other modules now.

**This issue is the authority for first-build scope:** [Gate 1 — local Create / Save / Restore](https://github.com/Arameair/identity/issues/2). The broader design documents are roadmap context only. If they describe GitHub sign-in or Thought Controller connection, those belong to future gates.

## Branch and development policy

- **Start from current owner-approved `alpha`** on a NEW `experiment/identity-local-v1-YYYYMMDD` branch. Alpha/main are still bootstrap; the design proposal lives on `experiment/design-foundation-20261023`. Read the docs from that design branch, but **do not branch from the unapproved design experiment** or claim Alpha includes those docs.
- Never change or promote `alpha`, `stable` or `main`; owner manually approves promotion after watching a working test.
- Build independently runnable Go web app unless compelling reason otherwise. Include `marquee.json` with Go build commands and `PORT` so owner can **Play and Stop in Project Controller**. No dependency on an already running Thought Controller or GitHub.
- Project Controller uses detached checkouts! Respect `PC_BRANCH`, `PC_DATA`, actual git commit and assigned `PORT`, and explicit data override. Each experimental workspace must have isolated persistent test data. Do not store state inside the project checkout or default all detached builds to one shared “default” data folder.
- Safe loopback-only local service; do not collect/store any credentials or private account data; no external calls or automatic account creation.
- Implement durable, versioned entity records, unique IDs, atomic saves, recoverable read/write errors, input validation, same-ID restoration, repeat-click safety, multiple entities, and test data isolation.
- UI should be calm, readable, simple, with **Create identity**, **Open identity**, local restore, **Copy ID**, **Copy Diagnostics**. Clearly label GitHub **NOT CONFIGURED**, Thought Controller **NOT CONNECTED**; no fake PASS badges.
- Record branch/SHA/data directory, exact outcome and error. Provide tests and owner-visible instructions; only report tests actually run. STOP leaves no orphan process.

## What to deliver

1. Link to the new experimental branch and full SHA (not Alpha).
2. Runnable Project Controller interface and `marquee.json`.
3. Tests for create/list/open/restore, distinct IDs, corrupt/denied writes, concurrent/repeated creation, data-folder separation, basic API safety, independent Stop/Play.
4. Owner report for [A–G](docs/FIRST_PROTOTYPE.md#owners-first-test-must-be-doable-without-terminal): PASS / FAIL / BLOCKED / NOT RUN with exact evidence, logs, timings, screenshot/context if available and rollback details.
5. Explain any unsupported functionality honestly. **Do not start Gate 2 without fresh owner instruction**.

Do not make an all-in-one companion, build a personality system, or require any external login just to create a local entity.
