# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`fluxexp` is a Go CLI (plus an embedded local web portal) that traverses the **full**
dependency graph of resources reconciled by FluxCD — from a starting Flux object down to
the concrete resources it ultimately produces — reporting health along the way. Where the
`flux` CLI stops at the Flux object boundary, `fluxexp` keeps following the chain
(Kustomization → inventory → HelmRelease → chart objects → Deployment → ReplicaSet → Pod),
and is designed to cross **backends** mid-graph (Kubernetes today, GCP or others later).

## Commands

```bash
make build                                   # build ./bin/fluxexp
make test                                    # go test ./...
make vet                                     # go vet ./...
make run ARGS="traverse -n flux-system --name apps"
make tidy                                    # go mod tidy

go test ./internal/resolver/                 # one package
go test ./internal/resolver/ -run TestKustomization_Health -v   # one test
```

Running the binary (all read-only; standard kubeconfig resolution, `--kubeconfig`/`--context`
to override):

```bash
fluxexp list --kind Kustomization [--unhealthy] [-n ns]  # discover starting points
fluxexp traverse -n flux-system --name apps              # defaults to Flux Kustomization
fluxexp traverse --api-version helm.toolkit.fluxcd.io/v2 --kind HelmRelease -n team-a --name web
fluxexp ui [--address 127.0.0.1:8765]                    # local no-auth portal
```

## Architecture

The design rule the whole codebase serves: **adding a new hop — even into a new backend —
means registering a new resolver, and nothing else changes.**

**`internal/engine`** — domain-agnostic traversal. Knows nothing about Kubernetes, Flux, or
Helm and performs **no backend I/O**. `Traverse(root Ref, resolve ResolveFunc) *Node` does a
BFS: it dedupes by `Ref.Key()` (cycle/diamond safe — a repeat is attached as a `Visited: true`
leaf pointer) and turns any resolve error into an `Error` node while siblings keep going.
`Ref` is `{Domain, Type, Coords, Display}`; `Health` and `Freshness` are opaque strings the
engine only carries, never computes. Adding backend knowledge to this package is a design
violation.

**`internal/resolver`** — the resolver contract, the matcher-based `Registry`, and the
concrete resolvers. A `Resolver` declares `Matches(ref)`, fetches its **own** object through
the shared clients in `ResolveContext`, and returns health + child refs (children may be in a
different domain). `Registry.For` tries specific resolvers in registration order, then the
per-domain fallback. **`NewDefaultRegistry()` is the single wiring point shared by the CLI and
the UI** — register new resolvers there so both surfaces behave identically.

Current resolvers: `KustomizationResolver` (children from `.status.inventory.entries`, plus
reconciliation freshness), `HelmReleaseResolver` (children from the gzip+base64 Helm release
storage Secret `sh.helm.release.v1.<name>.v<n>`), `WorkloadResolver` (real health from replica
counts / phase / conditions, and descent via `ownerReferences`: Deployment → active ReplicaSet
→ Pods), `GenericK8sResolver` (domain fallback: `Ready` condition, no children).

Kubernetes-specific reference encoding lives here, not in `internal/k8s`: `K8sRef` /
`DecodeK8sRef` encode `Type` as `"<apiVersion>|<Kind>"` and `Coords` as `namespace`/`name`.

**`internal/k8s`** — read-only dynamic client + discovery-backed RESTMapper, so arbitrary CRDs
resolve without generated types. Exposes `Get` / `List` / `Namespaced` (the `K8sGetter`
interface resolvers depend on) and `ListContexts` for the UI. It knows nothing about the engine.

**`internal/render`** — pure functions from `*engine.Node` / rows to text (tree, list). No I/O.

**`internal/ui`** — `Service` + `Handler` serving `/api/contexts`, `/api/roots`, `/api/expand`
and an `//go:embed web/*` single-page app. It reuses the same registry, but expands **one hop
per call**: `Expand` runs the node's full resolver and then computes only a *cheap* health for
each child (fetch + `Ready`) rather than recursing. It caches one client per kube context.

**`cmd/fluxexp`** — cobra command tree (`list`, `traverse`, `ui`). `traverse` exits non-zero
only when the *root* fails to resolve; deeper failures are rendered inline as error nodes.

### Invariants

- Traversal is strictly read-only; nothing in the graph path mutates cluster state.
- Partial failure is a first-class result, not an abort.
- Health/freshness derivation for Kubernetes lives in `resolver` (`K8sHealth`,
  `KustomizationFreshness`) and is shared by the CLI, the `list` command, and the UI — don't
  reimplement Ready-condition logic at a call site.
- `Freshness` (up-to-date / behind / failed / suspended) is a status **distinct from health**,
  set only by resolvers that can compute it; empty means "not applicable".

### Tests

Table-driven, no live cluster. `internal/resolver/fake_test.go` holds the shared `fakeGetter`
(in-memory objects keyed `kind|namespace|name`) and object builders; the UI injects a fake
cluster through `newService`. `internal/resolver/freshness.go` has an overridable `now` var for
deterministic relative times.

## Spec-driven workflow (OpenSpec)

This repo is developed spec-first with the OpenSpec CLI (`openspec`, v1.3.1) and the skills in
`.claude/skills/` — `/opsx:explore`, `/opsx:propose`, `/opsx:apply`, `/opsx:archive`.

- `openspec/project.md` — project context and architectural principles; read it before any
  design decision.
- `openspec/specs/<capability>/spec.md` — stabilized requirements in
  `### Requirement:` / `#### Scenario:` WHEN/THEN form. These are the source of truth for
  behavior.
- `openspec/changes/<name>/` — an active change (proposal, design, tasks, spec deltas);
  archived under `openspec/changes/archive/<date>-<name>/` once implemented.

Work is delivered in numbered increments (I1…I6 done, see the README roadmap). A feature
lands as: propose a change → implement its tasks → archive the change, which folds its spec
deltas into `openspec/specs/`. Prefer that flow over ad-hoc edits for anything behavioral.
