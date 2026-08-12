# fluxexp — project context

## What this is

`fluxexp` is a CLI (and, later, an embedded web UI) that **traverses the full
dependency graph of resources reconciled by FluxCD**, from a starting Flux
object all the way down to the concrete cloud resource it ultimately produces.

The goal is not to replace the `flux` CLI (which already lists Kustomizations,
shows last reconciliation, triggers a sync). The goal is to **follow the chain
to the end**:

```
Kustomization (flux) ──▶ GitRepository / another Kustomization
        │  discovered via .status.inventory.entries
        ▼
   HelmRelease (flux) ──▶ objects rendered by the Helm chart
        │  discovered via the Helm release storage (sh.helm.release.v1)
        ▼
   Operator CRD (e.g. Config Connector / Crossplane)
        │  discovered via ownerReferences; health via .status.conditions[Ready]
        ▼
   Real cloud resource (e.g. GCP) ──▶ verified via the cloud API
```

The Flux/Helm/GCP chain above is only an **example**. The tool must stay open to
**any resource type**, arbitrary chaining (e.g. Kustomization → Kustomization),
and **switching backends mid-graph** (a Kubernetes object whose real child lives
in GCP or elsewhere).

Each hop uses a **different discovery mechanism**. The core abstraction is a
**domain-agnostic engine** plus a **registry of resolvers selected by matcher**.
A node is a backend-neutral reference (`domain`, `type`, `coordinates`). Given a
reference it matches, a resolver fetches the object **itself** (via shared
clients) and returns (a) health and (b) child references — which MAY be in a
**different domain**. The engine only walks the graph (BFS), dedupes by a stable
key, and never performs any backend I/O.

This mirrors the pluggable "one handler per kind" pattern: adding support for a
new hop — even in a new backend — = registering a new resolver, without touching
the engine.

## Tech stack

- **Language**: Go (1.24). Native to the k8s/Flux ecosystem: `client-go`,
  `controller-runtime`, and the Flux API types are all Go.
- **CLI**: single static binary. Tree/text output first.
- **UI (later)**: a web app served by the same binary (embedded frontend) to
  visualize the resource graph. Not in scope for the first increments.
- **Cluster access**: standard kubeconfig; dynamic client for arbitrary CRDs.

## Architectural principles

1. **Engine is domain-agnostic.** The traversal engine knows nothing about
   Flux, Helm, Kubernetes, or GCP, and performs no backend I/O. It only knows:
   given a reference, ask the registry for a resolver, get health + children,
   recurse. Backend-specific logic never leaks into the engine.
2. **Resolvers own retrieval and are selected by matcher.** A resolver declares
   which references it handles via a predicate, fetches its own object through
   shared clients, and may emit children in another domain. A per-domain generic
   fallback handles unknown types (e.g. the Kubernetes fallback reports `Ready`).
3. **Discovery is read-only.** Traversal never mutates cluster state. Triggering
   a reconcile is a separate, explicit action (later increment).
4. **Cycle-safe.** The engine tracks visited (GVK, namespace, name) to avoid
   infinite loops on ownership cycles.
5. **Partial failure is a first-class result.** A node that can't be fetched or
   whose resolver errors is rendered as an error node; traversal of siblings
   continues.

## Increment roadmap (rough)

- **I1 (current)**: traversal engine + Kustomization resolver (reads
  `.status.inventory`) + generic fallback resolver. CLI tree output. No UI.
- I2: HelmRelease resolver (Helm release storage).
- I3: operator-CRD leaf resolver + Ready aggregation.
- I4: GCP verification (Config Connector self-link → cloud API).
- I5: web UI.

## Conventions

- Go module path: TBD at first `apply` (e.g. `github.com/<org>/fluxexp`).
- Package layout (proposed): `cmd/fluxexp` (CLI entry), `internal/engine`
  (traversal), `internal/resolver` (registry + resolvers), `internal/k8s`
  (client wiring).
- Tests: table-driven, using fake dynamic client / recorded fixtures so the
  engine and resolvers are testable without a live cluster.
