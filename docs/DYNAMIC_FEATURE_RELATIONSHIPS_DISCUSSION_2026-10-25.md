# Design discussion — Identity and dynamic feature relationships

**Date:** 2026-10-25  
**Status:** DESIGN ONLY. Owner explicitly wants to discuss architecture before further builds. These notes are not an approval to build a feature registry, grant API, publisher backend, install manager or code-generation system.  
**Collaborative provenance:** Based on owner discussion with ChatGPT (GPT-6) and reviewed existing Identity, Thought Processor, Thought Engine notes; external critique attributed to Grok and Claude's experimental reports are cited as research, not owner decisions.

## Owner-supported experience direction

Thought Processor is the primary everyday experience, organized as:

- **Profiles:** TASS, businesses, companions or other entities the user manages.
- **Catalog:** Thinking, Memory, Communication, Perception and other actually accessible features.
- **Collections:** Thoughts, activity and content belonging to the selected identity (from the feature that owns each item).

Identity is the **durable authority** behind the experience, not a second permanent full dashboard. The current user-facing intended first opening begins with the distributed `identity.exe` / `identity.sh` to establish GitHub account and installation/publisher relationship, then opens an approved Processor experience. Some newer `main` documents prefer Processor-first onboarding; reconcile this explicitly before coding. Keep recovery/standalone diagnostics separately from ordinary user navigation.

The model must allow a business identity with no cognitive capabilities. One identity survives adding, disabling, removing and replacing independent features. A feature may itself have a service identity, but that doesn't make it a normal person-facing profile.

## Identity records / boundaries to design

1. **Installation / administrator / entity / service / feature / running instance** are distinct durable vs ephemeral records. TASS's entity ID is canonical in Identity; Engine may not create a competing permanent TASS identity.
2. **Publisher access** is not the same as provider sign-in, visibility of a private repository, installation, local pairing, runtime grant, being enabled, or healthy. Claude's `repo-visibility-v0` rule is a deliberately narrow experimental Engine-install policy, not a general entitlement design. Existing local request drafts are NOT SUBMITTED or granted.
3. **Feature manifest** may declare provided/required capabilities, protocol/schema compatibility, identity/version/build provenance, data classes/events offered, prerequisites, and requested permissions. The exact signed manifest format is OPEN.
4. **Relationship grant** binds owner/actor, entity, source service, destination service, action, permitted event classes/fields, expiry and grant ID/version; receiving feature enforces a verifiable decision. An optional, separately scoped historical-log-read capability is not implicit in permission to ingest live events.
5. **Storage organization:** Identity retains policy and pointers; each feature owns and retains its raw logs, local operational data, Thoughts, Memory, etc. Identity does not centralize the full data plane.
6. **Lifecycle transitions:** propose, consent, authorize, install, register, attach, enable, pause, revoke, detach, uninstall and remove-data are separate operations. Revoke new processing without silently deleting old results. An uninstall must not wipe unrelated feature records or destroy history cited by persisted Thoughts without a deliberate policy.
7. **Verification:** only authenticated service/source identity and action-scoped policy can grant data sharing. Loopback availability or self-declared `source:memory` is not identity proof. No feature silently approves its own new permissions.
8. **Dynamic coding:** a feature proposing a new capability or version is not authorized to generate and execute software automatically; require explicit owner review of builds, capabilities, permissions, provenance and rollback.

## Example future relationship, not a grant

Selected entity: **TASS**. Feature: **Memory**. Provider capability: `memory.retrieval`; optional published event type `memory.conflict_detected`. Consumer: **Thought Engine**, capability `thought.observation.ingest`.

The UI might propose: “Allow the Memory feature to publish *conflict summary events* for TASS to this exact Engine.” Identity authenticates participants and decides the narrow scope; Memory produces only approved event classes; Engine verifies intake and its authorization policy, persists its own result, and Processor displays actual receipts. This relationship does **not** give Engine automatic read access to all memory contents, or grant access for another entity.

The Engine may later request bounded historical evidence retrieval from an authorized feature, but that needs a separate permission and must not turn Identity or Processor into a mandatory high-volume event relay.

## State and outage design candidates

Use separate axes: `not_known / registered / installed / running / paired / authorized / configured / waiting / ready / denied / revoked / unavailable`. Status must include source and check freshness. Deny unauthorized new protected work if the authority cannot be verified in the first integration; later offline leases are a new decision. Restarting a service must not mint a new canonical identity or implicitly renew revoked permissions.

## Questions for the Identity team and external reviewers

- Exact model for the customer's own GitHub-linked Identity repository versus publisher-controlled feature entitlements; where are requests submitted and approved?
- How does a new feature prove its service identity and publish a capability manifest?
- Who stores requested connection plans, who stores applied configs, and how is drift detected?
- What is the smallest bounded grant model for live observations and historical retrieval?
- How should approved feature removal handle orphaned relationships, citations, persisted data, version rollback and future reattachment?
- Should personality/purpose/goals be non-secret Identity attributes, or supplied as separate permitted context from a feature? Avoid quietly turning Identity into Memory.
- How does Identity avoid becoming a throughput bottleneck while remaining trusted for new privileged actions?
- What would falsify the claim that dynamic features improve the product enough to justify their complexity?

## Coordination / review sources

Primary product discussion: [Processor dynamic-feature ecosystem note](https://github.com/Arameair/Thought-Processor/blob/experiment/dynamic-feature-observation-design-20261025/docs/DYNAMIC_FEATURE_ECOSYSTEM_DESIGN_2026-10-25.md).  
Thought processing counterpart: [Engine observation-and-staging note](https://github.com/Arameair/Thought-engine/blob/experiment/dynamic-feature-observation-design-20261025/docs/FEATURE_OBSERVATIONS_AND_THOUGHT_STAGING_DISCUSSION_2026-10-25.md).  
Existing [Identity design decisions](https://github.com/Arameair/identity/blob/main/docs/DECISIONS.md), [owner first-run clarification](https://github.com/Arameair/identity/issues/16#issuecomment-6384852887), [Identity grant issue](https://github.com/Arameair/identity/issues/15), [Processor grant review](https://github.com/Arameair/Thought-Processor/issues/8), and [Grok-attributed critique](https://github.com/Arameair/Thought-Processor/issues/7#issuecomment-6380855246).

**Design discussion only, no new entitlement, account access, grant, installed feature, tested runtime or Alpha promotion is implied.**
