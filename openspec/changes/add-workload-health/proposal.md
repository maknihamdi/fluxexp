## Why

Core Kubernetes workloads (Deployment, StatefulSet, DaemonSet, ReplicaSet, Pod,
Job) have no `Ready` condition of the shape the generic fallback reads, so they
render as `unknown` in both the CLI tree and the web UI — the single biggest
source of noise when exploring a real cluster. Their health is well-defined from
replica counts and phase/conditions; this increment computes it, turning those
`unknown` leaves into accurate `healthy`/`unhealthy` nodes everywhere.

## What Changes

- New **workload health resolver** claiming the core workload kinds:
  `apps/v1` Deployment, StatefulSet, DaemonSet, ReplicaSet; `v1` Pod; `batch/v1`
  Job. It reports health from each kind's status (no children — leaves):
  - Deployment / StatefulSet / ReplicaSet: ready replicas vs desired.
  - DaemonSet: number ready vs desired scheduled.
  - Pod: `.status.phase` + the `Ready` condition.
  - Job: `Complete` / `Failed` conditions.
  Each surfaces a concise detail (e.g. `2/3 ready`).
- Registered ahead of the generic Kubernetes fallback so workloads get precise
  health while every other kind keeps its current behavior.
- **Refactor**: a single `resolver.NewDefaultRegistry()` builds the standard
  resolver set, used by both the CLI (`traverse`) and the web UI service, so
  adding a resolver no longer means editing two places.

Out of scope: descending a workload into its owned objects (Deployment →
ReplicaSet → Pods); this increment is health-only. Kinds with no health concept
(ConfigMap, Service, Secret, …) intentionally stay `unknown`.

## Capabilities

### New Capabilities

- `workload-health-resolver`: computing health for the core Kubernetes workload
  kinds from their status (replica counts, phase, conditions).

### Modified Capabilities

<!-- None: the engine, registry mechanism and existing resolvers are unchanged;
     this adds a resolver and centralizes registry construction. -->

## Impact

- New `internal/resolver/workload.go` (+ tests).
- New `resolver.NewDefaultRegistry()`; `cmd/fluxexp/command/traverse.go` and
  `internal/ui/service.go` switch to it (removing duplicated registration).
- No engine or client change. Improves both CLI and UI health output at once.
