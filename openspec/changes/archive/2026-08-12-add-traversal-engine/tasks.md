## 1. Project scaffolding

- [x] 1.1 `go mod init github.com/maknihamdi/fluxexp`; add Go version 1.24
- [x] 1.2 Add dependencies: `k8s.io/client-go`, `k8s.io/apimachinery`, `spf13/cobra`; `go mod tidy`
- [x] 1.3 Create package layout: `cmd/fluxexp`, `internal/engine`, `internal/resolver`, `internal/k8s`, `internal/render`
- [x] 1.4 Add a minimal `README.md` and a `Makefile` with `build`, `test`, `run` targets

## 2. Engine core types (domain-blind)

- [x] 2.1 Define `Health` enum (`Healthy`, `Unhealthy`, `Unknown`, `Error`) in `internal/engine`
- [x] 2.2 Define `Ref{Domain, Type, Coords, Display}` with a stable `Key()` (domain|type|sorted coords)
- [x] 2.3 Define `Node{Ref, Health, Detail, Err, Children}` tree type
- [x] 2.4 Unit tests for `Ref.Key()` stability/equality across coord ordering (satisfies `resource-traversal` stable-key scenarios)

## 3. Resolver contract + registry + ResolveContext

- [x] 3.1 Define `Resolver{Matches(Ref) bool; Resolve(ctx, *ResolveContext, Ref) (Result, error)}` and `Result{Health, Detail, Children []Ref}` in `internal/resolver`
- [x] 3.2 Define `ResolveContext` holding lazily-built shared clients per domain (I1: Kubernetes client + RESTMapper)
- [x] 3.3 Implement `Registry` with `Register(Resolver)`, `RegisterFallback(domain, Resolver)`, and `For(Ref) Resolver` (first matcher wins, else per-domain fallback)
- [x] 3.4 Unit tests: specific-before-fallback selection, per-domain fallback, matcher rejects other domains (satisfies `resource-traversal` registry scenarios)

## 4. Generic Kubernetes fallback resolver

- [x] 4.1 Implement the generic k8s fallback: match any `kubernetes` ref; fetch via `ResolveContext`; health from `.status.conditions[Ready]` (True/False/absent → Healthy/Unhealthy/Unknown); no children
- [x] 4.2 Register it as the `kubernetes`-domain fallback
- [x] 4.3 Unit tests with a fake k8s client: Ready True/False/absent (satisfies `resource-traversal` generic-fallback scenarios)

## 5. Traversal engine

- [x] 5.1 Implement BFS traversal seeded from a root `Ref`; per node call `Registry.For(ref).Resolve(...)`; engine performs no domain I/O
- [x] 5.2 Add `visited` set keyed by `Ref.Key()`; render a repeat as an "already visited" leaf (no re-expansion)
- [x] 5.3 On resolver error, emit an `Error` node with the reason and continue siblings/subtrees
- [x] 5.4 Enqueue returned children regardless of their domain; assemble the `*Node` tree with per-node health
- [x] 5.5 Unit tests with stub resolvers: tree build, mixed-domain tree, same-type chaining, cycle safety, partial failure, per-node health (satisfies `resource-traversal` engine/mixed-domain/chaining/cycle/partial-failure/health scenarios)

## 6. Flux Kustomization resolver

- [x] 6.1 Implement the matcher: claim `kubernetes` refs whose type is `kustomize.toolkit.fluxcd.io/*, Kind=Kustomization`
- [x] 6.2 Resolve: fetch the Kustomization via `ResolveContext`; parse `.status.inventory.entries` (`id` + `v` apiVersion) into `kubernetes` child `Ref`s
- [x] 6.3 Report health from the Kustomization `Ready` condition; surface message as detail on False
- [x] 6.4 Register the resolver in the registry
- [x] 6.5 Unit tests: matcher claims Kustomization only, inventory→children, empty/absent inventory, Ready True/False (satisfies `flux-kustomization-resolver` scenarios)

## 7. Kubernetes client wiring

- [x] 7.1 Implement kubeconfig loading (`KUBECONFIG`/default path) with optional `--context`
- [x] 7.2 Build a dynamic client + discovery-backed `RESTMapper`; expose a `Get(ctx, Ref) (*unstructured.Unstructured, error)` helper (namespaced vs cluster-scoped via mapper) consumed by resolvers through `ResolveContext`
- [x] 7.3 Return a clear error when no cluster is reachable

## 8. CLI + rendering

- [x] 8.1 Implement `render.Tree(*Node) string`: indented tree; each line shows domain, type, coordinates, health; error nodes distinct with reason
- [x] 8.2 Implement the `traverse` cobra command with `--kind/--namespace/--name`, `--kubeconfig`, `--context` flags; build the root `Ref` in the `kubernetes` domain
- [x] 8.3 Wire command: build `ResolveContext` → seed root `Ref` → run engine → `render.Tree` → print; exit non-zero with a clear message on missing start resource or unreachable cluster
- [x] 8.4 Unit tests for `render.Tree` (healthy nesting, error node) (satisfies `cli` tree-rendering scenarios)

## 9. List command

- [x] 9.1 Add `List(ctx, apiVersion, kind, namespace) ([]unstructured, error)` to the k8s client (namespace empty = all namespaces)
- [x] 9.2 Export `resolver.K8sHealth(obj) (engine.Health, string)` reusing the Ready-condition logic
- [x] 9.3 Implement `render` list output (rows with namespace/name + health) and unit-test it
- [x] 9.4 Implement the `list` cobra command: `--kind`, `--api-version`, `-A/--all-namespaces`, `-n`, `--unhealthy`; map known Flux kinds to apiVersions; clear error on unresolvable kind
- [x] 9.5 Smoke `list` against the real cluster (all kustomizations, and `--unhealthy`)

## 10. Verification

- [x] 10.1 `go build ./...` and `go vet ./...` clean; `go test ./...` green
- [x] 10.2 Manual smoke against a real cluster: traverse from an existing Kustomization and confirm inventory children appear with health
- [x] 10.3 Update `README.md` with usage example and the increment roadmap
