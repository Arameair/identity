# How Identity should work — notes for the build (2026-10-25)

**Status:** notes for discussion and planning. No code was changed. They pair with:
- `Arameair/Thought-Processor` → `docs/HOW_IT_SHOULD_WORK_2026-10-25.md`
- `Arameair/Thought-engine` → `docs/HOW_IT_SHOULD_WORK_2026-10-25.md`

The "whole picture" section is identical in all three; the rest is Identity's part.

Tags: **[KNOW]** seen in code or a run · **[INFER]** follows from that · **[GUESS]** opinion.

## The whole picture (this section is the same in all three repos)

### What the owner has asked for (collected from his directions, 10-23 → 10-25)
1. **Identity is the first program a person opens.**
   - It's a simple, polished setup for a non-technical person (for example the owner's brother).
   - **Connect GitHub** uses the publisher's built-in GitHub App, so nobody creates their own app. A guided "use your own app" walkthrough exists only for people who choose it.
   - Identity then finds and opens **Thought Processor**.
2. **Thought Processor comes first**, before work on the thinking itself.
   - Its job is to **have authority to install the Thought Engine**, and then start or attach it.
   - Processor installs the Engine **only if the person's Identity allows it**. First rule: the verified GitHub account can see `Arameair/Thought-engine`. A real grant service comes later.
   - Installs come from a **tagged release with a checksum**.
3. **Identity approves, Processor acts.** Processor starts or attaches the selected Engine only with Identity's approval.
   - **Strict rule:** Processor must **not** read the Engine's private status before an *Identity-approved* attachment.
4. **The Thought Engine is real.**
   - No simulations or mocks in normal use.
   - **Chat-Jimmy** is the current AI worker.
   - If the worker isn't ready, observations wait and say so.
   - Test stand-ins are allowed only when they are labelled **TEST FIXTURE** on screen and off by default.
5. **Truthful everywhere.** No fabricated state, no green light without evidence. Secrets (tokens, codes, `#key=` / `#code=` links, cookies) never appear in logs, diagnostics or issues.
6. **Features are the product.** Each program runs on its own, and turning one off doesn't break the others.
7. **How work is done:**
   - Work on experiment branches; the owner alone promotes to `alpha` / `stable` / `main`.
   - Every candidate must be runnable by the owner's Grok bot from an AGENTS.md runbook on **Windows**.
   - Report every check as PASS / FAIL / NOT RUN, tagged [KNOW] / [INFER] / [GUESS].

### The journey we're building toward (target, not today)

```text
 PERSON                IDENTITY (7790)            THOUGHT PROCESSOR (7860)        THOUGHT ENGINE (7798)      CHAT-JIMMY (7791)
   │ open Identity ──────►│
   │ Connect GitHub ─────►│ device sign-in (built-in app)
   │                      │ find / open Processor ───►│
   │                      │◄──── link (automatic if Identity started it, else one code)
   │                      │                            │ "may I install Thought Engine?"
   │                      │ rule: GitHub can see repo  │
   │                      │ ── signed INSTALL grant ──►│ download tagged release THROUGH Identity,
   │                      │                            │ check SHA-256, install to profile
   │                      │                            │ "may I start/attach it for TASS?"
   │                      │ ── signed ENGINE grant ───►│ start Engine (own child process),
   │                      │                            │ hand it the grant ───────────────►│ verifies Identity's
   │                      │                            │                                   │ signature, THEN attaches
   │                      │                            │◄── private status only now ───────│
   │ "start Chat-Jimmy live" (owner's explicit choice) │ start worker ─────────────────────┼──────────►│
   │ observations (typed, then real sources) ─────────►│ ──────── send ───────────────────►│ ◄── rate / question ──►│
   │◄──────────── Thoughts, with honest states ────────│◄──────────────────────────────────│
```

### Where today's code stands against that journey (2026-10-26 heads)

