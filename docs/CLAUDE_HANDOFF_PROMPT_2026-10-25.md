# Copy/paste prompt for Claude — executable prototype, not another planning pass

You are implementing a narrow experimental prototype across Arameair/identity and Arameair/Thought-Processor. The owner needs to run the **real** Thought Processor and a cleaned-up Identity together at home by Monday, October 26, 2026 (America/Chicago). Do not spend the session creating more disconnected demo branches or theory.

**START BY READING THE ACTUAL GITHUB SOURCE:**
1. Identity implementation assignment: https://github.com/Arameair/identity/blob/experiment/identity-consolidated-home-trial-20261025/docs/START_HERE_CLAUDE_2026-10-25.md
2. Acceptance: https://github.com/Arameair/identity/blob/experiment/identity-consolidated-home-trial-20261025/docs/HOME_TRIAL_ACCEPTANCE_2026-10-25.md
3. Processor companion handoff: https://github.com/Arameair/Thought-Processor/blob/experiment/processor-home-trial-handoff-20261025/docs/IDENTITY_HOME_TRIAL_HANDOFF_2026-10-25.md

**Work on Identity branch `experiment/identity-consolidated-home-trial-20261025`**, already created from Alpha. Start by inspecting `experiment/identity-real-manager-20261023`, `experiment/identity-standalone-access-trial-20261024`, and `experiment/identity-setup-wizard-20261024` (only reuse valuable code, do not bolt three apps together).

**FIRST: prove Thought Processor can actually run** using existing reference experiment `experiment/processor-controller-launch-fix-20261024` at exact known SHA `0ba8bd52facedda1405897e9136d68ab184a2e32` (it has `marquee.json` and Go `processor/`). Test with real local commands and supply exact Windows / owner Project Controller instructions. Do not rebuild Processor from scratch or demand a working Engine grant before this smoke test.

Then implement in ONE Identity executable: an independently runnable loopback-only app with real durable ID state, compact truthful status, a scrolling live persisted log and a safe one-click Copy Diagnostics report; read-only discovery of the live Thought Processor and a safe Open Original UI link. Keep GitHub App sign-in and local request draft if it fits; label any draft NOT SUBMITTED. Protect private routes, redact one-time `#code=` links/cookies/tokens/device codes and test redaction. Identity must not depend on Project Controller for ordinary standalone operation.

Fix only specific reproducible failures. Commit small coherent changes and show exact SHA, Go tests, local smoke, Windows cross-build, real-vs-fixture caveats, screenshots/evidence when available and short owner PowerShell run steps. Never touch Alpha/Stable/Main, never fabricate grants or a green connected status, and never claim you tested the owner's Windows system when you haven't.

**Delivery order:** real Processor launch first; consolidated Identity launch second; truthful two-process read-only demo third; enhanced GitHub/admin permissions later. The final answer must be a concise GREEN/YELLOW/RED status, links to the actual implementation branch/commits, exact commands to run when owner gets home, Copy Diagnostics sample with secrets removed, failing tests and blockers. Do the implementation now, not just a plan.
