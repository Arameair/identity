# AI collaboration, independent review and verification

**Disclosure:** Identity is a **human-directed software project developed with assistance from AI systems**. AI assistance may include system design, technical critique, documentation, experimental implementations, test planning and result interpretation. Human approval remains the project decision authority.

## Roles

- **Human owner:** Sets priorities, selects or reverses requirements, approves actions and decides whether experiments become Alpha, Stable or Main.
- **AI reviewers and assistants:** May disagree, identify counterexamples, propose alternative designs, write draft documentation/code or report experimental findings. Their opinions are **not** proof of behavior or automatic owner decisions.
- **Contributors:** Record material AI assistance in commits, issues or experiments when known; never invent authorship or pretend a service was consulted when it was not.

## Independent opinions are welcome

The owner requested that Grok and other assistants be able to provide independent architectural feedback. Use [Review](REVIEW.md) and [issue #14](https://github.com/Arameair/identity/issues/14) to preserve alternatives and counterarguments, especially around Identity's first run, organized feature-owned storage, auto-registration and optional background startup.

The existing [end-user design notes](archive/END_USER_DESIGN_NOTES_20261025.md) are deliberately kept as an independent perspective, **not silently recast as an accepted requirement**. A proposal for no background service can be retained alongside a proposal for an opt-in background service until the owner decides. If feedback was written by Grok, name Grok only when provenance is confirmed by the contributor or owner; do not attribute text merely because it appears in the discussion.

## Verification rules

1. Distinguish **idea**, **owner decision**, **source code present**, **fixture-based test**, **real local runtime**, **live cross-service result** and **owner acceptance**.
2. Record current branch, full SHA, environment/OS, test commands, actual side effects and rollback for implemented experiments.
3. Do not infer successful authentication from a responding port, valid grant enforcement from a UI toggle, or a deployed release from an AI-written GitHub commit.
4. For privileged operations, require documented caller authentication, scope, destination enforcement, errors, revocation and actual provider-side outcomes before a success claim.
5. Preserve contradictory review notes and include a reasoned comparison or measurable test instead of masking uncertainty with consensus.
6. Never commit or expose account credentials, raw private data or sensitive feature logs in an issue/model prompt.

## Attribution is not endorsement

The project acknowledges collaboration without implying any AI company has endorsed, verified or assumed responsibility for the work. A model-generated contribution is not independently validated merely because it was committed.

The `main` branch is documentation and a future release-facing lane. Adding review notes or design material on Main **does not** promote experimental software or change permission grants.

See [Workflow](WORKFLOW.md), [Status](STATUS.md), and [Experiment template](EXPERIMENT_TEMPLATE.md).