| Step | Today | Gap |
| --- | --- | --- |
| Identity sign-in, built-in app | **Works** (alpha-candidate-20261025) [KNOW] | Sign-in is forgotten on every Identity restart |
| Identity finds / opens Processor | **Works**, read-only discovery [KNOW] | Doesn't start Processor; no automatic link |
| Identity ↔ Processor link | **Works** on experiment: one-time code, hashed token, HMAC-proofed answers [KNOW] | Typed code every time; not on alpha |
| "May I install?" + install through Identity | **Works** on experiment with a TEST FIXTURE GitHub [KNOW] | On hold; no real release tag yet; Contents:read not approved |
| Identity approves start/attach | **Missing** [KNOW] | No signed grant exists; the Engine can't verify Identity |
| Processor starts the Engine | **Missing** (no runner, by design) [KNOW] | Three or more windows by hand |
| Processor ↔ Engine attach | **Works**, but with an **Engine-local code**, not Identity approval [KNOW] | Breaks owner rule 3 as written; interim only |
| Engine refuses mock, waits honestly | **Works** [KNOW] | Live Chat-Jimmy never run in this loop |
| Engine's own page and v1 routes | **Open to any local program** [KNOW] | Must be locked once Identity grants exist |
| Real observation sources | **Missing**: typed only [KNOW] | — |

### Shared words (use these exact names in all three repos)
| Word | Meaning |
| --- | --- |
| **Link** | Two local programs agree to trust each other. Done once with an 8-character one-time code, or automatically when one program started the other. Result: a 256-bit key; the issuer keeps only its hash. |
| **Grant** | A short signed statement **made by Identity**: *who* (GitHub account), *what* (feature + actions), *for which identity* (e.g. TASS), *which Engine installation*, *until when*. Signed with Identity's Ed25519 key; anyone holding Identity's public key can check it **without Identity running**. |
| **Actions** | `install`, `start`, `attach`, `observe`. Each one is granted separately. |
| **Entity** | An identity being served (TASS, a business). Today the Engine creates these; see the decision about who owns them. |
| **TEST FIXTURE** | Any stand-in. Off by default and labelled on every screen it touches. |

### Proposed grant format (to argue about; the same text is in every repo)
```json
{
  "v": "idg1",
  "grant_id": "g-…",
  "issuer": "<Identity system ID>",
  "account": {"github_login": "…", "github_id": 123},
  "feature": "thought-engine",
  "actions": ["start", "attach", "observe"],
  "entity_id": "<entity, when the action concerns one>",
  "engine_installation_id": "<pinned once attached>",
  "rule": "repo-visibility-v0",
  "issued_at": "RFC3339", "expires_at": "RFC3339 (e.g. +24 h)"
}
```
- Sent as `base64url(json) + "." + base64url(ed25519 signature)`.
- The Engine **pins Identity's public key once**, during the first owner-approved setup, by showing its fingerprint to the owner.
- From then on, the Engine checks grants offline.
- Revocation in v1: short expiry; Processor refreshes grants from Identity. Revocation lists come later.

### Order of work across the three repos (suggested)
1. **Identity:** signing key + `grant` endpoint for paired Processors (G1). Keep sign-in across restarts (G2).
2. **Engine:** pin Identity's key, and accept a grant for `attach` / `observe` *instead of* the local attach code (E1). Lock the page and v1 routes (E2).
3. **Processor:** ask Identity for grants and hand them to the Engine (P1). Then become the runner that starts the Engine and Chat-Jimmy (P2).
4. **All three:** one shared Link package; one Windows AGENTS.md covering the whole journey; one end-to-end acceptance script run on Windows CI.

---

## Identity's job, in one sentence
**Identity says who the person is and what they may do, and signs that answer so the other programs can check it without asking again.** It never does the thinking, never runs the Engine, and never holds Thoughts.

## What Identity has today [KNOW]
- **`alpha-candidate-20261025`** (@ `7addeee`):
  - setup flow: Welcome → GitHub → Features → Thought Processor → Done
  - device sign-in with the built-in app `Iv23livvYZb0CafGVssP`; the token is kept in memory only
  - feature catalog with honest states; drafts marked **NOT SUBMITTED**
  - read-only Processor discovery
  - monitoring view, persistent redacted log, Copy Diagnostics
- **`experiment/engine-access-pairing-20261026`** (@ `884e386`):
  - Processor pairing: 8-character code, hashed token, HMAC proof
  - `GET /api/v1/features/thought-engine/access` (rule repo-visibility-v0)
  - release info and streamed download with `SHA256SUMS`
  - the #19 wording fixes

## What Identity must add to work the owner's way

### G1. Signed grants (the most important missing piece)
Today Identity's answer is "ALLOWED", HMAC-signed **with the pairing token**. Only Processor can check that, and only while Identity is running. [KNOW] The Engine needs to check Identity's approval *itself* (owner rule 3). [INFER]

