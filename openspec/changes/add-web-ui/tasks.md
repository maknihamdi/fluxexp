## 1. Kubeconfig contexts

- [x] 1.1 Add `k8s.ListContexts() ([]ContextInfo, error)` (name + current flag) reading the standard kubeconfig
- [x] 1.2 Unit test with a temp kubeconfig fixture (contexts listed, current marked)

## 2. Backend service (reuses engine/resolvers)

- [x] 2.1 `internal/ui`: a service holding a per-context `*k8s.Client` cache (lazy `LoadClient("", ctx)`) and a shared resolver `Registry` (Kustomization + HelmRelease + generic fallback)
- [x] 2.2 Root extraction: pure `rootSummary(obj) RootDTO` reading health (`K8sHealth`), sourceRef, path, interval, lastAppliedRevision, Ready lastTransitionTime + message
- [x] 2.3 `Roots(ctx, namespace)`: list Kustomizations (default `flux`) → `[]RootDTO`
- [x] 2.4 `Expand(ctx, ref)`: full-resolve the node (health/detail + child refs) then compute cheap `K8sHealth` per child → `NodeDTO{ref, health, detail, err, children:[NodeDTO]}` (children unexpanded)
- [x] 2.5 Unit tests with a fake getter + registry: roots extraction, expand returns children with health, error node on unresolvable ref

## 3. HTTP server + DTOs

- [x] 3.1 JSON DTOs (`ContextDTO`, `RootDTO`, `RefDTO`, `NodeDTO`) with a stable ref encoding (domain/type/coords)
- [x] 3.2 Handlers: `GET /api/contexts`, `GET /api/roots`, `GET /api/expand`; validate params; JSON errors; read-only
- [x] 3.3 Bind `127.0.0.1` by default; serve embedded assets at `/`; graceful shutdown on signal
- [x] 3.4 Handler tests via `httptest` against the service with a fake cluster access

## 4. Frontend (embedded, dependency-free)

- [x] 4.1 `internal/ui/web/index.html` + `styles.css`: layout, context selector, home list, breadcrumb, node/health styling (light/dark ok)
- [x] 4.2 `app.js`: load contexts, select/switch context, render root Kustomizations with state + info
- [x] 4.3 Layer-by-layer drill-in: expand a node → fetch `/api/expand` → render immediate children with health; append to breadcrumb; navigate back via breadcrumb
- [x] 4.4 Error nodes rendered distinctly with reason; "open new exploration" opens a fresh page seeded from a resource; show the active context and flag a cross-context hop
- [x] 4.5 `//go:embed web/*` and serve via `http.FileServer` over the embedded FS

## 5. CLI command

- [x] 5.1 `cmd/fluxexp/command/ui.go`: `ui` command with `--address` (default `127.0.0.1:8765`) and `--context`; print the URL to open; start the server; block until interrupted
- [x] 5.2 Register `ui` in the root command

## 6. Verification

- [x] 6.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --strict`
- [x] 6.2 Manual smoke: `fluxexp ui`, open the URL, switch context, list roots under `flux`, drill Kustomization → HelmRelease → deployed objects, open a new exploration
- [x] 6.3 Update `README.md` (usage of `ui`, roadmap: UI initiated)
