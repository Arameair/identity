# Vision — Identity is the permanent connection point

[← Home](../README.md) · [How the user starts](FIRST_RUN_WIZARD.md) · [Entity model](ENTITY_MODEL.md)

## The problem we want to solve

Our projects are intentionally separate: Thought Controller, Chat-Jimmy, TASS, Communication Bridge, ScreenBridge and eventually Memory and other capabilities. Without a stable identity and configuration home, every app has to reinvent accounts, permissions and discovery, and changing branches risks losing the user's environment.

**Identity's job:** Create or restore a unique entity, maintain the entity's durable **nonsecret** description and repository association, guide account authorization and permissions safely, and allow other modules to register and discover approved capabilities.

An entity may be a companion, business, person, device, organization or project. Identity does **not** imply a personality. A business identity may never run Thought Controller; an AI companion may use Thought Controller continuously in some future version. Identity should be useful in both cases.

## Distinct responsibilities

| System | Owns |
| --- | --- |
| **Identity** | Persistent entity ID, entity repository association, setup process, authorized account connections, access policy, capability registry/locator |
| **Thought Controller** | When and how to think; thought/event lifecycle; deciding which capability to request, subject to policy |
| **Module** (Memory, Discord connector, perception) | Its own specialist implementation, data and operational lifecycle |
| **Project Controller** | Starting, stopping, building and observing project processes under its management |
| **GitHub** | The entity's private repository, source history, nonsecret manifest and owner-controlled development history |
| **Local secure store** | Device-scoped secrets, account grants and other information that must never be committed |

## First principles

- **One Identity app → many unique identities.** Each entity has a permanent ID and normally its **own private GitHub repo**.
- **A GitHub account is not an entity.** One person's authorized GitHub login can own several entity repos, with scoped access.
- **Restore, don't regenerate.** Restarting or checking out another version must not create a duplicate identity.
- **Independent puzzle pieces.** Register modules with typed, versioned capability contracts and stable identity IDs. No hardcoded all-in-one TASS or WoW experience.
- **Permission first.** Thought Controller requesting a capability is not authorization for Identity to use an account or start an arbitrary application. Sensitive changes require the owner's approval.
- **Evidence over green badges.** Show what is configured, what is actually running, what was verified, why something failed and how to copy redacted diagnostics.

## Things we explicitly do NOT make Identity responsible for

- Assigning personalities to everything.
- Generating thoughts, maintaining memories or acting as an AI worker.
- Storing account passwords/API tokens in a private repository.
- Creating GitHub *user accounts* programmatically or bypassing GitHub's signup/MFA flow.
- Silently sending messages, creating repositories, approving OAuth scopes, merging branches or starting unmanaged programs.
- Assuming the latest Thought Controller already supports entity registration. **That interface is a planned next step.**

[Next: entity model →](ENTITY_MODEL.md)
