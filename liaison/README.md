# Identity Liaison

Give TASS instructions **through GitHub issues**. The Liaison reads the md files you point at (from any of your repos), asks your **local Nemotron** what to do, and then either **answers in a comment** or **changes card files** and opens a pull request for you to merge.

It grew out of the nemotronTASS Lab 23 notebook (`notebooks/lab23_source.ipynb`). It reuses that notebook's context guard, truncation check and "never invent evidence" rules, now as an installable tool instead of notebook cells.

> **Experimental.** Branch `experiment/github-liaison-agent-20261103`, not promoted to `alpha`. See [the experiment report](../docs/LIAISON_AGENT_20261103.md) for what has and hasn't been tested.

## Install (WSL / Linux)

Needs Python 3.11+, your local llama.cpp model on `127.0.0.1:8081`, and GitHub CLI signed in (`gh auth login`).

```bash
curl -fsSL https://raw.githubusercontent.com/Arameair/identity/experiment/github-liaison-agent-20261103/liaison/install.sh | bash
identity-liaison check
```

Or by hand:

```bash
pipx install "git+https://github.com/Arameair/identity@experiment/github-liaison-agent-20261103#subdirectory=liaison"
identity-liaison init
identity-liaison check
```

`check` should print PASS for GitHub and the model. The config file is `~/.config/identity-liaison/config.toml`.

## Give it an instruction

Open an issue in **Arameair/identity** or **Arameair/nemotronTASS** with the label **`liaison`**:

```text
read:
- Arameair/nemotronTASS@experiment/tass-foundation-notebook-20261103:cards/*.md
- Arameair/identity@teams:teams/TEAM_OPERATING_MODEL.md

change:
- Arameair/identity@teams:teams/TEAM_LIAISON.md

task:
Add a rule: when a worker reports DONE without a commit or test result,
the Liaison asks for the evidence instead of escalating to the Project Lead.
```

- **`read:`** md files to use as reference. Use `owner/repo@branch:path`; leave out `@branch` for the default branch. Patterns such as `teams/*.md` work.
- **`change:`** cards to edit or create, as exact paths. Leave this section out to get an **answer as a comment** instead.
- **`task:`** what you want, in plain words. If there is no body, the issue title is the task.

You can also open an issue from the terminal:

```bash
identity-liaison new "Add the evidence rule" \
  --read "Arameair/identity@teams:teams/*.md" \
  --change "Arameair/identity@teams:teams/TEAM_LIAISON.md"
```

## Run it

```bash
identity-liaison once --dry-run   # show what it would do; writes nothing
identity-liaison once             # handle waiting issues once
identity-liaison run              # keep going; checks every 5 minutes (the "pulse")
identity-liaison ask "What does TASS want?" --read "Arameair/nemotronTASS:README.md"   # local preview, no issue
```

Each issue moves through labels that follow the Core Team protocol:

| Label | Meaning |
| --- | --- |
| `liaison` | New instruction, waiting |
| `liaison:working` | Picked up (a **WORKING** comment is posted) |
| `liaison:done` | Finished. The comment has the answer, or a link to the pull request |
| `liaison:blocked` | Could not finish. The comment says why. Remove this label to retry |

## Safety rules (built in)

- **Only your issues count.** Issues opened by anyone not in `[github] owners` are ignored, because the repos are public.
- **Allow-lists.** It reads only repos in `[read] allow` and changes only paths in `[write] allow`. The defaults allow `teams/`, `team/`, `cards/` and `docs/team/` md files in Arameair repos.
- **You merge.** The default `mode = "pr"` puts changes on a `liaison/...` branch and opens a pull request. `mode = "direct"` commits straight to the branch, but never to `main`, `alpha` or `stable`.
- **No half cards.** Nothing is written if the model ran out of room (truncated), if the context guard says the prompt won't fit, or if the new card is less than half the old size. Say "shorten" or "remove" in the task if you meant to cut it down.
- Files it reads are treated as information, never as instructions.
- The model's text is not evidence. PRs and comments say so.

## Tests

```bash
cd liaison && python3 -m unittest discover -s tests -v
```

The tests use a fake GitHub and a fake model. Passing them does **not** prove the real model's behavior.
