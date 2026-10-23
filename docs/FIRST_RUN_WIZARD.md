# First-run and restore wizard — proposed screens

[← Home](../README.md) · [Start here](START_HERE.md) · [Account security](ACCOUNTS_AND_SECURITY.md)

**Build-stage note:** This page describes the **future complete onboarding wizard**, NOT the entire Gate 1. The **first prototype only has name + type, unique ID, local save, list/open, and local restore** as specified in [FIRST_PROTOTYPE.md](FIRST_PROTOTYPE.md). GitHub/credentials/controller screens are **NOT IMPLEMENTED**, not fake or functioning, and must not be built in Gate 1.

**Goal:** A person unfamiliar with GitHub can set up an entity without needing terminal commands or manual token pasting. No screen below is implemented.

## Home view: calm, obvious choices

**Existing identities** shown as simple tiles with type, ID suffix, private repo, Thought Controller status and last successful connection. Buttons: **Open**, **Create identity**, **Restore identity**, **Diagnostics**.

Statuses must not conflate *configured*, *connected*, *authorized* and *ready*. A green-looking repo link isn't proof of authorized account access.

## Step-by-step new identity wizard

| Screen | User sees and does | Success condition |
| --- | --- | --- |
| **1. Welcome** | Create new / Restore existing. Explain in one paragraph what Identity does | Choice made; nothing external changes |
| **2. Name and type** | Name, entity type (AI/business/person/project/other), admin/owner context. No personality questions | Generate persistent ID exactly once, mark setup pending |
| **3. GitHub account** | **I have a GitHub account** → connect with provider-approved flow. **I need one** → open official [GitHub signup](https://github.com/signup) and guide user back after verification. **Not now** → save local draft | Confirm only that authorization was actually verified; never ask user for GitHub password or MFA codes |
| **4. Private repository** | Suggest a repo name, show owner and privacy **PRIVATE**, offer new or select existing; show what files would be written | Explicit confirm → repo created/selected, privacy and write permissions verified |
| **5. Local protection** | Explain protected credential storage, session access, revocation, backup limits; refuse insecure silent fallback | Sensitive credentials never committed, printed or copied |
| **6. Thought Controller** | Check local health/readiness without sending a model message. If running, register. If absent, show **Start via Project Controller** (or opt-in standalone start policy) and recheck | Controller reachable, authenticated registration acknowledged or clearly marked unsupported/pending |
| **7. Finish/dashboard** | Tiles for GitHub, Thought Controller, Memory, Discord, other modules, local state and diagnostics | Stable entity ID, linked private repo, clear per-capability status; no false green PASS |

**Important:** If GitHub is missing/offline, keep the identity in an explicitly **pending** draft state. Never invent a repo URL. Do not call setup complete until the real private repo link is verified.

## Return journey

Opening the same identity again should **restore the same entity ID and associated repo**. If a device changes, guide reauthorization; do not treat the GitHub manifest as a backup for local tokens. Never delete or replace the existing identity simply because a branch is different.

## Called from Thought Controller: “add an account”

A thought/workflow asks for e.g. **github.read on repository X for purpose Y**:

1. Identity receives a signed/authenticated **request with entity ID, calling app, purpose and scope**.
2. UI shows **Who requests what, for which entity, why, and what exact data/actions it would permit**.
3. User can **Deny**, **Connect existing**, or **Authorize via official provider**. A missing account opens the guided setup.
4. After confirmation, Identity verifies rights and records scoped authorization in local protected storage. Thought Controller receives **allowed/denied/pending + a nonsecret capability handle**, not provider tokens.
5. Revoking a grant updates status and prevents future authorized operations. The audit record contains no secret.

No model-generated proposal can self-approve its own requested permission.

## Example dashboard (text wireframe)

~~~text
Identity: TASS   [entity ID: ...]       [Copy safe diagnostics]
GitHub     : PRIVATE repo configured     [Manage] [Open repo]
Accounts   : GitHub authorized locally    [Manage] [Revoke]
Controller : Running / Registered         [Check] [Connect]
Memory     : Not configured               [Set up]
Discord    : Not connected                [Set up]
Status     : Last verified ... / errors ...
~~~

[Next: accounts and security →](ACCOUNTS_AND_SECURITY.md)
