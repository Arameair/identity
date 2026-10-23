# Decisions and open questions

[← Home](../README.md) · [Status](STATUS.md)

## Agreed direction from the owner discussion

| Decision | Status |
| --- | --- |
| Identity is reusable across companions, people, businesses and other entities | **Direction agreed** |
| Personality is NOT part of the required identity definition | **Agreed** |
| Each entity has its own associated GitHub repository | **Direction agreed**; repository must be private |
| The Identity application helps a user **create or connect a GitHub account** and set up the private entity repository | **Direction agreed**; use official signup/provider flows |
| Identity handles account connection and permissions, not Thought Controller or each module separately | **Direction agreed** |
| Identity persists across restarts and switching versions; another installation creates/restores a distinct entity | **Direction agreed** |
| Identity detects Thought Controller and registers entity; should support approved start if missing | **Direction agreed**; startup/permission details remain to test |
| Memory, curiosity, personality, Discord, perception are independent optional puzzle pieces | **Direction agreed** |
| Never put credentials in a GitHub repository, including private ones | **Security requirement** |
| Experimental branches do not auto-promote to Alpha | **Owner branch policy** |

## Proposed interpretations to confirm by testing

1. **Always private** per-entity repositories (this is the safe recommended default), even when entity is a public business. A later public-facing *separate* repo may be linked.
2. **One GitHub account may manage several entity repos**, so “every identity has a GitHub” doesn't force each identity to acquire a separate login.
3. Identity preserves a **stable entity ID** independent of code branch and OS, with explicit restore and ownership verification.
4. On first startup **discover controller automatically**. If not running, use approved Project Controller lifecycle or a previously approved standalone auto-start setting, never a hidden duplicate.
5. A private GitHub repo holds the **blueprint**; local secure storage holds grants and runtime locations; external modules hold their own data.
6. First version of Thought Controller handshake is **single test entity only**; real multi-entity thought isolation is later.

## Open technical decisions — don't pretend resolved

- Which exact **OS secure-store implementation** and account authorization method on Windows and on Linux? Decide via a security-reviewed prototype, not a plaintext fallback.
- How to prevent a malicious or wrong local process from impersonating Thought Controller at the expected port? Consider authenticated discovery and service identity.
- Where should durable local entity state live when two Project Controller experiments run side by side? **Default to isolated test state** and explicit selection of a protected live profile.
- Is the per-entity manifest the authority for display-name and types, or just a remote synchronization copy? Define conflict resolution, owner identity, migrations and backups.
- Which actions may be preauthorized by policy vs require a fresh approval every time? Start deny-by-default for remote writes.
- How do we support businesses with delegated administrators and service accounts without treating an untrusted manifest as proof of permission?
- Which exact API version should Identity and Thought Controller agree on once the owner has verified the repaired controller?

**Use a new design issue or PR comment** to record a decision with test evidence, risks and owner's choice. This file documents proposals and does not authorize implementation automatically.
