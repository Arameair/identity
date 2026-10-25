# Consolidation decisions — Prototype B (Claude), 2026-10-25

One new application written from `alpha` (8d08fec). Older branches were read, not copied in; nothing old is runnable from this branch.

| Source (branch @ SHA) | What was reusable | Decision |
| --- | --- | --- |
| `experiment/identity-standalone-access-trial-20261024` @ `6c2c30d` — `standalone/github.go`, `main.go` | GitHub App device flow; loopback UI with one-time key → cookie + CSRF; honest local draft "NOT SUBMITTED"; fixed 2-feature catalog | **Keep (adapted)**: device flow → `internal/github` (+ account/installations/repo-visibility read-back, event hooks, redaction registration, TEST FIXTURE label). Session model → `internal/app/session.go`. Drafts → `internal/features` (idempotent, tamper-checked). Its committed binary/CI were not trusted. |
| `experiment/identity-real-manager-20261023` @ `d060041` | durable System ID, profile lock, entity cards via `gh` CLI | **Keep idea, rewrite**: durable `idn_` System ID + per-profile OS lock in `internal/profile`. **Defer** entity cards (secondary S1) and **drop** the `gh` dependency. |
| `experiment/identity-setup-wizard-20261024` @ `81ee6b9` | step-by-step wizard UX; fixture-GitHub E2E approach | **Keep UX pattern** (5-step first-use flow) and fixture approach (`scripts/fixture_github.py`, Go `httptest` fixtures). **Drop** its Project Controller requirement. |
| `experiment/identity-controller-port-discovery-20261023` @ `ccfb894` | read-only loopback discovery concept | **Rewrite** as `internal/discovery`, targeting `Arameair/Thought-Processor` (not legacy Thought-Controller); rules mirror Processor's own `probe.go`. |
| Thought-Processor `experiment/processor-controller-launch-fix-20261024` @ `0ba8bd5` | the real Processor | **Used unchanged** as a separate process. No Processor source was modified; no new Processor branch was needed. |

Layout: `main.go` (flags) → `internal/app` (wiring, HTTP, UI, diagnostics) using `profile`, `logbook`, `redact`, `github`, `features`, `discovery`. One binary, one UI, one test suite, one data root.
