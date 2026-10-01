"use strict";

// The URL is the source of truth for where the user is: state.trail is a
// projection of it, written by every navigation and re-read on popstate, so a
// shared link and a Back press land in exactly the same view. Expanding a branch
// in the tree is not a navigation and never touches the URL.
const state = {
  context: "",
  trail: [], // array of ref DTOs (path to the selected node); empty => roots level
};

const DOMAIN_K8S = "kubernetes";
const HEALTHS = ["healthy", "pending", "unhealthy", "unknown", "error"];

// tree holds what has been fetched: one entry per expanded reference, each
// carrying the layer it returned and when it was read. Nothing here is ever
// rendered as current without its age beside it, and collapsing discards.
const tree = {
  roots: null,          // RootDTO[] for the roots level
  rootsAt: 0,
  nodes: new Map(),     // refKey -> { node: NodeDTO, at: ms }
  open: new Set(),      // refKeys whose children are revealed
  loading: new Set(),   // refKeys being fetched
};

const filter = { q: "", hidden: new Set() };

// detail holds the selected node's own resolution. It is never taken from the
// tree: selecting always re-resolves, so what the node pane shows was read when
// it was shown.
const detail = { node: null, at: 0, seq: 0 };

const el = (sel) => document.querySelector(sel);
const viewEl = () => el("#view");
const treeEl = () => el("#tree");

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
const kindOf = (ref) => {
  const typ = (ref && ref.type) || "";
  const i = typ.lastIndexOf("|");
  return i >= 0 ? typ.slice(i + 1) : typ;
};
const nameOf = (ref) => (ref && ref.namespace ? `${ref.namespace}/${ref.name}` : (ref && ref.name) || "");
const refKey = (ref) =>
  `${ref.domain || DOMAIN_K8S}|${ref.type || ""}|${ref.namespace || ""}|${ref.name || ""}`;
const sameRef = (a, b) => a && b && refKey(a) === refKey(b);

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

function expandQuery(ref) {
  const p = new URLSearchParams({
    context: state.context,
    domain: ref.domain || DOMAIN_K8S,
    type: ref.type || "",
    ns: ref.namespace || "",
    name: ref.name || "",
  });
  return `/api/expand?${p.toString()}`;
}

// ---- small builders -------------------------------------------------------

const elm = (tag, cls, txt) => {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (txt != null) e.textContent = txt;
  return e;
};

// mark renders health as a shape as well as a colour, so the five values stay
// apart for a colour-blind reader and in a greyscale screenshot. An empty health
// is a reference not read yet, which is not the same as "unknown".
function mark(health) {
  const h = (health || "").toLowerCase();
  const m = elm("span", `mk ${h || "unread"}`);
  m.title = h || "not read yet";
  return m;
}

function ageText(at) {
  if (!at) return "";
  const s = Math.round((Date.now() - at) / 1000);
  if (s < 5) return "just now";
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  return `${Math.floor(m / 60)}h${String(m % 60).padStart(2, "0")} ago`;
}

// ageEl is re-read by tickAges, so an age on screen never silently freezes.
function ageEl(at) {
  const e = elm("span", "age", ageText(at));
  e.dataset.at = String(at);
  return e;
}

function tickAges() {
  for (const e of document.querySelectorAll(".age[data-at]")) {
    e.textContent = ageText(Number(e.dataset.at));
  }
}

// clickable makes a row activate like a button while staying a <div>: a row holds
// its own controls (expand, re-read, new exploration), and a <button> may not
// contain buttons.
function clickable(e, onClick) {
  e.setAttribute("role", "button");
  e.tabIndex = 0;
  e.onclick = onClick;
  e.onkeydown = (ev) => {
    if (ev.key !== "Enter" && ev.key !== " ") return;
    ev.preventDefault();
    onClick();
  };
  return e;
}

function rereadButton(title, onClick) {
  const b = elm("button", "reread", "↻");
  b.type = "button";
  b.title = title;
  b.onclick = (ev) => { ev.stopPropagation(); onClick(); };
  return b;
}

function newExpButton(ref) {
  const b = elm("button", "newexp", "↗");
  b.type = "button";
  b.title = "Open a new exploration from here";
  b.onclick = (ev) => { ev.stopPropagation(); newExploration(ref); };
  return b;
}

