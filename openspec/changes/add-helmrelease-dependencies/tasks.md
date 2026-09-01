## 1. Make the derivation multi-kind

- [x] 1.1 Create `internal/resolver/dependencies.go` and move `DependencyRefsForFetched` there, turning its Kustomization-only check into a dispatch on group + kind that yields nothing for unknown kinds
- [x] 1.2 Keep `kustomizationDependencies` in `kustomization.go` and confirm the Kustomization behaviour is unchanged (existing tests must pass untouched)

## 2. HelmRelease dependencies

- [x] 2.1 Add `helmReleaseSourceRef` in `helmrelease.go`: `spec.chartRef` when present, otherwise `spec.chart.spec.sourceRef`; build the reference with `sourceAPIVersion`; default an omitted namespace to the HelmRelease's own; report absence rather than failing when neither is declared
- [x] 2.2 Add `helmReleaseDependencies`: the source first, then `spec.dependsOn` entries in declaration order, each taken as a HelmRelease at the apiVersion of the reference being resolved, with the same namespace defaulting
- [x] 2.3 Register the HelmRelease in the dispatch from 1.1
- [x] 2.4 Return `Dependencies` from `HelmReleaseResolver.Resolve` through that same function, including on the early-return path where the release is not stored yet

## 3. Tests

- [x] 3.1 Table tests in `helmrelease_test.go` for the seven spec scenarios: chart source grouped, dependsOn after the source, chartRef precedence, namespace defaulting, explicit namespace, missing source still reported, no declarations
- [x] 3.2 A HelmRelease counterpart to `TestDependencyRefsForFetched_AgreesWithResolver`, pinning that resolution and listing produce the same references in the same order
- [x] 3.3 Assert the listing path reads no Helm storage Secret — the derivation must not touch the getter
- [x] 3.4 Update the existing assertion that a fetched HelmRelease yields no dependency references

## 4. Verification

- [x] 4.1 `make vet` and `make test` green
- [x] 4.2 CLI: `traverse` a Kustomization that applies a HelmRelease on `dev` and confirm the HelmRepository is shown with the release and not among its children
- [x] 4.3 UI: open a layer listing HelmReleases and confirm each shows its source group without the layer getting slower (the storage Secrets must not be read for a listing)

## 5. Documentation

- [x] 5.1 Note in `CLAUDE.md` that `DependencyRefsForFetched` is multi-kind and where a new kind registers
- [x] 5.2 Add the increment to the README roadmap

## 6. The second hop: HelmChart (found on a real chartRef release)

- [x] 6.1 Add `helmChartDependencies` in `flux_object.go` — `spec.sourceRef` with namespace defaulting — and register the HelmChart in the dispatch
- [x] 6.2 Return `Dependencies` from `FluxObjectResolver.Resolve` so resolution and listing agree for this kind too
- [x] 6.3 Add the missing `HelmChart` case to `fluxObjectFields`: chart, version, source (kind/name), interval
- [x] 6.4 Tests: the chart's repository is declared, namespace defaulting, no source declares nothing, fields, and the resolver/listing agreement
- [x] 6.5 Verify on `prod`: opening `team-app` shows the HelmChart with the HelmRepository nested under it
