## Context

Greenfield Go project. We are building the reusable core of a tool that walks a
reconciliation graph across **heterogeneous resource types spanning different
backends** — Kubernetes today, but potentially GCP, other clouds, or anything
else. FluxCD (Kustomization → Helm → operator CRD → cloud) is the motivating
example, but those are only examples: the tool must be open to any resource type
and to arbitrary chaining, including switching backends mid-graph (a Kubernetes
object whose "real" child lives in GCP). Each hop has a *different* discovery
mechanism, so hop-specific logic must be fully isolated from the traversal.

This first increment delivers the domain-agnostic engine and the first resolver
(Kustomization → inventory) plus a generic Kubernetes fallback, driven by a CLI
that prints a tree. Later increments add resolvers — possibly in new domains —
without touching the engine.

Constraints: read-only; must handle arbitrary types (no compiled-in types for
children); must be unit-testable without a live cluster or cloud.

## Goals / Non-Goals

**Goals:**
- A **domain-agnostic** node reference and resolver contract: the engine assumes
  nothing about Kubernetes, GCP, or any backend.
- Resolvers own their own retrieval and may emit children in a **different
  domain** than their own (cross-backend hops).
- A traversal engine that is read-only, cycle-safe, partial-failure-tolerant,
  and supports arbitrary-depth chaining (incl. same-type chains).
- A matcher-based registry with per-domain fallback.
- First concrete domain: `kubernetes`, with a Kustomization resolver and a
  generic fallback.
- A CLI that renders the resolved tree with per-node domain/type/health.
- Offline unit tests via a fake Kubernetes client.

**Non-Goals:**
- Web UI (later increment).
- Any non-Kubernetes resolver (Helm, operator CRDs, GCP, …) — later increments;
  I1 only proves the abstraction with the `kubernetes` domain.
- Any write action (triggering reconcile, patching). Strictly read-only.
- Watch/streaming. One-shot traversal only.

## Decisions

### D1: Domain-agnostic reference (`Ref`)
The atom of the graph is a backend-neutral reference, not a Kubernetes object.
```go
type Ref struct {
    Domain  string            // "kubernetes", "gcp", … — extensible
    Type    string            // opaque within the domain: GVK string for k8s,
                              // resource type for a cloud, …
    Coords  map[string]string // e.g. {namespace, name} or {project, name, selfLink}
    Display string            // optional human label for rendering
}
func (r Ref) Key() string     // stable: domain|type|sorted(coords)
```
`Key()` drives dedup and cycle detection. **Alternative:** a Kubernetes-shaped
`{GVK, Namespace, Name}` ref — rejected: it hardcodes Kubernetes and blocks
cross-backend children (GCP, etc.), which is a core requirement.

### D2: Resolvers own retrieval; engine stays domain-blind
The engine does **no** backend I/O. Each resolver, given a `Ref` it matches,
fetches its own object (k8s GET, later a cloud API call, …) and interprets it.
```go
type Result struct {
    Health   Health   // Healthy | Unhealthy | Unknown | Error
    Detail   string   // human-readable (e.g. Ready message)
    Children []Ref    // may be in a DIFFERENT domain than the resolver
}
type Resolver interface {
    Matches(ref Ref) bool
    Resolve(ctx context.Context, rc *ResolveContext, ref Ref) (Result, error)
}
```
**Alternative:** engine fetches the object and passes it to the resolver (the
earlier design) — rejected: fetching is inherently domain-specific, so a
fetching engine cannot be domain-agnostic and cannot cross into GCP/other.

### D3: Shared clients via `ResolveContext`
To avoid every resolver re-wiring clients, a `ResolveContext` holds the shared,
lazily-built clients per domain (in I1: the Kubernetes dynamic client +
RESTMapper; later: a GCP client, etc.) and is injected into every `Resolve`
call. Cycle-tracking and graph assembly stay in the engine, so resolvers remain
pure-ish (object → result) and easy to unit test with a fake context.

### D4: Matcher-based registry with per-domain fallback
`Registry.For(ref) Resolver` returns the first registered resolver whose
`Matches(ref)` is true; otherwise the fallback registered for `ref.Domain`.
Resolvers declare interest by predicate, not by a fixed GVK key, so a resolver
can claim a single type, a family of types, or a whole domain. **Alternative:** a
`map[GVK]Resolver` — rejected: too narrow for domain-level or pattern matching
and Kubernetes-specific.

