# GitHub mailbox contract (Model B delivery)

Status: **candidate, fixture-tested only.** Arameair/TASS #24; Model B is Thought-Processor #134 D2-a.
Stacked on PR #47 at `a67064380cb55d1b55f9e8454cf2f60d12c6b306` (not merged, not installed). Nothing here
enables a grant, creates a credential or posts anything by itself.

## Roles

| Who | Holds | May |
| --- | --- | --- |
| Core (`core` key) | no GitHub write credential | mint `github.mailbox.reply`; ask the owner to approve a share; read its own share request |
| Owner (Identity page + passphrase) | — | enable/revoke grant `github.mailbox.reply`; approve or reject the exact text of one share |
| Communication Bridge (`github-publisher` key, redeem-only) | the GitHub write credential | redeem either action once, immediately before it posts, to Arameair/TASS #24 only |

## Actions

- `github.mailbox.reply`: bound, owner grant of the same name required (not a default), minted by `core`,
  redeemed only by `github-publisher`, single point of use (no `check`).
- `github.mailbox.share`: bound, UNLOCKED required, **never minted by a service**. `POST /v1/github/mailbox/share-approve`
  (owner page, passphrase, `text_sha256` of the text shown) makes Identity mint it from the text it holds.
  Share requests are memory-only (an Identity restart forgets them; Core asks again).

## Binding

Lowercase hex SHA-256 of these seven lines joined with `\n` (no trailing newline):

```
tass.github.mailbox/1
github-publisher
<action>
<repo>
<issue>
<source: accepted comment ID (reply) or share ID (share)>
<lowercase hex SHA-256 of the exact payload bytes>
```

The reply payload is the exact comment body. The share payload is the owner-approved text; the Bridge posts it
inside a fixed wrapper it does not take from Core.

## Routes added

| Route | Auth |
| --- | --- |
| `POST /v1/github/mailbox/share-requests` | service `core` |
| `GET /v1/github/mailbox/share-requests/{id}?entity_id=` | service `core`, same entity only; text never returned |
| `GET /v1/github/mailbox/share-requests` | owner page |
| `POST /v1/github/mailbox/share-approve` | owner page + passphrase (throttled like grant changes) |

## Not covered

Owner page UI for approvals (routes only), Windows protected-profile deployment (D2), and the in-flight window
between redemption and GitHub executing the POST, which no Identity check can close.
