# First new-administrator prototype — results, owner checklist, rollback

**Shared report for both apps.** Identity issue [#6](https://github.com/Arameair/identity/issues/6) · Thought Controller issue [#6](https://github.com/Arameair/Thought-Controller/issues/6) · [Identity handoff](https://github.com/Arameair/identity/blob/experiment/fresh-admin-bootstrap-design-20261023/docs/CLAUDE_FIRST_USER_PROTOTYPE_HANDOFF_2026-10-23.md) · [Thought Controller handoff](https://github.com/Arameair/Thought-Controller/blob/experiment/multi-identity-controller-design-20261023/docs/CLAUDE_FIRST_USER_PROTOTYPE_HANDOFF_2026-10-23.md)

| | |
|---|---|
| Identity | `experiment/first-admin-wizard-20261023` · code commit `8546426388dd01db7c35cc085dc0b538345314b9` (later commits on this branch are docs only) · from `alpha` @ `8d08fec` |
| Thought Controller | `experiment/first-user-dashboard-20261023` @ `a6b9dfa668bc0c8fd34af65832964abfae697125` (branch tip = the build Identity suggests pinning) · from `alpha` @ `02fc27f` |
| Alpha / main / stable | **Unchanged** in both repositories. Nothing promoted, nothing merged. |
| Rehearsal platform | Linux sandbox, Go 1.24. Project Controller **imitated**: detached checkout of each exact commit, built with the `marquee.json` setup command, run with `PORT`, `PC_BRANCH`, `PC_DATA`, stopped with SIGTERM. **Not your Project Controller and not Windows** (Windows builds compile; not run). |

## What is real and what is simulated

| Piece | Status |
|---|---|
| Welcome, create/restore, the three distinct durable IDs, profiles, Test as New User | **Real** (local files, restored on restart) |
| TASS / Example Workshop identities and their IDs | **Real** (local) |
| Saved startup permission (option B), revoke, re-ask on changed build | **Real** (saved per profile) |
| Controller discovery (`GET /api/health` on a loopback address you choose) | **Real** read-only check; verification of the answer is a **SIMULATED, unauthenticated handshake** |
| "Starting" Thought Controller | **SIMULATED launcher**: one request to a Thought Controller *host that is already running*; no program is ever launched or installed |
| GitHub | **NOT CONNECTED** is the true state; other states are **MOCK** displays. Zero GitHub requests, zero tokens |
| Thought dashboard, rows, detail, history, diagnostics, per-entity storage | **Real UI over real files**; thoughts are **SIMULATED** fixtures from buttons |
| Cross-entity / cross-installation separation | **Enforced server-side**, but **not authenticated** — any local program can call either API |
| AI worker, autonomous thinking, real launcher/installer, OAuth, repo creation, releases | **Not built** (out of scope by instruction) |

## Blocked / missing dependencies (reported, not faked)

1. **No approved installable build exists.** Both repositories are private with no published releases. The only pin possible is a **TEST / PINNED LOCAL ARTIFACT** — a branch commit you run yourself. Identity shows *Source access: ACCESS REQUIRED*.
2. **No trusted lifecycle owner.** Identity can't start Thought Controller; Project Controller (or a future approved launcher) must Play it. The simulated launcher only asks an already-running host.
3. **No authentication** between the apps or on either local API. Separation is real server-side scoping, not authorization (Stage B / P3).
4. **Thought engine not integrated.** The repaired V1 engine (`experiment/v1-repair-20261023`) is unapproved and was deliberately not ported.

## Automated tests

| Suite | Result |
|---|---|
| Identity `go test ./...` | **9/9 PASS** (fake controller in memory) |
| Identity `THOUGHT_CONTROLLER_SRC=… go test -run FirstUser` | **PASS** — both real programs, Project Controller-style, full flow incl. restart and second installation |
| Thought Controller `go test ./...` | **10/10 PASS** incl. `TestRealProcess` (real binary, restart, same-folder refusal) |
| Both with `-race` | PASS |
| Browser rehearsal (Playwright, real UIs, both real programs) | **30/30 checks PASS**, 0 page script errors |
| `GOOS=windows go build` (both), `GOOS=darwin` (TC) | compiles; **not run** |

## Handoff matrix

"Rehearsal" = the browser + process run above. **Owner run** is yours.

| # | Test | Rehearsal | Evidence | Owner run |
|---|---|---|---|---|
| F1 / N1 | Empty profile → Welcome, 0 identities, data path + branch/SHA visible | **PASS** | Welcome shows `test-profile-1`, 0 identities, data folder, `PC_BRANCH`, detached checkout, full commit | NOT RUN |
| F2 / N2 | Create administrator → distinct admin / installation IDs saved | **PASS** | 3 different UUIDs; same after restart | NOT RUN |
| F3 / N3 | Controller absent → NOT FOUND, not fake Connected | **PASS** | "Nothing answered at 127.0.0.1:7811" before Thought Controller was Played | NOT RUN |
| — | Host up, no consent → NOT RUNNING, nothing started | **PASS** | Thought Controller start requests = 0 | NOT RUN |
| F4 / N5 | Approve pinned TEST build → exactly one simulated start, consent saved & revocable | **PASS (simulated)** | 1 start request; service ID = Identity's; consent ID + fingerprint saved; Revoke works | NOT RUN |
| F5 / N9 | GitHub unconfigured; nothing changed | **PASS** | NOT CONNECTED (real) · ACCESS REQUIRED · 0 GitHub requests; MOCK states labeled | NOT RUN |
| F6 / N13 | Open simulated dashboard: TASS / business tiles, System Services, empty states | **PASS (simulated)** | "Who's thinking?" → TASS, Example Workshop, System Services; "No thoughts yet" | NOT RUN |
| T3–T5 / N14 | TASS thoughts don't appear under the business and vice versa | **PASS** | TASS 4 thoughts / Workshop 0; Workshop thought invisible from TASS; cross-entity write → HTTP 404 | NOT RUN |
| F7 / N6 / N15 / T9 | Stop both, Play both → same IDs, consent, thoughts; one start, no prompt, no wizard | **PASS** | one start request (trigger "Identity startup"); counts unchanged; overview not wizard; no leftover processes | NOT RUN |
| N7 | Restart Identity while controller runs → reuse, no request | **PASS** | Thought Controller start requests stayed at 1 | NOT RUN |
| F8 / N4 | Decline → Identity continues, nothing launched | **PASS** | "Automatic startup was declined"; service not started | NOT RUN |
| F8 | Revoke → nothing started | **PASS** | no attempt after revoke | NOT RUN |
| F8 / N8 | Changed build → re-approval needed, VERSION MISMATCH, no start | **PASS** | target changed → consent invalid → VERSION MISMATCH, 0 requests | NOT RUN |
| N8 | Wrong service at the port → not trusted | **PASS** | address pointed at Identity → INCOMPATIBLE | NOT RUN |
| N8 / T12 | Second installation meets the first one's running controller | **PASS** | WRONG INSTANCE; never used or taken over | NOT RUN |
| F8 / N16 / T12 | New test profile → separate IDs, identities and thoughts | **PASS** | new installation / admin / service IDs; 2 new entities with 0 thoughts; the first's TASS → 404 while B runs | NOT RUN |
| — | Switch back to the first profile | **PASS** | saved consent restarted it; TASS 4 / Workshop 1 thoughts intact | NOT RUN |
| F9 / N18 / T13 | Copy Diagnostics: SHAs, IDs, state, consent, attempts, times; no secrets | **PASS** | both diagnostics contain IDs/SHAs/attempts; 0 names or thought text | NOT RUN |
| T1 | Thought Controller runs independently, honest "not started" | **PASS** | page + health show NOT STARTED, branch/SHA, data folder | NOT RUN |
| T11 / N20 | No GitHub, no worker → usable, honest | **PASS** | Worker NOT READY; model calls 0 | NOT RUN |
| N10–N12, N17, N19 | Real GitHub login, private-source access, repo creation, upstream divergence, secure credential store | **NOT RUN — not built** (out of scope) | — | — |
| T2 / T6 / T7 / T10 / T14 / T15 (full) | Authenticated registration, auth denial, Think Once binding, migrations, real lifecycle | **BLOCKED** on Stage B / P3 | — | — |

The rehearsal's screenshots and diagnostics are from the sandbox and are not committed; the IDs in them are throwaway test IDs.

## Owner checklist

Do this in Project Controller. Nothing here touches your V1 Identity data or the repaired V1 Thought Controller data (different folder names).

| Step | Do | Expect |
|---|---|---|
| 1 | Play **Thought-Controller** `experiment/first-user-dashboard-20261023` | "No controller service is running"; note the port |
| 2 | Play **identity** `experiment/first-admin-wizard-20261023` | **Welcome**, profile `test-profile-1`, 0 identities, branch + full SHA |
| 3 | Type your name → **Create** | Three different IDs (Copy works) |
| 4 | **Next** → **Create TASS and Example Workshop** | Two identities with IDs |
| 5 | **Next** → *Where to look*: put the port from step 1 (or **Find on this computer** → **Use**) | **NOT RUNNING for this installation**; *What answered* shows `a6b9dfa…`, SIMULATED, auth none |
| 6 | **Approve and start** | **RUNNING · VERIFIED**; Launch history: 1 request, *started*; consent ID shown |
| 7 | **Next** | GitHub **NOT CONNECTED**, Source access **ACCESS REQUIRED**. Optional: MOCK states under *Test the GitHub card* |
| 8 | **Next** → **Open simulated Thought Controller dashboard ↗** | Who's thinking? → TASS, Example Workshop, System Services |
| 9 | TASS → **Load sample set**; open a card; add a note; **Resolve**; check *Development history* and *Diagnostics* | Four rows filled; history records each change; worker calls 0 |
| 10 | Example Workshop | **No thoughts yet**. Add one; switch back to TASS — it isn't there |
| 11 | System Services | Service / installation / administrator IDs match Identity's |
| 12 | Back in Identity → **Finish setup** | Overview |
| 13 | **Stop** both panes; **Play** Thought Controller, then Identity | Identity opens the overview (no wizard), same IDs, *Approved for this exact build*, Launch history +1 (*Identity startup*); dashboard thoughts unchanged |
| 14 | Stop + Play **only Identity** | Still VERIFIED; Thought Controller's start-request counter doesn't move |
| 15 | **Revoke**, then on the Thought Controller page **Stop service**, then **Check again** | Nothing starts. Approve again → one start |
| 16 | **Test as New User** → create a second admin → identities → controller step with the same port | New IDs; **WRONG INSTANCE**. Press **Not now** → nothing starts |
| 17 | Thought Controller page → **Stop service**; Identity → **Approve and start** | New installation's dashboard: TASS and Example Workshop with **no thoughts** |
| 18 | **Profiles ▾** → `test-profile-1` (stop the second one's service first) | First installation back, its thoughts intact |
| 19 | **Copy Diagnostics** in both apps | Paste into the issue: SHAs, IDs, consent, attempts; no names, no thought text |

Please paste PASS / FAIL per step (and anything confusing) into Identity issue #6.

## Diagnostics example (from the rehearsal; throwaway IDs)

```text
## Identity diagnostics (2026-10-23T14:39:39Z)
- Branch: experiment/first-admin-wizard-20261023 (from PC_BRANCH) · detached checkout
- Commit: 8546426388dd01db7c35cc085dc0b538345314b9
- installation_id 574f55cb-505b-4fa9-a064-cbb00a3f4769
- administrator_id d3fbb6ef-987d-4e59-ac7b-514d4cfb9919
- controller_service_id 996211e1-0e78-43c8-8921-9c74f9716dad
- Pinned target: Arameair/Thought-Controller · experiment/first-user-dashboard-20261023 · a6b9dfa668bc0c8fd34af65832964abfae697125 · TEST / PINNED LOCAL ARTIFACT
- Startup consent: approved · valid for pinned target: true · fingerprint 1643777a9df99d89
- Controller counters: checks 16 · start requests 4 · started 4 · reused 0 · refused 0 · errors 0
- Last check: VERIFIED … · worker NOT READY
- GitHub: NOT CONNECTED · source access ACCESS REQUIRED · GitHub requests made: 0 · tokens stored: 0

## Thought Controller diagnostics
- Mode: SIMULATED HOST · protocol tc-sim-handshake/0.1 · authentication NONE
- Lifecycle counters: start requests 4 · services started 4 · duplicate requests 0 · refused 0 · stops 3
- AI worker: NOT READY (none exists) · model calls: 0 · GitHub calls: 0 · processes launched: 0
- Managed entities: 2 (AI companion: 4 thoughts · Business: 1 thought)
```

## Known gaps to review

- Default controller address is `127.0.0.1:7797` (standalone). Under Project Controller the port differs, so step 5 needs the port or **Find on this computer** (scans 7778–7899).
- One simulated host serves one installation at a time; a second installation must stop the first one's service. A real setup would be one controller per administrator.
- The host's *own* log and diagnostics name every installation ID that started on it (not their identities or thoughts).
- The dashboard refreshes every 4 s; it pauses while you type or have a thought open.

## Rollback

Nothing to undo in `alpha`, `main` or `stable`. Stop using the branches, or delete them: `git push origin --delete experiment/first-admin-wizard-20261023` (identity) and `git push origin --delete experiment/first-user-dashboard-20261023` (Thought-Controller). Test data: Stop the apps, delete `$PC_DATA/identity-first-admin` and `$PC_DATA/thought-controller-first-user` (paths shown in each app).
