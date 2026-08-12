## Why

Operators debugging a FluxCD-managed platform can see that a `Kustomization`
reconciled, but not what it ultimately produced several hops downstream (Helm
releases, operator CRDs, a real cloud resource). The `flux` CLI stops at the
Flux object boundary. We need to **follow the reconciliation graph to the end**
and report health along the way. This first increment builds the reusable
traversal core and proves it against the first hop (Kustomization → the objects
it applied), so later increments can plug in each additional hop without
reworking the engine.

## What Changes

- New Go CLI binary `fluxexp` with a command that takes a starting resource
  (`kind`, `namespace`, `name`) and prints the resolved resource **tree** with
  a health status per node.
- A **traversal engine** that, starting from one object, recursively discovers
  children by delegating to a resolver selected per resource type (GVK). Walk
  is **read-only**, **cycle-safe** (visited-set on GVK/namespace/name), and
  **partial-failure-tolerant** (a failed node becomes an error node; siblings
  still traverse).
- A **resolver registry** keyed by GVK plus a **generic fallback resolver**
  that handles any unregistered GVK: it reports the object's `Ready` condition
  (if present) and returns no children.
- A **Flux Kustomization resolver** that discovers children from
  `.status.inventory.entries` and reports the Kustomization's own health.
- Kubernetes access via standard kubeconfig using a dynamic client (so
  arbitrary CRDs can be fetched without compiled-in types).

Out of scope for this increment: the web UI, HelmRelease/CRD/GCP resolvers,
and any write actions (triggering a reconcile).

## Capabilities

### New Capabilities

- `resource-traversal`: the resolver contract, the GVK-keyed resolver registry,
  the generic fallback resolver, and the traversal engine semantics
  (read-only, cycle safety, partial-failure handling, per-node health model).
- `flux-kustomization-resolver`: discovering a Kustomization's children from
  `.status.inventory.entries` and reporting the Kustomization's health.
- `cli`: the `fluxexp` command interface — selecting a start resource, cluster
  connection via kubeconfig, and rendering the resolved graph as a tree.

### Modified Capabilities

<!-- None: this is the first change; no existing specs. -->

## Impact

- New Go module and source tree (`cmd/fluxexp`, `internal/engine`,
  `internal/resolver`, `internal/k8s`). No existing code affected (greenfield).
- New dependencies: `k8s.io/client-go` (dynamic client, kubeconfig),
  `k8s.io/apimachinery` (unstructured, GVK), and a CLI framework
  (e.g. `spf13/cobra`).
- Requires a reachable cluster (kubeconfig) at runtime; engine and resolvers are
  unit-testable offline via the fake dynamic client / fixtures.
