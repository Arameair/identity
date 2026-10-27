# Identity

**Local-first identity and permission authority for the Thought Processor ecosystem.**

Identity assigns stable identities, records which features are installed, and decides **who may perform which action for which identity**. [Thought Processor](https://github.com/Arameair/Thought-Processor) is the human-facing coordination interface. [Thought Engine](https://github.com/Arameair/Thought-engine) and other independent services do their own work and must enforce the permissions Identity issues.

> **Project status · October 27, 2026:** Research and working prototypes, **not a production-ready identity provider**. `alpha` contains the owner-promoted **P0 runnable service baseline**. P1 administrator, feature-installation and scoped-grant code exists on an **unpromoted experiment branch**. The owner has **not yet completed the P1 hands-on acceptance test**. The public `main` branch is documentation only; no stable/public release is claimed.

## Why this project exists

Independent applications need to know whose data and work they are handling **without each inventing its own identity database or permission rules**. Identity provides one source of authority while avoiding a monolithic runtime or a centralized store of every feature's private data.

| Component | Responsibility |
| --- | --- |
| **Identity** | Canonical IDs, administrator authority, installation records, narrowly scoped access grants, revocation and authorization decisions. |
| **Thought Processor** | The normal UI for setting up identities, installing features, requesting approvals and seeing meaningful operational status. It is **not** a second permission authority. |
| **Thought Engine** | Optional reasoning, observations and thoughts associated with an Identity-managed subject; cannot authorize its own access. |
| **Fast AI Worker and other features** | Specialized execution and feature-owned data; each receiving service must enforce the right Identity authorization. |

### The key security distinction

**Discovered ≠ authenticated ≠ installed ≠ authorized ≠ running.** A GitHub login, a reachable localhost port, an Engine on/off switch or a feature's availability must never substitute for a grant. An identity may exist with **zero feature connections** and may recommend tools; that is not permission to install, execute or access another identity's information. Reusable valid grants support autonomous work, but expanded permissions require approval.

## Implementation and evidence — not the same thing

| Lane | What is there | What is *not* proven |
| --- | --- | --- |
| [`alpha` — P0 baseline](https://github.com/Arameair/identity/tree/alpha) | Runnable Go Identity service, local persistence, GitHub device flow, `identity/2` authenticated Processor pairing and signed exchanges. [Owner/implementation context](https://github.com/Arameair/Thought-Processor/issues/19). | P1 feature permissions, third-party isolation, secure general release. |
| [`experiment/p1-cleanup-claude-20261026`](https://github.com/Arameair/identity/tree/experiment/p1-cleanup-claude-20261026) | P1 Admin enrollment, separate persistent feature-installation IDs, operation-bound approvals, grants, single-use request tickets and live receiver checks for the **trusted deterministic test fixture**. | Real Fast AI Worker/Thought Engine integration, Windows owner P1 sign-off, production security. |
| [P1 cleanup report](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/P1_CLEANUP_REPORT_20261026.md) | Claude's recorded fresh-clone tests: **81/81 integration**, **15/15 adversarial checks**, with earlier independent review prompting fixes. | Reported results are not a substitute for the owner's hands-on trial or independent security certification. |
| `main` (this page) | Public explanation, decisions, honest status, attribution and review notes. | No runnable source in this branch; it is not an installation package. |

**Known security gaps:** Same-OS-user processes are not isolated; Admin approval currently uses a local passphrase rather than an OS-backed authenticator; recovery is deferred; genuine third-party publisher provenance and information-source permissions are not finished. See [security limitations](docs/SECURITY.md).

## Try the working experiment

For a **developer-only** P1 Identity + Thought Processor + deterministic fixture walkthrough, use the matched branches and [P1 START HERE](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/START_HERE.md). The fixture is **not real AI**. Use disposable local profiles; do not point a public service or sensitive data at this prototype.

For the older October 25 standalone Identity UI experiment, see the [historical subsystem trial](docs/TRY_THE_PROTOTYPE.md). It does not represent the final Processor-led experience.

## Explore the project

[Architecture](docs/ARCHITECTURE.md) · [Status and branch evidence](docs/STATUS.md) · [Engineering case study](docs/PROJECT_JOURNEY.md) · [Owner decisions](docs/DECISIONS.md) · [Roadmap](docs/ROADMAP.md) · [Documentation index](docs/README.md)

## Authorship and attribution

**Project direction and release decisions:** [Arameair](https://github.com/Arameair). This is a human-directed, AI-assisted software effort; AI-generated proposals and tests are attributed as appropriate in commits, issues and evidence reports. See [contributors, sources and attribution](docs/CREDITS.md) and [AI collaboration policy](docs/AI_COLLABORATION.md). Names of third-party technologies or services imply **neither affiliation nor endorsement**.

**Release policy:** Experiments do not promote themselves. `alpha`, `stable` and executable releases require explicit owner approval. Documentation updates here do **not** approve or release code.
