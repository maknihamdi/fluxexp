## Why

Two thirds of what a Kustomization applies has no status in fluxexp. Measured on
`dev`: of the 255 inventory entries across every Kustomization,
**172 (67%) render as `unknown`**, because health is derived from a single
`.status.conditions` entry of type `Ready` and most Kubernetes objects do not have
one. A tool whose purpose is to follow the chain to the end and report health along
the way currently goes silent on the majority of what it finds.

The 172 are not one problem but three, all collapsed into the same word:

| Family | Entries | Examples | Why `unknown` today |
|---|---|---|---|
| **A** — no `.status` at all | 101 (40%) | ClusterRole, ServiceAccount, Secret, ConfigMap, Role, RoleBinding, NetworkPolicy, webhooks, Middleware, IngressRoute | There is nothing to read — and nothing that *can* be unhealthy |
| **B** — conditions, but no `Ready` | 39 (15%) | CustomResourceDefinition (`Established`), Bundle (`Synced`), KubernetesRole / Policy (`Configured`), PodDisruptionBudget, HorizontalPodAutoscaler, ComputeClass (`Health`) | The object publishes a clear verdict that is simply not read |
| **D** — status, but no conditions | 32 (13%) | Namespace (`phase`), Service (`loadBalancer`), NifiCluster (`state`), ResourceQuota, DNSEndpoint | No condition to look at |

(Family **C**, the 83 entries that do carry `Ready`, is already correct and must stay
correct.)

`unknown` therefore means three different things at once — *nothing to know*, *not
reconciled yet*, and *I cannot read this* — which is why no single rule fixes it.

## What Changes

- Kubernetes health derivation moves from a hand-rolled `Ready`-only reading to
  **`sigs.k8s.io/cli-utils/pkg/kstatus`**, the library Flux itself uses for its
  health checks. Verified against 20 real object kinds from the cluster: 20/20 return
  a verdict, zero `unknown`.
- A new health value **`pending`** joins the fixed set, carrying what kstatus reports
  as `InProgress` — chiefly **generation drift** (`metadata.generation` ahead of
  `status.observedGeneration`), which today reads as `unknown` or, worse, as a
  premature `unhealthy`.
- Objects with no status at all (family A) report **healthy**: existing is the whole
  of what a ClusterRole can do.
- A **condition-polarity layer** sits above kstatus. kstatus only knows `Reconciling`,
  `Stalled` and `Ready`; everything else falls through to its generic "nothing
  recognised, generation current → `Current`" rule. Verified: a `Bundle` with
  `Synced=False` and a `KubernetesRole` with `Configured=False` both come back
  `Current`. Adopting kstatus unqualified would trade 39 noisy `unknown`s for 39
  **false greens**, which is strictly worse — an `unknown` invites a look, a green
  says move on. fluxexp therefore reads unrecognised conditions by polarity
  (`Synced` / `Configured` / `Established` / `Available` / `Healthy` / `Succeeded`
  false → unhealthy; `Degraded` / `Stalled` / `Failed` true → unhealthy) before
  accepting a generic `Current`.
- `Ready=False` keeps mapping to **unhealthy**, overriding kstatus, which classes it
  `InProgress`. Without the override a broken Certificate would go from red to amber —
  a regression across the 83 entries that are right today.
- `unknown` survives as a genuine last resort rather than a default.

No breaking change to the CLI or HTTP surface: `pending` is an additional value in an
existing enumeration, and `list --unhealthy` already prints everything that is not
healthy.

## Capabilities

### New Capabilities
- `object-status`: universal status derivation for Kubernetes objects — the kstatus
  base, the condition-polarity layer above it, the `Ready=False` override, the
  treatment of status-less objects, generation drift as `pending`, and the narrowed
  meaning of `unknown`. It is the single shared derivation every surface uses.

### Modified Capabilities
- `resource-traversal`: **Per-node health model** gains `pending` in the fixed set;
  **Generic Kubernetes fallback resolver** stops being specified as `Ready`-only and
  defers to the shared derivation.
- `workload-health-resolver`: **Replica-based health** checks generation drift first,
  so a Deployment whose spec the controller has not yet observed reads `pending`
  rather than `unhealthy` on a replica count that describes the previous spec.
- `cli`: **Tree rendering with health** must render `pending` distinctly from healthy,
  unhealthy and error.
- `web-ui`: a `pending` node must be visually distinct from the other health values.

## Impact

- **Dependency**: one new module, `sigs.k8s.io/cli-utils`. Verified that
  `k8s.io/api`, `k8s.io/utils` and `k8s.io/kube-openapi` are already in the build
  graph via client-go, so the addition is narrow against a `go.mod` that today holds
  only cobra, apimachinery and client-go.
- **Code**: the body of `K8sHealth` in `internal/resolver/resolver.go`. All six
  derivation sites already funnel through `K8sHealth` / `healthFromReady`
  (`internal/ui/service.go`, `flux_object.go`, `helmrelease.go`, `kustomization.go`,
  `generic_k8s.go`, `cmd/fluxexp/command/list.go`), so the invariant that health
  derivation lives in one place holds and must keep holding.
- **Rendering**: a glyph for `pending` in `internal/render/tree.go`; a CSS rule in
  `internal/ui/web/` (the badge already uses the health value as its class name).
- **Behaviour change for existing nodes**: objects previously reported `unknown` will
  now mostly report `healthy`, `unhealthy` or `pending`. Some workloads mid-rollout
  will move from `unhealthy` to `pending`. That is the point of the change, but it is
  a visible shift for anyone reading the tree today.
