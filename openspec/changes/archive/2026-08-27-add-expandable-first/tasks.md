## 1. Resolver contract

- [x] 1.1 Add `Expandable(ref engine.Ref) bool` to the `Resolver` interface in `internal/resolver/resolver.go`, documenting that it answers from the reference alone — no fetch, no context, no error
- [x] 1.2 `KustomizationResolver.Expandable` → true for any reference it matches
- [x] 1.3 `HelmReleaseResolver.Expandable` → true for any reference it matches
- [x] 1.4 `WorkloadResolver.Expandable` → true for its kinds, false for `Pod` (reuse `DecodeK8sRef` + the existing `workloadKinds` map rather than a second kind list)
- [x] 1.5 `GenericK8sResolver.Expandable` → false
- [x] 1.6 Update the in-tree test double (`stubResolver` in `registry_test.go` — the only one; `workload_test.go` uses the real resolvers) so the package compiles

## 2. Registry lookup and ordering

- [x] 2.1 `Registry.Expandable(ref) bool`: delegate to `For(ref)`, returning false when no resolver and no fallback match
- [x] 2.2 `Registry.OrderChildren(refs []engine.Ref) []engine.Ref`: stable partition, expandable first, each group keeping its relative order (single allocation, no comparison sort)
- [x] 2.3 Apply `OrderChildren` to `Result.Children` inside `Registry.ResolveFunc`, so the engine and the CLI inherit the order
- [x] 2.4 Tests: per-resolver declarations (Kustomization, HelmRelease, workload kinds, Pod, ConfigMap fallback); registry delegation; unresolvable domain → false
- [x] 2.5 Tests: ordering places containers first, preserves relative order within each group, leaves uniform lists untouched

## 3. CLI rendering

- [x] 3.1 Carry expandability onto the rendered node: add `Expandable bool` to `engine.Node` and set it in `Registry.ResolveFunc`, keeping the engine itself free of any expandability logic
- [x] 3.2 `render.Tree`: reserve a two-character slot between the health glyph and the label — `▸ ` when expandable, two spaces otherwise
- [x] 3.3 Tests: chevron present on an expandable node, padding on a leaf, labels stay aligned; children render containers-first

## 4. Web UI

- [x] 4.1 Add `expandable` to `NodeDTO` in `internal/ui/dto.go`
- [x] 4.2 `Service.Expand`: order children through `registry.OrderChildren`, set the flag on the expanded node and on each child (`childHealth`)
- [x] 4.3 `app.js`: render the chevron before the name and the accent on the card for an expandable child, and leave a leaf unmarked
- [x] 4.4 `styles.css`: accent styling distinct from the outlined health badges and the filled freshness badges
- [x] 4.5 Service/handler tests: response carries the flag on node and children; a Pod child is false while a HelmRelease child is true; children come back containers-first

## 5. Verification

- [x] 5.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --all`
- [x] 5.2 Manual smoke against `dev` (2026-08-26): `traverse flux/traefik` lists the HelmRelease first with a chevron (it was 5th before) and the Deployment ahead of the chart's leaves; `/api/expand` returns `expandable` on node and children, containers-first, Pod false; a scaled-to-0 Deployment confirms the hint semantics — expandable with zero children and no error
- [x] 5.3 Update `README.md` if the tree sample output changes

## 6. Flux-first ordering

- [x] 6.1 `IsFluxRef(ref) bool` in `internal/resolver`: true when the reference's API group ends in `toolkit.fluxcd.io` (guard against lookalike groups)
- [x] 6.2 Rework `Registry.OrderChildren` into a three-tier stable partition — Flux, then other expandable, then the rest
- [x] 6.3 Tests: a Flux leaf outranks a non-Flux container; every Flux group qualifies; a lookalike group does not; relative order preserved per tier

## 7. Kustomization source as a child

- [x] 7.1 `KustomizationResolver.Resolve`: emit the `spec.sourceRef` object as the first child, reusing the object already fetched for freshness (no second GET)
- [x] 7.2 Omit the child when no source is configured or the fetch failed, leaving the Kustomization's own health untouched
- [x] 7.3 Tests: source leads the children; namespace defaults to the Kustomization's; unfetchable source is omitted not errored; sourceless Kustomization unaffected

## 8. Flux source and image fields

- [x] 8.1 `FluxObjectResolver` in `internal/resolver/flux_object.go`: claims `source.toolkit.fluxcd.io` and `image.toolkit.fluxcd.io`, health from Ready, non-expandable, no children
- [x] 8.2 Source fields: GitRepository (url, tracked ref among branch/tag/semver/commit, interval), OCIRepository (url, ref, interval), Bucket (bucket, endpoint, interval), HelmRepository (url, type, interval)
- [x] 8.3 Image fields: ImageRepository (image, last scan), ImagePolicy (policy rule, selected tag)
- [x] 8.4 Every field optional — an absent path emits no field, never an empty value or an error
- [x] 8.5 Register ahead of the generic fallback in `NewDefaultRegistry`
- [x] 8.6 Tests: matcher scope; per-kind fields; tag-tracking variant; missing paths degrade silently; health still from Ready

## 9. Verification (round 2)

- [x] 9.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --all`
- [x] 9.2 Manual smoke against `dev` (2026-08-26): `traverse flux/traefik` leads with `GitRepository flux/flux — repo gitlab.example.com/infra/fleet · branch main · interval 1m`, then HelmRelease and HelmRepository (Flux tier), then the leaves; `/api/expand` returns the same order and the GitRepository node carries Repo/Branch/Interval with the full URL for the tooltip. ImageRepository/ImagePolicy fields are unit-tested only — no such objects exist on the reachable clusters
- [x] 9.3 Update `README.md` for the Flux-first order and the source child

