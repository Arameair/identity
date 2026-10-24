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

## Approved local Thought Controller startup (experimental)

Identity can launch **one approved executable** and stop only a process it owns. This is real process startup; it does not build, download, update or sign Thought Controller's code.

1. Use **the same Identity data folder** as your previous Identity manager if you want to preserve your account and cards. Project Controller may allocate different PC_DATA per branch; use `-data <existing Identity directory>` or `IDENTITY_DATA` explicitly.
2. Build a trusted Thought Controller version for the same OS. Example: check out the standalone [v1 repair](https://github.com/Arameair/Thought-Controller/tree/experiment/v1-repair-20261023) and run `go build -o thought-controller.exe .` on Windows (or `go build -o thought-controller .` on Linux). This is a separate **unapproved experimental** Thought engine; it is not the new Thought module.
3. In Identity, open **Thought Controller**, enter the **full absolute path** to its executable, choose a port (default 7797), check **Start automatically when Identity opens**, click **Approve this executable** and then **Start if stopped**.
4. Identity pins the executable SHA-256, records approval in `<Identity data>/controller-approval.json`, and runs the controller with its **own data folder** `<Identity data>/thought-controller-managed/`. Controller startup logs are at `<Identity data>/thought-controller-launch.log`. No GitHub repository is changed.
5. On Identity Stop, the controller it launched stops. On next Play using the same data folder, Identity automatically starts that exact approved executable if the port is free. If the executable was rebuilt or replaced, startup is blocked until you approve the new file.

**Important:** The `first-user-dashboard` Thought Controller build remains a SIMULATED thought host even when the **process startup is real**. The v1-repair controller has an experimental thought engine (live worker separate), but neither has live secure Thought-feature permission grants from Identity. The independent Thought repo integration comes later. Identity never takes over another already running instance, never triggers Project Controller's Play button, and never deletes GitHub repos.
