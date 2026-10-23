# Proposed new repository: Arameair/Thought

**Design approved in conversation, repository existence and code NOT established by this document.** Name recommendation: **Thought** (noun for independent capability and system-service identity); **Thinking** describes its behavior. Keep code in its own repository. [Identity's ID cards](FEATURE_CARDS_AND_THOUGHT_MODULE_2026-10-23.md) · [Thought Controller boundary](https://github.com/Arameair/Thought-Controller/blob/experiment/multi-identity-controller-design-20261023/docs/THOUGHT_FEATURE_BOUNDARY_2026-10-23.md).

## Purpose

Provide ONE reusable Thought Processing module for any approved entity. It accepts authorized observations/events, produces brief **sparks** (temporary relevance candidates), selected lightweight **traces** (evidence residue), and longer-lived **thoughts** (ideas/questions with state, notes and evolving evidence). Thinking is an **opt-in feature per identity**; an identity with no grant cannot make Thought process its events.

The service is itself a registered **Thought system/service identity**, tied to an installation's System Card. It has no implied right to read other entities or grant itself new features. **Thought Controller** is the external dashboard and developer control layer. **Identity** owns GitHub/accounts/permissions/feature grants. There must not be a second GitHub login system here.

## Vocabulary — don't confuse these

- **Identity:** permanent participant/card, including administrator, Thought service, TASS or business.
- **Thought feature:** installed, enabled and verified processing capability provided by this repo.
- **Spark:** tiny fleeting signal. May expire without trace.
- **Trace:** compact source-linked surviving evidence. May remain weak, decay, reinforce, merge or be discarded.
- **Thought record:** persistent owned question/hypothesis/activity that can be active, dormant, resumed or resolved.
- **Thought Controller:** browse and manage Spark/Trace/Thought activity for authorized entities.

## First prototype — bounded deterministic experiment (NO LLM)

Choose a tiny Go service or library; make it independently runnable and testable with a simple inspectable local interface (JSON API or small web UI). A small scripted test fixture sends events to an **enabled test entity**. Use deterministic rules to demonstrate a spark, selected trace, a small cluster, and a promoted thought; show WHY each decision occurred.

Minimum record attributes:
- **common:** schema_version, installation_id, entity_id, object_id, time, source/event id, correlation/reason.
- **spark:** short label, relevance/score, expiry, whether trace was kept.
- **trace:** source reference, event kind, evidence summary, subject/topic key, strength, time, retention/decay metadata.
- **thought:** trigger trace IDs, question/summary, lifecycle state, notes/developments, links, last change, why woke/slept/resolved, optional worker evidence.
- Distinguish observations (facts from external events) from model hypotheses; do not treat model guesses as verified facts.

### Three concrete tests

**Case A — a weak one-off:** an isolated event generates a spark; it expires and does NOT make a thought or permanent high-volume trace.

**Case B — repeated signals:** scripted returns to Goldshire leave related traces; multiple signals meet an explicit threshold and create ONE thought, e.g. “Why does the player return here?”. Further evidence reinforces it, without making duplicates. Show the originating trace IDs and rationale.

**Case C — lifecycle:** suspend/dormant thought; new relevant event wakes it. Stop/restart process and verify same persistent thought and trace relationships; a disabled or unapproved identity cannot create/read/modify another entity's thought records.

Include configurable low defaults for per-identity event rate, spark TTL, trace quota/retention, deduplication and max active thoughts. Handle unreadable events, duplicate delivery, write errors and crashes without losing earlier valid data.

## One secure interface (future integration; negotiate versions)

Identity registers/authorizes the Thought module. A feature enablement grant is scoped to a specific entity_id, installation_id, actions, expiry/revocation and optional resource limits. Thought validates it on ALL API operations, including status, history and diagnostics — the UI alone is not security. Use temporary test grants in first prototype, not fake claims of end-to-end security. Later negotiate protocol and approval/identity trust with Identity and Thought Controller.

Thought does NOT manage GitHub OAuth, repo creation, owner accounts, arbitrary process startup, or model session credentials. It may request an allowed worker capability from Identity in a future phase, not silently call a model.

## Suggested branch discipline and team handoff

1. Create private **Arameair/Thought** repository explicitly, with readable README describing boundaries. Bootstrap **main**, owner-approved **alpha**, then new **experiment/spark-trace-thought-v1-20261023** from Alpha. Do not auto-merge/promote.
2. First experiment uses deterministic mock observations and **no AI model**. Tests must prove records are owned by exactly one enabled entity and disabled entities cannot think.
3. Provide an optional read-only adapter/interface Thought Controller can use later. Don't require Thought Controller to be running for initial unit/standalone tests.
4. Provide exact branch/SHA, runnable test plan, counts (sparks, retained traces, thoughts, time, duplicates, dropped items), reasons for selection/expiration, process/CPU/RAM if available, safe diagnostics and rollback.

## Explicit out of scope now

Live Chat-Jimmy or other models, continuous autonomous scheduling, complex clustering ML, emotion simulation, permanent taxonomy of thought categories, GitHub account access, installers, remote multi-user access and self-modifying code.

**The first proof we want:** Turning Thought Processing on for TASS changes what TASS can do, while Business remains unable to think until its own feature is granted. The Spark → Trace → Thought machinery remains simple enough to explain, inspect and tune.
