# GPT-6 candidate — independent side-by-side evaluation (2026-10-25)

## Candidate identity

- Repository: `Arameair/identity`
- **Base, unchanged:** `alpha-candidate-20261025` at `7addeee048c76f95a6abc9a0ef390a85ac793055`.
- **GPT-6 branch:** `experiment/identity-gpt6-alpha-candidate-20261025`.
- This is an **experimental alternative**, not Alpha and not an approved release.
- Prior UX trial branch `experiment/alpha-candidate-ux-fixes-20261025` and other collaborators' branches are separate.
- See issues #17, #18 and #19 for original trial observations.

## Rationale and scope

The primary failure mode was treating `authorization_pending` as though Identity knew *why* GitHub had not approved the request. A flagged GitHub account can be refused on GitHub's own page, without Identity receiving a distinct device-flow response. Identity must **not falsely claim** it detected an account flag. After 20 seconds the screen directs the user to inspect GitHub for a warning, rejection, or wrong/unfinished code.

This candidate also guards the final token-to-account transition so an **old in-flight OAuth response cannot restore a cancelled session**. Fixture regressions cover pending answers and late approvals. The UX tells a new user what action to take for missing repo visibility, missing publisher-app installations, local draft requests, and Processor's self-reported status.

Deferred: feature grants, Thought Engine installation, Engine releases/checksums, feature marketplace, creating new public releases, and automatic Alpha promotion.

## Run side-by-side in Project Controller

Select **Identity → Add Branch** and choose `experiment/identity-gpt6-alpha-candidate-20261025`. Keep the other team's candidate in a separate pane/profile. Compare with the *same* fresh-account and Processor states. Do not share profile folders across live instances.

## Standalone Windows reproduction (without Project Controller)

Open PowerShell with Go and Git installed. Use a **new profile** so prior setup does not hide first-run problems:

```powershell
cd $HOME
git clone --branch experiment/identity-gpt6-alpha-candidate-20261025 --single-branch https://github.com/Arameair/identity.git identity-gpt6-trial
cd identity-gpt6-trial
go vet ./...
go test ./...
go build -ldflags "-X main.buildBranch=experiment/identity-gpt6-alpha-candidate-20261025" -o identity-gpt6.exe .
.\identity-gpt6.exe -port 7791 -data "$env:USERPROFILE\.identity\gpt6-trial" -processor 127.0.0.1:7860
```

Open the **local private link printed by Identity**. Do not paste the `#key=` part, GitHub device codes, OAuth tokens, cookies, or Thought Processor's `#code=` in messages, issues, screenshots, or commits.

Start Thought Processor **separately** using its own runbook. Identity only probes its health endpoint. Stopping one must not stop the other.

## Candidate comparison rubric (use on both candidates)

Record **PASS / FAIL / NOT RUN**, the exact observed behavior, branch SHA, OS, and safe Copy Diagnostics. No inferred PASS from fixture tests.

| Case | Expected behavior |
| --- | --- |
| Normal GitHub sign-in | Account chooser and newest code are clear; GitHub approval eventually yields a verified GitHub account (real test required). |
| Waiting >20 seconds | Displays an actionable "check GitHub tab" explanation **without alleging a confirmed account flag**. |
| GitHub refuses flagged account | Identity never claims success, tells person to inspect GitHub banner; issue is not silently attributed to an older code. |
| Cancel during approval | Old approval cannot verify the cancelled attempt. |
| Network failure and slow_down | Error or delay is visible; polling follows GitHub's interval and later recovers. |
| Feature not visible | User gets next action, no feature granted and no claim that a missing repo is proof of publisher refusal. |
| Access-request draft | Continue does **not** transmit it; it remains local. |
| Processor detected | "Detected" remains self-reported/unverified; user knows the separate operator code is required. |
| Processor stopped | Identity stays up and eventually reports Processor unavailable; reverse stop preserves Processor. |
| Restart | Same System ID and saved address; GitHub login no longer authenticated until reconnect. |
| No Project Controller | Fresh local checkout builds, runs and connects to GitHub without preexisting credentials or projects. |

## Automated evidence

Dedicated workflow: `.github/workflows/identity-gpt6-candidate.yml`, branch-limited and non-publishing.

- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
- `node --check internal/app/web/app.js`
- Windows and Linux amd64 builds with SHA256 sums; short-lived Action artifact if successful.

These checks do **not** validate a real GitHub authorization. Do not promote without the owner's manual comparison and approval.

## Known limits

- Identity cannot read GitHub's browser banner or distinguish a flagged account from other reasons for a pending device code.
- Repository access is checked using the signed-in account's GitHub API response; that alone never grants a feature.
- Identity detects but does not authenticate Thought Processor; there is no Identity-to-Processor trust contract yet.
- This candidate intentionally does not implement Engine installation or Identity grants.
- Existing local-only drafts are not sent to the publisher.

## Owner decision

Promote only after independent comparison demonstrates fewer user-blocking failures *and* no regressions. Do not overwrite `alpha` automatically. Preserve the original candidate and test logs for rollback.
