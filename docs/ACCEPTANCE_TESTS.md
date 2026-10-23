# Owner acceptance tests — observable PASS, FAIL, BLOCKED, UNVERIFIED

[← Home](../README.md) · [Roadmap](ROADMAP.md) · [Security](ACCOUNTS_AND_SECURITY.md)

**Pass criteria are behavioral and owner-visible**; successful compilation does not prove GitHub authorization or Thought Controller registration. Each test report includes branch/full SHA, environment (Windows/WSL/Linux), chosen data folder, exact action, UI status, sanitized diagnostic, outcome, and any manual approval.

| Test | Owner action | Expected observation |
| --- | --- | --- |
| **A1. Create** | Create local test entity named TASS | Unique permanent ID; correct type, pending setup; no account requests sent |
| **A2. Restore** | Stop/restart app and re-open entity | Same ID and metadata, no duplicate repo or entity |
| **A3. Branch sandbox** | Launch two independent app experiments with Project Controller | Test state separate by default; no modification to live identity; exact branch/SHA visible |
| **A4. Create second entity** | Create Example Workshop | Different entity ID, private repo target, no forced personality or memory |
| **A5. Pending/no GitHub** | Start without account or cancel authorization | Clear pending, saved local draft, no invented connected badge |
| **A6. Guided account** | Choose “I need a GitHub account” | Official GitHub sign-up opens; Identity never sees password/MFA |
| **A7. Private repo** | Explicitly authorize real test repo creation | Correct owner/name, privacy PRIVATE verified, nonsecret manifest only, no writes to unintended repos |
| **A8. Restore via repo** | On new test install, select repo and authorize | Same ID; reauthorization required for local secrets; conflict/owner mismatch safely rejected |
| **A9. Secret and revoke** | Inspect Git history, copied diagnostics, logs and revocation | No secrets, correct grant scope, revoked access stops working |
| **A10. Discover controller** | Run controller then open Identity | Read-only readiness, actual branch/port/version, no LLM message sent, no duplicate process |
| **A11. Missing controller** | Stop controller then open Identity | Honest unavailable state; approved start path; explicit or saved opt-in; no unowned orphan |
| **A12. Register** | Register twice with the same entity ID | First registered, second already_registered; one runtime entity; safe restart |
| **A13. Missing capability** | Request memory.primary or github.repo.read | Returns available handle **only if authorized**; otherwise needs approval / denied / unavailable |
| **A14. Cross-entity denial** | Entity A attempts to act through B's grant | Access refused, no token/other private data exposed |
| **A15. Offline/errors** | Disconnect network, mock provider expiry, controller incompatible | Recoverable status with exact reason; no duplicate writes, fabricated success or infinite retries |
| **A16. Logs and metrics** | Press Copy Diagnostics | Sanitized record of actions, approvals, failures, durations, version, evidence; no token/secret |

## Required gate-specific handoff

**Gate 1** must pass A1–A5, basic mock versions of A10/A12/A15/A16, clean independent Play/Stop and tests for storage failure. **Gate 2** must additionally pass A6–A9 with explicit approval and *one private disposable test repository*. **Gate 3** must pass A10–A14 against a genuinely compatible Thought Controller instance; fake-controller-only tests remain labeled SIMULATED.

Not all checks are possible in an external agent sandbox. Report that as **UNVERIFIED**, not PASS. No external repo creation or account signup is authorized by merely reading these notes.

## Copyable run report template

~~~text
IDENTITY PROTOTYPE REPORT
App branch / SHA:
Project Controller branch / SHA:
Thought Controller branch / SHA (if any):
Machine / OS:
Data folder / isolation:
Entity ID (short, no secrets):
GitHub: MOCK | REAL, repo visibility verified:
Controller: MOCK | REAL, API version:
Tests: A1 PASS; A2 ...; ...
Actions attempted / actual side effects:
Granted scopes, excluding token values:
Time, errors, owner confirmations:
Exact remaining failures / how to reproduce:
Recommended next step:
Owner promotion: NOT DONE / REQUESTED / APPROVED
~~~

[Next: open decisions →](DECISIONS.md)