// tallyOf counts health values over a layer, for the summary line and the bar.
function tallyOf(entries) {
  const counts = {};
  for (const e of entries) counts[e.health || "unknown"] = (counts[e.health || "unknown"] || 0) + 1;
  return counts;
}

function layerSummary(entries) {
  const counts = tallyOf(entries);
  const tally = elm("div", "tally");
  for (const h of HEALTHS) {
    if (!counts[h]) continue;
    const s = elm("span");
    s.appendChild(mark(h));
    s.appendChild(document.createTextNode(`${counts[h]} ${h}`));
    tally.appendChild(s);
  }
  const bar = elm("div", "bar");
  for (const h of HEALTHS) {
    if (!counts[h]) continue;
    const seg = elm("i", h);
    seg.style.flex = String(counts[h]);
    bar.appendChild(seg);
  }
  return { tally, bar };
}

// ---- filter ---------------------------------------------------------------

// matches answers for one entry alone. A branch is kept when it matches or when
// something under it does — filtering must never hide the path to a match.
function matches(entry) {
  if (filter.hidden.has(entry.health || "unknown")) return false;
  if (!filter.q) return true;
  const ref = entry.ref || {};
  return `${kindOf(ref)} ${ref.namespace || ""} ${ref.name || ""}`.toLowerCase().includes(filter.q);
}

// branchMatches descends into what is held. `seen` carries the keys already on
// this path: the Flux bootstrap Kustomization applies itself, so a graph walk
// without that guard never terminates.
function branchMatches(entry, seen) {
  if (matches(entry)) return true;
  const key = refKey(entry.ref);
  if (seen.has(key)) return false;
  const held = tree.nodes.get(key);
  if (!held) return false;
  const next = new Set(seen).add(key);
  return (held.node.children || []).some((c) => branchMatches(c, next));
}

const filtered = (entries, seen) => entries.filter((e) => branchMatches(e, seen || new Set()));

function buildHealthFilter() {
  const box = el("#hfilter");
  for (const h of ["healthy", "pending", "unhealthy"]) {
    const b = elm("button", "hf");
    b.type = "button";
    b.setAttribute("aria-pressed", "true");
    b.appendChild(mark(h));
    b.appendChild(document.createTextNode(h));
    b.onclick = () => {
      if (filter.hidden.has(h)) filter.hidden.delete(h); else filter.hidden.add(h);
      b.setAttribute("aria-pressed", String(!filter.hidden.has(h)));
      renderTree();
      renderDetailBody();
    };
    box.appendChild(b);
  }
}

// ---- fetching -------------------------------------------------------------

// loadBranch resolves one reference and holds its layer. Every call is a fresh
// read: nothing here is served from what is already held.
async function loadBranch(ref) {
  const key = refKey(ref);
  tree.loading.add(key);
  renderTree();
  try {
    const node = await api(expandQuery(ref));
    tree.nodes.set(key, { node, at: Date.now() });
  } catch (e) {
    tree.open.delete(key);
    toast(e.message);
  } finally {
    tree.loading.delete(key);
    renderTree();
  }
}

async function loadRoots() {
  const { roots } = await api(`/api/roots?context=${encodeURIComponent(state.context)}`);
  tree.roots = roots;
  tree.rootsAt = Date.now();
}

function resetTree() {
  tree.roots = null;
  tree.rootsAt = 0;
  tree.nodes.clear();
  tree.open.clear();
  tree.loading.clear();
}

// ---- tree pane ------------------------------------------------------------

function renderTree() {
  const t = treeEl();
  t.innerHTML = "";

  const head = elm("div", "tgroup-label");
  head.appendChild(document.createTextNode("root kustomizations · ns flux"));
  if (tree.rootsAt) {
    head.appendChild(ageEl(tree.rootsAt));
    head.appendChild(rereadButton("Re-read the root Kustomizations", refreshRoots));
  }
  t.appendChild(head);

  if (tree.roots === null) {
    t.appendChild(elm("div", "tloading", "loading…"));
    return;
  }
  const roots = filtered(tree.roots);
  if (!roots.length) {
    t.appendChild(elm("div", "tloading", tree.roots.length ? "nothing matches the filter" : "no root Kustomizations"));
  }
  for (const r of roots) t.appendChild(treeBranch(r, [r.ref], 0, new Set()));

  // A trail that does not start at a root — a new exploration opened from any
  // reference — still gets its spine, so the path is visible rather than absent.
  if (state.trail.length && !tree.roots.some((r) => sameRef(r.ref, state.trail[0]))) {
    t.appendChild(elm("div", "tgroup-label", "exploration"));
    t.appendChild(treeBranch({ ref: state.trail[0] }, [state.trail[0]], 0, new Set()));
  }
}

