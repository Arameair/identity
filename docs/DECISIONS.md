# Decisions and unresolved questions

Record **owner direction** separately from **architectural proposals** and **tested implementation**. This document summarizes known direction as of 2026-10-23; it does not make an unapproved integration mandatory.

## Current owner direction

| Area | Direction | Practical consequence |
| --- | --- | --- |
| Identity scope | One reusable identity authority for companions, people, businesses, services and projects | Entity type does not determine mandatory personality or thinking |
| Independence | Specialized features should remain separable | Identity controls access, not every feature's data or runtime |
| Features | Installed, enabled and authorized are different states | TASS can have Thought while Business does not |
| Security | Centralize external account management and scoped grants | No raw general GitHub tokens distributed to every feature |
| GitHub | Private per-entity repository as the future non-secret blueprint, with owner approval | A local identity may exist while GitHub is pending; no automatic repo deletion |
| Development | Branch from Alpha for new experiments; owner promotes manually | Alpha, Stable and Main are not changed by experiment implementation |
| Communication | Clear evidence and diagnostic statuses | Do not confuse a responding web server with an authorized integration |
| Disclosure | Acknowledge AI collaboration | Provide accurate attribution and independent verification |

## Proposals that still require design and tests

1. Register a minimal-permission GitHub App for a secure in-app authorization experience.
2. Define Identity-to-feature service identity, scoped grants, revocation and audit receipts.
3. Authenticate source/service discovery instead of trusting health-response markers.
4. Provide a clean-install administrator and installation record with safe migrations.
5. Support a reusable feature manifest without baking Thought into every identity.
6. Determine the secure local credential store and platform-specific behavior across Windows, WSL and VMs.

## Open decisions

| ID | Question | Why it matters | Status |
| --- | --- | --- | --- |
| I-01 | What is the protected credential/key storage design on each OS? | GitHub App integration must protect credentials | OPEN |
| I-02 | Which authenticated service/grant protocol is authoritative? | Receiving services must reject spoofed permissions | OPEN |
| I-03 | What is revocation/outage behavior for local and offline features? | Permission safety and continuity | OPEN |
| I-04 | How are live identity data migrated from experiment folders? | Avoid duplication or accidental loss | OPEN |
| I-05 | When and by whom may a private repo be created? | Permission, side effects and rollback | OPEN |
| I-06 | How will Identity find and authorize an approved controller process? | Discovery is not lifecycle ownership or trust | OPEN |
| I-07 | What qualifies a build for Alpha, Stable and Main? | Reproducible owner acceptance | OPEN |

## Record a future decision

Add the date, issue/PR, options considered, owner-selected outcome, observed test evidence, and any reversal/rollback implications. Preserve superseded decisions with explicit links rather than silently rewriting history.
