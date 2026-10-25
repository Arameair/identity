# Prototype B (Claude) — home-trial evidence and acceptance

```text
Verdict: YELLOW — both real programs run together and see each other read-only on a developer Linux
         machine; owner Windows run and real GitHub sign-in not yet observed.
Identity source: Arameair/identity, experiment/identity-home-claude-20261025, 7ea0dabba74dd90980bdfc4d0f6ff66b4080f312
Processor source: Arameair/Thought-Processor, experiment/processor-controller-launch-fix-20261024, 0ba8bd52facedda1405897e9136d68ab184a2e32 (unchanged)
Host: Claude's sandbox — Linux x86-64, Go 1.24.7, headless Chromium (Playwright). NOT the owner's Windows PC.
Identity run: ./identity-b -no-browser -data <fresh profile>  → pid 11814, port 7798, run_025b50f002a0c2fa; restart pid 11955
Processor run: ./thought-processor -port 7860 -data <fresh profile> → pid 11812 (22 ms start); restart pid 11934
Windows: identity-b.exe cross-compiled (GOOS=windows, vet clean); Processor .exe cross-compiled. Neither RUN on Windows.
Owner promotion decision: PENDING
```

## Acceptance (from HOME_TRIAL_ACCEPTANCE_2026-10-25)

| ID | Result | Type | Evidence |
| --- | --- | --- | --- |
| P0 Processor builds/starts/real UI | **PASS (dev Linux)** | real binary | SHA 0ba8bd52face printed at start; `go test` ok; UI screenshot `real-10-processor-sees-identity.png` |
| P1 Processor stop/restart, no orphan | **PASS (dev Linux)** | real | 11812 → SIGTERM → 11934; `ps` shows one Processor ([two-process-evidence.txt](evidence-home-claude-20261025/two-process-evidence.txt)) |
| I0 Identity standalone, no Controller | **PASS (dev Linux)** | real | started directly, no Controller/`gh`; health `{"app":"identity",…}` |
| I1 Simple UI: status + scrolling log + Copy Diagnostics | **PASS (dev Linux)** | real | `real-06-monitor.png`, `real-07-monitor-after-copy.png` |
| I2 Honest Processor diagnosis | **PASS (dev Linux)** | real + unit | `DETECTED — SELF-REPORTED, UNVERIFIED`; offline → `NOT RUNNING` (warn); wrong service / 404 / closed port covered by `internal/discovery` tests |
| I3 Both run concurrently; each survives the other stopping | **PASS (dev Linux)** | real | Processor stopped → Identity alive, logged NOT RUNNING; Identity stopped → Processor health still answers |
| I4 Copy Diagnostics paste-safe, canaries absent | **PASS** | unit + real | 16 canary patterns in `internal/redact`; app test injects `#code=`, bearer, cookie, device/user code, access token; live key/code grep = 0 in diagnostics and log file |
| I5 Log persists across restart, bounded; profiles isolated | **PASS (dev Linux)** | real + unit | restart reloaded 11 lines, same System ID, new run ID; 1 MiB × 4 files rotation test; second Identity on same profile refused |
| I6 Owner Windows PowerShell run | **NOT TESTED** | — | needs owner; steps in [TRIAL_START_HERE_CLAUDE.md](../TRIAL_START_HERE_CLAUDE.md) |
| S1 Entity cards create/archive/restore | **NOT TESTED / deferred** | — | not built in B (first-use focus) |
| S2 Real GitHub device login with registered app | **NOT TESTED** | — | sandbox proxy blocks `github.com/login/*` (HTTP 403/415 from proxy, not GitHub); no Client ID was available. Flow verified only against **TEST FIXTURE** (`fixture-*.png`) |
| S3 Local draft survives restart, NOT SUBMITTED | **PASS (fixture account)** | unit + runtime fixture | draft persisted across restart; duplicate returns existing; forged `"GRANTED"` drafts file refused and left untouched |
| S4 Denied / missing app fails honestly | **PASS (fixture + real-GitHub-config)** | unit | denied → `SIGN-IN CANCELLED`; wrong Client ID → "doesn't recognise"; no Client ID → `APP NOT CONFIGURED`, start refused |
| S5 No secrets in artifacts/diagnostics | **PASS** | unit + grep | see I4; GitHub token memory-only, never written |

