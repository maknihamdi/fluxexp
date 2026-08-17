"use strict";

const state = {
  context: "",
  trail: [], // array of ref DTOs (drill path); empty => home
};

const el = (sel) => document.querySelector(sel);
const viewEl = () => el("#view");
const crumbEl = () => el("#breadcrumb");

async function api(path) {
  const res = await fetch(path);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `HTTP ${res.status}`);
  return body;
}

function toast(msg) {
  const t = el("#toast");
  t.textContent = msg;
  t.hidden = false;
  setTimeout(() => (t.hidden = true), 4000);
}

function badge(health) {
  const h = (health || "unknown").toLowerCase();
  const b = document.createElement("span");
  b.className = `badge ${h}`;
  b.innerHTML = `<span class="dot"></span>${h}`;
  return b;
}

// freshnessBadge returns a badge for a freshness value, or null when empty.
function freshnessBadge(freshness) {
  if (!freshness) return null;
  const b = document.createElement("span");
  b.className = `badge fresh ${freshness}`;
  b.innerHTML = `<span class="dot"></span>${freshness}`;
  return b;
}

// fieldsPanel renders label/value fields; the full value (if any) is the title.
function fieldsPanel(fields) {
  const wrap = document.createElement("div");
  wrap.className = "fields";
  for (const f of fields || []) {
    const row = document.createElement("div");
    row.className = "field";
    const l = document.createElement("span");
    l.className = "flabel";
    l.textContent = f.label;
    const v = document.createElement("span");
    v.className = "fvalue";
    v.textContent = f.value;
    if (f.full) v.title = f.full;
    row.appendChild(l);
    row.appendChild(v);
    wrap.appendChild(row);
  }
  return wrap;
}

function expandQuery(ref) {
  const p = new URLSearchParams({
    context: state.context,
    domain: ref.domain || "kubernetes",
    type: ref.type || "",
    ns: ref.namespace || "",
    name: ref.name || "",
  });
  return `/api/expand?${p.toString()}`;
}

// ---- context bar ----------------------------------------------------------

async function loadContexts() {
  const { contexts } = await api("/api/contexts");
  const sel = el("#context");
  sel.innerHTML = "";
  for (const c of contexts) {
    const o = document.createElement("option");
    o.value = c.name;
    o.textContent = c.current ? `${c.name} (current)` : c.name;
    sel.appendChild(o);
  }
  const url = new URLSearchParams(location.search);
  const wanted = url.get("context");
  const current = (contexts.find((c) => c.current) || contexts[0] || {}).name || "";
  state.context = wanted || current;
  sel.value = state.context;
  el("#ctx-active").textContent = state.context ? `→ ${state.context}` : "";
  sel.onchange = () => {
    state.context = sel.value;
    el("#ctx-active").textContent = `→ ${state.context}`;
    state.trail = [];
    render();
  };
}

// ---- rendering ------------------------------------------------------------

function renderBreadcrumb() {
  const c = crumbEl();
  if (state.trail.length === 0) {
    c.hidden = true;
    c.innerHTML = "";
    return;
  }
  c.hidden = false;
  c.innerHTML = "";
  const home = document.createElement("a");
  home.textContent = "roots";
  home.onclick = () => { state.trail = []; syncURL(); render(); };
  c.appendChild(home);
  state.trail.forEach((ref, i) => {
    const sep = document.createElement("span");
    sep.className = "sep";
    sep.textContent = " / ";
    c.appendChild(sep);
    const a = document.createElement("a");
    a.textContent = ref.display || `${ref.type} ${ref.name}`;
    a.onclick = () => { state.trail = state.trail.slice(0, i + 1); syncURL(); render(); };
    c.appendChild(a);
  });
}

function nodeRow(node, { onClick } = {}) {
  const row = document.createElement("div");
  row.className = "row";
  row.appendChild(badge(node.health));

  const grow = document.createElement("div");
  grow.className = "grow";
  const ref = node.ref || {};
  const name = document.createElement("div");
  name.className = "name";
  name.textContent = ref.display || `${ref.type} ${ref.name}`;
  const detail = document.createElement("div");
  detail.className = "detail";
  detail.textContent = node.error ? node.error : (node.detail || "");
  grow.appendChild(name);
  grow.appendChild(detail);
  row.appendChild(grow);

  const nx = document.createElement("button");
  nx.className = "newexp";
  nx.textContent = "↗ new";
  nx.title = "Open a new exploration from here";
  nx.onclick = (e) => { e.stopPropagation(); newExploration(ref); };
  row.appendChild(nx);

  if (onClick) row.onclick = onClick;
  return row;
}

