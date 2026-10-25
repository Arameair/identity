# Prototype A — handoff and evidence (2026-10-25)
**Status: YELLOW / unverified committed-source build.** Do not promote to Alpha.

## Repository scope
- New experiment **only**: `experiment/identity-home-gpt6-20261025`, based on Identity Alpha `8d08fec6629dc9cc35ea2266556572b86e5499d7`.
- Exact Processor reference, left unchanged: `experiment/processor-controller-launch-fix-20261024` at `0ba8bd52facedda1405897e9136d68ab184a2e32`.
- The prior reference branches, Alpha, Stable and Main were not changed.

## What is committed
One root Go app with original GitHub App device flow from a previous experiment, durable registry with archive/restore, protected operator session, public health, bounded JSONL activity, Copy Diagnostics sanitizer with canary tests, and read-only Processor public-health discovery. Private A profile defaults to `~/.identity/home-gpt6-a`; default port `7797`. Owner may choose another `-port` and `-data`. `marquee.json` is optional developer tooling, never required for independent execution.

No publisher backend, app entitlement, Engine approval or trusted Processor service authentication has been added.

## Test evidence and boundaries
- **Linux standalone development source in this session, separate local implementation:** Go 1.23.2/linux-amd64: `go test ./...` PASS; `go test -race ./...` PASS; `go vet ./...` PASS; Linux native build PASS; `GOOS=windows GOARCH=amd64 go build` produced an .exe (not executed on Windows).
- **Linux standalone local HTTP smoke, separate local implementation:** actual PID, loopback server `127.0.0.1:7797`, HTTP 200 public health, 401 unauthenticated private state, operator exchange, private state, diagnostics, Ctrl+C graceful shutdown and lock cleanup all observed. This is **not** proof of the different GitHub-committed source being runnable.
- **GitHub committed source:** pushed real source and a workflow. Workflow run `40178751360` failed within seconds with no steps or usable job log; new runs also immediately failed. **CI FAILED / cause not established.** Accordingly branch build/test/Windows cross-build are **NOT VERIFIED**, despite local implementation smoke.
- **Processor exact source:** inspected the README, root manifest, API shape, and read-only contract through GitHub connector. No clone, build, startup, PID or live Processor health response observed within this sandbox. **P0/P1 NOT TESTED.**
- **Two-process handshake:** code targets a candidate HTTP endpoint with `app=thought-processor` and protocol prefix `thought-processor.diagnostic/` (self-assertion only). Live integration with actual Processor **NOT TESTED**. Synthetic Go HTTP test exists but is not a live Processor.
- **Real Github device login:** **NOT TESTED**; requires public App Client ID, network and person authorizing at GitHub. User identity does not establish feature access.
- **Owner Windows:** **NOT TESTED**; a Windows cross-compile is not an observed Windows execution.

## Copy Diagnostics example — Linux local smoke, secrets omitted
```text
Identity Prototype A — Copy Diagnostics
UTC: 2026-10-25T08:52:18Z
Source: Arameair/identity branch experiment/identity-home-gpt6-20261025
Build SHA: UNKNOWN
OS: linux/amd64 Go: go1.23.2
PID: 8033 Port: 7797 Uptime: 3s
Identity: RUNNING (this local process)
GitHub: NOT_CONFIGURED login=UNVERIFIED
Entities active/total: 0/0
Processor: NOT_CONFIGURED
Authorized Engine operations: NOT IMPLEMENTED
Publisher grants/install: NOT IMPLEMENTED
Log writer error: NONE
2026-10-25T08:52:15Z identity started [run id withheld] ok
```
This example is from the **separate local build**, not from the published branch. It contains no actual credentials. The branch's own diagnostics format is defined in `ops.go`.

## Home trial
See `README_HOME_GPT6.md` for PowerShell commands. On the user's machine, first run the exact Processor source in its own data profile at `7860`; then run this Identity branch in another profile at `7797`. Set Identity Processor target to `127.0.0.1:7860`. Confirm public-health self-claim is **DISCOVERED_UNVERIFIED**, never `AUTHORIZED`. The Processor's own UI can independently probe Identity's `127.0.0.1:7797` address.

## Known blockers / rollback
- CI runner failed without step/log evidence. Don't interpret as successful compilation; run `go test ./...` and `go build` locally before trial.
- No verified publisher approval / service-auth / Engine grant contracts.
- A hard crash may leave the per-profile `process.lock`. Verify the PID in the lock is no longer running before removing the stale file. Never delete someone else's lock.
- Stop with Ctrl+C. Switching Git branches leaves `%USERPROFILE%\identity-home-gpt6-a` isolated and untouched. Previous Identity profiles are never imported or rewritten.
- Owner promotion decision remains **PENDING**.
