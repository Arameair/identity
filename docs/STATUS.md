# Status — what's real versus planned

[← Home](../README.md) · [Roadmap](ROADMAP.md) · [Team instructions](../AGENTS.md)

**As of 2026-10-23:** Identity is a **documentation-first proposal**.

| Area | Status |
| --- | --- |
| Reusable Identity application repository | **Exists** — private GitHub repo |
| Main README, navigation, design documentation and safe examples | **Proposed on experimental design branch** |
| Alpha | **Bootstrap baseline only** — no working application |
| Identity creation/restoration wizard | NOT IMPLEMENTED |
| GitHub signup guidance / OAuth authorization | NOT IMPLEMENTED |
| Real per-entity private GitHub repository creation | NOT IMPLEMENTED |
| Protected credential store / account broker | NOT IMPLEMENTED |
| Thought Controller service discovery / registration | NOT IMPLEMENTED |
| Memory / Discord / other module registry | NOT IMPLEMENTED |
| Live Identity tests | NOT RUN |
| Alpha promotion | NOT DONE (owner-only) |

## Dependencies

The repaired Thought Controller experiment is in [Arameair/Thought-Controller — Issue #3](https://github.com/Arameair/Thought-Controller/issues/3). The team reported fixes and fake-site/Go tests, but the owner still needs to verify the first actual Chat-Jimmy thought and accept a controller baseline. Entity registration is a **future interface**, not an existing feature.

## The first demonstrable milestone

An owner can Play an Identity test prototype from Project Controller, create a distinct local test entity, restart and restore the exact same ID, inspect safe diagnostics, and see a **mock** GitHub/private-repo setup and **mock** controller registration. It must not touch real providers or modify live identities by accident.

## What the next coding team should do

Read [AGENTS.md](../AGENTS.md) and [Gate 1](ROADMAP.md); implement only one isolated experimental prototype from the current owner-approved Alpha, bring back tests and an owner-visible demo, and explicitly label anything not demonstrated. **Do not implement full account connections before the owner reviews the local prototype and security plan.**
