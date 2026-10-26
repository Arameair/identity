# Identity API changelog

The API label is reported by `GET /api/health` (`api`) and `GET /api/v1/status` (`api`). Processor should show **INCOMPATIBLE** when the label's major part differs from `identity/1`.

## identity/1-alpha — 2026-10-26 (branch `alpha-candidate-20261026`)

First API for Thought Processor. Full contract: [API_V1.md](API_V1.md).

- **Pairing:** `pair/request` shows the code in Identity's window only. Then `pair`, then `unpair`.
- **Signed answers:** `X-Identity-Proof`, `X-Identity-System-ID` and the `X-Processor-Nonce` request header.
- **Read routes:** `status`, then `identities` (create, rename, archive, restore), then `features`, then `identities/{id}/features`.
- **Decisions and lifecycle:** `authorize` (always DENIED, `cache: none`). Then `github` (connect, cancel, disconnect), then `shutdown` (requires confirm).
- **Locators:** `runtime/service.json`.

### Added the same day, after the Processor connection handoff review

- **Per-user locator** `…\Arameair\Identity\instances\<identity_system_id>.json`. Lets Processor find the installation it paired with without knowing the profile folder, and after a port change. `identity-probe` now uses it.
- **Generic install routes** `features/{id}/eligibility`, `features/{id}/release` and `features/{id}/release/download`, with Thought Engine as the only adapter. Engine-specific paths kept for older experiments.
- The Engine `release` answer and its 403 now also carry the signed headers.
- **Docs:** which answers are signed (and which can't be), and how to store the pairing token on Windows.

No breaking changes to earlier `identity/1-alpha` calls.

## identity/1-alpha + feature contract — 2026-10-26 (branch `experiment/feature-contract-20261026`)

Experiment for Thought Processor P0 (reusable feature onboarding). It does not change `alpha-candidate-20261026`.

- **`GET /api/v1/features/{id}/contract`** returns the **feature contract `thought.feature/1`**. It has these parts:
  - `source`: the approved release source. It names the repo, `tag_prefix`, `asset_template`, `checksums` file, `trusted_uploaders` and `allow_prerelease`.
  - `install`: the install kind and `max_bytes`.
  - `run`: how the feature runs. This is information only; Processor doesn't start features yet.
  - `would_ask_for`: the permissions the feature would request later. Nothing is granted.
- **Private features are hidden.** A `private` feature only appears to an installation whose verified GitHub account passes the feature's rule. For Thought Engine, that rule is `repo-visibility-v0`. To everyone else, every route answers exactly like an unknown feature (404 "Identity doesn't know a feature called …"). That includes `features`, `identities/{id}/features`, `eligibility`, `contract`, `release`, `download`, `authorize` and the old Engine-only paths. `catalog_note` tells signed-out people to sign in, without naming anything.
- **Approved-source enforcement on `release` and `download`.** The release must meet all of these:
  - it comes from the contract's repository;
  - its tag starts with `tag_prefix`;
  - it is not a draft, and not a pre-release unless the contract allows it;
  - the asset **and** the checksum file were uploaded by a trusted uploader (`github-actions[bot]`, i.e. the repository's own release workflow).

  Anything else returns **422 `UNAPPROVED_SOURCE`**. `release` now includes `source` (the repo, tag, URL, uploaders and the checks performed) and `not_checked`, which says honestly that there is no publisher signature yet.
- **Machine-readable release errors**, all signed: `NO_RELEASE`, `NO_ASSET_FOR_PLATFORM`, `UNAPPROVED_SOURCE`, `CANNOT_VERIFY`, `GITHUB_UNREACHABLE`, `BAD_PLATFORM`, `NO_INSTALL_ADAPTER`.
- **Behaviour change.** Before sign-in, Thought Engine no longer appears as `NOT_SIGNED_IN`; it is hidden. Processor should show `catalog_note` instead.
