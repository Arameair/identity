# Interfaces and future contracts

**This document defines questions and proposed contracts, not APIs that are currently implemented.** The experimental Go app has its own limited local HTTP endpoints. Do not assume other repositories accept the messages described here.

## Identity cards

An authoritative identity card should provide a stable entity ID, type, display name, creation time, schema version and explicit references to its administrator/installation, repositories and enabled features. Its ID must not change when a name, software branch or local port changes.

The current experiment stores System ID, cards, optional private repo associations, archives and one machine-scoped verified GitHub CLI login reference. Owner relationships and feature grants are a later gate.

## Account broker (planned)

A requesting feature should specify:

- Verified caller/service and target immutable identity ID.
- Requested provider operation and resource (for example, read repository metadata).
- Minimal scope and relevant owner approval reference.
- Request/operation ID for deduplication and audit.

Identity should return **allowed/denied/needs consent/unavailable** with a reason and traceable operation outcome. It should not return the GitHub master credential to the caller.

## Feature authorization (planned)

The receiving service must be able to determine which immutable identity has a given feature, which version and capability are approved, permitted data sources, expiration/revocation, and effective status. A revoked or missing grant must block new privileged work. Behavior during Identity outages requires an explicit, tested decision; do not silently treat local availability as permission.

Expected states should distinguish **NOT INSTALLED**, **INSTALLED / DISABLED**, **ENABLED / WAITING**, **AUTHORIZED / READY**, **REVOKED**, **INCOMPATIBLE**, and **UNAVAILABLE**.

## Feature self-registration (owner-selected direction; NOT IMPLEMENTED)

Independent services should be able to start first and **request enrollment or renewed registration** with Identity. Proposed records distinguish durable `service_id`, execution-scoped `instance_id`, installation/owner association, protocol/capability version, bounded runtime endpoint and current provenance. The actual wire format and trust mechanism are not decided.

A future registration interface must **authenticate the caller or require first-time enrollment approval**. A self-reported name/ID, local port or filesystem path is not proof. Identity may record an unverified discovery result, but it must not issue feature-to-feature permissions solely from that result.

Thought Processor is the independent **discovery, connection and diagnostics surface**. It queries authorized Identity service records and shows actual application instances. It does not issue its own canonical IDs or become a mandatory router of source data. After scoped authorization, source and destination features communicate **directly**, and the receiving service enforces access.

**Operational states:** `unknown` / `discovered-unverified` / `registered` / `running` / `authenticated` / `authorized` / `ready` must not be collapsed into a single "connected" state. Grant revocation, renewal and Identity-unavailable behavior need explicit, measured semantics.

See [Architecture](ARCHITECTURE.md), [Storage](STORAGE.md), and [First run](FIRST_RUN.md).

## Service discovery (current partial implementation)

The controller-discovery experiment scans only a bounded range of local ports (7778–7899) or checks a manually configured port, and recognizes a reported Thought Controller marker in a health response. That means **identified by response, not authenticated**. The UI must not claim it can start, register with or authorize the service from this signal.

Future contracts must report service/installation ID, protocol version, build SHA, readiness, authentication state and supported capabilities. A 200 response alone does not prove a connected integration.

## Required evidence

For a real authorized cross-service request, record safe request ID, actor, target identity, grant decision, destination service identity, operation status, elapsed time and sanitized failure reason. Never include raw credentials. Treat **accepted**, **processed** and **persisted** as different outcomes.

See [Security](SECURITY.md) and [Architecture](ARCHITECTURE.md).
