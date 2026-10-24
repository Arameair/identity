# Vision — durable identity without forced capabilities

[← Project home](../README.md) · [Status](STATUS.md) · [Architecture](ARCHITECTURE.md)

## Purpose

Identity aims to give every authorized entity a stable reference, a reliable configuration home, and clearly controlled access to shared services.

Entities can include a person, business, AI companion, project, organization, or system service. Being an identity **does not imply having a personality, an AI model, or the ability to think**. Those are optional capabilities, enabled separately when needed and allowed.

## The problem

Independent software components should not each invent a different notion of who owns their data, which GitHub account is usable, or which permissions an entity has. Disconnected implementations create duplicate records and inconsistent access decisions.

Identity is the shared **identity and authorization authority**, not the controller for every feature's internal work. Specialized applications remain independently installable and operable.

## Experience we want

- **Companion:** Create or restore TASS once. Enable Thought for TASS if approved, without enabling it for unrelated entities.
- **Business:** Create a distinct business identity with its own repository and approved features. It does not think or access TASS data by default.
- **Developer:** See actual connection status and permissions in one place, test new service versions independently, and preserve identity across branch changes.
- **New installation:** Restore known local identities first, then guide the administrator through any necessary official account authorization. Do not require GitHub merely to create a local identity.

## Product principles

1. **Identity persists.** A temporary process, git checkout, model provider or interface must not define the immutable identity.
2. **Features are independent.** Install, grant, revoke and operate each specialized capability separately.
3. **Approval has scope.** Owning an entity does not authorize every service call; permission must be verified at the receiving service.
4. **Data has an owner.** Identity manages ID records, account references, grants and audit decisions; Thought owns thoughts, and each source owns its operational data.
5. **Reliability is observable.** State must distinguish configured, connected, authorized, ready and verified.
6. **No secret in Git.** Entity repositories are non-secret blueprints, not account vaults.

This describes the **intended product**, not a declaration that all interfaces are operational. See [Status](STATUS.md) for evidence.
