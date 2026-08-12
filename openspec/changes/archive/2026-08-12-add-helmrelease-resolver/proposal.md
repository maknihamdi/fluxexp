## Why

In I1, a Flux `HelmRelease` is a leaf: `fluxexp` shows its health but stops
there, so the objects the chart actually deployed (Deployments, Services, CRDs,
…) are invisible. That is the next hop in the chain the tool exists to follow.
This increment teaches `fluxexp` to expand a HelmRelease into the resources it
rendered, so traversal continues past it — proven on the cluster where
`Kustomization → HelmRelease` chains already appear (e.g. `flux/cert-manager`,
`calypso-deployment`).

## What Changes

- New **HelmRelease resolver** for `helm.toolkit.fluxcd.io/v2`, kind
  `HelmRelease`, registered ahead of the generic Kubernetes fallback.
- Children discovery via the **Helm release storage**: locate the release from
  `.status.history` (newest entry → release `name`, storage `namespace`,
  `version`), fetch the Secret `sh.helm.release.v1.<name>.v<version>` (type
  `helm.sh/release.v1`), decode it (Kubernetes base64 → Helm base64 → gzip →
  JSON), parse the release `.manifest` (multi-document YAML), and emit one
  `kubernetes` child reference per rendered object. Objects with no explicit
  namespace inherit the release namespace.
- Health from the HelmRelease `.status.conditions[Ready]` (reuses the existing
  model), with the release `.info.status` surfaced as detail.
- New dependency: a YAML decoder to split/parse the rendered manifest.

Out of scope for this increment: Helm **hook** resources (`.hooks[]`), the
ConfigMap storage driver (only the default Secrets driver is supported), and any
diff between rendered manifest and live cluster state.

## Capabilities

### New Capabilities

- `helmrelease-resolver`: expanding a Flux HelmRelease into the resources it
  deployed, discovered from the Helm release storage, and reporting the
  HelmRelease's health.

### Modified Capabilities

<!-- None. The engine, registry, generic fallback and CLI are unchanged; the new
     resolver plugs into the existing resolver contract. -->

## Impact

- New file `internal/resolver/helmrelease.go` (+ tests); registered in
  `cmd/fluxexp/command` alongside the Kustomization resolver.
- New dependency: `sigs.k8s.io/yaml` (or `gopkg.in/yaml.v3`) for manifest
  parsing. Standard library `encoding/base64` + `compress/gzip` for decoding.
- No change to `internal/engine`; the resolver satisfies the existing
  `resource-traversal` contract. Reuses `resolver.K8sHealth` and `K8sRef`.
- `list --kind HelmRelease` already works (I1); this only affects `traverse`.
