# Identity roadmap — prove one puzzle piece at a time

[← Home](../README.md) · **[First prototype — exact scope](FIRST_PROTOTYPE.md)** · [Tests](ACCEPTANCE_TESTS.md)

No gate counts as complete until its owner-visible acceptance check is demonstrated. Build on a separate `experiment/*` branch created from latest **owner-approved alpha**; never auto-promote.

## Gate 0 — design (documentation only)

Define entity vs account vs repository, identity continuity, security boundaries, first-run journey and future interfaces. This is the purpose of the design branch.

## Gate 1 — FIRST CODE: local Create / Save / Restore

**Build this one step now.** Independent small Identity Manager app via Project Controller. Ask for name + type, create a permanent unique ID once, save versioned data locally, list/open identities, stop and restart, restore the **same** entities and IDs. Create an AI companion and a business. Show nonworking future connection statuses honestly. Copy safe diagnostics, respect `PC_DATA` and detached checkout, isolate test instances and handle storage failure.

**No real or fake GitHub adapter. No real or fake Thought Controller adapter.** This is intentionally narrower than an earlier roadmap draft.

The detailed build instructions and owner A–G acceptance checks are in **[FIRST_PROTOTYPE.md](FIRST_PROTOTYPE.md)**. Gate 1 does NOT require GitHub; locally created entities are clearly **pending setup** for the long-term GitHub-backed environment.

## Gate 2 — guided GitHub setup (only AFTER Gate 1 works and owner approves)

Connect existing GitHub account with provider-approved auth, or guide someone through GitHub's **official signup flow**, never collecting password/MFA. Secure tokens through supported OS credential protection; explicit scoped consent, denial and revocation. Create/select **a separate private GitHub repository per entity**, verify the owner and visibility, and write only a nonsecret manifest. Restore the same entity on a new device only after verified authorization; don't sync tokens through the repo.

## Gate 3 — Thought Controller integration (only AFTER Gate 2 and owner approval)

Agree on a minimal authenticated, versioned registration contract with Thought Controller; current V1 does **not** yet expose this. First fake-contract tests in a dedicated integration experiment, then live controller discovery, ready/unsupported status, idempotent register/reconnect, strict entity scoping and no raw credentials. Prefer reuse of already-running controller and keep process lifetime owned by Project Controller. An explicitly approved standalone startup may be explored later.

## Gate 4 — optional puzzle pieces (later)

Attach `memory.primary`, communication, perception, etc., through typed capability references, safe approval/health checks and independently managed modules. Personality is optional and not required for a business/person/project identity.

## Rule

**Finish the smallest real proof, inspect errors together, then choose the next piece.** No automatic accounts, thought processing, credentials, registration, GitHub remote effects or multi-entity AI behavior in Gate 1.