- **Key:** create an Ed25519 key pair on first start, in the profile, 0600. Show its **fingerprint** in Manage → Identity (e.g. `ID-KEY 3F9A-…`).
- **Public endpoint:** `GET /api/v1/keys` returns the public key and fingerprint. It's public: a public key is not a secret.
- **Grant endpoint, for paired programs only:** `POST /api/v1/grants`, with body `{feature, actions, entity_id?, engine_installation_id?}`.
  1. Check the live GitHub sign-in and the rule (repo-visibility-v0 today).
  2. Return the signed grant (format in "the whole picture"), or a refusal with reason + next step, same as today's access answer.
- **Show every grant issued** in the activity log: grant ID, actions, entity, expiry. Never log the grant text itself (it's a bearer credential until it expires).
- **Expiry:** about 24 h by default; `install` grants about 10 min. Processor refreshes them.
- **Tests:**
  - a signature from another key is refused
  - an expired grant is refused
  - changing an action, entity or installation breaks the signature
  - with no sign-in → `NOT_SIGNED_IN`, and no grant is issued
  - a denied account gets no grant

### G2. Keep the GitHub sign-in across restarts (owner decision needed)
Today every Identity restart signs the person out [KNOW], and Processor then shows NOT_SIGNED_IN until they reconnect. [KNOW] (seen in the pairing demo)

| Option | Feel | Risk |
| --- | --- | --- |
| Keep as is | reconnect after every restart | safest |
| Store the GitHub **refresh token** in Windows Credential Manager / DPAPI (Linux: keyring, or a 0600 file with a warning) | sign in once | a stolen user profile can act as the person until revoked on GitHub |

With G1 grants lasting about 24 h, this matters less [INFER]: the Engine keeps working on a valid grant while Identity is closed.

### G3. Start or open Processor, and link automatically
The owner's 10-24 flow includes "start it through Project Controller → open it". [KNOW]

| Option | How |
| --- | --- |
| Through Project Controller | Identity asks Controller to run Processor; Controller's approval stays in charge (the approved launch-request experiment) |
| Directly | Identity runs `thought-processor.exe` from a path the owner approved once |

Either way, **if Identity started Processor, link automatically.** Pass a one-time link secret in an environment variable at launch, so nobody types a code. If Processor was already running, fall back to today's code.

### G4. Entities: who owns "TASS"? (owner decision)
Today the **Engine** creates TASS (from Processor or the Engine page). [KNOW] Earlier directions had Identity own identities (create, save, restore TASS or a business). [KNOW] (10-23)

| Option | Result |
| --- | --- |
| **Identity owns entities**; the Engine stores Thoughts under Identity's entity ID | one list everywhere; grants name the entity; Engine identities become a mirror |
| The Engine owns them | simpler now; Identity can't grant per entity |

[GUESS] Identity should own them, with a **small** first version: name + type + ID, no profile pages.

### G5. Install/download is already built; keep it narrow
Today: release info + streamed download + checksum. [KNOW] Needs:
- Contents:read on the GitHub App (owner approval)
- a first `engine-v…` tag
- the app installed on Thought-engine (owner)

[GUESS] Later the download can require an `install` grant, so it uses the same mechanism as everything else.

### G6. Shared Link package
Identity's pairing code (`internal/pairing`) is the most complete of the three implementations (it has lockout and HMAC). [KNOW] Make it the shared package the Engine and Processor also use, so "link" works identically everywhere.

## What Identity must NOT do
- Start, stop or talk to the Engine directly. That's Processor's job, with a grant.
- Hold Thoughts, observations or worker settings.
- Print or log any grant, token, code or `#key=` link.
- Show anything as GRANTED that it didn't sign.

## Acceptance checks for "Identity done for this stage"
1. Fresh profile → Connect GitHub → signed in, without creating an app. [already PASS on alpha candidate]
2. `GET /api/v1/keys` shows a fingerprint matching Manage → Identity.
3. Paired Processor asks for an `attach` grant → gets a signed grant. The Engine (separately) verifies it with the pinned key.
4. Brother's account without repo access → refusal with reason and next step; no grant.
5. Restart Identity → an existing grant still verifies until it expires. If G2 is chosen, the person is still signed in.
6. No grant, token or code in logs, Copy Diagnostics or the console.

## Questions for the owner
- G2: remember the GitHub sign-in, yes or no?
- G3: start Processor through Project Controller, or directly?
- G4: who owns entities like TASS: Identity or the Engine?
- Grant length: 24 h? Shorter?
- Approve Contents:read for the GitHub App, and pick the commit for the first `engine-v…` tag?

— Claude
