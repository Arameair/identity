"""Offline tests (fake GitHub + fake model). Run: python -m unittest discover -s tests"""

import sys
import tomllib
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from fakes import FakeGitHubState, start_fake_github, start_fake_model  # noqa: E402
from identity_liaison.agent import BLOCKED, DONE, WORKING, Agent, clean_card, parse_change  # noqa: E402
from identity_liaison.config import Config, default_text  # noqa: E402
from identity_liaison.github import GitHub  # noqa: E402
from identity_liaison.instructions import InstructionError, parse  # noqa: E402
from identity_liaison.model import Model  # noqa: E402

CARD = """# Team Liaison

**Card ID:** teams/TEAM_LIAISON.md
**Status:** PROPOSED

## Rules
- Track acknowledgments.
- Never treat DONE as verified.
"""

ISSUE = """read:
- Arameair/TASS@main:cards/*.md

change:
- Arameair/identity@teams:teams/TEAM_LIAISON.md

task:
Add a rule: a DONE report without evidence gets a request for evidence.
"""


def card_reply(messages):
    text = messages[-1]["content"]
    if "OWNER INSTRUCTION" in text and "CARD TO UPDATE" in text:
        return ("<think>planning</think>SUMMARY: Added the evidence-request rule.\n<<<FILE\n"
                + CARD.rstrip() + "\n- A DONE report without evidence gets a request for evidence.\nFILE>>>")
    return "The TASS card says the owner reviews changes before publication."


class Base(unittest.TestCase):
    reply = staticmethod(card_reply)
    finish = "stop"
    n_ctx = 8192
    mode = "pr"

    def setUp(self):
        self.state = FakeGitHubState()
        self.state.add_file("Arameair/identity", "teams", "teams/TEAM_LIAISON.md", CARD)
        self.state.add_file("Arameair/identity", "main", "README.md", "# Identity\n")
        self.state.add_file("Arameair/TASS", "main", "cards/TASS.md", "# TASS\nOwner reviews changes.\n")
        self.state.add_file("Arameair/TASS", "main", "cards/Chief.md", "# Chief\n")
        self.state.add_file("Arameair/TASS", "main", "code.go", "package x\n")
        self.gh_srv = start_fake_github(self.state)
        self.model_srv = start_fake_model(self.reply, n_ctx=self.n_ctx, finish_reason=self.finish)
        data = tomllib.loads(default_text())
        data["github"]["instruction_repos"] = ["Arameair/identity"]
        data["write"]["mode"] = self.mode
        data["model"]["url"] = f"http://127.0.0.1:{self.model_srv.server_port}"
        self.cfg = Config(data)
        self.gh = GitHub("fake-token", f"http://127.0.0.1:{self.gh_srv.server_port}")
        self.logs = []
        self.agent = Agent(self.cfg, self.gh, Model(data["model"]), log=self.logs.append)

    def tearDown(self):
        for s in (self.gh_srv, self.model_srv):
            s.shutdown()
            s.server_close()

    def comments(self, n=1):
        return [c[2] for c in self.state.comments if c[1] == n]


class ParseTests(unittest.TestCase):
    def test_sections(self):
        i = parse("t", ISSUE)
        self.assertEqual([str(r) for r in i.read], ["Arameair/TASS@main:cards/*.md"])
        self.assertEqual(i.change[0].ref, "teams")
        self.assertIn("request for evidence", i.task)

    def test_title_is_task_when_body_empty(self):
        self.assertEqual(parse("What does TASS want?", "").task, "What does TASS want?")

    def test_bad_ref_and_glob_change(self):
        with self.assertRaises(InstructionError):
            parse("t", "read:\n- not a ref\n")
        with self.assertRaises(InstructionError):
            parse("t", "change:\n- Arameair/x:teams/*.md\ntask: go")

    def test_real_pr26_output_is_cleaned(self):
        # Real Nemotron 3 Nano 4B output from issue #25 / PR #26 (2026-11-03):
        # it duplicated the last paragraph and echoed the prompt's end marker.
        data = Path(__file__).parent / "data"
        old = (data / "team_liaison_original.md").read_text()
        raw = (data / "pr26_model_output.md").read_text()
        out = clean_card(raw, old)
        self.assertNotIn("=== end of current content ===", out)
        self.assertEqual(out.count("Smallest useful test:"), old.count("Smallest useful test:"))
        self.assertIn("asks for the evidence instead of escalating", out)
        # Everything else in the original card survives untouched.
        self.assertTrue(out.startswith(old.rstrip()))

    def test_clean_card_keeps_exact_text_when_nothing_to_fix(self):
        text = "# A\n\n\n- one\n- two\n"
        self.assertEqual(clean_card(text, "# A\n"), text)

    def test_parse_change_fallbacks(self):
        s, body = parse_change("SUMMARY: x\n```markdown\n# A\n```")
        self.assertEqual((s, body), ("x", "# A\n"))
        s, body = parse_change("<<<FILE\n```md\n# B\n```\nFILE>>>")
        self.assertEqual(body, "# B\n")


