# `discord.reply`: Identity contract for one outbound Discord reply

Status: proposed in identity #46 for Thought-Processor #127 (Discord Milestone A). Nothing in this
document enables a grant, sends a message, or changes an installed Identity.

Identity decides whether the entity may send one exact reply to one exact inbound Discord message.
It never sees message text, never talks to Discord, and never decides what the entity says.

## Three different things

| | What it is | Who acts |
| --- | --- | --- |
| Standing owner grant `discord.reply` | Permission for the entity to reply in Discord at all. Not a default. Sends nothing by itself. | The owner, on Identity's page: enabling needs the passphrase, revoking does not. |
| Single-use decision | Identity's answer for ONE reply, tied to one binding. Expires after 120 seconds; redeemed at most once. | Core obtains it; the Discord door redeems it just before posting. |
| Manual approval | The owner approving one specific message. | Not part of `discord.reply`. |

## Services and keys

`IDENTITY_SERVICE_KEYS` keeps its format: one `<service> <key of 32 or more characters>` per line.
The Discord door uses the service name `discord-door` and its own key.

| Service | Obtain (`mint`) `discord.reply` | Redeem `discord.reply` | Other service routes |
| --- | --- | --- | --- |
| `core` | yes | no | unchanged |
| `engine` | no | no | unchanged |
| `discord-door` | no | yes | none: `POST /v1/authorize` answers 403, `GET /v1/github/lesson` answers 403 |

The door cannot redeem any other action, and the Engine cannot redeem `discord.reply`.
A key file without a `discord-door` line simply means nothing can redeem `discord.reply`.

## Binding

The binding is the lowercase hex SHA-256 of the UTF-8 bytes of these eight lines joined with a
single `\n` (no trailing newline). Core and the door each compute it from the data they hold;
Identity treats it as opaque and only compares it.

```
tass.discord.reply/1
discord-door
<bridge inbound message id>
<Discord source message id>
<Discord sender id>
<Discord channel id>
discord:<Discord channel id>
<lowercase hex SHA-256 of the exact reply text bytes that will be transmitted>
```

| Line | Binds the decision to |
| --- | --- |
| 1 | this contract version |
| 2 | the authorized door (the only redeemer) |
| 3, 4 | the inbound message being answered |
| 5 | the sender |
| 6 | the channel the message came from |
| 7 | the destination: a reply goes only to that same channel |
| 8 | the exact reply content |

No field may contain a line break. If any value differs between what Core authorized and what the
door is about to send, the bindings differ and the redeem is refused.

### Test vector (fixture values, not real identifiers)

| Field | Value |
| --- | --- |
| Bridge inbound message id | `m_4d442e5526b608cfe449` |
| Discord source message id | `1562034986897375235` |
| Discord sender id | `2100000000000000001` |
| Discord channel id | `1100000000000000001` |
| Reply text | `TEST FIXTURE reply: hello from TASS` |
| SHA-256 of the reply text | `412238e0e55005100cbdeb1137fcec8122648f95aa03fe30bce39fa946ca5a6d` |
| Binding | `f5ab28d15e23a3850274a3604fb999d433bb3980d9e4ff970bec8029d095a25c` |

`cmd/tass-identity/discord_reply_test.go` recomputes this vector.

## Obtain a decision (Core)

`POST /v1/authorize` with Core's service key:

```json
{"action": "discord.reply", "binding": "<64 hex characters>", "detail": "reply to m_4d442e5526b608cfe449"}
```

Allowed only when the entity is UNLOCKED, is this installation's entity, and holds the
`discord.reply` grant. The answer carries `allow`, `decision_id`, `reason`, and (when allowed)
`expires_at`. `detail` is kept in `decisions.jsonl` (at most 200 characters) and must never carry
message text.

## Redeem a decision (the door)

`POST /v1/decisions/redeem` with the door's service key:

```json
{"decision_id": "dec_…", "action": "discord.reply", "binding": "<64 hex characters>"}
```

Allowed once, and only if all of these hold at that moment:

- the caller is `discord-door`;
- the decision exists, is for `discord.reply`, and has this exact binding;
- it has not been redeemed and has not expired;
- the entity is still UNLOCKED, still the same entity, and still holds the grant.

The last point is point-of-use revocation: revoking the grant or locking Identity after Core
obtained the decision, but before the door redeems it, means the reply is not sent.

Pending decisions live in memory only. An Identity restart forgets them, so nothing obtained before
the restart can be redeemed afterwards.

## Audit

Every obtain and redeem, allowed or refused, is appended to `decisions.jsonl` with the service,
action, binding, decision id and reason. No message text is recorded.

## Out of scope

Proactive messages, any Discord action other than a reply, any other messaging service, and any
change to default grants.
