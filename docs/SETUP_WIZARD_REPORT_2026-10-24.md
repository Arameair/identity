# Identity setup wizard — first experience report (2026-10-24)

Human-directed, AI-assisted (Claude). Owner direction: Identity is a simple five-step wizard from opening the app to a running Thought Processor, polished for a non-technical person. Owner decisions in chat: **GitHub app sign-in** (device flow, owner-registered app) and **a narrow launch request added to Project Controller on an experiment branch**.

## Branches (nothing promoted; Alpha, Stable, Main untouched in all three repos)

| Repo | Branch | Base (alpha) | Tested SHA |
| --- | --- | --- | --- |
| Arameair/identity | `experiment/identity-setup-wizard-20261024` | `8d08fec6629dc9cc35ea2266556572b86e5499d7` | `2d6122050489aeb5dd73025a95024c2fd8c47c9f` |
| Arameair/Project-controller | `experiment/identity-launch-request-20261024` | `ceed56e1cfdab2b81346efabfffe2c20f8fccc68` | `a321c2eabdc6872b5941880b07c13c38854593cd` |
| Arameair/Thought-Processor | `experiment/processor-first-prototype-20261024` | `9fac9fa44cb020bb6f8aa6bcb1e9faeda6e0b47a` | `7c1a69e70df4e03f940b6bff907f0628735d1a81` (built by Controller from GitHub) |

## The experience

1. **Welcome** — what's being set up, what you need; one button *Connect GitHub*.
2. **Connect GitHub** — *Open GitHub*, a large code with *Copy*, *Authorize*; the page notices by itself and shows **Connected as \<username\>** read back from GitHub.
3. **Find Thought Processor** — "Found it": project, version (*First prototype (test version)*), last updated, "✓ \<you\> can see this private project"; branch/commit under *Technical details*.
4. **Start Thought Processor** — progress list (Ask → Download → **Your approval** → Build → Start up). At the approval step a yellow box says exactly what to click in Project Controller and which commit to check, with *Open Project Controller*. Controller shows a matching yellow notice with *Review & allow…*, which opens its existing exact-commit approval dialog.
5. **Open Thought Processor** — "Started successfully and answering", "✓ The running copy is the same version GitHub showed"; *Open Thought Processor* opens it in a new tab, already signed in.

Problems are explained in plain words with *Try again*: Cancel on GitHub, code expired, account can't see the project, Project Controller closed or too old, build/start failure, Thought Processor stopped. *Show what's happening* reveals Controller's live log (with the sign-in code hidden).

## How the handoff stays safe

