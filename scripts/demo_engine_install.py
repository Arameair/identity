"""End-to-end demo (developer evidence): REAL Identity + REAL Thought Processor
binaries, with a TEST FIXTURE GitHub (scripts/fixture_github_engine.py)
serving a Thought-engine release. Drives both pages in headless Chromium.

  python3 -I scripts/demo_engine_install.py --identity-profile P --processor-profile Q \
      --fixture http://127.0.0.1:8791 --out DIR
Never prints the private #key= / #code= links or tokens.
"""
import argparse, json, os, urllib.request
from playwright.sync_api import sync_playwright

ap = argparse.ArgumentParser()
ap.add_argument("--identity-profile", required=True)
ap.add_argument("--processor-profile", required=True)
ap.add_argument("--fixture", required=True)
ap.add_argument("--out", required=True)
ap.add_argument("--phase", default="all", choices=["all", "denied", "restart"])
args = ap.parse_args()
os.makedirs(args.out, exist_ok=True)


def fixture_login(user):
    urllib.request.urlopen(urllib.request.Request(args.fixture + "/fixture/login?user=" + user, method="POST", data=b""))


def shot(page, name):
    page.screenshot(path=os.path.join(args.out, name + ".png"), full_page=True)
    print("saved", name)


def id_link():
    return open(os.path.join(args.identity_profile, "runtime", "open-link.txt")).read().strip()


def tp_link():
    code = open(os.path.join(args.processor_profile, "runtime", "operator-code")).read().strip()
    return "http://127.0.0.1:7860/#code=" + code


def github_connect(idp):
    idp.click("#manage-btn")
    idp.click("#manage .tabs [data-tab='github']")
    if idp.locator("[data-act='/api/github/disconnect']").count():
        idp.click("[data-act='/api/github/disconnect']")
        idp.wait_for_timeout(500)
    with idp.context.expect_page() as newtab:
        idp.click("#gh-connect")
    newtab.value.close()
    idp.wait_for_selector("#manage .account", timeout=20000)
    idp.wait_for_timeout(500)


with sync_playwright() as pw:
    b = pw.chromium.launch()
    ctx = b.new_context(viewport={"width": 1280, "height": 900}, permissions=["clipboard-read", "clipboard-write"])
    idp = ctx.new_page()
    idp.goto(id_link())
    idp.wait_for_selector("#shell:not([hidden])")
    idp.wait_for_timeout(500)
    tp = ctx.new_page()
    tp.goto(tp_link())
    tp.wait_for_selector("#app:not([hidden])")

    if args.phase == "all":
        # Identity first-use: GitHub (fixture, as Arameair)
        fixture_login("Arameair")
        idp.bring_to_front()
        idp.click(".step.on [data-go='1']")
        idp.wait_for_timeout(300)
        shot(idp, "e01-identity-github-step-explained")
        with ctx.expect_page() as newtab:
            idp.click("#gh-connect")
        newtab.value.close()
        idp.wait_for_selector(".step.on .account", timeout=20000)
        idp.click(".step.on [data-go='2']")
        idp.wait_for_timeout(500)
        shot(idp, "e02-identity-features-install-allowed")
        idp.click(".step.on [data-go='3']")
        idp.click("#tp-save")
        idp.wait_for_timeout(800)
        idp.click("[data-act='/api/pairing/code']")
        idp.wait_for_timeout(500)
        code = idp.inner_text("#tp-body .code")
        shot(idp, "e03-identity-pairing-code")

        # Processor: Identity address, pair, check, install
        tp.bring_to_front()
        tp.fill("#identity-addr", "127.0.0.1:7790")
        tp.click("#id-form button[type=submit]")
        tp.wait_for_timeout(1200)
        shot(tp, "e04-processor-found-not-paired")
        tp.fill("#pair-code", code)
        tp.click("#pair-form button[type=submit]")
        tp.wait_for_timeout(1500)
        shot(tp, "e05-processor-paired-allowed")
        tp.click("#inst-actions button.primary")
        for _ in range(150):  # page CSP forbids eval-based waits; poll text instead
            if tp.inner_text("#inst-pill") == "Installed":
                break
            tp.wait_for_timeout(200)
        tp.wait_for_timeout(500)
        shot(tp, "e06-processor-installed")
        print("installed facts:", tp.inner_text("#inst-facts").replace("\n", " | ")[:400])

        idp.bring_to_front()
        idp.click(".step.on [data-go='4']")
        idp.click("#finish-btn")
        idp.wait_for_timeout(1000)
        shot(idp, "e07-identity-monitor-after-install")
        idp.click("#copy-btn")
        idp.wait_for_timeout(800)
        open(os.path.join(args.out, "identity-diagnostics.txt"), "w").write(idp.evaluate("navigator.clipboard.readText()"))

    if args.phase in ("all", "denied"):
        # Second account that can't see Thought-engine
        fixture_login("brother-account")
        idp.bring_to_front()
        github_connect(idp)
        idp.click("#manage-close")
        tp.bring_to_front()
        tp.reload()
        tp.wait_for_selector("#app:not([hidden])")
        tp.wait_for_timeout(800)
        tp.locator("#inst-actions button:has-text('Check')").first.click()
        tp.wait_for_timeout(1500)
        shot(tp, "e08-processor-installed-but-account-denied")
        print("installed+denied:", tp.inner_text("#inst-status"), "|", tp.inner_text("#inst-next"))
        tp.once("dialog", lambda d: d.accept())
        tp.locator("#inst-actions button:has-text('Uninstall')").click()
        tp.wait_for_timeout(800)
        tp.locator("#inst-actions button:has-text('Check')").first.click()
        tp.wait_for_timeout(1500)
        shot(tp, "e09-processor-second-account-not-allowed")
        print("denied:", tp.inner_text("#inst-status"), "|", tp.inner_text("#inst-next"), "| install button present:", tp.locator("#inst-actions button:has-text('Install')").count())
        # Ask the server to install anyway (as a script would): it must refuse before any download.
        res = tp.evaluate("""async () => {
          const s = await (await fetch('/api/session')).json();
          const r = await fetch('/api/te/install', {method: 'POST', headers: {'Content-Type': 'application/json', 'X-TP-CSRF': s.csrf}, body: '{}'});
          await new Promise(r => setTimeout(r, 1500));
          const st = await (await fetch('/api/state')).json();
          return {status: r.status, phase: st.install.phase, problem: st.install.problem && st.install.problem.code, installed: !!st.install.installed};
        }""")
        print("forced install as denied account:", res)

    if args.phase == "restart":
        tp.bring_to_front()
        tp.locator("#inst-actions button:has-text('Check')").first.click()
        tp.wait_for_timeout(1500)
        shot(tp, "e10-processor-after-identity-restart")
        print("after restart:", tp.inner_text("#inst-status"), "|", tp.inner_text("#inst-next"))
        tp.click("#diag summary")
        tp.click("[data-tab='context']")
        tp.click("#copy")
        tp.wait_for_timeout(800)
        open(os.path.join(args.out, "processor-context.txt"), "w").write(tp.inner_text("#context"))
    b.close()
