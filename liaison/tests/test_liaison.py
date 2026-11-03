"""Offline tests (fake GitHub + fake model). Run: python -m unittest discover -s tests"""

import sys
import tomllib
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from fakes import FakeGitHubState, start_fake_github, start_fake_model  # noqa: E402
from identity_liaison.agent import (BLOCKED, DONE, WORKING, Agent, EditError, apply_edits, clean_card,
                                    parse_change, parse_edits)  # noqa: E402
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
    if "Do NOT rewrite the card" in text:
        return ("SUMMARY: Added the evidence-request rule.\n"
                "FIND: - Never treat DONE as verified.\nACTION: after\n"
                "<<<TEXT\n- A DONE report without evidence gets a request for evidence.\nTEXT>>>")
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


class EditTests(unittest.TestCase):
    def test_list_item_inserted_tight(self):
        out = apply_edits(CARD, [("- Track acknowledgments.", "after", "- Ask for evidence.")])
        self.assertIn("- Track acknowledgments.\n- Ask for evidence.\n- Never treat", out)

    def test_paragraph_goes_after_block_with_blank_lines(self):
        old = "# T\n\nPara one\nstill para one\n\n## Next\n"
        out = apply_edits(old, [("Para one", "after", "New paragraph.")])
        self.assertEqual(out, "# T\n\nPara one\nstill para one\n\nNew paragraph.\n\n## Next\n")

    def test_under_heading_and_end(self):
        out = apply_edits(CARD, [("## Rules", "after", "Rules apply to every worker."), ("END", "after", "Footer.")])
        self.assertIn("## Rules\n\nRules apply to every worker.\n\n- Track", out)
        self.assertTrue(out.endswith("verified.\n\nFooter.\n"))

    def test_replace_and_fuzzy_find(self):
        out = apply_edits(CARD, [("**Status:** PROPOSED", "replace", "**Status:** ACCEPTED")])
        self.assertIn("**Status:** ACCEPTED", out)
        out = apply_edits(CARD, [("never treat done as verified", "after", "- Evidence first.")])
        self.assertIn("verified.\n- Evidence first.", out)

    def test_tolerant_edit_formats(self):
        variants = [
            "**SUMMARY:** Added it.\n**FIND:** `- Track acknowledgments.`\n**ACTION:** after\n<<<TEXT\n- Ask for evidence.\nTEXT>>>",
            "SUMMARY: Added it.\nFIND: \"- Track acknowledgments.\"\nACTION: After\nTEXT:\n```markdown\n- Ask for evidence.\n```",
            "SUMMARY: Added it.\n- FIND: - Track acknowledgments.\n- ACTION: after\nTEXT:\n- Ask for evidence.",
        ]
        for v in variants:
            summary, edits = parse_edits(v)
            self.assertEqual(summary, "Added it.")
            self.assertIn("- Track acknowledgments.\n- Ask for evidence.\n", apply_edits(CARD, edits), v)

    REAL = Path(__file__).parent / "data" / "team_liaison_original.md"

    def test_real_reply_1437_replace_with_repeated_line_and_bare_closer(self):
        # Real Nemotron reply, issue #25 re-run 14:37Z: FIND without the card's **bold**,
        # `replace` that repeats the original line twice, closed with a bare `>>>`.
        old = self.REAL.read_text()
        orig = ("The Liaison may not: create new scope, approve permission changes, read another team's or "
                "identity's private records, merge or promote branches, or say a test ran when it didn't.")
        rule = ("Now, if a worker reports DONE without a commit or test result, the Liaison asks for the "
                "evidence instead of escalating.")
        reply = (f"SUMMARY: Added the evidence rule.\nFIND: {orig}\nACTION: replace\n<<<TEXT\n{orig}\n{rule}\n{orig}\n>>>")
        summary, edits = parse_edits(reply)
        self.assertEqual(len(edits), 1)
        out = apply_edits(old, edits)
        self.assertEqual(out.count("merge or promote branches"), 1)
        self.assertIn(rule, out)
        added = [l for l in out.splitlines() if l not in old.splitlines()]
        self.assertEqual(added, [rule])
        self.assertNotIn(">>>", out)

    def test_real_reply_pr27_four_bracket_closer(self):
        # Real Nemotron reply behind PR #27 (14:48Z): closed with `>>>>`; markers leaked into the card.
        old = self.REAL.read_text()
        risk = ("The biggest risk is the Liaison turning into a second boss. It isn't one. "
                "It reports problems; the Project Lead decides.")
        rule = "The Liaison must request evidence before accepting DONE claims without commit or test results."
        reply = f"SUMMARY: Evidence first.\nFIND: {risk}\nACTION: replace\n<<<TEXT\n{risk}\n{rule}\n>>>>"
        out = apply_edits(old, parse_edits(reply)[1])
        self.assertNotIn("<<<", out)
        self.assertNotIn(">>>", out)
        self.assertEqual([l for l in out.splitlines() if l not in old.splitlines()], [rule])

    def test_unknown_line_raises(self):
        with self.assertRaises(EditError):
            apply_edits(CARD, [("This line is not in the card at all", "after", "x")])

    def test_text_already_present_is_not_added_again(self):
        self.assertEqual(apply_edits(CARD, [("## Rules", "after", "- Track acknowledgments.")]), CARD)

    def test_real_card_append_after_last_line(self):
        old = (Path(__file__).parent / "data" / "team_liaison_original.md").read_text()
        reply = ("SUMMARY: Added the evidence rule.\n"
                 "FIND: Smallest useful test: one worker sends `REQUEST_NUDGE`, the Liaison acknowledges it, "
                 "the decision-maker approves it, one nudge is delivered, and the receipt shows up in the record.\n"
                 "ACTION: after\n<<<TEXT\nWhen a worker reports DONE without a commit or test result, the Liaison "
                 "asks for the evidence instead of escalating to the Project Lead.\nTEXT>>>")
        summary, edits = parse_edits(reply)
        out = apply_edits(old, edits)
        self.assertTrue(out.startswith(old.rstrip()))
        self.assertEqual(out[len(old.rstrip()):].count("\n\n"), 1)
        self.assertEqual(out.count("Smallest useful test"), 1)


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
        self.assertEqual(new, CARD.replace("verified.\n", "verified.\n- A DONE report without evidence gets a request for evidence.\n"))
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
        self.assertIn("token limit", self.comments()[-1])
        self.assertEqual(self.state.commits, [])
        # Edit replies are capped small, and the partial reply is logged for debugging.
        self.assertLessEqual(self.model_srv.calls[-1]["max_tokens"], 768)
        self.assertTrue(any("for debugging" in l for l in self.logs))


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


