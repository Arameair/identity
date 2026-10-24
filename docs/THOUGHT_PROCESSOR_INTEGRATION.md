# Identity ↔ Thought Processor — agreed architecture and integration direction

**Date:** 2026-10-23  
**Status:** OWNER-SELECTED DESIGN DIRECTION; documentation only, **not a working connection or approved implementation**  
**Base:** Identity `alpha` @ `8d08fec6629dc9cc35ea2266556572b86e5499d7`  
**Authorship:** Documented in collaboration with AI based on the owner's decisions.  
**Companion notes:** [Thought Processor — six decisions](https://github.com/Arameair/Thought-Processor/blob/experiment/six-design-decisions-20261023/docs/discussions/2026-10-23-six-design-decisions.md) and [open shared connection contract](https://github.com/Arameair/Thought-Processor/blob/experiment/six-design-decisions-20261023/docs/discussions/2026-10-23-feature-connection-contract.md).

> The older Identity foundation documents refer to *Thought Controller* and sometimes describe Project Controller as a test environment. These are **historical drafts**, not a dependency requirement. The current owner direction is to pair the independent **Identity** application with the independent **Thought Processor**, while keeping the personal Project Controller optional. This note does not silently rename, deprecate or migrate any existing repository or program.

## Important architecture refinement — 2026-10-24

**The owner clarified that Identity is the heart of the whole system: every independently developed feature also has its own identity.** Beyond entity account management and grants, Identity must represent feature/service participants, capability and relationship knowledge, and the authority under which Thought Processor makes connections. "Brains" means the system's trusted identity/relationship intelligence, **not** ownership of cognitive thought inference or a requirement to proxy every worker call.

See [Identity-first system model](IDENTITY_FIRST_SYSTEM_MODEL.md) for the separation of feature identity vs installation vs running instance, the open hierarchy-vs-independent-relations choice (**Option B was recommended, not yet approved**), bootstrap and questions for the next Processor discussion. This refinement does not overwrite the confirmed discovery/pairing/auth/telemetry decisions below.

## Why the two applications work together

**Identity** is a reusable, independently running application with its own real user interface. It manages durable **entity identities** (such as TASS), associations to private identity-linked repositories, local protected secrets and—eventually—authentication, service trust, per-identity feature authorization, permission scope, expiry and revocation. An *entity record* is **not** another copy of the Identity application.

**Thought Processor** is a lightweight **discovery, connection and observability control point** for independent, compatible running applications. It displays a dynamic card for the actual running Identity application, can expand **Identity's real UI** (or clearly offer to open the original UI when embedding is not safe/possible), and can display authorized diagnostics. It does **not** duplicate Identity's entity manager or decide grants.

**Thought Engine** owns observation ingestion, persisted thoughts and AI-worker heartbeat decisions. Worker calls can happen directly without routing inference through Thought Processor; Identity grants are enforced by recipients without asking Identity on each fast-path request.

**Other features** (memory, hearing, vision, communication, external workers) can run independently of both Processor and one another. Discovery doesn't activate capabilities or authorize data sharing; feature enrollment is per entity and opt-in. Losing Processor does not shut down features. Losing Identity may affect protected operations when short-lived grants expire.

## Owner-confirmed choices (design, not code)

| Subject | Selected approach | Identity responsibility |
| --- | --- | --- |
| Shared compatibility contract | **Small shared core** plus **optional capabilities**, not a huge universal API or a per-feature hardcoded Processor adapter | Support core identity/health/discovery descriptors once specified, and advertise extra identity/grant operations only where needed |
| Discovery | **Hybrid:** a small same-OS local discovery record/registry plus configured address or manual fallback across machines/VMs | Publish its own independently running application endpoint safely; reconnect after Processor restarts; no Project Controller requirement |
| Trusting a service | **One-time trusted pairing** followed by automatic credential-verified reconnection | Own trusted service identities, pairing approvals and revocation; never use port, process ID, claimed display name or a registration POST alone as proof |
| Sensitive access | **Auto-discover, Identity-authorize**; receiving feature **enforces** scoped grants | Approve a specific entity + feature + permission scopes; additional scopes require new approval; pairing is not a private-data grant |
| Identity outage | Existing approved connections can use **short-lived authorization** until expiry; expired grants cannot be renewed offline | Define validity, renewal and revocation rules; 5–15 minutes is merely an initial *test range*, not a chosen or coded TTL |
| Feature activation | **Discovered → owner-approved per entity → enabled**, with reconnection afterward while credentials and grants remain valid | Distinguish installed, running, discovered, paired, authorized and enabled; prevent unauthorized auto-activation |
| Status reporting | **Hybrid:** significant state changes pushed; bounded periodic health verification with honest stale/disconnected status | Report real runtime state without requiring inference or expensive scans; do not mistake liveness for authorization |
| Diagnostics | **Distributed logs with shared trace IDs**; bounded on-demand correlation | Keep local logs, redact secrets/private identity information, expose only specifically authorized read-only diagnostic metadata |
| Developer visibility | Dynamic lightweight cards, only selected original UI active, collapsible Logs/Flow/Metrics inspector | Supply an original UI entry point and truthful runtime/build metadata where feasible; do not encourage insecure iframe bypass |
| Version evidence | Compare Alpha/Stable/experiments with exact SHAs and tests, without build/promotion control inside Processor | Identify actual running build when known; preserve separate experimental branch policy and manual promotion |

**Not yet chosen:** the specific HTTP/JSON wire shape, whether optional Go helpers are supplied versus protocol-only integration, local registry format/location and cross-platform behavior, paired-service credential/storage mechanism, cryptographic grant representation, exact authorization lifetime, and status verification interval. These require testing and review before implementation.

## Non-negotiable boundaries

1. **Identity is the security authority, not a network proxy.** Grant decisions centralized; receiving services verify grants on the fast path. A compromise of a service must not automatically expose the credentials or records of every other identity.
2. **Discovery ≠ pairing ≠ authorization ≠ enablement.** A discovered/paired app gets no automatic right to private thoughts, entity metadata, Discord messages, screen contents, or account secrets.
3. **Pairing must survive restart** using a stable verifiable service credential or equivalent reviewed method; a PID/port/claimed name changes and is not an identity. Support revoke, replacement and legitimate key rotation rather than blindly trusting a reused address.
4. **No silent process management.** Identity and Processor work when installed by other users on other machines, without personal Project Controller or its ports. Processor card selection does not start/stop an application.
5. **Avoid recursive dependency.** Identity must start and manage its local state **before** Processor is present; discovery and pairing must not require a grant that can be obtained only by first being paired.
6. **No private material in discovery records.** No tokens, signing keys, cookies, passwords, personal entity records, private logs or full prompts in shared local registry, GitHub repositories or diagnostic exports.
7. **Failure truthfulness.** Mark UNVERIFIED, DISCOVERED, PAIRED, AUTHORIZED, ENABLED, STALE, INCOMPATIBLE or REVOKED based on evidence, not assumptions. If Identity goes down, Processor may still show the last-seen state clearly dated, not an invented successful authorization.
8. **Owner remains in control.** Never auto-promote `experiment/*` to `alpha`, `stable`, or `main`; implementation needs separate approval and live tests.

## Proposed conceptual exchange — not an API specification

```text
Start Identity alone                 Start Thought Processor alone
        |                                        |
Local Identity records work          Publish local discovery address/record
        |                                        |
        +-----> Find Processor (same OS registry or configured remote address)
                                                 |
                      Identify / advertise current app instance and capabilities
                                                 |
                      Processor displays DISCOVERED (not yet trusted/authorized)
                                                 |
                      User approves service pairing in Identity
                                                 |
                      Paired credential verified on later reconnections
                                                 |
      User selects entity, feature and scopes -> Identity approves scoped grant
                                                 |
      Receiving feature validates grant locally on sensitive operation
                                                 |
      Optional: feature reports sanitized status / trace for diagnostics
```

**Bootstrap caution:** The registration/discovery exchange must be possible *before* service pairing and must be safe when the sender is untrusted. A proposal is to publish non-sensitive metadata and mark it UNVERIFIED until a separately verified pairing succeeds. Details and threats are documented in [Trust, authorization and bootstrap questions](IDENTITY_PROCESSOR_TRUST_AND_BOOTSTRAP.md).

## First real validation gates (NONE RUN)

1. Start Identity independently; create/restore the same entity ID with **Processor offline**.
2. Start Processor independently (without Project Controller); Identity is discovered through the agreed local mechanism and receives **one genuine application card**, not one card per entity.
3. Verify a card shows **discovered but unpaired** without exposing private Identity data; attempt a sensitive operation and observe denial.
4. Complete **one-time pairing** after verifying the intended endpoint; restart Identity and Processor separately and confirm automatic verified reconnection **without new approval**.
5. Revoke the pairing/grant in Identity, and confirm protected access is denied according to the documented revocation/expiry behavior; do not claim instant revocation while an offline short-lived grant remains valid.
6. Connect a second compatible feature without changing Processor's fixed card code. Test disabled and expanded permissions separately.
7. Expand the **real Identity UI**, or present an explicit Open Original UI option if embedding is blocked; verify it is not a fake preview.
8. Induce stopped/unreachable/degraded/stale states; verify status push and occasional check display the correct state without inference requests.
9. Correlate read-only logs/metrics across applications using real request IDs; redact secrets and confirm copied diagnostic text remains bounded.
10. Report exact branch/commit, OS/machine, process/run IDs, test transcript, failures and resource measurements. **Nothing in this experiment is implemented or PASS.**

## Related owner-approved UI direction

Thought Processor uses Netflix-style **dynamic application cards**, lightweight status while collapsed (rendering **Option C**), **one real expanded app preview**, and a collapsible **Logs / Flow / Metrics** inspector. Its Flow view combines a connection map and an event timeline; version display uses evidence-based Alpha/Stable/experiment comparison. A future adaptive-preview **Option D** is deferred until benchmarks justify it. These are intended consumer behaviors; **Identity continues to own its own UI and records**.
