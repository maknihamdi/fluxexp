## Context

I1 shipped a domain-agnostic engine and two resolvers (Kustomization, generic
Kubernetes fallback). A `HelmRelease` currently hits the fallback: health only,
no children. This increment adds the resolver that expands a HelmRelease into
the objects its chart deployed, so the traversal continues past it. The Helm
release-storage mechanism was verified on the live cluster (see Decisions).

## Goals / Non-Goals

**Goals:**
- Register a HelmRelease resolver that discovers children from the Helm release
  storage Secret and reports HelmRelease health.
- Reuse the existing contract, `ResolveContext`, `K8sRef`, and `K8sHealth` — no
  engine change.
- Unit-testable offline (decode + manifest parsing are pure functions).

**Non-Goals:**
- Helm **hooks** (`.hooks[]`) — deferred; only `.manifest` is expanded.
- The **ConfigMap** storage driver — only the default Secrets driver.
- Manifest-vs-live **diff**; we only enumerate what the release rendered, then
  the engine resolves each child's live health via its own resolver/fallback.
- Multiple/older revisions — only the newest release from `.status.history`.

## Decisions

### D1: Locate the release from `.status.history` (verified on cluster)
On the live cluster, a HelmRelease's `.status.history[0]` carries `name`,
`namespace`, and `version`; the storage Secret is
`sh.helm.release.v1.<name>.v<version>` (type `helm.sh/release.v1`) in that
namespace. We use the newest history entry. **Alternatives:** deriving the name
from `.spec.releaseName`/`.spec.targetNamespace` and guessing the latest version
— rejected: history is authoritative and already encodes the exact revision.
Fallback: no history → no children, no error (release not yet stored).

### D2: Decode chain — base64 → base64 → gzip → JSON (verified)
The Secret's `release` value read via the dynamic client is Kubernetes-base64 of
the Helm-stored string, which is itself base64 of gzip of the release JSON.
Verified end-to-end against `sh.helm.release.v1.trust-manager.v10`. Implement as
`b64 → b64 → gunzip → json.Unmarshal`. A decode failure is a per-node resolve
error (the engine renders an error node; siblings continue).

### D3: Parse `.manifest` as multi-document YAML
The release JSON's `manifest` is a single string containing all rendered objects
separated by `---`. Parse it with a streaming YAML decoder
(`yaml.NewDecoder` over the string, decoding into
`unstructured`-shaped maps) rather than string-splitting on `---`, so that
`---` inside values does not break parsing. For each document read
`apiVersion`, `kind`, `metadata.name`, `metadata.namespace`; skip empty/among
null documents. Build children with the existing `resolver.K8sRef`.

### D4: Namespace defaulting
A rendered object with no `metadata.namespace` inherits the **release namespace**
(`release.namespace`, i.e. the storage/target namespace). Cluster-scoped kinds
legitimately have no namespace; that is fine — the child ref simply carries an
empty namespace and the k8s client lists/gets it cluster-scoped. We do not try to
distinguish scope here; the RESTMapper in the client already handles it at fetch
time. (We only *default* to the release namespace when the manifest omitted one,
matching Helm's own behavior.)

### D5: Health reuse
Health comes from `resolver.K8sHealth` on the HelmRelease object (Ready
condition). Detail: prefer the release `.info.status` (e.g. "deployed") when
available, else the Ready message.

### D6: No new client capability needed
The resolver fetches the HelmRelease (already-encoded in the incoming ref) and
the storage Secret via `ResolveContext.GetK8s` using a `K8sRef` for the Secret
(`v1`, `Secret`, storageNamespace, secretName). No change to the k8s client API.

### D7: File placement
`internal/resolver/helmrelease.go` with the resolver + decode/parse helpers;
`internal/resolver/helmrelease_test.go` for table tests. Register it in the CLI
command wiring before the generic fallback (order: Kustomization, HelmRelease,
then fallback).

## Risks / Trade-offs

- **Encoding drift across Helm versions** → the base64/base64/gzip chain is
  stable across Helm 3; isolate it in one decode function with a focused test on
  a recorded payload. If a future driver changes, only that function changes.
- **Large manifests** (some charts render dozens of objects) → acceptable;
  children are enqueued and fetched by the engine like any other. Sequential
  fetch cost is an existing I1 trade-off.
- **Secret read permissions** → the tool already reads arbitrary objects
  read-only; reading `helm.sh/release.v1` Secrets is the same access. A denied
  read becomes a per-node resolve error, not a crash.
- **YAML edge cases** (multi-doc, empty docs, comments) → use a real streaming
  decoder, not string splitting; cover with tests.

## Open Questions

- Should Helm **hooks** be shown (they can leave orphaned Jobs)? Deferred to a
  follow-up; if added, render them under a distinct grouping so they are not
  confused with the steady-state manifest.
- Should the resolver annotate each child with the fact that it came *from* a
  Helm release (provenance) for the future UI? Not needed for the text tree;
  revisit when the UI lands.