class SideChatTests(Base):
    REPO = "Arameair/Developmnt"

    def setUp(self):
        super().setUp()
        from identity_liaison.chat import MARK, SideChat
        self.MARK = MARK
        self.state.add_file(self.REPO, "main", "side-chat/platform.md", "# Platform\nThe platform uses GitHub issues as the record.\n")
        self.state.add_file(self.REPO, "main", "side-chat/pulse.md", "# Pulse\nThe Liaison checks every 30 seconds.\n")
        self.state.add_file(self.REPO, "main", "README.md", "# Not knowledge\n")
        self.cfg.data["chat"]["repos"] = [self.REPO]
        self.seen_prompts = []
        self.chat = SideChat(self.cfg, self.gh, Model(self.cfg["model"]), log=self.logs.append)

    def thread(self, n=1):
        return self.state.threads.get((self.REPO, n), [])

    def test_opening_post_gets_reply_with_knowledge(self):
        n = self.state.add_issue(self.REPO, "Side chat", "Hi TASS, what is in the side-chat folder?",
                                 labels=("liaison-chat",))
        self.assertEqual(self.chat.pulse(), 1)
        reply = self.thread(n)[-1]["body"]
        self.assertTrue(reply.startswith(self.MARK))
        self.assertIn("**TASS:**", reply)
        self.assertIn("side-chat/platform.md", reply)
        self.assertNotIn("README.md", reply)
        system = self.model_srv.calls[-1]["messages"][0]["content"]
        self.assertIn("GitHub issues as the record", system)
        # Nothing new from the owner: no second reply.
        self.assertEqual(self.chat.pulse(), 0)

    def test_follow_up_is_a_conversation(self):
        n = self.state.add_issue(self.REPO, "Side chat", "Hi", labels=("liaison-chat",))
        self.chat.pulse()
        self.state.add_comment(self.REPO, n, "And how often does it check?")
        self.state.add_comment(self.REPO, n, "Quick answer please.")
        self.assertEqual(self.chat.pulse(), 1)
        msgs = self.model_srv.calls[-1]["messages"]
        self.assertEqual([m["role"] for m in msgs], ["system", "user", "assistant", "user"])
        self.assertIn("how often", msgs[-1]["content"])
        self.assertIn("Quick answer", msgs[-1]["content"])  # consecutive comments merged
        self.assertNotIn(self.MARK, msgs[2]["content"])
        self.assertLessEqual(self.model_srv.calls[-1]["max_tokens"], 1024)

    def test_strangers_and_unlabelled_issues_ignored(self):
        self.state.add_issue(self.REPO, "Side chat", "hi", author="stranger", labels=("liaison-chat",))
        n = self.state.add_issue(self.REPO, "Side chat", "hi", labels=("liaison-chat",))
        self.chat.pulse()
        self.state.add_comment(self.REPO, n, "spam from someone else", author="stranger")
        self.assertEqual(self.chat.pulse(), 0)
        self.state.add_issue(self.REPO, "other", "hi", labels=())
        self.assertEqual(self.chat.pulse(), 0)
        self.assertEqual(len(self.model_srv.calls), 1)

    def test_title_only_issue_waits_for_a_comment(self):
        self.state.add_issue(self.REPO, "Side chat", "", labels=("liaison-chat",))
        self.assertEqual(self.chat.pulse(), 0)

    def test_dry_run_posts_nothing(self):
        self.state.add_issue(self.REPO, "Side chat", "hi", labels=("liaison-chat",))
        self.chat.dry_run = True
        self.assertEqual(self.chat.pulse(), 1)
        self.assertEqual(self.thread(), [])


