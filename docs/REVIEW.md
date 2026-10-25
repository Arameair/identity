# Independent design review

**Status:** Open for independent critique. Reviews are proposals until the human owner accepts a decision. Updated 2026-10-25.

**Accepted interface boundary:** People and administrators work through **Thought Processor**; Identity is the independent authority service behind it. Earlier Identity-first dashboard proposals remain historical review material, not the current UX direction. See [Decision D-08](DECISIONS.md) and [First run](FIRST_RUN.md).

Our architecture is being developed collaboratively with human direction and multiple AI perspectives, **including the opportunity for Grok to disagree**. No AI's proposal becomes policy by being persuasive, repeated or committed as notes.

## Review material and discussion

- [Open architecture review issue #14](https://github.com/Arameair/identity/issues/14) — questions about first-run authority, independent feature storage, auto-registration and background operation.
- [End-user design notes](END_USER_DESIGN_NOTES.md) — preserved external perspective supplied by the owner. Helpful advice, **not** a binding implementation requirement.
- [Core principle and boundaries](ARCHITECTURE.md), [First-run proposal](FIRST_RUN.md), [Storage](STORAGE.md).
- [Decision record](DECISIONS.md) — what is owner-selected versus still unresolved.
- [AI collaboration and verification](AI_COLLABORATION.md) — attribution and test-evidence rules.

## What a useful review contains

1. Which design assumption is challenged?
2. A concrete alternative and its benefits.
3. The security, reliability, UX and performance costs of both approaches.
4. A test or counterexample that could distinguish them.
5. What can be built now without depending on unfinished multi-service authorization?

Questions of special interest:
- Can feature self-registration work without a permanently running Identity service? How should offline startup queue and secure enrollment work?
- How can Identity establish administrator authority, protect account access and support safe backup/restore on a new machine?
- Does the proposed directory organization remain portable across Windows/WSL/VMs and multiple users?
- How do services authenticate rather than merely announce a friendly service name?
- How can the UI distinguish local record, discovered service, authenticated service and authorized capability honestly?

## Attribution and decision hygiene

If the owner brings feedback from Grok or another assistant, record the originating model **only when known**, with date/link if available. Distinguish the reviewer's own claims from the owner's decisions. Do not rewrite an independent review to manufacture consensus. When a reviewer recommendation conflicts with the architecture, keep both positions visible and record why one was selected or deferred.

**No direct Grok connection or notification is implied by creating a review issue.** The owner or another authorized contributor may share the questions and paste/link Grok's response; feedback can be added as a GitHub issue comment or future design note. This page does not claim Grok was contacted.

## Example: a live disagreement

[End-user design notes](END_USER_DESIGN_NOTES.md) recommend *one folder, one process* and *no background services* for cleanliness. The owner-approved cross-feature architecture calls for **centrally organized, independently owned feature storage** and startup self-registration. The **background service lifecycle remains OPEN**. A minimal single-process experiment is useful; it should not silently forbid a later, opt-in secure system service.

**Decision authority:** Human owner. **Evidence authority:** Reproducible real tests. See [Workflow](WORKFLOW.md).