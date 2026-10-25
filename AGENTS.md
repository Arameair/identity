# AGENTS.md — runbook for AI operators (e.g. a Grok bot)

Goal: on the owner's **Windows** computer, get **Thought Processor** running, **Identity** running, GitHub sign-in completed **by the human**, and the two programs seeing each other read-only. Then report with Copy Diagnostics.

Both repositories are public. Nothing here needs a GitHub token on the command line.

| Program | Repo | Branch | Port | Profile folder |
| --- | --- | --- | --- | --- |
| Thought Processor | `Arameair/Thought-Processor` | `alpha-candidate-20261025` | **7860** | `%USERPROFILE%\tp-alpha` |
| Identity | `Arameair/identity` | `experiment/identity-gpt6-alpha-candidate-20261025` | **7790** | `%USERPROFILE%\.identity\default` |

## Rules — read first

1. **Never paste, log, or send** anything after `#key=` (Identity's link) or `#code=` (Processor's link), GitHub device codes, tokens, or cookies, in chat, issues, commits or reports. Open those links locally only.
2. **The human approves GitHub sign-in.** You may click *Connect GitHub* and open the GitHub page. Entering the code and pressing **Authorize** is the owner's action, unless the owner explicitly tells you to do it in a browser already signed in as them.
3. Each program runs in **its own** terminal window. Stopping one must never stop the other. Don't kill processes you didn't start.
4. Don't edit `alpha`, `main` or anyone's experiment branches. Report problems; don't "fix" by changing code unless asked.
5. Never claim a step passed unless you saw the expected result below.

## Step 0 — prerequisites

```powershell
git --version     # any recent version
go version        # must be go1.22 or newer
```
If Go is missing: install it from https://go.dev/dl/ (Windows .msi), then open a **new** PowerShell window.

## Step 1 — Thought Processor (terminal window 1)

```powershell
cd $HOME
git clone --branch alpha-candidate-20261025 https://github.com/Arameair/Thought-Processor.git
cd Thought-Processor\processor
go test ./...
go build -o thought-processor.exe .
.\thought-processor.exe -port 7860 -data "$env:USERPROFILE\tp-alpha"
```
**Expected:** `go test` prints `ok`; the program prints `Thought Processor 0.1.0-experiment (0ba8bd52face)`, a `run … pid …` line, and an `open : http://127.0.0.1:7860/#code=…` line (private, rule 1). Leave this window open.

Check from another window:
```powershell
Invoke-RestMethod http://127.0.0.1:7860/api/health
```
**Expected:** `app = thought-processor`, `protocol = thought-processor.diagnostic/0-experimental`.

## Step 2 — Identity (terminal window 2)

```powershell
cd $HOME
git clone --branch experiment/identity-gpt6-alpha-candidate-20261025 https://github.com/Arameair/identity.git identity-gpt6-trial
cd identity-gpt6-trial
.\scripts\build-windows.ps1
.\identity.exe -processor 127.0.0.1:7860
```
If PowerShell blocks the script ("running scripts is disabled"), run the same steps by hand: `go vet ./...`, `go test ./...`, `go build -ldflags "-X main.buildBranch=alpha-candidate-20261025" -o identity.exe .`

**Expected console lines:**
- `Identity 0.3.0-alpha  branch experiment/identity-gpt6-alpha-candidate-20261025  commit <sha>`
- `github  : built into this build (Arameair's GitHub App)`. If it says *no publisher GitHub App built in*, stop: wrong branch or old checkout.
- `run … pid … port 7790 (loopback only)`
- `open : http://127.0.0.1:7790/#key=…` (private, rule 1). A browser window opens automatically.

Check: `Invoke-RestMethod http://127.0.0.1:7790/api/health` → `app = identity`, `version = 0.3.0-alpha`.

If an old `IDENTITY_GITHUB_CLIENT_ID` environment variable is set in that window, Identity treats it as a *developer override*. That's harmless, but for a clean test run `Remove-Item Env:IDENTITY_GITHUB_CLIENT_ID` first.

