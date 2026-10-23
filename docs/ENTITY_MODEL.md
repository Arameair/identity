# Identity, entity, account and repository — four different things

[← Home](../README.md) · [Security](ACCOUNTS_AND_SECURITY.md) · [Example manifest](../examples/entity-manifest.example.json)

## Plain definitions

- **Entity:** A distinct participant, such as an AI, person, business, organization or project. It has a unique stable ID regardless of whether its software is running.
- **Identity record:** The durable description of the entity: its ID, type, display name, associated private GitHub repo, ownership/relationships and registry of permitted capabilities. Personality is not a required field.
- **Account connection:** Authorization to use an external account or service (e.g. an owner's GitHub or a Discord bot integration), with scopes, provenance, expiry and revocation controls. This is **separate from the entity**.
- **Identity repository:** A **private GitHub repo per entity** that stores the *nonsecret* blueprint/configuration and links to its independently managed modules. It is not a vault and is not this reusable application's source repository.
- **Runtime instance:** A particular running installation/registration of an entity with Thought Controller. The same durable entity can later run on another device after reauthorization. Do not duplicate an entity simply because a process starts.

## What is required at creation?

| Field | Required? | Notes |
| --- | --- | --- |
| Stable unique entity ID | Yes | Random, globally unlikely to collide; never derive only from display name |
| Entity type | Yes | User-facing examples: companion, person, business, organization, project, other |
| Display name | Yes | User can rename without changing the stable ID |
| Created timestamp / schema version | Yes | For migrations and safe validation |
| Owner/administrator | Yes for managed entities | Explicit principal authorization; never assume OS username alone establishes remote ownership |
| GitHub repository association | Setup requirement | **Private repo required to complete first setup**, but allow a local **pending** state while offline or awaiting account approval |
| Account permissions and module links | Optional | Add later through explicit approval |
| Personality, AI model, memory | **No** | Optional modules or contextual attributes, never prerequisites for an entity |

## Where things live

~~~mermaid
flowchart LR
    Code["Identity software repo<br/>shared reusable code"] -.-> E["Entity record: stable ID"]
    E --> Repo["Per-entity PRIVATE GitHub repo<br/>versioned nonsecret manifest"]
    E --> Local["Local persistent entity store<br/>runtime references, migrations"]
    E --> Vault["Device keyring/credential store<br/>tokens, sensitive grants"]
    E --> TC["Thought Controller registration<br/>runtime status + thought ownership"]
~~~

The private repository is the durable **logical blueprint**. The local store records what is actually connected on this device. A repository URL/manifest is not proof that a service is reachable or that an action is authorized.

## How to restore an identity

1. Offer locally known identities and **Restore from a private GitHub repo**.
2. Confirm user-controlled GitHub authorization and read a versioned manifest from a repo the user is permitted to access.
3. Check stable entity ID and repository association for duplicates or conflicting owners; show an explicit warning if mismatched. Do not accept a repository's declaration as remote authorization.
4. Rebuild local references safely. External credentials should be reauthorized on a new device; never sync them in the manifest.
5. Reconnect to Thought Controller and modules when available, without automatically creating a new entity.

**Branch isolation nuance:** experiment branches of the *app code* get isolated **test data by default**, so experiments cannot mutate your real identity. An approved release may point to a durable user-selected entity store. A code branch must never be used as the ID.

## Example relationships

- A business entity **owns** a workspace and **authorizes** an employee/worker for a limited task.
- A companion entity **uses** Memory and Discord via separately granted capabilities.
- A person entity **administers** one or more companion entities.

Relationships are explanatory until enforced by Identity's separate, explicit access policy. Ownership labels alone do not grant token use.

[Next: guided first run →](FIRST_RUN_WIZARD.md)
