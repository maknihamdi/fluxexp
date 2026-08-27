"use strict";

// The URL is the source of truth for where the user is: state.trail is a
// projection of it, written by every navigation and re-read on popstate, so a
// shared link and a Back press land in exactly the same view.
const state = {
  context: "",
  trail: [], // array of ref DTOs (drill path); empty => home
};

const DOMAIN_K8S = "kubernetes";

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

// ---- refs and URLs --------------------------------------------------------

// labelFor mirrors friendlyLabel in internal/ui/dto.go: "<Kind> <ns>/<name>".
// A trail rebuilt from the URL carries no display label — it is derived from the
// type instead, so a shared link can never show a stale one.
function labelFor(ref) {
  const typ = (ref && ref.type) || "";
  const i = typ.lastIndexOf("|");
  const kind = i >= 0 ? typ.slice(i + 1) : typ;
  const name = (ref && ref.name) || "";
  if (ref && ref.namespace) return `${kind} ${ref.namespace}/${name}`;
  if (name) return `${kind} ${name}`;
  return kind;
}

const refLabel = (ref) => (ref && ref.display) || labelFor(ref);

// encodeComponent escapes a hop component on its own — so a separator inside a
// name can never break parsing — then restores "/" and "|", which are legal in a
// query string and keep the address bar readable ("…fluxcd.io/v1|Kustomization").
function encodeComponent(v) {
  // encodeURIComponent leaves "~" alone, and "~" separates hops — escape it
  // explicitly before restoring the two characters kept literal.
  return encodeURIComponent(v || "")
    .replace(/~/g, "%7E")
    .replace(/%2F/g, "/")
    .replace(/%7C/g, "|");
}

// encodeHop renders a ref as "type:ns:name", prefixed with the domain only when
// it is not the default — every hop is kubernetes today, a future gcp hop says so.
function encodeHop(ref) {
  const parts = [ref.type || "", ref.namespace || "", ref.name || ""];
  if (ref.domain && ref.domain !== DOMAIN_K8S) parts.unshift(ref.domain);
  return parts.map(encodeComponent).join(":");
}

// decodeHop is the inverse; it throws on anything it cannot read, so a mangled
// path is reported rather than turned into a half-built ref.
function decodeHop(hop) {
  const parts = hop.split(":").map((v) => decodeURIComponent(v));
  let domain = DOMAIN_K8S;
  if (parts.length === 4) domain = parts.shift();
  else if (parts.length !== 3) throw new Error(`bad path segment "${hop}"`);
  const ref = { domain, type: parts[0], namespace: parts[1], name: parts[2] };
  if (!ref.type || !ref.name) throw new Error(`bad path segment "${hop}"`);
  ref.display = labelFor(ref);
  return ref;
}

const encodeTrail = (trail) => trail.map(encodeHop).join("~");
const decodeTrail = (p) => (p ? p.split("~").map(decodeHop) : []);

// urlFor builds an address by hand: URLSearchParams' form-urlencoded serializer
// would escape "~" and ":" into %7E and %3A and lose the trail's readability.
function urlFor(contextName, trail) {
  const parts = [];
  if (contextName) parts.push(`context=${encodeURIComponent(contextName)}`);
  const p = encodeTrail(trail || []);
  if (p) parts.push(`p=${p}`);
  return parts.length ? `/?${parts.join("&")}` : "/";
}

// rawParam reads a query parameter WITHOUT decoding it. URLSearchParams would
// percent-decode the whole value first, turning an encoded ":" inside a name
// into a real separator before each component gets its own decode.
function rawParam(name) {
  const q = location.search.replace(/^\?/, "");
  if (!q) return null;
  for (const kv of q.split("&")) {
    const i = kv.indexOf("=");
    if ((i < 0 ? kv : kv.slice(0, i)) === name) return i < 0 ? "" : kv.slice(i + 1);
  }
  return null;
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
  // state.context may already be set from the URL; otherwise fall back to the
  // kubeconfig's current-context.
  const current = (contexts.find((c) => c.current) || contexts[0] || {}).name || "";
  state.context = state.context || current;
  syncContextBar();
  sel.onchange = () => {
    // A trail is a path through one cluster: carrying it into another context
    // would address objects that may not exist there.
    state.context = sel.value;
    navigateTo([]);
  };
}

// syncContextBar reflects state.context in the selector and the header, so a
// history entry carrying another context is shown as such.
function syncContextBar() {
  el("#context").value = state.context;
  el("#ctx-active").textContent = state.context ? `→ ${state.context}` : "";
}

// ---- rendering ------------------------------------------------------------

function renderBreadcrumb() {
  const bar = crumbEl();
  const c = el("#crumbs");
  // The bar holds the up control, so it is hidden as a whole on the roots home:
  // there is no parent to go up to.
  if (state.trail.length === 0) {
    bar.hidden = true;
    c.innerHTML = "";
    return;
  }
  bar.hidden = false;
  el("#up").onclick = goUp;
  c.innerHTML = "";
  const home = document.createElement("a");
  home.textContent = "roots";
  home.onclick = () => navigateTo([]);
  c.appendChild(home);
  state.trail.forEach((ref, i) => {
    const sep = document.createElement("span");
    sep.className = "sep";
    sep.textContent = " / ";
    c.appendChild(sep);
    const a = document.createElement("a");
    a.textContent = refLabel(ref);
    a.onclick = () => navigateTo(state.trail.slice(0, i + 1));
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
  name.appendChild(document.createTextNode(refLabel(ref)));
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
  v.innerHTML = `<div class="spinner">Resolving ${refLabel(ref)}…</div>`;

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
  t.appendChild(document.createTextNode(refLabel(node.ref)));
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

// navigateTo moves to a trail: it writes the URL first, then renders from the
// new state — the order boot and popstate follow too, so every path into a view
// goes through the same steps.
function navigateTo(trail) {
  state.trail = trail;
  syncContextBar();
  history.pushState(null, "", urlFor(state.context, state.trail));
  render();
}

function drillTo(ref) {
  navigateTo(state.trail.concat([ref]));
}

// goUp is a forward navigation, not history.back(): the previous history entry
// may be a sibling the user jumped to, or another exploration entirely — the
// parent is a property of the trail, not of where the user came from.
function goUp() {
  navigateTo(state.trail.slice(0, -1));
}

function newExploration(ref) {
  window.open(urlFor(state.context, [ref]), "_blank");
}

// readURL projects the URL onto state — the single decoder used by boot and by
// popstate. It returns a message when the path cannot be decoded, the trail then
// falling back to the roots home rather than rendering half of it.
function readURL() {
  const ctx = rawParam("context");
  if (ctx) state.context = decodeURIComponent(ctx);
  try {
    state.trail = decodeTrail(rawParam("p"));
    return "";
  } catch (e) {
    state.trail = [];
    return e.message;
  }
}

// ---- boot -----------------------------------------------------------------

window.addEventListener("popstate", () => {
  const err = readURL();
  if (err) toast(`Cannot read that path: ${err}`);
  syncContextBar();
  render();
});

(async function main() {
  const err = readURL();
  try {
    await loadContexts();
  } catch (e) {
    toast(`Cannot list contexts: ${e.message}`);
    return;
  }
  if (err) toast(`Cannot read that path: ${err}`);
  // Normalize the address of the entry the user landed on, without adding a
  // history entry of its own.
  history.replaceState(null, "", urlFor(state.context, state.trail));
  render();
})();
