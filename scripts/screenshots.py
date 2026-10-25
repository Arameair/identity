"""Drive the REAL running Identity B and REAL Thought Processor in a headless
browser and save screenshots. Developer evidence script (Linux), not part of
the product. Usage:

  python3 -I scripts/screenshots.py --identity-profile P --processor-profile Q --out DIR [--prefix real]

Reads each program's private open link from its own profile runtime folder
(as the owner would from the console). Never prints those links.
"""
import argparse, os, re, sys, time
from playwright.sync_api import sync_playwright

ap = argparse.ArgumentParser()
ap.add_argument("--identity-profile", required=True)
ap.add_argument("--processor-profile", default="")
ap.add_argument("--processor-port", default="7860")
ap.add_argument("--identity-port", default="7798")
ap.add_argument("--out", required=True)
ap.add_argument("--prefix", default="real")
ap.add_argument("--fixture-signin", action="store_true", help="drive the TEST FIXTURE GitHub sign-in")
args = ap.parse_args()
os.makedirs(args.out, exist_ok=True)
link = open(os.path.join(args.identity_profile, "runtime", "open-link.txt")).read().strip()


def shot(page, name):
    p = os.path.join(args.out, f"{args.prefix}-{name}.png")
    page.screenshot(path=p, full_page=False)
    print("saved", p)


with sync_playwright() as pw:
    b = pw.chromium.launch()
    ctx = b.new_context(viewport={"width": 1280, "height": 820}, permissions=["clipboard-read", "clipboard-write"])
    page = ctx.new_page()
    page.goto(link)
    page.wait_for_selector("#shell:not([hidden])")
    page.wait_for_timeout(800)
    assert "#key" not in page.url, "private key left in address bar"
    shot(page, "01-welcome")
    page.click(".step.on [data-go='1']")
    page.wait_for_timeout(400)
    if args.fixture_signin:
        page.click("[data-act='/api/github/start']")
        page.wait_for_selector(".code-box")
        page.wait_for_timeout(300)
        shot(page, "02a-github-waiting-code")
        page.wait_for_selector(".account", timeout=20000)
        page.wait_for_timeout(500)
    shot(page, "02-github")
    page.click(".step.on [data-go='2']")
    page.wait_for_timeout(400)
    if args.fixture_signin:
        page.click("[data-act='/api/drafts'][data-feature='thought-processor']")
        page.wait_for_timeout(800)
    shot(page, "03-features")
    page.click(".step.on [data-go='3']")
    page.fill("#tp-addr", f"127.0.0.1:{args.processor_port}")
    page.click("#tp-save")
    page.wait_for_timeout(1200)
    shot(page, "04-processor-step")
    page.click(".step.on [data-go='4']")
    page.wait_for_timeout(300)
    shot(page, "05-summary")
    page.click("#finish-btn")
    page.wait_for_timeout(1500)
    shot(page, "06-monitor")
    page.click("#copy-btn")
    page.wait_for_timeout(1200)
    clip = page.evaluate("navigator.clipboard.readText()")
    with open(os.path.join(args.out, f"{args.prefix}-copied-diagnostics.txt"), "w") as f:
        f.write(clip)
    shot(page, "07-monitor-after-copy")
    page.click("#manage-btn")
    page.click("#manage .tabs [data-tab='features']")
    page.wait_for_timeout(400)
    shot(page, "08-manage-features")
    page.click("#manage .tabs [data-tab='about']")
    page.wait_for_timeout(300)
    shot(page, "09-manage-identity")
    page.click("#manage-close")

    if args.processor_profile:
        code = open(os.path.join(args.processor_profile, "runtime", "operator-code")).read().strip()
        tp = ctx.new_page()
        tp.goto(f"http://127.0.0.1:{args.processor_port}/#code={code}")
        tp.wait_for_timeout(1500)
        tp.fill("#identity-addr", f"127.0.0.1:{args.identity_port}")
        tp.click("#id-form button[type=submit]")
        tp.wait_for_timeout(2500)
        tp.locator("#card-identity").scroll_into_view_if_needed()
        p = os.path.join(args.out, f"{args.prefix}-10-processor-sees-identity.png")
        tp.screenshot(path=p)
        print("saved", p)
        print("processor identity card:", tp.inner_text("#card-identity")[:400].replace("\n", " | "))
    page.wait_for_timeout(1500)
    page.bring_to_front()
    shot(page, "11-identity-log-after-processor")
    b.close()
