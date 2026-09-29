## Context

`KustomizationResolver.Resolve` returns `Dependencies`, and
`DependencyRefsForFetched` (`internal/resolver/kustomization.go:88`) derives the
same references from an object a caller already holds. Both go through
`kustomizationDependencies`, which is what keeps a listed row and a traversed
node from disagreeing — an invariant pinned by
`TestDependencyRefsForFetched_AgreesWithResolver`.

`HelmReleaseResolver.Resolve` returns children and health only. The derivation
function hard-codes `kind != "Kustomization"` and returns nothing for everything
else, so a listed HelmRelease shows no group at all.

A HelmRelease declares its source in one of two places. `spec.chart.spec.sourceRef`
is the classic form — all 13 HelmReleases on the smoke-test cluster use it,
every one pointing at a HelmRepository. `spec.chartRef` is the v2 form pointing
straight at an OCIRepository or a HelmChart. `spec.dependsOn` names other
HelmReleases, with the namespace optional (the one cluster case omits it).

## Goals / Non-Goals

**Goals:**
- A HelmRelease is grouped with what it needs, exactly as a Kustomization is.
- A *listed* HelmRelease shows that group **without being resolved**.
- One derivation serves both resolution and listing, for HelmReleases as it does
  for Kustomizations.

**Non-Goals:**
- `spec.valuesFrom` references.
- The intermediate HelmChart Flux creates for a `chart`-style source.
- New inline fields on the HelmRelease (chart name, version).
- Any change to the engine, the UI or the API.

## Decisions

### The per-kind derivation moves to its own file, not onto the interface

`DependencyRefsForFetched` becomes a small dispatch in a new
`internal/resolver/dependencies.go`, keyed by group and kind, delegating to
`kustomizationDependencies` and a new `helmReleaseDependencies` that each stay
beside their own resolver. Kinds not in the table yield nothing, as today.

Rejected: adding a `DependenciesForFetched` method to the `Resolver` interface.
Every resolver — including the generic fallback, which must stay ignorant of any
particular ecosystem — would have to answer it, and callers would have to go
through the registry to reach it. The reference is already in the manifest the
caller holds; deriving it needs no interface.

Rejected: keeping the new branch inside `kustomization.go`. That file would own
a kind it knows nothing about, and the next resolver would make it worse.

### Source precedence: `chartRef` wins over `chart.spec.sourceRef`

The v2 API treats the two as mutually exclusive, so in practice only one is set.
When both appear the resolver takes `chartRef`, matching the API's own
precedence, rather than guessing or emitting two sources. When neither is set the
HelmRelease simply declares no source — an invalid object must produce an empty
group, never a panic.

### Source references reuse `sourceAPIVersion`

Every kind a HelmRelease can point at — HelmRepository, GitRepository, Bucket,
OCIRepository, HelmChart — is served under `source.toolkit.fluxcd.io/v1`
(verified against the smoke-test cluster's `api-resources`). The existing
constant covers all of them, so no kind-to-version table is introduced. On a
cluster serving an older version the reference becomes an error node with the
reason attached, which is the same outcome as a missing source and needs no
special case.

### `dependsOn` entries inherit the HelmRelease's own apiVersion

A `dependsOn` entry is necessarily another HelmRelease, served by the same
cluster at the same version as the object in hand — whose apiVersion is already
in the reference being resolved. Using it, rather than a hard-coded constant,
costs nothing and keeps the resolver working on a cluster still serving
`v2beta1`. (`kustomizationDependencies` uses a constant; it predates this
reasoning and is not worth churning here.)

### The chain continues through the HelmChart, it is not flattened

A `chartRef`-style HelmRelease points at a HelmChart, not at a repository — the
shape on `prod`, where `team-app` names HelmChart `cluster`, whose
own `spec.sourceRef` names HelmRepository `infra-repo`. Reporting the repository
directly under the release would mean fetching the HelmChart to read its
sourceRef, which is exactly the retrieval this derivation must not perform.

So the HelmChart joins the dispatch and declares its own source. Each hop is
derived from an object already in hand: opening the release shows the chart, and
the repository nested under it. It stays a leaf — a dependency is not a child,
and the kind remains non-expandable.

Where the grouping stops (a listed row nests one level only), the HelmChart's new
**Source** field carries the repository's kind and name. That is why the fields
come with this change rather than later: without them the row is a dead end, and
`fluxObjectFields` had no HelmChart case at all.

### Ordering and namespace defaulting mirror the Kustomization rule

Source first, then `dependsOn` in declaration order. An omitted namespace — on
the source ref or on a `dependsOn` entry — defaults to the HelmRelease's own. No
reference is pre-fetched to check it exists: a HelmRelease pointing at a missing
HelmRepository is precisely what a reader needs to see, as an error node.

## Risks / Trade-offs

- **The `node-dependencies` spec currently asserts a fetched HelmRelease yields
  nothing** → that scenario is replaced, not merely extended; the delta must
  carry the requirement's full updated content, or archiving loses detail.
- **A HelmRepository shared by many HelmReleases is fetched once per listed row**
  → already handled: the UI's per-request `memoGetter` collapses those into one
  call, which is why it exists.
- **`chartRef` is untested against a live cluster** — none of the 13 HelmReleases
  on the smoke-test cluster uses it. It is covered by table tests only, and the
  code path is a field read with no retrieval, so the exposure is a wrong field
  name, not wrong behaviour.
