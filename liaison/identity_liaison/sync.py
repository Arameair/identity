"""Mirror selected repo files into a local folder.

Used by the Thought Processor workspace: the Liaison (the only container with
the GitHub token) copies files from one or more repos into a shared volume,
and the workspace reads them from there. Edit a file on GitHub and the copy
updates on the next pulse.

Sources (config [sync] sources, or IDENTITY_LIAISON_SYNC_SOURCES separated by ';'):
    "Arameair/TASS@alpha:mind/*.md"                  the identity's own repo (listed first)
    "Arameair/Developmnt:library/*,projects/*"       shared team content
A file path keeps its repo path (mind/who-i-am.md). If two sources provide the
same path, the first source wins.
"""

from __future__ import annotations

import base64
import fnmatch
import json
import os
import re
import time
from dataclasses import dataclass
from pathlib import Path

from .config import Config
from .github import GitHub

MAX_FILE_BYTES = 256 * 1024
MAX_FILES = 300
SOURCE = re.compile(r"^(?P<repo>[\w.-]+/[\w.-]+)(?:@(?P<branch>[^:\s]+))?:(?P<patterns>\S+)$")


@dataclass
class Source:
    repo: str
    branch: str
    patterns: list[str]

    @classmethod
    def parse(cls, text: str) -> "Source":
        m = SOURCE.match(text.strip())
        if not m:
            raise ValueError(f"Bad sync source `{text}`; use owner/repo[@branch]:pattern,pattern")
        return cls(m["repo"], m["branch"] or "", [p for p in m["patterns"].split(",") if p])

    def __str__(self) -> str:
        return f"{self.repo}{'@' + self.branch if self.branch else ''}:{','.join(self.patterns)}"


def sources_from(cfg: Config) -> list[Source]:
    s = cfg["sync"]
    if s.get("sources"):
        return [Source.parse(x) for x in s["sources"]]
    if s.get("repo"):
        return [Source(s["repo"], s.get("branch") or "", list(s["paths"]))]
    return []


def _atomic_write(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    tmp.write_bytes(data)
    os.replace(tmp, path)


class Mirror:
    def __init__(self, cfg: Config, gh: GitHub, log=print):
        self.cfg, self.gh, self.log = cfg, gh, log
        self.sources = sources_from(cfg)
        self.dest = Path(cfg["sync"]["dest"])
        self.manifest_path = self.dest / "manifest.json"

    def _manifest(self) -> dict:
        try:
            return json.loads(self.manifest_path.read_text("utf-8"))
        except (OSError, ValueError):
            return {"files": {}}

    def pulse(self) -> list[str]:
        """Sync once. Returns the paths that changed (added, updated or removed)."""
        wanted: dict[str, tuple[Source, str, dict]] = {}
        for src in self.sources:
            if not self.cfg.may_read(src.repo):
                raise RuntimeError(f"Mirroring {src.repo} is not allowed by [read] allow.")
            branch = src.branch or self.gh.default_branch(src.repo)
            tree = self.gh.request("GET", f"/repos/{src.repo}/git/trees/{branch}", params={"recursive": "1"})
            for item in tree.get("tree", []):
                if item.get("type") == "blob" and any(fnmatch.fnmatchcase(item["path"], p) for p in src.patterns):
                    if item["path"] not in wanted:          # first source wins
                        wanted[item["path"]] = (src, branch, item)
        if len(wanted) > MAX_FILES:
            raise RuntimeError(f"{len(wanted)} files match; the mirror limit is {MAX_FILES}.")
        old = self._manifest()
        files = dict(old.get("files", {}))
        changed: list[str] = []
        for path, (src, branch, item) in sorted(wanted.items()):
            same = files.get(path, {}).get("sha") == item["sha"] and files[path].get("repo", src.repo) == src.repo
            if same and (self.dest / "files" / path).exists():
                continue
            if item.get("size", 0) > MAX_FILE_BYTES:
                self.log(f"  sync skip {path}: larger than {MAX_FILE_BYTES // 1024} KB")
                continue
            blob = self.gh.request("GET", f"/repos/{src.repo}/git/blobs/{item['sha']}")
            data = base64.b64decode("".join(blob["content"].split()))
            _atomic_write(self.dest / "files" / path, data)
            files[path] = {"sha": item["sha"], "size": len(data), "updated_at": time.time(),
                           "repo": src.repo, "branch": branch}
            changed.append(path)
        for path in list(files):
            if path not in wanted:
                try:
                    (self.dest / "files" / path).unlink()
                except FileNotFoundError:
                    pass
                del files[path]
                changed.append(path)
        first = self.sources[0] if self.sources else None
        manifest = {
            "repo": first.repo if first else None,
            "branch": (first.branch or (self.gh.default_branch(first.repo) if first else "")) if first else None,
            "sources": [str(s) for s in self.sources],
            "patterns": [p for s in self.sources for p in s.patterns],
            "synced_at": time.time(), "files": files,
            "last_change_at": time.time() if changed else old.get("last_change_at"),
            "last_changed": changed or old.get("last_changed", []),
        }
        _atomic_write(self.manifest_path, json.dumps(manifest, indent=1).encode("utf-8"))
        if changed:
            self.log(f"  sync: {len(changed)} changed ({', '.join(changed[:5])})")
        return changed
