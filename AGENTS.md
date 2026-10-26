# AGENTS.md — Identity alpha candidate 2026-10-26 runbook (owner + AI operators, e.g. a Grok bot)

**Goal:** on the owner's **Windows** computer, install this Identity candidate, prove it works as a **background service** that a program pairs with, and report PASS/FAIL with evidence.

Thought Processor does not speak this API yet; the Processor team is building that now. Until then, **`identity-probe.exe` plays Processor's part**. It is a test client with no UI.

| | |
| --- | --- |
| Repo / branch | `Arameair/identity` · `alpha-candidate-20261026` (public, no token needed) |
| Port | **7790** (the self-test uses 7891, so it doesn't clash) |
| Profile for this trial | `%USERPROFILE%\.identity\alpha-20261026` (fresh, so old trials don't mix in) |

## Rules — read first

1. **Never paste, log or send** any of these into chat, issues, commits or reports:
   - the 8-character **pairing code** (`ABCD-EFGH`)
   - the GitHub device code
   - anything after `#key=`
   - the contents of `.identity-probe\pairing.json`
   - tokens or cookies

   The tools print none of these except where a person must read them.
2. **A human approves GitHub sign-in** on github.com. The AI may start it and open the page. The owner enters the code and presses **Authorize**, unless the owner explicitly tells the AI to do it.
3. Identity runs in **its own window** (window 1). Run commands in window 2. Don't kill processes you didn't start.
4. Don't edit `alpha`, `main` or other branches. Report problems; don't "fix" code unless asked.
5. A step passes only if you saw the **Expected** result. Otherwise it's FAIL or NOT RUN.

## Step 0 — prerequisites

```powershell
git --version
go version        # go1.22 or newer; else install from https://go.dev/dl/ and open a NEW PowerShell
```
Close any older Identity window first: an old one on port 7790 makes Step 3 fail with `cannot listen`.

## Step 1 — get and build (window 2)

```powershell
cd $HOME
git clone --branch alpha-candidate-20261026 https://github.com/Arameair/identity.git identity-1026
cd identity-1026
git log -1 --format="%h %s"
.\scripts\build-windows.ps1
```
If PowerShell blocks scripts, run the same steps by hand:
```powershell
go vet ./... ; go test ./...
go build -ldflags "-X main.buildBranch=alpha-candidate-20261026" -o identity.exe .
go build -o identity-probe.exe ./cmd/identity-probe
```
**Expected:** every `go test` line says `ok`, and you end up with `identity.exe` and `identity-probe.exe`.

## Step 2 — automatic self-test (window 2, about 30 seconds)

```powershell
.\identity-probe.exe smoke
```
This starts real `identity.exe` processes on throwaway profiles and runs 11 scenarios:

- background mode
- pairing
- default-deny
- clean shutdown
- restart and reconnect
- Processor-first start
- Identity down (fail closed)
- wrong installation
- duplicate instance
- unpair
- restart after a crash

**Expected:** `SMOKE RESULT: PASS` (the 11 scenarios are checked with 19 PASS lines). The last line gives the path of `smoke-report.txt`; **attach only that file**. It contains no codes or tokens. The other files in that folder are working files; private links and codes in them are masked, but don't attach them.

## Step 3 — start Identity for real (window 1)

```powershell
cd $HOME\identity-1026
.\identity.exe -data "$env:USERPROFILE\.identity\alpha-20261026"
```
**Expected console:**
- `Identity 0.4.0-alpha  branch alpha-candidate-20261026  commit <first 12 characters of the Step 1 commit>`
- `Identity is running in the background. Use it through Thought Processor —`
- `github  : built into this build (Arameair's GitHub App)`. If it says *no publisher GitHub App built in*, you have the wrong build: stop.
- `locator : …\AppData\Local\Arameair\Identity\instances\idn_….json` (how Processor finds this installation)
- `paired  : none yet — … a code will appear HERE`
- **No browser opens.** The `troubleshooting page` link is private (rule 1) and not needed.

Leave window 1 open.

## Step 4 — pair, as Processor will (window 2)

```powershell
.\identity-probe.exe pair
```
**Expected:**
1. Window 1 shows a `PAIRING REQUEST` box with a code.
2. Window 2 asks you to type the code. Type it from window 1 (rule 1: don't paste it anywhere else).
3. Window 2 prints `PAIRED as probe_… with Identity installation idn_…`.

## Step 5 — create an identity and check (window 2)

```powershell
.\identity-probe.exe create -name "Tass" -kind companion
.\identity-probe.exe check
```
**Expected:**
- `CREATED ent_… Tass (companion) — no access granted`.
- `check` prints `PASS` lines and ends with `RESULT: PASS`. In particular, `authorize defaults to DENIED` must be PASS, because identities start with no access.

## Step 6 — GitHub sign-in through the API (window 2 + owner)

```powershell
.\identity-probe.exe github
```
It prints a GitHub link and a code. **Owner:** open the link, enter the code, press **Authorize**.

**Expected:**
- `SIGNED IN as <login>` within about 15 seconds of GitHub's "all set" page.
- Then `.\identity-probe.exe check` shows `thought-engine eligible_to_install=ALLOWED` (your account can see Arameair/Thought-engine) or `NOT_ALLOWED`. Either is a valid result; record which.
- Eligibility means **install only**. `authorize` must still say DENIED.

## Step 7 — restart and reconnect (C05)

1. Window 1: press **Ctrl+C**. Expected: `Identity stopped.`
2. Start it again with the Step 3 command. Expected: `paired  : 1 Thought Processor(s) — they reconnect on their own`.
3. Window 2: `.\identity-probe.exe check`. Expected: `RESULT: PASS`, with **no new code** and the same `idn_…` and `Tass`.
4. GitHub shows `SIGNED_OUT` again. That is by design: the token is kept in memory only.

## Step 8 — Processor-first start (C01)

1. Window 1: press **Ctrl+C**.
2. Window 2: run `.\identity-probe.exe check -wait 2m`. It prints `waiting for Identity … AUTHORITY UNAVAILABLE`.
3. Within 2 minutes, start Identity again in window 1.

**Expected:** window 2 connects by itself and ends `RESULT: PASS`.

## Step 9 — Identity down means fail closed (C06)

1. Window 1: press **Ctrl+C**.
2. Window 2: run `.\identity-probe.exe check`.

**Expected:** `FAIL Identity reachable (expected when Identity is stopped) AUTHORITY UNAVAILABLE …` with exit code 2. This FAIL line is the **correct** behaviour: report Step 9 as PASS when you see it. Then start Identity again in window 1.

## Step 10 — shutdown as Processor's "Shut down" choice (C04)

```powershell
.\identity-probe.exe shutdown
```
**Expected:**
- Window 2 shows `PASS Identity stopped (port closed)`.
- Window 1 shows `Identity stopped.`
- The file `%USERPROFILE%\.identity\alpha-20261026\runtime\service.json` is gone.

## Step 11 — report

Report each step as **PASS / FAIL / NOT RUN**, with what you actually saw. Include:
- the commit from Step 1 (`git log -1`)
- `smoke-report.txt` from Step 2
- the `check` output from Steps 5–8

Optional, for deeper troubleshooting: open the troubleshooting link from window 1 **locally**, press **Copy Diagnostics**, and attach the result (secrets are removed; glance over it anyway).

## Troubleshooting

| You see | Meaning / fix |
| --- | --- |
| `cannot listen on 127.0.0.1:7790` | An older Identity (or something else) is using the port. Close it, or start Identity with `-port 7792` and run the probe as `.\identity-probe.exe -url http://127.0.0.1:7792 check` (same for other commands). |
| `profile in use` | Another Identity already uses that `-data` folder. Close it first. |
| `smoke` says `something already answers on port 7891` | Run `.\identity-probe.exe smoke -port 7893`. |
| `pair` says `pairing failed: … wrong or expired` | Run `pair` again and use the **newest** code in window 1. After 5 wrong codes, wait 1 minute. |
| `check` says `token refused` | This Identity doesn't know the probe: you unpaired, or used a different `-data` folder. Run `pair` again. |
| `check` says `NOT signed by the paired Identity` / `DIFFERENT Identity installation` | Something other than the paired Identity answered. Report it; don't continue. |
| GitHub stays `WAITING` | Approve the **newest** code. Network problems show as `github_said=…` in the next `check`. |
| *GitHub sign-in isn't switched on for this app* | Owner: enable **Device Flow** in the GitHub App settings. |
| Window 1 shows a pairing box you didn't ask for | Another local program asked to pair. Ignore it; the code expires in 5 minutes. |

## Not in scope (don't try)

- Granting permissions (none exist yet).
- Starting or stopping Thought Engine.
- Thought Processor's UI for this API (the Processor team's work).
- The older Engine install demo, which lives on `experiment/engine-access-pairing-20261026`.
