"""Outbox: owner-approved change requests from the workspace become pull requests.

The workspace has no GitHub token. When the owner approves a change there (for
example a TASS mind change), it writes  <outbox>/<id>.json :

    {"id": "...", "repo": "Arameair/TASS", "branch": "alpha", "path": "mind/how-i-think.md",
     "content": "...", "title": "...", "summary": "...", "approved_at": "..."}

On each fast pulse the Liaison checks [write] allow, opens a pull request
(never a direct commit), writes <id>.result.json with the PR link or the
error, and renames the request to <id>.done.json. The owner still merges.
"""

from __future__ import annotations

import json
import os
import re
import time
from pathlib import Path

from .config import Config
from .github import GitHub, GitHubError

ID = re.compile(r"^[A-Za-z0-9_-]{4,40}$")
MAX_CONTENT = 128 * 1024


class Outbox:
    def __init__(self, cfg: Config, gh: GitHub, log=print):
        self.cfg, self.gh, self.log = cfg, gh, log
        self.dir = Path(cfg["outbox"]["dir"])

    def _result(self, rid: str, **data) -> None:
        tmp = self.dir / f".{rid}.result.tmp"
        tmp.write_text(json.dumps({"id": rid, "at": time.time(), **data}))
        os.replace(tmp, self.dir / f"{rid}.result.json")

    def pulse(self) -> int:
        if not self.dir.is_dir():
            return 0
        done = 0
        for req_path in sorted(self.dir.glob("*.json")):
            name = req_path.name
            if name.endswith((".result.json", ".done.json")) or name.startswith("."):
                continue
            rid = name[:-5]
            if not ID.match(rid):
                continue
            try:
                req = json.loads(req_path.read_text("utf-8"))
                url = self.handle(rid, req)
                self._result(rid, ok=True, pr_url=url)
                self.log(f"  outbox {rid}: opened {url}")
            except (ValueError, KeyError, GitHubError, RuntimeError, OSError) as e:
                self._result(rid, ok=False, error=str(e))
                self.log(f"  outbox {rid}: {e}")
            os.replace(req_path, self.dir / f"{rid}.done.json")
            done += 1
        return done

    def handle(self, rid: str, req: dict) -> str:
        repo, path, content = req["repo"], req["path"].lstrip("/"), req["content"]
        if not isinstance(content, str) or len(content.encode()) > MAX_CONTENT:
            raise ValueError("content missing or too large")
        if ".." in path.split("/"):
            raise ValueError("bad path")
        base = req.get("branch") or self.gh.default_branch(repo)
        if not self.cfg.may_write(repo, base, path):
            raise ValueError(f"Writing {repo}@{base}:{path} is not allowed by [write] allow.")
        current = self.gh.read_file(repo, base, path)
        if current and current[0] == content:
            raise ValueError("No change: the file already has exactly this text.")
        branch = f"tass/{path.rsplit('/', 1)[-1].removesuffix('.md')}-{rid}"
        self.gh.create_branch(repo, branch, self.gh.branch_sha(repo, base))
        title = (req.get("title") or f"Update {path}")[:200]
        self.gh.put_file(repo, branch, path, content,
                         f"{title}\n\nProposed by TASS, approved by the owner in the workspace ({req.get('approved_at', '?')}).",
                         current[1] if current else None)
        body = (f"**Proposed by TASS in the Thought Processor workspace; approved there by the owner.**\n\n"
                f"{req.get('summary') or ''}\n\n- File: `{path}` on `{base}`\n- Workspace request: `{rid}` "
                f"(see the workspace audit log)\n\nMerging this changes how TASS thinks within about 30 seconds. "
                "Nothing here was tested; it's text for TASS's mind.")
        return self.gh.create_pr(repo, branch, base, title, body)["html_url"]