## 10. Dependency edge class

- [x] 10.1 `engine.Result.Dependencies []Ref` and `engine.Node.Dependencies []*Node`, documented as shown-but-not-descended
- [x] 10.2 `engine.Traverse`: resolve each dependency one level deep, attach as a node, never enqueue its children, never record it in `visited`
- [x] 10.3 A dependency that fails to resolve becomes an error node without affecting its parent
- [x] 10.4 Engine tests: subtree not traversed; chain does not follow; unresolvable dependency is an error node; a dependency does not suppress a later child expansion

## 11. Kustomization dependencies

- [x] 11.1 Move the source out of `Children` into `Dependencies`; keep the inventory as children
- [x] 11.2 Append `spec.dependsOn` entries as dependencies, in declaration order, namespace defaulting to the Kustomization's own; emit without pre-fetching
- [x] 11.3 Tests: source leads the dependencies; inventory stays in children; namespace defaults; unfetchable source omitted; missing dependsOn target becomes an error node; sourceless Kustomization unaffected

## 12. Fields without a second fetch

- [x] 12.1 Export `FieldsForFetched(ref, obj) []engine.Field` in `internal/resolver` — fields computable from an already-retrieved object, empty for kinds that have none
- [x] 12.2 Have `FluxObjectResolver.Resolve` use it, so there is one extraction path
- [x] 12.3 Tests: fields from an object in hand; a ConfigMap yields nothing

## 13. CLI rendering of dependencies

- [x] 13.1 Render a node's dependencies before its children, with a dependency marker in the existing two-character slot (distinct from the expandable chevron)
- [x] 13.2 Tests: dependencies print first with their marker; their fields show inline; a node without dependencies is unchanged

## 14. UI card

- [x] 14.1 `NodeDTO.Dependencies []NodeDTO`; `Service.Expand` resolves dependencies one level (health + fields) and keeps them out of `Children`
- [x] 14.2 `childHealth` uses `FieldsForFetched` so Flux fields show inline on child rows
- [x] 14.3 `app.js`: nest dependencies inside the node's card, each selectable to drill in; children section becomes what the node applies
- [x] 14.4 `app.js`/`styles.css`: render inline fields on list rows; no empty dependency block when there are none
- [x] 14.5 Tests: dependencies returned separately from children; source absent from children; a Flux child row carries fields; a ConfigMap row carries none

## 15. Verification (round 3)

