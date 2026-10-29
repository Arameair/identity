"use strict";
(() => {
  const $ = (q) => document.querySelector(q);
  let csrf = "", ap = null;
  const kinds = {
    enroll_admin: ["Establish Admin Identity", "Admin Identity — once per installation"],
    install_feature: ["Install a feature", "Elevated action — registers a new installation identity"],
    grant_capability: ["Grant one capability", "Elevated action — allows one narrow request type"],
    uninstall_feature: ["Uninstall a feature", "Elevated action — revokes its grants and credential"],
    revoke_grant: ["Revoke a permission", "Admin emergency revocation — only removes access"],
  };
  function show(id) { for (const s of ["#ap-gate", "#ap-main", "#ap-done"]) $(s).hidden = s !== id; }
  function gate(msg) { $("#ap-gate-msg").textContent = msg || ""; show("#ap-gate"); }
  function fact(dl, k, v) {
    const dt = document.createElement("dt"); dt.textContent = k;
    const dd = document.createElement("dd"); dd.textContent = v; dl.append(dt, dd);
  }
  function render(j) {
    ap = j.approval;
    const [title, kind] = kinds[ap.kind] || [ap.kind, ""];
    if (ap.state !== "PENDING") return done(j);
    const d = j.display || {}, rq = d.requester || {};
    $("#ap-kind").textContent = kind;
    $("#ap-title").textContent = title + "?";
    $("#ap-headline").textContent = d.headline || "";
    $("#ap-warn").textContent = rq.warning ? "⚠ " + rq.warning : "";
    $("#ap-warn").hidden = !rq.warning;
    const dl = $("#ap-facts"); dl.textContent = "";
    fact(dl, "Requested by", (rq.label || ap.requested_by) + (rq.is_primary ? " — your primary Thought Processor" : "") +
      (rq.paired_at ? " (paired " + new Date(rq.paired_at).toLocaleString() + ")" : ""));
    if (d.who_gets_access) fact(dl, "Who gets to ask", d.who_gets_access.label);
    if (d.destination) fact(dl, "Feature", d.destination.name + (d.destination.version ? " " + d.destination.version : "") +
      (d.destination.trust_basis ? " — trusted because: " + d.destination.trust_basis : ""));
    if (d.destination && d.destination.publisher) fact(dl, "Publisher", d.destination.publisher);
    if (d.capability) fact(dl, "Allowed action (one)", d.capability.id + (d.capability.description ? " — " + d.capability.description : ""));
    if (d.capabilities_declared) fact(dl, "Declares (not granted)", d.capabilities_declared.map((c) => c.id).join(", ") || "none");
    if (d.subject) fact(dl, "For identity", (d.subject.name || d.subject.identity_id) + (d.subject.kind ? " (" + d.subject.kind + ")" : ""));
    if (ap.kind === "grant_capability" || ap.kind === "install_feature") fact(dl, "Information sources", "none");
    if (d.expiry) fact(dl, "Permission expires", d.expiry.until_revoked ? "NEVER — until revoked" : new Date(d.expiry.expires_at).toLocaleString());
    if (d.effect) fact(dl, "Effect", d.effect);
    if (d.reason) fact(dl, "Reason given", d.reason);
    fact(dl, "This request expires", new Date(ap.expires_at).toLocaleTimeString());
    $("#ap-forever").hidden = !(d.expiry && d.expiry.until_revoked); $("#ap-forever-ok").checked = false;
    const ul = $("#ap-summary"); ul.textContent = "";
    for (const s of ap.summary || []) { const li = document.createElement("li"); li.textContent = s; ul.append(li); }
    const adv = $("#ap-adv"); adv.textContent = "";
    for (const [k, v] of Object.entries(d.advanced || {})) fact(adv, k.replace(/_/g, " "), String(v));
    if (d.destination && d.destination.installation_id) fact(adv, "installation id", d.destination.installation_id);
    fact(adv, "Identity installation", j.system_id || "");
    if (j.admin && j.admin.admin) fact(adv, "Admin Identity", j.admin.admin.admin_id + " (GitHub alias " + j.admin.admin.github_binding.login_label + ")");
    const enroll = ap.kind === "enroll_admin";
    $("#ap-enroll").hidden = !enroll; $("#ap-auth").hidden = enroll;
    $("#ap-ok").textContent = enroll ? "Establish Admin Identity" : "Approve with Admin passphrase";
    show("#ap-main");
  }
  function done(j) {
    const a = j.approval;
    const [title] = kinds[a.kind] || [a.kind];
    $("#ap-done-kind").textContent = title;
    const words = { APPROVED: "Approved", CONSUMED: "Approved and carried out", DENIED: "Denied", EXPIRED: "Expired" };
    $("#ap-done-title").textContent = words[a.state] || a.state;
    $("#ap-done-msg").textContent = a.state === "DENIED" ? "Nothing was changed." :
      a.kind === "enroll_admin" && a.state === "CONSUMED" ? "The Admin Identity is established: " + (j.admin && j.admin.admin ? j.admin.admin.admin_id : "") :
      a.state === "APPROVED" ? "Thought Processor can now carry out exactly this action, once, within the time limit." : "";
    show("#ap-done");
  }
  async function post(path, body) {
    const r = await fetch(path, { method: "POST", headers: { "Content-Type": "application/json", "X-Identity-CSRF": csrf }, body: JSON.stringify(body) });
    let j = {}; try { j = await r.json(); } catch (_) {}
    if (!r.ok) throw new Error(j.error || ("HTTP " + r.status));
    return j;
  }
  function formError(message, field) {
    $("#ap-err").textContent = message;
    $("#ap-err").hidden = false;
    if (field) field.focus();
  }
  async function decide(decision) {
    const body = { decision };
    $("#ap-err").hidden = true;
    if (decision === "approve") {
      if (ap.kind === "enroll_admin") {
        const pass1 = $("#ap-pass1");
        const pass2 = $("#ap-pass2");
        // A password manager can autofill separate values. Do not submit a
        // known mismatch to Identity or discard both values on a validation error.
        if (pass1.value.length < 12) {
          return formError("Choose a passphrase with at least 12 characters.", pass1);
        }
        if (pass1.value.length > 256) {
          return formError("The passphrase must be 256 characters or fewer.", pass1);
        }
        if (pass1.value !== pass2.value) {
          return formError("The two entries do not match. Check Bitdefender autofill and enter the same passphrase in both boxes.", pass2);
        }
        if (!$("#ap-ack").checked) {
          return formError("Confirm that you own this installation before establishing its Admin Identity.", $("#ap-ack"));
        }
        body.passphrase = pass1.value;
        body.passphrase_confirm = pass2.value;
        body.acknowledge = true;
      } else { body.passphrase = $("#ap-pass").value; body.confirm_until_revoked = $("#ap-forever-ok").checked; }
    }
    $("#ap-ok").disabled = $("#ap-deny").disabled = true;
    let succeeded = false;
    try {
      done(await post("/api/approval/decide", body));
      succeeded = true;
    } catch (e) {
      formError(e.message);
    } finally {
      $("#ap-ok").disabled = $("#ap-deny").disabled = false;
      // Keep inputs after a rejected submission so autofill problems can be
      // corrected locally; clear only after a successful decision.
      if (succeeded) for (const f of ["#ap-pass", "#ap-pass1", "#ap-pass2"]) $(f).value = "";
    }
  }
  async function start() {
    $("#ap-ok").addEventListener("click", () => decide("approve"));
    $("#ap-deny").addEventListener("click", () => decide("deny"));
    const m = location.hash.match(/a=(apr_[0-9a-f]+)&t=([0-9a-f]+)/);
    if (m) {
      history.replaceState(null, "", "/approve"); // the link is single-use and private
      try { const j = await post("/api/approval/open", { id: m[1], token: m[2] }); csrf = j.csrf; sessionStorage.setItem("ap_csrf", csrf); render(j); }
      catch (e) { gate(e.message); }
      return;
    }
    csrf = sessionStorage.getItem("ap_csrf") || "";
    const r = await fetch("/api/approval/view");
    const j = await r.json().catch(() => ({}));
    if (!r.ok) return gate(j.error);
    render(j);
  }
  start();
})();
