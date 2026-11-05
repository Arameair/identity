# Identity — authority boundary for Go loop junctions

**2026-11-05 · design coordination only; no changes to permissions, credentials or release refs.**

Canonical design handoff: [Thought-Processor cross-loop alignment](https://github.com/Arameair/Thought-Processor/blob/docs/loop-junction-alignment-20261105/core/docs/CROSS_LOOP_ALIGNMENT_20261105.md). Related [Identity #30](https://github.com/Arameair/identity/issues/30), [Cognition #82](https://github.com/Arameair/Thought-Processor/issues/82), [TASS #25](https://github.com/Arameair/TASS/issues/25).

## What Identity must protect

- Thought Engine owns the **20-second** Go control pulse; the proposed Liaison at a communication junction is not another authority or scheduler. Do not implement a 20-minute pulse from the old design draft.
- Markdown cells at junctions provide *descriptions/context for approved information lookups*. A saved Markdown file, notebook cell, graph edge, model intent, or worker's DONE response **does not constitute permission or proof of execution**.
- Protected request must identify **source entity → target capability/identity → requested action → scoped resource**, with a pinned cell/feature version and correlation ID. Check current grant at protected use/dispatch time and fail closed when denied, locked, revoked or unavailable. Identity owns canonical IDs and cross-entity authorization; Engine-local identifiers cannot silently become global Identity IDs.
- Versioned reusable features may be shared across TASS and later identities, but **grants, credentials, model contexts, personal lessons, channel destinations and receipts remain per-identity**. An owner-approved TASS skill is not automatically authorized for KYSSA.
- All secrets stay under existing protected custody. Cognition's Go read-model/export layer must show safe authorization status/receipts without leaking tokens or raw private prompts. No raw Identity log mount/copy into Core merely for convenient UI viewing.

## What is NOT implemented/approved

No full end-to-end circle, general Markdown junction runner, cross-identity feature activation, Liaison-to-Liaison automatic handoff or Discord communication has been accepted. No security model should be invented to make a demo work. Authentication and grant interfaces must be verified against the actual deployed Identity version before an integration test.

## Future permission and evidence tests

Start with **one read-only information lookup** via an allowlisted Go handler; independently verify permitted and denied identities, revocation/LOCKED behavior, cross-identity isolation, no credential leak, and explicit durable receipt. Test absence/timeout while the 20-second Engine pulse remains responsive. No owner-machine authorization, token, deployment or Alpha changes from this docs handoff.
