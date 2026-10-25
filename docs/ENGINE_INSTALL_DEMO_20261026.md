# Demo — Thought Processor installs Thought Engine only if Identity allows it

**Status:** experiment for comparison ("which team's approach is best"). Implements the plan posted on [Thought-Processor#13](https://github.com/Arameair/Thought-Processor/issues/13#issuecomment-6400140494) and the friction fixes from [identity#19](https://github.com/Arameair/identity/issues/19). Not promoted anywhere.

| Repo | Branch | Based on |
| --- | --- | --- |
| Arameair/identity | `experiment/engine-access-pairing-20261026` | `alpha-candidate-20261025` |
| Arameair/Thought-Processor | `experiment/install-thought-engine-20261026` | `alpha-candidate-20261025` |
| Arameair/Thought-engine | `experiment/release-workflow-20261026` | `experiment/thought-engine-v1-20261023` (code unchanged; adds the release workflow) |

## What it does

1. **Pair once.** In Identity, *Manage → Thought Processor → Pair Thought Processor* shows a one-time code (8 characters, 5 minutes, single use). Type it into Processor's **Pair with Identity** box. Identity returns a 256-bit token: Identity keeps only its hash, and Processor keeps it in its profile (0600). Pairing survives restarts of both programs.
2. **Ask.** Processor calls `GET /api/v1/features/thought-engine/access` with the token. Identity answers from the person's live GitHub sign-in under rule **repo-visibility-v0** (the account can see `Arameair/Thought-engine`). The possible answers are `ALLOWED`, `NOT_ALLOWED`, `NOT_SIGNED_IN`, `SIGN_IN_EXPIRED` or `CHECK_FAILED`, each with a plain reason and next step. Every answer carries an **HMAC proof** keyed by the token, and Processor rejects anything that doesn't verify.
3. **Install.** If allowed:
   - Identity reads the latest Thought-engine **GitHub release** and its `SHA256SUMS`, then **streams** the file. The GitHub token never leaves Identity.
   - Processor checks size and SHA-256, then installs into `<processor profile>/features/thought-engine/<tag>/` with an `install.json` (decision ID, account, rule).
   - No admin rights are needed, and installing doesn't start anything.
4. **Not allowed** → one sentence explaining why, plus what to do. No download is ever attempted.

## Evidence (developer Linux sandbox; GitHub was a TEST FIXTURE)

Real Identity and real Processor binaries ran as separate processes. GitHub was replaced by `scripts/fixture_github_engine.py`, which signs in as chosen accounts and serves a release whose asset is the **real Thought-engine binary** built from the branch. Every Identity view labels that mode **TEST FIXTURE**. Screenshots are in [demo-engine-install-20261026/](demo-engine-install-20261026/).

| Check | Result |
| --- | --- |
| Pair (code from Identity typed into Processor) | PASS: Identity logs `pairing.paired`; Processor shows **Paired** |
| Owner account → access | PASS: `ALLOWED`, rule repo-visibility-v0 |
| Install | PASS: 10,388,205 bytes; installed SHA-256 `08b75a50…e0e13e` equals the built binary's SHA-256 |
| Switch to a second account without access | PASS: `NOT_ALLOWED`, naming the account and what to ask for. The installed copy is kept, with a warning |
| After uninstall, second account | PASS: no Install button; a forced `POST /api/te/install` ends in `NOT_ALLOWED` **before download** (Identity log has no `download_streamed`) |
| Identity restarted | PASS: Processor shows `NOT_SIGNED_IN` — "Open Identity and press Connect GitHub"; the pairing still works |
| Secrets | PASS: the pairing token appears in no log, console, Copy Diagnostics or Copy Context; Identity stores only its hash |

**Automated:**
- Identity has `TestEngineAccessPairingAllowDenyDownload`, covering pairing refusals from a web page or with a wrong code, missing or wrong tokens, every access state, the owner-without-app-installed hint, the release proof, the download checksum, a denied account getting no release or download, restart and unpair. It also has pairing unit tests.
- Processor has installer tests: allowed, checksum mismatch leaves nothing behind, denied never downloads, forged answer rejected, unreachable, tampered file detected, pairing file 0600.
- `go test -race ./...` is clean in both repos.

**Not tested here:**
- Real github.com: the sandbox blocks GitHub's login endpoints.
- A real published release: no tag was pushed.
- Windows: the code cross-compiles; Processor's tests skip the file-mode check on Windows.

## What the owner needs to do for a real run

1. Install Identity's GitHub App on **Thought-engine** (that's why it shows UNAVAILABLE today), and add **Contents: read-only** to the app (needed to read releases of a private repo).
2. Pick the Thought-engine commit for the first release and push a tag `engine-v…` on `experiment/release-workflow-20261026`, or merge the workflow first.
3. Build Identity and Processor from these branches. Pair, check access, install.
4. Try your brother's account before and after giving it access to Thought-engine.

## Owner decisions still open

Personal account vs an organization · whether Thought-engine stays private · the first release commit · approving Contents read · what happens to an installed copy when access is removed (currently: kept, with a warning) · whether typing a pairing code is OK for a new user.
