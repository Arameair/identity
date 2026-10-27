# Identity documentation

**Current scope:** [Identity](../README.md) is the independent identity/authorization authority; [Thought Processor](https://github.com/Arameair/Thought-Processor) is the intended user and administrator interface. **No production security or released runnable application is claimed.**

## Read these first

| Document | Why it matters |
| --- | --- |
| [Architecture](ARCHITECTURE.md) | Component responsibilities and the security boundary between Identity, Processor, and independent features. |
| [Decisions](DECISIONS.md) | Owner-approved design choices versus unresolved questions. |
| [Status](STATUS.md) | Tested facts, candidate branches, blockers, and release state. |
| [Active backlog](BACKLOG.md) | The small set of open tracking issues; what was superseded and why. |
| [First run](FIRST_RUN.md) | Intended Processor-first experience, **not yet implemented end-to-end**. |
| [Engineering case study](PROJECT_JOURNEY.md) | Project history, tradeoffs, implementation evidence, and portfolio narrative. |
| [Credits and acknowledgments](CREDITS.md) | Project ownership, named AI contribution sources, and appropriate technology attribution. |

## Build, evaluate, and review

[Current P1 hands-on trial](https://github.com/Arameair/Thought-Processor/blob/experiment/p1-cleanup-claude-20261026/docs/P1/START_HERE.md) · [Historical October 25 subsystem trial](TRY_THE_PROTOTYPE.md) · [Roadmap](ROADMAP.md) · [Security](SECURITY.md) · [Branch workflow](WORKFLOW.md) · [Experiment report template](EXPERIMENT_TEMPLATE.md)

## Detailed reference material

These documents remain available for deeper review but are **not separate competing requirements**:

- [Vision](VISION.md) and [Identity-first authority model](IDENTITY_FIRST_MODEL.md).
- [Storage ownership](STORAGE.md) and [proposed interfaces](INTERFACES.md).
- [Independent review](REVIEW.md) and [AI collaboration rules](AI_COLLABORATION.md).
- [Archived October 25 end-user critique](archive/END_USER_DESIGN_NOTES_20261025.md) — preserved history; its Identity-first UI recommendation is superseded.

**Reading rule:** When an older suggestion disagrees with a recorded owner decision, use [DECISIONS.md](DECISIONS.md) and identify the change explicitly. If implementation and documentation differ, [STATUS.md](STATUS.md) and verifiable branch evidence describe what exists; neither implies owner acceptance.
