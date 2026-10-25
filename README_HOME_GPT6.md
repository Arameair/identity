# Identity Prototype A — GPT-6
An experimental **real local Go application** on the owner-assigned branch. Do not promote automatically. Created with AI collaboration.

## Run (PowerShell; Go installed)
```powershell
git clone --branch experiment/identity-home-gpt6-20261025 https://github.com/Arameair/identity.git
cd identity
go test ./...
go build -o identity-home-a.exe .
.\identity-home-a.exe -port 7797 -data "$env:USERPROFILE\identity-home-gpt6-a"
```
Open http://127.0.0.1:7797/ and read the private operator code from `$env:USERPROFILE\identity-home-gpt6-a\runtime\operator-code` **only on that machine**. Paste code only in local page. Ctrl+C to stop.

To run Processor in another terminal: clone `Arameair/Thought-Processor` branch `experiment/processor-controller-launch-fix-20261024`; from its `processor` directory run `go test ./...`, `go build -o thought-processor.exe .`, `.\thought-processor.exe -port 7860 -data "$env:USERPROFILE\tp-home-trial"`. In Identity > Setup, set `127.0.0.1:7860`. Processor owns its own private operator sign-in code.

Identity listens on loopback only. Default profile is `~/.identity/home-gpt6-a`. It has a durable System ID and entity cards (create/archive/restore), persistent rotating local logs, private Copy Diagnostics, device-flow GitHub login if a **public** GitHub App Client ID is configured, and **read-only** Processor health discovery. Do not share operator codes, cookies or GitHub device codes.

Configure GitHub App Client ID (public, no secret) via `IDENTITY_GITHUB_CLIENT_ID` or `github-app.json` adjacent to executable: `{"client_id":"Iv..."}`. Device flow is the official GitHub API; only a verified `/user` response changes to connected. Credentials stay in RAM.

**Non-claims:** health self-report does not authenticate Processor. Login does not confer publisher permission. Feature drafts are not submitted. No grants, automatic installs, secure Engine authorization or worker calls exist.

**Troubleshoot:** A crashed process can leave `process.lock`; verify the PID in that file is not running before deleting only that lock. Profiles from earlier experiments are not imported or modified. Rollback: Ctrl+C; switch Git branch back; leave or remove the isolated A profile only after backing it up. No Alpha, Stable or Main changes.

Windows owner run and genuine Processor-to-Identity live discovery have **not** been observed here; do not treat automated CI as owner acceptance.
