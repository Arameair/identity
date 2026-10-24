# Roadmap — verify each integration before expanding

Roadmap gates are acceptance criteria, **not release promises**. Use [Status](STATUS.md) for what has actually been verified.

| Gate | Goal | Evidence before acceptance | Current state |
| --- | --- | --- | --- |
| 0 | Professional documentation and traceable branches | README, vision, architecture, status, decisions, workflow, AI disclosure | Documentation foundation in progress |
| 1 | Real local identity records | Owner creates TASS and Business, restarts, restores same IDs, handles disk failure | Implemented in experimental code; owner acceptance not documented as complete |
| 2 | Safe local GitHub verification | Live CLI account verified in executing OS; private repo linked read-only; no repo mutation | Experiment available; owner live check still required |
| 3 | Secure in-app GitHub account connection | Official browser authorization, least privilege, protected credential storage, revocation and accurate states | Planned |
| 4 | Authorized private repository lifecycle | Owner explicitly approves creation or attach; non-secret manifest; duplicate prevention, rollback, no unsafe deletion | Planned |
| 5 | Identity-aware feature grants | Thought enabled for TASS, denied for Business; audited grant, enforcement and revocation survive restart | Planned |
| 6 | Verified independent service handshake | Authenticated service discovery, compatible versions, accurate live status; no arbitrary process takeover | Read-only local discovery exists experimentally; authenticated handshake planned |
| 7 | Clean-install and release acceptance | Separate administrator/installation, safe restore/migration, permission review, owner tests, rollback and manual promotion | Planned |

## Thin next demonstration

Start with the [current identity manager](https://github.com/Arameair/identity/tree/experiment/identity-controller-port-discovery-20261023). Prove the local identity lifecycle and real GitHub CLI verification in the owner's actual Project Controller/OS. Keep controller discovery labeled read-only and unauthenticated. Only after that should a separate experiment attempt official provider authorization or independently enforced feature grants.

Every gate needs the full commit SHA, test environment, failure cases and manual owner decision. If a real dependency is absent, record **BLOCKED / NOT RUN**, not PASS.

## Guardrails

- A user may create a local ID without GitHub being connected.
- Do not silently install or restart independent services or merge branches.
- Preserve other features when Thought is disabled.
- Never claim a model or Thought Engine operates just because an ID card exists.
- Do not turn a prototype into an all-in-one administrator, engine and controller merely to accelerate a demo.

Related: [Identity owner issue #10](https://github.com/Arameair/identity/issues/10), [GitHub account roadmap #9](https://github.com/Arameair/identity/issues/9), [Thought feature design #8](https://github.com/Arameair/identity/issues/8).
