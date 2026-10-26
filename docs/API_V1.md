# Identity API v1 — contract for Thought Processor

**Branch:** `alpha-candidate-20261026` · **API label:** `identity/1-alpha` · **Status:** alpha candidate, not promoted. Examples below are real responses captured from this build (tokens removed).

Thought Processor is the normal human interface (decision **C01**). Identity runs in the background, and Processor uses this API to show and manage it. A working reference client is in [`cmd/identity-probe`](../cmd/identity-probe/main.go). Copy its `call()` function: it does the proof and wrong-service checks described below.

## Rules every caller follows

| Rule | Why |
| --- | --- |
| Address is `127.0.0.1:<port>`; the default port is **7790**. Each running Identity writes a **per-user locator** at `%LOCALAPPDATA%\Arameair\Identity\instances\<identity_system_id>.json` (Linux/macOS: `<user config dir>/arameair-identity/instances/`). It holds `base_url`, `port`, `pid`, `run_id`, `identity_system_id` and `profile`. `<profile>/runtime/service.json` holds the same data minus the ID. Both are removed on a clean stop; a crash leaves a stale file, which you can spot because it doesn't answer. | Lets Processor find **the installation it paired with** (match `identity_system_id`) whichever program started first, and on whatever port, without knowing the profile folder. A locator only says where to look; it proves nothing. |
| Never send a browser `Origin` header. | Identity refuses requests that carry an `Origin`, so web pages can't call this API. |
| Apart from the two pairing calls, send `Authorization: Bearer <token>`. | Proves this caller is the program the person paired. |
| Send a fresh random `X-Processor-Nonce` on each call. Check `X-Identity-Proof` = hex(HMAC-SHA256(token, `"identity-v1\n" + nonce + "\n" + <exact body bytes>`)). | Proves the answer came from the Identity you paired with, and that it isn't an old answer replayed. |
| Check that `X-Identity-System-ID` equals the `identity_system_id` you saved when pairing. | If it differs, a **different Identity installation** answered. Show WRONG SERVICE. |
| A `401` means "this Identity doesn't know your token". | You were unpaired, or this is a different profile. Pair again. |
| No answer means **AUTHORITY UNAVAILABLE**. Block protected actions; do not use earlier answers instead. | C06: fail closed, no cached allow. |
| Never show, log or copy the token. Show the GitHub `user_code` only to the person who is signing in. | — |

**Which answers are signed:** every `/api/v1/*` answer that Identity's handlers produce, including refusals such as 400, 404 and 409. **Not signed:** `GET /api/health`, the two pairing calls, and refusals made *before* the token is accepted. Those are 401 (token refused), 403 (browser `Origin`), 421 (wrong `Host`), 429 (pairing too fast) and transport errors. Map 401 to "connection removed", not "wrong service". Treat any *other* unsigned or badly signed answer as wrong service.

`GET /api/health` is public and unsigned (`app`, `version`, `protocol`, `api`). Use it only to see that *something* is answering. It is not trust.

## Pairing (C05)

1. **Processor asks.** `POST /api/v1/pair/request` `{"processor_id":"tp_7f3a","processor_label":"Thought Processor"}`
   → `202 {"status":"CODE_SHOWN_IN_IDENTITY","expires_at":…,"next":"POST /api/v1/pair …"}`
   - The code is **not** in this answer. Identity prints it in its own console window, and its troubleshooting page also shows it.
   - Processor's screen should say: *"Look at the Identity window and type the code shown there."*
   - Requests less than 3 seconds apart get `429` with a `Retry-After` header.
2. **The person types the code into Processor.** Processor sends `POST /api/v1/pair` `{"code":"ABCD-EFGH","processor_id":"tp_7f3a","processor_label":"Thought Processor"}`
   → `200 {"token":"<64 hex>","identity_system_id":"idn_…","identity_version":"0.4.0-alpha"}`
   - The code works once and expires after 5 minutes.
   - After 5 wrong codes, pairing locks for 1 minute and the current code is burned.
3. **Save** `token` and `identity_system_id` in Processor's profile with file mode 0600. After a restart of either program, reconnect with them. No new code is needed.
4. **Disconnect:** `POST /api/v1/unpair` `{}` → `200 {"unpaired":true}`. This answer is still signed, and the token stops working immediately after it. The person can also unpair from Identity's troubleshooting page.

Pairing grants **no** feature or data access.

## Status — `GET /api/v1/status`

Returns these sections:

- `state` is always `READY` when Identity answers.
- `identity_system_id` and `installation`.
- `build`: version, branch, commit.
- `lifecycle`: `pid`, `run_id`, `port`, `started_at`, `uptime_s`, `ownership`, `started_by`.
  - `ownership` is `MANAGED` if Identity was started with `-managed-by <who>`, otherwise `EXTERNAL`.
- `you`: the paired processor.
- `github`: see the GitHub section below.
- `identities`: counts.
- `authority`: grants, default policy, cache policy, and the last Engine install check.
- `not_implemented`: a list. Show it honestly.

## Identities (C03)

| Call | Result |
| --- | --- |
| `GET /api/v1/identities` (`?archived=1` to include archived) | `{"identities":[…],"count":n,"kinds":["person","companion","business","project"]}` |
| `POST /api/v1/identities` `{"name":"Tass","kind":"companion"}` | `201` record. `409` if an active identity already has that name. `400` for a bad name or kind. `system` cannot be created. |
| `GET /api/v1/identities/{id}` | record or `404` |
| `POST /api/v1/identities/{id}/rename` `{"name":"…"}` | record |
| `POST /api/v1/identities/{id}/archive` · `/restore` | record. Nothing is ever deleted. |

Example record:

