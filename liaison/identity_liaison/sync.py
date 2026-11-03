"""Mirror selected repo files into a local folder.

Used by the Thought Processor workspace: the Liaison (the only container with
the GitHub token) copies `mind/` and `library/` from a repo into a shared
volume, and the workspace reads them from there. Edit a file on GitHub and the
copy updates on the next pulse.
"""

from __future__ import annotations

import base64
import fnmatch
import json
import os
import time
from pathlib import Path

from .config import Config
from .github import GitHub

MAX_FILE_BYTES = 256 * 1024
MAX_FILES = 300


def _atomic_write(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    tmp.write_bytes(data)
    os.replace(tmp, path)


class Mirror:
    def __init__(self, cfg: Config, gh: GitHub, log=print):
        self.cfg, self.gh, self.log = cfg, gh, log
        s = cfg["sync"]
        self.repo = s["repo"]
        self.branch = s.get("branch") or ""
        self.patterns = s["paths"]
        self.dest = Path(s["dest"])
        self.manifest_path = self.dest / "manifest.json"

    def _manifest(self) -> dict:
        try:
            return json.loads(self.manifest_path.read_text("utf-8"))
        except (OSError, ValueError):
            return {"files": {}}

    def pulse(self) -> list[str]:
        """Sync once. Returns the paths that changed (added, updated or removed)."""
        if not self.cfg.may_read(self.repo):
            raise RuntimeError(f"Mirroring {self.repo} is not allowed by [read] allow.")
        branch = self.branch or self.gh.default_branch(self.repo)
        tree = self.gh.request("GET", f"/repos/{self.repo}/git/trees/{branch}", params={"recursive": "1"})
        wanted = {
            item["path"]: item for item in tree.get("tree", [])
            if item.get("type") == "blob" and any(fnmatch.fnmatchcase(item["path"], p) for p in self.patterns)
        }
        if len(wanted) > MAX_FILES:
            raise RuntimeError(f"{len(wanted)} files match; the mirror limit is {MAX_FILES}.")
        old = self._manifest()
        files = dict(old.get("files", {}))
        changed: list[str] = []
        for path, item in sorted(wanted.items()):
            if files.get(path, {}).get("sha") == item["sha"] and (self.dest / "files" / path).exists():
                continue
            if item.get("size", 0) > MAX_FILE_BYTES:
                self.log(f"  sync skip {path}: larger than {MAX_FILE_BYTES // 1024} KB")
                continue
            blob = self.gh.request("GET", f"/repos/{self.repo}/git/blobs/{item['sha']}")
            data = base64.b64decode("".join(blob["content"].split()))
            _atomic_write(self.dest / "files" / path, data)
            files[path] = {"sha": item["sha"], "size": len(data), "updated_at": time.time()}
            changed.append(path)
        for path in list(files):
            if path not in wanted:
                try:
                    (self.dest / "files" / path).unlink()
                except FileNotFoundError:
                    pass
                del files[path]
                changed.append(path)
        manifest = {
            "repo": self.repo, "branch": branch, "patterns": self.patterns,
            "synced_at": time.time(), "files": files,
            "last_change_at": time.time() if changed else old.get("last_change_at"),
            "last_changed": changed or old.get("last_changed", []),
        }
        _atomic_write(self.manifest_path, json.dumps(manifest, indent=1).encode("utf-8"))
        if changed:
            self.log(f"  sync {self.repo}@{branch}: {len(changed)} changed ({', '.join(changed[:5])})")
        return changed
