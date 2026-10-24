# Identity standalone access-request trial

Experimental branch `experiment/identity-standalone-access-trial-20261024`, derived from Claude's setup wizard experiment. Independent from Project Controller; `alpha`, `stable` and `main` remain unchanged.

## What actually works

- Standalone `identity` binary opens a loopback-only browser window; no Project Controller, remote API key or companion service required.
- Real GitHub App **device-flow** sign-in, reusing the existing wizard's `github.go`. It checks the real GitHub user profile after authorization; tokens remain in process memory and disappear when the app exits.
- Publisher label `Arameair/identity` baked into the build. **Label is not a cryptographically verified release provenance.**
- Feature catalog lists Thought Processor and Thought Engine, both **NOT GRANTED**.
- Signed-in user can save a **local access request draft**, restart and reload it, without creating duplicates. Optional export downloads an unverified JSON draft for manual review.
- Requests are **NOT sent to the publisher**. This build has NO publisher request API, backend queue, approval/revocation service or feature entitlement; it does NOT download/install/start Thought Processor. The program never grants access.

## Run this on a development machine

From `standalone/`:

```sh
go run .
```

Or build a platform executable:

```sh
go build -o identity .
```

Windows: `go build -o identity.exe .`.

Open the local link printed by the program; it also attempts to open your browser automatically. Keep the link private. Default port **7790**; use `-port <number>` if needed. For isolated testing use `-data <folder>`. Stop with Ctrl+C.

### GitHub app configuration

The owner already created a GitHub App for Identity with **Device Flow enabled**. On your first prototype run, paste that App's **public Client ID** (starts with `Iv`) into the `Publisher setup` field on the welcome screen and press **Save Client ID**. It is saved in the local data folder as `github-app.json`; no client secret required. Other users of a future signed release should never have to do this: the publisher's public Client ID can be embedded in a verified build.

Alternatively set `IDENTITY_GITHUB_CLIENT_ID` or put `{"client_id":"Iv..."}` beside the executable.

To test offline with fake GitHub, use **both** `-test-github-web` and `-test-github-api` (clearly labeled TEST MODE in UI). Fake GitHub is **not a real connection**.

## Owner acceptance checklist

1. Open Identity from executable on a clean data folder; no Controller required.
2. Enter the public GitHub App Client ID, click Connect GitHub; confirm official github.com and short device code.
3. Authorize on GitHub; confirm actual GitHub login appears.
4. See Arameair as publisher and the two feature cards, **neither granted or runnable**.
5. Press **Save access request** for Thought Processor; confirm `Saved locally — not sent`. Press twice/restart, confirm no duplicate and record persists.
6. Export the JSON draft, observe that it explicitly says **UNVERIFIED DRAFT — NOT AN ACCESS GRANT**.
7. Disconnect; the previous account's drafts must not show to another GitHub account.
8. Stop and restart; GitHub login is not silently reused, since token is intentionally memory-only.

**Pending work:** authentic signed publisher metadata; deliver requests to an actual publisher-hosted service; authenticated administrative approvals and durable grants; secure artifact download, runner, separate Engine permissions, release packaging and native Windows owner-machine testing. Do not call this a complete customer onboarding flow.

AI-assisted experimental implementation, for owner review. No Alpha promotion.