async function renderHome() {
  renderBreadcrumb();
  const v = viewEl();
  v.innerHTML = `<div class="spinner">Loading root Kustomizations…</div>`;
  let roots;
  try {
    ({ roots } = await api(`/api/roots?context=${encodeURIComponent(state.context)}`));
  } catch (e) {
    v.innerHTML = "";
    toast(e.message);
    return;
  }
  v.innerHTML = "";
  const title = document.createElement("div");
  title.className = "section-title";
  title.textContent = `root kustomizations · ns flux · ${roots.length}`;
  v.appendChild(title);

  if (roots.length === 0) {
    const e = document.createElement("div");
    e.className = "empty";
    e.textContent = "No root Kustomizations in namespace 'flux' for this context.";
    v.appendChild(e);
    return;
  }

  for (const r of roots) {
    const card = document.createElement("div");
    card.className = "card";
    const head = document.createElement("div");
    head.className = "card-head";
    head.appendChild(badge(r.health));
    const fb = freshnessBadge(r.freshness);
    if (fb) head.appendChild(fb);
    const t = document.createElement("span");
    t.className = "title";
    t.textContent = `${r.ref.namespace}/${r.ref.name}`;
    head.appendChild(t);
    card.appendChild(head);

    if (r.fields && r.fields.length) card.appendChild(fieldsPanel(r.fields));

    const meta = document.createElement("div");
    meta.className = "meta";
    const bits = [];
    if (r.sourceKind) bits.push(`<span>source <b>${r.sourceKind}/${r.sourceName}</b></span>`);
    if (r.path) bits.push(`<span>path <b>${r.path}</b></span>`);
    if (r.interval) bits.push(`<span>interval <b>${r.interval}</b></span>`);
    meta.innerHTML = bits.join("");
    card.appendChild(meta);

    if (r.message) {
      const m = document.createElement("div");
      m.className = "msg";
      m.textContent = r.message;
      card.appendChild(m);
    }

    card.style.cursor = "pointer";
    card.onclick = () => drillTo(r.ref);
    v.appendChild(card);
  }
}

async function renderExplore() {
  renderBreadcrumb();
  const ref = state.trail[state.trail.length - 1];
  const v = viewEl();
  v.innerHTML = `<div class="spinner">Resolving ${ref.display || ref.name}…</div>`;

  let node;
  try {
    node = await api(expandQuery(ref));
  } catch (e) {
    v.innerHTML = "";
    toast(e.message);
    return;
  }
  v.innerHTML = "";

  if (node.context && state.context && node.context !== state.context) {
    const n = document.createElement("div");
    n.className = "notice";
    n.textContent = `This resource resolves under context "${node.context}". Switch context to continue.`;
    v.appendChild(n);
  }

  // Current node header.
  const head = document.createElement("div");
  head.className = "card";
  const hh = document.createElement("div");
  hh.className = "card-head";
  hh.appendChild(badge(node.health));
  const nfb = freshnessBadge(node.freshness);
  if (nfb) hh.appendChild(nfb);
  const t = document.createElement("span");
  t.className = "title";
  t.textContent = node.ref.display || `${node.ref.type} ${node.ref.name}`;
  hh.appendChild(t);
  head.appendChild(hh);
  if (node.fields && node.fields.length) head.appendChild(fieldsPanel(node.fields));
  if (node.error || (node.detail && !(node.fields && node.fields.length))) {
    const d = document.createElement("div");
    d.className = "msg";
    d.textContent = node.error || node.detail;
    head.appendChild(d);
  }
  v.appendChild(head);

  const title = document.createElement("div");
  title.className = "section-title";
  const children = node.children || [];
  title.textContent = `children · ${children.length}`;
  v.appendChild(title);

  if (children.length === 0) {
    const e = document.createElement("div");
    e.className = "empty";
    e.textContent = "No children — this is a leaf in the current layer.";
    v.appendChild(e);
    return;
  }

  for (const child of children) {
    v.appendChild(nodeRow(child, { onClick: () => drillTo(child.ref) }));
  }
}

function render() {
  if (state.trail.length === 0) renderHome();
  else renderExplore();
}

// ---- navigation -----------------------------------------------------------

function drillTo(ref) {
  state.trail.push(ref);
  syncURL();
  render();
}

function newExploration(ref) {
  const p = new URLSearchParams({
    context: state.context,
    e_domain: ref.domain || "kubernetes",
    e_type: ref.type || "",
    e_ns: ref.namespace || "",
    e_name: ref.name || "",
    e_display: ref.display || "",
  });
  window.open(`/?${p.toString()}`, "_blank");
}

// Keep the top-level context in the URL so a reload/new tab is reproducible.
function syncURL() {
  const p = new URLSearchParams();
  if (state.context) p.set("context", state.context);
  history.replaceState(null, "", `/?${p.toString()}`);
}

function seedFromURL() {
  const u = new URLSearchParams(location.search);
  const type = u.get("e_type");
  const name = u.get("e_name");
  if (type && name) {
    state.trail = [{
      domain: u.get("e_domain") || "kubernetes",
      type,
      namespace: u.get("e_ns") || "",
      name,
      display: u.get("e_display") || "",
    }];
  }
}

// ---- boot -----------------------------------------------------------------

(async function main() {
  try {
    await loadContexts();
  } catch (e) {
    toast(`Cannot list contexts: ${e.message}`);
    return;
  }
  seedFromURL();
  render();
})();
