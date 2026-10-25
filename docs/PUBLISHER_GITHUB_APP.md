# Publisher: register Identity's GitHub App once, ship its Client ID

End users never create a GitHub App. **Arameair (the publisher) registers one app once**, puts its *public* Client ID into Identity's source, and every copy built from that source shows a plain **Connect GitHub** button. The Client ID is not a secret — GitHub designs it to be shipped inside apps. The device flow needs **no client secret**; never create or commit one.

## 1. Register the app (once, ~3 minutes)
github.com → your avatar → **Settings → Developer settings → GitHub Apps → New GitHub App** (or the same under the Arameair organization's settings if Arameair is an org).

| Field | Value |
| --- | --- |
| GitHub App name | `Arameair Identity` (any unused name) |
| Homepage URL | `https://github.com/Arameair/identity` |
| Callback URL | leave empty |
| **Enable Device Flow** | **ticked** (required) |
| Webhook → Active | unticked |
| Repository permissions → Contents | Read-only (Metadata becomes read-only automatically) |
| Where can this GitHub App be installed? | **Any account** if other people (e.g. family, testers) will sign in. "Only on this account" may stop anyone but you from authorizing it. |

Create it, then copy the **Client ID** (starts with `Iv`) from its General page.

## 2. Install it on the feature repositories
On the app page: **Install App → Arameair → Only select repositories → Thought-Processor, Thought-engine**. This is what lets Identity show "repository visible" for people who have access to those repos. It grants nobody access by itself.

## 3. Ship the Client ID
Edit `internal/publisher/publisher.go`:
```go
var GitHubAppClientID = "Iv23liXXXXXXXXXXXXXX"
```
Commit it. Every build from then on includes it; the console shows `github : built into this build (Arameair's GitHub App)` and Copy Diagnostics shows `kind=built-in`.

Alternative without a commit (for a one-off build):
```powershell
go build -ldflags "-X github.com/Arameair/identity/internal/publisher.GitHubAppClientID=Iv23li... -X main.buildBranch=experiment/identity-home-claude-20261025" -o identity-b.exe .
```

## How the choices stack (highest first)
1. `IDENTITY_GITHUB_CLIENT_ID` environment variable — developer override.
2. The person's **own** app, set up with Identity's built-in guide ("Use my own GitHub App instead") — saved in their profile; "Stop using it" returns to the built-in app.
3. `github-app.json` next to the exe or in the current folder — developer override.
4. **The built-in publisher app** — the normal case.

## Why a code, not a plain "log in" redirect
A redirect-style browser login needs the app's **client secret** at the end, and a secret can't be kept secret inside a program people download. GitHub's device flow is its official way for installed apps to sign in without one. Identity makes it feel like a browser login: Connect GitHub opens GitHub in a new tab and copies the code; if you're already signed in to GitHub you paste, press Continue, then Authorize.
