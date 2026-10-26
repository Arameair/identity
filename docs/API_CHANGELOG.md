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