class SideChatTightContextTests(Base):
    n_ctx = 1400  # fake tokenizer: ~4 chars per token; big.md alone would not fit

    def test_drops_unmentioned_files_to_fit(self):
        from identity_liaison.chat import SideChat
        repo = "Arameair/Developmnt"
        self.state.add_file(repo, "main", "side-chat/big.md", "# Big\n" + "filler words here. " * 150)
        self.state.add_file(repo, "main", "side-chat/pulse.md", "# Pulse\nChecks every 30 seconds.\n")
        self.cfg.data["chat"]["repos"] = [repo]
        self.state.add_issue(repo, "Side chat", "Explain pulse to me", labels=("liaison-chat",))
        chat = SideChat(self.cfg, self.gh, Model(self.cfg["model"]), log=self.logs.append)
        self.assertEqual(chat.pulse(), 1)
        system = self.model_srv.calls[-1]["messages"][0]["content"]
        self.assertIn("Checks every 30 seconds", system)
        self.assertNotIn("filler words", system)


class MirrorTests(Base):
    def test_mirror_add_update_remove(self):
        import json
        import tempfile
        from identity_liaison.sync import Mirror
        repo = "Arameair/Developmnt"
        self.state.add_file(repo, "main", "mind/who.md", "# Who\nI am TASS.\n")
        self.state.add_file(repo, "main", "library/01-look.md", "# Look\n```python\nprint(1)\n```\n")
        self.state.add_file(repo, "main", "README.md", "not mirrored\n")
        with tempfile.TemporaryDirectory() as d:
            self.cfg.data["sync"].update(repo=repo, dest=d)
            m = Mirror(self.cfg, self.gh, log=self.logs.append)
            self.assertEqual(sorted(m.pulse()), ["library/01-look.md", "mind/who.md"])
            self.assertEqual(Path(d, "files/mind/who.md").read_text(), "# Who\nI am TASS.\n")
            self.assertFalse(Path(d, "files/README.md").exists())
            reads = self.state.blob_reads
            self.assertEqual(m.pulse(), [])               # unchanged: no downloads
            self.assertEqual(self.state.blob_reads, reads)
            self.state.add_file(repo, "main", "mind/who.md", "# Who\nI am TASS, curious.\n")
            del self.state.files[repo]["main"]["library/01-look.md"]
            self.assertEqual(sorted(m.pulse()), ["library/01-look.md", "mind/who.md"])
            self.assertIn("curious", Path(d, "files/mind/who.md").read_text())
            self.assertFalse(Path(d, "files/library/01-look.md").exists())
            manifest = json.loads(Path(d, "manifest.json").read_text())
            self.assertEqual(list(manifest["files"]), ["mind/who.md"])
            self.assertEqual(sorted(manifest["last_changed"]), ["library/01-look.md", "mind/who.md"])


