## Why

I4 gave workloads real health but left them as leaves, so exploration stops at
the Deployment/StatefulSet and never reaches the Pods that are actually running.
This increment descends from a workload to the objects it owns — the first
**ownerReference-based descent** — so the graph reaches the running Pod, the
"real thing" on a Kubernetes cluster. The same owner-descent mechanism will later
generalize to operator-managed resources.

## What Changes

- The workload resolver stops being a leaf: it discovers children via
  **`.metadata.ownerReferences`** (matching the parent's UID):
  - Deployment → its active ReplicaSets (revisions with replicas > 0);
    scaled-to-0 old revisions are omitted and their count is surfaced on the
    Deployment's detail (no silent drop).
  - ReplicaSet / StatefulSet / DaemonSet / Job → their Pods.
  - Pod → leaf.
- A reusable owner-descent helper: list a child kind in the parent's namespace
  and keep those whose `ownerReferences` include the parent's UID.
- The resolver gains list access via the shared cluster client (the `K8sGetter`
  contract gains `List`).

Out of scope: descending arbitrary operator CRDs (later — this proves the
mechanism on workloads), and cross-namespace ownership (owners are same-namespace
for these kinds).

## Capabilities

### Modified Capabilities

- `workload-health-resolver`: workloads are no longer leaves; the resolver now
  descends to owned objects via ownerReferences (Deployment → ReplicaSets →
  Pods, StatefulSet/DaemonSet/Job → Pods).

## Impact

- `internal/resolver/workload.go`: add owner-descent; `resolver.K8sGetter` gains
  `List` (already implemented by `*k8s.Client` and the UI cluster).
- `internal/resolver/*_test.go` and `internal/ui` fakes implement `List`.
- No engine change. Improves both CLI and UI (the UI's single-hop expansion now
  reveals Pods under workloads).
