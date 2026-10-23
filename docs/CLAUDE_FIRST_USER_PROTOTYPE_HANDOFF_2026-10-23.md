# Claude handoff — Identity: first NEW ADMINISTRATOR prototype

**Planning instructions, not a completed feature.** [Full welcome flow](BOOTSTRAP_START_HERE_2026-10-23.md) · [Architecture](NEW_ADMIN_BOOTSTRAP_2026-10-23.md) · [Test matrix](NEW_ADMIN_TEST_MATRIX_2026-10-23.md) · [Identity issue #6](https://github.com/Arameair/identity/issues/6) · [Thought Controller's counterpart](https://github.com/Arameair/Thought-Controller/blob/experiment/multi-identity-controller-design-20261023/docs/CLAUDE_FIRST_USER_PROTOTYPE_HANDOFF_2026-10-23.md)

## The real goal

**We are going to act like a new administrator who has never installed our system.** On a clean, isolated TEST data folder, opening Identity should guide us through establishing an installation, discovering Thought Controller, remembering permission to request approved startup, checking GitHub availability, and opening the developer thought workspace.

Build a testable first-run experience; do not mistake a simulated installer for an actual publicly distributable product.

## Screens and interactions to implement first

1. **Welcome:** Create administrator or Restore setup. Create a durable administrator identity and a distinct installation ID; display and save both. Don't ask for GitHub first unless authentication is actually needed to obtain a private controller build.
2. **Controller check:** Show checking, found-and-verified, not found, incompatible, and wrong/untrusted instance accurately. A readiness check never sends a model request.
3. **Startup permission B:** On the first missing Thought Controller, explain the exact selected trusted source/version/build. Ask **Approve automatic startup for this specific controller on later launches?** Save consent, show revoke, and require renewed consent if selected source/version/trust changes.
4. **First mock launcher:** Initially use a bounded FAKE launcher/controller; show **SIMULATED** prominently. Deduplicate startup requests and report actual mock status. Don't start arbitrary executables or claim Thought Controller was really installed.
5. **System identity:** Create/restore a durable controller-service entity ID bound to the installation. It is NOT the administrator ID and NOT TASS's ID; its code comes from the owner's reference design, not the owner's data or credentials.
6. **GitHub status:** Show NOT CONNECTED / VERIFIED / OFFLINE / ACCESS REQUIRED. Do not initiate real OAuth, access tokens, private repo creation or downloads in the first mocked prototype. Both source repos are PRIVATE and have no approved published releases: show the constraint instead of assuming public access.
7. **Enter workspace:** Offer Open Thought Controller for an actual verified controller, or **Open simulated dashboard** for the mock. Display TASS and Business identity profiles, correctly scoped thoughts and empty states.

## Returning-user test

Stop Identity; close simulated controller; restart Identity using the SAME test data folder. The installation ID, administrator ID, controller service ID and approved startup policy must be unchanged. No duplicate service, repeated first-run wizard, new ID, or consent prompt for the same approved target. If the controller is already running, reuse it. If revoked or changed build, ask again. A SECOND test data folder produces completely different IDs/data/grants.

## Code and branch rules

Review the latest Alpha and the two unpromoted Identity branches: local V1 at commit 2b31e996706d8d4510964e4fa3a727b78bf1eab5 and current design notes. Start the NEW coding experiment from the latest OWNER-APPROVED Alpha, not an unpromoted experiment. If you need to reuse the V1 code, explicitly list commits/files to port; do not silently merge or promote it. Maintain independent Play/Stop, correct PC_BRANCH, PC_DATA, PORT, detached commit SHA, and a clearly selectable isolated test directory. Make ordinary UI use straightforward.

For this first pass, **do not** build a real GitHub connector, real controller installer/launcher, real registry API, autonomous thinking, or a public release. The first mock run is a test of onboarding and trust decisions, not a claim of production integration. Preserve the existing owner data. Report the missing real dependencies honestly.

## Owner acceptance checklist

| Test | What we'll do | Expected observation |
| --- | --- | --- |
| F1 | Launch with a new empty test profile | Welcome; zero prior identities; data path and branch/SHA visible |
| F2 | Create administrator | Stable distinct admin/installation IDs saved |
| F3 | Controller absent | Shows NOT FOUND, not fake Connected |
| F4 | Approve pinned TEST build automatic startup | Exactly one simulated startup; consent saved and revocable |
| F5 | Check GitHub | Unconfigured/offline shown; no tokens or repositories changed |
| F6 | Open simulated Thought Controller dashboard | Identity-scoped TASS/business tiles; explicitly simulated content |
| F7 | Stop, restart with same test profile | Same IDs/consent, no unnecessary first-run or duplicate starts |
| F8 | Revoke, change version, deny startup, new test profile | Accurate blocked/re-consent behavior, separate data |
| F9 | Copy diagnostics | Full source SHA, IDs, state, test labels, attempted starts, error and time; no secrets |

Return runnable experimental branch + full SHA, owner Play/Stop instructions, simulated/real distinction, F1–F9 PASS/FAIL/BLOCKED/NOT RUN, evidence, and rollback. No Alpha/main/stable changes without owner approval.