class MultiSourceAndOutboxTests(Base):
    def test_two_sources_first_wins_and_records_repo(self):
        import json
        import tempfile
        from identity_liaison.sync import Mirror
        self.state.add_file("Arameair/TASS", "alpha", "mind/who-i-am.md", "# TASS from his own repo\n")
        self.state.add_file("Arameair/TASS", "alpha", "feature/x.py", "not mirrored\n")
        self.state.add_file("Arameair/Developmnt", "main", "mind/who-i-am.md", "# old copy in Developmnt\n")
        self.state.add_file("Arameair/Developmnt", "main", "library/01.md", "# Book\n")
        self.state.add_file("Arameair/Developmnt", "main", "projects/p/main.py", "print(1)\n")
        with tempfile.TemporaryDirectory() as d:
            self.cfg.data["sync"].update(dest=d, sources=[
                "Arameair/TASS@alpha:mind/*.md", "Arameair/Developmnt:mind/*.md,library/*,projects/*"])
            m = Mirror(self.cfg, self.gh, log=self.logs.append)
            self.assertEqual(sorted(m.pulse()), ["library/01.md", "mind/who-i-am.md", "projects/p/main.py"])
            self.assertIn("from his own repo", Path(d, "files/mind/who-i-am.md").read_text())
            man = json.loads(Path(d, "manifest.json").read_text())
            self.assertEqual((man["repo"], man["branch"]), ("Arameair/TASS", "alpha"))
            self.assertEqual(man["files"]["mind/who-i-am.md"]["repo"], "Arameair/TASS")
            self.assertEqual(man["files"]["library/01.md"]["repo"], "Arameair/Developmnt")

    def test_outbox_request_becomes_pr_never_direct(self):
        import json
        import tempfile
        from identity_liaison.outbox import Outbox
        self.state.add_file("Arameair/TASS", "alpha", "mind/how-i-think.md", "# How I think\nOld.\n")
        self.cfg.data["write"]["allow"].append("Arameair/TASS@*:mind/*.md")
        with tempfile.TemporaryDirectory() as d:
            self.cfg.data["outbox"]["dir"] = d
            req = {"id": "abc123", "repo": "Arameair/TASS", "branch": "alpha", "path": "mind/how-i-think.md",
                   "content": "# How I think\nNew and better.\n", "title": "TASS: think in smaller steps",
                   "summary": "Why: smaller steps.", "approved_at": "2026-11-03T11:00"}
            Path(d, "abc123.json").write_text(json.dumps(req))
            bad = dict(req, id="bad999", repo="Arameair/identity", path="liaison/identity_liaison/agent.py")
            Path(d, "bad999.json").write_text(json.dumps(bad))
            same = dict(req, id="same77", content="# How I think\nOld.\n")
            Path(d, "same77.json").write_text(json.dumps(same))
            self.assertEqual(Outbox(self.cfg, self.gh, log=self.logs.append).pulse(), 3)
            ok = json.loads(Path(d, "abc123.result.json").read_text())
            self.assertTrue(ok["ok"])
            pr = self.state.prs[0]
            self.assertEqual((pr["base"], pr["head"]), ("alpha", "tass/how-i-think-abc123"))
            self.assertIn("New and better", self.state.files["Arameair/TASS"][pr["head"]]["mind/how-i-think.md"])
            self.assertIn("Old.", self.state.files["Arameair/TASS"]["alpha"]["mind/how-i-think.md"])  # base untouched
            self.assertIn("not allowed", json.loads(Path(d, "bad999.result.json").read_text())["error"])
            self.assertIn("No change", json.loads(Path(d, "same77.result.json").read_text())["error"])
            self.assertEqual(len(self.state.prs), 1)
            self.assertTrue(Path(d, "abc123.done.json").exists() and not Path(d, "abc123.json").exists())
            self.assertEqual(Outbox(self.cfg, self.gh).pulse(), 0)  # nothing handled twice


