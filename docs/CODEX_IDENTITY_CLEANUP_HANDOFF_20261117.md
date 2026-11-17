# Identity repository — Codex cleanup and workflow handoff (2026-11-17)

Status: **TRIAGE / NOT OWNER APPROVAL**. Documentation only. No source change, merge, PR/issue closure, deployment, grant, credential, or repository setting change is authorized by this note.

## Purpose and evidence
GitHub review on 2026-11-17 found 14 open PRs and 6 open issues. Default branch: `main`. The `main` README still describes the October 27 documentation-only state; do **not** infer it contains installed Identity code. Preserve installed pins independently of default branch.

Read alongside the agreed workflow: Thought-Processor `docs/WORKFLOW.md` in draft policy PR #118 and rollout issue #119. Process: issue -> focused branch -> draft PR -> tests / risk-based independent review -> owner acceptance -> main -> separately approved stable release. This note does not replace that policy.

## A. Protect the active authorization stack
Observed PR base/head chain (each PR is open, unmerged on GitHub):
- #38: `alpha` -> `experiment/skill-grant-lesson-workspace-20261105`; head `87dd9243`. One allowlisted skill grant.
- #40: #38 head -> `experiment/junction-grant-lesson-metadata-20261105`; head `7da7f7c5`.
- #42: #40 head -> `experiment/nudge-draft-grant-20261108`; head `64fdf8e8`.
- #43: #42 head -> `experiment/nudge-process-stop-20261108`; head `cfc4079b`. Single-use bound decisions.
- #45: #43 head -> `experiment/stage3b-experiment-run-20261112`; head `ee7ba1b0`. Experiment-run authorization. Issue #46 cites this as last owner-accepted installed Identity pin; **verify locally**.
- #47: #45 head -> `feature/discord-reply-bound-decision-20261116`; head `9c36f6e4`. Discord reply grant and redeemer. **NEEDS REPAIR; do not merge**.

These are ancestry/base observations from PR metadata, not proof that all behavior is accepted or deployed. Do not squash/close/rebase the stack blindly. Inventory exact code and tests at accepted/deployed revisions first; agree on main migration plan and compatible Core/Engine pin with owner.

Other related PRs:
- #35 -> #36: learning grant / learning card UX, older stacked changes; compare against accepted runtime and issue #37 before deciding whether to preserve, adapt, or close.
- #33: optional GitHub device sign-in / read-only lesson; PR description says CI was queued at updated head and live OAuth trial not run. Verify prior to disposition.
- #31: Go-core policy candidate targeting main; security-sensitive; verify whether the installed service uses this policy and how it relates to current grant authority. Do not merge by age.
- #41: documentation refresh on main, README + docs/README.md + docs/STATUS.md; reconcile against current installed pins and governance.
- #29: older TASS mind mirror/outbox; compare against current source and restrictions.
- #28: Liaison team evidence rule; decide if unique instruction survives elsewhere.
- #22: older admin passphrase/autofill UI repair; test whether still applicable; do not dismiss reported bug without evidence.

## B. Open issues — proposed dispositions, not closure decisions
- #46 Discord reply Milestone A: **BLOCKED / ACTIVE SECURITY** until #47 remediation and door integration verification; Core issue Thought-Processor #127.
- #37 obsolete developer test UI: verify current owner screen, preserve any developer-only diagnostic deliberately; routine cleanup candidate.
- #30 discussion-only multi-identity/token boundaries: retain as design record; do not pretend implemented.
- #21 repository/portfolio polish: compare actual docs and release policy; may be closed only with evidence and owner decision.
- #15 observation grant/revocation design gate: confirm relevant code path and whether superseded; authorization risk if still open.
- #9 actual secure GitHub account management: feature scope item, not automatically a cleanup blocker; classify backlog vs delivered after evidence.

## C. PR #47 — mandatory independent security repair gate
Source: independent audit comment https://github.com/Arameair/identity/pull/47#issuecomment-6914380703 (base `ee7ba1b0`, head `9c36f6e4`, synthetic merge `63013fee`).
1. Reject Discord `check:true` after redemption (ideally handler and Store) while retaining existing Engine semantics. Test after consumed decision and beyond 120s. Discord door must transmit only after successful consuming `mode=redeem`.
2. Add deterministic barrier-start concurrent redemption test for same decision: **exactly one** successful consumption; run `go test -race`.
3. Door-v2 contract/fixture tests: authenticate persisted inbox/outbox correlation; validate nonempty/newline-free identifiers, approved sender and channel, exact outbound UTF-8 hash, reply reference and destination; deny transformed payloads, mentions, retries after ambiguous send, and stale/mismatched binding.
4. Record inherited caveats: audit append failures ignored, and service-key config does not reject duplicate key values. Evaluate as separately scoped security follow-ups, not silently fixed.
5. Re-audit repaired exact head and integration pins; passing synthetic-merge CI alone is insufficient. No release or grant changes until owner approval.

## D. Codex at home — execution order
1. Record `git rev-parse HEAD`, branches, dirty worktrees and accepted/installed SHA for Identity, Core, Engine. Preserve backups and existing owner data. Check live Docker config and key scopes **without exposing secret values**.
2. Run applicable Go tests, race detector and targeted negative authorization tests on exact candidate revisions. Verify lock, grant revoke, wrong caller, wrong binding, one-use expiry, duplicate service key provisioning, and audit failure handling.
3. Repair #47 in a focused branch/PR only with owner authorization and independently audit it. Keep #45 baseline intact.
4. Test cross-service policy, Core mint -> Identity authorize -> door redeem -> single Discord outbound reply using safe offline fixtures before any live channel use. No autonomous changes.
5. Inventory which old PR diffs already exist in accepted source versus which contain unique security/tests/docs. Record evidence per PR before proposing **specific** closures or archive labels. Do not delete historical branches or force-change main.
6. Correct main/docs/CI and branch policy per approved Thought-Processor #118 / #119 rollout, keeping stable a separately approved release.
7. Demonstrate two low-risk parallel changes through the agreed workflow, plus a rollback/checkpoint; close only on owner's explicit acceptance.

## Completion gate
Cleanup is ready to close only when all PRs/issues have evidence-backed classifications and owner decisions, authorization repairs and tests are verified, accepted and installed source pins agree with release records, docs reflect reality, and a safe main/stable migration has been approved and demonstrated. This handoff documents outstanding work; it does NOT claim cleanup or runtime verification complete.
