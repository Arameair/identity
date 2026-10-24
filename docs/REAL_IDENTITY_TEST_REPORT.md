# Identity real manager — implementation and verification report

**Branch:** `experiment/identity-real-manager-20261023` (from owner-approved Alpha). **Scope:** standalone real local Identity app; no simulated states or synthetic identity cards; official GitHub CLI session verification and local account binding, identity create/archive/restore, verified read-only private repo association.

## What actually works in the current code

- Automatically creates one durable local System ID; allows adding AI companion, business, person, project and other identity cards with new UUIDs; persists locally, including across a full process restart.
- **Remove** means reversible local archive (with confirmation). **Restore** uses the SAME permanent ID and same repository association. No GitHub delete endpoint/command exists.
- **GitHub sign-in uses the official GitHub CLI**. The owner must first perform `gh auth login --hostname github.com --web` in the OS running Identity. In the app, **Connect signed-in GitHub account** verifies the real GitHub API `/user` via `gh api` and saves only the login name and time. Mismatching active account blocks repository access. Identity **does not import/store GitHub credentials**.
- A card can attach an existing private repository by `owner/name` after read-only verification through `gh api repos/owner/name`. No repo creation or mutation.
- Isolated `PC_DATA/identity-real` storage (not the old Identity V1/wizard profile dirs), actual branch and SHA, PORT, and loopback-only binding; same-directory process lock prevents concurrent writers.
- No fake Connected states, mock controller startup, simulated thoughts or demo event controls.

## Evidence collected in a Linux container

| Check | Result |
| --- | --- |
| Go unit tests: persistence, archive/restore + saved repo, duplicate handling, account mismatch, denied unauthed connect, private repo-only association, invalid/corrupt storage, environment token filtering, no deletion code, same-folder lock | **PASS** (`go test -count=1 ./...`) |
| Linux build `go build -o identity .` | **PASS** |
| Windows amd64 cross-build `GOOS=windows GOARCH=amd64 go build -o identity.exe .` | **PASS** (not executed on Windows) |
| UI JavaScript static syntax `node --check` | **PASS** |
| Real local HTTP service: status → add TASS → remove → restore | **PASS** |
| Attempt GitHub Connect with CLI absent | **PASS** expected error `GitHub CLI (gh) is not installed here`; no fake connected account |
| Real GitHub CLI login and verified repository association | **NOT RUN** (no `gh` installation or GitHub network in test sandbox) |
| Actual Project Controller and Windows start/stop/clipboard UI | **NOT RUN** (must be owner-tested) |

## Manual owner test (Windows Project Controller)

1. Play **Arameair/identity**, branch `experiment/identity-real-manager-20261023`. This is a fresh local data area `PC_DATA/identity-real`, leaving prior wizard test profiles untouched.
2. Create TASS and Business. Confirm distinct IDs and correct types.
3. Click **Remove identity** for TASS, confirm. TASS appears in **Removed identities**. Restore it; ID remains unchanged. **Any GitHub repo associated with TASS must remain untouched.**
4. In the Windows/WSL environment actually running Identity, install the official GitHub CLI if needed, and sign in with `gh auth login --hostname github.com --web`. In Identity click **Connect signed-in GitHub account**; verify the correct login appears.
5. On TASS choose **Attach existing private repo** and enter an existing repository `owner/name`. Verify that the association is saved, and no GitHub repository contents have changed. Do not use a test you cannot safely access.
6. Stop/Play Identity; verify same System ID, card IDs, archive state, account binding and repo association. An invalid/expired GitHub session must fail verification clearly rather than show connected.
7. Confirm **Unlink account** only removes the local association; it does not log out GitHub CLI or change any repository.

## Limitations and next gate

This is a **real local app**, not a complete production-secure account system. A fully in-app one-click OAuth sign-in requires a registered GitHub app and approved permission/credential handling. This version uses a terminal-based official GitHub CLI login, followed by an in-app real verification/link action. The local HTTP interface uses Origin and loopback checks but **does not provide authenticated multi-user access**: do not expose it outside the host. No GitHub repository creation, Thought feature grants, memory integration or safe migration from the old prototype yet.

**Alpha/main/stable untouched.** Do not promote until owner confirms GitHub login, UI, storage and removal behavior.
