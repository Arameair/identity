"""Minimal GitHub REST client (standard library only)."""

from __future__ import annotations

import base64
import fnmatch
import json
import os
import shutil
import subprocess
import urllib.error
import urllib.parse
import urllib.request


class GitHubError(RuntimeError):
    def __init__(self, status: int, message: str):
        super().__init__(f"GitHub {status}: {message}")
        self.status = status


def find_token(configured: str = "") -> str:
    if configured:
        return configured
    for name in ("GITHUB_TOKEN", "GH_TOKEN"):
        if os.environ.get(name):
            return os.environ[name]
    if shutil.which("gh"):
        p = subprocess.run(["gh", "auth", "token"], capture_output=True, text=True, timeout=20)
        if p.returncode == 0 and p.stdout.strip():
            return p.stdout.strip()
    raise RuntimeError(
        "No GitHub token. Run `gh auth login`, or set GITHUB_TOKEN, "
        "or put a token in the [github] section of the config."
    )


class GitHub:
    def __init__(self, token: str, api_url: str = "https://api.github.com"):
        self.token = token
        self.api = api_url.rstrip("/")
        self._default_branch: dict[str, str] = {}

    # --- transport ----------------------------------------------------------

    def request(self, method: str, path: str, body=None, params: dict | None = None):
        url = self.api + path
        if params:
            url += "?" + urllib.parse.urlencode(params)
        data = None if body is None else json.dumps(body).encode("utf-8")
        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Accept", "application/vnd.github+json")
        req.add_header("X-GitHub-Api-Version", "2022-11-28")
        req.add_header("User-Agent", "identity-liaison")
        if self.token:
            req.add_header("Authorization", f"Bearer {self.token}")
        if data is not None:
            req.add_header("Content-Type", "application/json")
        try:
            with urllib.request.urlopen(req, timeout=60) as r:
                raw = r.read()
        except urllib.error.HTTPError as e:
            text = e.read().decode("utf-8", "replace")
            try:
                text = json.loads(text).get("message", text)
            except ValueError:
                pass
            raise GitHubError(e.code, text[:300]) from None
        return json.loads(raw) if raw else None

    # --- identity / repos -----------------------------------------------------

    def whoami(self) -> str:
        return self.request("GET", "/user")["login"]

    def default_branch(self, repo: str) -> str:
        if repo not in self._default_branch:
            self._default_branch[repo] = self.request("GET", f"/repos/{repo}")["default_branch"]
        return self._default_branch[repo]

    # --- files ----------------------------------------------------------------

    def read_file(self, repo: str, ref: str, path: str) -> tuple[str, str] | None:
        """Return (text, blob_sha) or None when the file does not exist."""
        quoted = urllib.parse.quote(path, safe="/")
        try:
            data = self.request("GET", f"/repos/{repo}/contents/{quoted}", params={"ref": ref})
        except GitHubError as e:
            if e.status == 404:
                return None
            raise
        if isinstance(data, list) or data.get("type") != "file":
            raise RuntimeError(f"{repo}@{ref}:{path} is not a file")
        text = base64.b64decode("".join(data["content"].split())).decode("utf-8")
        return text, data["sha"]

    def list_paths(self, repo: str, ref: str, pattern: str) -> list[str]:
        tree = self.request(
            "GET", f"/repos/{repo}/git/trees/{urllib.parse.quote(ref, safe='')}",
            params={"recursive": "1"},
        )
        return sorted(
            item["path"] for item in tree.get("tree", [])
            if item.get("type") == "blob" and fnmatch.fnmatchcase(item["path"], pattern)
        )

    def put_file(self, repo: str, branch: str, path: str, text: str, message: str,
                 sha: str | None) -> dict:
        body = {
            "message": message,
            "content": base64.b64encode(text.encode("utf-8")).decode("ascii"),
            "branch": branch,
        }
        if sha:
            body["sha"] = sha
        quoted = urllib.parse.quote(path, safe="/")
        return self.request("PUT", f"/repos/{repo}/contents/{quoted}", body)

    # --- branches / PRs -------------------------------------------------------

    def branch_sha(self, repo: str, branch: str) -> str:
        ref = urllib.parse.quote(f"heads/{branch}", safe="/")
        return self.request("GET", f"/repos/{repo}/git/ref/{ref}")["object"]["sha"]

    def create_branch(self, repo: str, name: str, from_sha: str) -> None:
        self.request("POST", f"/repos/{repo}/git/refs", {"ref": f"refs/heads/{name}", "sha": from_sha})

    def create_pr(self, repo: str, head: str, base: str, title: str, body: str) -> dict:
        return self.request("POST", f"/repos/{repo}/pulls",
                            {"head": head, "base": base, "title": title, "body": body})

    # --- issues / labels ------------------------------------------------------

    def list_issues(self, repo: str, label: str) -> list[dict]:
        items = self.request("GET", f"/repos/{repo}/issues",
                             params={"labels": label, "state": "open", "per_page": "50",
                                     "sort": "created", "direction": "asc"})
        return [i for i in items if "pull_request" not in i]

    def comment(self, repo: str, number: int, body: str) -> dict:
        return self.request("POST", f"/repos/{repo}/issues/{number}/comments", {"body": body})

    def add_labels(self, repo: str, number: int, labels: list[str]) -> None:
        self.request("POST", f"/repos/{repo}/issues/{number}/labels", {"labels": labels})

    def remove_label(self, repo: str, number: int, label: str) -> None:
        try:
            self.request("DELETE", f"/repos/{repo}/issues/{number}/labels/{urllib.parse.quote(label, safe='')}")
        except GitHubError as e:
            if e.status != 404:
                raise

    def ensure_label(self, repo: str, name: str, color: str, description: str) -> None:
        try:
            self.request("POST", f"/repos/{repo}/labels",
                         {"name": name, "color": color, "description": description})
        except GitHubError as e:
            if e.status != 422:  # 422 = already exists
                raise

    def create_issue(self, repo: str, title: str, body: str, labels: list[str]) -> dict:
        return self.request("POST", f"/repos/{repo}/issues",
                            {"title": title, "body": body, "labels": labels})
