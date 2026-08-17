## 1. Engine model carriers

- [x] 1.1 Add `Freshness` enum (`UpToDate`, `Behind`, `Failed`, `Suspended`, "") and `Field{Label,Value}` in `internal/engine`
- [x] 1.2 Add `Freshness` and `Fields []Field` to `Result` and `Node`; copy them in `resolveNode`
- [x] 1.3 Unit test: engine carries freshness + fields (in order) from a stub resolver

## 2. Freshness computation (resolver)

- [x] 2.1 `shortRevision(rev)` (handle `@sha1:`, `@`, `/`) and `humanizeSince(ts)` helpers
- [x] 2.2 `KustomizationFreshness(ks, source *unstructured) (engine.Freshness, []engine.Field)`: suspend → Ready False (failed) → applied==source (up-to-date) → behind; fields Applied/Synced/Source (+ Attempted/Error on failure); graceful fallback when source is nil
- [x] 2.3 Unit tests: up-to-date, behind (both revisions in a field), failed (attempted+message), suspended, nil-source fallback, short-revision form

## 3. Kustomization resolver wiring

- [x] 3.1 In `KustomizationResolver.Resolve`: fetch the source (`spec.sourceRef` → `source.toolkit.fluxcd.io/v1`), compute freshness+fields, set them on the Result (keep existing health + inventory children)

## 4. CLI rendering

- [x] 4.1 `render.Tree`: render `[freshness]` after `[health]` when set; append key fields to the detail; leave non-freshness nodes unchanged
- [x] 4.2 Unit test: Kustomization line shows freshness + revisions; a Deployment line unchanged

## 5. UI

- [x] 5.1 `RootDTO`/`NodeDTO` gain `freshness` + `fields`; `refToDTO`/expand carry them
- [x] 5.2 `Roots`: fetch sources (cache by name) and compute freshness via the shared helper; `rootSummary` becomes source-aware
- [x] 5.3 `Expand`: carry the resolved node's freshness + fields into `NodeDTO`
- [x] 5.4 Frontend: freshness badge (distinct from health) on root cards + node header; a fields panel; short sha with full revision in `title`
- [x] 5.5 Handler/service tests: roots include freshness; expand carries freshness + fields

## 6. Verification

- [x] 6.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --strict`
- [ ] 6.2 Manual smoke (BLOCKED: cluster gcloud creds expired — needs `gcloud auth login`): CLI `traverse` shows freshness; UI roots home shows freshness badges; node view shows the fields panel
- [x] 6.3 Update `README.md`
