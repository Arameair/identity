# Identity policies

> **PROTOTYPE.** These are static files, not live Identity grants. Identity's own `authorize` API is not connected yet, so a policy is only as strong as the review of changes to it. Each file is enforced by TASS's Go core, and by the Thought Engine before model work, but nothing here is signed, scoped per request or revocable faster than the next sync.

Each file here says what one identity may do. The running core reads its policy before every action and
keeps the last good copy locally, so permissions still apply offline. **No policy, no action.**

Changing a policy is a reviewed change to this repo: open a pull request, and it applies once it's merged
into `main` (the core checks every 20 seconds).

## `tass.json`

| Field | Meaning |
| --- | --- |
| `identity` | The identity this policy is for (`TASS`). The Thought Engine refuses model work for any other name. |
| `owner` | GitHub login that owns this identity. |
| `obey` | GitHub logins whose issues and comments TASS answers. Everyone else is ignored. |
| `tass_md` | Where the versioned copy of `tass.md` lives: `owner/repo@branch:path`. Never `main`, `alpha` or `stable`. |
| `offline_grant_hours` | How long a locally cached copy still authorizes **local** work (thinking, the read-only action) while GitHub can't be reached. Default 24. After that, everything fails closed until GitHub confirms the policy again. GitHub actions always need a fresh copy. |
| `allow` | What TASS may do. Anything missing or `false` is refused. |

| Action | Lets TASS… |
| --- | --- |
| `model.request` | think with the local model, through the Thought Engine (also checked by the engine itself, again right before the model runs) |
| `thought.process` | send what you say through the Thought Engine's Spark → Trace → Thought pipeline (the engine's Thought Processing switch follows this) |
| `action.read_tass` | run the one action TASS may propose: count things in his own local `tass.md` (read-only). Checked at the moment it would run. |
| `github.read_messages` | read issues labelled `tass` and their comments in the `tass_md` repo |
| `github.reply` | post his answers as comments there |
| `tass_md.sync` | keep his local `tass.md` and the GitHub copy in step |
| `tass_md.conflict_pr` | open a pull request when both copies changed the same section |

To stop TASS doing something, set it to `false` (or delete the file to stop everything). It takes effect on
the next sync; the engine refuses model work as soon as the local copy is gone.