// treeBranch renders one row plus, when open, the layer it holds. `entry` is what
// the parent layer said about the reference; a fresher read of the reference
// itself wins over it.
function treeBranch(entry, path, depth, seen) {
  const frag = document.createDocumentFragment();
  const ref = entry.ref;
  const key = refKey(ref);
  // A reference already on this path is a repeat — the Flux bootstrap
  // Kustomization applies itself. It is shown as a leaf pointer, like the
  // engine's visited node, rather than descended into again.
  const repeat = seen.has(key);
  const held = repeat ? null : tree.nodes.get(key);
  const node = held ? held.node : entry;
  const children = held ? (held.node.children || []) : [];
  const spine = !held && !repeat && isTrailPrefix(path) ? state.trail[path.length] : null;
  const canOpen = !repeat && node.expandable !== false;

  const row = elm("div", "trow" + (canOpen ? " container" : ""));
  row.style.paddingLeft = `${0.5 + depth * 0.9}rem`;
  if (sameRef(ref, state.trail[state.trail.length - 1])) row.setAttribute("aria-current", "true");

  const twist = elm("button", "twist" + (canOpen ? "" : " leaf"), "▶");
  twist.type = "button";
  twist.setAttribute("aria-expanded", String(tree.open.has(key)));
  twist.setAttribute("aria-label", `${tree.open.has(key) ? "Collapse" : "Expand"} ${refLabel(ref)}`);
  twist.onclick = (ev) => { ev.stopPropagation(); toggleBranch(ref); };
  row.appendChild(twist);
  row.appendChild(mark(node.health));

  const name = elm("span", "tname");
  name.appendChild(elm("em", null, kindOf(ref)));
  name.appendChild(document.createTextNode(nameOf(ref)));
  if (repeat) {
    name.appendChild(elm("span", "tcount", " ↩"));
    name.title = "Already shown higher on this path";
  }
  row.appendChild(name);

  const meta = elm("div", "tmeta");
  if (held) {
    meta.appendChild(elm("span", "tcount", String(children.length)));
    meta.appendChild(ageEl(held.at));
    meta.appendChild(rereadButton(`Re-read ${refLabel(ref)}`, () => loadBranch(ref)));
  }
  row.appendChild(meta);
  clickable(row, () => navigateTo(path));
  frag.appendChild(row);

  if (tree.loading.has(key)) {
    const l = elm("div", "tloading", "reading…");
    l.style.paddingLeft = `${1.4 + depth * 0.9}rem`;
    frag.appendChild(l);
  }
  const below = new Set(seen).add(key);
  if (tree.open.has(key) && !repeat) {
    for (const c of filtered(children, below)) {
      frag.appendChild(treeBranch(c, path.concat([c.ref]), depth + 1, below));
    }
  } else if (spine) {
    // The trail continues through here but this layer was never read: show the
    // next hop from its own identifier rather than hiding the path.
    frag.appendChild(treeBranch({ ref: spine }, path.concat([spine]), depth + 1, below));
  }
  return frag;
}

// isTrailPrefix reports whether `path` is the trail's own prefix, so the spine is
// drawn along the current path and nowhere else.
function isTrailPrefix(path) {
  if (path.length >= state.trail.length) return false;
  return path.every((r, i) => sameRef(r, state.trail[i]));
}

async function toggleBranch(ref) {
  const key = refKey(ref);
  if (tree.open.has(key)) {
    // Collapsing discards what the branch held, so re-opening reads again rather
    // than restoring a layer nobody can vouch for.
    tree.open.delete(key);
    tree.nodes.delete(key);
    renderTree();
    return;
  }
  tree.open.add(key);
  if (tree.nodes.has(key)) renderTree();
  else await loadBranch(ref);
}

