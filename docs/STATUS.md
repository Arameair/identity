# Current status and branch ledger

**Reviewed:** October 25, 2026. This page distinguishes **source present**, **test evidence**, **real-world acceptance**, and **proposed architecture**. It is not a release announcement.

## Executive status

**Experimental runnable app: yes, on candidate branches. Released secure Identity authority: no.** The default `main` branch contains documentation only. Recent Go candidates implement a first-run UI, GitHub device-flow integration, local state/diagnostics, and a **read-only** Thought Processor status probe. Authenticated service pairing, publisher feature-request submission, real feature grants and grant enforcement remain unimplemented or unverified.

The immediate goal is to let another person build and run Identity with a separate Thought Processor instance on a clean computer, then report a traceable PASS/FAIL result without relying on the author's Project Controller.

[Try the candidate](TRY_THE_PROTOTYPE.md) · [Project case study](PROJECT_JOURNEY.md) · [Acceptance checklist](https://github.com/Arameair/identity/blob/experiment/identity-consolidated-home-trial-20261025/docs/HOME_TRIAL_ACCEPTANCE_2026-10-25.md)

## Branch and release lanes

| Branch/lane | What exists | Acceptance |
| --- | --- | --- |
| [`main`](../README.md) | Public README, architecture and project record; **no Go application in this branch** | Documentation maintained; no software release implied |
| [`alpha`](https://github.com/Arameair/identity/tree/alpha) | Older owner-controlled README/design baseline | Not equivalent to latest candidate |
| [`alpha-candidate-20261025`](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) | Standalone Go candidate with Windows build script, tests, app code, and paired-processor runbook | **PENDING owner promotion and reproducible clean-machine acceptance** |
| [`experiment/identity-home-claude-20261025`](https://github.com/Arameair/identity/tree/experiment/identity-home-claude-20261025) | Independently developed Prototype B; evidence report and Windows trial guide | Developer Linux runtime evidence; owner Windows and real GitHub login not established in evidence report |
| [`experiment/identity-home-gpt6-20261025`](https://github.com/Arameair/identity/tree/experiment/identity-home-gpt6-20261025) | Separate requested prototype lane | Do **not** assume a successful implementation or owner acceptance from the branch name |
| [`experiment/identity-gpt6-alpha-candidate-20261025`](https://github.com/Arameair/identity/tree/experiment/identity-gpt6-alpha-candidate-20261025) | Additional candidate branch | Experimental; compare exact SHA and tests before using |
| [`experiment/alpha-candidate-ux-fixes-20261025`](https://github.com/Arameair/identity/tree/experiment/alpha-candidate-ux-fixes-20261025) | Additional UX-fix experiment | Experimental; not an automatic Alpha promotion |
| `stable` | Not found in the branch inventory checked October 25 | **No verified stable snapshot** |

**Provenance caution:** Some experimental README files contain inherited descriptions of other branches. Verify `git branch --show-current` and `git rev-parse HEAD` before reporting a result. Branch names and README claims alone are not executable evidence.

## Recent implementation vs. verification

| Capability | Source or report | What can be claimed today |
| --- | --- | --- |
| Standalone Go application, loopback UI and first-run flow | [Candidate source](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) | **CODE PRESENT**; candidate documents Windows commands. Clean owner Windows acceptance is pending |
| GitHub App device-code sign-in, account visibility | Candidate `internal/github`, `internal/publisher`; [publisher setup](https://github.com/Arameair/identity/blob/alpha-candidate-20261025/docs/PUBLISHER_GITHUB_APP.md) | **CODE PRESENT / REAL LOGIN NEEDS ACCEPTANCE**; device-flow fixture testing and candidate fixes exist; no secret-bearing client ID required |
| GitHub polling and stale-code fixes | Candidate [README](https://github.com/Arameair/identity/blob/alpha-candidate-20261025/README.md) | **FIX CLAIMED IN CANDIDATE**, including regression tests; not a verified end-user sign-in pass |
| Feature list and access-request draft | Candidate `internal/features`; [Prototype B report](https://github.com/Arameair/identity/blob/experiment/identity-home-claude-20261025/docs/HOME_TRIAL_EVIDENCE_CLAUDE.md) | **LOCAL DRAFT ONLY**; **NOT SUBMITTED**, **NOT GRANTED** |
| Thought Processor detection | Candidate `internal/discovery`; Prototype B report | **READ-ONLY LOCAL DISCOVERY**; the other service's reported name is **UNVERIFIED**, not paired or authorized |
| Persistent activity log and redacted Copy Diagnostics | Candidate `internal/logbook`, `internal/redact`; Prototype B report | Source and development test evidence; verify secret redaction and persistence again on real Windows |
| Independent Identity/Processor stop and restart | Prototype B [Linux results](https://github.com/Arameair/identity/blob/experiment/identity-home-claude-20261025/docs/HOME_TRIAL_EVIDENCE_CLAUDE.md) | **LOCAL DEV-LINUX RUNTIME VERIFIED FOR B**; not proof of every candidate or operating system |
| Secure local administrator bootstrap and recovery | [Target first-run model](FIRST_RUN.md) | **DESIGN / INCOMPLETE**, not proven authorization |
| Authenticated service enrollment, scoped grants and receiver-side enforcement | [Interfaces](INTERFACES.md), [Security](SECURITY.md) | **DESIGN ONLY / NOT VERIFIED** |
| Published signed, generally usable binaries and releases | Candidate release workflow source | **WORKFLOW PRESENT**, but first real release execution not established |
| Thought Engine, Memory and other optional features | Separate repositories, future integration plans | **NOT DEPLOYED BY IDENTITY** |

## What the October 25 trial has shown — and has not

Prototype B's report documents a **real pair of processes on a developer Linux machine**, source tests, private local profiles, independent stops/restarts, sample diagnostics and mocked GitHub responses. Its author explicitly marks **real GitHub sign-in and owner Windows operation as not tested** at the time of that report.

A later [Alpha candidate](https://github.com/Arameair/identity/tree/alpha-candidate-20261025) documents attempted fixes to problems found during owner testing: GitHub responding `slow_down`, stale device approvals, insufficient waiting-state visibility, a conflicting port, and tab-state issues. The presence of fix descriptions **does not establish that the owner reran and passed all cases**.

The next meaningful test is the clean Windows walkthrough with real GitHub approval, an independently running Processor, offline/restart checks and a redacted operator report. These steps are in [Try the prototype](TRY_THE_PROTOTYPE.md).

## Terms used in this ledger

- **DESIGN:** intention or proposed behavior, without a working implementation.
- **CODE PRESENT / UNVERIFIED:** source claims a behavior; no sufficient reproducible live evidence yet.
- **TESTED WITH FIXTURES:** tests use stand-in GitHub or Processor responses, not the real provider.
- **LOCAL RUNTIME VERIFIED:** actual local process state was observed, with machine/branch scope recorded.
- **LIVE INTEGRATION VERIFIED:** real external service behavior and exact authenticated operation were observed.
- **OWNER ACCEPTED:** the owner explicitly accepted results; not equivalent to branch existence or passing CI.
- **FAILED / BLOCKED / NOT RUN:** retain these results explicitly.

Do not relabel **detected** as **trusted**, **account logged in** as **feature granted**, **local draft** as **submitted request**, or **build passed** as **installation accepted**.

## Engineering and review links

[Engineering story](PROJECT_JOURNEY.md) · [Roadmap](ROADMAP.md) · [Decisions](DECISIONS.md) · [Workflow](WORKFLOW.md) · [Experiment report template](EXPERIMENT_TEMPLATE.md) · [Independent review](REVIEW.md)

All code changes to owner-controlled release lanes require explicit approval; improving this documentation does not move `alpha` or release software.
