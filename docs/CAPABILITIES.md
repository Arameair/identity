# Puzzle pieces — capability registry and environment

[← Home](../README.md) · [Controller integration](CONTROLLER_INTEGRATION.md) · [Example manifest](../examples/entity-manifest.example.json)

The owner's design is not an all-in-one companion. Each ability is an independent **puzzle piece**, and Identity is the **index that says which entity has which pieces**.

## What a piece looks like

A capability has a **logical name**, module identity/version, configuration reference, declared permissions, current runtime locator, readiness result and owner relationship. Avoid baking an absolute Windows path, Debian path or runtime port into the long-term GitHub manifest.

Examples:

| Logical capability | Specialized module | Does Identity own its data? |
| --- | --- | --- |
| memory.primary | Separate Memory module / store | **No** — Identity records how to find the permitted memory |
| thinking.controller | Thought Controller instance | **No** — controller runs thought lifecycle |
| communication.discord | Discord connector | **No** — Identity brokers/records authorization; connector handles transport |
| perception.screen | ScreenBridge | **No** — explicit screen source permission and runtime location |
| github.repo.read | GitHub account connection | **No** — Identity manages grants and checks, doesn't place tokens in repo |
| personality | Optional personality module | **No** — not required for an identity |

A business identity might connect repositories and a billing/reporting capability, without Memory, Thought Controller or personality. A companion can connect thinking, memory and communication. Both use the same Identity system.

## What happens when something is missing?

1. A participant (or Thought Controller) asks for `memory.primary` for entity `id-X`.
2. Identity checks that the caller is authorized for entity X.
3. If installed and verified, Identity returns a **scoped capability handle/location**.
4. If absent, Identity responds **not configured** and prepares an owner-visible **Connect / Install / Create** wizard.
5. Only after appropriate approval does the installer configure the external module and register a locator, version, ownership, granted actions and health probe.
6. Store a durable **nonsecret** reference in the entity's private repository, plus local installation information outside GitHub. Show success only after the new module passes a real health/use test.
7. The module keeps its **own lifecycle and data**. Replacing it requires a deliberate migration/relink decision, not overwriting an old instance automatically.

## Four distinct status fields

- **Configured:** a manifest or local setting exists.
- **Authorized:** the calling principal has a current valid grant for specific actions.
- **Reachable:** the implementation responds.
- **Verified ready:** the specific requested operation can be performed (where a safe non-mutating check exists).

This prevents the “configured but not running” ambiguity that caused trouble in other controllers.

## Safety boundaries

Model text may *request* a missing capability, but may **not** directly create repositories, install executables, read tokens or approve external writes. Identity must separate **request**, **owner approval**, **provisioning**, **verification**, **audit**, and **revocation**.

[Next: roadmap →](ROADMAP.md)
