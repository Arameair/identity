# Identity experiment — GitHub-instructed Liaison that updates card files

**Date:** 2026-11-03
**Owner request and scope:** Turn the nemotronTASS Lab 23 notebook into something in the identity repo that (1) installs, (2) takes instructions through GitHub, (3) reads md files from repos, and (4) changes card files in GitHub repos.
**Branch:** `experiment/github-liaison-agent-20261103`
**Exact starting Alpha SHA:** `a582f8ea595eb12145c4116e67e30de46cb470c8`
**Exact experimental head SHA:** see the branch head / pull request

## Hypothesis and expected behavior

The owner opens a GitHub issue labelled `liaison` that lists the md files to read, the cards to change and a plain-language task. A locally installed `identity-liaison` (Python, standard library only) notices the issue, posts **WORKING**, reads the files and asks the local Nemotron (llama.cpp `127.0.0.1:8081`) through the Lab 23 context guard. It then either answers in a comment or opens a pull request with the changed cards, and finishes with **DONE**. Anything else ends in **BLOCKED** with the reason.

The Go Identity service is **not touched**. The Liaison lives in `liaison/` and runs as its own process.

## Boundaries and changes

- **Code/files added:** `liaison/` (package, install script, tests, source notebook with outputs stripped), `.github/workflows/liaison-tests.yml`, this report. No existing file changed.
- **Data paths:** config only, at `~/.config/identity-liaison/config.toml`. No local state. Issue labels are the state.
- **Other services contacted:** GitHub REST API (issues, contents, git refs, pulls, labels); the local OpenAI-compatible model server.
- **Credentials:** `gh auth token`, or `GITHUB_TOKEN`/`GH_TOKEN`. Never logged, commented or sent to the model.
- **Writes and side effects:** issue comments and labels in the instruction repos. Card files are written only on a new `liaison/<repo>-<issue>-<time>` branch plus a PR (default). `direct` mode commits to the named branch, except `main`/`alpha`/`stable`.
- **Excluded:** Identity grant enforcement (P1 is unpromoted, so the allow-lists in the config stand in for it for now), auto-merge, running code, nudging workers, Docker packaging.

## Verification results

