# Identity — first new-administrator prototype + real read-only GitHub CLI check

> **Experimental GitHub verification branch** \`experiment/identity-github-verification-20261023\`. This deliberately ports the first-administrator prototype into a new branch **from approved Alpha**, then adds an explicitly user-triggered read-only verification of an existing official GitHub CLI session. It does NOT authenticate Identity to GitHub, store tokens, or create repositories.

> **Ported baseline:** `experiment/first-admin-wizard-20261023` @ `4686c94f` (original [Issue #6](https://github.com/Arameair/identity/issues/6)). This *new* branch starts from `alpha` @ `8d08fec` and explicitly ports that prototype as a separate commit before adding the GitHub CLI check. **Neither Alpha nor the earlier experiment was changed.**
>
> Partner branch: Thought Controller `experiment/first-user-dashboard-20261023` @ `a6b9dfa668bc0c8fd34af65832964abfae697125`. **Results and the owner checklist for both apps: [docs/FIRST_USER_PROTOTYPE_RESULTS_2026-10-23.md](docs/FIRST_USER_PROTOTYPE_RESULTS_2026-10-23.md).**

Identity is the first app a new administrator opens. On a clean **test profile** it walks you through:

1. **Welcome** — *Create my installation* or *Restore an existing installation*.
2. **Your installation** — three separate, permanent IDs: **administrator**, **installation**, **Thought Controller service**.
3. **Identities** — *Create TASS and Example Workshop*, or add your own.
4. **Thought Controller** — checks whether it's running and belongs to you, then asks **once**: *Approve automatic startup for this specific controller on later launches?*
5. **GitHub** — honest status: **NOT CONNECTED**, and source access **ACCESS REQUIRED** (both repos are private with no published releases).
6. **Open your workspace** — opens the simulated Thought Controller dashboard.

After setup, Identity opens an overview instead of the wizard. **Test as New User** (top right) opens a brand-new empty profile so you can do it all again as someone else; **Profiles ▾** switches back.

**Nothing here is a real integration.** The launcher and controller handshake are **SIMULATED**, the GitHub alternative states are **MOCK**. Identity never starts a program, never contacts GitHub or any other server, stores no credentials and makes no AI calls. Its only outbound requests go to a Thought Controller address **on this computer** that you can see and change.

## Test it in Project Controller

1. **Arameair/Thought-Controller** → branch **`experiment/first-user-dashboard-20261023`** → **Play**. Note its port. Its page says *No controller service is running* — correct.
2. **Arameair/identity** → branch **`experiment/first-admin-wizard-20261023`** → **Play**. You land on **Welcome** (a fresh test profile).
3. Create your administrator → create TASS and Example Workshop → on **Thought Controller**, put the port from step 1 in *Where to look* (or press **Find on this computer**) → you should see **NOT RUNNING for this installation**.
4. **Approve and start** → exactly one simulated start request → **RUNNING · VERIFIED (simulated handshake)**.
5. **GitHub** → **NOT CONNECTED**. **Open simulated Thought Controller dashboard ↗**.
6. **Stop** both panes, **Play** both again (Thought Controller first). Same IDs, approval kept, the controller starts once without asking, thoughts are back.
7. **Test as New User** → second installation: different IDs; its controller check says **WRONG INSTANCE** while the first installation's controller is running. Stop that one (Thought Controller page → *Stop service*), approve for the new one, and its dashboard has none of the first one's identities or thoughts.

The full step-by-step checklist with expected results is in the [results doc](docs/FIRST_USER_PROTOTYPE_RESULTS_2026-10-23.md#owner-checklist).

## Test it standalone (no Project Controller)

```sh
# Thought Controller checkout, branch experiment/first-user-dashboard-20261023
go build -o thought-controller . && PORT=7797 ./thought-controller -data ~/tc-first-user-test
# Identity checkout, this branch (second terminal)
go build -o identity . && PORT=7796 ./identity -data ~/identity-first-admin-test
# open http://127.0.0.1:7796/
```
On Windows PowerShell: `$env:PORT=7797; .\thought-controller.exe -data $HOME\tc-first-user-test` and `$env:PORT=7796; .\identity.exe -data $HOME\identity-first-admin-test`. A new profile already looks at `127.0.0.1:7797`.

Needs Go 1.22+. The pinned build is the exact commit above, so run that commit (the branch tip). Any other build shows **VERSION MISMATCH** until you pin it yourself (*Change target…* or *Pin the build answering at this address*) and approve again — by design.

## Where the data lives

| Priority | Source | Folder |
|---|---|---|
| 1 | `-data DIR` | `DIR` |
| 2 | `IDENTITY_FIRST_ADMIN_DATA` | that folder |
| 3 | Project Controller `PC_DATA` | `$PC_DATA/identity-first-admin` |
| 4 | standalone | `~/.identity-first-admin-test/<branch>` |

These names deliberately differ from the local V1 experiment (`IDENTITY_DATA`, `$PC_DATA/identity`, `~/.identity-test`), so this prototype **cannot open or change your V1 test identities**. Inside: `active-profile.json` and `profiles/<profile>/` with `installation.json`, `identities/<id>.json`, `controller.json` (address, pinned target, consent, launch attempts), `github.json`, `events.jsonl`. A folder inside the code checkout is refused; a second Identity on the same folder refuses to start.

## Startup permission (option B), exactly

- Asked only when you have a pinned target. Approving with nothing pinned pins **the suggested test build shown on screen** — nothing else is ever chosen for you.
- Saved per profile with a consent ID and a fingerprint of *repo + commit + launcher*. It authorizes **only** asking the simulated launcher to start that build. Not model calls, GitHub, updates, messages or other programs.
- Used automatically on Identity startup, on opening the controller screen, on *Check again*, and right after approving — **one request per check**, never in a loop. A running controller for your installation is reused with no request.
- **Revoke**, **Not now** (declined), or a **changed target** → nothing is started; the question comes back.
- If no Thought Controller host is running, nothing is attempted: the simulated launcher can only ask a host that is already running. A real launcher/installer is not built.

## Controller check results

| Shown | Meaning |
|---|---|
| **NOT FOUND** | Nothing answered at the address |
| **NOT RUNNING for this installation** | A Thought Controller host answered; your controller service isn't started |
| **RUNNING · VERIFIED (simulated handshake)** | Bound to your installation and service ID, and running the pinned commit |
| **WRONG INSTANCE** | Running, but bound to another installation — never used or taken over |
| **VERSION MISMATCH** | Not the pinned commit — never started or used |
| **INCOMPATIBLE** | Something answered that isn't a Thought Controller (or speaks another protocol) — not trusted just because it answered |

## What was reviewed and what was (not) carried over

- **`alpha` @ `8d08fec`** (bootstrap README) — this branch starts here.
- **`experiment/identity-local-v1-20261023` @ `2b31e99`** (Gate 1 local manager, owner test pending in [Issue #2](https://github.com/Arameair/identity/issues/2)) — **not merged.** Ported by copy, with comments naming the source: `lock.go` / `lock_unix.go` / `lock_windows.go` (unchanged but the lock name), `store.go` (one file per identity, atomic write + read-back, repeat-click safety, broken files left alone — extended to schema 2 with `role` and `installation_id`), the event-log, data-folder and loopback / same-origin guard code. The V1 branch itself is unchanged.
- **Design notes** on `experiment/fresh-admin-bootstrap-design-20261023` and `experiment/design-foundation-20261023` — followed; not modified.

## Tests

`go test ./...` — 9 tests with an in-memory fake controller (IDs distinct and restored, profile isolation, consent / deny / revoke / changed target, one start under concurrent checks, NOT FOUND attempts nothing, wrong instance, incompatible, consent survives restart, loopback-only addresses, failed install writes nothing, V1 folder names never used, HTTP guards, diagnostics without names).

`THOUGHT_CONTROLLER_SRC=/path/to/Thought-Controller go test -run FirstUser -v` — builds and runs **both real programs** Project Controller-style and walks the whole new-administrator flow, restart included, plus a second installation.

## Rollback

Nothing was promoted. Stop using this branch — `alpha` (`8d08fec`) is untouched. To clear test data, Stop Identity and delete its data folder (shown on the Welcome screen and in diagnostics). To remove the experiment: `git push origin --delete experiment/first-admin-wizard-20261023`.


## New: verify GitHub CLI (not Identity OAuth yet)

On the GitHub step, click **Check GitHub CLI account (REAL)**. Identity invokes the installed official `gh` CLI with a single read-only `gh api user` request, using the account it has already authenticated. Success shows the verified GitHub username and makes clear that this is an **external machine session, not a connection to this Identity ID Card**. It never imports or stores a token.

If GitHub CLI is absent, the page shows **CLI NOT INSTALLED**. If it has no working login, it shows **CLI NOT AUTHENTICATED OR UNAVAILABLE**. To set up the CLI itself, a user may install [GitHub CLI](https://cli.github.com/) and run `gh auth login --hostname github.com --web` in a terminal; this is a separate, explicit GitHub authorization, outside Identity.

The check is optional, and Identity's local test profiles and cards keep working without it. The app's **mock connected** selector remains only a mock UI demonstration; it must not replace this real check. Per-installation OAuth, secure account brokerage, access scope verification, and approved private repository creation remain future separately tested features. Do not treat a verified CLI username as authorization for an entity to use GitHub.
