# FIRST prototype — local Identity Manager (the next coding task)

[← Project home](../README.md) · [First-build issue #2](https://github.com/Arameair/identity/issues/2) · [Test checklist](ACCEPTANCE_TESTS.md)

## In one sentence

**Build one small app that creates a unique entity, saves it locally, and restores exactly the same entity after the app is stopped and restarted.** Prove it works for both an AI companion and a business. **No GitHub or Thought Controller connection yet, not even mock connectors.**

This is **the FIRST step**, not the entire Identity vision. The long-term GitHub account manager and controller registration are described elsewhere, but **must not creep into this implementation**.

## What the owner should see

A calm local web app launched via [Project Controller](https://github.com/Arameair/Project-controller), using its Play/Stop controls. No terminal use for routine create/view/restart.

~~~text
IDENTITY MANAGER                              [Copy Diagnostics]
Local experiment  ·  branch/SHA  ·  data folder

YOUR IDENTITIES
┌──────────────────────┐  ┌──────────────────────┐
│ TASS                 │  │ Example Workshop     │
│ AI companion         │  │ Business             │
│ ID: 7b29...d084      │  │ ID: e5c3...81af      │
│ Saved locally        │  │ Saved locally        │
│ [Open]               │  │ [Open]               │
└──────────────────────┘  └──────────────────────┘

[Create New Identity]        [Open Existing Identity]

What comes later (not connected in this build):
  GitHub repository: NOT CONFIGURED
  Thought Controller: NOT CONNECTED
  Memory / Discord / other modules: NOT CONFIGURED
~~~

**The first-run form asks for exactly two user choices:** display name and entity type. Support at least **AI companion, Business, Person, Project, Other**. Do **not** ask for personality, password, token, model, GitHub login, or Thought Controller port.

When an entity is opened, show **name, type, full persistent ID with Copy ID, creation date, local save status, and future connection statuses clearly labeled NOT CONFIGURED**. Provide a back button and a useful error if anything failed. The permanent ID must not change when the display name is changed later; editing can wait for another prototype.

## Exact functional scope

1. **Create.** Validate non-empty reasonable name and a supported type. Generate a collision-resistant UUID (or equivalent) exactly once on creation; don't derive the ID from name or type.
2. **Save.** Store a small schema-versioned record locally: `id`, `display_name`, `entity_type`, `created_at`, `schema_version`. Include no external account secrets. Save atomically where possible and surface errors; don't display “Saved” when a write failed.
3. **List/Open.** Show all saved local test identities in an easy-to-read dashboard. Open a record and inspect its exact fields.
4. **Restore.** Stop the app in Project Controller. Restart the **same branch and data profile**. Every saved record must have the exact same ID, type, name, and timestamp. Creating a second entity does not overwrite the first. “Restore” initially means load a previously saved local identity; import from GitHub or another computer is **future work**.
5. **Branch safety.** Project Controller frequently checks out an exact SHA in **detached HEAD** and provides `PC_BRANCH` and `PC_DATA`. Use the actual `PC_DATA` (or explicitly selected `-data`/environment override) as the default data area; don't infer separate branch storage only from `git branch --show-current`. For standalone runs, use a safe local test directory. Never write identity data into the checked-out repo; do not share test data across branches unintentionally. A later explicit “use live profile” flow is out of scope.
6. **Clean independent lifecycle.** A `marquee.json` must let Project Controller build/Play/Stop the app on an assigned `PORT`; run loopback-only. Stopping this app must not start/stop Thought Controller, Chat-Jimmy, GitHub or anything else. Shut down without leaving orphan workers.
7. **Honest diagnostics.** Show the exact running branch/full SHA if known, app version/build, data directory, count of saved identities, errors and recent create/restore events. Provide a **Copy Diagnostics** action that excludes private content beyond deliberately public test metadata and never includes tokens. Do not falsely display GitHub or controller as working.
8. **Robustness.** Handle empty state, malformed/corrupt state, duplicate IDs, read-only/unwritable folder, repeated clicks, and restart gracefully. Preserve earlier good records when a new write fails. Tests must be real and reproduce failures.

### Data and security rules for Gate 1

These are **local TEST identities only**, containing no credentials or personal secrets. Restrict the local API to loopback and validate input, including non-browser requests. Treat the local data path as user-controlled; prevent path traversal. Use safe file permissions for persisted data where supported. Do not check state files into Git. A more extensive OS credential store and authorization system is **later**, before any real account connector.

## Owner's first test (must be doable without terminal)

| Step | Owner action | What counts as PASS |
| --- | --- | --- |
| A | Play the Identity experimental branch in Project Controller | Real page appears; actual run branch/SHA/data folder visible |
| B | Create `TASS` (AI companion) | Unique ID generated, saved, visible in card and detail |
| C | Press Stop; then Play the same identity app again | **Same TASS ID, name, type, timestamp**; no duplicates |
| D | Create `Example Workshop` (Business) | Different ID; TASS remains intact; neither has a personality requirement |
| E | Inspect future connections | GitHub = **NOT CONFIGURED**, Thought Controller = **NOT CONNECTED**; no requests sent |
| F | Test a failed save/corrupt fixture; press Copy Diagnostics | Real warning and safe diagnostic; earlier data remains recoverable |
| G | Run a separate experimental instance/data directory | Its test identities cannot overwrite the first instance's records; each stops independently |

The team must supply a **copyable Owner Test Report** with exact branch/SHA, Project Controller workspace, environment (Windows/WSL/Linux), data folder, commands actually run, test results PASS/FAIL/BLOCKED/NOT RUN, timings/errors, unsupported features and rollback instructions. A successful `go test` is helpful **but not the same** as the owner Play/Stop test.

## Explicitly DO NOT build in Gate 1

- GitHub signup, GitHub OAuth, account linking, private repository creation, GitHub mock/fake adapters, tokens, credential manager or remote sync.
- Thought Controller discovery, starting it, registration, fake-controller adapter, model calls or thought state.
- Memory, Discord, ScreenBridge, personality, curiosity, module installers, external automation, multi-entity authorization, account permission flows.
- Automatic merge/promotion to Alpha/main, and deletion/migration of a real identity.

**Next after owner verifies this:** Guided private GitHub repository setup and secure account authorization; after that, a small negotiated Thought Controller entity-registration interface. See [Roadmap](ROADMAP.md). The first experiment should give us an **actual durable entity** to connect later instead of inventing an entirely new one at each step.
