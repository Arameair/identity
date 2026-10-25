# Vision — durable authority for modular software

[← Project home](../README.md) · [Architecture](ARCHITECTURE.md) · [Status](STATUS.md)

> **Identity owns the organization and authority of the system. Each feature owns its actual work.**

Identity's purpose is to give every authorized entity and feature a durable place in a larger, independently extensible software ecosystem. **Thought Processor, not Identity, is the human and administrator-facing application.** Identity operates as the authority service behind it. A person, companion, business, project, system service or feature may each have its own stable identity without inheriting a personality, thought engine, memory system or account permissions.

## The problem

A modular application ecosystem quickly becomes unreliable if individual features each invent different user IDs, copy master credentials, write logs anywhere on the computer or assume a service is authorized just because it answers on a port. Identity addresses the **common organization and authorization** problem while preserving each feature's specialization.

## The desired experience

- **A new computer:** Open **Thought Processor** as the user-facing control point. From there, create or restore an installation backed by Identity, establish trust, then add optional features. This integrated first-run path is a **design target**, not yet implemented.
- **A companion:** TASS can exist without thought or Memory. The optional Thought Engine may form observations into thoughts; enabled Memory can later offer past context. Neither becomes part of Identity.
- **A business or project:** Has a separate durable identity, its own feature choices, safe configuration and appropriately isolated data.
- **A developer:** Can run different experimental versions independently, discover their real state through Thought Processor and compare evidence without automatically promoting a branch.
- **An administrator:** Uses **Thought Processor** to inspect Identity-backed records: who has which approved access, where feature-owned data belongs, and why a connection succeeded, failed or was revoked. The Processor displays and requests changes; Identity remains the canonical authority.

## Design commitments

1. **Central authority, decentralized work.** Identity governs IDs, accounts, grants and storage ownership. Features keep control over their runtime, content and processing.
2. **Identity survives change.** A name, branch, worker, endpoint, repository or machine migration does not silently replace an entity.
3. **No forced intelligence.** Thinking, Memory, screen interpretation and conversations are independently enabled capabilities.
4. **Discovery isn't trust.** Authenticate actors and receiving-service grants before any privileged data access.
5. **Visible, organized ownership.** Storage locations, retention, safe backups and feature isolation are part of the system design.
6. **Honest operational evidence.** No fake green indicators, simulated integrations presented as live or undocumented external side effects.
7. **Owner-controlled evolution.** New approaches are evaluated in isolated experiments with clear rollback and human promotion.
8. **Open independent review.** Invite reasoned disagreement and testable counterexamples; a review or AI-generated claim is not a decision.

## What Identity is not

Identity is not a chatbot, Thought Engine, Memory database, autonomous code editor, process launcher, GitHub token warehouse, mandatory high-volume log bus **or a second normal-user dashboard**. It remains usable if any optional component is stopped.

## Current limits

The current `main` branch is documentation only. Earlier experiments demonstrate local IDs and GitHub CLI checks. The October 25 Go candidate additionally contains a guided **Identity-owned** local UI, GitHub App device-flow code and read-only Thought Processor detection. **That UI placement conflicts with the selected Thought Processor-first human experience and is a known prototype limitation.** Production service identity, authenticated grants and secure first-run recovery remain unproven. See [Status](STATUS.md), [Roadmap](ROADMAP.md) and [Review](REVIEW.md).
