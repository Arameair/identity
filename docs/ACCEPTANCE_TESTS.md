# Owner test checklist — first Identity prototype

[← Home](../README.md) · [Exact Gate 1 design](FIRST_PROTOTYPE.md) · [Roadmap](ROADMAP.md)

**Only Gate 1 is the next coding task.** Its acceptance items are A–G below. Account setup and Thought Controller tests are deliberately deferred. Don't call fake connections a success or a compile a functional UI test.

| Test | Owner-visible action | PASS means |
| --- | --- | --- |
| **A. Launch** | Play the experimental Identity app inside Project Controller | Usable local web UI; shows correct actual branch/full SHA and isolated data folder; no need to start another project |
| **B. Create** | Create `TASS`, AI companion | Permanent unique ID, name/type/date saved; one clear card; opening it shows its fields and Copy ID |
| **C. Restore** | Stop then Play same app / same local data folder | Identical ID/name/type/created timestamp; no duplicated entity |
| **D. Multiple types** | Create `Example Workshop`, Business; open both | Distinct IDs and records; no personality field required; no overwrites |
| **E. Honest future slots** | Inspect each entity's status panel | GitHub NOT CONFIGURED; Thought Controller NOT CONNECTED; memory/Discord NOT CONFIGURED; no external requests |
| **F. Failures** | Simulate unwritable/corrupt data (test fixture) and use Copy Diagnostics | Error reported instead of false “Saved”; earlier valid data not silently lost; safe diagnostic contains no secrets |
| **G. Isolation/lifecycle** | Start another experiment with another data folder; stop each | No accidental sharing/wiping of state; no orphan process; same-branch restart still restores |

**Test with mock entities only** (fictional/test names, no personal secrets). Further automated tests must cover invalid names/types, conflicting/duplicate records, repeated clicks, missing/corrupt JSON, read-only folders, non-loopback/unsafe endpoints and safe shutdown.

**Owner handoff must include:** exact Identity branch/full SHA, Project Controller branch/OS, `PC_DATA` or effective data path, actual ports, tested steps A–G with PASS/FAIL/BLOCKED/NOT RUN, number of records, logs/diagnostics, failures and reproductions, build/run elapsed time where available, and a clear rollback. No Alpha promotion until explicit owner approval.

## Later test inventory — NOT Gate 1

- Guided official GitHub signup vs existing-account OAuth; private repo creation/verification; credential protection and revocation; restore on another device (Gate 2).
- Versioned controller health, authenticated idempotent registration, absence/incompatibility handling, no unauthorized side effects and scoped entity data (Gate 3).
- Missing memory capability request, owner approval, independent module registration, relocation and revocation (Gate 4).

These remain important but are **future experiments**, not blockers for creating and restoring the first local entity.
