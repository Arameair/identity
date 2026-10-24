# Design discussion — Identity application as a live feature card

**Date:** 2026-10-23  
**Status:** OWNER DIRECTION + DISCUSSION NOTES, **not implemented or live-tested**  
**Branch:** `experiment/dynamic-feature-cards-notes-20261023`, created from `alpha` @ `8d08fec6629dc9cc35ea2266556572b86e5499d7`.  
**Attribution:** AI-assisted documentation of the owner's design discussion; proposed technical specifics below are not owner-approved implementations.

## Owner's clarified direction

- **Identity itself is a separately running GitHub project:** [`Arameair/identity`](https://github.com/Arameair/identity).
- In the **Thought Processor's Netflix / Prime Video-style workspace**, the **Identity card** means **the real running Identity application**, not merely a decorative avatar, static identity profile, or fabricated card.
- If Identity is running and its UI can be safely displayed, the card should provide a **live preview of that application**. Selecting it expands the actual Identity UI into the main workspace, with diagnostic information available beside it.
- The same card interaction should work for other independently running projects (Thought Engine, AI worker, Memory, hearing/Discord, vision/screen, future features), discovered **dynamically** as projects evolve.
- We will build, test and approve **one independent capability at a time**; an identity gains authorized feature use after the capability is validated and deliberately enabled. A new feature must not require redesigning the whole home screen.
- **Identity and Thought Processor are both foundational and remain distinct.** Identity manages entity identities, identity-linked repositories, and ultimately feature entitlements/grants; Thought Processor discovers/connects components and provides operator visibility. No duplicate Identity manager UI should be reimplemented inside Processor.

## Two different meanings of "identity" — avoid confusion

1. **Identity application / runtime:** a process started independently (for example via Project Controller) that owns the real identity-management UI and services. **One Identity application card** represents that running project instance, including its branch/build/connection status and genuine preview.
2. **Entity identity:** a record administered inside the Identity application (e.g., TASS). Each entity has its own immutable identifier and may have its own authorized/active capabilities. Entity cards, if rendered inside Identity, remain Identity's responsibility.

**Do not equate an Identity application card with one TASS entity record.** A single Identity process may manage multiple distinct entities without turning into multiple copies of the application.

## Integration boundary: Identity ↔ Thought Processor (proposal to test)

| Identity owns | Thought Processor owns |
| --- | --- |
| Entity creation, archive/restore, stable IDs, repository associations and their UI | Discover real local app instances and identify compatible services |
| Future authoritative feature grants/identity permissions | Display **verified** authorization and connection states; request authorized feed connections |
| Local identity persistence and secrets handling | Card catalog, safe preview handling, connection and trace diagnostics |
| Confirming which identity an action concerns | Never self-grant feature access or write another app's identity store |

Project Controller (a third independent application) is responsible for building, starting, stopping, and comparing branches. **Neither Identity nor Thought Processor should silently take over those process lifecycles** merely because a new card is found.

### Candidate discovery / preview metadata — draft only

A future version could expose a read-only description of the running Identity app:

- application identity: `service_kind = identity-manager`, instance ID, supported protocol/capabilities;
- verified local endpoint and separate UI preview URL, selected branch, exact SHA/build and health checked-at time;
- safe status such as **NOT RUNNING / DISCOVERED / CONNECTED / INCOMPATIBLE / UNVERIFIED / FAILED**;
- approved actions or navigation links (not implicit credentials or automation).

**This is not an implemented endpoint or format.** Detecting a port or locating a GitHub repository does **not** prove that the matching app is running. Preview access (same-origin restrictions, framing policies, browser sessions, authentication) must be tested; if embedded preview isn't supported, present a **clearly labeled Open existing UI** action rather than a blank or fake preview. Do not bypass CSP/X-Frame-Options or expose secrets to make embedding work.

A preview must display live Identity-owned content or a clearly labeled, honestly sourced read-only activity/status view. It must not imitate the Identity UI while silently using invented placeholder state.

## Experience to validate later

1. Start a real experimental Identity build independently in Project Controller.
2. Thought Processor discovers and identifies that exact process and presents an **Identity application card** with actual runtime status, branch/SHA and safe preview.
3. Expand the card: interact with **Identity's original live UI** while watching read-only logs/metrics as needed.
4. Create/restore an entity *in Identity*, and confirm that the result remains owned/persisted in Identity, not duplicated by Processor.
5. Stop Identity independently: Processor marks the preview **STOPPED / last verified at ...**, never live. Other running applications continue unaffected.
6. Start another compatible feature: its new card appears through discovery without a code change to a fixed set of cards.
7. Demonstrate missing grant and incompatible service; the card does not imply authorization or working integration.
8. Capture exact branch/SHA, connection result, error owner and safe diagnostics (no tokens/prompts/private identity data by default).

**No part of this sequence is marked PASS yet.** Existing Identity real-manager / controller-discovery experimental branches are useful **reference implementations**, not proof this contract or live embed already works. Reuse their findings intentionally without merging their implementations by accident.

## Open UI and contract questions

- Should Identity offer an embeddable app view, or should Thought Processor use a separate approved preview integration?
- Can one Processor workspace watch multiple Identity **applications**, and how are instances distinguished?
- Which identity/entity is the currently selected authorization context, and who chooses it?
- How are capability grants refreshed or revoked in the Processor's UI without caching a false approval?
- What information is okay in a card thumbnail vs only after expansion?
- Where should Project Controller provide service discovery/port metadata, if at all?

## Linked discussion

[Thought Processor — dynamic application cards and developer experience](https://github.com/Arameair/Thought-Processor/blob/experiment/dynamic-feature-cards-notes-20261023/docs/discussions/2026-10-23-dynamic-project-cards.md).

**No code was written on this branch.** Keep Alpha unchanged; any implementation belongs on a separately approved experiment, and any promotion requires owner approval.
