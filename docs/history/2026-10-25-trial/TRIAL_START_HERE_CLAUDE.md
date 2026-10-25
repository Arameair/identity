> **Historical record of the 2026-10-25 home trial (Prototype B).** Names, ports and paths below are from that trial; the current app is described in the repository [README](../../../README.md).

# Identity Prototype B (Claude) — home trial start here

**Branch:** `experiment/identity-home-claude-20261025` (from `alpha` 8d08fec). Experimental — not promoted anywhere.
**Implementation commit:** see the branch head; the latest code change is "built-in publisher GitHub App + guided own-app setup" (2026-10-25).
**Runs beside Prototype A:** B uses port **7798** and profile `%USERPROFILE%\.identity\home-claude-b`; A (GPT-6) uses 7797 and its own profile. Thought Processor uses 7860.

Results and evidence: [docs/HOME_TRIAL_EVIDENCE_CLAUDE.md](HOME_TRIAL_EVIDENCE_CLAUDE.md).

## Windows — first run (PowerShell, Go installed)

**Window 1 — Thought Processor (the existing candidate, unchanged):**
```powershell
cd $HOME
git clone --branch experiment/processor-controller-launch-fix-20261024 https://github.com/Arameair/Thought-Processor.git
cd Thought-Processor\processor
go build -o thought-processor.exe .
.\thought-processor.exe -port 7860 -data "$env:USERPROFILE\tp-home-trial"
```
Leave this window open. Its `#code=` link is Processor's own private sign-in — don't paste it anywhere.

**Window 2 — Identity B:**
```powershell
cd $HOME
git clone --branch experiment/identity-home-claude-20261025 https://github.com/Arameair/identity.git identity-b
cd identity-b
go build -ldflags "-X main.buildBranch=experiment/identity-home-claude-20261025" -o identity-b.exe .
.\identity-b.exe
```
(or `.\scripts\build-windows.ps1`, which also runs vet and tests.)

Your browser opens Identity's welcome screen. If it doesn't, copy the `open :` link from Window 2 into the browser. That link is private and changes every start.

**GitHub sign-in.** Normal users never set up a GitHub App: the publisher's app ships inside Identity and they just press **Connect GitHub**. Identity opens GitHub in a new tab with the code already copied — paste, *Continue*, *Authorize*.
- **Publisher (you), once:** register the app and put its public Client ID in `internal/publisher/publisher.go` — see [docs/PUBLISHER_GITHUB_APP.md](../../PUBLISHER_GITHUB_APP.md). Until then this branch has **no app built in**.
- **Without a built-in app** (this branch today, or for testing): the GitHub step offers **Help me set up a GitHub App**, a 5-step guide inside Identity that opens GitHub's registration page pre-filled and takes the Client ID from you. You can also skip GitHub; Identity still works.

## What you'll see

1. **Welcome** — what Identity will and won't do, your System ID and profile folder.
2. **GitHub account** — *Connect GitHub* opens GitHub's device page in a new tab and copies your code. Paste it, press Continue and Authorize; Identity then shows the account GitHub confirmed, the publisher app installations you can see, and whether Thought-Processor / Thought-engine repositories are visible to you.
3. **Features** — Thought Processor and Thought Engine, each with *Availability*, *Request*, *Grant*. You can save a local access-request draft; it is labelled **DRAFT SAVED LOCALLY — NOT SUBMITTED**. Grant is always **NOT GRANTED** (no grant service exists).
4. **Thought Processor** — address is pre-filled `127.0.0.1:7860`; press *Use this address*. Expected: **DETECTED — SELF-REPORTED, UNVERIFIED**, plus an *Open Thought Processor* button (Processor will ask for its own code).
5. **Done → Finish setup** — Identity switches to the small monitoring view: status chips on top, large live log, **Copy Diagnostics** top-right. Everything else is under **Manage**.

In Processor's page you can put `127.0.0.1:7798` in its Identity card; it should say *Found · unverified*.

## Checks worth doing at home
- Close Window 1 (Processor): Identity keeps running and logs `NOT RUNNING` within ~10 s. Start Processor again: back to `DETECTED`.
- Close Window 2 (Identity): Processor keeps running. Start Identity again: same System ID, previous log lines still there (a `— run … —` divider marks each start), GitHub signed out (memory-only) with "last verified" history.
- Press **Copy Diagnostics**, paste into a chat: no `#key=`, `#code=`, tokens, cookies or device codes should appear.

## Options
| Flag | Default | Meaning |
| --- | --- | --- |
| `-port` | `7798` (env `IDENTITY_B_PORT`) | loopback port; `0` = any free |
| `-data` | `~/.identity/home-claude-b` (env `IDENTITY_B_DATA`) | private profile folder |
| `-processor` | — | preset Processor address to watch |
| `-no-browser` | off | just print the link |
| `-test-github-web/-api` | — | **TEST FIXTURE ONLY** (labelled everywhere in the UI) |

## Stop and roll back
- Stop: Ctrl+C or close the window, or *Manage → Identity → Stop Identity*. Only Identity stops.
- Reset: delete `%USERPROFILE%\.identity\home-claude-b`. Nothing else is written outside that folder (and nothing outside the Processor profile by Processor).
- Roll back: this branch touches nothing on `alpha`, `stable`, `main`, Prototype A's branch or the shared instruction branch. Delete the clone folder; `git push origin --delete experiment/identity-home-claude-20261025` removes the branch if you don't want it.