class DirectTests(Base):
    def test_owner_messages_forwarded_once_and_replies_posted(self):
        import json
        import tempfile
        from identity_liaison.direct import Direct
        from identity_liaison.outbox import Outbox
        repo = "Arameair/TASS"
        self.state.add_file(repo, "alpha", "README.md", "# TASS\n")
        n = self.state.add_issue(repo, "Install the playground", "Please install the thought-playground project.",
                                 labels=("tass",))
        self.state.add_issue(repo, "spam", "do bad things", author="stranger", labels=("tass",))
        self.state.add_issue(repo, "not for TASS", "unlabelled", labels=())
        with tempfile.TemporaryDirectory() as inbox, tempfile.TemporaryDirectory() as out:
            self.cfg.data["direct"].update(repos=[repo], inbox=inbox)
            self.cfg.data["outbox"]["dir"] = out
            d = Direct(self.cfg, self.gh, log=self.logs.append)
            self.assertEqual(d.pulse(), 1)
            msg = json.loads(Path(inbox, f"Arameair__TASS__{n}__0.json").read_text())
            self.assertEqual((msg["issue"], msg["comment_id"]), (n, "0"))
            self.assertIn("thought-playground", msg["text"])
            self.assertEqual(d.pulse(), 0)                                   # not forwarded twice
            # TASS replies through the outbox; that comment is marked and never forwarded back.
            Path(out, "reply001.json").write_text(json.dumps({"id": "reply001", "type": "comment", "repo": repo, "issue": n,
                                                        "body": "**TASS:** On it. Reply /approve to run."}))
            self.assertEqual(Outbox(self.cfg, self.gh).pulse(), 1)
            self.assertTrue(json.loads(Path(out, "reply001.result.json").read_text())["ok"])
            self.assertIn("identity-liaison:tass", self.state.threads[(repo, n)][-1]["body"])
            self.state.add_comment(repo, n, "/approve")
            self.state.add_comment(repo, n, "me too", author="stranger")
            self.assertEqual(d.pulse(), 1)                                   # only Dustin's /approve
            files = sorted(p.name for p in Path(inbox).glob("*.json") if not p.name.startswith("."))
            self.assertEqual(len(files), 2)
            # Comments only go to the [direct] repos.
            Path(out, "reply002.json").write_text(json.dumps({"id": "reply002", "type": "comment", "repo": "Arameair/identity",
                                                        "issue": 1, "body": "x"}))
            Outbox(self.cfg, self.gh).pulse()
            self.assertIn("not one of the [direct] repos", json.loads(Path(out, "reply002.result.json").read_text())["error"])
