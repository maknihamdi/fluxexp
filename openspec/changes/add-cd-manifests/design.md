## Context

What the manifests have to run, taken from the code rather than assumed:

- **One process, one port.** `fluxexp ui` starts a single `http.Server` on
  `--address`, default `127.0.0.1:8765` (`cmd/fluxexp/command/ui.go`). The
  frontend is compiled in (`//go:embed web/*`), so there is no sidecar, no
  volume, no init container.
- **No state.** `internal/ui.Service` caches one client per context in memory and
  wraps each request in a memo that lives **for one request only** (CLAUDE.md:
  caching across requests would serve stale health). Nothing is persisted, so any
  replica can serve any request and a restart loses nothing.
- **No disk writes.** The discovery client is `discovery.NewDiscoveryClientForConfig`,
  not the cached variant — the cached one writes to `~/.kube/cache`, this one does
  not. That is what makes `readOnlyRootFilesystem: true` safe rather than
  hopeful.
- **Credentials already work in a pod.** `LoadClient` builds its config through
  `clientcmd.NewNonInteractiveDeferredLoadingClientConfig`, whose deferred loader
  falls back to `rest.InClusterConfig()` when the loading rules yield an empty
  config. No code change is needed to *authenticate* in a pod.
- **But the context selector comes back empty.** `k8s.ListContexts` calls
  `rules.Load()`, which skips missing kubeconfig files without erroring, so in a
  pod it returns an empty list and no error. The frontend's `loadContexts()` then
  builds an empty `<select>` and sets `state.context = ""`. Requests still work
  (an empty context is exactly what resolves in-cluster) but the user is shown a
  context bar with nothing in it.
- **Read pressure is bursty.** A single UI layer can be tens of parallel GETs;
  `LoadClient` raises `QPS`/`Burst` to 50/100 for that reason, and that limiter is
  **per process**. This is the fact that decides the resource limits below.
- **The portal has no authentication.** By design, for a loopback tool. Deploying
  it changes who can reach it, and that is the one genuinely uncomfortable part of
  this change.

## Goals / Non-Goals

**Goals:**

- `kubectl apply -k deploy/` yields a working, read-only portal in the cluster it
  is applied to.
- The permissions the pod holds are the ones the traversal actually needs, stated
  with their reason, and no write verb anywhere.
- Availability and scaling are declared coherently — a PDB that protects rather
  than deadlocks, an HPA that owns the replica count alone.
- The in-cluster context gap is closed with the smallest change that makes the
  selector honest.

**Non-Goals:**

- External exposure (Ingress, LoadBalancer), authentication, NetworkPolicy.
- Overlays, environments, a Flux `Kustomization` to apply this.
- Metrics, ServiceMonitor, dashboards, alerting.
- Any change to traversal, health derivation or the resolvers.

## Decisions

### One kustomize base, no overlays

`deploy/kustomization.yaml` plus one file per object, `namespace: fluxexp` set in
the kustomization, and a `Namespace` object so the base is self-contained.

Alternative: `deploy/base` + `deploy/overlays/dev`. Rejected — an overlay exists
to express a difference, and there is one environment. Creating the directory pair
now means writing a patch that patches nothing, and the day a second environment
appears the base moves in one `git mv`.

Common labels via `labels:` with `includeSelectors: true`, which generates the
Deployment's selector and pod labels and the Service's selector from one place.

**It does not reach the PodDisruptionBudget's selector, nor a spread
constraint's** — measured on kustomize v5.0.4, where both came out empty. That is
worse than it looks: an empty selector on either of those matches *every* pod in
the namespace, so the manifests would have behaved correctly only because the
namespace is dedicated to this one workload. Both are therefore written
explicitly, with a comment saying why, and the rendered output is checked for all
five selectors agreeing rather than assumed from the transformer's reputation.

### The Deployment declares no `replicas`

An HPA owns `spec.replicas`. If the Deployment also declares it, every
`kubectl apply` / Flux reconcile resets the count to the manifest value and the
HPA scales it back — a flap that costs a rollout each time. Omitting the field
means the Deployment is created at 1 and the HPA immediately takes it to
`minReplicas: 2`.

Alternative: keep `replicas: 2` in the manifest and rely on Flux ignoring the
field. Rejected — that requires the applier to be configured to ignore it, which
is a property of how this is deployed, and the manifests must be correct under
plain `kubectl apply -k`.

### RBAC: `get`/`list` on everything, and why that is the requirement

```yaml
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list"]
```

Two properties of the tool force this:

1. **The set of kinds is not knowable in advance.** A Kustomization's
   `.status.inventory.entries` names whatever it applied, including CRDs of
   operators this build has never heard of — following them is the entire premise
   (`GenericK8sResolver` is the fallback for exactly that). An enumerated
   ClusterRole would turn every unlisted kind into an error node, which reads as
   "broken" rather than "not permitted".
