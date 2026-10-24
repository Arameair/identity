# Identity setup wizard (experiment)

Five steps from opening Identity to a running Thought Processor:

1. **Welcome** — what's being set up; one button: *Connect GitHub*
2. **Connect GitHub** — GitHub's official sign-in (short code, approve on github.com); shows the real username
3. **Find Thought Processor** — reads `Arameair/Thought-Processor` and the approved version `experiment/processor-first-prototype-20261024` from GitHub with that account
4. **Start Thought Processor** — asks Project Controller to start it; **you approve in Project Controller**; live progress and log
5. **Open Thought Processor** — checks it really answers, then opens it already signed in. Failures explain what happened and offer *Try again*.

No entities, Thought Engine, AI worker or fake states.

## Before the first run (owner, once)

- Register Identity's GitHub app: [docs/SETUP_GITHUB_APP.md](../docs/SETUP_GITHUB_APP.md) → `github-app.json` next to the program.
- Use the Project Controller build that accepts launch requests: `Arameair/Project-controller` branch `experiment/identity-launch-request-20261024`, running on its default address `http://127.0.0.1:7777`.

## Run

```bash
git clone -b experiment/identity-setup-wizard-20261024 https://github.com/Arameair/identity
cd identity/setup-wizard
go build -o identity .        # Windows: go build -o identity.exe .
./identity                    # opens your browser at the wizard
```

Running it again while it's open just brings its window back. Close it with Ctrl+C; Thought Processor and Project Controller keep running.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-controller` | `http://127.0.0.1:7777` | Project Controller address |
| `-port` | `7790` | Identity's own window |
| `-data` | `~/.identity-setup` | Lock and "already running" file only |
| `-github-app` | next to the program | `github-app.json` location |
| `-test-github-web`, `-test-github-api` | — | **Test only**: replace GitHub with a stand-in. Every screen then shows a TEST MODE banner. |

## Safety boundaries

- Identity **cannot approve** anything in Project Controller. It can ask Controller to download a branch and read that one request's progress; Controller's own "Review & allow" click is required to run code.
- The GitHub sign-in stays in memory; it is never written to disk or shown in the page.
- Thought Processor's one-time sign-in link is read from Controller's log for this run and handed to the browser only when you click *Open Thought Processor*; it is hidden in the log shown on screen.
- The window only answers on its own loopback address, from its own page, with a per-run link and anti-forgery token.