class PRModeTests(Base):
    def test_issue_to_pull_request(self):
        self.state.add_issue("Arameair/identity", "Liaison evidence rule", ISSUE)
        self.assertEqual(self.agent.pulse(), 1)
        labels = self.state.label_names("Arameair/identity", 1)
        self.assertIn(DONE, labels)
        self.assertNotIn(WORKING, labels)
        self.assertEqual(len(self.state.prs), 1)
        pr = self.state.prs[0]
        self.assertEqual(pr["base"], "teams")
        self.assertTrue(pr["head"].startswith("liaison/identity-1-"))
        new = self.state.files["Arameair/identity"][pr["head"]]["teams/TEAM_LIAISON.md"]
        self.assertIn("request for evidence", new)
        self.assertNotIn("<think>", new)
        # The base branch itself is untouched in PR mode.
        self.assertEqual(self.state.files["Arameair/identity"]["teams"]["teams/TEAM_LIAISON.md"], CARD)
        c = self.comments()
        self.assertTrue(c[0].startswith("**WORKING**"))
        self.assertIn("**DONE**", c[-1])
        self.assertIn(pr["html_url"], c[-1])
        # Glob read picked up only md under cards/.
        self.assertIn("cards/Chief.md", c[-1])
        self.assertNotIn("code.go", c[-1])
        # Done issues are not picked up again.
        self.assertEqual(self.agent.pulse(), 0)

    def test_labels_created_on_first_pulse(self):
        self.assertEqual(self.agent.pulse(), 0)
        self.assertEqual(self.state.labels["Arameair/identity"],
                         {"liaison", WORKING, DONE, BLOCKED})

    def test_answer_only(self):
        self.state.add_issue("Arameair/identity", "Q", "read:\n- Arameair/TASS:cards/TASS.md\ntask: What does TASS require?")
        self.agent.pulse()
        self.assertIn("owner reviews changes", self.comments()[-1])
        self.assertEqual(self.state.prs, [])
        self.assertEqual(self.state.commits, [])

    def test_non_owner_ignored(self):
        self.state.add_issue("Arameair/identity", "evil", ISSUE, author="stranger")
        self.assertEqual(self.agent.pulse(), 0)
        self.assertEqual(self.state.comments, [])
        self.assertEqual(self.model_srv.calls, [])

    def test_write_outside_allow_list_blocked(self):
        body = "change:\n- Arameair/identity@main:README.md\ntask: rewrite readme"
        self.state.add_issue("Arameair/identity", "x", body)
        self.agent.pulse()
        self.assertIn(BLOCKED, self.state.label_names("Arameair/identity", 1))
        self.assertIn("not allowed", self.comments()[-1])
        self.assertEqual(self.state.commits, [])

    def test_read_outside_allow_list_blocked(self):
        self.state.files["Other/repo"] = {"main": {"a.md": "x"}}
        self.state.add_issue("Arameair/identity", "x", "read:\n- Other/repo:a.md\ntask: summarize")
        self.agent.pulse()
        self.assertIn(BLOCKED, self.state.label_names("Arameair/identity", 1))

    def test_dry_run_writes_nothing(self):
        self.state.add_issue("Arameair/identity", "x", ISSUE)
        self.agent.dry_run = True
        self.agent.pulse()
        self.assertEqual((self.state.comments, self.state.prs, self.state.commits), ([], [], []))
        self.assertTrue(any("+- A DONE report without evidence" in l for l in self.logs))


class DirectModeTests(Base):
    mode = "direct"

    def test_direct_commit(self):
        self.state.add_issue("Arameair/identity", "x", ISSUE)
        self.agent.pulse()
        self.assertIn("request for evidence", self.state.files["Arameair/identity"]["teams"]["teams/TEAM_LIAISON.md"])
        self.assertEqual(self.state.prs, [])

    def test_protected_branch_refused(self):
        self.state.add_file("Arameair/identity", "main", "teams/TEAM_LIAISON.md", CARD)
        body = "change:\n- Arameair/identity@main:teams/TEAM_LIAISON.md\ntask: add a rule"
        self.state.add_issue("Arameair/identity", "x", body)
        self.agent.pulse()
        self.assertIn("protected", self.comments()[-1])
        self.assertEqual(self.state.commits, [])


class TruncatedModelTests(Base):
    finish = "length"

    def test_truncation_blocks_write(self):
        self.state.add_issue("Arameair/identity", "x", ISSUE)
        self.agent.pulse()
        self.assertIn(BLOCKED, self.state.label_names("Arameair/identity", 1))
        self.assertIn("truncated", self.comments()[-1])
        self.assertEqual(self.state.commits, [])


class TinyContextTests(Base):
    n_ctx = 300

    def test_context_guard_blocks_before_model(self):
        self.state.add_issue("Arameair/identity", "x", ISSUE)
        self.agent.pulse()
        self.assertIn("Context guard BLOCKED", self.comments()[-1])
        self.assertEqual(self.model_srv.calls, [])


class ShrinkingModelTests(Base):
    reply = staticmethod(lambda m: "SUMMARY: oops\n<<<FILE\n# Team Liaison\nFILE>>>")

    def test_card_that_lost_sections_is_not_written(self):
        self.state.add_issue("Arameair/identity", "x", ISSUE)
        self.agent.pulse()
        self.assertIn(BLOCKED, self.state.label_names("Arameair/identity", 1))
        self.assertIn("less than half", self.comments()[-1])
        self.assertEqual(self.state.prs, [])


class NewCardTests(Base):
    def test_create_card(self):
        body = "change:\n- Arameair/identity@teams:teams/TEAM_SCRIBE.md\ntask: create a Scribe role card"
        self.state.add_issue("Arameair/identity", "x", body)
        self.agent.pulse()
        pr = self.state.prs[0]
        self.assertIn("teams/TEAM_SCRIBE.md", self.state.files["Arameair/identity"][pr["head"]])


if __name__ == "__main__":
    unittest.main()
