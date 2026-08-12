## Context

The CLI (`list`, `traverse`) proved the engine and resolvers against a live
cluster. This increment puts a visual, read-only portal on top of exactly that
machinery: pick a context, see the root Kustomizations, and drill into the graph
one layer at a time. It must stay self-contained (single Go binary), require no
auth, and read only the local machine's state (kubeconfig now; `gcloud` later).

## Goals / Non-Goals

**Goals:**
- `fluxexp ui` starts a loopback-bound HTTP server serving an embedded SPA.
- List kubeconfig contexts and switch the active one.
- Home: root Kustomizations for a context with health + useful info.
- Layer-by-layer, click-by-click expansion reusing the CLI resolvers (single hop
  per interaction), with error nodes tolerated.
- Two navigation modes: drill-in with a breadcrumb trail, or open a new
  exploration.
- Flag when a hop would need a different context.

**Non-Goals:**
- Any cloud/`gcloud` integration (later increment).
- Authentication, RBAC, remote exposure (loopback only).
- Any write action (reconcile/suspend/patch). Read-only.
- A JS build pipeline / framework — the frontend is dependency-free and embedded.
- Resolving whole subtrees eagerly; expansion is always one hop.

## Decisions

### D1: Embedded dependency-free SPA (Go `embed`)
The frontend is plain HTML/CSS/JS embedded via `//go:embed` and served by the Go
binary. No node build step, no framework, no external runtime assets (CSP-safe,
offline). **Alternative:** React/Vite built and embedded — rejected now for build
complexity; revisit only if the UI outgrows vanilla JS.

### D2: JSON HTTP API reusing the resolvers
Endpoints (all read-only), served same-origin:
- `GET /api/contexts` → `{contexts:[{name,current}]}` from the kubeconfig.
- `GET /api/roots?context=&namespace=` → root Kustomizations with health + info.
- `GET /api/expand?context=&domain=&type=&ns=&name=` → the resolved node plus its
  immediate children (each with health). One hop only.
The engine's `Traverse` (full BFS) stays for the CLI; the UI uses a **single-hop**
resolution built on `Registry` + `ResolveContext` (already returns one node's
`Result{health, detail, children}`), so both surfaces share identical resolver
behavior.

### D3: Per-context client cache
A small service holds a `map[context]*k8s.Client`, building a client lazily on
first use of a context (via `k8s.LoadClient("", context)`). Add
`k8s.ListContexts()` to read context names + current-context from the kubeconfig.
Clients are read-only and cheap to keep for the server's lifetime.

### D4: Expansion = full-resolve the node + cheap health for each child
Expanding node N runs N's full resolver once (yielding N's health/detail and its
child **refs**). To render each child with health without prematurely expanding
it, the backend computes a **cheap health** per child: fetch the object and apply
`resolver.K8sHealth` (the Ready-condition model) — it does *not* run the child's
full resolver, so a HelmRelease child shows health without decoding its chart.
The child's own children are only computed when the user expands that child.
**Alternative:** fully resolve every child — rejected: it would eagerly do
expensive work (Helm decode, inventory walks) for nodes the user may never open.

### D5: Roots listing + info extraction
`GET /api/roots` reuses `client.List` for Kustomizations in the requested
namespace (default `flux`; a flag/param can widen to all namespaces). For each,
a pure extractor reads: health (`K8sHealth`), `spec.sourceRef` (kind/name),
`spec.path`, `spec.interval`, `status.lastAppliedRevision`, and the Ready
condition's `lastTransitionTime` + `message`. Grounded on the live cluster
(verified fields).

### D6: Navigation model in the frontend
The SPA keeps an in-memory exploration state: an ordered **trail** of expanded
nodes (breadcrumb). Drill-in appends to the trail on the same page; "open a new
exploration" opens a new browser tab/route seeded from the chosen resource,
leaving the current trail intact. State lives client-side; the backend is
stateless per request (context + ref are passed each call).

### D7: Context carried through exploration; cross-context flagged
Every explored node is resolved under the **selected context**, which the SPA
carries in each request and displays prominently. A child ref may declare a
target context; when it differs from the active one, the SPA shows a
"switch context to continue" notice instead of resolving it under the wrong
context. On a single-cluster setup this notice is not triggered, but the
mechanism (per-node context + comparison) is in place for when cross-context
links exist.

### D8: Security posture
The server binds `127.0.0.1` by default (configurable via `--address`). The API
is unauthenticated and returns cluster data, so off-host exposure is opt-in and
the operator's responsibility. No CORS is enabled (same-origin only). Strictly
read-only handlers.

### D9: Package layout
```
internal/ui/          HTTP server, handlers, JSON DTOs, //go:embed assets
internal/ui/web/      index.html, app.js, styles.css (embedded)
internal/k8s/         + ListContexts()
cmd/fluxexp/command/  + ui.go (the `ui` command)
```
`internal/ui` imports engine/resolver/k8s; the engine and resolvers are unchanged.

## Risks / Trade-offs

- **Unauthenticated local server** → loopback bind by default + explicit opt-in
  for any other address; documented. Read-only handlers limit blast radius.
- **Cheap-health double fetch** (child fetched for health, re-fetched on expand)
  → acceptable; fetches are fast and cached at neither layer for simplicity. Can
  add a short-lived cache later.
- **Vanilla JS ceiling** → fine for a tree/drill-in UI; if interactions grow
  (search, graph view) we can introduce a build step in a later increment.
- **Large root lists / wide namespaces** → default to the `flux` namespace;
  widening is explicit. Same sequential-fetch trade-off as the CLI.
- **Context-change flag not exercised** on single-cluster clusters → mechanism
  present but dormant; honestly documented rather than faked.

## Open Questions

- Should "open a new exploration" be a new browser tab or an in-app secondary
  panel? Proposed: a new tab/route (simplest, matches "leave the current page").
- Should roots optionally include non-`flux` namespaces by detecting true graph
  roots (Kustomizations not referenced by any other inventory)? Deferred; the
  namespace default covers the common case now.
- Auto-open the browser on `ui`? Proposed: just print the URL (predictable,
  scriptable); an `--open` flag can come later.
