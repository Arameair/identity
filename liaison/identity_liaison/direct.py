"""Direct TASS from GitHub.

Issues labelled `tass` (config [direct] label) in the identity's repo are a line
to the running TASS. The Liaison forwards each new owner message (the issue
description, then every owner comment) into the workspace inbox:

    <inbox>/<repo>__<issue>__<comment-id>.json
      {"repo", "issue", "comment_id", "text", "url", "title", "at"}

The workspace has TASS think about it and puts its reply in the outbox as
{"type": "comment", ...}; the Liaison posts that back on the issue. Only owners'
messages are forwarded; the Liaison's own comments (marked) are skipped.
"""

from __future__ import annotations

import json
import os
import time
from pathlib import Path

from .config import Config
from .github import GitHub, GitHubError

MARK = "<!-- identity-liaison:tass -->"


class Direct:
    def __init__(self, cfg: Config, gh: GitHub, log=print):
        self.cfg, self.gh, self.log = cfg, gh, log
        d = cfg["direct"]
        self.repos, self.label = d["repos"], d["label"]
        self.inbox = Path(d["inbox"])
        self.seen_path = self.inbox / ".seen.json"
        self._labels_ready: set[str] = set()

    def _seen(self) -> dict:
        try:
            return json.loads(self.seen_path.read_text("utf-8"))
        except (OSError, ValueError):
            return {}

    def _write(self, name: str, data: dict) -> None:
        tmp = self.inbox / f".{name}.tmp"
        tmp.write_text(json.dumps(data))
        os.replace(tmp, self.inbox / name)

    def pulse(self) -> int:
        self.inbox.mkdir(parents=True, exist_ok=True)
        seen = self._seen()
        forwarded = 0
        for repo in self.repos:
            if repo not in self._labels_ready:
                self.gh.ensure_label(repo, self.label, "0e8a16", "Message for the running TASS (identity-liaison)")
                self._labels_ready.add(repo)
            try:
                issues = self.gh.list_issues(repo, self.label)
            except GitHubError as e:
                self.log(f"  direct {repo}: {e}")
                continue
            for issue in issues:
                if not self.cfg.is_owner(issue["user"]["login"]):
                    continue
                key = f"{repo}#{issue['number']}"
                done = set(seen.get(key, []))
                messages = []
                if "0" not in done and (issue.get("body") or "").strip():
                    messages.append(("0", f"{issue['title']}\n\n{issue['body']}".strip(), issue["html_url"]))
                elif "0" not in done:
                    messages.append(("0", issue["title"], issue["html_url"]))
                if issue.get("comments"):
                    for c in self.gh.list_comments(repo, issue["number"]):
                        cid = str(c["id"])
                        body = c.get("body") or ""
                        if cid in done or "<!-- identity-liaison" in body or not self.cfg.is_owner(c["user"]["login"]):
                            continue
                        messages.append((cid, body.strip(), c.get("html_url", issue["html_url"])))
                for cid, text, url in messages:
                    name = f"{repo.replace('/', '__')}__{issue['number']}__{cid}.json"
                    self._write(name, {"repo": repo, "issue": issue["number"], "comment_id": cid, "text": text,
                                       "url": url, "title": issue["title"], "at": time.time()})
                    done.add(cid)
                    forwarded += 1
                seen[key] = sorted(done)
        tmp = self.inbox / ".seen.tmp"
        tmp.write_text(json.dumps(seen))
        os.replace(tmp, self.seen_path)
        if forwarded:
            self.log(f"  direct: forwarded {forwarded} message(s) to TASS")
        return forwarded
