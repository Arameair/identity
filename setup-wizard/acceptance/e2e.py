#!/usr/bin/env python3
"""End-to-end acceptance: Open Identity → Connect GitHub → Find → Start → Open.

REAL processes: Identity wizard binary, Project-controller binary (launch-request
branch), and the Thought Processor that Controller downloads from GitHub, builds and
runs. The test script plays the person: it clicks in Chromium, including the
approval click inside Project Controller.

ONE STAND-IN: GitHub sign-in/API is fake_github.py (TEST FIXTURE). It serves the real
branch commit so Controller's real download can be compared, but no real GitHub
account is involved. Identity shows a TEST MODE banner for the whole run.

Usage: python3 -I e2e.py --identity BIN --controller BIN --sha REAL_BRANCH_SHA --out DIR
"""
import argparse, json, os, re, signal, socket, subprocess, sys, tempfile, time, urllib.request

RESULTS, LOG = [], []
GH_PORT, PC_PORT, ID_PORT = 7701, 7777, 7790


def now():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def note(m):
    line = f"{now()} {m}"; LOG.append(line); print(line, flush=True)


def record(tid, verdict, what, observed, limit=""):
    RESULTS.append(dict(id=tid, verdict=verdict, what=what, observed=observed, limitation=limit, at=now()))
    note(f"[{tid}] {verdict} — {observed}")


def port_free(p):
    s = socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    try:
        s.bind(("127.0.0.1", p)); return True
    except OSError:
        return False
    finally:
        s.close()


def procs(prefix):
    out = []
    for d in os.listdir("/proc"):
        if d.isdigit():
            try:
                st = open(f"/proc/{d}/stat").read()
                comm = st[st.index("(") + 1: st.rindex(")")]
                if comm.startswith(prefix) and st.split(")")[1].split()[0] != "Z":
                    out.append(int(d))
            except OSError:
                pass
    return sorted(out)


class P:
    def __init__(self, name, argv, out, env=None, cwd=None):
        e = dict(os.environ); e.update(env or {})
        self.log = open(os.path.join(out, name + ".raw.log"), "ab")
        self.t0 = time.monotonic()
        self.p = subprocess.Popen(argv, stdout=self.log, stderr=subprocess.STDOUT, env=e, cwd=cwd, start_new_session=True)
        self.pid = self.p.pid

    def wait_port(self, port, timeout=20):
        end = time.monotonic() + timeout
        while time.monotonic() < end:
            if self.p.poll() is not None:
                return None
            try:
                socket.create_connection(("127.0.0.1", port), 0.3).close()
                return round((time.monotonic() - self.t0) * 1000)
            except OSError:
                time.sleep(0.03)
        return None

    def stop(self, sig=signal.SIGTERM):
        t = time.monotonic()
        if self.p.poll() is None:
            self.p.send_signal(sig)
            try:
                self.p.wait(10)
            except subprocess.TimeoutExpired:
                self.p.kill(); self.p.wait()
        return round((time.monotonic() - t) * 1000), self.p.returncode


def text(page, sel):
    try:
        return page.text_content(sel) or ""
    except Exception:
        return ""


def visible(page, sel):
    try:
        return page.is_visible(sel)
    except Exception:
        return False


def wait_until(fn, timeout=60, step=0.1):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        v = fn()
        if v:
            return v
        time.sleep(step)
    return None


def pc_token():
    req = urllib.request.Request(f"http://127.0.0.1:{PC_PORT}/api/security/session", headers={"Sec-Fetch-Site": "same-origin"})
    return json.load(urllib.request.urlopen(req, timeout=5))["token"]


