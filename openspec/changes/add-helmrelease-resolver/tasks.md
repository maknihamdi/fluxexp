## 1. Dependencies

- [x] 1.1 No new dependency: reuse `k8s.io/apimachinery/pkg/util/yaml` (`NewYAMLOrJSONDecoder`) for multi-doc manifest parsing

## 2. Release decoding (pure helpers)

- [x] 2.1 Implement `decodeHelmRelease([]byte) (release, error)`: base64 → base64 → gzip → JSON into a minimal struct (`name`, `namespace`, `version`, `manifest`, `info.status`)
- [x] 2.2 Unit-test the decode chain on a small recorded payload fixture (satisfies `helmrelease-resolver` decode scenarios, incl. corrupt payload → error)

## 3. Manifest parsing (pure helper)

- [x] 3.1 Implement `manifestChildren(manifest, releaseNamespace) ([]engine.Ref, error)`: streaming multi-doc YAML decode; per doc read apiVersion/kind/metadata.{name,namespace}; skip empty docs; default namespace to the release namespace; build `K8sRef`s
- [x] 3.2 Unit tests: two objects → two children, namespaceless object inherits release namespace, empty docs skipped (satisfies `helmrelease-resolver` manifest scenarios)

## 4. HelmRelease resolver

- [x] 4.1 Implement matcher: claim `kubernetes` refs whose type is `helm.toolkit.fluxcd.io/*, Kind=HelmRelease` (satisfies matcher scenario)
- [x] 4.2 Resolve: fetch the HelmRelease via `ResolveContext`; read newest `.status.history` entry → name/namespace/version; no history → no children, no error
- [x] 4.3 Fetch the storage Secret `sh.helm.release.v1.<name>.v<version>` via `GetK8s` (K8sRef for `v1/Secret`); read `.data.release`; decode; parse manifest → children
- [x] 4.4 Health via `resolver.K8sHealth`; detail from release `.info.status` when present
- [x] 4.5 Unit tests wiring it together with a fake getter (HelmRelease + its storage Secret): children discovered, health reported (satisfies remaining scenarios)

## 5. Registration

- [x] 5.1 Register `HelmReleaseResolver` in the CLI command wiring, before the generic fallback

## 6. Verification

- [x] 6.1 `go build ./...`, `go vet ./...`, `go test ./...` green
- [x] 6.2 Manual smoke: `fluxexp traverse -n flux --name cert-manager` now expands each HelmRelease into its deployed objects with health
- [x] 6.3 Update `README.md` roadmap (mark I2 done) and `openspec validate --strict`