2. **Secrets are load-bearing, not incidental.** `HelmReleaseResolver` finds a
   release's objects by reading `sh.helm.release.v1.<name>.v<n>` and
   gunzipping it. The built-in `view` ClusterRole excludes Secrets, so under it
   every HelmRelease becomes a dead end.

So the honest statement is: **this pod can read every Secret in the cluster**, and
anyone who reaches the portal reads through it, unauthenticated. That is stated in
the spec as a property of the deployment, not buried.

This design first claimed the Secret grant could be split into a second, deletable
rule. **It cannot**, and writing the manifests is what exposed it. RBAC is
allow-only with no exclusion syntax: the wildcard already includes Secrets, so a
second rule naming them is redundant and deleting it revokes nothing; and "every
group except the core one" is inexpressible, because `apiGroups: ["*"]` spans the
core group where Secrets live. Nor can it be narrowed by name —
`resourceNames` takes exact names and does not apply to `list`, so
`sh.helm.release.v1.*` is not a thing RBAC can say.

The grant is therefore **one rule, all-or-nothing**, carrying the comment that
says so. An operator who refuses it has one real alternative: enumerate the
resources explicitly, and accept that a kind nobody listed in advance becomes a
permission error instead of a node — which is trading away the premise. That
trade is documented beside the rule rather than pretended away by a rule that
looks optional.

`watch` is deliberately absent — nothing in the code opens a watch — and no write
verb appears, which makes the read-only invariant enforceable by the API server
rather than only by code review.

### PDB `minAvailable: 1` with HPA `minReplicas: 2`

The two numbers are one decision. `minAvailable: 1` at one replica makes a node
drain block forever: the only pod can never be evicted. At two replicas it means
"drain one at a time", which is what a budget is for.

Alternative: `maxUnavailable: 1`, which is deadlock-free at any replica count.
Rejected because it also permits evicting the last pod when there is only one, so
it protects nothing in the case that matters.

Two replicas without spreading them is a budget in name only — both pods on one
node and a single drain violates it. `topologySpreadConstraints` with `maxSkew:
1` over `kubernetes.io/hostname` and `whenUnsatisfiable: ScheduleAnyway` spreads
them when the cluster allows and never blocks scheduling when it does not.

### The HPA targets CPU, and that is a compromise worth naming

`minReplicas: 2`, `maxReplicas: 6`, `averageUtilization: 70` on CPU.

The honest assessment: **CPU is close to the wrong signal here.** A portal request
is mostly waiting on the API server — the measurement in CLAUDE.md has a 67-entry
layer going from 25s to ~1.2s purely by raising the client-go rate limiter, which
is the profile of a workload bound by request concurrency, not by local compute.
CPU utilisation will stay low while the portal feels slow, so the HPA will not
react to the thing a user complains about. Worse, scaling out multiplies API
pressure: the 50 QPS / 100 burst limiter is per process, so six replicas can put
300 QPS on the API server.

It ships anyway, because CPU is the one metric available with no metric adapter
and no exported application metrics, and because horizontal capacity for
*concurrent users* is real even when it does nothing for one slow layer. What
makes it safe is `maxReplicas: 6` — a bound on aggregate API pressure, chosen for
that reason rather than for capacity. The upgrade path is an exported
`inflight-requests` metric and a `Pods` metric source, which needs the binary to
expose metrics first.

### Resource requests: a CPU request, no CPU limit

`requests: cpu 100m, memory 128Mi`; `limits: memory 256Mi` and **no CPU limit**.

A CPU limit throttles at quota boundaries, and the workload is precisely a burst
of tens of concurrent requests followed by idle — the shape CFS throttling
punishes hardest, turning a 1.2s layer back into a multi-second one. A request
with no limit gives the scheduler what it needs and lets a burst use idle
capacity. The memory limit stays, because a memory burst is a leak, not a spike.

The HPA needs the CPU *request* (utilisation is computed against it), not a limit,
so this costs nothing there. The numbers are a starting point to be revised from
observed usage, not a measurement — recorded as such rather than presented as
tuned.

### Probes on `/`

`readinessProbe` and `livenessProbe` both `httpGet: /` on the container port. The
embedded file server answers `/` with the SPA, so a 200 proves the HTTP server is
serving.

It deliberately does **not** prove cluster access. A probe that checked the API
server would take the whole Deployment down when the API server has a bad minute —
and the portal's correct behaviour then is to render and report the failure per
node, which is the partial-failure invariant. A `/healthz` endpoint would say
exactly what `/` already says, so it is not added.

### `securityContext` asserts what the image is

`runAsNonRoot: true`, `runAsUser: 65532`, `readOnlyRootFilesystem: true`,
`allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`,
`seccompProfile: RuntimeDefault`.

The distroless `:nonroot` base already runs as 65532, so this is redundant today —
and that is the point: it is an assertion that fails loudly if the base image ever
changes, instead of a property nobody notices losing. `readOnlyRootFilesystem` is
safe because of the non-cached discovery client noted in Context; no `emptyDir` for
`/tmp` is needed, and adding one pre-emptively would hide the day something starts
writing.

