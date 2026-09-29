# fluxexp

Traverse the full dependency graph of resources reconciled by **FluxCD** — from
a starting Flux object all the way down to the concrete resource it ultimately
produces — and report health along the way.

The `flux` CLI stops at the Flux object boundary. `fluxexp` follows the chain to
the end: a `Kustomization` applies objects (some of which are themselves Flux
objects, or operator CRDs, or references to cloud resources), and `fluxexp`
walks that graph across **any backend** (Kubernetes today, GCP or others later).

## How it works

The core is a **domain-agnostic traversal engine** plus a **registry of
resolvers selected by matcher**. A node is a backend-neutral reference
(`domain`, `type`, `coordinates`). Given a reference it matches, a resolver
fetches the object itself and returns its health plus child references — which
may live in a **different domain**. The engine only walks the graph (BFS),
dedupes by a stable key (cycle-safe), and tolerates partial failure. Adding a new
hop — even in a new backend — means registering a new resolver; the engine never
changes.

## Usage

```bash
# List Flux resources of a kind with their health (discover starting points)
fluxexp list --kind Kustomization              # all namespaces
fluxexp list --kind HelmRelease --unhealthy    # only the ones with problems
fluxexp list --kind GitRepository -n team-a    # a single namespace

# Open the local exploration portal (web UI) — pick a context, list root
# Kustomizations, drill layer by layer. No auth; loopback only.
fluxexp ui                       # http://127.0.0.1:8765
fluxexp ui --address 127.0.0.1:9000

# Traverse from a Flux Kustomization (apiVersion/kind default to Kustomization)
fluxexp traverse -n flux-system --name apps

# Start from any Kubernetes resource
fluxexp traverse --api-version helm.toolkit.fluxcd.io/v2 --kind HelmRelease -n team-a --name web

# Point at a specific kubeconfig / context
fluxexp traverse -n flux-system --name apps --kubeconfig ~/.kube/config --context staging
```

Output is an indented tree; each line shows the node's health, type, coordinates
and domain. Error nodes are marked inline with their reason.

A node's **dependencies** — what it needs to reconcile — are printed first with a
`⇢` marker and their fields inline; they are resolved one level deep and never
descended into, so a `dependsOn` chain never explodes the tree. Then come the
objects it **applies**, ordered Flux-first, then other containers (marked `▸`),
then leaves:

```
✖ ▸ Kustomization flux/cert-issuer [unhealthy] [failed] — error dependency 'flux/cert-manager' is not ready
├─ ✔ ⇢ GitRepository flux/flux [healthy] — repo gitlab/infra/fleet · branch main · interval 1m
├─ ✖ ⇢ Kustomization flux/cert-manager [unhealthy] [failed] — error dependency 'flux/alloy' is not ready
├─ ✔   Certificate cert-manager/lets-encrypt-dns-account [healthy]
└─ ?   Policy cert-manager/team-app-read-cf-token [unknown]
```

The web UI groups the same information into one card: the node with its
dependencies nested inside it, and what it applies listed below.

## Increment roadmap

- **I1 (done)**: traversal engine + Flux Kustomization resolver
  (`.status.inventory`) + generic Kubernetes fallback. CLI tree output.
- **I2 (done)**: HelmRelease resolver — expands a HelmRelease into the objects
  its chart deployed, via the Helm release storage Secret.
- **I3 (done)**: web UI initiated — a local, no-auth portal (`fluxexp ui`) that
  lists kube contexts, shows root Kustomizations, and drills the resource graph
  layer by layer, reusing the same resolvers.
- **I4 (done)**: workload-health resolver — real health for Deployment,
  StatefulSet, DaemonSet, ReplicaSet, Pod and Job (replica counts / phase /
  conditions), removing the `unknown` noise in both CLI and UI.
- **I5 (done)**: workload descent via ownerReferences — Deployment → active
  ReplicaSet → Pods (and StatefulSet/DaemonSet/Job → Pods), so the graph reaches
  the running Pod.
- **I6 (done)**: reconciliation freshness — a status distinct from health that
  tells whether a Kustomization has applied its source's latest revision
  (up-to-date / behind / failed / suspended), with the applied/source commit ids
  and sync times, in the CLI tree and the UI (badge on the roots home + a fields
  panel).
- **I7 (done)**: expandable-first exploration — resolvers declare whether a
  reference can descend; children are ordered in three stable tiers (Flux
  objects, then other containers, then leaves); the CLI marks containers with a
  chevron and the UI adds an accent. The graph also gains a second edge class,
  **dependencies** — shown with their health and fields but never descended into
  — so a Kustomization surfaces its `sourceRef` and its `dependsOn` targets
  grouped with it, in one card in the UI. Flux source / image-automation objects
  expose their useful fields (repository, branch/tag, interval, scanned image,
  selected tag) inline, without a click.
- **I8 (done)**: navigable UI — the exploration path lives in the URL
  (`/?context=<ctx>&p=<type>:<ns>:<name>~…`), so any node can be reloaded,
  bookmarked and shared; browser Back/Forward walk the trail instead of leaving
  the app, and the breadcrumb gains an up-to-parent control.
- **I10 (done)**: a status for every object. A Kustomization's inventory is full
  of kinds nothing knew how to read — 67% of a real cluster's 255 entries showed
  `unknown`. Health derivation now runs through `kstatus`, the library Flux uses
  itself, with a condition-polarity layer on top so a `Bundle` with
  `Synced=False` renders red rather than green. **Expect a visible shift**: most
  former `unknown`s become `healthy` (a ClusterRole has nothing to report, and
  existing is all it can do), and a new fifth status, **`pending`**, takes the
  cases that were neither working nor broken — a controller that has not yet
  observed the current spec, a Pod still scheduling, a running Job. On the
  measured cluster: 642 nodes, zero `unknown`.
- Next: generalize owner-descent to operator CRDs; cloud verification (e.g. GCP
  via a `gcp`-domain resolver, using `gcloud`).

## Development

```bash
make build   # build ./bin/fluxexp
make test    # go test ./...
make run ARGS="traverse -n flux-system --name apps"
```

Spec-driven via [OpenSpec](https://github.com/Fission-AI/OpenSpec); see
`openspec/`.
