# New administrator — clean installation acceptance tests

[← Journey](BOOTSTRAP_START_HERE_2026-10-23.md) · [Technical plan](NEW_ADMIN_BOOTSTRAP_2026-10-23.md)

**Run with a fresh throwaway installation/profile and isolated data folder. Never wipe existing owner state.** Start with no configured administrator, no controller running, no GitHub authorization, no Chat-Jimmy. Distinguish LOCAL TEST, MOCK, PINNED LOCAL BUILD, and REAL APPROVED RELEASE.

| # | User action / condition | Observable PASS |
| --- | --- | --- |
| N1 | Launch Identity with empty test profile | Create/restore admin screen, new installation ID on consented create, no unexpected network calls or external resources |
| N2 | Create administrator, restart Identity | Same administrator + installation IDs and stored startup preference; no duplicates |
| N3 | No Thought Controller found, first time | Show identity and exact approved source/version; ask **once** to allow future automatic startup |
| N4 | Deny startup | Identity continues locally, reports unavailable, no process launched |
| N5 | Approve startup | Trusted launcher requests **one** pinned approved target; verifies resulting service ID, version/SHA/protocol and readiness; no model request |
| N6 | Restart Identity when controller is stopped | Consent B reused for same approved target; one startup request, no new prompt, no duplicate process |
| N7 | Restart Identity when controller is already running | Reuse *same compatible instance*, no second launch, no changed service ID |
| N8 | Unapproved/new target, wrong service at port, or changed SHA | Requires fresh confirmation or rejects. Do not trust an unrelated loopback server based only on port |
| N9 | No GitHub login / network offline | GitHub marked UNCONFIGURED/OFFLINE; system remains usable locally; no fabricated repo |
| N10 | Connect an authorized GitHub account | User goes through official provider flow; verified account/scopes, token protected locally; no password/MFA captured by app |
| N11 | Source is private and user has no permission | ACCESS REQUIRED/BLOCKED **before download**; permit explicit authorized path, do not pretend public approved build exists |
| N12 | User approves creation/selection of private entity repo | Check correct account/repo owner, PRIVATE visibility and manifest; no hidden extra repos; no secret committed |
| N13 | Open Thought Controller main dashboard | Each visible identity section displays only its own current thoughts and clear **No thoughts yet** empty state; system identity separate |
| N14 | Create identities TASS and Example Workshop, add mock thought to each | IDs remain distinct; no cross-identity read/update/delete; rejected cross-scope API attempts |
| N15 | Stop and restart both apps | Same admin ID, controller service ID and managed IDs; same thought histories/permissions; no corrupted data or orphan processes |
| N16 | Two different administrator installs from same reference | Same approved code provenance, **different IDs, data, account grants and thoughts**; cannot access each other's private repo |
| N17 | Developer modifies their installation and later checks upstream | Shows local divergence and offered approved updates; no forced merge, branch reset, silent binary replacement or loss of data |
| N18 | Copy status/diagnostics after errors | Exact stage, launcher ownership, source/version/full SHA, entity IDs, grant state, call count, elapsed time and redacted errors; never secrets |
| N19 | No supported secure credential store or incompatible controller | Honest BLOCKED/UNSUPPORTED state and next action, no plaintext fallback or unverified registration |
| N20 | Approved controller starts but AI worker absent | Dashboard usable, worker clearly NOT READY; startup makes **zero AI model calls** |

### Smallest integration demonstration to attempt first

Run N1, N2, N3, N4, N5, N6, N7 using a **fake launcher/controller** with visible simulated status. In parallel Thought Controller team tests N13–N15 with mock thoughts and two entities. Only then connect actual launcher, actual services and GitHub. Do **not** mark N5/N7/N13/N15 as real end-to-end PASS from mock-only runs.

### Required owner report

- Identity branch/full SHA, Thought Controller branch/full SHA, launcher source/commit, OS/machine, effective data roots and whether detached checkout was used
- New-user state (no prior config), source artifact provenance and whether private access was required
- Approved launch policy (target/source, when granted, re-consent behavior), controller PID/port, health/protocol/identity verification
- GitHub auth state, explicitly approved repo operations only, secret-handling checks
- Thought counts per entity, which worker actually answered, simulated vs live origin
- Each test N1–N20 as PASS / FAIL / BLOCKED / NOT RUN with reproduction and timing where possible
- Any error or safety regression, copied redacted diagnostics, independent Stop, and explicit owner decision on Alpha promotion

**A successful build, mock adapter call or HTTP 200 alone is not a completed clean-install test.**