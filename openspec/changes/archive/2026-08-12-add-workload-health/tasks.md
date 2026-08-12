## 1. Workload resolver

- [x] 1.1 `internal/resolver/workload.go`: `WorkloadResolver` with a matcher for apps/v1 Deployment/StatefulSet/DaemonSet/ReplicaSet, v1 Pod, batch/v1 Job
- [x] 1.2 Replica-based health for Deployment/StatefulSet/ReplicaSet (desired from spec.replicas default 1, 0⇒healthy; ready from status.readyReplicas) with `<ready>/<desired> ready` detail
- [x] 1.3 DaemonSet health (numberReady vs desiredNumberScheduled, 0/0 healthy)
- [x] 1.4 Pod health (phase + Ready condition) with phase as detail
- [x] 1.5 Job health (Complete/Failed conditions)
- [x] 1.6 Resolve returns no children (leaf)

## 2. Centralize registry

- [x] 2.1 Add `resolver.NewDefaultRegistry()` (Kustomization + HelmRelease + Workload + generic fallback, correct order)
- [x] 2.2 `cmd/fluxexp/command/traverse.go` uses `NewDefaultRegistry`
- [x] 2.3 `internal/ui/service.go` uses `NewDefaultRegistry`

## 3. Tests

- [x] 3.1 Matcher: claims workload kinds, rejects ConfigMap (satisfies matcher scenarios)
- [x] 3.2 Replica health: fully-ready healthy, under-ready unhealthy, zero-desired healthy (Deployment/StatefulSet/ReplicaSet)
- [x] 3.3 DaemonSet: all-ready healthy, missing-pods unhealthy
- [x] 3.4 Pod: Running+Ready healthy, Running+not-ready unhealthy, Pending unknown, Succeeded healthy, Failed unhealthy
- [x] 3.5 Job: Complete healthy, Failed unhealthy, running unknown
- [x] 3.6 `NewDefaultRegistry`: a Deployment ref selects the workload resolver, a ConfigMap ref selects the fallback

## 4. Verification

- [x] 4.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --strict`
- [x] 4.2 Manual smoke: `traverse` a Kustomization/HelmRelease and confirm Deployments/Pods now show real health (fewer `unknown`); check the same in `ui`
- [x] 4.3 Update `README.md` roadmap
