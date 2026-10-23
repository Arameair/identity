# Identity ↔ Thought Controller: discovery, start and registration

[← Home](../README.md) · [Entity model](ENTITY_MODEL.md) · [Capabilities](CAPABILITIES.md)

**Status: FUTURE GATE 3, NOT THE FIRST PROTOTYPE.** Gate 1 only creates/saves/restores local entities; it must NOT discover, start, register with, or simulate Thought Controller. Read [FIRST_PROTOTYPE.md](FIRST_PROTOTYPE.md). Thought Controller's repaired V1 is an owner-operated thought test controller, **not yet a registered multi-entity service**. [Current Thought Controller work](https://github.com/Arameair/Thought-Controller/issues/3).

## Who starts whom?

- **Project Controller** owns the processes it launches. If it started Identity or Thought Controller, its Play/Stop are authoritative. Identity may discover and request a start through the approved mechanism; **it must not spawn an unmanaged duplicate or stop another app**.
- In standalone mode, Identity's desired behavior is **discover Thought Controller on launch**, then **start an approved version if absent and the user has opted into this behavior**. First-run UI makes that choice visible; later auto-start obeys saved explicit preference. Show the actual process/port and any failure.
- If an already-running controller is found, check a **versioned service identity, compatibility and authorization**, not just that a port returns HTTP 200. No blind network scans or trust in an unrelated loopback service.
- If missing/incompatible, report **Not running / Not compatible / Needs approval** rather than claiming the entity was registered.
- Discover via an explicit configured URL, an authenticated local registry/discovery file, or a Project Controller supplied endpoint. Prioritize loopback or OS local IPC; validate origin/host, restrict writes to authorized callers.

## Proposed minimal API contract (NOT live endpoints)

~~~text
GET   /v1/health
      -> {service, protocol_version, ready, supported_features}

POST  /v1/entities/register
      <- {request_id, entity_id, entity_type, identity_service,
          approved_capability_refs, schema_version}
      -> {status: registered|already_registered|denied|unsupported,
          registration_id, protocol_version}

GET   /v1/entities/{entity_id}/status
      -> {registered, runtime_state, last_verified_at}

POST  /v1/entities/{entity_id}/capability-requests
      <- {request_id, capability, purpose, requested_action}
      -> {status: available|approval_required|denied|unavailable,
          capability_handle?}
~~~

Endpoint names and payload fields are **proposals to negotiate** with Thought Controller's team, not APIs implemented in that repository. Identity should authenticate its caller and controller, verify the entity owner, and avoid relaying external provider tokens.

**Registration semantics:** idempotent by stable entity ID and request ID. Reconnecting may update verified runtime registration, not create a second identity. A controller cannot change who owns a registered entity merely by submitting another name or repo URL.

## Two example flows

~~~mermaid
sequenceDiagram
    actor User
    participant I as Identity
    participant PC as Project Controller
    participant TC as Thought Controller
    User->>I: Open entity / TASS
    I->>TC: Health and service identity check
    alt Controller already running
        TC-->>I: Ready and compatible
    else Controller absent
        I-->>User: Start controller? (or use saved opt-in)
        User->>I: Authorize
        I->>PC: Request approved start
        PC-->>I: Actual reachable endpoint
    end
    I->>TC: Authenticated idempotent register(entity_id)
    TC-->>I: Registered / already_registered / error
    I-->>User: Exact connection status
~~~

~~~mermaid
sequenceDiagram
    participant TC as Thought Controller
    participant I as Identity
    actor User
    TC->>I: Request memory.primary for entity X
    I->>I: Verify caller, ownership and current grants
    alt Capability already authorized
        I-->>TC: Scoped handle + ready status
    else Capability absent
        I-->>User: Show guided connection / approval
        User->>I: Approve or deny
        I-->>TC: Approved handle OR denied/pending
    end
~~~

## What must the controller know?

Only the **stable entity ID**, the **versioned identity endpoint/contract**, explicit **scoped capabilities**, and which entity's thought state belongs to it. It should **never receive Discord or GitHub tokens**. Thoughts/events/traces must ultimately be isolated by entity ID. The initial handshake may use **one test entity** only; proper multi-entity isolation is a later acceptance gate.

## Integration evidence required

Show which controller **was actually running**, its branch/full SHA, host/port, readiness, registered entity ID, registration response, no duplicates after restart, and redacted diagnostics. First test with a **fake controller adapter**, then a dedicated compatible experimental Thought Controller branch. Do not claim integration on a version whose registration API does not yet exist.

[Next: modules and resource discovery →](CAPABILITIES.md)
