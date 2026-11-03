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
| Real Nemotron 3 Nano 4B produces a usable card in the `<<<FILE` format | Actual | Owner WSL | — | **NOT RUN** |
| Real issue → real PR on GitHub | Actual | Owner WSL | — | **NOT RUN** |
| `install.sh` on owner WSL | Actual | Owner WSL | — | **NOT RUN** |

Fixture results do not prove that the real model follows the reply format or writes good cards.

## One owner acceptance test

1. `curl -fsSL https://raw.githubusercontent.com/Arameair/identity/experiment/github-liaison-agent-20261103/liaison/install.sh | bash`, then `identity-liaison check`. Both lines should say PASS.
2. With the model running, open an issue in Arameair/identity labelled `liaison`. Use the example in `liaison/README.md` (change `teams/TEAM_LIAISON.md` on the `teams` branch).
3. Run `identity-liaison once --dry-run`. A diff should print and GitHub should be unchanged.
4. Run `identity-liaison once`. The issue should get WORKING, then DONE with a PR link, and the PR diff should match the request. The `teams` branch must be unchanged until you merge.
5. Record PASS / FAIL / NOT RUN and the actual output below.

## Failures and unknowns

- A 4B model may not follow the `<<<FILE … FILE>>>` format. The parser also accepts a fenced markdown block. If neither is present, the issue is BLOCKED and nothing is written.
- If a PR-mode run fails partway, a `liaison/...` branch may be left without a PR. It is safe to delete.
- **Rollback:** `pipx uninstall identity-liaison`, delete `~/.config/identity-liaison`, close/delete `liaison/...` branches. Alpha is unaffected.

## Owner review

- Decision: **PENDING** (accept / revise / reject).
- Promotion: **None unless the owner explicitly approves.**
