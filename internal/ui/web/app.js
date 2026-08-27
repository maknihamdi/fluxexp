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

// inlineFields renders fields compactly on a single row, for list entries.
function inlineFields(fields) {
  const wrap = document.createElement("div");
  wrap.className = "detail inline-fields";
  for (const f of fields) {
    const l = document.createElement("span");
    l.className = "iflabel";
    l.textContent = f.label.toLowerCase();
    const v = document.createElement("span");
    v.className = "ifvalue";
    v.textContent = f.value;
    if (f.full) v.title = f.full;
    wrap.appendChild(l);
    wrap.appendChild(v);
  }
  return wrap;
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

// listEntry renders one list entry: its row, plus its own dependencies nested
// under it when it has any — so a listed Kustomization shows its source and what
// it waits on without being opened.
function listEntry(node, { dependency } = {}) {
  const row = nodeRow(node, { onClick: () => drillTo(node.ref), dependency });
  const deps = node.dependencies || [];
  if (!deps.length) return row;

  const group = document.createElement("div");
  group.className = "entry";
  group.appendChild(row);
  const nested = document.createElement("div");
  nested.className = "nested-deps";
  for (const dep of deps) {
    nested.appendChild(nodeRow(dep, { onClick: () => drillTo(dep.ref), dependency: true }));
  }
  group.appendChild(nested);
  return group;
}

function nodeRow(node, { onClick, dependency } = {}) {
  const row = document.createElement("div");
  // An expandable node carries a subtree: the accent tells the user, before any
  // click, that opening it leads somewhere. A dependency is styled apart: it is
  // something the node needs, not something it produces.
  const classes = ["row"];
  if (dependency) classes.push("dep");
  else if (node.expandable) classes.push("container");
  row.className = classes.join(" ");
  row.appendChild(badge(node.health));

  const grow = document.createElement("div");
  grow.className = "grow";
  const ref = node.ref || {};
  const name = document.createElement("div");
  name.className = "name";
  if (dependency) {
    const arrow = document.createElement("span");
    arrow.className = "chevron dep-arrow";
    arrow.textContent = "\u21e2";
    arrow.title = "Required by this resource";
    name.appendChild(arrow);
  } else if (node.expandable) {
    const chev = document.createElement("span");
    chev.className = "chevron";
    chev.textContent = "\u25b8";
    chev.title = "Contains other resources — click to open";
    name.appendChild(chev);
  }
  name.appendChild(document.createTextNode(ref.display || `${ref.type} ${ref.name}`));
  grow.appendChild(name);
  // Flux objects carry their fields inline — repo, branch, interval — so the
  // reader never has to open a row to learn where the code comes from.
  if (node.fields && node.fields.length) {
    grow.appendChild(inlineFields(node.fields));
  } else {
    const detail = document.createElement("div");
    detail.className = "detail";
    detail.textContent = node.error ? node.error : (node.detail || "");
    grow.appendChild(detail);
  }
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
  if (node.expandable) {
    const chev = document.createElement("span");
    chev.className = "chevron";
    chev.textContent = "\u25b8";
    t.appendChild(chev);
  }
  t.appendChild(document.createTextNode(node.ref.display || `${node.ref.type} ${node.ref.name}`));
  hh.appendChild(t);
  head.appendChild(hh);
  if (node.fields && node.fields.length) head.appendChild(fieldsPanel(node.fields));
  if (node.error || (node.detail && !(node.fields && node.fields.length))) {
    const d = document.createElement("div");
    d.className = "msg";
    d.textContent = node.error || node.detail;
    head.appendChild(d);
  }

  // Dependencies live inside the node's own card: what it needs, grouped with
  // it, rather than mixed into what it applies. No block at all when there are
  // none.
  const deps = node.dependencies || [];
  if (deps.length) {
    const wrap = document.createElement("div");
    wrap.className = "deps";
    const dt = document.createElement("div");
    dt.className = "deps-title";
    dt.textContent = "depends on";
    wrap.appendChild(dt);
    for (const dep of deps) {
      wrap.appendChild(listEntry(dep, { dependency: true }));
    }
    head.appendChild(wrap);
  }
  v.appendChild(head);

  const title = document.createElement("div");
  title.className = "section-title";
  const children = node.children || [];
  title.textContent = `applies · ${children.length}`;
  v.appendChild(title);

  if (children.length === 0) {
    const e = document.createElement("div");
    e.className = "empty";
    e.textContent = "Applies nothing — this is a leaf in the current layer.";
    v.appendChild(e);
    return;
  }

  for (const child of children) {
    v.appendChild(listEntry(child));
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
