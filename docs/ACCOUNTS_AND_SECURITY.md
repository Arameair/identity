# GitHub accounts, private entity repositories and security

[← Home](../README.md) · [Setup wizard](FIRST_RUN_WIZARD.md) · [Threat/acceptance tests](ACCEPTANCE_TESTS.md)

## Key rule: every entity gets a private repo, NOT necessarily a new GitHub login

A single human's GitHub account can manage many private entity repos. For example, Example Workshop and TASS each get a distinct repo and stable entity ID. An admin can authorize a specialized worker to access only the repository/tasks approved for it.

If the user has **no GitHub account**, Identity guides them through [GitHub's own signup process](https://github.com/signup) in their browser. GitHub handles passwords, email verification, CAPTCHA and MFA. Identity never asks for these details and never impersonates a human to mass-create accounts.

## Distinguish three stores

| Storage | Permitted content | Forbidden content |
| --- | --- | --- |
| **Private entity GitHub repo** | Schema-versioned nonsecret entity ID/type/name, module types, human-readable docs, declared relationships/permissions *without grants*, immutable audit references | OAuth tokens, passwords, MFA, raw cookies, Discord tokens, key material, private memory, raw chat history, access grants |
| **Protected local entity state** | Actual runtime locations, connected status, local installation bindings, revocation timestamps, migration backups | Plaintext secrets unless backed by authenticated encryption with separately protected key |
| **OS credential manager / secure store** | Encrypted provider tokens and permitted grants for this device, bound to a principal and scopes | Credentials exposed through general diagnostics, environment dumps, unrestricted APIs, source control |

Even a private GitHub repo is **not a safe token vault**. Rotate/revoke credentials if ever leaked, including through Git history. A repo manifest cannot authorize its own privileged actions.

## Proposed authorization design

- Prefer **provider-approved OAuth / GitHub App permissions** appropriate for desktop/local apps; PKCE/state and appropriate token handling where applicable. If a particular operation needs repo creation permission, clearly show the additional scope and obtain explicit user consent.
- Request the **minimum** action scope for the specified repository/entity: `github.repo.read`, `github.repo.write`, `discord.channel.post` are *logical capability names*, **not claims about GitHub's actual API scope strings**.
- A connection may be **configured but unauthorized**, **authorized but expired**, **ready**, **denied** or **revoked**. Do not call all of these “connected.”
- Tokens do not leave Identity's trust boundary. Rather than returning a token, Identity should **broker** an approved operation, or issue a **short-lived, limited, revocable local capability** checked on use.
- All requests require authenticated caller identity, entity authorization, CSRF/replay protection where relevant, constrained inputs and explicit approval for write actions. Treat text from modules/models as untrusted.
- Audit who requested an action, what entity, scope, outcome and time, but never secret values or arbitrary private input.
- Test on Windows and Linux; fail with a clear actionable message if no suitable protected secret store is available. Do not silently downgrade to plaintext files.
- Do not place client secrets in browser code, logs or any checked-in `config.json`; a public OAuth client uses appropriate public-client protections.

## Connection/setup checks

When creating/selecting a repository, check the returned **owner/name, identity association, privacy, authorization and write capability** before reporting success. Never silently switch a private repo to public, create another repo to recover from a conflict, overwrite another entity's manifest or promote a branch.

When connecting to Discord or other services, start with a deliberately small documented permission set and test **Revoke** as well as **Connect**. Each account request should explain exactly which actions become possible and which still require owner approval.

## What happens on a new computer?

The nonsecret entity blueprint can be restored from the same private repo *after the human logs into GitHub*. Locally stored account grants are **device-specific** and generally require reauthorization. A revoked credential must not be revived by restoring an old manifest.

## First prototype safety boundary

**Gate 1 is a local entity-store prototype only:** no GitHub/Discord account connection, no mock GitHub adapter, no synthetic account credentials, and no account permissions UI. It merely shows future connection statuses as **NOT CONFIGURED**. After Gate 1 is tested and the owner approves, Gate 2 may implement supported protected credential storage and connect a real GitHub account to create one small private test repository after explicit owner confirmation. See [First prototype](FIRST_PROTOTYPE.md).

[Next: controller connection →](CONTROLLER_INTEGRATION.md)
