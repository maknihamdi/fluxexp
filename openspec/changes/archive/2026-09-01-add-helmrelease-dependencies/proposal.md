## Why

A Kustomization is shown with what it needs — its source and its `dependsOn`
targets, grouped in its own card — while a HelmRelease is shown alone. The
HelmRepository the chart is pulled from appears nowhere, although it is the first
thing to look at when a release will not install. The dependency edge class
already exists, with all its rules; the HelmRelease simply never declared any.

## What Changes

- A HelmRelease declares dependencies, in the order a Kustomization does: its
  **source** first, then its `spec.dependsOn` entries in declaration order.
- The source is the one **declared in the manifest**: `spec.chartRef` when
  present (it takes precedence in the v2 API), otherwise
  `spec.chart.spec.sourceRef`. Any source kind is accepted — HelmRepository,
  GitRepository, Bucket, OCIRepository, HelmChart. An omitted namespace defaults
  to the HelmRelease's own.
- `spec.dependsOn` entries are other HelmReleases, with the same namespace
  defaulting.
- No reference is pre-checked: a missing source becomes an error node downstream,
  which is exactly the signal a reader needs.
- The derivation from an **already-fetched** object covers the HelmRelease too,
  so a *listed* HelmRelease shows its dependency group without being resolved.
  This is the point of the change as much as the grouping itself: resolving a
  HelmRelease gunzips its Helm storage Secret, the probing cost this design
  exists to avoid.
- A **HelmChart** declares its own `spec.sourceRef`, so a HelmRelease using
  `spec.chartRef` reaches its repository one hop further: release → chart →
  repository. Without it the chain stops at the chart, which is what a
  `chartRef`-style release looks like on a real cluster.
- A HelmChart also gains inline fields (chart, version, source, interval) — it
  had none, so wherever the grouping stops the row said nothing at all.
- **BREAKING** (spec only): the existing scenario stating that a fetched
  HelmRelease yields no dependency references no longer holds.

Nothing else needs code: a declared dependency is already dropped from the node's
children, dependencies are already never descended into, the portal already
groups them into the card, and a HelmRepository already carries its health and
its inline fields through the Flux object resolver.

Deliberately excluded from this increment:
- `spec.valuesFrom` (ConfigMap/Secret). A ConfigMap has no Ready condition, so
  those rows would read `unknown`, and `optional: true` makes a missing one
  legitimate. Worth its own decision later.
- The intermediate **HelmChart** Flux creates (`<ns>-<name>` in the source's
  namespace). We show what is declared, not what Flux manufactures; its health
  largely restates the release's.
- New inline fields on the HelmRelease itself (chart, version).

## Capabilities

### New Capabilities
<!-- none: this teaches an existing resolver an edge class the graph already has -->

### Modified Capabilities
- `helmrelease-resolver`: gains a requirement stating which references a
  HelmRelease declares as dependencies, and in what order.
- `node-dependencies`: the requirement covering derivation from a fetched object
  must stop being Kustomization-only — including its scenario that currently
  names the HelmRelease as a kind yielding nothing.
- `flux-object-details`: a HelmChart declares the repository it pulls from, and
  gains the fields it never had. It stays a leaf — a dependency is not a child.

## Impact

- `internal/resolver/helmrelease.go` — dependency references, returned by
  `Resolve` and derived from a fetched object.
- `internal/resolver/kustomization.go` — `DependencyRefsForFetched` becomes
  multi-kind; the single-source rule it guarantees must survive the split.
- `internal/resolver/flux_object.go` — the HelmChart's source dependency and its
  fields; the resolver now carries dependencies too.
- Tests in `internal/resolver`, table-driven against the shared `fakeGetter`.
- No engine, UI or API change.