- Identity **cannot approve**. It can ask Controller to add the project and download the branch (Controller works out the build plan without running project code), and read that one request's progress with a per-request secret. The **owner's click** in Controller runs it.
- Browser pages cannot create launch requests (any `Origin`/`Sec-Fetch-Site` is refused). Every other Controller action still needs Controller's own token.
- Retry after a failure/stop reruns only the **same already-approved commit and plan** (Controller's existing rule); a new commit asks for approval again.
- GitHub sign-in stays in Identity's memory; never written to disk or sent to the page.
- Thought Processor's one-time sign-in link is read from Controller's log for the current run and handed to the browser only on *Open*.

## Test results

**Live end-to-end** (`setup-wizard/acceptance/e2e.py`), 2026-10-24 ~09:50 UTC, Ubuntu 24.04 x86-64, Go 1.24.7, headless Chromium. Real Identity, real Project Controller, and the real Thought Processor that Controller downloaded from GitHub, built and ran. The test clicks like a person, including the approval click inside Controller.
**One stand-in:** GitHub sign-in/API was `acceptance/fake_github.py` (labeled TEST FIXTURE; Identity showed a TEST MODE banner throughout). It served the real branch commit `7c1a69e7`.

| Check | Result | Evidence |
| --- | --- | --- |
| Run Identity twice | PASS | second run reopens the window and exits; one Identity process |
| Opened without its link | PASS | asks to start Identity from its program; no data served |
| 1 Welcome | PASS | one primary button; sign-in link removed from the address bar |
| 2 GitHub — Cancel | PASS | "You chose Cancel on GitHub, so nothing was connected." + Try again |
| 2 GitHub — Authorize | PASS | "Connected as fixture-user" (read back with the new token) |
| 3 No access | PASS | "…account fixture-user can't see Arameair/Thought-Processor. Ask the owner…" |
| 3 Found | PASS | plain facts; technical details hold branch + full commit |
| Phone width 390 px | PASS | 0 px horizontal overflow |
| 4 Controller closed | PASS | "Project Controller isn't running at … Open Project Controller, then click Try again." |
| 4 Waiting for you | PASS | one pane in Controller, status *approval*, review commit = GitHub commit, **no Thought Processor process before approval** |
| Approve in Controller | PASS | notice names Identity (as self-reported) and the branch; dialog shows full commit |
| 4 Build + run | PASS | pane *running*, commit `7c1a69e`, port 7778 |
| 5 Open | PASS | Processor opened at its port, signed in, reporting build `7c1a69e70df4` |
| Code hidden in shown log | PASS | `code=<hidden>` |
| Stopped → Try again | PASS | plain message; running again in 2.0 s; still one pane |
| Close Identity | PASS | Identity exits; Controller and Thought Processor keep running |
| Orphan audit | PASS | nothing left that wasn't there before |

Totals: **16/16 PASS**, repeated identically in 3 consecutive runs (`660e83bb` ×2 before the last small change, `2d612205` ×1).

**Unit tests:** Identity wizard 6 tests pass (device-flow outcomes against a fixture, refusal of non-GitHub sign-in pages, find/no-access/no-branch, Controller missing/old, request guards, code hiding). Controller: new `TestLaunchRequestBoundary` passes; **two Controller tests already fail on its Alpha `ceed56e1` without my change** (`TestRestartUIHasCompactActionNotPermanentStatusCard`, `TestSnapshotRejectsCandidateWithoutMatchingApprovedPlan`). Thought Processor: 13 tests, 12 pass, 1 skipped as root.

**Timings (machine time, from the run):** Identity start 34 ms · welcome shown 0.3 s · find 0.16 s · request → waiting for approval 2.9 s (download) · approval → running and verified 2.4 s (Go build cached) · whole journey including the person's GitHub and approval clicks ≈ 20 s. A cold first build on a new machine will take longer (Go module download/compile).

### Not tested / BLOCKED

| Item | Status | Why |
| --- | --- | --- |
| Real GitHub sign-in | **BLOCKED** | Needs your registered GitHub App Client ID ([setup steps](SETUP_GITHUB_APP.md)) and a person to authorize |
| Windows / macOS / WSL | NOT RUN | Only Linux tested; Windows `marquee.json` commands are written but untested |
| Controller download with a different machine's git sign-in | NOT RUN | This sandbox's git already had access |

## Known problems

1. **Real GitHub sign-in waits on you** registering the app (about 2 minutes) and adding `github-app.json`.
2. **Controller's approval dialog is technical** (shows setup/run commands). Identity tells the person exactly what to check, but Controller's own wording was not changed.
3. **Two separate GitHub sign-ins.** Identity checks the *person's* access; Controller downloads with the *computer's* git sign-in. If the computer's git isn't signed in, step 4 fails with a clear "couldn't download" message.
4. **Thought Processor opens on its own "Tell Processor where Identity runs" screen.** Typing `127.0.0.1:7790` now shows Identity as *Found · unverified*. Connecting the two automatically was out of scope.
5. **Restarting Identity forgets the GitHub sign-in** by design (memory only); clicking through again is quick, and Start reuses the already-running Thought Processor.
6. **The launch request trusts its "requester" name** — shown in Controller as "name reported by that app". Any program on this computer could create a pending request; none can run it without your click.

## Your walkthrough (first real test)

1. **Once:** register the GitHub App ([steps](SETUP_GITHUB_APP.md)) and put `github-app.json` next to Identity.
2. Start Project Controller from `experiment/identity-launch-request-20261024` (its `run.sh` / `run.bat`, or `go build` and run). Leave it open.
3. Build and run Identity:
   ```bash
   git clone -b experiment/identity-setup-wizard-20261024 https://github.com/Arameair/identity
   cd identity/setup-wizard && go build -o identity . && ./identity
   ```
   Your browser opens **Welcome to Identity**.
4. Connect GitHub → Find Thought Processor → Continue → Start Thought Processor.
5. When Identity says *Your turn*, click **Open Project Controller** → **Review & allow…** → check the commit matches → **Approve this SHA and run**.
6. Back in Identity, wait for **Thought Processor is running**, then **Open Thought Processor**.
7. Try one failure: stop Thought Processor in Project Controller and click **Try again** in Identity.

Owner verdict: **PENDING**. Promotion: **NOT AUTHORIZED**.
