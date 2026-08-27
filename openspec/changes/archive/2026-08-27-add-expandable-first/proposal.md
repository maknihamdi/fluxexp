## Why

When a node is expanded, its children arrive in inventory order and all look
alike: a `HelmRelease` that opens onto a whole chart is rendered exactly like a
`ConfigMap` that opens onto nothing. The user cannot tell where the graph
continues without clicking things at random — and most clicks are dead ends,
because leaves dominate a typical inventory. Surfacing which children carry a
subtree, and putting them first, turns exploration from guesswork into reading.

## What Changes

- The resolver contract gains a declarative **expandability predicate**: a
  resolver states whether a reference it matches can descend, without fetching
  anything. **BREAKING** for the `Resolver` interface — every implementation
  (including the two test doubles) must supply it.
- Each resolver declares its own answer: `Kustomization` and `HelmRelease`
  always descend; workloads descend except `Pod`; the generic Kubernetes
  fallback never does.
- Child references are ordered in **three stable tiers**: Flux objects (any
  `*.toolkit.fluxcd.io` group) first, then other expandable children, then the
  rest. Each tier keeps its relative order. The rule is applied once, in the
  registry layer, so the CLI and the UI cannot drift apart.
- The graph gains a **second class of edge**: *dependencies*, distinct from
  children. A dependency is **shown with its health and fields but not descended
  into**, which is what makes it safe to include Kustomization-to-Kustomization
  links without the tree exploding.
- A Kustomization declares as dependencies its `spec.sourceRef` object
  (GitRepository, OCIRepository, Bucket, HelmRepository) and the Kustomizations
  in `spec.dependsOn`. The source was already fetched to compute freshness but
  was invisible in the graph; `dependsOn` explains the `dependency X is not
  ready` messages that `list` surfaces today.
- The UI groups a Kustomization and its dependencies into **one card**: the
  dependencies are nested inside the Kustomization's own card with their fields
  visible, and no longer appear in the applied-objects list.
- Flux object fields are **visible without a click**: any Flux source or image
  object rendered in a list shows its fields inline, derived from the object
  already fetched for its health — no additional API call.
- Flux **source and image-automation objects surface their useful fields**:
  repository URL, branch/tag, and interval for a GitRepository; the equivalent
  per kind for OCIRepository, Bucket, HelmRepository, ImageRepository and
  ImagePolicy. Today they fall to the generic fallback and show nothing but a
  Ready condition.
- The CLI tree marks an expandable node with a leading chevron `▸`.
- The UI marks an expandable child with the same chevron plus an accent on the
  card, and exposes the flag in the JSON API so the frontend does not have to
  re-derive it.
- The traversal engine is **not** touched: expandability is domain knowledge and
  stays on the resolver side of the boundary.

## Capabilities

### New Capabilities
- `node-dependencies`: the dependency edge class — how a resolver declares it,
  how the engine resolves it one level deep without descending, and how it stays
  separate from children.
- `expandable-nodes`: the expandability predicate, each resolver's declaration,
  the three-tier ordering rule (Flux objects, then containers, then leaves), and
  the fact that expandability is a static hint rather than a guarantee of
  non-empty children.
- `flux-object-details`: a resolver for Flux source and image-automation kinds
  that surfaces each kind's useful fields (repository, ref, interval, scan
  results, selected tag) instead of leaving them to the generic fallback.

### Modified Capabilities
- `resource-traversal`: the Resolver contract requirement gains a third
  obligation — declaring expandability alongside matching and resolving.
- `cli`: the tree marks expandable nodes and orders children Flux-first, then
  containers, then leaves.
- `web-ui`: child lists follow the three-tier order, expandable children carry
  the chevron and accent, the node API exposes the flag, a Kustomization and its
  dependencies render as one card, and Flux fields show inline in lists.
- `flux-kustomization-resolver`: the Kustomization declares its `spec.sourceRef`
  object and its `spec.dependsOn` Kustomizations as dependencies, separate from
  its inventory children.
- `resource-traversal`: the resolver contract and the engine gain the dependency
  edge class, resolved one level deep and never descended into.

## Impact

- `internal/resolver`: the `Resolver` interface, all concrete resolvers, a new
  Flux source/image resolver, the `Registry` (expandability lookup plus the
  shared three-tier ordering helper), and the test double in `registry_test.go`.
- `internal/render`: the tree line gains the chevron.
- `internal/ui`: `NodeDTO` gains an `expandable` field; `Expand` orders children
  through the shared helper; `app.js` and `styles.css` render the marker.
- `internal/engine`: unchanged.
- No new dependencies. No change to cluster access, which stays read-only.