| Test | Actual vs fixture | OS/environment | Observed evidence | Verdict |
| --- | --- | --- | --- | --- |
| Offline suite (16 tests): issue→PR, answer-only, non-owner ignored, read/write allow-lists, protected branch, truncation, context guard, shrunken card, new card, dry run | **Fixture** GitHub + **fixture** model | Linux container, Python 3.13 | `Ran 16 tests … OK` | PASS (fixture only) |
| `pip install ./liaison` → `identity-liaison --help`, `init` | Actual | Linux container venv | CLI installed; config written | PASS |
| `identity-liaison check` against real GitHub | **Actual GitHub**, fixture model | Linux container | Signed in as Arameair; identity and nemotronTASS reachable | PASS |
| `identity-liaison read Arameair/identity@teams:teams/TEAM_LIAISON.md` | **Actual GitHub read** | Linux container | Printed the real Team Liaison charter | PASS |
| `ask` preview: glob read of nemotronTASS `cards/*.md` on the foundation branch + change TEAM_LIAISON.md | Actual reads, **fixture model**, no writes | Linux container | Diff shown; nothing written | PASS (model is fixture) |
| Docker image build + `--version` | Actual image; **stand-in base** (`python:3.12-alpine` and ghcr are blocked from the build sandbox, so a local Python 3.13 rootfs was used via `BASE=`) | Linux container, Docker 29.9 | Built; runs as UID 1000, read-only FS | PASS (stand-in base) |
| `compose.yaml` wiring: Liaison → `model` service by name, healthcheck gate, `init` writes `/config/config.toml` | Fixture model in place of llama.cpp image | Docker Compose 5.6 | `PASS model at http://model:8081` | PASS (fixture model) |
| `compose.external-model.yaml` in container against **real GitHub** | Actual GitHub (no `liaison` issues existed, so nothing written), fixture model | Docker, host network | `check` all PASS; `once --dry-run` handled 0; `run` pulsed every 5s | PASS |
| Missing/empty token in container | Actual | Docker | Logs `pulse failed: … is empty` each pulse; container stays Up (no restart loop) | PASS |
| `./docker-run.sh` end to end with real `python:3.12-alpine` + `llama.cpp:server-cuda` images | Actual | Owner WSL (DESKTOP-HEK2HFN), Docker via sudo | Owner console 2026-11-03 07:56: image built, model container Healthy (32.5s), `check`: GitHub signed in as Arameair, both instruction repos reachable, `model at http://model:8081: context 8192 tokens`; Liaison running, pulse 300s | PASS |
| Real issue → answer (issue #24) | **Actual** GitHub + real Nemotron 3 Nano 4B | Owner WSL, Docker | WORKING 14:10:06Z → DONE 14:11:04Z; 1149/8192 prompt tokens; 3-bullet answer grounded in TEAM_LIAISON.md | PASS |
| Real issue → card PR (issue #25 → PR #26) | **Actual** GitHub + real Nemotron | Owner WSL, Docker | Rule added, nothing dropped, `teams` untouched; BUT model duplicated the last paragraph and echoed `=== end of current content ===` | FAIL → fixed in code (marker strip + duplicate-paragraph removal, prompt tightened); regression test uses the real PR #26 output; cleaned diff = rule only. PR #26 closed by owner. |
| Re-run of issue #25 after cleanup fix | **Actual** GitHub + real Nemotron | Owner WSL, Docker | BLOCKED 14:28:08Z: output hit the ~6.7k-token limit on a ~1.3k-token card (repetition loop); nothing written | FAIL (safety check worked) → switched existing-card changes to **edit mode** (model returns FIND/ACTION/TEXT; Liaison applies it), temperature 0.3, reply cap 768 tokens, partial reply logged. |
| Re-run of issue #25 in edit mode | **Actual** GitHub + real Nemotron | Owner WSL, Docker | BLOCKED 14:37:24Z: reply had no FIND/ACTION/TEXT block the parser recognized; nothing written | FAIL (safety check worked) → parser now accepts bold labels, quoted/backticked FIND lines, fenced or unfenced TEXT. Raw reply (owner log): text closed with bare `>>>`; `replace` repeated the original line twice around the new rule |
| Issue #25 after tolerant parser → PR #27 | **Actual** GitHub + real Nemotron | Owner WSL, Docker | WORKING 14:48:47Z → DONE 14:48:56Z (9 s). Rule placed under Mission, nothing lost; BUT `>>>>` closer not recognized, so `<<<TEXT` / `>>>>` lines leaked into the card | FAIL → closers `>>>`/`>>>>`/`TEXT>>>` accepted, marker lines stripped, repeated lines dropped, retyped FIND line keeps the card's original formatting, final guard blocks any leftover `<<<`/`>>>`/`FIND:` line. Both real replies are regression tests. PR #27 not to be merged |
| Side chat, Developmnt #10 | **Actual** GitHub + real Nemotron | Owner WSL, Docker | Replied 14:48:42Z: accurate 4-bullet platform summary from both `side-chat/` files, follow-up question; 1255/8192 tokens | PASS |
| Side chat dry run on real Developmnt #10 | Actual GitHub reads, stand-in model, nothing posted | Linux container | Found #10, read `side-chat/README.md` and `side-chat/development-platform.md`, built 1-message conversation | PASS (model is stand-in) |
| `install.sh` on owner WSL | Actual | Owner WSL | — | NOT RUN (Docker used instead) |

Fixture results do not prove that the real model follows the reply format or writes good cards.

## One owner acceptance test

1. `git clone -b experiment/github-liaison-agent-20261103 https://github.com/Arameair/identity.git ~/identity-liaison && cd ~/identity-liaison/liaison && ./docker-run.sh check`. GitHub and the model should both say PASS. (Use `--external` if your TASS stack's model is already running.) Without Docker: `install.sh`, then `identity-liaison check`.
2. Open an issue in Arameair/identity labelled `liaison`. Use the example in `liaison/README.md` (change `teams/TEAM_LIAISON.md` on the `teams` branch).
3. Run `./docker-run.sh dry-run` (or `identity-liaison once --dry-run`). A diff should print and GitHub should be unchanged.
4. Run `./docker-run.sh` and watch `./docker-run.sh logs` (or `identity-liaison once`). The issue should get WORKING, then DONE with a PR link, and the PR diff should match the request. The `teams` branch must be unchanged until you merge.
5. Record PASS / FAIL / NOT RUN and the actual output below.

## Failures and unknowns

- A 4B model may not follow the `<<<FILE … FILE>>>` format. The parser also accepts a fenced markdown block. If neither is present, the issue is BLOCKED and nothing is written.
- If a PR-mode run fails partway, a `liaison/...` branch may be left without a PR. It is safe to delete.
- **Rollback:** `pipx uninstall identity-liaison`, delete `~/.config/identity-liaison`, close/delete `liaison/...` branches. Alpha is unaffected.

## Owner review

- Decision: **PENDING** (accept / revise / reject).
- Promotion: **None unless the owner explicitly approves.**
