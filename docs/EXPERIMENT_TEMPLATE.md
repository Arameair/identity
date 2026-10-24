# Identity experiment — <specific goal>

**Date:**  
**Issue:**  
**Owner request and scope:**  
**Branch:** `experiment/<topic>-YYYYMMDD`  
**Exact starting Alpha SHA:**  
**Exact experimental head SHA:**  

## Hypothesis and expected behavior

What owner-visible problem does the change solve? What must stay independent? Identify affected identities, accounts, repositories and processes.

## Boundaries and changes

- Code/files changed:
- Data paths and migration:
- Other services and versions contacted:
- Credentials/access requirements (metadata only, **never secret values**):
- Potential writes or side effects:
- Explicitly excluded from this experiment:

## Verification results

| Test | Actual vs fixture | OS/environment | Observed evidence | Verdict |
| --- | --- | --- | --- | --- |
| Build and unit tests | | | | NOT RUN |
| Project Controller Play and Stop | | | | NOT RUN |
| Create/save/restore exact identity | | | | NOT RUN |
| Data isolation and restart | | | | NOT RUN |
| GitHub account and read-only repository check | | | | NOT RUN |
| Feature grant denied/allowed (if implemented) | | | | NOT RUN |
| Service identity/readiness vs simple HTTP 200 | | | | NOT RUN |
| Offline, denied, corrupted and incompatible states | | | | NOT RUN |
| No unwanted process/repository modification | | | | NOT RUN |

Record request IDs, safe logs and actual duration where measurable. Do not upgrade fixture tests to real integration evidence.

## Failures and unknowns

List error details, incomplete acceptance tests, unsupported features, data risk and rollback instructions.

## Owner review

- Decision: **PENDING** (accept / revise / reject).
- Promotion: **None unless the owner explicitly approves**.
- Rollback source SHA and preserved data:
- [Status ledger](STATUS.md) update after approved promotion:
