"""TEST FIXTURE ONLY — a local stand-in for github.com / api.github.com that
can sign in as different accounts and serves a Thought-engine release.

Nothing produced with it is a real sign-in or a real release. Identity labels
fixture mode TEST FIXTURE everywhere.

  python3 -I scripts/fixture_github_engine.py PORT ASSET_FILE PLATFORM
    PLATFORM e.g. linux-amd64 (asset served as thought-engine-<PLATFORM>[.exe])

Control (fixture only):
  POST /fixture/login?user=NAME     next device sign-in becomes NAME
                                    (only "Arameair" can see Arameair/Thought-engine)
  POST /fixture/mode?k=v&...        asset=good|tamper|cut   (tamper: bytes differ from SHA256SUMS;
                                                              cut: connection drops halfway)
                                    uploader=NAME           (default github-actions[bot])
                                    release=yes|no          (no: repository has no release)
                                    tag=engine-v0.1.0-fixture
"""
import hashlib, json, sys, threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse, parse_qs

PORT, ASSET, PLATFORM = int(sys.argv[1]), sys.argv[2], sys.argv[3]
NAME = "thought-engine-" + PLATFORM + (".exe" if PLATFORM.startswith("windows") else "")
DATA = open(ASSET, "rb").read()
SUMS = (hashlib.sha256(DATA).hexdigest() + "  " + NAME + "\n").encode()
state = {"next": "Arameair", "polls": 0, "asset": "good", "uploader": "github-actions[bot]",
         "release": "yes", "tag": "engine-v0.1.0-fixture", "downloads": 0}
lock = threading.Lock()
ALLOWED = {"Arameair"}
IDS = {"Arameair": 1001}


class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *a):
        pass

    def send(self, code, obj=None, raw=None, ctype="application/json", headers=None):
        b = raw if raw is not None else json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(b)))
        for k, v in (headers or {}).items():
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(b)

    def who(self):
        a = self.headers.get("Authorization", "")
        return a[len("Bearer ghu_FIXTURE_"):] if a.startswith("Bearer ghu_FIXTURE_") else None

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0) or 0))
        u = urlparse(self.path)
        q = {k: v[0] for k, v in parse_qs(u.query).items()}
        if u.path == "/fixture/login":
            with lock:
                state["next"] = q["user"]
            return self.send(200, {"next_login": state["next"]})
        if u.path == "/fixture/mode":
            with lock:
                for k in ("asset", "uploader", "release", "tag"):
                    if k in q:
                        state[k] = q[k]
                snap = dict(state)
            return self.send(200, snap)
        if u.path == "/login/device/code":
            with lock:
                state["polls"] = 0
            return self.send(200, {"device_code": "fixture-device", "user_code": "FIXT-URE2",
                                   "verification_uri": f"http://127.0.0.1:{PORT}/login/device", "expires_in": 300, "interval": 5})
        if u.path == "/login/oauth/access_token":
            with lock:
                state["polls"] += 1
                if state["polls"] < 2:
                    return self.send(200, {"error": "authorization_pending"})
                return self.send(200, {"access_token": "ghu_FIXTURE_" + state["next"], "expires_in": 28800})
        self.send(404, {})

    def do_GET(self):
        u = urlparse(self.path)
        p, who = u.path, self.who()
        if p == "/login/device":
            return self.send(200, raw=b"TEST FIXTURE device page (not GitHub)", ctype="text/plain")
        if p == "/fixture/state":
            with lock:
                return self.send(200, dict(state))
        if p.startswith("/storage/"):
            if p.endswith("/12"):
                return self.send(200, raw=SUMS, ctype="application/octet-stream")
            with lock:
                mode = state["asset"]
                state["downloads"] += 1
            if mode == "tamper":
                data = bytes([DATA[0] ^ 0xFF]) + DATA[1:]
                return self.send(200, raw=data, ctype="application/octet-stream")
            if mode == "cut":
                self.send_response(200)
                self.send_header("Content-Type", "application/octet-stream")
                self.send_header("Content-Length", str(len(DATA)))
                self.end_headers()
                self.wfile.write(DATA[: len(DATA) // 2])
                self.wfile.flush()
                self.close_connection = True
                return
            return self.send(200, raw=DATA, ctype="application/octet-stream")
        if who is None:
            return self.send(401, {"message": "Bad credentials"})
        if p == "/user":
            return self.send(200, {"login": who, "id": IDS.get(who, 2002), "name": who + " (fixture)", "type": "User", "html_url": "https://github.com/" + who})
        if p.startswith("/user/installations/5/repositories"):
            return self.send(200, {"repositories": [{"full_name": "Arameair/Thought-Processor"}, {"full_name": "Arameair/Thought-engine"}]})
        if p.startswith("/user/installations"):
            return self.send(200, {"installations": [{"id": 5, "app_id": 1, "app_slug": "identity-fixture-app", "repository_selection": "selected", "account": {"login": "Arameair"}}]})
        if p == "/repos/Arameair/Thought-Processor":
            return self.send(200, {"full_name": "Arameair/Thought-Processor", "private": False})
        allowed = who in ALLOWED
        if p == "/repos/Arameair/Thought-engine":
            return self.send(200, {"full_name": "Arameair/Thought-engine", "private": True}) if allowed else self.send(404, {"message": "Not Found"})
        if p == "/repos/Arameair/Thought-engine/releases/latest":
            with lock:
                st = dict(state)
            if not allowed or st["release"] != "yes":
                return self.send(404, {"message": "Not Found"})
            return self.send(200, {"tag_name": st["tag"], "name": "TEST FIXTURE release", "published_at": "2026-10-26T12:00:00Z",
                                   "draft": False, "prerelease": False, "author": {"login": "github-actions[bot]"},
                                   "html_url": f"http://127.0.0.1:{PORT}/fixture-release",
                                   "assets": [{"id": 11, "name": NAME, "size": len(DATA), "uploader": {"login": st["uploader"]}},
                                              {"id": 12, "name": "SHA256SUMS", "size": len(SUMS), "uploader": {"login": "github-actions[bot]"}}]})
        if p.startswith("/repos/Arameair/Thought-engine/releases/assets/"):
            if not allowed or self.headers.get("Accept") != "application/octet-stream":
                return self.send(404, {"message": "Not Found"})
            aid = p.rsplit("/", 1)[1]
            # like GitHub: redirect to storage (Go drops the token on cross-host redirects)
            self.send_response(302)
            self.send_header("Location", f"http://localhost:{PORT}/storage/{aid}")
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self.send(404, {"message": "Not Found"})


ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
