# Owner home-trial acceptance card — Identity + Thought Processor
Date: 2026-10-25. This is a **test plan**, not results. Owner verdict is PENDING. Mark each line PASS/FAIL/BLOCKED/NOT RUN only with real receipts. Use isolated profiles. No Alpha/Stable/Main changes.

## First priority (green ONLY if the owner sees it)

| ID | Check | Required evidence |
| --- | --- | --- |
| P0 | Existing Processor exact experiment branch builds, starts and displays actual UI | Repo, branch, SHA, build command, port, process PID, timestamp; no mock |
| P1 | Processor stops/restarts without duplicate/orphan process | Before/after PID, status, log; no unrelated process killed |
| I0 | One new consolidated Identity app builds/runs as standalone without Controller | Exact SHA, executable, private profile, port, health, observed UI |
| I1 | The normal UI is simple: verified status + persistent scrolling live log + Copy Diagnostics | Screenshot and real log lines, no fake green |
| I2 | Identity diagnoses Processor at actual address; mismatch/offline are honest | Identity `DISCOVERED / UNVERIFIED`, Processor public health receipt; no false authorization |
| I3 | Both processes run concurrently; each survives closing the other's browser/independent process | Distinct PID, recovery, logs |
| I4 | Copy Diagnostics paste-able into AI chat, includes build and request evidence; secret canary NOT reproduced | Redacted example and tests for `#code=`, access token, auth header, cookies, GitHub device code |
| I5 | Persisted Identity events survive full restart, bounded/rotated; different profile remains isolated | Before/after run IDs and record samples |
| I6 | Windows PowerShell step-by-step owner test works on real home machine | Actual Windows run (not just Linux or cross-compile), PASS only after owner runs it |

## Secondary capability (do not block Gate 0 on this)

| ID | Check | Required evidence |
| --- | --- | --- |
| S1 | Create, archive, restore two different entity cards without losing IDs | Before/after persisted IDs; no GitHub deletions |
| S2 | GitHub device login succeeds with the actual registered App and actual owner account | Screenshot with token/code masked; no fixture |
| S3 | Local Thought Processor feature request draft survives restart and stays `NOT SUBMITTED` | Local request record; no publisher grant claim |
| S4 | Third-party Github account / missing publisher app / denied auth fails honestly | Denied/blocked state; no bypass |
| S5 | No sensitive logs/credentials in GitHub artifacts or chat diagnostics | Negative canary proof |

## Explicit NON-PASS conditions

- `go test` or fixture screenshots alone are not owner Windows PASS.
- Identity returning HTTP 200 does not equal authentication.
- A Processor health name is a self-claim, not verified trust.
- A GitHub user login or repo collaborator role does not equal publisher feature entitlement.
- A local request draft is NOT an actual submitted or accepted permission.
- A Controller Play approval is not an Identity runtime Engine grant.
- Engine feature `enabled` toggle is not Identity authorization.
- A worker response is never faked.
- Failed CI without job logs = `CI FAILED / cause unverified`, not build verified.

## Handoff report format — fill with measured facts

```text
Verdict: GREEN / YELLOW / RED
Identity source: repo, branch, SHA
Processor source: repo, branch, SHA
Host: OS, architecture, Go version, owner machine vs CI vs developer fixture
Identity run: command, PID, port, private profile, elapsed start, result
Processor run: command, PID, port, elapsed start, result
Tests: P0 ... P1 ... I0 ... I6 ... S1 ... (PASS/FAIL/NOT RUN)
Real vs fixture: explicit per test
Owner actions: exact minimum steps
Known blockers: missing API, failure reproduction, fix proposal
Diagnostic excerpt: redacted and source-tagged
Rollback: how to stop each process and restore original test profile/branch
Owner promotion decision: PENDING
```

**Minimum end-of-day report:** show an actual run attempt (or concrete reproducible failure), a truthful status/log/copy flow, Windows owner steps, and precise remaining blocker. Never defer to a new round of speculative planning.
