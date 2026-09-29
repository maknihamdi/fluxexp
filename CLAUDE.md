# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`fluxexp` is a Go CLI (plus an embedded local web portal) that traverses the **full**
dependency graph of resources reconciled by FluxCD — from a starting Flux object down to
the concrete resources it ulx@timately produces — reporting health along the way. Where the
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
`GenericK8sResolver` (domain fallback: the shared status derivation, no children).

**`internal/resolver/status.go` is the whole of Kubernetes health derivation.** `K8sHealth(obj)`
is the only function that decides what an object's state means, and every surface goes through
it. Its base verdict comes from `kstatus` (`sigs.k8s.io/cli-utils`), the library Flux uses for
its own health checks, with three deliberate layers on top:

- **A condition-polarity table**, consulted only when kstatus returns a *generic* `Current` —
  meaning it recognised neither the kind nor any of the three condition types it knows
  (`Reconciling`, `Stalled`, `Ready`). Without it a `Bundle` with `Synced=False` renders green,
  and a false green is worse than an `unknown` because it never invites a second look. Polarity
  cannot be inferred: `Degraded=False` is good news and `Synced=False` is bad news, and both are
  just a type and a status. New condition types are one line each.
- **`Ready=False` is `unhealthy`**, where kstatus says in-progress. This is the one place
  fluxexp knowingly disagrees with flux; without it every broken Certificate turns amber.
- **Generation drift** (`metadata.generation` > `status.observedGeneration`) is `pending` and is
  checked *first*: everything else the status says describes a superseded spec.

The workload rules (replica counts, Pod phase, Job conditions) live in `workload.go` beside the
resolver that descends those kinds, but `K8sHealth` dispatches to them through `workloadHealth`.
That indirection is load-bearing: the CLI resolves a Pod through `WorkloadResolver` while a UI
layer merely lists it and derives health from the fetched object, and the two must not reach
different verdicts for the same Pod.

An object with no `.status` at all is **healthy** — existing is the whole of what a ClusterRole
or a ConfigMap can do, and that family alone was 40% of a real cluster's inventory.

Two helpers exist so a surface holding an object can read more from it **without a second
call**: `FieldsForFetched(ref, obj)` and `DependencyRefsForFetched(ref, obj)`. A Kustomization's
`sourceRef` and `dependsOn` are in its own manifest, so a listed row never needs to be resolved
— and its inventory never needs computing — just to show its dependency group. The same holds
for a HelmRelease's chart source and `dependsOn`, where it matters more: resolving one gunzips
its Helm storage Secret. `DependencyRefsForFetched` lives in `dependencies.go` and **dispatches
on group + kind** — a new kind that declares dependencies registers there, its derivation staying
beside its own resolver; kinds absent from the dispatch yield nothing. A HelmChart is in it too:
a HelmRelease using `spec.chartRef` names a chart, not a repository, so the chain continues one
hop — release → chart → repository — each hop derived from an object already in hand rather than
flattened by an extra fetch. Declaring a dependency does not make a kind expandable: the HelmChart
stays a leaf. `IsFluxRef`
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

The frontend is a **tree pane plus a node pane**. The tree holds every branch the user
has expanded (`tree.nodes`: refKey → `{node, at}`); the node pane shows the selected
reference. Expanding and selecting are separate: the twisty resolves one reference and
reveals its children without touching the node pane, the row label selects. Only
selection is a navigation, so expanding adds no history entry.

The URL stays **the source of truth for where the user is**: `state.trail` is a
projection of `?p=<hop>~<hop>…` (a hop is `type:ns:name`, or `domain:type:ns:name` off
the `kubernetes` default) and holds the path to the **selected** node — open branches
are deliberately not in the address. Every navigation pushes a history entry and
`popstate` re-reads the URL, so a shared link and a Back press render identically. A
trail is restored without resolving its intermediate hops: they are drawn as a *spine*
in the tree from their own identifiers (labels derived from the type, mirroring
`friendlyLabel`), carrying no health mark, and only the selected node is resolved.

Because the tree keeps fetched layers on screen, the old "never hold a layer" rule
became **never show held material as current**: selecting always re-resolves (the node
pane is never rendered from `tree.nodes`), every held branch shows the age of its read
with a re-read control, and collapsing discards what the branch held. That is why going
back still re-resolves, the same reason the server memo lives for one request only.

The tree walk carries a `seen` set of the keys on the current path: the Flux bootstrap
Kustomization applies itself, and a repeat is rendered as a leaf pointer (`↩`) instead
of being descended into — the same guard the engine's `visited` map provides.

The name and health filters narrow the tree and the open layer over material already
fetched — never a call — and a branch is kept when anything under it matches, so
filtering cannot hide the path to a match.

`dropNestedDuplicates` enforces, in the UI only, that a layer shows each reference once: a row
already displayed as another entry's nested dependency is dropped, unless it carries its own
dependency group (so mutual references cannot erase both). It is UI-only on purpose — in the CLI
a top-level row carries its whole subtree, so dropping it would lose information.

**`cmd/fluxexp`** — cobra command tree (`list`, `traverse`, `ui`). `traverse` exits non-zero
only when the *root* fails to resolve; deeper failures are rendered inline as error nodes.

### Invariants

- Traversal is strictly read-only; nothing in the graph path mutates cluster state.
- Partial failure is a first-class result, not an abort.
- Health/freshness derivation for Kubernetes lives in `resolver` (`K8sHealth` in `status.go`,
  `KustomizationFreshness`) and is shared by the CLI, the `list` command, and the UI — don't
  reimplement condition reading, generation comparison or replica counting at a call site. A
  kind with rules of its own registers them in `workloadHealth`, not at the surface that
  happens to ask.
- **Every object gets a status; `unknown` is a last resort, never a default.** It is reserved
  for state the system genuinely cannot read — a status kstatus fails to parse, a Pod whose
  node stopped reporting. "Nothing to say" is `healthy`, "not there yet" is `pending`.
- `Pending` is the fifth health value and separates three meanings `unknown` used to conflate:
  nothing to know, not yet reconciled, cannot be read. It covers generation drift,
  `Reconciling=True`, termination, a `Pending` Pod and a running Job.
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

Two things a test object must get right, both because health derivation now runs through
kstatus:

- **Integers must be `int64` literals.** kstatus reads `metadata.generation` and
  `status.observedGeneration` with unstructured's typed accessors and fails hard on anything
  else ("1 is of the type float64, expected int64"). `encoding/json` produces exactly that
  `float64`, so decode any fixture with `k8s.io/apimachinery/pkg/util/json`.
- **`apiVersion` and `kind` must be set.** Derivation dispatches on them to find the kind's
  rules, so a workload fixture without them is not a workload and silently falls through to the
  generic path.

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

Work is delivered in numbered increments (I1…I10 done, see the README roadmap). A feature
lands as: propose a change → implement its tasks → archive the change, which folds its spec
deltas into `openspec/specs/`. Prefer that flow over ad-hoc edits for anything behavioral.

Utilise les skills openspec pour explorer, proposer et appliquer des changements. 