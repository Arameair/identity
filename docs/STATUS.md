# Identity — status and branch ledger

**Reviewed:** 2026-10-23  
**Overall:** Documentation and experimental implementations exist. **No owner-approved production release or secure multi-service authorization integration has been verified.**

This ledger records what is actually known. A branch, green build, or HTTP 200 response does not by itself establish end-to-end functionality.

## Release lanes

| Lane | Purpose | Verified status |
| --- | --- | --- |
| [Main](../README.md) | Welcoming project documentation; later explicitly approved releases | README and documentation only; no application in this branch |
| [Alpha](https://github.com/Arameair/identity/tree/alpha) | Owner-controlled baseline and starting point for experiments | Documentation-only baseline at `8d08fec6629dc9cc35ea2266556572b86e5499d7` at review |
| Stable | Last known-good, owner-approved snapshot | **No `stable` branch found at review**; do not claim a stable build exists |
| [Experiments](https://github.com/Arameair/identity/branches) | Independent design and implementation candidates | Several unpromoted designs and Go prototypes; details below |

## Current experimental work

| Branch / evidence | Observed or documented capability | Verification limit |
| --- | --- | --- |
| [Identity local V1](https://github.com/Arameair/identity/tree/experiment/identity-local-v1-20261023) · [Gate #2](https://github.com/Arameair/identity/issues/2) | Local create/save/restore prototype | Owner acceptance must be recorded separately |
| [Real identity manager](https://github.com/Arameair/identity/tree/experiment/identity-real-manager-20261023) · [Issue #10](https://github.com/Arameair/identity/issues/10) | Go app for local identity cards, reversible archive, existing GitHub CLI verification and read-only private-repo attachment | Reported unit/build/HTTP checks; live GitHub sign-in and owner-environment verification not established by its report |
| [Controller port discovery](https://github.com/Arameair/identity/tree/experiment/identity-controller-port-discovery-20261023) | Read-only local Thought Controller detection added to the real manager | Owner's Project Controller log for commit `ccfb8940100310d387461195d8c4d899f0175c18` showed build and HTTP 200 on port 7790; **not** proof of authenticated registration or feature grants |
| [Bootstrap design](https://github.com/Arameair/identity/tree/experiment/fresh-admin-bootstrap-design-20261023) · [Issue #6](https://github.com/Arameair/identity/issues/6) | Plans for administrator initialization, discovery, explicit startup consent | Design / mock scope; not a real production installer |
| [Design foundation](https://github.com/Arameair/identity/tree/experiment/design-foundation-20261023) | Entity model, security, decisions and initial acceptance specifications | Design; does not imply implementation |

Other experiment branches may exist; this table is not an assertion that their candidates are merged or approved.

## Capability ledger

| Capability | Status | Evidence needed for acceptance |
| --- | --- | --- |
| Permanent local identity cards | **Implemented in experiment** | Owner create/restart/restore with unchanged IDs and isolated data |
| Archive / restore without deleting GitHub repos | **Implemented in experiment** | Owner test, GitHub unchanged, same restored record |
| GitHub CLI-based account check | **Implemented in experiment** | Real account and repository verification in owner's executing OS |
| Official in-app GitHub App authorization | **Planned** | Provider authorization, scoped permissions, secure storage, revocation |
| GitHub private-repo creation | **Planned** | Explicit approval, private visibility, idempotency, rollback |
| Thought Controller local discovery | **Implemented in experiment** | Identify correct independent service; distinguish port-open vs authenticated service |
| Authenticated cross-service identity and grants | **Planned** | Authorization enforced at receiving service, including denial and revocation |
| Optional Thought processing on selected identities | **Planned** | TASS allowed and Business denied until independently granted |

## Reporting language

Use **DESIGN**, **CODE PRESENT / UNVERIFIED**, **TESTED WITH FIXTURES**, **LOCAL RUNTIME VERIFIED**, **LIVE INTEGRATION VERIFIED**, **OWNER ACCEPTED**, **FAILED** or **BLOCKED**. Never replace one with another without corresponding evidence.

Each experiment should report: Alpha base SHA; experimental head SHA; scope; OS/runtime; exact tests; mocked vs real dependencies; processes and ports; data directory; persistence; failures; rollback; and owner decision. Link the report from this ledger **only through an owner-approved documentation update**.

See [Workflow](WORKFLOW.md), [Roadmap](ROADMAP.md), and [Experiment report template](EXPERIMENT_TEMPLATE.md).