async function refreshRoots() {
  try {
    await loadRoots();
  } catch (e) {
    toast(e.message);
  }
  renderTree();
  if (state.trail.length === 0) renderDetailBody();
}

// ---- node pane ------------------------------------------------------------

function render() {
  renderTree();
  renderDetail();
}

// renderDetail resolves the selected reference — always, even when the tree
// already holds its layer — and paints it.
async function renderDetail() {
  const seq = ++detail.seq;
  const v = viewEl();

  if (state.trail.length === 0) {
    detail.node = null;
    if (tree.roots === null) {
      v.innerHTML = "";
      v.appendChild(elm("div", "spinner", "Loading root Kustomizations…"));
      try {
        await loadRoots();
      } catch (e) {
        if (seq !== detail.seq) return;
        v.innerHTML = "";
        toast(e.message);
        return;
      }
      if (seq !== detail.seq) return;
      renderTree();
    }
    renderDetailBody();
    return;
  }

  const ref = state.trail[state.trail.length - 1];
  v.innerHTML = "";
  v.appendChild(elm("div", "spinner", `Resolving ${refLabel(ref)}…`));
  // The roots level backs the tree; fetch it once so the path is visible even on
  // a deep link, without blocking the node itself.
  if (tree.roots === null) loadRoots().then(renderTree).catch(() => {});

  let node;
  try {
    node = await api(expandQuery(ref));
  } catch (e) {
    if (seq !== detail.seq) return;
    v.innerHTML = "";
    toast(e.message);
    return;
  }
  if (seq !== detail.seq) return;

  detail.node = node;
  detail.at = Date.now();
  // Selecting reveals: the layer just read is the tree's, too.
  tree.nodes.set(refKey(ref), { node, at: detail.at });
  if (node.expandable) tree.open.add(refKey(ref));
  renderTree();
  renderDetailBody();
}

// renderDetailBody paints what is already held — used again when a filter
// changes, which must never trigger a call.
function renderDetailBody() {
  if (state.trail.length === 0) return renderRootsLevel();
  if (!detail.node) return;
  const node = detail.node;
  const ref = node.ref || {};
  const v = viewEl();
  v.innerHTML = "";

  if (node.context && state.context && node.context !== state.context) {
    v.appendChild(elm("div", "notice",
      `This resource resolves under context "${node.context}". Switch context to continue.`));
  }

  const head = elm("div", "head");
  const top = elm("div", "head-top");
  top.appendChild(elm("div", "kind", kindOf(ref)));
  const h1 = elm("h1");
  if (ref.namespace) h1.appendChild(elm("span", null, `${ref.namespace}/`));
  h1.appendChild(document.createTextNode(ref.name || ""));
  top.appendChild(h1);

  const marks = elm("div", "head-marks");
  const hc = elm("span", `chip ${(node.health || "unknown").toLowerCase()}`);
  hc.appendChild(mark(node.health));
  hc.appendChild(document.createTextNode(node.health || "unknown"));
  marks.appendChild(hc);
  if (node.freshness) {
    const f = elm("span", "chip fresh");
    f.appendChild(document.createTextNode(node.freshness));
    marks.appendChild(f);
  }
  const read = elm("span", "read");
  read.appendChild(document.createTextNode("read "));
  read.appendChild(ageEl(detail.at));
  read.appendChild(rereadButton("Re-read this node", renderDetail));
  marks.appendChild(read);
  const up = elm("button", "up", "↑ parent");
  up.type = "button";
  up.title = "Go up to the parent";
  up.onclick = goUp;
  marks.appendChild(up);
  marks.appendChild(newExpButton(ref));
  top.appendChild(marks);
  head.appendChild(top);

  const extras = rootExtras(ref);
  const msg = node.error || node.detail || extras.message;
  if (msg) head.appendChild(elm("div", "msg", msg));
  const fields = (node.fields || []).concat(extras.fields);
  if (fields.length) head.appendChild(fieldsPanel(fields));

  // Dependencies live inside the node's own block: what it needs, grouped with
  // it, rather than mixed into what it applies. No block at all when there are none.
  const deps = node.dependencies || [];
  if (deps.length) {
    const box = elm("div", "deps");
    box.appendChild(elm("div", "deps-h", "depends on"));
    for (const dep of deps) box.appendChild(depRow(dep));
    head.appendChild(box);
  }
  v.appendChild(head);

  const children = node.children || [];
  const shown = filtered(children, new Set([refKey(ref)]));
  const lh = elm("div", "layer-h");
  lh.appendChild(elm("h2", null,
    `applies · ${shown.length === children.length ? children.length : `${shown.length} of ${children.length}`}`));
  const { tally, bar } = layerSummary(children);
  lh.appendChild(tally);
  v.appendChild(lh);
  if (children.length) v.appendChild(bar);

  if (!children.length) {
    v.appendChild(elm("div", "empty", "Applies nothing — this is a leaf in the current layer."));
    return;
  }
  if (!shown.length) {
    v.appendChild(elm("div", "empty", "Nothing in this layer matches the filter."));
    return;
  }

  const rows = elm("div", "rows");
  for (const child of shown) {
    rows.appendChild(childRow(child));
    const cdeps = child.dependencies || [];
    if (cdeps.length) {
      const sub = elm("div", "subdeps");
      for (const dep of cdeps) sub.appendChild(depRow(dep));
      rows.appendChild(sub);
    }
  }
  v.appendChild(rows);
}

