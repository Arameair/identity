"""In-memory fake GitHub API and fake llama.cpp server for offline tests.

These are fixtures: passing tests here are NOT evidence of real GitHub or a real model.
"""

from __future__ import annotations

import base64
import hashlib
import json
import re
import threading
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def _sha(text: str) -> str:
    return hashlib.sha1(text.encode()).hexdigest()


class FakeGitHubState:
    def __init__(self, login="Arameair"):
        self.login = login
        # repo -> branch -> {path: text}
        self.files: dict[str, dict[str, dict[str, str]]] = {}
        self.issues: dict[str, list[dict]] = {}
        self.comments: list[tuple[str, int, str]] = []
        self.prs: list[dict] = []
        self.labels: dict[str, set[str]] = {}
        self.commits: list[tuple[str, str, str]] = []
        self.threads: dict[tuple[str, int], list[dict]] = {}

    def add_comment(self, repo, n, body, author=None):
        self.comment_seq = getattr(self, "comment_seq", 1000) + 1
        self.threads.setdefault((repo, n), []).append({"id": self.comment_seq, "user": {"login": author or self.login},
                                                       "body": body,
                                                       "html_url": f"https://github.com/{repo}/issues/{n}#c{self.comment_seq}"})

    def add_file(self, repo, branch, path, text):
        self.files.setdefault(repo, {}).setdefault(branch, {})[path] = text

    def add_issue(self, repo, title, body, author="Arameair", labels=("liaison",)):
        lst = self.issues.setdefault(repo, [])
        n = len(lst) + 1
        lst.append({"number": n, "title": title, "body": body, "user": {"login": author},
                    "labels": [{"name": l} for l in labels],
                    "html_url": f"https://github.com/{repo}/issues/{n}"})
        return n

    def issue(self, repo, n):
        return self.issues[repo][n - 1]

    def label_names(self, repo, n):
        return {l["name"] for l in self.issue(repo, n)["labels"]}


