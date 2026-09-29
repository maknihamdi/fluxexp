## 1. Dependency

- [x] 1.1 Add `sigs.k8s.io/cli-utils` to `go.mod` (`go get sigs.k8s.io/cli-utils`), run `make tidy`, and confirm the module graph gains only that module — `k8s.io/api`, `k8s.io/utils` and `kube-openapi` are already pulled by client-go
- [x] 1.2 Confirm `make build` and `make test` still pass with the dependency in place and nothing yet using it

## 2. Health model

- [x] 2.1 Add `Pending Health = "pending"` to `internal/engine/health.go`, documented as "the backend has not caught up with the declared spec", distinct from `Unknown`
- [x] 2.2 Extend the health-value validity check in `internal/engine/engine_test.go` to include `Pending`

## 3. Status derivation

- [x] 3.1 Create `internal/resolver/status.go`: map a `kstatus` verdict to `engine.Health` (`Current`→healthy, `InProgress`→pending, `Failed`→unhealthy, `Terminating`→pending, `NotFound`→unhealthy), surfacing the kstatus message as the detail
- [x] 3.2 Add the condition-polarity table (positive: `Synced`, `Configured`, `Established`, `Available`, `Healthy`, `Succeeded`, `Complete`; negative: `Degraded`, `Stalled`, `Failed`) with a comment explaining why polarity cannot be inferred from the status value
- [x] 3.3 Apply the polarity table **only** when the kstatus verdict is a generic `Current` — i.e. when kstatus recognised neither a kind-specific rule nor one of its three condition types — surfacing the deciding condition's message as the detail
- [x] 3.4 Override `Ready=False` to unhealthy, with a comment naming this as the one deliberate divergence from `flux` and pointing at `design.md` Decision 4
- [x] 3.5 Expose the generation-drift check as a reusable helper so `WorkloadResolver` can call it without reimplementing the comparison
- [x] 3.6 Replace the body of `K8sHealth` in `internal/resolver/resolver.go` with the new derivation; keep `healthFromReady` only if a caller still genuinely needs Ready-only semantics, otherwise delete it and update its callers
- [x] 3.7 Verify the six existing call sites (`internal/ui/service.go` ×2, `flux_object.go`, `helmrelease.go`, `kustomization.go`, `generic_k8s.go`, `cmd/fluxexp/command/list.go`) all go through `K8sHealth` and that none reimplements condition reading

## 4. Workload resolver

- [x] 4.1 In `internal/resolver/workload.go`, run the shared generation-drift check before the replica arithmetic for Deployment, StatefulSet and ReplicaSet, returning pending when the controller has not observed the current spec
- [x] 4.2 Confirm the existing replica, DaemonSet, Pod and Job health rules are otherwise unchanged, and that owner-reference descent is untouched
- [x] 4.3 Map a `Pending` Pod phase and a still-running Job to pending, leaving `unknown` to the `Unknown` phase alone — these were the last two places using `unknown` as a default
- [x] 4.4 Route the workload rules through `K8sHealth` so a UI layer that only lists a Pod reaches the same verdict as the CLI resolving it

## 5. Rendering

- [x] 5.1 Add a `pending` glyph in `internal/render/tree.go` `glyph()`, distinct from `✔` / `✖` / `!` / `?`
- [x] 5.2 Add a `badge.pending` CSS rule in `internal/ui/web/` (the badge already derives its class from the health value, so no JS change should be needed — confirm)
- [x] 5.3 Confirm `list --unhealthy` prints pending rows, since it filters on "not healthy", and that no code change is required

## 6. Tests

- [x] 6.1 Note in `internal/resolver/fake_test.go` that `metadata.generation` must be an `int64` — `kstatus.Compute` fails on the `float64` that `encoding/json` produces — and make the object builders take `int64`
- [x] 6.2 Table-driven tests in `internal/resolver/status_test.go` covering one case per measured family: A (ConfigMap, ClusterRole — no status → healthy), B (Bundle `Synced=False` → unhealthy, CRD `Established=True` → healthy, `Configured=False` → unhealthy), C (Certificate `Ready=True`/`Ready=False`), D (Namespace, Service, an operator CR with a status but no conditions)
- [x] 6.3 Polarity tests: `Degraded=False` → healthy, `Stalled=True` → unhealthy, an unknown condition type alone, and a condition message surfacing as the detail
- [x] 6.4 Generation-drift tests: generation ahead of observed → pending with both numbers in the detail; generation equal to observed → status from conditions; `Reconciling=True` → pending
- [x] 6.5 Update existing tests that asserted `unknown` for objects without a `Ready` condition (`generic_k8s_test.go`, `render/tree_test.go` and any UI test) to the new expected values
- [x] 6.6 Add a workload test for an unobserved spec change reading pending rather than unhealthy on stale replica counts

## 7. Verification

- [x] 7.1 `make test`, `make vet`, `make build` all green
- [x] 7.2 Smoke-test against `dev` (check the gcloud token is fresh first, per the cluster notes): traverse a Kustomization and confirm the previously-`unknown` inventory entries now carry real statuses
- [x] 7.3 Count the remaining `unknown`s across the cluster's inventories and confirm the drop from the measured 172/255 baseline; investigate any residual `unknown`
- [x] 7.4 Spot-check for false greens: find an object whose non-standard condition is currently false and confirm it renders unhealthy, not healthy
- [x] 7.5 Serve the UI and confirm through `/api/expand` that a pending row is emitted with the `pending` health value the badge CSS keys on and confirm the pending badge is legible and distinct in a layer, on the roots home and in a node header

## 8. Documentation

- [x] 8.1 Update `CLAUDE.md`: the health derivation paragraph, the new `pending` value, the polarity layer and the `Ready=False` divergence, plus the `int64` generation gotcha in the tests section
- [x] 8.2 Note the behaviour shift in the README roadmap entry for this increment — most former `unknown`s become `healthy`, some mid-rollout workloads move from `unhealthy` to `pending`