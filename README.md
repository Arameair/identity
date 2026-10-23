# Identity — the reusable entity and account manager

**Project state: design proposal. Nothing below is an implemented feature.** The software repository is **private**. The owner approves each prototype and any promotion to Alpha.

Identity is the reusable **starting point for a new entity** in our Thought Controller ecosystem. An entity may be a companion, business, person, organization, project, or another independent participant. It does **not** have to think or possess a personality.

The idea: **download Identity → create or restore your own identity → link a private GitHub repository → connect to Thought Controller → add independent capabilities as needed.**

## Start here — click through the whole project

| Read | What you'll learn |
| --- | --- |
| **[1. Start here: an easy walkthrough](docs/START_HERE.md)** | See what the user experiences, with examples and a suggested reading route |
| **[2. Vision and boundaries](docs/VISION.md)** | What Identity is for, and what it deliberately does not do |
| **[3. Identity and entity model](docs/ENTITY_MODEL.md)** | What records exist, how identities persist, and how business and AI examples differ |
| **[4. First-run setup wizard](docs/FIRST_RUN_WIZARD.md)** | Every screen and the choices a new or returning user makes |
| **[5. GitHub, accounts and security](docs/ACCOUNTS_AND_SECURITY.md)** | Official sign-up, private repos, permissions, credentials, approval and recovery |
| **[6. Connecting Thought Controller](docs/CONTROLLER_INTEGRATION.md)** | Discovery, startup boundaries, registration and requesting a capability |
| **[7. Puzzle pieces and environments](docs/CAPABILITIES.md)** | How memory, communication, and other independent modules are found and linked |
| **[8. Prototype roadmap](docs/ROADMAP.md)** | Small experiments, order of work, and what not to build first |
| **[9. Owner acceptance tests](docs/ACCEPTANCE_TESTS.md)** | Specific steps to prove the system actually works |
| **[10. Decisions and open questions](docs/DECISIONS.md)** | What we agreed on versus what still needs testing |
| **[11. Current status](docs/STATUS.md)** | What is actually done versus designed |
| **[Instructions for a coding team](AGENTS.md)** | Branch policy, evidence, safety and first assignment |
| **[Example safe entity manifest](examples/entity-manifest.example.json)** | A nonsecret example of what an entity's GitHub repository might store |

## A picture of the goal

~~~mermaid
flowchart TD
    U["A person downloads Identity"] --> I["Identity setup / restore"]
    I --> G["Entity's private GitHub repo<br/>Nonsecret identity + module blueprint"]
    I --> S["Local protected state / credential store<br/>NOT stored in GitHub"]
    I --> T["Thought Controller<br/>find it, or approved start, then register entity"]
    T --> M["Optional Memory"]
    T --> C["Optional Communication"]
    T --> P["Optional Perception"]
    T --> X["Other puzzle pieces"]
~~~

**Crucial distinctions:** the **Identity application repo** (this software) is *not* an entity's personal repository. The **entity** is the durable participant. A **GitHub account** may manage many private entity repositories. The repository describes its configuration and relationships, **not** its access tokens or live thought contents. Thought Controller does not automatically gain any privilege possessed by an entity.

## The first working test we want

Create a new test entity, close the app, reopen it, verify the *same ID* is restored. Guide the owner through authorizing an existing GitHub account and creating a **private** repository (with explicit confirmation). Discover an already running Thought Controller, register the test entity once, reconnect idempotently, and show exact statuses and safe copyable diagnostics. Test failure and denial cases too. **Start using a fake GitHub/controller adapter; do not attach actual accounts until security is demonstrated.**

## Project conventions

- **main** — bootstrap entry point; **alpha** — owner-controlled approved baseline; **experiment/*** — unapproved branches; **stable** — optional owner-selected snapshot. Never auto-merge or silently promote. Start development candidates from current **alpha**.
- Use independent Play/Stop through [Project Controller](https://github.com/Arameair/Project-controller). Do not secretly launch or stop other managed applications.
- Read [Thought Controller's current repair/acceptance issue](https://github.com/Arameair/Thought-Controller/issues/3). **Entity registration is planned, not part of the current V1 interface.**
- Real account permissions require user-approved narrow authorizations. No OAuth client secrets, personal tokens, passwords, MFA codes, private keys or session cookies in any repository, logs, issues, screenshots or generated prompts.

To propose an implementation, use the gated [roadmap](docs/ROADMAP.md) and [acceptance criteria](docs/ACCEPTANCE_TESTS.md). This branch is documentation-only.