`go test ./...` — 7 packages ok, 28 tests; `go vet` clean (linux and windows). `-race` clean.

## What is real vs fixture
- **Real:** both binaries, ports, PIDs, profiles, log files, Processor's own UI and its probe of Identity, Identity's probe of Processor, restarts, diagnostics.
- **Fixture (labelled TEST FIXTURE in every Identity view):** the GitHub account `fixture-owner`, its installation and repo visibility, and therefore the AVAILABLE/UNAVAILABLE and draft screenshots (`fixture-*.png`).

## Copy Diagnostics example (real run, abridged; full file: [copy-diagnostics-example.txt](evidence-home-claude-20261025/copy-diagnostics-example.txt))
```text
=== IDENTITY DIAGNOSTICS — Prototype B (Claude) ===
[build]  repo=Arameair/identity branch=experiment/identity-home-claude-20261025
commit=7ea0dabba74dd90980bdfc4d0f6ff66b4080f312 modified=false
[process] status=RUNNING pid=11955 run=run_0dd3f4e1fa45f681 bind=127.0.0.1 port=7798
[github] mode=real status=NOT_CONFIGURED client_id=NOT CONFIGURED
[thought processor — read-only discovery]
address=127.0.0.1:7860 state=DETECTED — SELF-REPORTED, UNVERIFIED freshness=CURRENT
authentication=NONE (no Identity↔Processor contract exists)
[features] Thought Processor: availability=UNKNOWN — sign in to GitHub to check | request=NOT REQUESTED | grant=NOT GRANTED
[latest problems] warn processor.state_changed result=NOT RUNNING … from DETECTED — SELF-REPORTED, UNVERIFIED
[not implemented — never shown as working] submission service, grants, installs, Processor auth contract, Engine grants …
```

## Screenshots (headless Chromium driving the real programs)
`real-01-welcome` · `real-04-processor-step` · `real-06-monitor` · `real-08-manage-features` · `real-10-processor-sees-identity` — and fixture-only: `fixture-02a-github-waiting-code` · `fixture-02-github` · `fixture-03-features`. All in [evidence-home-claude-20261025/](evidence-home-claude-20261025/).

## Known blockers / next security gates
1. **Real GitHub sign-in unobserved** — needs the publisher app's public Client ID with Device Flow on, and an owner run.
2. **No publisher submission or grant service** — drafts stay local; nothing can ever show REQUESTED (submitted) or GRANTED.
3. **No authenticated Identity↔Processor contract** — both sides top out at "self-reported, unverified".
4. **Windows file lock** (`lock_windows.go`, no-sharing handle) compiles but is unobserved on Windows.
5. Processor's "Open" link opens its page; the owner still types Processor's own operator code (by design).

## Rollback
Stop: Ctrl+C / close window / *Stop Identity*. Remove profile `~/.identity/home-claude-b`. No other branch or profile was touched; delete this branch to remove the prototype entirely.

## Update 2026-10-25 07:40 — built-in publisher app, one-click browser sign-in, guided own-app setup
- **Built-in app:** `internal/publisher.GitHubAppClientID` is shipped inside the binary (source constant or `-ldflags -X`). Verified: an `-ldflags` build prints `github : built into this build (Arameair's GitHub App)`. **The real Client ID is not in the source yet** — the publisher adds it once ([PUBLISHER_GITHUB_APP.md](PUBLISHER_GITHUB_APP.md)).
- **Connect GitHub** now opens GitHub's device page in a new tab (opened during the click, so pop-up blockers allow it; `opener` cleared) and copies the code. Verified in headless Chromium against the TEST FIXTURE: new tab URL = fixture device page, clipboard = displayed code (`v2-05-builtin-waiting.png`, `v2-06-builtin-verified-FIXTURE.png`).
- **No built-in app:** the GitHub step says so plainly and offers **Help me set up a GitHub App** — a 5-step guide that opens GitHub's registration page pre-filled (name, homepage, read-only contents, webhook off) and accepts the Client ID (`v2-01`, `v2-02`, `v2-03`). The person's own app is saved in their profile and can be removed to return to the built-in app.
- Tests: new `main_test.go` (choice order: developer env > own app > developer file > built-in) and `TestOwnAppGuideSwitchesAndReverts`. Real github.com still **NOT TESTED** from this sandbox (its proxy blocks `github.com/login/*`).
