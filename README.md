# Identity — real local identity cards and GitHub CLI connection

This branch is a working standalone Go application (no mock/sample data). It maintains a permanent local System ID Card and persistent identities, supports adding, removing (local archiving) and restoring them, and can link an existing **private** GitHub repository to an identity after verifying access through the account linked locally.

**Removing an identity never removes its GitHub repository.** No repository deletion command, endpoint, or API request exists in this application. Its original identity ID and repo link remain in the archive and can be restored.

## GitHub sign-in (real, not simulated)

Identity uses the official GitHub CLI as a credential broker. It does **not** read or save a token. On the machine running Identity, first install [GitHub CLI](https://cli.github.com/) and sign in with the official browser flow:

```shell
gh auth login --hostname github.com --web
```

In Identity click **Connect signed-in GitHub account**. Identity verifies the active account by calling the live GitHub API via the official CLI, then stores *only* the account name and linkage time on the System ID Card. Every repository attachment rechecks the active account matches the linked account. A GitHub username alone does not grant your other identities access to that account. No GitHub write requests are made.

**Limits:** Login is currently initiated in a terminal via the official CLI, not embedded in Identity's web page. A full in-app GitHub App OAuth flow requires an Identity app registration, protected credentials and permission review. On Linux/WSL the CLI login may be different from the one on Windows. GitHub CLI may fall back to plaintext credential storage if no OS credential store is available; inspect its auth status and do not use this test build with sensitive accounts where that is unacceptable.

## Run with Project Controller

Play the new experiment branch. Its `marquee.json` builds it using Go and starts the binary on the assigned `PORT`; data lives in `PC_DATA/identity-real`, not the earlier Identity V1 test data folders. It binds to `127.0.0.1` only and shuts down on Stop. A same-data-folder process lock prevents another instance from writing the store concurrently; if a crash leaves a stale lock, first confirm no instance is running before removing it.

## Standalone

```shell
go build -o identity .
PORT=7796 ./identity -data "$HOME/.identity-real-test"
```

Open `http://127.0.0.1:7796/`. Windows PowerShell: `go build -o identity.exe .`; `$env:PORT='7796'; .\identity.exe -data "$HOME\identity-real-test"`.

## Supported operations

- Create a new AI companion, business, person, project or other identity with a permanent UUID.
- Remove an identity from active cards (archive locally) and restore it with its original UUID and repo association.
- Connect/unlink a real existing official GitHub CLI account. Unlink affects Identity only, not the machine's GitHub CLI credential.
- Verify and attach an existing **private** repository using its `owner/name` string (read-only GitHub operation).
- Restart and preserve ID cards, account linkage, archived records, and repository associations.

## Not implemented yet

Integrated one-click browser OAuth, GitHub App installation, creating private repositories, Thought Processing grants, authenticated multi-user HTTP access, automatic release updates. The application has no mock or simulation UI. It is a local experimental app, not yet a production-secure central account broker.

## Safety and testing

`go test ./...` checks the single-writer folder lock, archive/restore and persistence, duplicate handling, account mismatch, denied anonymous actions, private repo checks, and the absence of repository deletion operations. Linux Go tests pass; Windows cross-compilation passes. The developer has **not** tested this on the owner's Windows machine or in the actual Project Controller UI. Alpha/main/stable must remain untouched unless the owner approves promotion.


## Thought Controller — discover instead of launching

**This experiment deliberately does not start, install, stop or manage Thought Controller processes.** Use the Thought Controller tile in Project Controller to control its lifecycle.

1. Open **Identity → Thought Controller**. Identity checks the configured loopback port (default **7797**) and repeats the read-only check every 5 seconds.
2. If unavailable, the page says **NOT RUNNING — Start Thought Controller in Project Controller**.
3. If running on a different port, click **Find on this computer**. This checks **127.0.0.1 ports 7778–7899**, recognizing only services that answer with the Thought Controller identification at `/api/health` or (older build) `/api/heartbeat`. If the assigned port is outside that range, copy it from Project Controller and enter it manually.
4. Click **Use this port**. The selection is saved in `<Identity data directory>/controller-discovery.json` and survives restart.
5. Once recognized, Identity shows **DETECTED** and **Open Thought Controller ↗**. Clicking Open loads the other application's existing web UI; no subprocess or fake registration is created.

A TCP port being open by itself is **not** proof of Thought Controller. A different application or unresponsive controller shows **OTHER SERVICE OR NOT READY** and does not produce an Open link. The discovery is **not authenticated**; it only establishes that a local service identified itself as Thought Controller. Thought feature grants and secure data sharing are a separate later integration. If the detected Thought Controller reports simulated thought behavior, Identity labels it as simulated without itself simulating the process.

Existing local System ID Cards, archives and GitHub associations are preserved **when the new branch uses the same Identity data directory**. Project Controller may assign a different `PC_DATA` per run/branch; if so, explicitly set `IDENTITY_DATA` to the previous Identity data directory. Never assume an empty new folder means the old identity cards were deleted.

**Tests:** `go test -race ./...`, `go vet ./...`, Windows cross-compilation and a browser-script syntax check were run against the same local implementation before posting this experiment. Owner Windows/Project Controller integration is **NOT YET RUN**. None of these tests exercise live Identity authorization against the separate Thought service.
