# Identity policies

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
| `allow` | What TASS may do. Anything missing or `false` is refused. |

| Action | Lets TASS… |
| --- | --- |
| `model.request` | think with the local model, through the Thought Engine (also checked by the engine itself) |
| `github.read_messages` | read issues labelled `tass` and their comments in the `tass_md` repo |
| `github.reply` | post his answers as comments there |
| `tass_md.sync` | keep his local `tass.md` and the GitHub copy in step |
| `tass_md.conflict_pr` | open a pull request when both copies changed the same section |

To stop TASS doing something, set it to `false` (or delete the file to stop everything). It takes effect on
the next sync; the engine refuses model work as soon as the local copy is gone.