- [x] 15.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --all`
- [x] 15.2 Manual smoke against `dev` (2026-08-26): `traverse flux/cert-issuer` prints the GitRepository (repo/branch/interval inline) plus the three `dependsOn` Kustomizations with `⇢`, none descended into — `cert-manager` shows its own `dependency 'flux/alloy' is not ready`, which explains the parent's failure at a glance; `/api/expand` returns `dependencies` separately from `children`, a HelmRepository child row carries repo/interval inline, and a Deployment returns no dependency block
- [x] 15.3 Update `README.md` for the dependency grouping

## 16. Dependencies on listed nodes

- [x] 16.1 `DeclaresDependencies(ref) bool` on the `Resolver` interface — true for Kustomization, false elsewhere; update the test double
- [x] 16.2 `Registry.DeclaresDependencies(ref)` delegating through the usual selection, false when nothing matches
- [x] 16.3 `Service.childHealth`: when a listed ref declares dependencies, resolve it through its resolver to obtain health, freshness, fields and dependencies; keep the cheap fetch path for everything else
- [x] 16.4 Resolve those nested dependencies with the cheap path (health + fields, no further descent) so a layer stays bounded
- [x] 16.5 Apply the same treatment to the dependencies shown inside the card, so a `dependsOn` Kustomization also shows its source
- [x] 16.6 `app.js`/`styles.css`: render a nested dependency group under a list row, indented under it
- [x] 16.7 Tests: a child Kustomization carries its source and dependsOn; a ConfigMap/HelmRelease child is not resolved and carries none; nesting stops at one level
- [x] 16.8 CLI regression test: a child node's dependencies render nested under it (already produced by the engine)

## 17. Verification (round 4)

- [x] 17.1 `go build ./...`, `go vet ./...`, `go test ./...` green; `openspec validate --all`
- [x] 17.2 Manual smoke (2026-08-26, `dev`): `/api/expand` on `flux/flux` shows each of the 23 child Kustomizations with its own GitRepository (repo/branch/interval inline) and its `dependsOn` entries with their health — e.g. `cert-issuer` lists `cert-manager` as unhealthy. CLI verified too: `alloy` renders its own dependency block nested under it

## 18. Bounded cost for a layer

- [x] 18.1 Per-request memoizing getter in `internal/ui`: cache Get results (hits and misses) for the duration of one Expand, so a source shared by N listed Kustomizations is fetched once
- [x] 18.2 Tests: the same object requested twice is fetched once; a failure is cached too; the cache does not leak across requests
- [x] 18.3 Measured on `flux/flux` (23 child Kustomizations, GKE europe-west9): 25.5s before → 12.9s with the memo → **1.2s** once the client-go rate limiter was raised. The limiter, not the scheduling, was the dominant cost; the memo still removes ~46 redundant GETs of the shared source. Full CLI `traverse flux/flux` (643 lines) went from exceeding 120s to 79s
- [x] 18.4 Resolve a layer's entries concurrently with a bounded worker pool, preserving order; re-measure
- [x] 18.5 Raise the client-go rate limiter (default QPS 5 / Burst 10 throttles a whole layer); re-measure

## 19. Derive dependencies from the fetched object

- [x] 19.1 `DependencyRefsForFetched(ref, obj) []engine.Ref` in `internal/resolver` — dependency refs straight from a fetched object's manifest, empty for kinds that declare none
- [x] 19.2 `KustomizationResolver.Resolve` uses the same helper, so resolution and listing cannot diverge; the source is emitted unconditionally (an unfetchable one becomes an error node)
- [x] 19.3 Remove `DeclaresDependencies` from the `Resolver` interface, its four implementations, the registry and the test double — the fetched-object helper makes it unnecessary
- [x] 19.4 `Service.listedNode`: one fetch per entry, then dependency refs from that object, each retrieved once for health and fields; never resolve the entry
- [x] 19.5 Tests: refs derived without retrieval; resolver and helper agree; a listed Kustomization's inventory is never read; freshness no longer claimed on listed entries
- [x] 19.6 Re-measured 2026-08-27 on `flux/flux` (67 children, 23 of them Kustomizations, GKE europe-west9), each run checked for a valid payload first: 1.25 / 1.17 / 1.18 / 1.66 / 1.11 s. An earlier set of readings taken after the credentials expired was discarded — they were fast only because the requests failed on auth

## 20. No duplicate between dependencies and children

- [x] 20.1 `Registry.ResolveFunc`: drop from the ordered children any reference already declared as a dependency
- [x] 20.2 `Service.Expand` goes through `Registry.ResolveFunc` instead of re-applying ordering itself, so both surfaces share one rule
- [x] 20.3 Tests: an object that is both source and inventory entry appears once, as a dependency; unrelated children untouched
- [x] 20.4 Verified 2026-08-27 on `dev`: `flux/flux` reconciles from `GitRepository flux/flux` *and* applies it (Flux bootstrap). It now appears once — 1 in the card's dependencies, 0 among the children, which drop from 67 to 66. The CLI tree shows it once too, since both surfaces now go through the same registry entry point

## 21. One occurrence per layer

- [x] 21.1 `Service.Expand`: after resolving a layer, drop a top-level child already shown as another entry's nested dependency (card dependencies included in the scan)
- [x] 21.2 Never drop an entry that carries its own dependency group, so mutual references cannot erase both
- [x] 21.3 Tests: the reported shape (Kustomization + its GitRepository both applied) renders once; an entry with its own group survives; a layer with no overlap is untouched
- [x] 21.4 Verified 2026-08-27 on `prod` (read-only): `flux/project-dev-enr` applies both `Kustomization dev-enr/energies-deployment` and the `GitRepository` it reconciles from. The GitRepository now shows 0 top-level rows and 1 nested occurrence under its Kustomization; children drop from 68 to 67

