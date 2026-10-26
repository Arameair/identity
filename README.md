# Identity — alpha candidate 2026-10-26

**Identity is the independent identity and permission service behind Thought Processor.** It runs in the background, holds the identities this installation is the authority for, signs in to GitHub on the person's behalf, and answers a paired Thought Processor. People use it **through Thought Processor**. Its own web page is now a troubleshooting view only.

> **Branch `alpha-candidate-20261026`.** This is a candidate and has not been promoted: only the owner decides whether it becomes `alpha`. It is built from `experiment/engine-access-pairing-20261026` (`884e386`), which itself builds on `alpha-candidate-20261025` (`7addeee`). It applies the owner decisions of 2026-10-26 (C01–C09).
>
> **Testing on Windows (owner or AI operator such as a Grok bot):** follow **[AGENTS.md](AGENTS.md)**.
> **Building Processor against it:** follow **[docs/API_V1.md](docs/API_V1.md)**.
> **What changed and why, with evidence:** see **[docs/ALPHA_CANDIDATE_20261026.md](docs/ALPHA_CANDIDATE_20261026.md)**.

## Quick start (Windows PowerShell, Go 1.22+)

```powershell
git clone --branch alpha-candidate-20261026 https://github.com/Arameair/identity.git identity-1026
cd identity-1026
.\scripts\build-windows.ps1     # vet + tests + builds identity.exe and identity-probe.exe
.\identity-probe.exe smoke      # self-test: 11 two-process scenarios, ~30 s → SMOKE RESULT: PASS
.\identity.exe                  # runs in the background; no browser opens
```

## What it does — and doesn't

| Does | Doesn't (yet) |
| --- | --- |
| Runs as a background service on `127.0.0.1:7790`, in either start order with Processor | Start, supervise or stop any other program |
| Pairs with Thought Processor. Processor asks, Identity shows a one-time code in **its own window**, and the person types the code into Processor. The pairing survives restarts, and either side can unpair. | Grant feature or source access. Every identity starts with none, and `authorize` always answers **DENIED**. |
| Signs every answer to the paired program (HMAC over the body plus a nonce, and the installation ID) so Processor can detect a wrong service | Make features enforce permissions; there is no receiver-side enforcement yet |
| Keeps **identities** (person, companion, business, project): create, rename, archive, restore. IDs never change. | Per-person administrator sign-in, because secure bootstrap is still open |
| Runs GitHub device sign-in that Processor drives. Shows Thought Engine **install eligibility** (rule `repo-visibility-v0`) separately from any permission to use it. | Keep the GitHub sign-in across restarts (the token is memory-only, by design for now) |
| Stops cleanly when Processor sends the person's **Shut down** choice. Logs why it stopped, and removes `runtime/service.json`. | Grace periods when offline. If Identity is down, callers must block protected actions. |
| Persistent redacted activity log; Copy Diagnostics in the troubleshooting page | — |

## Defaults and flags

| | Default | Change with |
| --- | --- | --- |
| Port | **7790** (127.0.0.1 only) | `-port N` or `IDENTITY_PORT` |
| Profile folder | `%USERPROFILE%\.identity\default` | `-data DIR` or `IDENTITY_DATA` |
| Browser | not opened | `-open` opens the troubleshooting page |
| Started by | `EXTERNAL` | `-managed-by thought-processor:<id>` (or `IDENTITY_MANAGED_BY`) when Processor launches Identity |

Other programs' ports: Thought Processor 7860, Thought-engine 7798.

## Repository map

- `main.go`: flags and console.
- `internal/app/apiv1.go`: the Processor-facing API.
- `internal/entities`: identities.
- `internal/pairing`: codes and tokens.
- `internal/app`: server, troubleshooting UI, diagnostics, Engine-install routes.
- `internal/github`: device flow.
- `internal/profile`: data folder, lock and `service.json`.
- `internal/logbook`, `internal/redact`.
- `cmd/identity-probe`: test client and self-test (it stands in for Processor).
- `docs/`: the API, candidate notes, publisher GitHub App, history.

Tests: run `go test ./...` for unit tests and real-HTTP runtime tests (GitHub stand-ins are labelled TEST FIXTURE). Run `identity-probe smoke` to drive the real binary as separate processes.

## Publisher GitHub App

Normal users never create a GitHub App: the publisher's **public** Client ID is compiled in. See [docs/PUBLISHER_GITHUB_APP.md](docs/PUBLISHER_GITHUB_APP.md).

## Releases

Pushing a tag `identity-v*` runs `.github/workflows/identity-release.yml`. It tests, builds `identity.exe`, the Linux binary, `identity-probe.exe` and `SHA256SUMS`, then publishes a GitHub Release. *The workflow has not been run yet.*
