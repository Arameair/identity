# Identity P1 — bounded security-hardening prototype

Date: 2026-10-26  
Base: `alpha` @ `c8c0a54ab333077ccbfbd86c5952fb8a00b55dc1`  
Experiment: `experiment/identity-p1-security-hardening-20261026`  
Decision references: [Identity #14](https://github.com/Arameair/identity/issues/14), [Identity #15](https://github.com/Arameair/identity/issues/15), [Thought Processor shared P1 #20](https://github.com/Arameair/Thought-Processor/issues/20), [Processor acceptance #19](https://github.com/Arameair/Thought-Processor/issues/19).

## Status and scope

**Experimental correction candidate, not a secure feature system.** This branch targets a few independently testable problems in current Alpha. It does **not** implement the original Admin Identity, grant issuance, installation registry, third-party sandboxing, signed provenance, GitHub-native invitations, or offline Admin authentication. Existing `/api/v1/authorize` stays **DENIED**. Do not promote or advertise P1 as security-complete based on these tests.

A redesign of Project Controller is **not required** to fix these internal Identity defects. Before connecting the next feature, however, Thought Processor and Identity must agree on an authenticated request/response contract, user approval flow, and how the receiving feature enforces a grant. Project Controller is only a development launcher, not the intended user authority.

## Concrete patches in this experiment

1. **Unpair false-success correction.** Previously `POST /api/v1/unpair` emitted a signed success response *before* persisting credential revocation. It now persists first and only then signs success, using the verified request's response key retained in context. If persistence fails, it returns a signed 503 without claiming the old key was revoked.
2. **Bounded local operator session.** Existing session cookies remained valid throughout a process lifetime. Sessions now expire after a 30-minute fixed maximum; lookup and count prune expired state. A session authenticates possession of an operator link, **not** the original human Admin Identity; it cannot authorize feature grants.
3. **Signed-mutation replay across restart.** The existing `identity/2-alpha` nonce guard was process-memory-only. For signed `POST` operations, a short-lived replay ledger (`paired-nonces.json`) is persisted **before** the protected route is allowed. On corrupted/unreadable persistence, signed operations fail closed; an attempted mutation whose ledger write fails is rejected. Signed `GET` requests remain in-memory replay-checked; a replay across restart may re-read data but is not intended to mutate local state. Revisit read-side cost/side effects, ledger throughput, and possible malicious same-user file edits before declaring a hostile-process boundary. Only a 3-minute replay window is retained, using the existing timestamp guard. No cached ALLOW grants exist.
4. **Regression tests:** disk-failed unpair + signed failure + subsequent successful revoke; session expiration; signed POST replay after restart; failed and corrupted replay-ledger storage. They run alongside existing API v2 and profile tests.

**Known trade-off:** Every signed mutation rewrites the replay ledger using the existing profile JSON helper (temporary file + rename). This is deliberately simple for a low-volume local prototype, **not** a high-throughput durable transaction log. No arbitrary third-party process is permitted to manipulate this profile directory. A later stronger transaction/idempotency model will be required for installations/grants.

## P1 blocking security requirements still OPEN

- **Original Admin bootstrap:** On first GitHub verification show an explicit `Establish Admin Identity` confirmation; make one permanent system-local Admin principal, bind the verified GitHub numeric user ID as an alias, and prevent an account switch from taking over. Atomic one-time enrollment, session/credential revocation and recovery tests remain pending.
- **Privileged operation guards:** A paired Processor must not, merely by being paired, create grants or assume human approval. Every sensitive route requires server-side Admin policy, exact target/action/scope, intentional fresh approval for elevated changes, and durable redacted audit evidence. Existing paired management API remains a **P0/trusted-process limitation** until this gate is implemented.
- **Offline Admin approvals:** Owner requires local approvals even without GitHub/internet. **Authentication mechanism was explicitly deferred by the owner.** Do not treat a confirm button, localhost request, OS account, or pairing secret as approved offline Admin proof. The new grants and untrusted plugin paths stay BLOCKED until authentication, recovery and policy enforcement are selected and tested.
- **Feature identities and permissions:** Each installed feature copy requires its own durable ID and isolated state, initially zero grants. Receiving features enforce specific current authorization from Identity; denial/revocation/Identity outage are all fail-closed. Prove one deterministic trusted Fast AI Worker action before cloud providers/Thought Engine.
- **Untrusted executable isolation:** Same-OS-user unsandboxed features can directly access files and network outside Identity's API. Real Windows and Linux/WSL process/credential/file/network isolation must be established before allowing unknown third-party binaries; separate data directories alone are not sandboxing.
- **Artifact provenance:** Existing repo, uploader and co-published SHA256SUMS checks are not a publisher signature or trusted build attestation. Define provenance trust roots before general third-party installation.
- **Compatibility and documentation:** Update stale Alpha descriptions and reconcile the legacy `Protocol` value vs `APIVersion` only after assessing Thought Processor compatibility. Existing Linux fixture evidence does not establish Windows readiness.

## Test and evidence procedure

CI: [experiment checks](https://github.com/Arameair/identity/actions/workflows/identity-p1-security-test.yml) target Ubuntu and Windows with `go vet ./...`, `go test ./...`, and `go build ./...`, without making releases. CI may validate formatted copies; branch formatting must be resolved and checked before promotion.

For independent local verification, clone the experiment into a **new profile**, then run:

```bash
go vet ./...
go test ./...
go test ./internal/pairing -run '^TestP1'
go test ./internal/app -run '^TestP1'
go build ./...
```

Record exact HEAD SHA, platform, branch, run logs and results. Do **not** test on the owner's only live Alpha profile. Denial-only tests must not be represented as working grants.

Acceptance states: **PASS / FAIL / BLOCKED / NOT RUN**, plus KEEP / REVISE / REJECT from the owner. `alpha`, `stable`, and `main` are not modified by this experiment. No promotion without explicit approval.

## Next prototype gate

Decide and test **original Admin local authentication and recovery**, then implement one Admin-approved, receiver-enforced **synthetic** grant, including offline allow/revoke and Identity-unavailable denial. Until that foundational trust boundary exists, do not claim that all discovered security problems are fixed.
