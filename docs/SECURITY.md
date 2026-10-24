# Security and account-management boundaries

**Current implementation:** experimental local identity manager using an existing official GitHub CLI login. **Not a production-ready identity provider or authenticated multi-service broker.**

## Trust principles

- Identity is intended to control **who can use what**, not to give every feature a shared master token.
- A local identity card is a reference; an account connection and its privileges require independent verification.
- Creating a business, person or companion does **not** grant Thought, GitHub write access, Discord, or any other capability by default.
- The receiving feature must independently reject absent, revoked or incorrectly scoped authorization. A UI toggle is not enforcement.
- No private GitHub repo, log, model prompt, diagnostic bundle or screenshot should contain passwords, access tokens, keys, session cookies or MFA codes.

## GitHub: current and target state

**Today, in an experiment:** The Go app invokes `gh api` using a previously authenticated local GitHub CLI session. It stores a verified login name and time, not the credential. It can read an existing private repository's metadata before associating that repository with an ID card. The app has **no GitHub create, modify, or delete repository operation**.

This is **machine-scoped CLI authentication** and must not be described as Identity issuing secure entity-scoped GitHub grants. GitHub CLI may use an unprotected credential file where an OS keyring is unavailable. A WSL/VM login may differ from the host login.

**Target:** Evaluate provider-approved GitHub App authorization with narrowly scoped permissions, proper revocation and operating-system-protected credential storage. Require explicit user approval before creating or writing a private repository. Make operations auditable and idempotent. Do not collect GitHub passwords or bypass provider authorization.

See [Issue #9](https://github.com/Arameair/identity/issues/9) for G1–G5.

## HTTP and service-to-service access

The current experimental UI binds to loopback and checks request origins. **Loopback and Origin checks are not equivalent to authenticated user sessions or service authorization.** Do not expose experimental account operations to remote networks or treat a detected local port as trusted.

Before cross-service grants are live, design and test actor authentication, installation/service identity verification, scoped grants, revocation, replay defenses, version compatibility and outage behavior. Feature services need enforcement at their own ingress.

## Data safety

- Preserve the original identity ID across restart, restore and code changes.
- Archive should be reversible and must never delete a GitHub repository or independent feature data.
- Distinguish disposable experiment profiles from owner-authorized durable storage; back up data before migrations.
- A repository manifest may contain non-secret configuration but cannot become a remote source of unrestricted authorization.

**Not yet verified:** Production credential vault, remotely exposed service authentication, official GitHub App login, a secure grant protocol, or durable grant revocation across independently running apps.