## Step 3 — first-use setup in the Identity window

1. **Welcome** → *Begin*.
2. **GitHub account** → *Connect GitHub*. A GitHub tab opens and the code is copied. **Hand over to the owner** (rule 2): paste the code, *Continue*, *Authorize*.
   - Expect Identity to show the account after a successful poll; timing depends on GitHub's interval and any `slow_down` responses. Check GitHub for authorization problems if Identity remains pending.
   - While waiting, the screen shows *Last checked with GitHub … GitHub says …*.
   - Use **only** the newest tab, and don't press *Cancel* in between: every new start issues a new code.
3. **Features** → Thought Processor and Thought Engine show *Availability / Request / Grant*. Grant always says **NOT GRANTED** (correct; no grant service exists yet). Saving a draft is optional and stays **NOT SUBMITTED**.
4. **Thought Processor** → the address `127.0.0.1:7860` is filled in → *Use this address*. **Expected:** `DETECTED — SELF-REPORTED, UNVERIFIED` and an *Open Thought Processor* button.
5. **Done** → *Finish setup*. Identity switches to the monitoring view.

## Step 4 — Processor sees Identity

Open Processor's own page with the `#code=` link from window 1 (locally). In its **Identity** card enter `127.0.0.1:7790` → *Save*. **Expected:** `Found · unverified`, "it CLAIMS to be Identity".

## Step 5 — independence check

- Press Ctrl+C in window 1 (Processor). Identity keeps running; within ~10 s its log shows `processor.state_changed … NOT RUNNING`. Start Processor again with the same command: Identity shows `DETECTED` again.
- Press Ctrl+C in window 2 (Identity). Processor keeps running (`Invoke-RestMethod http://127.0.0.1:7860/api/health` still answers). Start Identity again: same System ID, old log lines still there, GitHub signed out (by design, sign in again).

## Step 6 — report

In Identity press **Copy Diagnostics** and paste it, plus Processor's *Copy Context*, into the report. Both are built to remove codes and tokens; still glance over them before sending. Report each step as **PASS / FAIL / NOT RUN** with what you actually saw.

## Troubleshooting

| You see | Meaning / fix |
| --- | --- |
| `cannot listen on 127.0.0.1:7790` | Something already uses the port (often an older Identity). Close it, or add `-port 7791`. |
| `profile in use` | Another Identity already runs with that profile. Use it, or close it first. |
| GitHub chip says *NO GITHUB APP IN THIS BUILD* | Wrong branch/old build. `git log -1` must show the alpha-candidate commit; rebuild. |
| GitHub chip shows *your own GitHub App* | This profile saved a tester's own app earlier. *Manage → Account → Stop using it* returns to the built-in app. |
| GitHub "all set" but Identity still checking | Allow for the next GitHub poll; check that the newest code was approved. Inspect the GitHub tab for account restrictions or refusals. Identity cannot read GitHub's page banners. Copy Diagnostics shows its latest actual poll result. |
| `Can't get an answer from GitHub … attempt N` | Network/proxy/firewall blocking `github.com` from Identity. Report the diagnostics. |
| *GitHub sign-in isn't switched on for this app* | Owner: enable **Device Flow** in the GitHub App settings. |
| *GitHub doesn't recognise this app's Client ID* | Wrong/old build, or the app was deleted. Report it. |
| Processor shows `WRONG SERVICE` / `NOT RUNNING` in Identity | Check the address and that window 1 is still running. |
| Identity page says "This window isn't signed in" | Identity restarted; open the **new** `#key=` link from window 2. |

## Not in scope here (don't try to do these)

Installing or starting the Thought Engine, Identity-granted permissions, Processor start/attach/stop of the Engine. These are design topics; see Thought-Processor's `docs/INSTALL_AUTHORITY_DISCUSSION_2026-10-25.md`.
