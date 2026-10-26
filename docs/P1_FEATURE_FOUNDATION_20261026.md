# Identity — P1 Feature Foundation (experiment, 2026-10-26)

**Branch:** `experiment/p1-feature-foundation-claude-20261026`, created from `alpha` at `c8c0a54ab333077ccbfbd86c5952fb8a00b55dc1`. Not promoted. Thought Processor counterpart: `Arameair/Thought-Processor` same branch name. Assignment: [Thought-Processor #21](https://github.com/Arameair/Thought-Processor/issues/21). How to run and test: Thought-Processor `docs/P1/START_HERE.md`.

Also included (cherry-picked with `-x`, authorship kept) from the parallel `experiment/identity-p1-security-hardening-20261026`: unpair persisted **before** signed success; 30-minute operator-session expiry; **durable signed-POST nonce ledger** (replay refused across restart; unavailable ledger → 503, fail closed).

## What P1 adds

| Object | File (profile, 0600) | Rule |
| --- | --- | --- |
| **Admin Identity** `adm_…` | `admin.json` | Created **once** (atomic link-if-absent), only after a GitHub-verified sign-in **and** explicit confirmation in Identity's own page, with a local authenticator. Binds the GitHub **numeric id** as a sign-in alias. A damaged file is never treated as "not enrolled". |
| **Local authenticator** | `admin.json`, `admin-throttle.json` | `local-passphrase-pbkdf2-sha256` (600 000 iterations, 16-byte salt). Typed only in Identity's approval page; never stored or sent to Processor. 5 wrong → lockout 5 min, doubling to 1 h, persisted. Works offline. Windows Hello / passkeys: **BLOCKED (not implemented)**. |
| **Approval** `apr_…` | `approvals.json` | One fresh Admin decision bound to one exact action (kind, requester, SHA-256 of the action), valid 10 min, consumed **before** the action runs (never twice; a retry returns the recorded result; a crash mid-action → "uncertain, ask again"). One-time view link. |
| **Installation identity** `ins_…` | `installations.json` | One per installed copy, minted by Identity, **zero grants**. Idempotent by install key. `PENDING_CONFIRM → INSTALLED` (the feature's credential must sign the confirm) `→ UNINSTALLED` tombstone; `ABANDONED` for an unconfirmed one. Only `TRUSTED_LOCAL_TEST_FIXTURE` from `local-trusted-package` can be registered; everything else `UNTRUSTED_EXECUTION_BLOCKED`. |
| **Installation credential** | `installation-keys.json`, `installation-nonces.json` | Separate registry from paired Processors (a key from one can never pass as the other). identity/2 signing with `X-Installation-ID`. Rotated on a pre-confirm retry; removed on uninstall. |
| **Grant** `gnt_…` | `grants.json` | requester (`processor:…` or `installation:…`) → destination installation → **one capability** → subject identity; source scope `none`; optional expiry; revocation bumps the version and kills outstanding tickets. Created only from a consumed Admin approval. |
| **Ticket** `tkt_…` | memory only | 60 s, single use, bound to request body digest, signed with a key only Identity and the destination know. Restart forgets them (fail closed). |

## Routes (all program routes refuse a browser `Origin`; every answer on signed routes carries `X-Identity-Proof`)

The full, test-enforced inventory with one guard per route is `internal/app/routes.go` (`TestP1ElevatedRoutesRequireAdminApproval` fails if a route is unclassified).

| Route | Guard | Notes |
| --- | --- | --- |
| `GET /api/v1/admin` | paired | state NOT_ENROLLED / ENROLLED / DAMAGED, binding, current GitHub and whether it is the Admin's |
| `POST /api/v1/admin/enrollment` | paired (request only) | needs a verified GitHub sign-in; 409 `ALREADY_ENROLLED` afterwards |
| `GET /api/v1/approvals/{id}` | paired | requester's own approvals only |
| `POST /api/v1/installations/requests` | paired (request only) | validates the exact install proposal |
| `POST /api/v1/installations` | **ELEVATED** | `{approval_id}` → installation + credential (once; rotated on pre-confirm retry) |
| `GET /api/v1/installations[/{id}]` | paired | with grants and recent decisions |
| `POST /api/v1/installations/{id}/abandon` | paired | own unconfirmed registration only |
| `POST /api/v1/installations/{id}/uninstall/requests` · `…/uninstall` | paired · **ELEVATED** | revokes grants, removes credential, keeps tombstone |
| `POST /api/v1/grants/requests` · `POST /api/v1/grants` | paired · **ELEVATED** | re-validated at commit |
| `GET /api/v1/grants` · `POST /api/v1/grants/{id}/revoke` | paired | revoke narrows access → no approval, but **only the grant's requester or the program that asked for it** (cleanup, Grok M2); others `403 REVOKE_NOT_PERMITTED`; persisted before success; `revoked_by` recorded |
| `POST /api/v1/installation/grants/{id}/revoke` | installation | the destination feature may end grants naming it |
| `POST /api/v1/grants/{id}/revoke/requests` · `…/admin-revoke` | paired · **ELEVATED** | Admin emergency revocation of any grant (approval kind `revoke_grant`) |
| `POST /api/v1/tickets` | paired | only under an ACTIVE grant for this exact requester/recipient/subject/capability |
| `GET /api/v1/decisions` | paired | recent decisions (all are also in the persistent log) |
| `POST /api/v1/installation/confirm` · `GET /api/v1/installation/self` · `POST /api/v1/installation/tickets` | installation | the feature's own credential |
| `POST /api/v1/authz/check` | installation | **the live decision**; consumes the ticket; anything but ALLOW is not allow |
| `GET /approve` · `POST /api/approval/open` · `GET /api/approval/view` · `POST /api/approval/decide` | approval page | one-time link → approval-scoped cookie + CSRF + Origin; the passphrase is checked here |

Elevated routes all go through one guard, `beginApproval` (`internal/app/p1.go`): approval ID present and valid, Admin record healthy, approval kind/requester/expiry/state/digest correct, decided by this installation's Admin, consumed before execution. `"approved": true` or pairing alone gets `403 ADMIN_APPROVAL_REQUIRED`.

`POST /api/v1/authorize` (the P0 route) is unchanged and still always DENIED.

## P1 cleanup (branch `experiment/p1-cleanup-claude-20261026`, Grok findings #22/#23)

- **M1 pinned artifacts.** `InstallRequest.Validate` now requires the exact `artifact_sha256` to be pinned: built-in `authority.BuiltinFixturePins` (the reviewed `p1-deterministic-worker 0.1.0`, same digest as Thought-Processor's `featpkg.BuiltinFixturePins`) or `-trust-fixture-artifact <sha256>` (repeatable). The claimed `trust_class` is no longer enough (`403 UNTRUSTED_EXECUTION_BLOCKED: ARTIFACT_NOT_PINNED`). Re-checked at commit, and every ticket / live check for an installation whose artifact is no longer pinned is denied (`ARTIFACT_NOT_PINNED`). Installations record `trust_basis`.
- **M2 revocation.** See routes above. `Grant.requested_by` is stored for new grants; legacy grants can be revoked directly only by their named requester (or the destination feature, or the Admin).
- **M3 approval page.** `POST /api/approval/open` and `GET /api/approval/view` add a `display` object built only from Identity's records: requester label + paired time (from the pairing store — the label is the one printed next to the pairing code), `is_primary` / `warning` (the primary Thought Processor is the program that asked for the Admin enrollment, stored as `primary_requester` in `admin.json`; older records → a warning on every approval), feature name/version/trust basis, the one capability + description, subject name/kind, information sources none, expiry, effect; raw IDs/digests in `advanced`. Grants default to **1 hour**; `until_revoked: true` needs `confirm_until_revoked: true` on decide (else `400`), and an unconfirmed no-expiry approval can't be committed. Old stored grants keep their stored expiry.
- **L2 pairing codes** are bound to the `processor_id` that asked for them: another program's attempt is `403` and doesn't count toward the 5-wrong-codes lockout or burn the code. The label shown next to the code becomes the paired label. (Requesting a new code still replaces the pending one — documented limit.)

## Tests

`go test ./...` (also with `-race`): `internal/authority` (RFC 7914 PBKDF2 vectors, enroll-once race, damaged admin, persisted throttle, approval lifecycle incl. tampered action and expiry, grant/ticket/check incl. wrong recipient, digest, replay, expiry, revoke, persistence, damaged grants, install idempotency and trust rule) and `internal/app/p1_test.go` (real HTTP server: enrollment once + account switch + restart + damaged record; route inventory; elevated routes without/with wrong approvals; credential separation; idempotent commit with rotation; full grant/ticket/check/revoke/expiry/restart/archive/uninstall/damaged-storage flow). Real multi-process tests are in Thought-Processor `processor/acceptance/`.

## Honest limits

- **Bootstrap is trust-on-first-use**: whoever controls this computer during first enrollment (with a GitHub account Identity verified) becomes Admin.
- **Same-OS-user processes are not contained.** They can read profile files (offline passphrase guessing against the PBKDF2 verifier; installation credentials), key-log the browser, or call loopback ports. No OS sandbox exists. Grants bind cooperating processes.
- Transport is loopback HTTP with HMAC authentication, not encryption.
- Signed-POST nonce ledger: at most 4 096 POSTs per 3 minutes per registry (≈22/s sustained), then 503 (fail closed); written with rename, not fsync.
- Recovery (lost passphrase / device / GitHub account): **DEFERRED** by the owner — a disposable test profile can become unrecoverable; no fallback owner is ever created.
- Not built: information-source grants, invitations, publisher signatures/provenance, Windows authenticators, Windows run.
