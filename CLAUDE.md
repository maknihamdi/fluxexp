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
`Ref` is `{Domain, Type, Coords, Display}`; `Health`, `Freshness` and `Expandable` are opaque
values the engine only carries, never computes. Adding backend knowledge to this package is a
design violation.

A node has **two kinds of outgoing edge**. `Children` are what it produces and are traversed
recursively. `Dependencies` are what it requires to reconcile: resolved **one level deep**,
never descended into, and deliberately kept **out of the `visited` map** — marking one visited
would make a later genuine child occurrence of the same reference collapse to an
already-visited pointer and lose its subtree. That asymmetry is what makes a Kustomization's
`dependsOn` safe to show: descending would splice whole inventories into the tree.

**`internal/resolver`** — the resolver contract, the matcher-based `Registry`, and the
concrete resolvers. A `Resolver` declares `Matches(ref)` and `Expandable(ref)` — the latter
answered from the reference alone, no fetch, no error — fetches its **own** object through the
shared clients in `ResolveContext`, and returns health plus child and dependency refs (either
may be in a different domain). `Registry.For` tries specific resolvers in registration order,
then the per-domain fallback. **`NewDefaultRegistry()` is the single wiring point shared by the
CLI and the UI** — register new resolvers there so both surfaces behave identically.

**`Registry.ResolveFunc` is the single entry point both surfaces resolve through.** It applies
the rules that must not diverge: three-tier child ordering (`OrderChildren`: Flux objects, then
other expandable refs, then the rest — stable within each tier), removal of children that merely
repeat a declared dependency, and the `Expandable` flag. The UI used to call `Resolve` directly
and re-apply ordering itself; that duplication is exactly how the two surfaces drifted. Do not
reintroduce it.

Current resolvers: `KustomizationResolver` (children from `.status.inventory.entries`;
dependencies from `spec.sourceRef` + `spec.dependsOn`; reconciliation freshness),
`HelmReleaseResolver` (children from the gzip+base64 Helm release storage Secret
`sh.helm.release.v1.<name>.v<n>`), `WorkloadResolver` (health from replica counts / phase /
conditions, descent via `ownerReferences`: Deployment → active ReplicaSet → Pods),
`FluxObjectResolver` (the `source.` and `image.toolkit.fluxcd.io` kinds — leaves, but they
surface repository / tracked ref / interval / scanned image / selected tag),
`GenericK8sResolver` (domain fallback: `Ready` condition, no children).

Two helpers exist so a surface holding an object can read more from it **without a second
call**: `FieldsForFetched(ref, obj)` and `DependencyRefsForFetched(ref, obj)`. A Kustomization's
`sourceRef` and `dependsOn` are in its own manifest, so a listed row never needs to be resolved
— and its inventory never needs computing — just to show its dependency group. `IsFluxRef`
answers the Flux-group question for ordering; it lives in the ordering helper rather than on the
`Resolver` interface, because the generic fallback (which handles GitRepository) must stay
ignorant of any particular ecosystem.

Kubernetes-specific reference encoding lives here, not in `internal/k8s`: `K8sRef` /
`DecodeK8sRef` encode `Type` as `"<apiVersion>|<Kind>"` and `Coords` as `namespace`/`name`.

**`internal/k8s`** — read-only dynamic client + discovery-backed RESTMapper, so arbitrary CRDs
resolve without generated types. Exposes `Get` / `List` / `Namespaced` (the `K8sGetter`
interface resolvers depend on) and `ListContexts` for the UI. It knows nothing about the engine.

**`internal/render`** — pure functions from `*engine.Node` / rows to text (tree, list). No I/O.

**`internal/ui`** — `Service` + `Handler` serving `/api/contexts`, `/api/roots`, `/api/expand`
and an `//go:embed web/*` single-page app. It resolves through `Registry.ResolveFunc` and
expands **one hop per call**: each listed entry gets a single fetch (health, fields, and its
dependency refs), never a full resolve. It caches one client per kube context, wraps each
request in a `memoGetter` (one layer asks for the same shared source many times; the memo lives
for **one request only**, because caching across requests would serve stale health), and
resolves a layer's entries concurrently through a bounded pool (`layerConcurrency`).

The frontend keeps **the URL as the source of truth for where the user is**:
`state.trail` is a projection of `?p=<hop>~<hop>…` (a hop is `type:ns:name`, or
`domain:type:ns:name` off the `kubernetes` default), every navigation pushes a
history entry, and `popstate` re-reads the URL — so a shared link and a Back press
render identically. A trail is restored without resolving its intermediate hops
(labels are derived from the type, mirroring `friendlyLabel`), and going back
**re-resolves**: caching a visited layer client-side would serve stale health, the
same reason the server memo lives for one request only.

`dropNestedDuplicates` enforces, in the UI only, that a layer shows each reference once: a row
already displayed as another entry's nested dependency is dropped, unless it carries its own
dependency group (so mutual references cannot erase both). It is UI-only on purpose — in the CLI
a top-level row carries its whole subtree, so dropping it would lose information.

**`cmd/fluxexp`** — cobra command tree (`list`, `traverse`, `ui`). `traverse` exits non-zero
only when the *root* fails to resolve; deeper failures are rendered inline as error nodes.

### Invariants

- Traversal is strictly read-only; nothing in the graph path mutates cluster state.
- Partial failure is a first-class result, not an abort.
- Health/freshness derivation for Kubernetes lives in `resolver` (`K8sHealth`,
  `KustomizationFreshness`) and is shared by the CLI, the `list` command, and the UI — don't
  reimplement Ready-condition logic at a call site.
- `Freshness` (up-to-date / behind / failed / suspended) is a status **distinct from health**,
  set only by resolvers that can compute it; empty means "not applicable". Precedence:
  suspended > failed > up-to-date > behind. It is never claimed for a listed entry, because
  computing it without that entry's source could wrongly report "up-to-date" for something
  behind.
- **Expandability is a hint about a type, not a measurement of an instance.** A Deployment
  scaled to zero is marked expandable and opens empty; that is specified behaviour, not a bug.
- **Dependencies are shown, never descended into**, and a reference appears **once**: a declared
  dependency is dropped from its own node's children (Flux bootstrap applies the GitRepository
  it reconciles from), and in the UI from the surrounding layer too.
- Anything derivable from an object already fetched must not trigger a second call — that is
  what the `*ForFetched` helpers are for. Resolving every row to learn something about it is the
  probing cost this design exists to avoid: a HelmRelease resolve gunzips a storage Secret.

### Performance gotcha

`k8s.LoadClient` raises `cfg.QPS`/`cfg.Burst` to 50/100. client-go defaults to **5 QPS / burst
10**, sized for a controller reconciling in the background, and that default alone turned a
67-entry UI layer into a 25s wait. Parallelising changed nothing until the limiter was raised
(25s → 12s with the per-request memo → ~1.2s). If a layer ever feels slow again, check the
limiter before rewriting the scheduling.

The recursive CLI `traverse` of a whole cluster root is still ~79s: the engine is sequential,
unlike the UI. Known, not addressed.

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
