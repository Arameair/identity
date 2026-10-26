# P1 Reality Audit — Implemented Code vs. Fixtures vs. Decisions

**Date:** 2026-10-26  
**Subject branch:** `experiment/identity-p1-security-hardening-20261026`  
**Code SHA audited:** `a7a470dd164002d37e71eca30ef3b85662e94dbe`  
**Original base:** Identity `alpha` `c8c0a54ab333077ccbfbd86c5952fb8a00b55dc1`  
**Cross-team design:** [Processor shared P1 #20](https://github.com/Arameair/Thought-Processor/issues/20), [Processor acceptance #19](https://github.com/Arameair/Thought-Processor/issues/19), [Identity Admin #14](https://github.com/Arameair/identity/issues/14), [Identity grants #15](https://github.com/Arameair/identity/issues/15).

> **AUDIT RESULT: PARTIAL PROTOTYPE, NOT FEATURE-COMPLETE OR SECURITY-COMPLETE.**
> The changed production functions are real code, not dummy implementations. The repository **does** contain explicit fixtures/stand-ins in automated tests and demos. No genuine installed Fast AI Worker, original Admin approval system, feature permission grant, receiver enforcement, or trusted third-party sandbox is demonstrated by this experiment. These are not simulated as successful in the new production code; they simply remain **NOT IMPLEMENTED**.

## 1. Verification method and hard limits

- Examined the exact branch head, Alpha comparison (13 commits, only nine changed/added paths), changed application and pairing functions, both P1 test files, original P0 test harness, feature contract/catalog, UI and shared P1 Issue #20 owner decisions through **configurable idle shutdown**.
- [GitHub Actions run 40346653186](https://github.com/Arameair/identity/actions/runs/40346653186): **PASS** on `ubuntu-latest` and `windows-latest`, each for formatting, `go vet ./...`, `go test ./...` and `go build ./...`, on audited code SHA. Tests use Go test harnesses and some local test fixtures. They do **not** prove running both real apps on the owner's Windows PC or an end-to-end P1 deployment.
- No new manual install, live GitHub sign-in, native Windows UI test, penetration test, crash-consistency/power-failure test, OS sandbox breakout test, or real Fast AI Worker exchange was run **in this audit**.
- This file is audit documentation only. Its commit moves the experimental branch HEAD, but makes **no application-code changes**. CI pass above applies to code SHA `a7a470dd...`; do not say an untested later code change passed.

## 2. Real application behavior added by this branch

| Change | Status | Exact evidence / limitation |
|---|---|---|
| Persist unpair before responding with signed success | **IMPLEMENTED + TESTED (automated)** | `internal/app/apiv1.go` `POST /api/v1/unpair` calls `Pair.Unpair` before `signed(200)`. A disk-write failure returns signed 503; prior pairing stays valid. Tested through real local HTTP handler in `internal/app/p1_security_test.go`. |
| Expire local browser session | **IMPLEMENTED + TESTED (automated)** | `internal/app/session.go` rejects/prunes `sessionRec` after 30 minutes. **Important limit:** the per-process private bootstrap `#key=` open-link remains reusable to mint a *new* browser session until Identity restarts. Session timeout is not a human Admin authenticator, full logout, or a revocation of the open-link key. |
| Reject signed POST replay across an Identity restart | **IMPLEMENTED + TESTED (automated), PARTIAL SECURITY CONTROL** | `internal/pairing/pairing.go` stores used nonces in `paired-nonces.json` **before allowing POST handlers**, loads ledger after restart; errors/corruption block signed verification. Test covers a repeated signed POST after reopening store, plus disk failure and corruption. **Only POST is durable**; GET nonce history is in-memory, other future mutating HTTP methods need classification. |
| Return service-unavailable on replay-ledger storage failure | **IMPLEMENTED + TESTED (automated)** | `internal/app/server.go` maps `ErrReplayStore` to HTTP 503 rather than incorrect 401 credentials failure. |
| Full installation and authorization system | **NOT IMPLEMENTED** | No grant-writing endpoint was added; `POST /api/v1/authorize` still unconditionally returns `DENIED`. Do not call this an installed-feature or secure-approval prototype. |

**Replay limitation that remains:** `profile.WriteJSON` is a temporary-file + rename helper; it does **not** demonstrate `fsync`/directory sync durability across sudden power loss, multi-file transaction atomics or idempotent application mutations. The nonce ledger is not an adversarial same-OS-user security boundary, and writes are O(number of retained nonces) for each POST. Test crash consistency, bounded traffic, metadata/file permissions on Windows/WSL, and durable transaction IDs before treating it as production-safe. The 4096-entry guard can refuse new writes under heavy traffic; this is intentional fail-closed but needs usability/DoS testing.

## 3. Which mocks/fixtures exist — and what they do **not** prove

| Where | Real or simulated? | Honest description |
|---|---|---|
| `internal/app/p1_security_test.go` | **REAL HTTP code path, CONTROLLED TEST SETUP** | Runs a real `App` and HTTP server with a disposable profile, but creates synthetic pairing, expiry and disk failure conditions. No live GitHub login or Thought Processor UI. |
| `internal/pairing/p1_security_test.go` | **REAL pairing/store functions, TEST INPUTS** | Uses generated local code/token, fabricated signed requests, temp files, and a reopened local store. Confirms nonce behavior, not real admin-grant enforcement. |
| Existing `internal/app/app_test.go` | **MIXED: real local app + fixtures** | `ghFixture()` mimics GitHub device flow/user API; `httptest.Server` mimics Thought Processor health. Source explicitly labels `TEST FIXTURE`. The fixture account and tokens are not evidence of authentic GitHub access. |
| `scripts/fixture_github.py`, `scripts/fixture_github_engine.py` | **TEST FIXTURES ONLY** | Local emulations of GitHub authentication/releases/checksums for development. |
| `scripts/demo_engine_install.py` and older demo screenshots | **DEMONSTRATION WITH FIXTURE GITHUB** | The script says real Identity + Processor binaries but test-fixture GitHub, and may show a synthetic fixture release. This is not proof of real publisher-authenticated release provenance, arbitrary external installs or working P1. Historical screenshots are not current deployment tests. |
| `internal/features/features.go`, `internal/features/contract.go` | **REAL STATIC METADATA, NOT DISCOVERY/GRANTS** | Embedded feature catalog and `thought.feature/1` descriptive contract. Only a Thought Engine-specific install adapter is presently connected. Metadata is not a signed external manifest, installation registry or live runtime authorization. |
| `cmd/identity-probe` | **REAL PURPOSE-BUILT TEST CLIENT** | Exercises a real Identity HTTP/API path without a real Processor UI. Do not present a probe success as full Processor compatibility. |
| GitHub Actions Windows/Linux matrix | **REAL COMPILATION/TEST EXECUTION, NOT OWNER USE TEST** | Builds and runs Go tests on hosted Windows/Linux runners. Does not mean user downloaded and ran a Windows executable or connected two real products. |

**No deliberately fake P1 success path was found in the application changes.** This is a static inspection finding, not an assertion that no other latent stub exists elsewhere in the repo. The existing UI generally identifies unavailable features and `TEST FIXTURE` status; however, older README/AGENTS/demo labels and the `Protocol=identity/1-alpha` vs `APIVersion=identity/2-alpha` inconsistency require reconciliation before presenting a polished public P1 build.

## 4. All shared P1 decisions mapped to what this prototype actually does

Statuses: **REAL** = code exists and has relevant automated evidence; **PARTIAL** = precursor only; **NOT BUILT** = still planned; **DEFERRED** = deliberate owner choice; **BLOCKED** = cannot claim safe behavior until a dependency is settled.

| Owner decision / P1 requirement | Audit status | Why absent and what is needed |
|---|---|---|
| Original durable Admin principal, explicitly established after initial verified GitHub login | **NOT BUILT — BLOCKER** | Alpha stores system ID and mutable last verified GitHub account; it has no separately authenticated Admin principal. Define an atomic once-only Admin enrollment record, bind immutable GitHub numeric user ID as an authentication alias, ensure account switch cannot transfer authority, and test restart/race/replay. |
| Routine admin session vs fresh approval for high-risk actions | **NOT BUILT — BLOCKER** | Paired Processor can call management endpoints without proving original Admin intent; local browser link is not human Admin proof. Introduce server-side route classification, action-bound fresh approval, expiration, audit and denial tests. Existing pairing must not become the approval credential. |
| Offline local Admin authorization | **NOT BUILT — BLOCKER** | Requirement is **CONFIRMED**. A later Issue #20 decision selected a **flexible local authenticator policy** (platform authenticator when verified/available, protected fallback), but exact Windows/WSL implementation is still undecided. Must enroll a real local authenticator and validate offline challenge, allow/deny/revoke, error handling and fallback assurance. Internet outage ≠ Identity service outage. |
| Recovery process for lost Admin credentials/device | **DEFERRED BY OWNER** | Not needed to demonstrate a disposable trusted lab prototype; public/unattended release is blocked. No silent re-enrollment or fake safe recovery; define and test recovery later. |
| GitHub authentication, publisher eligibility vs local runtime authority | **PARTIAL — BASELINE REAL** | GitHub device flow/client already exists, but most automated verification uses fixtures. `LastAccount` is sign-in history, not Admin ownership. Separate trust domains and do a real opt-in owner sign-in test without sharing tokens. |
| One repo per independently developed feature; outside developers can publish | **NOT BUILT AS A DYNAMIC FEATURE WORKFLOW** | No registry/import/publish for arbitrary authors; static embedded feature catalog. Define publisher/repo/manifest identity, independently verifiable release provenance and invitation permissions. Repo structure policy by itself is a valid design convention. |
| One durable local installation identity per installed copy, separate from running process | **NOT BUILT** | Alpha's `idn_` system ID and `ent_` profile entities are **not** installation records. Need transactional install registration, version/digest, idempotency, state transitions, per-install credentials, scoped data and safe uninstall/tombstones. Restart must reuse install ID. |
| Feature appears as typed identity in Processor's common Identity workspace | **NOT BUILT HERE** | Processor UX, not Identity's troubleshooting UI. Add versioned installation-identity read contract after registry exists; demonstrate in real Processor integration. |
| Initial zero grants / allow one action / revoke / fail closed | **ZERO-GRANT DEFAULT REAL; ALLOW/REVOKE NOT BUILT** | `/api/v1/authorize` always denies. Needs Admin-approved persistent scoped grants, authenticated caller/recipient/subject/action check and receiver enforcement; prove deny-without, allow-with, revoke, wrong scope, outage, queued requests. Never use a UI toggle as proof. |
| Fast AI Worker running safely standalone with deterministic local adapter | **NOT BUILT OR INTEGRATED BY THIS BRANCH** | This repo contains no P1 worker install/runtime. Use trusted deterministic test package only; actual feature self-test must be clearly labeled deterministic test data and not an external provider response. |
| Language-agnostic versioned manifest / capability declarations | **PARTIAL (LEGACY EMBEDDED CONTRACT ONLY)** | Current `thought.feature/1` describes embedded features and single-executable releases, not publisher-authored generic packages for Go/Python/JS/Rust. Specify external schema, declared capabilities, compatibility/version policy, asset validation and negative-case tests. |
| Standard authenticated local API for running features | **NOT BUILT** | Identity has its own signed API but that is not a common feature API. Define feature identity/readiness/action/error schema, authentication and endpoint discovery; demonstrate a real worker receiver. |
| Manual / on-demand / always-running modes, manual default | **NOT BUILT** | Configuration, ownership, startup status, readiness and runtime permissions are undecided implementation. Build authorized process supervision in Processor/feature runtime without making Identity an implicit process manager. |
| Authorized on-demand dependent feature start | **NOT BUILT** | Needs validated grant **before** launch, duplicate-request protection, process supervision, readiness, crash recovery, queue handling, and no unapproved provider activity. |
| Configurable idle shutdown, preserve in-flight jobs/IDs/logs | **NOT BUILT** | Requires lease/job tracking, idle duration setting, graceful stopping, concurrent request safety and visible STOPPED vs FAILED. Never restart ChatJimmy to evade limits. |
| GitHub-native private tester invitations, in-Processor inbox | **NOT BUILT / TRANSPORT UNDECIDED** | GitHub sign-in is not a native feature-invitation inbox. Choose actual authenticated delivery/addressing, recipient privacy, expiry/revocation and separately approved install; do not treat collaborator invitations as runtime grants. |
| Untrusted third-party code safety | **BLOCKED, NOT PROVIDED** | Same-user unsandboxed executables can bypass API grants. Need proven OS process/file/credential/network isolation on Windows and WSL/Linux, along with signed release provenance/attestation and explicit owner install approval. Only trusted local deterministic features are suitable for initial P1 lab. |
| Runtime observation to Thought Engine, Memory/companion interoperation | **NOT BUILT / DEFERRED FROM FIRST P1** | Depends on real per-identity grants, a verified receiver and defined event schema. Do not infer this from fixture demonstrations or existing Thought Engine experimental branches. |

## 5. Precise next dependency order (not authorization to implement)

1. **Required before high-risk grants:** finish original Admin local authenticator design/implementation (flexible platform + appropriately secure fallback as confirmed, exact choices still open), atomic enrollment and operation-specific elevated approval. Can defer full recovery UX only in a disposable trusted test profile; lost credentials must fail closed.
2. **Trusted minimal feature contract:** standardize versioned language-neutral manifest and authenticated local feature API; pin a deterministic **trusted** worker package; no public/untrusted install.
3. **Actual installation proof:** one owner-approved package installs, unique ID persists, safe standalone run and health, three startup modes and approved on-demand behavior as planned, with explicit errors.
4. **Actual authorization proof:** one narrowly scoped Admin-approved synthetic request accepted by the **real** worker; no-grant, wrong actor/target, revoke, offline-local-Admin, and Identity-down scenarios tested. No mocked ALLOW.
5. **Integration evidence:** run real Identity + real Thought Processor + real worker together, including clean profile, WSL and Windows platform-specific runs, restarts, concurrent requests, stop/idle, no leaked secrets, exact SHAs and logs. Label NOT RUN until actually executed.

Do not call step 4 implemented because a deny-only test passes; do not call step 5 implemented because go test builds or fixtures pass.

## 6. Decision and audit verdict

**KEEP AS LIMITED EXPERIMENT / DO NOT PROMOTE YET.** The branch improved unpair correctness, session TTL and signed POST replay checks; none of that completes Admin/feature system authority. There are **genuine fixture-based tests** in the repository but no evidence that the new implementation secretly substitutes a fake worker/grant for a real one. This review is about actual source and existing CI evidence, not an independently executed production security assessment.

Before any promotion: eliminate misleading version/protocol/docs claims, assess open-link/session revocation and nonce-ledger crash durability/traffic, implement the specific Admin and feature interfaces intended for that next candidate, then re-run independent actual 3-process integration.
