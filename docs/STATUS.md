# Status and branch ledger

**Reviewed:** 2026-10-24. **No owner-approved production release or verified secure multi-service registration/grant implementation.**

The `main` branch has been reorganized as the documentation and collaboration entry point. **No Go application was merged into Main by that change.** New architecture documents describe intended direction, not functioning services.

## Release lanes

| Lane | Current status |
| --- | --- |
| [`main`](../README.md) | Public-facing project documentation, architecture, first-run proposal, storage and independent review |
| [`alpha`](https://github.com/Arameair/identity/tree/alpha) | Owner-controlled documentation-only baseline as last verified @ `8d08fec6629dc9cc35ea2266556572b86e5499d7`; no promotion in these docs edits |
| `stable` | No Stable branch verified as of this review |
| [`experiment/*`](https://github.com/Arameair/identity/branches) | Separate designs and Go prototypes. No experiment is released or promoted by updating Main |

## Current experimental implementations

| Work | Present/observed | Still missing |
| --- | --- | --- |
| [Local V1](https://github.com/Arameair/identity/tree/experiment/identity-local-v1-20261023) | Create/save/restore local identity prototype | Full administrator trust, storage policy and owner-release acceptance |
| [Real local manager](https://github.com/Arameair/identity/tree/experiment/identity-real-manager-20261023) | Local identity cards, reversible archive, existing GitHub CLI account verification and read-only private-repo association in Go experiment | Official in-app OAuth/permissions broker, verified rollout |
| [Controller port discovery](https://github.com/Arameair/identity/tree/experiment/identity-controller-port-discovery-20261023) | Read-only local port/service-marker checks; owner reported build and HTTP UI result for a candidate | Service authentication, approved registration and grant enforcement |
| [Administrator/bootstrap design](https://github.com/Arameair/identity/tree/experiment/fresh-admin-bootstrap-design-20261023) | First-administrator proposals and mock-test plan | End-to-end verified production installer |
| [Thought Processor design](https://github.com/Arameair/Thought-Processor/blob/experiment/identity-authority-registration-20261024/docs/discussions/2026-10-24-identity-authority-registration.md) | Cross-repo principle and proposed authenticated self-registration contract | Real integrations and measured independent live connections |

**Reminder:** Some older bootstrap plans centered launching Thought Controller early. Those are retained as historical proposals; the current direction is **Identity first, zero optional features required**.

## Capability evidence matrix

| Capability | Evidence/status |
| --- | --- |
| Local entity cards and archive | **CODE PRESENT IN EXPERIMENT**; owner acceptance still separate |
| GitHub CLI session check | **CODE PRESENT IN EXPERIMENT**; not Identity-native account broker |
| Thought Controller port detection | **CODE PRESENT IN EXPERIMENT**; discovery only, not trust |
| Distinct first-run installation/admin/service identity with secure recovery | **DESIGN** |
| Centrally organized feature-owned storage allocation/policy | **DESIGN — owner-selected direction** |
| Feature self-registration and authenticated cross-service registry | **DESIGN — owner-selected direction** |
| Grant issuance and receiver-side enforcement | **DESIGN** |
| Thought Engine, Memory, Thought Processor as installed optional components | **DESIGN**; those components belong to separate repositories |
| Opt-in always-available Identity background service | **OPEN QUESTION / NOT IMPLEMENTED** |

## What counts as evidence

Use **DESIGN**, **CODE PRESENT / UNVERIFIED**, **TESTED WITH FIXTURES**, **LOCAL RUNTIME VERIFIED**, **LIVE INTEGRATION VERIFIED**, **OWNER ACCEPTED**, **FAILED** or **BLOCKED**. An AI response, a UI health check or a compile result does not turn a planned feature into a real authorized connection.

For each new experiment, report Alpha SHA, head SHA, OS/runtime, build/test commands, fixture-vs-live integration, exact operation IDs, failures and rollback. Link relevant issues and owner decisions.

## Review and references

- [Decision register](DECISIONS.md)
- [Roadmap](ROADMAP.md)
- [Workflow](WORKFLOW.md)
- [Independent review issue #14](https://github.com/Arameair/identity/issues/14)
- [Original end-user design notes](END_USER_DESIGN_NOTES.md)

Documentation may describe a target design confidently while always marking unimplemented security-sensitive features as **not yet verified**.