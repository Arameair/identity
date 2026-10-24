# First-run Experience launch — Identity as the user's starting point

**Owner-led design and implementation handoff · 2026-10-24**  
**Status:** PLAN ONLY. Current `main` and `alpha` are documentation-first; current Go manager/discovery exists on separate experimental branches, and **real grant issuance to Thought Engine is not yet working**.  
**Tracking:** [Identity #16 — Start Experience](https://github.com/Arameair/identity/issues/16) and [Identity #15 — grant decision](https://github.com/Arameair/identity/issues/15).  
**Thought Processor handoff:** [First Experience brief](https://github.com/Arameair/Thought-Processor/blob/experiment/first-run-experience-brief-20261024/docs/FIRST_EXPERIENCE_DEMO_2026-10-24.md), [issue #9](https://github.com/Arameair/Thought-Processor/issues/9).  
**Experiment policy:** owner approval required for promotions; build new candidate from current Alpha, do not auto-merge into Alpha/Stable/Main.  
**Attribution:** human-directed architecture with AI-assisted planning.

## One-sentence user experience

**I open Identity, choose Start Experience, Identity starts or attaches to the exact Thought Processor version selected for the demo, helps prepare its small approved set of services, and opens one working place where I can see what happened.**

This is an explicit user-requested **demo launch experience**. It refines—not repeals—the earlier principle that **opening Identity by itself must never silently start all optional features**.

> **Identity owns the organization and authority of the system. Each feature owns its actual work.**
>
> **Identity decides; Thought Engine enforces; Thought Processor connects, tests, and explains.**

## UX in the Identity window

```text
IDENTITY                                   local profile: First Experience

Status: READY FOR OPTIONAL FEATURES
Installation: <durable install ID>        Administrator: <authenticated owner>
Data location: <safe isolated root>       Registered features: <real count>

[ Start Experience ]

What will be started or attached?
  ✓ Thought Processor   repo Arameair/Thought-Processor
    selected EXPERIMENT branch @ <pinned exact SHA, not just label>
  ✓ Thought Engine      selected owner test branch @ <pinned SHA>
  ✓ Demo Source         distinct test service S
  ◦ Chat-Jimmy          optional if a genuinely working instance is available

[Start selected session]  [Change selection]  [Cancel]

Startup timeline (live):
  Identity       ready / failed
  Processor      installed / downloading? / building? / starting / ready / blocked
  Engine         same explicit evidence
  Test Source    authenticated / ready
  Worker         ready / unavailable / unconfigured
  Grant          not requested / approved / denied / unavailable
[Open Thought Processor] [Copy Context] [Stop Demo]
```

This is a **design example only**, not a claim these statuses or steps are already implemented. Real UI must reflect evidence, not fill green badges by default.

### First-start rules

1. **Identity stays useful alone.** Fresh machine/profile needs only its own local records and legitimate administrator approval. No companion, GitHub account, Thought Engine, worker or Processor is required for “Identity ready.”
2. **Identity has one selected local Experience profile** (not auto-register every project). Profile lists the exact **product/service kind**, trusted source, pin or local executable, verified branch/SHA, compatible protocol, necessary local directory and required demo features.
3. **The correct application means Thought Processor, not Thought-Controller.** Branch aliases or the first localhost page found are not sufficient; verify actual service identity/product, API protocol and executable provenance. Pin the selected candidate to a SHA; permit the owner to change it explicitly.
4. **Start only with approval.** The owner sees the list of planned launches, filesystem effects and any builds/downloads before clicking. First prototype can use a prebuilt/preselected local executable if an installer is not ready; show missing installation honestly.
5. **Use one bounded session runner** (may be a helper in this demo) to supervise *only the session-owned processes*. It may start/attach Processor, Engine and Demo Source as explicitly configured, and optionally a real worker only with a known approved start mechanism. This runner owns process supervision; Identity is not a general scheduler, worker, or data relay.
6. **Separate startup and authority.** Starting/attaching an Engine does *not* approve data access. Identity issues narrowly scoped grants only through an authenticated owner action. Processor cannot create a grant with a local Engine switch.
7. **Direct data path:** Demo Source S sends event to Engine E; E checks a signed, scope-specific decision from Identity for entity X; Processor reads the real receipts. Neither Identity nor Processor must forward every observation.
8. **Close or Stop carefully:** stopping the session cannot kill other users' sessions or pre-existing services; terminate only owned child processes using their process handles, not name/port-wide kill commands. Clear orphan processes, report failures and preserve durable data. Closing Processor alone should not delete an approved Identity grant.
9. **No silent startup at machine boot.** OS background service installation remains a separate, currently OPEN architectural choice. This demo begins when the user actively launches Identity and presses Start Experience.

## Model of “Everything running” — deliberate small scope

The phrase means **all parts of this selected mini-demo** are actually ready or honestly blocked. Not every installed plugin on the user's machine.

| Thing | Who governs / provides it | Demo requirement |
| --- | --- | --- |
| Identity installation/entity/service IDs | Identity's durable registry | Need persistent, distinct IDs; demo profile is isolated |
| Correct selected Processor build | Owner-approved Experience profile / session runner | Exact repo + SHA + verified instance and health |
| One Thought Engine instance | Its own independent application | Supports verified version/status and Identity grant checks |
| Demo Source S | Distinct restricted test source | Authenticates as S; cannot claim to be TASS by passing string |
| Approval of S→E for X | Identity authority | Real durable grant + authenticated check + revocation |
| AI inference | Thought Engine's independently selected worker | Optional real Chat-Jimmy; unavailable must be explicit |
| Observability / controls | Thought Processor | Correlated Identity decision, event receipt, status and safe logs |

The demo data root is **distinct from Alpha/Stable and existing user data**. Identity controls the mapping/ownership, while individual features write their own events, traces, thoughts and worker logs. No application runtime data or secrets inside repository clones.

## Smallest authority contract to deliver now

The [Engine experiment runbook](https://github.com/Arameair/Thought-engine/blob/experiment/identity-grant-enforcement-20261024/docs/IDENTITY_AUTHORIZATION_EXPERIMENT_2026-10-24.md) proposes:

```text
POST /v1/authorization/check
Caller: authenticated Engine E (Engine→Identity credential)
Body:   entity_id X, source_service_id S, target_service_id E,
        action thought.observation.ingest, request nonce
Return: signed scoped allow/deny decision with decision_id,
        grant_id/version, same source/entity/target/action/nonce
```

This is a **draft experiment API**, not a standardized platform contract. Identity currently does not implement it. Test signing, ownership and revocation; never interpret local port discovery, a hardcoded service ID or an Engine-local `grant.json` toggle as real Identity authorization.

**Issuer responsibilities:** identity & service registration, authenticated owner decision, protected durable grant, denial if unrecognized/mismatched, revocation without an Engine restart and tamper-resistant decision receipts. Key distribution/rotation and administrator auth must be consciously designed; the first demo is isolated same-host loopback-only and not a general production secret-management system.

**At startup** the Engine must have an Identity-known, authenticated service ID and an X that exists consistently in both services. The current Engine prototype generates an independent local card and service ID; reconciling them with Identity is a real integration task. Never merely display two matching human-friendly names and assume they're the same subject.

### Failure contract

| Failure | Visible Identity/Processor behavior | Actual protected result |
| --- | --- | --- |
| Identity itself can't initialize | Setup FAILED / repair guidance, no demo launch | No new grant or processing |
| Selected Processor missing/wrong version | NOT INSTALLED / INCOMPATIBLE; choose/build explicitly | Do not open a different app |
| Processor cannot start | FAILED + stderr/exit code/port/conflict | Identity stays running, child cleanup |
| Engine missing or wrong version | ENGINE UNAVAILABLE / INCOMPATIBLE | No fake observation success |
| No scoped grant yet | GRANT REQUIRED / DENIED | Engine 403; no worker call |
| Identity unavailable while Engine runs | AUTHORITY UNAVAILABLE / 503 | Engine fails closed (first experiment: no cached ALLOW) |
| Revoked after one successful observation | REVOKED | Next intake 403 and queued work recheck before model |
| Chat-Jimmy not ready | WORKER UNAVAILABLE | Authorized event may remain queued; no simulated output |
| User clicks Stop Demo | STOPPING / STOPPED or failure | Stop session-owned children only; data/other apps persist |

## Proposed delivery slices

**Slice 1: First-use launcher.** Identity runs independently and reports actual state. Click Start Experience opens the selected, verified **real** Processor candidate or reports precisely why it cannot. Demonstrate correct branch/SHA, PID/port, stop and restart. This is **useful even before authorization is complete**, but label `AUTHORIZATION NOT CONNECTED`.

**Slice 2: Trust bridge.** Identity provides a real owner-approved grant + signed decision; Engine verifies it before intake. Distinguish a fixture from the actual Identity process. Handle **allow / deny / revoked / unavailable** correctly, including queued work. Review with Engine team and record test evidence.

**Slice 3: End-to-end experience.** Processor shows one true authorized event from Demo Source, Engine receipt, a real worker call if available and persisted trace/outcome. Include a Revoke + Try again button that demonstrates the authority boundary. 

**Slice 4: Failure polish.** Recovery from missing/unreachable process, repeated launch without duplicate service, child ownership and clean Stop, conservative log redaction, latency and proof of durable restart.

Do not let a rich dashboard delay a small **real** interaction. Do not mark Slice 3 complete when only the launcher or fixture is running.

## Implementation evidence the owner should receive

A one-page test card with exact repository and branch/SHA for **Identity, Processor and Engine**, OS, local profile/data roots, launch commands or recorded sanitized actions, process PID/port/run ID, time to ready, request/decision/event IDs, allow and revoke results, model-call count, errors/retries, stop/orphan audit and whether the owner ran it. A button that worked in isolation is not evidence of the whole path.

### Never change these boundaries for demo convenience

- Identity cannot silently start every feature merely because it starts itself.
- Processor is an optional experience/control surface, **not** the universal data bus or grant authority.
- Engine remains independently launchable and does its own protected intake/worker calls.
- Grant revocation is enforced in Engine regardless of Processor visibility.
- GitHub, Memory, TASS real data, ScreenBridge, remote networking, autonomous waves and background on-boot startup are **not required** to prove this first experience.

**Owner acceptance rule:** Make the small demonstration *usable and truthful*. Any missing element is shown explicitly, never simulated as operational.
