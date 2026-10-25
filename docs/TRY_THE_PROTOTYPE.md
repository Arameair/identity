# Try the current Identity prototype

**Last reviewed:** 2026-10-25 · **Audience:** testers and first-time contributors · **Status:** experimental, not a release.

The default `main` branch contains project documentation, not an application to launch. To run Identity, explicitly check out an **experimental branch**. The currently documented test candidate is [`alpha-candidate-20261025`](https://github.com/Arameair/identity/tree/alpha-candidate-20261025). It has not been promoted or certified for general use.

> This guide describes what the candidate's source and runbook provide. It is **not** a report of a successful Windows test on your machine. Record what actually happens; do not assume a green state.

## 1. Requirements

- Windows PowerShell, Git, and **Go 1.22 or newer**.
- An ordinary GitHub account for the optional real account test.
- Two terminal windows if you also want to run [Thought Processor](https://github.com/Arameair/Thought-Processor).
- No Project Controller installation or GitHub token pasted into a command.

Verify your tools:

```powershell
git --version
go version
```

## 2. Build and start Identity by itself

```powershell
cd $HOME
git clone --branch alpha-candidate-20261025 https://github.com/Arameair/identity.git identity-trial
cd identity-trial
.\scripts\build-windows.ps1
.\identity.exe
```

The script runs `go vet ./...`, `go test ./...`, and a Windows build. Identity uses loopback `127.0.0.1:7790` by default and stores its profile under `%USERPROFILE%\.identity\default`. The application should print a local opening URL and open a browser tab; use **the URL from your own terminal**, not one copied from someone else.

If your PowerShell execution policy blocks the script, use the candidate's [runbook](https://github.com/Arameair/identity/blob/alpha-candidate-20261025/AGENTS.md) for the direct `go` commands rather than weakening system policy unnecessarily.

**Expected first-run journey:** welcome; optional GitHub sign-in; feature availability; optional Processor address; monitor with live log and **Copy Diagnostics**. A fresh standalone Identity may report Processor as offline. That is expected if Processor is not running.

### GitHub account connection

The candidate says its publisher GitHub App's **public Client ID** is built into the application. Press **Connect GitHub** and authorize through GitHub's device-code page if you choose to test this. Normal users should not be required to create their own GitHub App.

- GitHub sign-in establishes the account GitHub reported; **it does not grant access** to a publisher's features.
- A local feature request is a draft marked **NOT SUBMITTED**, not an actual request sent or approved.
- If the flow stalls, record the on-screen state and redacted diagnostics; do not post device codes, tokens, sign-in URLs containing private fragments, or cookies.

## 3. Optional: run alongside Thought Processor

For the paired trial, the candidate's [AGENTS.md](https://github.com/Arameair/identity/blob/alpha-candidate-20261025/AGENTS.md) supplies exact commands for its matching Processor candidate. Follow that tested *plan* rather than assuming any Processor branch is interchangeable.

In brief:

1. Start Processor separately on `127.0.0.1:7860` in terminal A.
2. Start Identity in terminal B with `-processor 127.0.0.1:7860`.
3. Identity may show **DETECTED — SELF-REPORTED, UNVERIFIED** and a link to Processor. This is read-only health discovery, **not secure service pairing**.
4. Stop Processor; confirm Identity stays running and reports the offline state. Restart Processor; confirm the status updates.
5. Stop Identity; confirm Processor stays alive. Restart Identity; verify its System ID and permitted local logs remain while the GitHub session requires fresh sign-in.

Both applications must retain independent lifecycles. Neither should silently start, stop, or authorize the other.

## 4. Record real test evidence

Capture the exact branch and revision before a test:

```powershell
git branch --show-current
git rev-parse HEAD
go version
go test ./...
```

Use this minimal report:

```text
OS / Go version:
Identity branch / SHA:
Processor branch / SHA (if used):
Build & tests: PASS / FAIL / NOT RUN
Identity started and local page opened: PASS / FAIL / NOT RUN
GitHub real login: PASS / FAIL / NOT RUN / SKIPPED
Read-only Processor discovery: PASS / FAIL / NOT RUN
Independent stop / restart: PASS / FAIL / NOT RUN
Redacted diagnostic excerpt:
Unexpected behavior and reproduction:
Owner acceptance: PENDING / ACCEPTED / REJECTED
```

For a broader evidence checklist see [home-trial acceptance criteria](https://github.com/Arameair/identity/blob/experiment/identity-consolidated-home-trial-20261025/docs/HOME_TRIAL_ACCEPTANCE_2026-10-25.md). **Fixture tests, a successful compile, and an HTTP health check are distinct from a real owner Windows run or verified authorization.**

## 5. Stop / troubleshoot / reset

- Stop the Identity process with Ctrl+C in its own terminal. Do not kill unrelated processes or clear another application's profile.
- If `7790` is occupied, close the previous Identity instance or run `.\identity.exe -port 7791`.
- If a profile is in use, stop its current instance; never delete active profile files.
- If Processor is offline, confirm that Processor is running on the separately configured address. A response identifying itself as Processor is still unverified.
- **Reset is destructive:** only after backing up any wanted test data and stopping Identity, you may remove the *specific test profile you created*. For isolation on future runs, start Identity with `-data "$env:USERPROFILE\.identity\portfolio-trial"` instead of the default profile.

Do not use this experiment to guard sensitive services or confidential data. The authorization/grant layer is not implemented.

## Where next?

[Project overview](../README.md) · [Current verified status](STATUS.md) · [Engineering case study](PROJECT_JOURNEY.md) · [Architecture](ARCHITECTURE.md) · [Branch workflow](WORKFLOW.md)