### The image is pinned to a version, never `latest`

`quay.io/hamdi_makni/fluxexp:0.4.0`, with `imagePullPolicy: IfNotPresent`. A
manifest pointing at `latest` describes a different workload on every pull, and
the CI change moves `latest` on every release. Bumping the tag is the deployment.

**The version is the one this change releases, not the newest that exists.** The
first attempt pinned `0.3.0`, which was measured deploying the exact bug this
change fixes: the portal came up answering `{"contexts":[]}`, because the
in-cluster context listing is code this change adds and `0.3.0` predates it. So
the manifest references `0.4.0`, and the task verifying the context selector moves
after the release rather than before it. The cost is a window between this commit
and the `v0.4.0` tag where applying the base gives ImagePullBackOff — preferable
to a manifest that ships the defect.

### The in-cluster context: a sentinel name, mapped in one place

`ListContexts` gains: when the kubeconfig yields no contexts **and**
`rest.InClusterConfig()` is available, return a single entry named `in-cluster`,
marked current, with the API server host as its cluster. `Service.clientFor` maps
that one name to the empty string before calling `LoadClient`, because the empty
context is what `clientcmd` resolves in-cluster.

Alternative: return a context whose name is `""`. Rejected — the frontend renders
the option label as `name + " (current)"`, so it would show `" (current)"`, and an
empty `<option>` value is indistinguishable from "nothing selected".

Alternative: pass `in-cluster` through as `overrides.CurrentContext`. Rejected
because it breaks: with a non-existent context name, `DirectClientConfig`'s
`ConfirmUsable` fails with "context was not found", which is *not* an empty-config
error, so the deferred loader never reaches its in-cluster fallback. The mapping
has to happen before `LoadClient` is called.

The mapping lives in `clientFor` — the single place the UI turns a context name
into a client — and not in `LoadClient`, which is the CLI's path too and has no
business inventing context names.

## Risks / Trade-offs

- **An unauthenticated portal with cluster-wide read, reachable from any pod in
  the cluster** → no Ingress and no LoadBalancer in this change; access is
  `kubectl port-forward`, which requires the viewer to already have cluster
  credentials. Stated as a spec requirement so that adding an Ingress is a visible
  decision rather than a YAML edit. A NetworkPolicy restricting ingress is the
  first thing to add on a shared cluster.
- **The pod can read every Secret in the cluster** → the two RBAC rules are kept
  separate so the Secret rule can be deleted; the cost of deleting it (HelmRelease
  expansion stops working) is documented next to it.
- **The HPA will not react to the slowness users notice** → named above;
  `maxReplicas: 6` bounds the damage it can do, and the real fix is an exported
  in-flight metric.
- **Six replicas × 50 QPS against one API server** → `maxReplicas: 6` is chosen as
  that bound. If the API server shows pressure, lower `maxReplicas` before
  touching the per-process limiter, which is what makes a single layer fast.
- **`readOnlyRootFilesystem` breaks if anything starts writing to disk** → a
  Go-level surprise (a temp file, a switch to the cached discovery client) becomes
  a crash-loop, not silent corruption. Accepted; it is also the early warning.
- **The in-cluster context listing is only reachable in a pod** → a unit test
  cannot call `rest.InClusterConfig()` honestly. The detection is put behind a
  function variable in the same style as `internal/resolver/freshness.go`'s
  overridable `now`, so both branches are testable without a cluster.
- **Resource numbers are guesses** → recorded as a starting point in the spec and
  the README, to be revised from `kubectl top` after a week.

## Migration Plan

Nothing exists to migrate. Order:

1. Land the in-cluster context change with its tests (`make test` green), since
   the manifests are useless without it.
2. `kubectl apply -k deploy/ --dry-run=server` against a real cluster: it
   validates the schemas, the RBAC and the HPA target reference without creating
   anything.
3. Apply for real on the dev cluster, `port-forward`, and confirm the portal lists
   the `in-cluster` context and expands a Kustomization and a HelmRelease — the
   HelmRelease is the test that the Secret permission works.
4. Drain a node (or delete a pod) and confirm the PDB allows exactly one at a
   time.

Rollback: `kubectl delete -k deploy/`. Nothing outside the namespace and the two
cluster-scoped RBAC objects is touched, and the tool is read-only, so deleting it
cannot have left the cluster in a different state than it found it.

## Open Questions

- The namespace name: `fluxexp` assumed. If the convention on the target cluster
  is to co-locate Flux tooling in `flux-system`, that is a one-line change in the
  kustomization — but it also means the ClusterRoleBinding's subject moves, so it
  is worth settling before the first apply.
- Whether the portal should eventually get an authenticating proxy (oauth2-proxy)
  or be left as a `port-forward` tool. This change assumes the latter and makes
  the former a later, explicit decision.
- `deploy/` as the directory name versus `manifests/` or `config/` (the
  kubebuilder convention). `deploy/` chosen as the least surprising for a
  non-operator project.