// renderRootsLevel is the node pane at the roots level: the same rows as the
// tree, with the information a root carries in its own manifest.
function renderRootsLevel() {
  const v = viewEl();
  v.innerHTML = "";
  const roots = tree.roots || [];
  const shown = filtered(roots);

  const lh = elm("div", "layer-h");
  lh.appendChild(elm("h2", null,
    `root kustomizations · ${shown.length === roots.length ? roots.length : `${shown.length} of ${roots.length}`}`));
  const { tally, bar } = layerSummary(roots);
  lh.appendChild(tally);
  const read = elm("span", "read");
  read.appendChild(document.createTextNode("read "));
  read.appendChild(ageEl(tree.rootsAt));
  read.appendChild(rereadButton("Re-read the root Kustomizations", refreshRoots));
  lh.appendChild(read);
  v.appendChild(lh);
  if (roots.length) v.appendChild(bar);

  if (!roots.length) {
    v.appendChild(elm("div", "empty", "No root Kustomizations in namespace 'flux' for this context."));
    return;
  }
  if (!shown.length) {
    v.appendChild(elm("div", "empty", "No root Kustomization matches the filter."));
    return;
  }

  const rows = elm("div", "rows");
  for (const r of shown) {
    const row = rootRow(r);
    rows.appendChild(row);
  }
  v.appendChild(rows);
}

function rootRow(r) {
  const row = elm("div", "row container");
  row.appendChild(mark(r.health));
  row.appendChild(elm("span", "rchev", "▶"));
  row.appendChild(elm("span", "rkind", "Kustomization"));
  row.appendChild(elm("span", "rname", nameOf(r.ref)));

  // Freshness has a cell of its own, so it survives the narrow layout where the
  // rest of the row's fields are dropped. The root's path, source and interval
  // are shown when it is opened, from this same listing.
  row.appendChild(r.freshness ? elm("span", "chip fresh", r.freshness) : elm("span", "rfresh"));
  const detailCell = elm("div", "rdetail inline-fields");
  for (const f of r.fields || []) {
    detailCell.appendChild(elm("span", "iflabel", f.label.toLowerCase()));
    const v = elm("span", "ifvalue", f.value);
    if (f.full) v.title = f.full;
    detailCell.appendChild(v);
  }
  row.appendChild(detailCell);
  row.appendChild(newExpButton(r.ref));
  row.title = r.message || "";
  return clickable(row, () => navigateTo([r.ref]));
}

// rootExtras reads a root Kustomization's path, interval and Ready message from
// the roots listing already in hand. They live in its spec, not in its status,
// and /api/expand does not carry them — reading them here costs no extra call.
function rootExtras(ref) {
  const r = (tree.roots || []).find((x) => sameRef(x.ref, ref));
  if (!r) return { fields: [], message: "" };
  const fields = [];
  if (r.path) fields.push({ label: "Path", value: r.path });
  if (r.interval) fields.push({ label: "Interval", value: r.interval });
  if (r.lastTransition) fields.push({ label: "Since", value: r.lastTransition });
  return { fields, message: r.message || "" };
}

