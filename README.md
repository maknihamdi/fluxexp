# fluxexp

[![ci](https://github.com/maknihamdi/fluxexp/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/maknihamdi/fluxexp/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/maknihamdi/fluxexp)](https://github.com/maknihamdi/fluxexp/releases/latest)

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

## Install

`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` and `windows/amd64`
are published for every version.

**With the Go toolchain** — one line, every platform including Windows, no
platform to pick and no download to verify; it reports the right version through
build info:

```bash
go install github.com/maknihamdi/fluxexp/cmd/fluxexp@latest
```

**Without it, on Linux or macOS** — the installer detects your platform, resolves
the current version, verifies the checksum and installs to `/usr/local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/maknihamdi/fluxexp/main/install.sh | sh
```

That line executes a script fetched from the network. The same thing in three
steps, if you would rather read it first — it is short:

```bash
curl -fsSL -O https://raw.githubusercontent.com/maknihamdi/fluxexp/main/install.sh
less install.sh
sh install.sh
```

Two variables, both optional:

- `FLUXEXP_BIN_DIR` — where to install, default `/usr/local/bin`. A directory you
  own needs no `sudo`: `FLUXEXP_BIN_DIR=~/.local/bin sh install.sh`. Elevation is
  used only when the destination is not writable, and the script says when it is.
- `FLUXEXP_VERSION` — install that version instead of the newest. **Pin it in
  CI**: the install is then reproducible, and it skips the release API, which is
  rate-limited to 60 requests an hour per address.

The script verifies or refuses: `fluxexp` reads your kubeconfig and talks to your
clusters with your credentials, so if neither `sha256sum` (Linux) nor `shasum`
(macOS) is present it stops rather than installing something unchecked. Each
release also carries `install.sh` as an asset, covered by that release's
`SHA256SUMS` — an immutable copy of the thing that does the verifying.

**On Windows** there is no installer; this one is POSIX shell. With the Go
toolchain, use the line above. Without it, download
`fluxexp_<version>_windows_amd64.zip` from the
[latest release](https://github.com/maknihamdi/fluxexp/releases/latest) — it
contains `fluxexp.exe` — verify it against `SHA256SUMS`, and put it on your
`PATH`.

Or run the container image — see [Container image](#container-image). It is the
right choice for running the portal *in* a cluster, and the wrong one for a CLI
whose job is to read your local kubeconfig.

> The `ci` badge above reports the latest run on `main`, not the build of the
> released tag. GitHub's badge accepts a branch or an event, and has no parameter
> for a tag, so a green badge means `main` is healthy — read the `release` badge
> for which version is out.

`fluxexp` is MIT licensed; the licence travels inside every release archive.

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

## Run it in the cluster

`deploy/` is a kustomize base that runs the portal inside the cluster it
inspects. The pod's ServiceAccount becomes the credential, so there is no
kubeconfig to distribute.

```bash
kubectl apply -k deploy/
kubectl -n fluxexp port-forward svc/fluxexp 8765:8765
# then open http://127.0.0.1:8765
```

The context selector will show the one cluster the pod runs in.

**Read this before applying it anywhere shared.** The pod is granted `get` and
`list` on **every resource in every API group, Secrets included** — and the
portal has no authentication. Anyone who can reach it reads the whole cluster
through that grant.

The breadth is not laziness. The traversal follows a Kustomization's inventory
into CRDs of operators this build has never seen, which is the entire premise, and
expanding a HelmRelease means reading its Helm storage Secret. The built-in `view`
role excludes Secrets, so under it every HelmRelease is a dead end. It also cannot
be narrowed by deleting a rule: authorization is allow-only, so the wildcard
already covers Secrets and "everything except Secrets" is inexpressible. The one
real alternative is to enumerate resources explicitly and accept that an unlisted
kind becomes a permission error instead of a node.

For that reason the base ships **no Ingress and no LoadBalancer**: access is a
`port-forward`, which requires the viewer to already hold cluster credentials.
Exposing it on a hostname is a decision about an authenticating proxy, and a
NetworkPolicy restricting who may reach the Service is the first thing to add on a
shared cluster.

The resource requests are a starting point, not a measurement — revise them from
`kubectl top pod`. The autoscaler targets CPU, which is a compromise: the workload
waits on the API server rather than on local compute, so utilisation stays low
exactly when the portal feels slow. `maxReplicas` is a bound on aggregate
API-server pressure, since the client-go rate limiter is per process.

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
- **I9 (done)**: HelmRelease dependencies — a HelmRelease is grouped with what it
  needs, like a Kustomization: the source its chart comes from (`spec.chartRef`,
  else `spec.chart.spec.sourceRef`) then its `spec.dependsOn` entries. A *listed*
  HelmRelease shows that group without being resolved, so no Helm storage Secret
  is read to draw a row. A `chartRef`-style release continues one hop — release →
  HelmChart → HelmRepository — and the HelmChart gains the fields it never had.
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
- **I11 (done)**: CI, and an image. GitHub Actions validates every push and pull
  request (`gofmt`, `vet`, tests, build, plus an *unpushed* image build so a
  broken `Dockerfile` fails a pull request rather than a release), and a `v*` tag
  publishes `quay.io/hamdi_makni/fluxexp` — a static binary on a distroless base,
  no shell, uid 65532. Nothing is published outside a tag.
- **I12 (done)**: releases for humans. The same tag now also builds `fluxexp` for
  five targets (`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`,
  `windows/amd64`) and attaches the archives, with one `SHA256SUMS` covering them
  all, to a GitHub Release. The binary gained `--version`, which reports the
  released version, or the version the Go toolchain recorded for anything built
  otherwise. MIT licensed.
- **I13 (done)**: run it in the cluster. A kustomize base (`deploy/`) with the
  ServiceAccount and cluster-wide read the traversal actually needs, a Deployment
  that declares no replicas because the HPA owns them, a PodDisruptionBudget that
  is meaningful rather than deadlocking, and a security context that asserts what
  the image already is. The portal also stopped being laptop-only: with no
  kubeconfig it now reports the one cluster it runs in, instead of an empty
  context selector and no explanation.
- **I14 (done)**: one line to install. `install.sh` detects the platform from
  `uname` (never from `go env GOARCH`, which reports `amd64` on an Apple Silicon
  machine whose toolchain is an amd64 build), resolves the current version from
  the release API so no instruction names a version, verifies the archive with
  whichever of `sha256sum` or `shasum` exists — and **refuses to install** if
  neither does — and uses `sudo` only when the destination is not writable. The
  script ships as a release asset too, covered by that release's `SHA256SUMS`.
  The README install section no longer asks the reader for the version, the OS or
  the architecture.
- Next: generalize owner-descent to operator CRDs; cloud verification (e.g. GCP
  via a `gcp`-domain resolver, using `gcloud`).

## Development

```bash
make build   # build ./bin/fluxexp
make test    # go test ./...
make run ARGS="traverse -n flux-system --name apps"
```

Every push and pull request runs the same checks in CI (`.github/workflows/ci.yml`):
`gofmt`, `make vet`, `make test`, `make build`, plus an image build that is not
pushed — so a broken `Dockerfile` fails a pull request rather than a release.

### Container image

`quay.io/hamdi_makni/fluxexp`, published **only when a `v*` tag is pushed**: an
image exists because someone tagged a release, so anything deployed can be
reproduced from a tag. A tag `v1.4.2` publishes `1.4.2` and `1.4`, and moves
`latest` only if it is the highest released version.

The image is a static binary on a distroless base — no shell, no package manager,
non-root by default. The frontend is compiled in, so there is no asset step.

```bash
docker build -t fluxexp:dev .
docker run --rm -p 8765:8765 fluxexp:dev ui --address 0.0.0.0:8765
```

The `--address` is required to reach it from outside the container: the binary
binds loopback by default and that default does not change.

More commands — the image checks, reading an image's labels back, watching a run
— are in [`docs/commands.md`](docs/commands.md).

Spec-driven via [OpenSpec](https://github.com/Fission-AI/OpenSpec); see
`openspec/`.
