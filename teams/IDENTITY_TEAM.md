# Identity Team (`IDN`) — team identity

**Status:** Proposed team identity, v0.1, October 31, 2026. No one is appointed; all five roles are **unassigned**. Shared rules: [TEAM_OPERATING_MODEL.md](TEAM_OPERATING_MODEL.md). Other teams: [README.md](README.md).

## Who we are

| | |
| --- | --- |
| **Team code** | `IDN` |
| **Repository** | [Arameair/identity](https://github.com/Arameair/identity) |
| **Owner / final approval** | Dustin (Arameair) |
| **Supervising identity** | None assigned. Dustin directs. |
| **Roles** | Project Lead · Engineer · Test Architect · UX Architect · Team Liaison (all unassigned) |

**Our mission:** Be the one place that decides **who exists** and **who may do what, for whom**. Every other team relies on our answer, so our answer must be correct, narrow and checkable, and it must fail closed when we can't give one.

**The sentence we live by** (owner principle, 2026-10-24):

> Identity owns the organization and authority of the system. Each feature owns its actual work.

## What we own

- Stable IDs: installation, Admin, identities (TASS, KYSSA, a business…), and each installed feature copy.
- Admin enrollment and the Admin's approvals, made on **Identity's own page**.
- Feature installation records (which exact package, which digest, approved by whom).
- Scoped grants: **one capability, for one identity, for one feature installation**. Plus revocation.
- Single-use request tickets, and the **live check** a receiving feature calls before acting.
- GitHub device-flow sign-in, with the publisher's GitHub App Client ID built in so users never create their own app.
- The audit trail of decisions (who, what, when, never the content).

## What we don't own

- Screens people use day to day. That is `TP`. Our own page is for Admin approvals and troubleshooting only.
- Any feature's work or data: TASS conversations, Thoughts, Engine records.
- Running or installing features. `TP` does that, with our approval.
- Deciding whether a feature is a good idea. We decide whether it is **allowed**.

## Where we are right now [KNOW, from the repo on 2026-10-31]

- `alpha` @ `a582f8e` (2026-10-30): part of the coordinated TASS alpha with Processor `e4b45a9`, TASS `ae33d33` and Thought `772200b`. Adds opt-in `-no-open-link-file`.
- `main` @ `8d16772`: documentation only. Its README status (October 27) is older than `alpha`.
- Open: [#15](https://github.com/Arameair/identity/issues/15) observation grant check design gate · [#20](https://github.com/Arameair/identity/issues/20) alpha promotion gate · [#22](https://github.com/Arameair/identity/issues/22) Bitdefender autofill clearing the Admin enrollment form.
- Known gaps (from `docs/SECURITY.md`): processes under the same OS user aren't isolated; Admin approval uses a local passphrase; recovery is deferred; real third-party publisher provenance isn't finished.

## What each role means on this team

| Role | On the Identity team |
| --- | --- |
| **Project Lead** | Guards scope. Says no to anything that would make Identity a second dashboard or a data store for features. Owns the promotion-gate checklist (#20). |
| **Engineer** | Go service, API (`docs/API_V1.md`), ticket and grant code. Never weakens a check to make another team's demo pass. |
| **Test Architect** | Proves denials, not just successes: cross-identity denial, revocation, expired/replayed tickets, Identity offline, corrupt store. Reports PASS / FAIL / NOT RUN per check. |
| **UX Architect** | The Admin approval page: who is asking, for which capability, for which identity, and what happens if you say yes. Plain words for a non-technical person. |
| **Team Liaison** | Tracks every incoming request for a new capability or contract change, and makes sure each one gets an answer. |

## Working with the other teams

| Team | We give them | We need from them |
| --- | --- | --- |
| **`TP` Thought Processor** | Pairing, Admin approval prompts, install approvals, tickets, decisions, readable denial reasons | Requests that name the exact identity, capability and feature installation. Never asking us to approve something the person didn't see. |
| **`TASS`** | Grants for `tass.chat.*`, `tass.self.read`, `tass.model.set`; live checks on every request | Capability names and descriptions inside our limits (200 characters); a live check before **every** action |
| **`THT` Thought** | Grants for `thought.event.submit`, `thought.thoughts.read`, `thought.thought.update`; live checks | Same: separate read / write capabilities, a live check first, data kept per identity |
| **`TE` Thought Engine** | Install eligibility and separate data grants (when Engine joins P1) | Engine must check our grant before accepting or processing an observation ([TE #1](https://github.com/Arameair/Thought-engine/issues/1), [TE #4](https://github.com/Arameair/Thought-engine/issues/4)). Engine-local switches and attach keys are **not** grants. |

### Our contracts (other teams build on these)

Changing any of these needs a handoff to every affected team **before** the change lands:

- the Identity API and its changelog (`docs/API_V1.md`, `docs/API_CHANGELOG.md` on `alpha`);
- ticket fields and the live-check call;
- capability naming rules and limits;
- the meaning of "installed" vs "authorized" vs "revoked".

### How to reach us

Open an issue in this repo titled `[HANDOFF HO-<YOUR TEAM>-IDN-<date>-<nn>] …` with the handoff card from the operating model. Our Liaison answers `RECEIVED`, then our Project Lead answers `ACCEPTED`, `DECLINED` or `NEEDS-INFO`.

## Open questions we know about

- **Who owns "Thought" logic?** `TE` and `THT` both implement Spark → Trace → Thought. For us it only changes which capability names we issue, but Dustin should decide who owns what. (Recorded as **CONFLICT** in the `TE` and `THT` team files.)
- **Grants for team members themselves.** If AI team members (Liaison, workers) ever act through the system rather than through GitHub, each one needs its own identity and narrow grants. Not designed yet.

## What we will never do

- Grant access because someone has GitHub access, a reachable port, or asked politely.
- Say "approved" without a record a person made.
- Read a feature's content to make a decision.
