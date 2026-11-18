# Identity PR #47: audit repair report and Codex handoff (2026-11-18)

**For:** the Identity team, the Codex auditor, and Thought-Processor #127 (Discord Milestone A).
**Status:** repaired, **not independently approved**. Do not merge, deploy, or enable `discord.reply`
until a Codex audit of the repaired head passes.

| | SHA |
| --- | --- |
| Base (PR #45 head, `experiment/stage3b-experiment-run-20261112`) | `ee7ba1b0f9cf894235be63071c493b74e2a0eef9` |
| Audited head (GPT-6, NEEDS REPAIR, [comment 6914380703](https://github.com/Arameair/identity/pull/47#issuecomment-6914380703)) | `9c36f6e43da5668f1392726da9e7d99108e625e0` |
| Repair commit (code, tests, contract) | `9394a0a31fd138780c98d22469505322840919ed` |
| Audit target | the PR #47 head that contains this file (the repair commit plus this report; the report changes no code) |

The branch was unchanged between the audit and the repair: its head was still `9c36f6e`.

## Findings and what was done

| # | Audit finding | Repair |
| --- | --- | --- |
| 1 | MEDIUM: after a successful redemption, `check:true` still answers `allow:true` for `discord.reply`, and the check ignores expiry (+24 h) | **Fixed.** `entity.SinglePointOfUse = {discord.reply}`. `RedeemBound` refuses a check of it for every service, before or after the redeem and after the 120 s window. The HTTP handler reaches decisions only through `RedeemBound`, so both paths are covered. The Engine's redeem-then-check flow is unchanged. Reproduced on `9c36f6e` before the fix: 5 of 5 checks answered allow. |
| 2 | MEDIUM: no deterministic concurrent-redemption proof | **Added.** Barrier-start, 48 goroutines, 25 rounds, against the real `Store` and the real HTTP handler, for `discord.reply` and `cognition.nudge.process`. Exactly one wins each round, and `decisions.jsonl` shows 1 allowed and 48 refused per round. Also a mixed redeem/check race. Clean under `-race`. |
| 3 | LOW: binding fields and transmission duties are door-enforced | **Documented** in `docs/DISCORD_REPLY_CONTRACT.md`, under "What the door must do": the eight door-v2 acceptance requirements. Door-v2 tests belong in the Communication Bridge. |
| 4 | LOW: failed audit writes are ignored; duplicate key values are accepted | **Duplicate keys fixed:** `loadKeys` refuses a key on more than one line, and the error never prints the key. **Audit write:** documented as an inherited limitation that applies to every bound action and is unchanged here, because failing closed would change Engine behaviour. |

## Files changed by the repair

- `internal/entity/bound.go`: `SinglePointOfUse` and the check refusal in `RedeemBound`
- `cmd/tass-identity/main.go`: `loadKeys` refuses a duplicate key
- `cmd/tass-identity/discord_reply_repair_test.go`: new regression tests
- `docs/DISCORD_REPLY_CONTRACT.md`: single point of use, door obligations, known limitations

Not changed: `internal/entity/skills.go` and `entity.go` (grants, default grants), `page.html`, Core,
Engine, Communication Bridge.

## Test evidence (local, Go 1.24.7 linux/amd64, on `9394a0a`)

| Check | Result |
| --- | --- |
| `go vet ./cmd/tass-identity ./internal/github ./internal/entity` | PASS |
| `go test -race -count=1 ./cmd/tass-identity ./internal/github ./internal/entity` | PASS |
| `go build ./...` | PASS |
| `go test -count=1 ./...` | PASS (13 of 13 packages with tests) |
| New: `TestDiscordReplyCheckBypassIsClosed`, `TestDiscordReplyCheckRefusedAfterTheOriginalWindow`, `TestDiscordReplyNegativeRedemptions` (wrong service ×2, wrong action ×2, mismatched binding, empty binding, unknown id, expired, revoked, locked, reused), `TestEngineRedeemThenCheckStillWorks`, `TestConcurrentRedeemSucceedsExactlyOnce`, `TestConcurrentRedeemAndCheckOnDiscordReply`, `TestLoadKeysRefusesASharedKey` | PASS |
| Existing Discord and bound-decision tests | PASS |
| Mutation: single-point-of-use guard removed | bypass, after-window and mixed-race tests FAIL |
| Mutation: already-used guard removed | concurrency test FAIL (48/48 won) |
| Mutation: `RedeemBound` mutex removed | concurrency test FAIL (2/48 won), plus `WARNING: DATA RACE` |
| Mutation: duplicate-key guard removed | `TestLoadKeysRefusesASharedKey` FAIL |
| GitHub Actions on the repaired head | see the PR checks (the audit notes CI runs on the synthetic merge, not the head) |
| Real Discord, door v2, Core integration | NOT RUN (out of scope) |

## Known limitations (inherited, outside this repair)

- A failed `decisions.jsonl` write does not block a decision (all bound actions).
- Service names in the key file aren't checked against a list. A misspelled `discord_door`
  loads but can redeem nothing.
- Identity can't tell whether the door really recomputes the binding.
- Engine checks of a redeemed decision don't enforce expiry (the existing 24 h design).
- Revocation racing a simultaneous redemption isn't separately proven: each redemption
  re-checks state under the table lock, but a revoke isn't taken under that same lock.

## Remaining integration dependencies

- Core mints `discord.reply` with the contract binding (Thought-Processor #127).
- The Communication Bridge / Discord door v2 implements the contract's door requirements and tests.
- The owner provisions a separate `discord-door` key. This repair creates no credentials.
- Merge sequencing keeps PR #45's accepted pin. PR #47's stacked CI isn't a test of a later merge into main.

## Codex audit handoff

```
Codex: independent security audit of Arameair/identity PR #47.
Target: the current PR #47 head (repair commit 9394a0a + docs/PR47_REPAIR_REPORT_20261118.md).
Base ee7ba1b; prior audited head 9c36f6e; prior audit: PR #47 comment 6914380703 (NEEDS REPAIR).
Verify each of the four prior findings against the code:
1. discord.reply never gets allow=true via check:true: before or after redeem, after 120 s, any service
   (internal/entity/bound.go SinglePointOfUse / RedeemBound).
2. Only discord-door redeems discord.reply; each redemption consumes once; wrong service, action or
   binding, unknown, expired, revoked, locked and reused are refused; refusals don't consume.
3. The Engine's redeem-then-check for cognition.nudge.process, cognition.nudge.stop and
   cognition.experiment.run is unchanged.
4. The concurrency tests use the real Store and HTTP handler, and would fail without the mutex or
   the used-flag.
5. loadKeys refuses a duplicate key without printing it.
6. No grant, default-grant, Core, Engine or Bridge change; the contract's door obligations and
   limitations are accurate.
Run on the exact head: go vet ./cmd/tass-identity ./internal/github ./internal/entity;
go test -race -count=1 ./cmd/tass-identity ./internal/github ./internal/entity; go build ./...; go test -count=1 ./...
Return PASS / NEEDS REPAIR with findings. No merge, deploy, credential, grant, or Discord action.
```
