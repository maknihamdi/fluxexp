## Context

Workloads dominate a real cluster's graph and currently show as `unknown`
(no `Ready` condition of the shape the generic fallback reads). Their health is
deterministic from status fields, verified on the live cluster (Deployment:
`readyReplicas`/`replicas` + `Available`; DaemonSet: `numberReady`/
`desiredNumberScheduled`; StatefulSet: `readyReplicas`/`replicas`; Pod:
`phase` + `Ready`). This increment adds a resolver that computes it, improving
the CLI and the UI at once.

## Goals / Non-Goals

**Goals:**
- Accurate health for Deployment, StatefulSet, DaemonSet, ReplicaSet, Pod, Job.
- Concise per-node detail (`<ready>/<desired> ready`, or the pod phase).
- One shared `NewDefaultRegistry()` used by CLI and UI (no duplicated wiring).
- Pure, offline-testable health functions.

**Non-Goals:**
- Descending workloads into owned objects (Deployment → ReplicaSet → Pods) —
  later increment.
- Health for kinds with no health concept (ConfigMap, Service, Secret, …) — they
  stay `unknown`.
- Container-level or event-based diagnostics.

## Decisions

### D1: One multi-kind resolver, dispatch by kind
A single `WorkloadResolver` matches the workload kinds and switches on kind in
`Resolve`. **Alternative:** one resolver per kind — rejected: six near-identical
registrations and matchers; the health logic is small and cohesive enough to
live together.

### D2: Health rules (grounded on the live cluster)
- **Deployment / StatefulSet / ReplicaSet**: desired = `spec.replicas` (default
  1; 0 ⇒ healthy). ready = `status.readyReplicas` (absent ⇒ 0). healthy iff
  ready ≥ desired. Detail `<ready>/<desired> ready`.
- **DaemonSet**: desired = `status.desiredNumberScheduled`, ready =
  `status.numberReady`. healthy iff ready ≥ desired (0/0 healthy).
- **Pod**: `Running` ⇒ healthy iff `Ready` condition True else unhealthy;
  `Succeeded` ⇒ healthy; `Failed` ⇒ unhealthy; else unknown. Detail = phase.
- **Job**: `Complete` True ⇒ healthy; `Failed` True ⇒ unhealthy; else unknown.
Numbers are read tolerantly (unstructured stores them as int64/float64).

### D3: Leaves for now
The resolver returns no children. Removing `unknown` is the goal; descent to pods
is deferred so this increment stays small and low-risk. Documented as a
non-goal.

### D4: Centralize registry construction
Add `resolver.NewDefaultRegistry()` building Kustomization + HelmRelease +
Workload + generic fallback in the right order (specific matchers before the
fallback). `cmd/fluxexp/command/traverse.go` and `internal/ui/service.go` both
use it, so a future resolver is added in exactly one place. This also guarantees
the CLI and UI resolve identically.

### D5: File placement
`internal/resolver/workload.go` (+ `workload_test.go`) and
`internal/resolver/registry.go` gains `NewDefaultRegistry`. No engine/client
change; the resolver satisfies the existing contract and reuses `DecodeK8sRef`.

## Risks / Trade-offs

- **Replica field edge cases** (absent `readyReplicas`, `spec.replicas` nil) →
  default ready 0 and desired 1, with 0-desired treated healthy; covered by
  tests. A DaemonSet with `desiredNumberScheduled` 0 (no matching nodes) is
  healthy by design.
- **Pod churn** (a `Pending`/terminating pod shows unknown/unhealthy) → correct
  and informative; not a regression since pods were `unknown` before.
- **Ordering in the registry** → workload resolver must precede the fallback;
  `NewDefaultRegistry` fixes the order once for both surfaces.

## Open Questions

- Should a `Running` pod with some (not all) containers ready be `unhealthy` or a
  distinct "degraded"? For now the pod `Ready` condition (which aggregates
  containers) drives it; revisit if container-level detail is wanted.
- Rollout-in-progress Deployments (ready == desired but `Progressing` updating)
  are reported healthy; a future "progressing" nuance could use the
  `Progressing` condition + `updatedReplicas`.
