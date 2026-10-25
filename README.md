# Identity

**Identity** is the first program a person opens. It confirms **who they are** through GitHub, shows the **features** the publisher (Arameair) offers and honestly what state each one is in, and finds a **Thought Processor** running on the same computer. Then it settles into a small monitoring view: status on top, a live activity log, and **Copy Diagnostics**.

> **Branch `alpha-candidate-20261025`.** Built from the 2026-10-25 home trial, with every problem found that day fixed (list below). The owner decides whether it becomes `alpha`.
> **This branch (`experiment/engine-access-pairing-20261026`) adds:** pairing with Thought Processor and the Thought Engine install-access API. See [docs/ENGINE_INSTALL_DEMO_20261026.md](docs/ENGINE_INSTALL_DEMO_20261026.md).
> **AI operators (e.g. a Grok bot):** follow **[AGENTS.md](AGENTS.md)** — exact steps, expected results, and what only the human may do.

## Quick start (Windows PowerShell, Go 1.22+)

```powershell
git clone --branch alpha-candidate-20261025 https://github.com/Arameair/identity.git
cd identity
.\scripts\build-windows.ps1        # vet + tests + build identity.exe
.\identity.exe                     # opens your browser on the welcome screen
```

Thought Processor is a separate program (repo `Arameair/Thought-Processor`); start it first, on port 7860. [AGENTS.md](AGENTS.md) has both programs step by step.

## What it does — and doesn't

| Does | Doesn't (yet) |
| --- | --- |
| GitHub sign-in through the publisher's GitHub App (device flow; no password or client secret; sign-in kept in memory only) | Submit access requests to the publisher (drafts stay local, marked **NOT SUBMITTED**) |
| Shows the account GitHub confirmed, the publisher-app installations it can see, and which feature repositories are visible | Grant features (nothing is ever shown as **GRANTED**) |
| Finds Thought Processor read-only via its public health page (**DETECTED — SELF-REPORTED, UNVERIFIED**) and links to it | Sign you in to Thought Processor (it has its own operator code) or start/stop it |
| Persistent, redacted activity log; Copy Diagnostics safe to paste into a chat | Persist the GitHub sign-in across restarts (by design) |

## Defaults

| | Default | Change with |
| --- | --- | --- |
| Port | **7790** (127.0.0.1 only) | `-port N` or `IDENTITY_PORT` (`0` = any free port) |
| Profile folder | `~/.identity/default` (`%USERPROFILE%\.identity\default`) | `-data DIR` or `IDENTITY_DATA` |
| Thought Processor address | asked during setup (pre-filled `127.0.0.1:7860`) | `-processor 127.0.0.1:7860` |

Other programs' ports: Thought Processor 7860, Thought-engine 7798.

## GitHub App (publisher, once)

Normal users never create a GitHub App. The publisher's app **public** Client ID is compiled in from `internal/publisher/publisher.go`. If it is empty, the build says so and offers a guided "set up your own app" option instead. Setup and permissions: [docs/PUBLISHER_GITHUB_APP.md](docs/PUBLISHER_GITHUB_APP.md).

## Fixed since the 2026-10-25 trial

| Problem seen | Fix |
| --- | --- |
| GitHub said "all set" but Identity never finished signing in: GitHub kept answering `slow_down` | Polls at GitHub's returned interval **+2 s** margin, never at the exact minimum (regression test included) |
| While waiting, failed checks were silent | Every change in GitHub's answer is logged; the waiting screen shows "last checked … GitHub says …"; Copy Diagnostics includes it |
| Cancelling and reconnecting left an approved-but-old code | Replaced sign-ins stop polling; the screen explains that an older code was approved |
| Users would have had to create their own GitHub App | Publisher's Client ID built in; the own-app guide moved to *Manage → Identity → Advanced* |
| The guide asked for more GitHub permission than needed | Metadata read-only only |
| Switching tabs in *Manage* lost panels | Fixed |
| Default port 7798 clashed with Thought-engine | Identity now defaults to 7790 |
| Build label said "UNKNOWN" branch under Project Controller | Branch taken from `-ldflags`, else Project Controller's `PC_BRANCH` |

## Releases

Push a tag `identity-v*` and the workflow `.github/workflows/identity-release.yml` tests, builds `identity.exe` + Linux binary + `SHA256SUMS`, and publishes a GitHub Release. It refuses to release a build without the publisher Client ID. *(Not yet run; the first tag is its first test.)*

## Repository map

`main.go` flags and startup · `internal/app` HTTP server, UI, diagnostics · `internal/github` device flow · `internal/discovery` Thought Processor probe · `internal/features` catalog and local drafts · `internal/logbook` persistent log · `internal/profile` data folder and lock · `internal/redact` secret removal · `internal/publisher` built-in publisher info · `scripts/` build and test helpers · `docs/` decisions, publisher setup, trial history.

Tests: `go test ./...` (unit + real-HTTP runtime tests; GitHub and Processor stand-ins are labelled TEST FIXTURE).
