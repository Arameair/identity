# Owner Test Report — Identity Gate 1

**Copy this, fill in the "Owner run" column, and post it to [Issue #2](https://github.com/Arameair/identity/issues/2).**

| | |
|---|---|
| Identity branch | `experiment/identity-local-v1-20261023` |
| Code commit tested below | `013a1b139b7dcb518f17201329c0e98c4b658c86` (later commits on the branch change docs only) |
| Built from | `alpha` @ `8d08fec` (alpha, main and stable unchanged) |
| Project Controller workspace / OS | _owner fills in_ |
| Effective data folder | shown at the top of the page; with Project Controller, `$PC_DATA/identity` |
| Port | assigned by Project Controller (`PORT`); standalone default 7796 |

## Results

"Rehearsal" = run by the team in a Linux sandbox that copies Project Controller: a **detached checkout** of the exact commit, built with the `marquee.json` command, started with `PC_BRANCH`, `PC_DATA` and `PORT`, and stopped with SIGTERM. It is **not** your Project Controller.

| Step | What was done | Rehearsal | Owner run |
|---|---|---|---|
| **A. Launch** | Build 0.77 s; page up in 25 ms; `GET /` 200 | ✅ PASS: header shows `experiment/identity-local-v1-20261023 (PC_BRANCH, detached checkout)`, commit `013a1b13…`, data `…/pcdata-A/identity (PC_DATA)` | NOT RUN |
| **B. Create TASS** | AI companion | ✅ PASS: ID `fe8dc6ab-7ea5-4a37-869c-51d9a70985ab`; card + detail with Copy ID; file `identities/<id>.json` with mode 0600 | NOT RUN |
| **C. Restore** | Stop (SIGTERM) → Play | ✅ PASS: 1 identity; **same ID, name, type and created_at**; log "restored AI companion … id=fe8dc6ab…"; lock removed on Stop | NOT RUN |
| **D. Multiple types** | Create `Example Workshop`, Business | ✅ PASS: different ID `238f1db4-…`; TASS unchanged; no personality field | NOT RUN |
| **E. Future slots** | Open detail | ✅ PASS: GitHub NOT CONFIGURED, Thought Controller NOT CONNECTED, Memory/Discord/other NOT CONFIGURED; no network calls exist in the code | NOT RUN |
| **F. Failures** | "Make the next save fail" → Create; "Add a broken record file"; Copy Diagnostics | ✅ PASS: create returned HTTP 500 "save failed … nothing was written", no card; broken file listed, both identities intact; diagnostics contain **0** names | NOT RUN |
| **G. Isolation / lifecycle** | Second copy on the same `PC_DATA`; second copy on its own `PC_DATA`; stop each | ✅ PASS: same folder **refused** ("another Identity app (pid …) is already using data folder"); own folder ran alongside (A: 2, B: 1); stopping B left A running; after both stopped, **0 identity processes** | NOT RUN |

**Automated tests:** `go test ./...` → 18 tests: **17 PASS, 1 SKIP** (the read-only-folder test skips as root). It was also run as a normal user: **PASS**. The real-process test (`TestRealProcessStopAndRestart`) covers A, B, C, D and G with the actual binary. Windows build: compiles (`GOOS=windows go build`). **Not run on Windows.**

## Log from the rehearsal (Project Controller pane)

```text
07:49:44  start     Identity identity-local-v1 on http://127.0.0.1:7801/ (pid 3261) · branch experiment/identity-local-v1-20261023 (PC_BRANCH, detached checkout) · commit 013a1b139b7dcb518f17201329c0e98c4b658c86 · data …/pcdata-A/identity (PC_DATA)
07:49:44  restore   loaded 0 saved identities from disk
07:49:44  create    created and saved AI companion id=fe8dc6ab-7ea5-4a37-869c-51d9a70985ab
07:49:44  stop      stopped cleanly (signal terminated); all saves were already on disk
07:49:44  start     Identity identity-local-v1 on http://127.0.0.1:7801/ (pid 3294) · …
07:49:44  restore   loaded 1 saved identity from disk
07:49:44  restore   restored AI companion created 2026-10-23T12:49:44Z id=fe8dc6ab-7ea5-4a37-869c-51d9a70985ab
07:49:44  create    created and saved Business id=238f1db4-52ad-4151-8c72-d4eba9bae459
07:49:44  test-tool the next save will fail on purpose (acceptance F)
07:49:44  error     create failed: save failed: simulated failure (owner test tool) — nothing was written
07:49:44  test-tool wrote deliberately broken file corrupt-test-1792759784873595481.json (acceptance F)
07:50:03  stop      stopped cleanly (signal terminated); all saves were already on disk
```

## Not supported in this build (by design)

GitHub signup/OAuth/repositories, Thought Controller discovery or registration, any mock/fake adapters, credentials, memory, personality, Discord, AI, editing or deleting identities, import/export. Renaming will come later; the ID will never change when it does.

## Rollback

Nothing was promoted. To roll back, stop using this branch: `alpha` (`8d08fec`) is untouched. To clear the test data, stop the app and delete its data folder (the path shown at the top of the page).
