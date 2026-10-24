#!/usr/bin/env python3
"""TEST FIXTURE — a stand-in for github.com / api.github.com.

It speaks the device-flow and the few REST calls Identity uses, and shows a
small "Authorize / Cancel" page so a browser test can play the person. It is
NOT GitHub; any result obtained through it proves Identity's handling only.

Usage: python3 -I fake_github.py PORT REAL_BRANCH_SHA
Control: POST /fixture/no-access?on=1|0
"""
import json, sys, threading, urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT, SHA = int(sys.argv[1]), sys.argv[2]
REPO, BRANCH = "Arameair/Thought-Processor", "experiment/processor-first-prototype-20261024"
state = {"decision": None, "no_access": False, "code": "WDJB-MJHT", "polls": 0}
lock = threading.Lock()
TOKEN = "gho_FIXTURE_NOT_REAL"


class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *a):
        sys.stdout.write("fake-github " + (fmt % a) + "\n"); sys.stdout.flush()

    def send(self, code, body, ctype="application/json"):
        b = body.encode() if isinstance(body, str) else json.dumps(body).encode()
        self.send_response(code); self.send_header("Content-Type", ctype); self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)

    def form(self):
        n = int(self.headers.get("Content-Length") or 0)
        return dict(urllib.parse.parse_qsl(self.rfile.read(n).decode()))

    def do_POST(self):
        p = urllib.parse.urlparse(self.path)
        if p.path == "/login/device/code":
            f = self.form()
            if not f.get("client_id"):
                return self.send(400, {"error": "unauthorized_client"})
            with lock:
                state["decision"] = None
            return self.send(200, {"device_code": "dc-fixture", "user_code": state["code"], "verification_uri": f"http://127.0.0.1:{PORT}/login/device",
                                   "expires_in": 900, "interval": 1})
        if p.path == "/login/oauth/access_token":
            with lock:
                d = state["decision"]; state["polls"] += 1
            if d == "approve":
                return self.send(200, {"access_token": TOKEN, "token_type": "bearer"})
            if d == "cancel":
                return self.send(200, {"error": "access_denied"})
            return self.send(200, {"error": "authorization_pending"})
        if p.path == "/login/device/decide":
            f = self.form()
            with lock:
                ok = f.get("code", "").strip().upper() == state["code"]
                state["decision"] = f.get("choice") if (ok or f.get("choice") == "cancel") else None
            return self.send(200, "<!doctype html><title>Fixture</title><h1 id='done'>" + ("Done — return to Identity" if ok or f.get('choice') == 'cancel' else "Wrong code") + "</h1>", "text/html")
        if p.path == "/fixture/no-access":
            with lock:
                state["no_access"] = urllib.parse.parse_qs(p.query).get("on", ["1"])[0] == "1"
            return self.send(200, {"no_access": state["no_access"]})
        self.send(404, {})

    def do_GET(self):
        p = urllib.parse.urlparse(self.path)
        auth = self.headers.get("Authorization") == "Bearer " + TOKEN
        if p.path == "/login/device":
            return self.send(200, """<!doctype html><title>TEST GitHub stand-in</title>
<p style="background:#fbf0d6;padding:8px"><b>TEST FIXTURE</b> — this is not GitHub.</p>
<h1>Device activation</h1><form method="post" action="/login/device/decide">
<label>Code <input name="code" id="code"></label>
<button name="choice" value="approve" id="authorize">Authorize</button>
<button name="choice" value="cancel" id="cancel">Cancel</button></form>""", "text/html")
        if p.path == "/user":
            return self.send(200, {"login": "fixture-user", "id": 42}) if auth else self.send(401, {})
        if p.path == f"/repos/{REPO}":
            if not auth or state["no_access"]:
                return self.send(404, {"message": "Not Found"})
            return self.send(200, {"full_name": REPO, "private": True})
        if p.path == f"/repos/{REPO}/branches/" + urllib.parse.quote(BRANCH, safe=""):
            if not auth or state["no_access"]:
                return self.send(404, {})
            return self.send(200, {"name": BRANCH, "commit": {"sha": SHA, "commit": {"message": "real branch head (served by fixture)", "committer": {"date": "2026-10-24T09:36:15Z"}}}})
        self.send(404, {})


ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