### D5: Engine algorithm (BFS, domain-blind, cycle-safe)
Iterative BFS over a queue of `Ref`. A `visited` set keyed by `Ref.Key()` guards
cycles and diamonds. For each ref: pick a resolver via the registry; call
`Resolve`; on error, emit an `Error` node carrying the reason and continue; else
record health + detail and enqueue children (whatever their domain). Output is a
`*Node` tree. A repeated ref becomes an "already-visited" leaf (not re-expanded)
so graph shape stays visible without duplicating subtrees. BFS over DFS: simpler
cycle handling, bounded recursion, stable output order.

### D6: Node/tree model separate from Result
`Result` is what a resolver returns. `Node` is engine output: it wraps the `Ref`,
`Health`, `Detail`, an optional error, and resolved child `*Node`s. Resolvers
never build tree structure.

### D7: Generic Kubernetes fallback is a *domain* fallback, not universal
"Read `.status.conditions[Ready]`" is Kubernetes semantics, so the generic
fallback is registered as the **`kubernetes`-domain** fallback, not a universal
one. Other domains get their own fallback when they are introduced. This keeps
"unknown type" a normal path *within* a domain while staying honest that health
semantics are domain-specific.

### D8: CLI with cobra; rendering decoupled and domain-aware
`cmd/fluxexp` uses `spf13/cobra`. The traverse command builds the
`ResolveContext`, seeds the root `Ref` in the `kubernetes` domain from
`--kind/--namespace/--name`, runs the engine, and passes the `*Node` to a
`render` package printing an indented tree (each line: domain, type, coords,
health). Rendering is a pure function of the tree → unit-testable and reusable by
the future web UI (same tree serialized to JSON).

### D9: Package layout
```
cmd/fluxexp/           CLI entry (cobra commands)
internal/engine/       Ref, Node, Health, BFS traversal (domain-blind)
internal/resolver/     Resolver interface, Registry, ResolveContext,
                       generic k8s fallback, kustomization resolver
internal/k8s/          kubeconfig, dynamic client, RESTMapper (feeds ResolveContext)
internal/render/       tree rendering of *Node
```
`internal/*` keeps the surface private until a stable API is warranted. Note the
engine no longer imports any k8s package — the dependency points inward from
resolvers to the engine's `Ref`/`Node` only.

## Risks / Trade-offs

- **Over-abstraction for one domain** (we build a domain-agnostic contract while
  only shipping `kubernetes`) → the abstraction is thin (a `Ref` struct + a
  matcher) and directly justified by the stated cross-backend requirement; the
  Kustomization resolver exercises it end-to-end. Accept.
- **`Coords` as `map[string]string`** loses compile-time structure per domain →
  each resolver owns/validates its own coord keys; documented per resolver and
  covered by tests. Accept for openness.
- **RESTMapper staleness** (a CRD added after mapper init isn't mapped) → build
  the mapper from live discovery per run; fine for a one-shot CLI.
- **Inventory format coupling** (Flux encodes entries as
  `<namespace>_<name>_<group>_<kind>` plus an apiVersion map) → isolate parsing
  in the Kustomization resolver with focused table tests; only that resolver
  changes if the format shifts across Flux versions.
- **Large graphs** slow (sequential fetches) → acceptable for I1; add bounded
  concurrency later. Note it, don't prematurely optimize.

## Open Questions

- Should an "already-visited" reference render as a leaf pointer or be omitted?
  Proposed: render as a leaf annotated `(already visited)`. Revisit when the UI
  needs real DAG semantics rather than a tree.
- Exit-code semantics: in I1 the exit code reflects command success, not
  aggregate health. A future flag (`--fail-on-unhealthy`) could make aggregate
  health affect the exit code.
- How a resolver signals "this child lives in another domain but I only know a
  partial address" (e.g. a selflink but not full coords) — defer until the first
  cross-domain resolver (GCP) actually lands; `Coords` is flexible enough to
  carry whatever that resolver needs.
