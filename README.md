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

# Traverse from a Flux Kustomization (apiVersion/kind default to Kustomization)
fluxexp traverse -n flux-system --name apps

# Start from any Kubernetes resource
fluxexp traverse --api-version helm.toolkit.fluxcd.io/v2 --kind HelmRelease -n team-a --name web

# Point at a specific kubeconfig / context
fluxexp traverse -n flux-system --name apps --kubeconfig ~/.kube/config --context staging
```

Output is an indented tree; each line shows the node's health, type, coordinates
and domain. Error nodes are marked inline with their reason.

## Increment roadmap

- **I1 (current)**: traversal engine + Flux Kustomization resolver
  (`.status.inventory`) + generic Kubernetes fallback. CLI tree output.
- I2: HelmRelease resolver (Helm release storage).
- I3: operator-CRD leaf resolver + Ready aggregation.
- I4: cloud verification (e.g. GCP via a `gcp`-domain resolver).
- I5: web UI (backend + embedded frontend) over the same resolved tree.

## Development

```bash
make build   # build ./bin/fluxexp
make test    # go test ./...
make run ARGS="traverse -n flux-system --name apps"
```

Spec-driven via [OpenSpec](https://github.com/Fission-AI/OpenSpec); see
`openspec/`.