function fieldsPanel(fields) {
  const dl = elm("dl", "fields");
  for (const f of fields) {
    dl.appendChild(elm("dt", null, f.label));
    const dd = elm("dd", null, f.value);
    if (f.full) dd.title = f.full;
    dl.appendChild(dd);
  }
  return dl;
}

function childRow(node) {
  const ref = node.ref || {};
  const row = elm("div", "row" + (node.expandable ? " container" : ""));
  row.appendChild(mark(node.health));
  row.appendChild(elm("span", "rchev" + (node.expandable ? "" : " leaf"), "▶"));
  row.appendChild(elm("span", "rkind", kindOf(ref)));
  row.appendChild(elm("span", "rname", nameOf(ref)));
  // Freshness is never claimed for a listed entry — the cell keeps the columns
  // aligned with the roots level, which does carry one.
  row.appendChild(elm("span", "rfresh"));
  row.appendChild(detailCell(node));
  row.appendChild(newExpButton(ref));
  return clickable(row, () => drillTo(ref));
}

function depRow(node) {
  const ref = node.ref || {};
  const row = elm("div", "dep");
  row.appendChild(mark(node.health));
  row.appendChild(elm("span", "rkind", kindOf(ref)));
  row.appendChild(elm("span", "rname", nameOf(ref)));
  row.appendChild(detailCell(node));
  return clickable(row, () => drillTo(ref));
}

// detailCell shows a Flux object's own fields inline — repo, branch, interval —
// so the reader never has to open a row to learn where the code comes from.
function detailCell(node) {
  if (node.fields && node.fields.length) {
    const wrap = elm("div", "rdetail inline-fields");
    for (const f of node.fields) {
      wrap.appendChild(elm("span", "iflabel", f.label.toLowerCase()));
      const v = elm("span", "ifvalue", f.value);
      if (f.full) v.title = f.full;
      wrap.appendChild(v);
    }
    return wrap;
  }
  const d = elm("div", "rdetail", node.error || node.detail || "");
  if (node.error || node.detail) d.title = node.error || node.detail;
  return d;
}

// ---- context bar ----------------------------------------------------------

async function loadContexts() {
  const { contexts } = await api("/api/contexts");
  const sel = el("#context");
  sel.innerHTML = "";
  for (const c of contexts) {
    const o = document.createElement("option");
    o.value = c.name;
    // In a pod there is no kubeconfig to name contexts from, and the server
    // reports a single sentinel. Label it with the cluster it points at, so the
    // bar reads as the cluster rather than as a magic word.
    const label = c.name === "in-cluster" && c.cluster ? c.cluster : c.name;
    o.textContent = c.current ? `${label} (current)` : label;
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
    resetTree();
    navigateTo([]);
  };
}

function syncContextBar() {
  el("#context").value = state.context;
}

// ---- navigation -----------------------------------------------------------

// navigateTo moves to a trail: it writes the URL first, then renders from the
// new state — the order boot and popstate follow too, so every path into a view
// goes through the same steps.
function navigateTo(trail) {
  state.trail = trail;
  syncContextBar();
  history.pushState(null, "", urlFor(state.context, state.trail));
  closeDrawer();
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

// ---- drawer (narrow viewports) --------------------------------------------

function closeDrawer() {
  treeEl().classList.remove("open");
  el("#drawer").setAttribute("aria-expanded", "false");
}

el("#drawer").onclick = () => {
  const open = treeEl().classList.toggle("open");
  el("#drawer").setAttribute("aria-expanded", String(open));
};

el("#q").oninput = (e) => {
  filter.q = e.target.value.trim().toLowerCase();
  renderTree();
  renderDetailBody();
};

// ---- boot -----------------------------------------------------------------

window.addEventListener("popstate", () => {
  const err = readURL();
  if (err) toast(`Cannot read that path: ${err}`);
  syncContextBar();
  render();
});

(async function main() {
  const err = readURL();
  buildHealthFilter();
  setInterval(tickAges, 15000);
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