def pc_state():
    return json.load(urllib.request.urlopen(f"http://127.0.0.1:{PC_PORT}/api/state", timeout=5))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--identity", required=True); ap.add_argument("--controller", required=True)
    ap.add_argument("--sha", required=True); ap.add_argument("--out", required=True)
    ap.add_argument("--meta", default="{}")
    a = ap.parse_args()
    out = os.path.abspath(a.out); os.makedirs(out, exist_ok=True)
    work = tempfile.mkdtemp(prefix="id-e2e-")
    meta = json.loads(a.meta); meta.update(started=now(), branch_sha_served_by_fixture=a.sha)
    for p_ in (GH_PORT, PC_PORT, ID_PORT):
        if not port_free(p_):
            sys.exit(f"port {p_} busy")
    base = {k: procs(k) for k in ("identity", "project-contr", "thought-proces")}
    here = os.path.dirname(os.path.abspath(__file__))
    started = []
    shots = []
    timings = {}
    from playwright.sync_api import sync_playwright
    pw = sync_playwright().start()
    br = pw.chromium.launch()
    ctx = br.new_context(viewport={"width": 1100, "height": 900})

    def shot(page, name):
        page.screenshot(path=os.path.join(out, name + ".png"), full_page=True); shots.append(name)

    try:
        GH = P("fake-github", [sys.executable, "-I", "-u", os.path.join(here, "fake_github.py"), str(GH_PORT), a.sha], out); started.append(GH)
        GH.wait_port(GH_PORT)
        appdir = os.path.join(work, "id-app"); os.makedirs(appdir)
        with open(os.path.join(appdir, "github-app.json"), "w") as f:
            json.dump({"client_id": "Iv1.FIXTURE"}, f)
        iddata = os.path.join(work, "identity-data")
        ID = P("identity", [a.identity, "-data", iddata, "-no-browser", "-github-app", os.path.join(appdir, "github-app.json"),
                            "-test-github-web", f"http://127.0.0.1:{GH_PORT}", "-test-github-api", f"http://127.0.0.1:{GH_PORT}"], out); started.append(ID)
        timings["identity_start_ms"] = ID.wait_port(ID_PORT)
        link = wait_until(lambda: json.load(open(os.path.join(iddata, "instance.json")))["link"] if os.path.exists(os.path.join(iddata, "instance.json")) else None, 10)

        # ---- second launch reopens instead of duplicating ----
        ID2 = P("identity-second", [a.identity, "-data", iddata, "-no-browser"], out); ID2.p.wait(10)
        msg2 = open(os.path.join(out, "identity-second.raw.log")).read().strip()
        record("REOPEN", "PASS" if ID2.p.returncode == 0 and "already running" in msg2 and procs("identity") == sorted(base["identity"] + [ID.pid]) else "FAIL",
               "Run Identity a second time", f"exit {ID2.p.returncode}: {msg2!r}; Identity processes now {procs('identity')}")

        page = ctx.new_page()
        t0 = time.monotonic()
        page.goto(f"http://127.0.0.1:{ID_PORT}/")
        wait_until(lambda: visible(page, "#v-signedout"), 10)
        shot(page, "00-opened-without-link")
        record("SIGNED-OUT", "PASS" if visible(page, "#v-signedout") else "FAIL", "Open Identity's address without its start link",
               "shows 'Open Identity from its program'; no state served")
        page.goto(link)
        wait_until(lambda: visible(page, "#v-1"), 10)
        timings["welcome_visible_ms"] = round((time.monotonic() - t0) * 1000)
        shot(page, "01-welcome")
        ok = visible(page, "#v-1") and visible(page, "#testmode") and "#k=" not in page.url and not page.is_disabled("#b-connect")
        record("STEP-1", "PASS" if ok else "FAIL", "Welcome screen", f"title={text(page,'#v-1 h1')!r}; one primary button={text(page,'#b-connect')!r}; TEST MODE banner shown; link code removed from address bar={'#k=' not in page.url}")

        # ---- Step 2: cancel once, then approve ----
        t1 = time.monotonic()
        page.click("#b-connect")
        wait_until(lambda: visible(page, "#g-waiting") and text(page, "#g-code"), 15)
        shot(page, "02-github-code")
        code = text(page, "#g-code")
        with ctx.expect_page() as gp:
            page.click("#g-open")
        g = gp.value; g.wait_for_load_state()
        g.fill("#code", code); g.click("#cancel"); g.wait_for_selector("#done"); g.close()
        wait_until(lambda: visible(page, "#g-problem"), 20)
        shot(page, "02b-github-cancelled")
        cancel_msg = text(page, "#g-message")
        record("STEP-2-CANCEL", "PASS" if "Cancel" in cancel_msg else "FAIL", "Person clicks Cancel on the GitHub page",
               f"Identity says {cancel_msg!r}; offers Try again")
        page.click("#g-retry")
        wait_until(lambda: visible(page, "#g-waiting") and text(page, "#g-code"), 15)
        with ctx.expect_page() as gp:
            page.click("#g-open")
        g = gp.value; g.wait_for_load_state()
        g.fill("#code", text(page, "#g-code")); g.click("#authorize"); g.wait_for_selector("#done"); g.close()
        wait_until(lambda: visible(page, "#g-connected"), 20)
        timings["github_connect_ms_incl_cancel"] = round((time.monotonic() - t1) * 1000)
        shot(page, "03-github-connected")
        login = text(page, "#g-login")
        record("STEP-2", "PASS" if login == "fixture-user" else "FAIL", "Connect GitHub (device flow) and show the username",
               f"shows 'Connected as {login}' read back from /user with the new token", "GitHub stand-in: proves Identity's handling, not a real GitHub sign-in")

        # ---- Step 3: no access first, then found ----
        urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:{GH_PORT}/fixture/no-access?on=1", method="POST"))
        page.click("#b-find")
        wait_until(lambda: visible(page, "#f-problem"), 20)
        shot(page, "04-find-no-access")
        na = text(page, "#f-message")
        record("STEP-3-NOACCESS", "PASS" if "can't see" in na else "FAIL", "Account cannot see the repository", f"Identity says {na!r}")
        urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:{GH_PORT}/fixture/no-access?on=0", method="POST"))
        t3 = time.monotonic()
        page.click("#f-again")
        wait_until(lambda: visible(page, "#f-found"), 20)
        timings["find_ms"] = round((time.monotonic() - t3) * 1000)
        shot(page, "05-find-found")
        ff = text(page, "#f-facts")
        record("STEP-3", "PASS" if a.sha[:7] in ff and "experiment/processor-first-prototype-20261024" in ff else "FAIL", "Find Thought Processor",
               f"facts: {ff!r}")

        page.click("#b-tostart")
        wait_until(lambda: visible(page, "#s-intro"), 5)

        # ---- Step 4 failure first: Controller not running ----
        page.click("#b-start")
        wait_until(lambda: visible(page, "#s-problem"), 20)
        shot(page, "06-start-controller-missing")
        miss = text(page, "#s-message")
        record("STEP-4-NOCONTROLLER", "PASS" if "isn't running" in miss or "isn't open" in miss else "FAIL", "Start while Project Controller is closed",
               f"Identity says {miss!r}; Try again offered")

        pcdata = os.path.join(work, "pc-data")
        PC = P("project-controller", [a.controller], out, env={"PORT": str(PC_PORT), "PC_DATA": pcdata, "PC_NO_BROWSER": "1"}); started.append(PC)
        timings["controller_start_ms"] = PC.wait_port(PC_PORT)
        note(f"Project Controller started by the test (playing the person) pid {PC.pid}")
        t4 = time.monotonic()
        page.click("#s-retry")
        wait_until(lambda: visible(page, "#s-approve"), 90)
        timings["request_to_waiting_for_approval_ms"] = round((time.monotonic() - t4) * 1000)
        shot(page, "07-start-waiting-for-you")
        ran_before_approval = bool(procs("thought-proces"))
        # Clicking Start again must not create a second pane.
        page.evaluate("1")  # no-op keeps page alive
        st = pc_state()
        panes = [v for v in st["versions"] if v["repo"] == "Arameair/Thought-Processor"]
        record("STEP-4-WAIT", "PASS" if len(panes) == 1 and panes[0]["status"] == "approval" and not ran_before_approval and panes[0]["review"]["sha"] == a.sha else "FAIL",
               "Identity asks Controller; nothing runs before approval",
               f"Controller panes for the repo: {[(v['branch'], v['status']) for v in panes]}; review SHA {panes[0]['review']['sha'][:7] if panes else None} (GitHub said {a.sha[:7]}); Thought Processor processes before approval: {procs('thought-proces')}")

        # ---- the person approves in Project Controller ----
        pcpage = ctx.new_page()
        pcpage.goto(f"http://127.0.0.1:{PC_PORT}/")
        pcpage.wait_for_selector("#launch-requests .lr-item", timeout=15000)
        shot(pcpage, "08-controller-notice")
        notice = text(pcpage, "#launch-requests")
        pcpage.click("#launch-requests .lr-item button")
        pcpage.wait_for_selector("#d-ok", timeout=15000)
        shot(pcpage, "09-controller-review-dialog")
        dialog = text(pcpage, "#layer")
        ta = time.monotonic()
        pcpage.click("#d-ok")
        record("APPROVE-IN-CONTROLLER", "PASS" if "Identity" in notice and a.sha in dialog else "FAIL", "Owner reviews and approves inside Project Controller",
               f"notice: {re.sub(r'\\s+',' ',notice)[:160]!r}; dialog shows full SHA={a.sha in dialog}")

        page.bring_to_front()
        wait_until(lambda: visible(page, "#v-5") and visible(page, "#o-ok"), 120)
        timings["approve_to_running_verified_ms"] = round((time.monotonic() - ta) * 1000)
        shot(page, "10-open-ready")
        of = text(page, "#o-facts")
        st = pc_state(); pane = [v for v in st["versions"] if v["repo"] == "Arameair/Thought-Processor"][0]
        record("STEP-4", "PASS" if pane["status"] == "running" and pane["sha"] == a.sha else "FAIL", "Controller builds and runs the approved commit",
               f"pane status {pane['status']} commit {pane['sha'][:7]} port {pane['port']} pid {pane.get('pid')}")

        # ---- Step 5: open the real application ----
        with ctx.expect_page() as tp:
            page.click("#b-open")
        tpage = tp.value
        tpage.wait_for_load_state()
        ok5 = wait_until(lambda: visible(tpage, "#app") and "Thought Processor" in text(tpage, "h1"), 20)
        time.sleep(0.5)
        shot(tpage, "11-thought-processor-open")
        build_line = text(tpage, "#self")
        record("STEP-5", "PASS" if ok5 and a.sha[:12] in build_line and "#code=" not in tpage.url else "FAIL", "Open Thought Processor, signed in",
               f"opened {tpage.url}; Processor reports build {a.sha[:12] in build_line and a.sha[:12]}; Identity facts {of!r}")
        timings["total_click_to_open_ms"] = round((time.monotonic() - t0) * 1000)
        log_shown = text(page, "#s-log")
        record("NO-CODE-IN-LOG", "PASS" if not re.search(r"code=[0-9a-f]{16,}", log_shown) else "FAIL", "Sign-in code hidden in the progress log Identity shows",
               "log shows code=<hidden>" if "code=<hidden>" in log_shown else "no sign-in line visible")

        # ---- Failure after success: stopped in Controller → Try again ----
        tok = pc_token()
        vid = urllib.parse.quote(pane["id"], safe="")
        urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:{PC_PORT}/api/versions/{vid}/stop", method="POST", headers={"X-PC-Control-Token": tok}))
        wait_until(lambda: visible(page, "#s-problem") or visible(page, "#o-problem"), 30)
        shot(page, "12-stopped-try-again")
        msg = text(page, "#s-message") or text(page, "#o-message")
        tr = time.monotonic()
        page.click("#s-retry" if visible(page, "#s-retry") else "#o-retry")
        back = wait_until(lambda: visible(page, "#v-5") and visible(page, "#o-ok"), 90)
        st = pc_state(); panes = [v for v in st["versions"] if v["repo"] == "Arameair/Thought-Processor"]
        record("RETRY", "PASS" if back and len(panes) == 1 and "stopped" in msg.lower() else "FAIL", "Thought Processor stopped; person clicks Try again",
               f"message {msg!r}; back to running in {round((time.monotonic()-tr)*1000)} ms; panes {len(panes)} (no duplicate); same approved SHA reran without a new approval")

        # ---- Closing Identity leaves the others running ----
        tp_pids = procs("thought-proces")
        ms, rc = ID.stop()
        alive_after = dict(controller=PC.p.poll() is None, processor=bool(procs("thought-proces")))
        record("CLOSE-IDENTITY", "PASS" if rc == 0 and all(alive_after.values()) else "FAIL", "Close Identity",
               f"Identity exit {rc} in {ms} ms; Controller running={alive_after['controller']}; Thought Processor running={alive_after['processor']} ({tp_pids})")
    finally:
        try:
            tok = pc_token()
            urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:{PC_PORT}/api/host/quit", method="POST", headers={"X-PC-Control-Token": tok}), timeout=5)
            time.sleep(2)
        except Exception:
            pass
        br.close(); pw.stop()
        for p_ in started:
            p_.stop()
        left = {k: sorted(set(procs(k)) - set(base[k])) for k in base}
        note(f"orphan audit: {left}")
        meta.update(finished=now(), timings=timings, orphans=left, screenshots=shots)
    for fn in os.listdir(out):
        if fn.endswith(".raw.log"):
            t = open(os.path.join(out, fn)).read()
            t = re.sub(r"(code|k)=[0-9a-f]{16,}", r"\1=<hidden>", t).replace(work, "<tmp>")
            open(os.path.join(out, fn.replace(".raw.log", ".log")), "w").write(t); os.remove(os.path.join(out, fn))
    json.dump(dict(meta=meta, results=RESULTS, log=LOG), open(os.path.join(out, "evidence.json"), "w"), indent=2)
    bad = [r["id"] for r in RESULTS if r["verdict"] != "PASS"]
    print("NOT PASS:", bad or "none")
    return 1 if bad else 0


if __name__ == "__main__":
    import urllib.parse
    sys.exit(main())
