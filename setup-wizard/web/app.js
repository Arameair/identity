"use strict";
// Identity setup wizard. Every screen is drawn from /api/state, which is built
// from real evidence (GitHub's answers, Project Controller's answers, and
// Thought Processor answering on its port). Text from other programs is only
// ever inserted with textContent.
(() => {
  const $ = (id) => document.getElementById(id);
  let csrf = "";
  let S = null;
  // Moments the person has moved past. Kept in memory only.
  const seen = { welcomed: false, findClicked: false, continued: false };

  const short = (sha) => (sha || "").slice(0, 7);
  const when = (iso) => { const d = new Date(iso); return isNaN(d) ? "" : d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }); };
  const clock = (iso) => { const d = new Date(iso); return isNaN(d) ? "" : d.toLocaleTimeString(); };

  async function call(method, path, body) {
    const opt = { method, credentials: "same-origin", headers: {} };
    if (method !== "GET") {
      opt.headers["Content-Type"] = "application/json";
      opt.headers["X-Identity-CSRF"] = csrf;
      opt.body = JSON.stringify(body || {});
    }
    try {
      const r = await fetch(path, opt);
      return { ok: r.ok, status: r.status, data: await r.json().catch(() => null) };
    } catch { return { ok: false, status: 0, data: null }; }
  }

  function facts(dl, rows) {
    dl.replaceChildren();
    for (const [k, v] of rows) {
      const dt = document.createElement("dt"); dt.textContent = k;
      const dd = document.createElement("dd");
      if (v instanceof Node) dd.append(v); else dd.textContent = v;
      dl.append(dt, dd);
    }
  }
  function code(text) { const c = document.createElement("code"); c.textContent = text; return c; }

  function show(view) {
    for (const s of document.querySelectorAll("main > section")) s.hidden = s.id !== view;
    const n = Number(view.replace("v-", "")) || 0;
    for (const li of document.querySelectorAll("#steps li")) {
      const k = Number(li.dataset.s);
      li.classList.toggle("current", k === n);
      li.classList.toggle("done", n > 0 && k < n);
      if (k === n) li.setAttribute("aria-current", "step"); else li.removeAttribute("aria-current");
    }
  }

  // ---------- which screen ----------
  function pick(s) {
    const gh = s.github.status;
    if (!seen.welcomed && gh !== "connected" && gh !== "waiting") return "v-1";
    if (s.step === 2) return "v-2";
    if (s.step === 3) return (!seen.findClicked && !s.find) ? "v-2" : "v-3";
    if (s.step === 4) return seen.continued || s.launch.phase !== "idle" ? "v-4" : "v-3";
    return "v-5";
  }

  function render(s) {
    S = s;
    $("testmode").hidden = !s.about.test_mode;
    $("foot").textContent = `Identity setup ${s.about.version} · commit ${short(s.about.commit)}${s.about.modified ? " + local changes" : ""}`;
    const view = pick(s);
    show(view);
    renderWelcome(s); renderGitHub(s); renderFind(s); renderStart(s); renderOpen(s);
  }

  function renderWelcome(s) {
    const nc = s.github.status === "not_configured";
    $("w-notconfigured").hidden = !nc;
    $("b-connect").disabled = nc;
  }

  function renderGitHub(s) {
    const g = s.github;
    const waiting = g.status === "waiting", connected = g.status === "connected";
    const problem = ["denied", "expired", "error", "not_configured"].includes(g.status) || (!waiting && !connected && g.message);
    $("g-waiting").hidden = !waiting;
    $("g-connected").hidden = !connected;
    $("g-problem").hidden = !problem || connected || waiting;
    $("g-title").textContent = connected ? "GitHub is connected" : "Connect GitHub";
    if (waiting) {
      $("g-code").textContent = g.user_code;
      $("g-open").href = g.verify_url;
      const left = Math.max(0, Math.round((new Date(g.expires_at) - Date.now()) / 60000));
      $("g-expires").textContent = left ? `(code works for about ${left} more minute${left === 1 ? "" : "s"})` : "";
    }
    if (connected) $("g-login").textContent = g.login;
    if (problem) $("g-message").textContent = g.status === "not_configured"
      ? "This copy of Identity hasn't been linked to a GitHub app yet, so it can't sign in."
      : (g.message || "GitHub sign-in didn't finish.");
  }

  function renderFind(s) {
    const f = s.find;
    $("f-searching").hidden = !s.finding;
    $("f-go").hidden = s.finding || !!f;
    $("f-found").hidden = !(f && f.status === "found");
    $("f-problem").hidden = !(f && f.status !== "found");
    if (f && f.status === "found") {
      $("f-lead").textContent = "This is the version Identity will ask Project Controller to start.";
      facts($("f-facts"), [
        ["Project", "Thought Processor"],
        ["Version", s.target.name],
        ["Last updated", when(f.commit_date)],
        ["Your access", `✓ ${f.as_login} can see this ${f.private ? "private " : ""}project`],
      ]);
      facts($("f-tech"), [
        ["Repository", code(f.repo)], ["Branch", code(f.branch)], ["Commit", code(f.commit)],
        ["Change note", f.message || "—"], ["Checked", clock(f.checked_at)],
      ]);
    } else if (f) {
      $("f-message").textContent = f.detail || "Something went wrong.";
    }
  }

  const steps = ["ask", "download", "approve", "build", "run"];
  const phaseAt = { requesting: 0, downloading: 1, waiting_for_approval: 2, building: 3, starting: 4, running: 4 };
  function renderStart(s) {
    const L = s.launch, P = s.processor;
    const started = L.phase !== "idle";
    $("s-intro").hidden = started;
    $("s-progress").hidden = !started || ["controller_missing", "controller_old"].includes(L.phase);
    const failed = ["failed", "stopped", "gone", "error", "controller_missing", "controller_old"].includes(L.phase);
    const at = failed ? lastReached(L) : (phaseAt[L.phase] ?? 0);
    document.querySelectorAll("#s-progress li").forEach((li, i) => {
      li.className = "";
      const st = li.querySelector(".st"); st.textContent = "";
      if (i < at || (L.phase === "running" && P.status === "running")) { li.className = "done"; st.textContent = "✓"; }
      else if (i === at && failed) { li.className = "failed"; st.textContent = "!"; }
      else if (i === at && L.phase === "waiting_for_approval") { li.className = "you"; st.textContent = "…"; }
      else if (i === at) li.className = "now";
    });
    $("s-approve").hidden = L.phase !== "waiting_for_approval";
    $("s-repo").textContent = s.target.repo;
    $("s-commit").textContent = short(L.commit || (s.find && s.find.commit));
    $("s-openpc").href = L.controller_url;
    const mismatch = L.commit_match === false;
    $("s-mismatch").hidden = !mismatch;
    if (mismatch) $("s-mismatch").textContent = `Heads up: Project Controller downloaded commit ${short(L.commit)}, but GitHub showed ${short(s.find && s.find.commit)} a moment ago. The version changed in between. Check the commit in Project Controller before approving.`;
    let problem = "";
    if (failed) problem = friendly(L);
    else if (L.phase === "running" && (P.status === "not_answering" || P.status === "wrong_app")) problem = P.detail;
    $("s-problem").hidden = !problem;
    $("s-message").textContent = problem;
    $("s-details").hidden = !(L.log && L.log.length);
    const log = $("s-log");
    const text = (L.log || []).join("\n");
    if (log.textContent !== text) { log.textContent = text; log.scrollTop = log.scrollHeight; }
  }
  function lastReached(L) {
    const log = (L.log || []).join("\n");
    if (/HTTP readiness|running at|exited before/.test(log)) return 4;
    if (/owner approved/.test(log)) return 3;
    if (/build plan from|require owner approval/.test(log)) return 2;
    if (/clone|Cloning/.test(log)) return 1;
    return 0;
  }
  function friendly(L) {
    switch (L.phase) {
      case "controller_missing": return L.error || "Project Controller isn't open. Open it, then click Try again.";
      case "controller_old": return L.error;
      case "gone": return "Thought Processor's panel was closed in Project Controller. Click Try again to ask for it again.";
      case "stopped": return "Thought Processor was stopped in Project Controller. Click Try again to start it again.";
      case "failed": {
        const e = (L.error || "").toLowerCase();
        if (e.includes("build failed")) return "Building Thought Processor failed. Open “Show what's happening” below to see why, then click Try again.";
        if (e.includes("readiness") || e.includes("exited")) return "Thought Processor started but stopped right away. The details below show what it said. Click Try again.";
        if (e.includes("clone") || e.includes("fetch") || e.includes("git")) return "Project Controller couldn't download Thought Processor. This computer's git may not have access to the private repository. Details below.";
        return "Starting didn't finish: " + (L.error || "unknown reason") + ". Click Try again.";
      }
      default: return L.error || "Something went wrong. Click Try again.";
    }
  }

  function renderOpen(s) {
    const P = s.processor, L = s.launch;
    const ok = P.status === "running";
    $("o-ok").hidden = !ok;
    $("o-problem").hidden = ok;
    $("o-title").textContent = ok ? "Thought Processor is running" : "Thought Processor isn't answering";
    if (ok) {
      facts($("o-facts"), [
        ["Version", s.target.name],
        ["Exact match", L.commit_match ? "✓ The running copy is the same version GitHub showed" : "Not confirmed — see technical details"],
        ["Last checked", clock(P.checked_at)],
      ]);
      facts($("o-tech"), [
        ["Address", code(P.url)], ["Branch", code(s.target.branch)], ["Running commit", code(L.commit || "unknown")],
        ["GitHub commit", code((s.find && s.find.commit) || "unknown")],
      ]);
      $("b-open").disabled = !P.can_open;
    } else {
      $("o-message").textContent = P.detail || "It stopped answering. Click Try again.";
    }
  }

  // ---------- actions ----------
  async function act(path) { const r = await call("POST", path); if (r.data && r.data.step) render(r.data); return r; }
  $("b-connect").onclick = async () => { seen.welcomed = true; await act("/api/github/start"); };
  $("g-retry").onclick = async () => { seen.welcomed = true; await act("/api/github/start"); };
  $("g-cancel").onclick = () => act("/api/github/cancel");
  $("g-copy").onclick = async () => {
    try { await navigator.clipboard.writeText($("g-code").textContent); $("g-copy").textContent = "Copied"; setTimeout(() => ($("g-copy").textContent = "Copy"), 1500); } catch {}
  };
  const switchAccount = async () => { seen.findClicked = false; seen.continued = false; await act("/api/github/disconnect"); };
  $("g-switch").onclick = switchAccount;
  $("f-switch").onclick = switchAccount;
  const doFind = async () => { seen.findClicked = true; render(Object.assign({}, S, { finding: true, find: null })); await act("/api/find"); };
  $("b-find").onclick = doFind;
  $("b-search").onclick = doFind;
  $("f-again").onclick = doFind;
  $("b-tostart").onclick = () => { seen.continued = true; render(S); };
  $("b-start").onclick = () => act("/api/start");
  $("s-retry").onclick = () => act("/api/retry");
  $("o-retry").onclick = () => act("/api/retry");
  $("b-open").onclick = async () => {
    const w = window.open("about:blank", "_blank"); // open now so pop-up blockers allow it
    const r = await call("POST", "/api/open");
    if (r.ok && r.data && /^http:\/\/127\.0\.0\.1:\d+\/#code=[0-9a-f]+$/.test(r.data.url)) {
      if (w) { w.opener = null; w.location.href = r.data.url; } else location.href = r.data.url;
    } else {
      if (w) w.close();
      $("o-message").textContent = (r.data && r.data.error) || "Couldn't open it.";
      $("o-problem").hidden = false;
    }
  };

  // ---------- start ----------
  async function boot() {
    const m = location.hash.match(/k=([0-9a-f]+)/);
    if (m) {
      history.replaceState(null, "", "/");
      const r = await fetch("/api/session", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ k: m[1] }) }).catch(() => null);
      if (r && r.ok) csrf = (await r.json()).csrf;
    }
    if (!csrf) {
      const r = await call("GET", "/api/session");
      if (r.ok) csrf = r.data.csrf;
    }
    if (!csrf) { show("v-signedout"); return; }
    const tick = async () => {
      const r = await call("GET", "/api/state");
      if (r.status === 401) { show("v-signedout"); return; }
      if (r.ok) render(r.data);
      setTimeout(tick, 1000);
    };
    tick();
  }
  // A start link opened in a tab that already shows Identity only changes the
  // #fragment, which doesn't reload the page. Reload so the link is used.
  window.addEventListener("hashchange", () => { if (/k=/.test(location.hash)) location.reload(); });
  boot();
})();
