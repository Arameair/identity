# End-user design notes for Identity

*(captured while the design is still open — October 2026)*

These notes come from an outside-user perspective. They are not requirements or decisions; they are observations about what would make Identity feel clean, honest, and usable while the rest of the system (Thought-Processor, grants, feeds) is still being designed.

## Core principles to protect

- **No fake states.** Never show “connected,” “authorized,” “ready,” or a green indicator unless a real check just succeeded and the evidence can be pointed to. Prefer precise language: “GitHub CLI not found,” “signed in as *username*,” “worker mock mode — refused,” “port open but not Thought Controller,” “grant held locally by engine (Identity not connected yet).”
- **One folder, one process.** Everything Identity owns stays in the single data directory the user chose. No hidden caches, no background services, no system-wide changes. Make the path always visible.
- **Identity does not think.** It issues durable IDs, records account links, and (later) grants. It does not store thoughts, run models, or control other processes’ lifecycles.

## What the first usable Identity should help the user accomplish

1. Create a durable identity card (permanent ID, type, name) that survives restarts.
2. Link an existing GitHub CLI session by verifying it and storing **only** the username. Explicitly state that the token is never seen or stored.
3. Show a single, factual readiness panel that answers three questions without ambiguity:
   - Identity cards exist? (yes/no + System ID)
   - GitHub linked? (not linked / linked as *username*)
   - Thought Processing readiness? (not available yet / Chat-Jimmy not found / engine not running / engine waiting for worker / LIVE)
4. Make the next physical step obvious and copyable (“Start Chat-Jimmy without -mock, then start Thought-engine on port …”) while remaining honest that those programs are still separate.

## GitHub attachment

- Keep the current real CLI verification approach.
- Surface exact status: CLI missing, not authenticated, authenticated as different user, or successfully linked.
- Unlink must only remove the local record; it must not log the user out of the CLI.
- Never offer repository create/modify/delete until those operations are real, scoped, and explicitly approved.
- Show the data folder and the fact that only the username is stored.

## Path toward Thought-Processor / Thought-engine

- Identity should be the starting screen, not a peer that the user has to discover later.
- Discovery of a local Thought Controller or Engine must be labeled “read-only discovery, not authenticated.” Distinguish “not running,” “port open but wrong service,” and “detected.”
- Until a real grant protocol exists, any ON/OFF flag that lives inside the engine must be labeled as such (“permission currently held by engine”). Do not present it as an Identity grant.
- When Processor exists, it should open already knowing the selected identity and show the real grant status (or the honest “not yet issued by Identity” state).

## UI and language rules

- Every control that performs an action must show the outcome: requested → accepted/refused → observed effect.
- Buttons stay disabled or clearly marked unavailable until the authoritative service confirms the capability.
- Persistent footer or status bar: data directory, System ID, listening address (127.0.0.1), current branch/commit, “no GitHub writes,” “loopback only.”
- Avoid collapsing distinct states into a single “online” or “enabled” badge. Keep the distinctions already called for in the design docs (not installed / installed but disabled / enabled but no feeds / waiting for worker / ready / degraded / revoked).

## Security and cleanliness reminders

- Loopback + origin checks are not authentication. Do not treat them as sufficient for privileged actions.
- Never store tokens, passwords, or session material in the identity store.
- Archive/restore must be local-only and reversible; never imply a remote resource was deleted.
- Provide an obvious way to see and remove the single data folder.
- Document the exact GitHub CLI credential-storage behavior the user will inherit so expectations stay accurate.

## What to defer until it can be real

- In-app OAuth / GitHub App flows.
- Cross-process feature grants that the receiving service actually enforces.
- Automatic feed wiring or process startup.
- Any UI that suggests Thought Processing is available before the engine, worker, and grant path have been verified on the user’s machine.

These constraints keep the design honest, minimal, and usable while the rest of the system is still being built. The user should always be able to answer: “What is actually running, where does the data live, and what is still missing?”
