# Identity alpha candidate — 2026-10-26

**Branch:** `alpha-candidate-20261026` · **Code commit tested:** `df983ce` · **Version:** `0.4.0-alpha` · **API:** `identity/1-alpha`
**Built from:** `experiment/engine-access-pairing-20261026` @ `884e386`, which is built on `alpha-candidate-20261025` @ `7addeee`.
**Status:** CANDIDATE. Not promoted. Only the owner promotes it to `alpha`.

## What this candidate is for

The Oct 25 candidate worked as a backend subsystem, but it put the person in front of an **Identity-first browser wizard**. On Oct 26 the owner decided that **Thought Processor is the normal interface** and **Identity is the independent authority behind it**.

This candidate turns Identity into that background service. It gives the Processor team a real, signed API to build against now. It also gives the owner and the Grok bot a way to install and test it on Windows today, without waiting for Processor's UI.

## Changes from the last candidate, mapped to the Oct 26 decisions

| Decision | Oct 25 candidate | This candidate |
| --- | --- | --- |
| **C01** Processor is the interface; either start order | Opened its own browser wizard on start | Runs in the background, and no browser opens. The console tells the person to use Thought Processor. The wizard only appears if someone asks for it in the troubleshooting page (which is relabelled as such). `runtime/service.json` tells Processor where Identity is, whichever program started first. |
| **C02** install ≠ use | Install check only (experiment branch) | Each feature reports separate states: `eligible_to_install`, `installed`, `service_registered`, `enabled_for_identity`, `authorized_sources`, `running`. Eligibility is labelled "install only". |
| **C03** no access by default | — | **Identities** are real records (person, companion, business, project) with IDs that never change. Each starts with empty access lists. `POST /api/v1/authorize` answers **DENIED**, with a reason, for every request. |
| **C04** exit choice, clean stop | "Stop Identity" button only | `POST /api/v1/shutdown` requires an explicit confirm and records the reason. The process exits and removes `service.json`. Status reports whether Identity was started by Processor (`MANAGED`, via `-managed-by`) or by a person (`EXTERNAL`). Identity never touches other programs. |
| **C05** operator-approved pairing, reconnect, revoke | Code requested in Identity's page | **Processor asks**, Identity shows the code in **its own window**, and the person types it into Processor. The pairing survives restarts of both programs. Processor can unpair itself, and the person can also unpair from Identity's page. Every answer is HMAC-signed over the exact body, with Processor's nonce, and carries the installation ID, so a wrong or replayed service shows up. |
| **C06** fail closed | — | Decisions say `cache: none` and `valid_for_seconds: 0`. The reference client shows **AUTHORITY UNAVAILABLE** and exits with code 2 when Identity is down. |
| **C07** Memory separate | — | Nothing about Memory. |
| **C09** owner promotes | — | This branch is a candidate only. `alpha`, `main` and `stable` are untouched. |
| #19 friction | Engine-access "next step" said *Open Identity…* | Now says to connect GitHub from Thought Processor |

**New tool: `identity-probe`.** It is a small test client that talks to the API exactly as Processor should: it pairs, checks, creates identities, runs GitHub sign-in, shuts Identity down, and unpairs. Its `smoke` command starts the real `identity` binary as separate processes and runs **11 scenarios** on throwaway profiles. It is how the owner and Grok can test today, and its `call()` function is reference code for the Processor team.

## Evidence (this environment)

| Check | Result | Scope |
| --- | --- | --- |
| `go vet` (Linux + `GOOS=windows`), `go test -race ./...` (10 packages) | **PASS** | Linux dev container; GitHub = labelled TEST FIXTURE where used |
| Windows cross-build of `identity.exe`, `identity-probe.exe` | **BUILDS** | Not run on Windows here |
| `identity-probe smoke` against the real binary, commit `df983ce` | **PASS, 19/19 checks** · [report](evidence-20261026/smoke-report-linux.txt) | Linux, real processes, no GitHub |
| Console in service mode, with the pairing box (code masked) | [sample](evidence-20261026/console-sample.txt) | Linux |
| Troubleshooting page: banner shown, wizard not shown, Processor chip says `PAIRED · 1` | [home](evidence-20261026/t01-troubleshooting-home.png) · [manage](evidence-20261026/t02-manage.png) | Headless Chromium |

The smoke test covers these scenarios:

- background start
- `service.json` written
- pairing code shown only in Identity's window
- pairing succeeds
- the code and token never appear in the log
- an identity is created with no access
- signed checks of status, identities, features and authorize (DENIED)
- confirmed shutdown: the process exits, `service.json` is removed, and the reason is logged
- restart and reconnect without a new code, with identities kept
- **Processor-first** start: the client waits, then connects once Identity starts
- Identity down gives AUTHORITY UNAVAILABLE
- a different installation refuses the token
- a second instance on the same profile is refused
- unpair, after which the old token is refused
- restart after a crash-style kill

**Not yet shown — this is what your test is for:**

1. A run on **real Windows**: `AGENTS.md` Steps 1–10.
2. A **real GitHub** sign-in started through the API (`identity-probe github`), plus the Thought Engine eligibility result for your account.
3. Thought Processor using API v1. The Processor team is on this now; `docs/API_V1.md` is their contract.

## IN / OUT for this candidate

**IN:**

- background service mode
- API v1: pairing, status, identities, features, authorize (DENIED), GitHub, shutdown, unpair
- signed answers
- service locator
- test client and self-test
- troubleshooting page
- the older Engine-install routes, kept unchanged so `experiment/install-thought-engine-20261026` still works

**OUT (on purpose):**

- creating grants, and receivers enforcing them
- feature service registration
- per-person admin sign-in and secure bootstrap
- keeping GitHub signed in across restarts
- OS-protected token storage
- a process supervisor
- signed releases
- Memory
- any change to Thought Engine

## Decisions only the owner can make (none block testing)

1. **Promote?** Once Windows runs pass, decide whether this replaces `alpha`. The current `alpha` is still the Oct 23 docs-only baseline.
2. **GitHub sign-in after a restart.** Right now each Identity restart signs GitHub out, which keeps the token out of storage. For a non-technical user that means signing in again often. Options:
   - keep it as it is;
   - store the token with Windows DPAPI;
   - store a refresh token only.
3. **When Processor launches Identity itself,** Processor will see Identity's console output, including the pairing code. Decide whether "Processor started it" counts as the operator's approval, or whether the person must still type the code.
4. **First grant to implement.** The smallest useful one is probably: one identity → `thought-engine` → one source. Building it also needs Thought Engine to check `authorize` before accepting work.

## How to try it

See [AGENTS.md](../AGENTS.md). The short version: run `.\scripts\build-windows.ps1`, then `.\identity-probe.exe smoke`, then `.\identity.exe`, then `.\identity-probe.exe pair`, then `.\identity-probe.exe check`.
