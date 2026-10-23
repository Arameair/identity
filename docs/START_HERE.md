# Start here — what are we actually building?

**You do not need to know programming to follow this proposal.** [← Back to the project home](../README.md)

Imagine a fresh installation of the Identity application. It should help someone get set up even if they have never used GitHub.

## Story A: creating TASS

1. You launch Identity. It says **Create a new identity** or **Restore an existing identity**.
2. You choose **Create**, select type **AI companion**, and name it **TASS**. The app assigns an internal, permanent, unique entity ID.
3. It checks for GitHub. If you do not have a GitHub account, it **guides you to GitHub's own sign-up page**. It never asks for a GitHub password or two-factor code. If you already have an account, it offers a provider-approved authorization flow.
4. It asks permission to **create a private GitHub repository**, or lets you select a suitable existing private repository. The repo becomes TASS's nonsecret environment blueprint. You can see the selected repo before confirming.
5. It looks for **Thought Controller**, asks to register TASS, and reports whether registration was accepted. If the controller isn't running, the app explains why and offers a safe startup path rather than silently creating a second process.
6. It shows TASS's home dashboard: GitHub connected, Thought Controller registered, Memory not installed, Discord not connected, and a **request/setup** action for each missing component.
7. You close everything and come back another day, even through another code branch. It recognizes TASS and restores the same ID and configured resources instead of making another TASS.

This is **proposed behavior, not a working flow**. [Read the complete setup screen specification →](FIRST_RUN_WIZARD.md).

## Story B: creating a business

A second installation can create a **business entity** called Example Workshop. It has a separate ID, repository, owners, permissions and capabilities. It does **not** need a personality, memory or thought process. A business may later authorize a worker entity to perform an approved task on its behalf. Relationships do not automatically grant account access.

[Understand entity types and relationships →](ENTITY_MODEL.md).

## Story C: a thought asks for memory

- Thought Controller knows the currently registered entity ID.
- A workflow needs a memory capability, so it asks Identity to resolve **memory.primary** for that entity.
- Identity returns the registered capability/location **if permitted**; if absent, it prepares a guided setup request for the owner.
- Once a Memory module is configured and tested, Identity records a nonsecret pointer/manifest update and a local runtime locator. The Memory module still owns actual memory data.
- Thought Controller can then interact with the approved capability. Identity did not have to contain Memory or become a thinking engine.

[See how puzzle pieces work →](CAPABILITIES.md).

## Walk through in this order

| If you want to understand... | Open |
| --- | --- |
| What an identity is (without personality) | [Entity model](ENTITY_MODEL.md) |
| Which buttons the user sees | [First-run wizard](FIRST_RUN_WIZARD.md) |
| What GitHub stores and why tokens must stay out | [Accounts & security](ACCOUNTS_AND_SECURITY.md) |
| How the two applications find each other | [Controller integration](CONTROLLER_INTEGRATION.md) |
| How we test the first prototype | [Roadmap](ROADMAP.md) / [Acceptance tests](ACCEPTANCE_TESTS.md) |

**Non-goal for version one:** a fully autonomous multi-entity companion system. We are building one reusable puzzle piece and proving it genuinely connects.
