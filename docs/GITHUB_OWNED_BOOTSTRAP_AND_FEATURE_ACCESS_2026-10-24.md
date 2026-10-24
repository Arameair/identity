# GitHub-owned bootstrap and publisher feature access
Date: 2026-10-24
Status: owner direction plus design questions; documentation only, not implemented.
Human-directed project, developed in collaboration with AI. Credit to Dustin / Arameair for the originating project; precise attribution wording and license remain undecided.

## Read alongside
- [Current first-run design on main](https://github.com/Arameair/identity/blob/main/docs/FIRST_RUN.md)
- [First Experience launch brief](https://github.com/Arameair/identity/blob/experiment/first-run-experience-brief-20261024/docs/FIRST_EXPERIENCE_LAUNCH_2026-10-24.md)
- [Start Experience tracking](https://github.com/Arameair/identity/issues/16)
- [Grant decision tracking](https://github.com/Arameair/identity/issues/15)

This notes-only experiment starts from Alpha. It does not merge other experiments or promote Alpha/Stable/Main.

## Owner direction from this discussion
1. The product experience requires a verified connection to the user's own GitHub account. Setup and repair must remain accessible when that connection is missing.
2. The user's initial version lives in their own repository under their own account, reproducing the starting software/materials they need. Do not reduce this to a configuration-only repository: the owner explicitly rejected that assumption.
3. The starting version derives from Dustin's project, retains credit and links to its origin, and can connect to the user's GitHub account.
4. Dustin must be able to control which features users may access from his feature repositories.
5. Before presenting an operational workspace, establish the actual GitHub account, selected repository, Identity readiness, and current working identity.
6. Continue design discussion before committing to a finished interface or implementing a broad licensing/entitlement platform.

## Conflict with earlier plans — make explicit
Existing FIRST_RUN and First Experience notes say GitHub is optional and not required for local Identity readiness/the demo. The owner now requires GitHub for this product experience.
Reconcile the documents before implementation. Do not claim both policies are simultaneously settled. Whether low-level Identity initialization can still run locally for setup/recovery is a boundary question; it must not be used to bypass the product's operational GitHub gate.
Earlier demo plans excluding GitHub from acceptance are insufficient to demonstrate this revised first-opening journey.

## Keep four kinds of evidence separate
| Evidence | What it establishes | What it does not establish |
| --- | --- | --- |
| GitHub authentication and repository permission | Account and actual access to a specific repository | Companion identity or runtime permission |
| Publisher feature entitlement | Whether the publisher permits this customer to obtain/use a specified feature, under a defined policy | Automatic authority over local companion data |
| Identity runtime authorization | Whether a particular authenticated source/feature/instance may perform a scoped operation for an entity | Ownership of GitHub credentials or publisher repositories |
| Service/test receipts | Observed reachability and operation outcome for a particular instance and request | Permanent readiness of all features |

Do not infer paid entitlement from a GitHub username, discovery result, README, client-editable flag, or successful local launch.
Publisher entitlement authority/protocol is NOT decided. Do not presume the user's editable local Identity can authoritatively grant publisher entitlements to itself.
Never distribute Dustin's personal GitHub token, signing private keys, or publisher administrative credentials in starter repositories or application clients.

## Proposed first-opening flow (design proposal, not accepted API)
Open setup → connect/verify user's GitHub → create or select their starter repository with explicit confirmation → verify resulting ownership/access and origin/version → initialize or connect Identity → confirm operator and working entity → check available feature access → open the selected real Thought Processor instance.

If Identity is the launcher, it begins this journey and carries verified context into Processor; Processor must not silently substitute a different account, identity, or build. Preserve the earlier distinction between opening Identity and deliberately starting an Experience.

Explain repository creation, intended visibility, source version, and contents before the user approves it. No starter secrets or personal source-user records. Never overwrite an existing repository. Separate app source from runtime data/secrets; owning a repository does not imply committing all companion records to it.

Returning users should revalidate required evidence and show changed, unavailable, or stale context without silently creating another repository or identity. Define behavior for offline GitHub, expired credentials, lost repository access, Identity unavailable, and publisher entitlement unavailable.

## Important enforcement limitation
[INFER] Access to publisher-controlled private repositories or hosted services can be enforced at those resource boundaries. Copies already delivered into a user's editable repository or machine cannot be made reliably revocable merely by adding a local check. Public source access cannot be selectively restricted by a card in Thought Processor.

Describe the actual boundary, not an unqualified claim that a publisher can remotely disable any copied code. Do not implement a kill switch or delete customer data as an assumed consequence of entitlement loss.

## Decisions still open
- Template, fork, or controlled bootstrap mechanism? Define ownership, attribution, provenance and future update/conflict behavior before selecting.
- What exactly is reproduced into the customer's first repository, and what remains a separately delivered feature?
- Does publisher withdrawal prevent future downloads/updates, hosted use, continued installed operation, or some explicit combination? The owner has not answered this.
- Which service issues/verifies publisher entitlement evidence, and how does it relate to Identity's local grants?
- GitHub outage/session-expiry policy: new privileged actions versus already-running independent applications. Mandatory sign-in does not yet define outage or shutdown behavior.
- Attribution display versus license obligations: exact license and legal requirements are not selected.
- Companion entity, human operator, feature identity, installation and runtime instance remain distinct; GitHub account association alone does not collapse them.

## Small reviewable next deliverable
Prepare an implementation-free first-run walkthrough plus a responsibility table. Show a fresh account/repository path, returning installation, unavailable GitHub, inaccessible publisher feature and revoked local grant.
Label fixture demonstrations as fixtures. Any later real prototype must separately prove GitHub account/repo permission, actual repository creation, Identity readiness, and feature authorization.
Report conflicts and recommend one bounded next experiment. Do not start broad implementation, change repository visibility/permissions, provision customer repositories, or promote branches on the strength of this note alone.
