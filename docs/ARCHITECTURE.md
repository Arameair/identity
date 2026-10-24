# Architecture — identity authority and independent features

**Target design.** Cross-service authorization and grant protocols are not implemented by the current Identity experiment.

```text
Owner / administrator
        |
        v
    Identity
    - immutable entity IDs and identity cards
    - account references and approved access
    - feature catalog and scoped grants (planned)
    - revocation/audit (planned)
        |
        +---- authorized capability reference ----> Thought service / engine
        |                                           (owns thought records)
        +---- authorized capability reference ----> Communication / Memory / other features
        |                                           (own their data and lifecycle)
        +---- account broker operations (planned) -> GitHub and other providers

Project Controller: separately owns local build, start, stop and diagnostic observation.
Thought Processor: separately owns operation and monitoring of the Thought Processing feature.
```

## Ownership boundaries

| Component | Owns | Does not own |
| --- | --- | --- |
| **Identity** | Persistent IDs; local entity registry; account connections; policy and approved feature grants | Thought generation, model response content, arbitrary process lifecycle |
| **Thought / Thought Engine** | Ingestion, Sparks, Traces, Thoughts, local thought persistence and validation | GitHub authentication or global entitlements |
| **Thought Processor** | Operator control surface for Thought Processing, real feed and worker status | Independent source logs or another identity authority |
| **Source features** | Logs, observations, media, domain-specific processing and their own lifecycle | Implicit access to every other feature |
| **Project Controller** | Development workspace, independent build/start/stop, logs and metrics | Issuing identity or feature authorization |
| **GitHub** | Hosted repositories and native authorization | Live local identity state or credentials stored in README files |

## Identity vs account vs capability

An **entity** is a person, companion, organization, business or service with a stable ID. An **account connection** is the authorized link to an external provider. A **feature grant** permits specific behavior on behalf of a specific identity. These are different records, and none automatically implies another.

An entity may reference a private GitHub repository containing **nonsecret** configuration. Neither access to the repository nor a declared entity ID proves permission to use another service.

## Local data and experiments

The current Go prototype persists a local System ID, cards, archived cards, an external GitHub login reference and controller port settings. The secure credential remains with the official GitHub CLI rather than being copied into Identity. No live cross-process feature-grant service exists.

Separate disposable checkouts must not silently share identity state. Project Controller passes `PC_DATA`, `PC_BRANCH` and `PORT`; experiments use isolated data and preserve original records. A production profile must be selected and backed up intentionally.

## Discovery does not establish trust

A port answering `/api/health`, or claiming to be Thought Controller, is **discovery**, not authentication. Before privileged integration, the service must have a verified installation/service identity, compatible protocol and authenticated per-action permission checks. A service must reject unauthorized requests even when its caller bypasses Identity's UI.

See [Interfaces](INTERFACES.md), [Security](SECURITY.md), and [Status](STATUS.md).
