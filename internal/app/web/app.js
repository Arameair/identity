"use strict";
// Identity UI. Plain JS, no external code (strict CSP).
(() => {
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => [...r.querySelectorAll(s)];
  const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const fmtTime = (iso) => { if (!iso) return "—"; const d = new Date(iso); return isNaN(d) ? "—" : d.toLocaleString(); };
  const ago = (iso) => { const s = Math.max(0, Math.round((Date.now() - new Date(iso)) / 1000)); return s < 60 ? s + "s ago" : s < 3600 ? Math.round(s / 60) + " min ago" : Math.round(s / 3600) + " h ago"; };

  let csrf = "", state = null, step = 0, lastLogId = 0, filter = "all", lastRun = "";
  const logRows = [];

  // ---------- networking ----------
  async function api(method, path, body) {
    const opt = { method, headers: {}, credentials: "same-origin" };
    if (method !== "GET") {
      opt.headers["Content-Type"] = "application/json";
      opt.headers["X-Identity-CSRF"] = csrf;
      opt.body = JSON.stringify(body || {});
    }
    const r = await fetch(path, opt);
    const ct = r.headers.get("Content-Type") || "";
    const data = ct.includes("json") ? await r.json().catch(() => ({})) : await r.text();
    if (!r.ok) {
      const err = new Error((data && data.error) || ("HTTP " + r.status));
      err.status = r.status; err.data = data; err.requestId = r.headers.get("X-Request-ID");
      throw err;
    }
    return data;
  }

  function toast(msg, bad) {
    const t = $("#toast");
    t.textContent = msg; t.className = "toast" + (bad ? " bad" : ""); t.hidden = false;
    clearTimeout(toast.t); toast.t = setTimeout(() => (t.hidden = true), bad ? 7000 : 3500);
  }
  async function act(path, body, okMsg) {
    try {
      const r = await api("POST", path, body);
      if (r && r.app) apply(r); else if (r && r.state) apply(r.state);
      if (okMsg) toast(okMsg);
      pollLogs();
      return r;
    } catch (e) {
      if (e.data && e.data.state) apply(e.data.state);
      toast(e.message + (e.requestId ? "  (" + e.requestId + ")" : ""), true);
      pollLogs();
      return null;
    }
  }

  // ---------- session bootstrap ----------
  async function start() {
    const m = location.hash.match(/key=([0-9a-f]+)/);
    // Remove the private key from the address bar and history immediately.
    if (location.hash) history.replaceState(null, "", location.pathname);
    try {
      if (m) {
        const r = await api("POST", "/api/session", { key: m[1] });
        csrf = r.csrf;
      } else {
        const r = await api("GET", "/api/session");
        csrf = r.csrf;
      }
    } catch (e) {
      $("#gate").hidden = false;
      if (m) $("#gate-msg").textContent = "That link is not valid for the Identity that is running now (" + e.message + "). Use the newest link from Identity's console window.";
      return;
    }
    $("#shell").hidden = false;
    wire();
    await refresh();
    const s = state && state.setup;
    if (s && !s.completed) showStep(state.github.status === "verified" ? 2 : 0);
    pollLogs();
    setInterval(refresh, 2000);
    setInterval(pollLogs, 1500);
  }

  async function refresh() {
    try { apply(await api("GET", "/api/state")); }
    catch (e) {
      if (e.status === 401) { $("#shell").hidden = true; $("#gate").hidden = false; $("#gate-msg").textContent = "Identity restarted or this window's session ended. Open the new link from Identity's console window."; }
      else setChip("#chip-identity", "bad", "NOT RESPONDING");
    }
  }

  // ---------- rendering ----------
  function setChip(sel, cls, text, title) {
    const c = $(sel); c.className = "chip " + cls; $(".v", c).textContent = text; c.title = title || text;
  }

  function apply(s) {
    state = s;
    const a = s.app, b = a.build, gh = s.github, p = s.processor;
    $("#brand-sub").textContent = b.version + " · " + (b.revision === "UNKNOWN" ? "commit UNKNOWN" : b.revision.slice(0, 10) + (b.modified === "true" ? "+dirty" : ""));
    setChip("#chip-identity", "ok", "RUNNING · port " + a.port, "PID " + a.pid + " · " + a.profile_path + " · " + a.system_id);

    if (gh.status === "verified" && gh.account) setChip("#chip-github", gh.mode === "real" ? "ok" : "warn", (gh.mode === "real" ? "VERIFIED · " : "FIXTURE · ") + gh.account.login, "Confirmed by GitHub API at " + fmtTime(gh.account.verified_at));
    else if (gh.status === "waiting") setChip("#chip-github", "info", "CHECKING GITHUB APPROVAL");
    else if (gh.status === "not_configured") setChip("#chip-github", "warn", "NO GITHUB APP IN THIS BUILD");
    else if (gh.status === "error") setChip("#chip-github", "bad", "SIGN-IN FAILED", gh.message);
    else if (gh.status === "denied") setChip("#chip-github", "warn", "SIGN-IN CANCELLED");
    else if (gh.status === "expired") setChip("#chip-github", "warn", "SIGN-IN EXPIRED");
    else setChip("#chip-github", "", s.last_account ? "NOT SIGNED IN · last " + s.last_account.login : "NOT SIGNED IN");

    const pcls = p.state.startsWith("DETECTED") ? (p.freshness === "STALE" ? "warn" : "info") : p.state === "NOT CONFIGURED" || p.state === "NOT CHECKED YET" ? "" : p.state === "WRONG SERVICE" ? "bad" : "warn";
    const plabel = p.state.startsWith("DETECTED") ? "DETECTED · UNVERIFIED" : p.state;
    setChip("#chip-processor", pcls, plabel + (p.freshness === "STALE" ? " · STALE" : ""), (p.address || "no address") + " — " + (p.note || ""));
    const open = $("#open-tp");
    if (p.open_url && p.freshness === "CURRENT") { open.href = p.open_url; open.hidden = false; open.title = "Opens Thought Processor's own page. It will ask for its own operator code — Identity does not sign you in."; }
    else open.hidden = true;

    $("#setup").hidden = s.setup.completed;
    renderGitHub(); renderFeatures(); renderProcessor(); renderAbout(); renderFacts();
  }

  function renderFacts() {
    const a = state.app, gh = state.github, p = state.processor;
    $("#welcome-facts").innerHTML =
      `<dt>This Identity</dt><dd>${esc(a.system_id)}</dd><dt>Private profile</dt><dd>${esc(a.profile_path)}</dd>` +
      `<dt>Address</dt><dd>127.0.0.1:${esc(a.port)} (this computer only)</dd><dt>Publisher</dt><dd>${esc(state.publisher.name)} — ${esc(state.publisher.evidence)}</dd>`;
    const drafts = state.features.filter((f) => f.request_kind === "draft").map((f) => f.name);
    $("#summary-facts").innerHTML =
      `<dt>GitHub</dt><dd>${gh.status === "verified" ? esc(gh.account.login) + " — verified by GitHub (" + esc(gh.mode) + ")" : "not connected (" + esc(gh.status) + ")"}</dd>` +
      `<dt>Local drafts</dt><dd>${drafts.length ? esc(drafts.join(", ")) + " — NOT SUBMITTED" : "none"}</dd>` +
      `<dt>Grants</dt><dd>none (no grant service exists yet)</dd>` +
      `<dt>Thought Processor</dt><dd>${esc(p.state)}${p.address ? " at " + esc(p.address) : ""}</dd>`;
  }

  function renderGitHub() {
    const gh = state.github, el = $("#gh-body");
    const appLine = gh.client_id_kind === "built-in"
      ? `Uses ${esc(state.publisher.name)}'s GitHub App, built into Identity — nothing to set up.`
      : gh.client_id_kind === "own"
        ? `Uses <strong>your own GitHub App</strong> (${esc(gh.client_id_hint)}). <button type="button" class="linkish" data-act="/api/github/own-app/remove">Stop using it</button>`
        : gh.client_id_kind === "developer"
          ? `Developer override: ${esc(gh.client_id_source)} (${esc(gh.client_id_hint)}).` : "";
    let h = "";
    if (gh.mode !== "real") h += `<div class="note bad-note"><strong>TEST FIXTURE MODE.</strong> Identity is talking to a local stand-in for GitHub, not github.com. Nothing here is a real sign-in.</div>`;
    if (gh.status === "not_configured") {
      h += `<div class="note warn-note"><strong>This copy of Identity has no GitHub App built in.</strong>
        <p>Official copies from ${esc(state.publisher.name)} include one, so you'd just press Connect GitHub. With this copy you can set up your own in a few minutes — Identity walks you through each step.</p></div>
        <div class="row"><button class="btn primary" type="button" data-guide="open">Help me set up a GitHub App</button></div>
        <p class="muted small">(This is a test build. In an official build this step is just a Connect GitHub button.)</p>
        <p class="muted small">Or skip for now: Identity works without GitHub, but feature availability stays UNKNOWN.</p>`;
    } else if (gh.status === "waiting") {
      h += `<p><strong>GitHub opened in a new tab and your code was copied.</strong> There: select your account if prompted, paste the current code (Ctrl+V), press <em>Continue</em>, then <em>Authorize</em>. Return to this Identity tab while it checks.</p>
        <div class="code-box"><span class="code" id="ucode">${esc(gh.user_code)}</span>
        <button class="btn" type="button" id="copy-code">Copy code</button>
        <a class="btn primary" id="gh-open" href="${esc(gh.verify_url)}" target="_blank" rel="noopener noreferrer">Open ${esc(new URL(gh.verify_url).host)} again ↗</a></div>
        <p class="small"><strong>${gh.last_checked_at ? "Last checked with GitHub " + esc(ago(gh.last_checked_at)) : "Checking with GitHub every few seconds"}</strong>${gh.poll_note ? " — " + esc(gh.poll_note) : ""}</p>
        <p class="muted small">GitHub said "all set" but this didn't change within ~10 seconds? Check the GitHub tab for an account restriction or authorization refusal. If you restarted sign-in, make sure you approved the current code shown here, not an older one.</p>
        <p class="muted small">No new tab? Your browser blocked it — use the button above. The code expires ${gh.expires_at ? "at " + esc(fmtTime(gh.expires_at)) : "soon"} and is never written to Identity's log or diagnostics.</p>
        <div class="row"><button class="btn ghost" type="button" data-act="/api/github/cancel">Cancel sign-in</button></div>`;
    } else if (gh.status === "verified" && gh.account) {
      const ac = gh.account, pub = gh.publisher;
      h += `<dl class="account"><dt>Account</dt><dd><strong>${esc(ac.login)}</strong>${ac.name ? " (" + esc(ac.name) + ")" : ""}</dd>
        <dt>GitHub user id</dt><dd>${esc(ac.id)}</dd><dt>Verified</dt><dd>${esc(fmtTime(ac.verified_at))} — read back from the GitHub API with your sign-in</dd>
        <dt>Sign-in kept</dt><dd>in memory only${gh.token_expires_at ? ", GitHub expires it " + esc(fmtTime(gh.token_expires_at)) : ""}</dd>
        <dt>GitHub App</dt><dd>${appLine}</dd></dl>`;
      if (pub) {
        h += `<h3>Publisher information from GitHub</h3><dl class="account"><dt>Publisher (build label)</dt><dd>${esc(state.publisher.name)} — ${esc(state.publisher.evidence)}</dd>
          <dt>App installations you can see</dt><dd>${esc(pub.installations_result)}${pub.installations_detail ? " — " + esc(pub.installations_detail) : ""}</dd>`;
        for (const i of pub.installations) h += `<dt></dt><dd><code>${esc(i.app_slug)}</code> on <strong>${esc(i.account)}</strong> · installation ${esc(i.id)} · ${esc(i.repository_selection)} repositories</dd>`;
        for (const r of pub.repos) h += `<dt>${esc(r.repo)}</dt><dd>${tagFor(r.result)} <span class="muted small">checked ${esc(ago(r.checked_at))}</span></dd>`;
        h += `</dl><p class="muted small">Seeing a repository is <strong>not</strong> permission to use the feature. No publisher approval exists yet. If no publisher app installation is listed, ask the publisher whether installation is required.</p>`;
      }
      h += `<div class="row"><button class="btn ghost" type="button" data-act="/api/github/recheck">Check again</button><button class="btn ghost" type="button" data-act="/api/github/disconnect">Disconnect</button></div>`;
    } else {
      if (gh.message) h += `<div class="note ${gh.status === "error" ? "bad-note" : "warn-note"}">${esc(gh.message)}</div>`;
      if (state.last_account) h += `<p class="muted">Last verified account on this profile: <strong>${esc(state.last_account.login)}</strong> (${esc(state.last_account.mode)}, ${esc(fmtTime(state.last_account.verified_at))}). That is history — you are not signed in now.</p>`;
      h += `<div class="row"><button class="btn primary" type="button" id="gh-connect">Connect GitHub</button></div>
        <p class="muted small">Opens GitHub in a new tab. Select your account if prompted, paste the current code, authorize, and return to Identity. Identity never sees your password.</p>
        <p class="muted small">${appLine}</p>`;
    }
    if (el.dataset.html !== h) { el.innerHTML = h; el.dataset.html = h; }
    $("#gh-next").textContent = gh.status === "verified" ? "Continue" : "Skip for now";
  }

  // Connect: open the tab during the click (so pop-up blockers allow it),
  // then point it at GitHub's device page once Identity has the code.
  async function connectGitHub() {
    let w = null;
    try { w = window.open("about:blank", "_blank"); } catch (e) { w = null; }
    const r = await act("/api/github/start", {});
    const gh = r && r.github;
    if (!gh || gh.status !== "waiting") { if (w) w.close(); return; }
    try { await navigator.clipboard.writeText(gh.user_code); } catch (e) { /* code stays visible on screen */ }
    if (w) { try { w.opener = null; w.location.href = gh.verify_url; } catch (e) { w.close(); } }
  }

  function tagFor(result) {
    const cls = result === "VISIBLE" ? "ok" : result === "NOT VISIBLE" ? "warn" : "bad";
    return `<span class="tag ${cls}">${esc(result)}</span>`;
  }

  function renderFeatures() {
    const signed = state.github.status === "verified";
    let h = "";
    if (state.drafts_error) h += `<div class="note bad-note">${esc(state.drafts_error)}</div>`;
    h += `<div class="cards">`;
    for (const f of state.features) {
      const av = f.avail_kind === "available" ? "ok" : f.avail_kind === "unavailable" ? "warn" : "";
      const rq = f.request_kind === "draft" ? "info" : "";
      let extra = "";
      if (f.id === "thought-processor") {
        const p = state.processor;
        extra = `<dt>On this computer</dt><dd><span class="tag ${p.state.startsWith("DETECTED") ? "info" : ""}">${esc(p.state)}</span></dd>`;
      } else {
        extra = `<dt>On this computer</dt><dd><span class="tag">NOT CHECKED</span> <span class="muted small">its status page holds private data; Identity doesn't read it</span></dd>`;
      }
      let btn = "";
      if (!signed) btn = `<button class="btn" type="button" disabled title="Connect GitHub first">Save access-request draft</button>`;
      else if (f.request_kind === "draft") btn = `<button class="btn ghost" type="button" data-act="/api/drafts/withdraw" data-feature="${esc(f.id)}">Delete local draft</button>`;
      else btn = `<button class="btn" type="button" data-act="/api/drafts" data-feature="${esc(f.id)}">Save access-request draft</button>`;
      h += `<div class="card"><div class="t">${esc(f.name)}</div><div class="d">${esc(f.description)}</div>
        <dl class="states"><dt>Availability</dt><dd><span class="tag ${av}">${esc(f.availability)}</span></dd>
        <dt>Request</dt><dd><span class="tag ${rq}">${esc(f.request)}</span>${f.draft ? ` <span class="muted small">saved ${esc(fmtTime(f.draft.created_at))}</span>` : ""}</dd>
        <dt>Grant</dt><dd><span class="tag">${esc(f.grant)}</span></dd>${extra}</dl>
        <div class="row">${btn}</div></div>`;
    }
    h += `</div><p class="muted small"><strong>Continue does not submit requests.</strong> Drafts remain local. Saving a draft or finishing setup does not grant access.</p>`;
    const el = $("#feat-body");
    if (el.dataset.html !== h) { el.innerHTML = h; el.dataset.html = h; }
    const lg = state.state_legend;
    const lh = ["available", "unavailable", "not_requested", "draft", "requested", "granted"].map((k) => `<dt>${esc(k.replace("_", " "))}</dt><dd>${esc(lg[k])}</dd>`).join("");
    if ($("#legend").dataset.html !== lh) { $("#legend").innerHTML = lh; $("#legend").dataset.html = lh; }
  }

  function renderProcessor() {
    const p = state.processor;
    const inp = $("#tp-addr");
    if (document.activeElement !== inp && !inp.dataset.touched) inp.value = p.address || "127.0.0.1:7860";
    const cls = p.state.startsWith("DETECTED") ? "info-note" : p.state === "WRONG SERVICE" ? "bad-note" : p.state === "NOT CONFIGURED" || p.state === "NOT CHECKED YET" ? "note" : "warn-note";
    let h = `<div class="note ${cls}"><strong>${esc(p.state)}</strong>${p.freshness === "STALE" ? " — <strong>STALE</strong> (no recent check)" : ""}<p>${esc(p.note || "")}</p></div>`;
    if (p.state.startsWith("DETECTED")) h += `<p class="muted small"><strong>Next:</strong> Open Processor with its own operator code. Detection is not authenticated trust.</p>`;
    else h += `<p class="muted small"><strong>Next:</strong> Start Processor separately, enter its local address above, and press Check now.</p>`;
    if (p.checked_at) {
      h += `<dl class="account"><dt>Address</dt><dd>${esc(p.address)}</dd><dt>Last checked</dt><dd>${esc(fmtTime(p.checked_at))} (${esc(p.elapsed_ms)} ms)</dd>`;
      if (p.claimed_app) h += `<dt>It says it is</dt><dd>${esc(p.claimed_app)} ${esc(p.claimed_version)} · ${esc(p.claimed_protocol)} <span class="tag warn">SELF-REPORTED</span></dd>`;
      h += `<dt>Authentication</dt><dd>none — no Identity↔Processor contract exists yet</dd></dl>`;
    }
    if (p.open_url && p.freshness === "CURRENT") h += `<div class="row"><a class="btn primary" id="open-tp-2" href="${esc(p.open_url)}" target="_blank" rel="noopener noreferrer">Open Thought Processor ↗</a></div>
      <p class="muted small">Opens Processor's own page. Processor will ask for <em>its</em> operator code (shown in Processor's console window). Identity never reads or sends that code.</p>`;
    h += `<p class="muted small">Identity re-checks every 10 seconds and logs only changes. Closing either program never stops the other.</p>`;
    const el = $("#tp-body");
    if (el.dataset.html !== h) { el.innerHTML = h; el.dataset.html = h; }
  }

  function renderAbout() {
    const a = state.app, b = a.build;
    const h = `<dt>Status</dt><dd>RUNNING since ${esc(fmtTime(a.started_at))}</dd><dt>Process</dt><dd>PID ${esc(a.pid)} · port ${esc(a.port)} · run ${esc(a.run_id)}</dd>
      <dt>Build</dt><dd>${esc(b.version)} · branch ${esc(b.branch)}</dd><dt>Commit</dt><dd>${esc(b.revision)}${b.modified === "true" ? " (uncommitted changes)" : ""}</dd>
      <dt>Go</dt><dd>${esc(b.go_version)}</dd><dt>Profile</dt><dd>${esc(a.profile_path)}</dd><dt>System ID</dt><dd>${esc(a.system_id)}</dd>
      <dt>Log</dt><dd>${esc(state.log.in_memory)} entries in view · ${esc(state.log.write_errors)} write errors · ${esc(state.log.unreadable_lines)} unreadable lines</dd>`;
    if ($("#about-facts").dataset.html !== h) { $("#about-facts").innerHTML = h; $("#about-facts").dataset.html = h; }
    const ni = state.not_implemented.map((x) => `<li>${esc(x)}</li>`).join("");
    if ($("#notimpl").dataset.html !== ni) { $("#notimpl").innerHTML = ni; $("#notimpl").dataset.html = ni; }
  }

  // ---------- setup steps ----------
  function showStep(n) {
    step = n;
    $$(".step").forEach((s) => s.classList.toggle("on", +s.dataset.step === n));
    $$("#steps li").forEach((li) => { const i = +li.dataset.step; li.classList.toggle("on", i === n); li.classList.toggle("done", i < n); });
    const slot = $(`.step[data-step="${n}"] .slot`);
    if (slot) slot.appendChild($("#panel-" + slot.dataset.panel));
  }

  // ---------- manage dialog ----------
  function openManage(tab) {
    const d = $("#manage");
    $$("#manage .tabs button").forEach((b) => b.classList.toggle("on", b.dataset.tab === tab));
    // Park the panel currently shown back in storage first; replaceChildren
    // alone would detach it from the page and lose it on the next tab switch.
    $$("#manage-slot .panel").forEach((p) => $("#panels").appendChild(p));
    $("#manage-slot").appendChild($("#panel-" + tab));
    if (!d.open) d.showModal();
  }
  function closeManage() {
    $("#manage").close();
    // Return panels to storage, and back to the current setup step if setup is open.
    $$("#manage-slot .panel").forEach((p) => $("#panels").appendChild(p));
    if (!state.setup.completed) showStep(step);
  }

  // ---------- log ----------
  async function pollLogs() {
    try {
      const r = await api("GET", "/api/logs?after=" + lastLogId);
      for (const e of r.entries) addLog(e);
      $("#log-meta").textContent = `${r.stats.in_memory} entries · saved in your profile · ${r.stats.write_errors ? r.stats.write_errors + " write errors!" : "no write errors"}`;
    } catch (e) { /* state refresh reports connectivity */ }
  }
  function addLog(e) {
    lastLogId = Math.max(lastLogId, e.id);
    const box = $("#log");
    const follow = $("#follow").checked && box.scrollHeight - box.scrollTop - box.clientHeight < 40;
    if (e.run_id !== lastRun) {
      const d = document.createElement("div");
      d.className = "ev run-divider"; d.textContent = `— run ${e.run_id} —`;
      box.appendChild(d); logRows.push(d); lastRun = e.run_id;
    }
    const row = document.createElement("div");
    row.className = "ev " + e.level; row.dataset.level = e.level;
    const st = e.state ? Object.entries(e.state).map(([k, v]) => k + "=" + v).join(" ") : "";
    row.innerHTML = `<span class="tm" title="${esc(e.local)}">${esc(new Date(e.ts).toLocaleTimeString())}</span><span class="lv">${esc(e.level)}</span><span class="cp">${esc(e.component)}</span><span class="evn">${esc(e.event)}</span>
      <span class="msg">${e.result ? `<span class="res">${esc(e.result)}</span> ` : ""}${esc(e.detail)}${e.request_id ? ` <span class="rid">${esc(e.request_id)}</span>` : ""}${st ? `<br><span class="st">${esc(st)}</span>` : ""}</span>`;
    if (filter === "problems" && e.level === "info") row.hidden = true;
    box.appendChild(row); logRows.push(row);
    while (logRows.length > 1500) logRows.shift().remove();
    if (follow) box.scrollTop = box.scrollHeight;
  }
  function applyFilter() {
    $$("#log .ev").forEach((r) => { if (r.dataset.level) r.hidden = filter === "problems" && r.dataset.level === "info"; });
  }

  // ---------- copy diagnostics ----------
  async function copyDiagnostics() {
    const btn = $("#copy-btn"); btn.disabled = true;
    let text;
    try { text = await api("GET", "/api/diagnostics"); }
    catch (e) { toast("Could not build diagnostics: " + e.message, true); btn.disabled = false; return; }
    let ok = false, method = "", why = "";
    try { await navigator.clipboard.writeText(text); ok = true; method = "clipboard API"; }
    catch (e) {
      why = e.message || "clipboard API unavailable";
      const ta = document.createElement("textarea"); ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0";
      document.body.appendChild(ta); ta.select();
      try { ok = document.execCommand("copy"); method = "execCommand"; } catch (e2) { why += "; " + e2.message; }
      ta.remove();
    }
    api("POST", "/api/diagnostics/result", { ok, method, error: why }).catch(() => {});
    if (ok) toast("Diagnostics copied (" + text.length + " characters, secrets removed). Paste it into the chat.");
    else {
      $("#diag-msg").textContent = "Your browser blocked automatic copying (" + why + "). Select the text below and copy it with Ctrl+C.";
      $("#diag-text").value = text; $("#diag-fallback").showModal(); $("#diag-text").select();
    }
    btn.disabled = false; pollLogs();
  }

  // ---------- wiring ----------
  function wire() {
    $("#copy-btn").addEventListener("click", copyDiagnostics);
    $("#manage-btn").addEventListener("click", () => openManage("github"));
    $("#manage-close").addEventListener("click", closeManage);
    $("#manage").addEventListener("close", () => { if ($("#manage-slot").children.length) closeManage(); });
    $$("#manage .tabs button").forEach((b) => b.addEventListener("click", () => openManage(b.dataset.tab)));
    $("#diag-close").addEventListener("click", () => $("#diag-fallback").close());
    $$("[data-go]").forEach((b) => b.addEventListener("click", () => showStep(+b.dataset.go)));
    $("#finish-btn").addEventListener("click", async () => { const r = await act("/api/setup/complete", {}, "Setup finished — Identity is now monitoring."); if (r) $("#log").scrollTop = $("#log").scrollHeight; });
    $("#reopen-setup").addEventListener("click", async () => { closeManage(); await act("/api/setup/reopen", {}); showStep(0); });
    $("#stop-btn").addEventListener("click", async () => {
      if (!confirm("Stop Identity? Only Identity stops; Thought Processor keeps running.")) return;
      await act("/api/shutdown", {});
      document.body.innerHTML = '<div class="gate"><h1>Identity stopped</h1><p>Only Identity was stopped. Start it again from its program to continue; your profile and log are kept.</p></div>';
    });
    $("#own-save").addEventListener("click", async () => {
      const r = await act("/api/github/own-app", { client_id: $("#own-client-id").value }, "Your GitHub App is set. Now press Connect GitHub.");
      if (r) { $("#gh-guide").hidden = true; $("#own-client-id").value = ""; }
    });
    $("#guide-close").addEventListener("click", () => ($("#gh-guide").hidden = true));
    $("#tp-addr").addEventListener("input", (e) => (e.target.dataset.touched = "1"));
    $("#tp-save").addEventListener("click", async () => { const r = await act("/api/processor/address", { address: $("#tp-addr").value }); if (r) { delete $("#tp-addr").dataset.touched; toast("Address saved: " + r.processor.state); } });
    $("#tp-check").addEventListener("click", () => act("/api/processor/check", {}));
    $$(".seg button").forEach((b) => b.addEventListener("click", () => { filter = b.dataset.filter; $$(".seg button").forEach((x) => x.classList.toggle("on", x === b)); applyFilter(); }));
    // Delegated buttons inside re-rendered panels.
    document.addEventListener("click", (ev) => {
      const t = ev.target.closest("[data-act]");
      if (t) { ev.preventDefault(); act(t.dataset.act, t.dataset.feature ? { feature: t.dataset.feature } : {}); return; }
      if (ev.target.closest("[data-guide]")) { if ($("#manage").open) openManage("github"); $("#gh-guide").hidden = false; $("#gh-guide").scrollIntoView({ behavior: "smooth", block: "start" }); return; }
      if (ev.target.id === "gh-connect") { connectGitHub(); return; }
      if (ev.target.id === "copy-code") {
        navigator.clipboard.writeText($("#ucode").textContent).then(() => toast("Code copied."), () => toast("Select the code and copy it by hand.", true));
      }
      const o = ev.target.closest("#open-tp, #open-tp-2");
      if (o) api("POST", "/api/processor/opened", {}).catch(() => {}).finally(pollLogs);
    });
    showStep(0);
  }

  start();
})();
