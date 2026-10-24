# Branches, testing, promotion and rollback

Identity follows a **human-controlled promotion workflow**. Experiments do not update approved branches automatically.

## Branch meanings

```text
alpha (current owner-approved starting point)
  |
  +-- experiment/one-hypothesis-YYYYMMDD
  +-- experiment/alternate-YYYYMMDD
             |
             v
    owner compares results
             |
             v
      owner-approved alpha
             |
        explicit snapshot
             v
           stable
             |
       release approval
             v
            main
```

- **`alpha`** is the starting point for every new coding experiment. Never assume experimental code is approved merely because it compiles.
- **`experiment/<topic>-YYYYMMDD`** is a separately testable change from the *latest Alpha*. Port work from other branches only deliberately and document the source SHA.
- **`stable`** is the owner's known-good snapshot when created and approved. As of 2026-10-23, no `stable` branch was found.
- **`main`** is the welcoming project/release-facing lane. Documentation maintenance here does **not** approve an experimental executable or move Alpha/Stable.

The historical creation of README-only main/alpha baselines is a bootstrap case, not evidence that a product was released.

## Development procedure

1. Read [README](../README.md), [STATUS](STATUS.md), [ARCHITECTURE](ARCHITECTURE.md), [SECURITY](SECURITY.md) and the active GitHub issue.
2. Record the exact current Alpha SHA; branch independently from that Alpha.
3. State one functional goal, exclusions, affected data and services, and rollback plan.
4. Build and run the candidate independently; do not take over Thought, Thought Processor, Project Controller or another feature's processes.
5. Test local persistence, failure states, boundary enforcement, and any actual external account/service operation. Separate fixtures from real integrations.
6. Where appropriate, use Project Controller Play, Stop, Copy Context and Copy Logs; record actual port, data folder, build SHA, errors and timings.
7. Update the **experiment's** report and its own status record with exact evidence. Do not modify Alpha/Stable/Main as a substitute for owner testing.
8. Present the results and ask for explicit keep/revise/reject/promote. Only the owner advances approved branches.

## Required evidence

| Check | Minimum report |
| --- | --- |
| Provenance | Base Alpha SHA, experimental head SHA, environment/OS |
| Build and runtime | Command, elapsed time, process, port, clean Stop |
| Identity records | Immutable ID, save/archive/restore, restart and data isolation |
| Accounts | Exact account type, real vs fixture sign-in, effective read/write permission |
| Authorization | Cross-identity denial and revocation where implemented |
| Discovery | Claimed service vs authenticated connection clearly separated |
| External side effects | Which account/repo/process was read or modified |
| Failures | Denied, offline, incompatible, corrupt, duplicate and uncertain states |
| Recovery | Backup, known-good SHA, reversal and rollback impact |
| Owner verdict | Pending, accepted, revise or reject; promotion SHA if approved |

Use the [Experiment template](EXPERIMENT_TEMPLATE.md). Never treat a successful HTTP 200 on the local UI as end-to-end proof.