def start_fake_github(state: FakeGitHubState):
    class H(BaseHTTPRequestHandler):
        def log_message(self, *a):
            pass

        def _send(self, code, obj=None):
            b = b"" if obj is None else json.dumps(obj).encode()
            self.send_response(code)
            self.send_header("Content-Length", str(len(b)))
            self.end_headers()
            self.wfile.write(b)

        def _body(self):
            n = int(self.headers.get("Content-Length") or 0)
            return json.loads(self.rfile.read(n)) if n else None

        def _route(self, method):
            u = urllib.parse.urlparse(self.path)
            q = dict(urllib.parse.parse_qsl(u.query))
            p = urllib.parse.unquote(u.path)
            body = self._body() if method in ("POST", "PUT", "PATCH") else None
            if p == "/user":
                return self._send(200, {"login": state.login})
            m = re.match(r"^/repos/([^/]+/[^/]+)(.*)$", p)
            if not m:
                return self._send(404, {"message": "Not Found"})
            repo, rest = m.groups()
            if repo not in state.files and repo not in state.issues:
                return self._send(404, {"message": "Not Found"})
            branches = state.files.setdefault(repo, {})
            if rest == "" and method == "GET":
                return self._send(200, {"default_branch": "main", "full_name": repo})
            m = re.match(r"^/contents/(.+)$", rest)
            if m:
                path = m.group(1)
                if method == "GET":
                    ref = q.get("ref", "main")
                    text = branches.get(ref, {}).get(path)
                    if text is None:
                        return self._send(404, {"message": "Not Found"})
                    return self._send(200, {"type": "file", "sha": _sha(text),
                                            "content": base64.b64encode(text.encode()).decode()})
                if method == "PUT":
                    br = body["branch"]
                    if br not in branches:
                        return self._send(404, {"message": "Branch not found"})
                    cur = branches[br].get(path)
                    if cur is not None and body.get("sha") != _sha(cur):
                        return self._send(409, {"message": "sha mismatch"})
                    branches[br][path] = base64.b64decode(body["content"]).decode()
                    state.commits.append((repo, br, path))
                    return self._send(200, {"content": {"path": path}})
            m = re.match(r"^/git/trees/(.+)$", rest)
            if m:
                ref = m.group(1)
                return self._send(200, {"tree": [{"path": k, "type": "blob", "sha": _sha(v), "size": len(v.encode())}
                                                 for k, v in branches.get(ref, {}).items()]})
            m = re.match(r"^/git/blobs/(\w+)$", rest)
            if m:
                for files in branches.values():
                    for v in files.values():
                        if _sha(v) == m.group(1):
                            state.blob_reads = getattr(state, "blob_reads", 0) + 1
                            return self._send(200, {"content": base64.b64encode(v.encode()).decode()})
                return self._send(404, {"message": "Not Found"})
            m = re.match(r"^/git/ref/heads/(.+)$", rest)
            if m:
                if m.group(1) not in branches:
                    return self._send(404, {"message": "Not Found"})
                return self._send(200, {"object": {"sha": "base-" + m.group(1)}})
            if rest == "/git/refs" and method == "POST":
                name = body["ref"].removeprefix("refs/heads/")
                if name in branches:
                    return self._send(422, {"message": "exists"})
                src = body["sha"].removeprefix("base-")
                branches[name] = dict(branches.get(src, {}))
                return self._send(201, {})
            if rest == "/pulls" and method == "POST":
                pr = dict(body, number=len(state.prs) + 1,
                          html_url=f"https://github.com/{repo}/pull/{len(state.prs) + 1}", repo=repo)
                state.prs.append(pr)
                return self._send(201, pr)
            if rest == "/labels" and method == "POST":
                s = state.labels.setdefault(repo, set())
                if body["name"] in s:
                    return self._send(422, {"message": "exists"})
                s.add(body["name"])
                return self._send(201, {})
            if rest == "/issues" and method == "GET":
                want = q.get("labels")
                out = [dict(i, comments=len(state.threads.get((repo, i["number"]), [])))
                       for i in state.issues.get(repo, [])
                       if not want or want in {l["name"] for l in i["labels"]}]
                return self._send(200, out)
            if rest == "/issues" and method == "POST":
                n = state.add_issue(repo, body["title"], body["body"], state.login, body.get("labels", []))
                return self._send(201, state.issue(repo, n))
            m = re.match(r"^/issues/(\d+)/(comments|labels)(?:/(.+))?$", rest)
            if m:
                n, kind, label = int(m.group(1)), m.group(2), m.group(3)
                issue = state.issue(repo, n)
                if kind == "comments" and method == "GET":
                    return self._send(200, state.threads.get((repo, n), []))
                if kind == "comments":
                    state.comments.append((repo, n, body["body"]))
                    state.add_comment(repo, n, body["body"])
                    return self._send(201, {})
                if method == "POST":
                    for name in body["labels"]:
                        if name not in state.label_names(repo, n):
                            issue["labels"].append({"name": name})
                    return self._send(200, [])
                if method == "DELETE":
                    before = len(issue["labels"])
                    issue["labels"] = [l for l in issue["labels"] if l["name"] != label]
                    return self._send(200 if len(issue["labels"]) < before else 404, [])
            return self._send(404, {"message": f"fake: no route {method} {p}"})

        def do_GET(self):
            self._route("GET")

        def do_POST(self):
            self._route("POST")

        def do_PUT(self):
            self._route("PUT")

        def do_DELETE(self):
            self._route("DELETE")

    srv = ThreadingHTTPServer(("127.0.0.1", 0), H)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv


def start_fake_model(reply_fn, n_ctx=8192, finish_reason="stop"):
    """reply_fn(messages) -> str. Implements the llama.cpp endpoints the client uses."""
    calls = []

    class H(BaseHTTPRequestHandler):
        def log_message(self, *a):
            pass

        def _send(self, obj):
            b = json.dumps(obj).encode()
            self.send_response(200)
            self.send_header("Content-Length", str(len(b)))
            self.end_headers()
            self.wfile.write(b)

        def do_GET(self):
            if self.path == "/health":
                return self._send({"status": "ok"})
            if self.path == "/props":
                return self._send({"default_generation_settings": {"n_ctx": n_ctx}})
            self.send_response(404)
            self.end_headers()

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            if self.path == "/apply-template":
                return self._send({"prompt": "\n".join(m["content"] for m in body["messages"])})
            if self.path == "/tokenize":
                return self._send({"tokens": list(range(len(body["content"]) // 4 + 1))})
            if self.path == "/v1/chat/completions":
                calls.append(body)
                return self._send({"choices": [{"finish_reason": finish_reason,
                                                "message": {"content": reply_fn(body["messages"])}}]})
            self.send_response(404)
            self.end_headers()

    srv = ThreadingHTTPServer(("127.0.0.1", 0), H)
    srv.calls = calls
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv
