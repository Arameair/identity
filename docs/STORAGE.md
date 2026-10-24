# Storage ownership and organization

**Status: target design, not implemented.** Owner-confirmed architectural direction, 2026-10-24.

> **Identity organizes and governs storage; each feature owns and processes its own data.**

Identity should **not** become the single database for game logs, model transcripts, screenshots, memories, or thought histories. It defines identities, ownership, permitted data locations, access references and policy. Specialized features retain responsibility for their own records and lifecycle.

## Illustrative logical hierarchy

```text
<user-selected durable root>/
├── identity/                 # canonical IDs, registry, policies, grants
│   ├── installation/
│   ├── entities/
│   ├── services/
│   └── audit/
├── features/                 # independent feature-owned storage areas
│   ├── tass/
│   ├── thought-engine/
│   ├── memory/               # created only if feature enabled/installed
│   └── screenbridge/
└── backups/                  # design pending; protected appropriately
```

**Illustration only:** directory names and physical colocation are not a mandatory production layout. Different devices, volumes, encrypted locations or operating systems can require different roots. Identity can track separately located feature data through stable references. Never put raw credentials, logs, or production data inside a source-code checkout by default.

## Explicit ownership boundary

| Data | Owner / authoritative source | Identity keeps |
| --- | --- | --- |
| Installation, entities, service identities | Identity | Canonical records |
| Scoped access decisions, grants, revocations | Identity; enforcement at the receiving service | Canonical policy/decision evidence |
| Account connections and protected credential references | Identity plus approved OS/provider credential facilities | References and permission metadata, **not unprotected raw tokens** |
| Feature storage allocations and location mappings | Identity policy; actual persistence by feature | Location, owner, quota/retention policy references |
| Thought Engine waves, prompts, traces and outputs | Thought Engine | Enabled/authorized status and service identity only |
| Optional Memory's historical records/index | Memory | Service ID and access policy, not memory content |
| TASS events/Discord state | TASS or corresponding source feature | Access policy and reference, not complete event stream |
| Runtime logs and metrics | Originating service | Bounded sanitized diagnostic references; Processor may view on demand |

For performance, an authorized source can send its data **directly** to the receiving feature. Identity and Thought Processor must not be required data-path proxies.

## Rules to protect the user and the machine

1. Identity never silently chooses a destructive migration or deletes unrelated application data.
2. Each feature receives only its intended data root and the minimum privileges required. A path entry does not itself confer filesystem authorization.
3. Separate durable user profiles from disposable `experiment/*` data (for example Project Controller `PC_DATA` paths); no accidental promotion/copy of test secrets.
4. Respect Windows, WSL and VM filesystem boundaries. A host path is not implicitly accessible/secure from the guest.
5. Use versioned metadata schemas, safe atomic writes and recoverable backups for Identity's authority records; features own their own migration and durability.
6. Configure retention, backup and deletion **per feature** where practical. Identity may enforce policy without collecting all content.
7. External accounts should use an OS-backed secure credential facility when verified available; provider CLI tokens belong to that provider facility, not in a nonsecret repository.
8. Operational logs and temporary thought context **are not automatically equivalent** to an enabled long-term Memory feature. Retention of checkpoints and audit metadata needs its own explicit policy.

## Failure scenario

If Thought Engine stops, its isolated data remains under its feature storage location and Identity still recognizes its registered service ID. If Identity is unavailable, Thought Engine may perform independent, nonprivileged work and use only still-valid prior grants. A Thought Engine error must not damage TASS's records or Identity's authority store.

## Unresolved design choices

- One physical user-selected root versus several roots registered under Identity.
- Feature sandbox/OS enforcement mechanism for each supported platform.
- Backup encryption, recovery/export format and restore across machines.
- Whether an optional background Identity service maintains registrations and storage mounts.
- Revocation and expiry semantics for access to feature-owned data.
- Safe deletion and tombstone policy for an identity, service or feature.

See [Architecture](ARCHITECTURE.md), [First run](FIRST_RUN.md), [Security](SECURITY.md), and [Decisions](DECISIONS.md).