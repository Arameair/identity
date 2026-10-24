# Trust, authorization and bootstrap with Thought Processor

**Date:** 2026-10-23 · **Status: Discussion/specification questions only. No security protocol is implemented or security-audited.**  
**Attribution:** Human-owned architectural choices documented with AI collaboration.  
**Context:** [Identity ↔ Thought Processor architecture](THOUGHT_PROCESSOR_INTEGRATION.md).

## Primary design principle

**Put policy authority in Identity, keep data operations out of its hot path.** Thought Processor detects and connects independent applications, but never grants permission on its own. Applications that handle protected data enforce independently verifiable grants.

Protecting Identity alone is **not sufficient**: individual receiving services must still validate authorization and isolate sensitive data. A newly discovered local port or remembered hostname cannot inherit privileges merely because it once worked.

## Distinct trust layers

1. **Discovery (untrusted):** The application announces service kind, instance/endpoint, protocol version and safe capabilities. Mark *discovered/unverified*; do not disclose private entity data or follow unsafe callback endpoints without checks.
2. **Pairing (trusted service identity):** An owner-approved process establishes which service credentials Identify recognizes. Verify the correct intended application; store trust carefully, allow revocation and legitimate rotation. A port/hostname/PID/repository label alone is not the credential.
3. **Authorization (specific operation):** Identity grants a **particular entity** access to a **particular feature** with **defined scopes, validity and audience**. A paired service cannot automatically read Memory, Discord, thoughts, accounts or screen data.
4. **Enablement and runtime (separate):** An allowed capability might remain *disabled*, *not installed*, *not running*, *degraded* or *disconnected*. These states should be displayed separately.
5. **Service enforcement:** When processing protected requests, the recipient verifies expiry, intended service/audience, scopes and entity binding against a mechanism selected later. Re-asking Identity on every fast AI heartbeat should not be mandatory.

## The bootstrap problem we must solve

How can **Identity itself** be safely discovered and paired if pairing approvals are managed by Identity?

- Identity must launch, display its own UI, and manage existing entity records **with Thought Processor entirely absent**.
- Processor's discovery layer can initially observe a limited **untrusted Identity instance descriptor** without assuming any grants or access to secrets.
- The owner must be able to inspect and approve the specific intended service pairing in a **trusted Identity-owned interaction**. A link in an untrusted discovery record must not silently become the trust authority.
- After pairing, Processor can verify possession of the paired credential on reconnection; discovery identifiers do not confer the credential.
- **Open:** decide actual first-admin/initial-trust bootstrap and recovery process, including first run, inaccessible Identity UI, reinstall, credential rotation, and multiple Identity instances. **No UI flow or cryptographic mechanism is approved yet.**

## Outage and revocation behavior

**Selected:** short-lived authorization, with a **5–15 minute range as a candidate for experiments, not a commitment.** Identity offline does not shut down all applications. Existing grants may continue only until expiry; no new grants or scope expansion. After expiry, protected operations pause or return an explicit *authorization unavailable* error, while unprotected features may continue.

This creates a bounded revocation delay when disconnected. An identity owner revoking a grant should be warned that disconnected features may retain existing authorization until natural expiry; how to accelerate revocation when connected remains to be designed.

**Need to specify before shipping:**
- Issue/renewal/expiry format and reasonable clock skew handling.
- Credential storage and rotation, identity/entity isolation and intended grant audiences.
- Revocation push/refresh when Identity is reachable and offline behavior when not.
- Pairing loss, emergency revoke, administrator recovery, multi-device scopes.
- Replay/downgrade resistance and safe parsing/version negotiation.
- Which minimal status data Processor can access before and after pairing.

## Honest operational statuses

| State | Meaning | Must **not** imply |
| --- | --- | --- |
| DISCOVERED | Compatible-looking application address/descriptor observed | Credential-verified or safe |
| UNVERIFIED | Service authenticity has not been established | Trusted pairing |
| PAIRED | Service credential verified against a trusted record | Any entity-level feature entitlement |
| AUTHORIZED | Valid scoped grant demonstrated for the **requested** operation | Feature enabled or healthy |
| ENABLED | Feature allowed by owner for an entity | Running and processing |
| STALE / UNREACHABLE | Last health confirmation is old / endpoint isn't responding | Last reported green still current |
| EXPIRED / REVOKED | Authorization or pairing cannot be used | Automatic renewal without Identity approval |
| INCOMPATIBLE | Protocol or capability mismatch | Silent fallback to privileged unknown API |

Precise status names are candidates; the **semantic separation** is owner-confirmed.

## Security and diagnostic minimalism

- Local discovery entries should be protected from trivial tampering where possible, but **must be treated as untrusted** regardless. Do not depend on discoverability for authentication.
- Require explicit approval and scoped permissions before any sensitive operation.
- Validate callback URLs and network destinations: a malicious local/remote advertisement could otherwise trick the UI into accessing unexpected services.
- Do not put personal repository credentials, bearer tokens, private keys, cookies or entity-private data into GitHub, global service registry, logs, traces, cards or copied context.
- Logs live with their producing application; request IDs and bounded metadata can be correlated by Processor. Sensitive diagnostic payloads require independent permission checks.
- Allow services to operate without Processor and recover when it returns. Avoid circular startup dependencies between security, UI, discovery and inference.
- Paired applications are not automatically *secure*. Treat credential compromise and permission misuse as separate threat scenarios.

## Interface choices already selected vs still open

**Owner chose:** small common core + optional per-feature interfaces; hybrid same-OS registry/configured or manual remote discovery; one-time pairing through Identity, automatic verifiable reconnection; hybrid status push plus occasional independent health verification; distributed logs with trace IDs.

**Owner has NOT yet chosen:** whether the open wire protocol will have maintained language helpers (**protocol-only vs protocol + optional Go SDK** remains open), specific endpoint paths, cryptographic keys/schemes, TTL value, status polling cadence, how each UI embeds safely, or whether remote pairing uses a separate QR/code or other mechanism.

## Practical next experiment (proposal)

One independently running **Identity** application and one **Thought Processor** instance on the same OS:
- Discover the Identity instance and render a **discovered/unverified** live card without access to entity data.
- Approve pairing using a trusted Identity-owned flow; demonstrate service credential verification on reconnect.
- Use a dummy test feature to ask Identity for one **specific identity + scope** grant; receiving service tests allow and deny outcomes. Avoid any real credentials or sensitive accounts for first trials.
- Stop Identity, validate short-lived grant behavior and expiry; stop Processor, confirm Identity records remain functional.
- Publish results, exact SHAs, machine/OS, measured costs, redacted diagnostics and failed checks. Do not claim security from mocked UI only.

**This document defines no API and changes no running service. Do not promote to Alpha without explicit owner approval.**
