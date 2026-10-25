"""TEST FIXTURE ONLY — a tiny local stand-in for github.com / api.github.com.

It answers the documented device-flow and REST shapes so the Identity UI can
be exercised without the publisher's real GitHub App. Anything produced with
it is fixture evidence, never a real sign-in. Identity labels it TEST FIXTURE
in every view.

  python3 -I scripts/fixture_github.py 8790
  identity-b -port 7808 -data <scratch profile> \
     -test-github-web http://127.0.0.1:8790 -test-github-api http://127.0.0.1:8790
  (with IDENTITY_GITHUB_CLIENT_ID=IvFIXTURE00000001)
"""
import json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8790
TOKEN = "ghu_FIXTUREonlyNOTrealTOKEN0000000000"
polls = {"n": 0}


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def send(self, code, obj):
        b = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
        if self.path == "/login/device/code":
            polls["n"] = 0
            return self.send(200, {"device_code": "fixture-device-code", "user_code": "FIXT-URE1",
                                   "verification_uri": f"http://127.0.0.1:{PORT}/login/device", "expires_in": 300, "interval": 5})
        if self.path == "/login/oauth/access_token":
            polls["n"] += 1
            if polls["n"] < 2:
                return self.send(200, {"error": "authorization_pending"})
            return self.send(200, {"access_token": TOKEN, "expires_in": 28800})
        self.send(404, {})

    def do_GET(self):
        authed = self.headers.get("Authorization") == "Bearer " + TOKEN
        if self.path == "/user":
            return self.send(200, {"login": "fixture-owner", "id": 4242, "name": "Fixture Owner (not real)", "type": "User",
                                   "html_url": "https://github.com/fixture-owner"}) if authed else self.send(401, {})
        if self.path.startswith("/user/installations"):
            return self.send(200, {"installations": [{"id": 1001, "app_id": 99, "app_slug": "fixture-publisher-app",
                                                      "repository_selection": "selected", "account": {"login": "Arameair"}}]})
        if self.path == "/repos/Arameair/Thought-Processor":
            return self.send(200, {"full_name": "Arameair/Thought-Processor", "private": True})
        if self.path == "/repos/Arameair/Thought-engine":
            return self.send(404, {"message": "Not Found"})
        if self.path == "/login/device":
            b = b"TEST FIXTURE device page (not GitHub)"
            self.send_response(200); self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
            return
        self.send(404, {})


ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