```json
{"id":"ent_5d08a8362f646f9e8915","name":"Tass","kind":"companion","created_at":"…","created_by":"processor:tp_7f3a","updated_at":"…","enabled_features":[],"authorized_sources":[]}
```

`id` never changes. Use it everywhere, never the name. The two access lists are always empty in this alpha.

## Features (C02)

Call `GET /api/v1/features`, or `GET /api/v1/identities/{id}/features` to get the same states plus `enabled_for_identity` and `authorized_sources`. Each feature reports separate states. **Show them separately**:

| Field | Values today | Meaning |
| --- | --- | --- |
| `eligible_to_install.state` | Thought Engine: `ALLOWED` · `NOT_ALLOWED` · `NOT_SIGNED_IN` · `SIGN_IN_EXPIRED` · `CHECK_FAILED`. Thought Processor: `NOT_APPLICABLE`. | Rule `repo-visibility-v0`. **Install only.** It is not permission to run the feature for an identity or to read any data. The result includes `reason` and `next_step` in plain language. |
| `installed`, `running` | `UNKNOWN_TO_IDENTITY` | Processor knows these. Identity doesn't track them yet. |
| `service_registered` | Thought Processor: `PAIRED`. Others: `NOT_REGISTERED`. | Features don't register with Identity yet. |
| `enabled_for_identity` | `NOT_GRANTED` | No grants exist yet. |
| `authorized_sources` | `[]` | No source is allowed by default. |

## Authorization (C03, C06) — `POST /api/v1/authorize`

Request: `{"identity_id":"ent_…","feature":"thought-engine","operation":"observe","source":"discord"}`

Response:

```json
{"decision":"DENIED","state":"NO_GRANT","reason":"No grant allows this. Every identity starts with no access, and grants can't be created in this alpha.",
 "decision_id":"dec_…","checked_at":"…","cache":"none","valid_for_seconds":0,"policy":"default deny (C03); ask again before every protected action; …"}
```

`state` is one of `NO_GRANT`, `UNKNOWN_IDENTITY`, `IDENTITY_ARCHIVED` or `UNKNOWN_FEATURE`. Every answer is **DENIED** in this alpha. Use this call to build the "blocked, and here is why" path now; grants come later.

## GitHub sign-in from Processor

| Call | Result |
| --- | --- |
| `GET /api/v1/github` | `state`: `SIGNED_OUT` · `WAITING` · `VERIFIED` · `EXPIRED` · `DENIED` · `ERROR` · `NOT_CONFIGURED`, plus `next_step` |
| `POST /api/v1/github/connect` | Starts device sign-in. The answer includes `user_code` and `verification_uri`. Show both to the person, then poll `GET /api/v1/github` every few seconds until `VERIFIED`. |
| `POST /api/v1/github/cancel` · `/disconnect` | — |

Two things to tell the person:

- A `VERIFIED` account is **not** permission to use any feature.
- The GitHub token stays inside Identity and only in memory, so restarting Identity signs GitHub out.

## Shutdown (C04) — `POST /api/v1/shutdown`

Send this only when the person chose **Shut down managed workspace** on explicit Processor exit.

Request: `{"confirm":"shut-down-identity","reason":"person chose Shut down on exit"}`
→ `202 {"stopping":true,"pid":…,"run_id":…}`

- Without `confirm` the answer is `400`.
- To verify the stop, check that the pid exits and that `runtime/service.json` is gone. Identity logs the reason.
- For **Keep services running**, do nothing.
- If `lifecycle.ownership` is `EXTERNAL`, Processor did not start this Identity. Ask the person before stopping it.

Identity never starts or stops any other program.

## Installing a feature — generic routes (use these)

| Call | Result |
| --- | --- |
| `GET /api/v1/features/{id}/eligibility` | `{feature, eligible_to_install{state, rule, reason, next_step, …}, install_adapter}`. `install_adapter` is `"NONE"` when Identity has no install rule for that feature. |
| `GET /api/v1/features/{id}/release?platform=windows-amd64` | `{tag, asset, size, sha256, decision_id, …}`. Returns 403 `not_allowed` (with the decision) when not eligible, and 404 `NO_INSTALL_ADAPTER` for features without a rule. |
| `GET /api/v1/features/{id}/release/download?platform=…&tag=…` | The asset bytes, streamed by Identity. Its GitHub token never leaves Identity. Check the SHA-256 from `release` yourself. A 409 means a newer release appeared since you asked. |

Thought Engine is the **only adapter** so far (rule `repo-visibility-v0`). A future feature gets an adapter in Identity, and Processor's code stays the same.

**Honest limit:** the checksum comes from the same GitHub release as the file. It proves the download wasn't damaged; it does **not** prove the publisher made it. Publisher-signed release metadata is not built yet.

The older Engine-only paths (`/api/v1/features/thought-engine/access`, `…/release`, `…/release/download`) still work for `experiment/install-thought-engine-20261026`. New code should use the generic routes.

## Storing the pairing on Windows

Identity itself stores **no secrets**: `paired-clients.json` keeps only SHA-256 hashes of tokens. Processor holds the token. Keep it inside the user's profile folder (`%USERPROFILE%` or `%LOCALAPPDATA%`), which Windows already restricts to that user. "0600" in these notes means "readable only by this user". **Better [not built]:** protect the token with DPAPI (`CryptProtectData`, current-user scope). That stops other Windows users and offline copies from reading it, though not other programs running as the same user.

## Not in this API yet

- Grants, and enforcement by the feature that receives a request.
- Feature service registration.
- Per-person administrator sign-in. In this alpha, a paired Processor acts for the operator who approved the pairing.
- Key rotation, and OS-protected token storage such as DPAPI.
- Server-sent events. Poll `status` instead; every 5–10 s is fine.
