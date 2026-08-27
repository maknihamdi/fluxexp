## Context

Expansion currently returns children in the order the resolver produced them —
inventory order for a Kustomization, manifest order for a HelmRelease. That order
carries no signal about where the graph continues. In a real inventory the
leaves dominate (a `flux/traefik` expansion is 6 children, of which 2 descend),
so a user hunting for the next hop clicks mostly dead ends.

The constraint that shapes the whole design is architectural: the traversal
engine is domain-blind and performs no I/O, and "can this thing be opened" is
domain knowledge. Whatever we build must therefore live on the resolver side of
that boundary, and must be shared by the two surfaces — the CLI reaches
resolvers through `Registry.ResolveFunc`, while the UI's `Expand` calls
`Registry.For(ref)` and then `Resolve` directly. Anything implemented in only
one of those paths will drift.

## Goals / Non-Goals

**Goals:**

- Tell the user, before they click, which children carry a subtree.
- Order children Flux-first, then containers, then leaves, identically in the
  CLI and the UI.
- Make a Kustomization's source and declared dependencies visible as one group,
  with their health and their useful fields (repository, branch, interval)
  readable without any interaction.
- Cost nothing at resolve time: no extra API call to answer "is this expandable".
- Keep `internal/engine` untouched.

**Non-Goals:**

- Traversing into dependencies. They are shown one level deep, never expanded.

- Counting children ("HelmRelease — 12 objects"). That needs the real resolve of
  every child; see the rejected probing option below.
- Guaranteeing a container is non-empty. The flag is a hint about the *type*,
  not a measurement of the instance.
- Re-ordering by health, name, or kind. Only the container/leaf split is
  introduced; existing relative order is preserved inside each group.
- Any change to the roots home ordering (roots are all Kustomizations).

## Decisions

### Declarative predicate on the resolver, not probing

`Resolver` gains `Expandable(ref engine.Ref) bool`, answered from the reference
alone — no fetch, no context, no error. The declarations are:

| Resolver | Expandable |
|---|---|
| `KustomizationResolver` | always true |
| `HelmReleaseResolver` | always true |
| `WorkloadResolver` | true, except `Pod` → false |
| `GenericK8sResolver` | always false |

*Alternative rejected — probing:* actually resolve each child to see whether it
returns children. It is exact, and it would give us child counts for free, but it
turns one expansion into N full resolutions: expanding a 30-object Kustomization
would fetch 30 objects plus, for each HelmRelease among them, its storage Secret.
That is a large regression on the portal's core interaction to buy precision we
do not need — the user only needs to know where it is worth clicking.

### On the interface, accepting the breaking change

The predicate goes on the `Resolver` interface itself rather than an optional
side-interface discovered by type assertion.

*Alternative rejected — optional interface* (`if e, ok := r.(expandable); ok`,
default false): non-breaking, but the default is silent and wrong in the
dangerous direction. A future resolver that descends — the operator-CRD or GCP
resolvers on the roadmap — would compile fine, return children, and still have
them sorted as leaves, with nothing to catch it. Putting it on the interface
makes the compiler ask the question once per resolver. The cost is bounded and
mechanical: four resolvers and two test doubles.

### Ordering applied once, in the registry

`Registry` gains two methods: `Expandable(ref) bool`, which delegates to the
matched resolver (and reports false when nothing matches), and
`OrderChildren(refs []engine.Ref) []engine.Ref`, which returns the
containers-first ordering.

`Registry.ResolveFunc` applies `OrderChildren` to every `Result.Children` it
returns, so the engine — and therefore the CLI — inherits the order without
knowing the rule exists. The UI's `Expand`, which bypasses `ResolveFunc`, calls
`OrderChildren` on the same slice before building its DTOs.

*Alternative rejected — sorting in the engine:* it is the one place both surfaces
already share, but it would require the engine to ask a domain question about
each ref, breaking the principle that keeps the engine reusable across backends.
*Alternative rejected — each resolver sorts its own children:* four
implementations of one rule, and every future resolver silently opts out by
forgetting.

### Flux objects rank above other containers

Ordering is three tiers, not two: any reference in a `*.toolkit.fluxcd.io` group
first, then the remaining expandable children, then the rest. A GitRepository is
a leaf — it descends nowhere — yet it outranks an expandable Deployment, because
when reading a Kustomization the question "what drives this?" comes before "what
does it run?".

The tier predicate is computed **inside the ordering helper**, from the
reference's group, rather than becoming a `Priority(ref) int` on the `Resolver`
interface.

*Alternative rejected — priority on the interface:* it looks symmetric with
`Expandable`, but it puts Flux knowledge in the wrong place. A GitRepository is
handled by `GenericK8sResolver`, so the generic fallback — whose entire purpose
is to know nothing about any particular ecosystem — would have to recognise Flux
groups. Even after introducing a dedicated Flux source resolver, the
notification kinds (`Alert`, `Provider`, `Receiver`) still land on the fallback,
so the group test cannot be fully delegated to resolvers anyway. Keeping it in
the helper also keeps the promise made above: the ordering rule has exactly one
implementation.

### Dependencies are a second edge class, shown but not descended

The first attempt made the source an ordinary child. Reviewed in place, it read
badly: the GitRepository sat among ConfigMaps as a peer of things the
Kustomization *applies*, and its repository, branch and interval stayed hidden
behind a click. Those are the two facts a reader wants first, so they must be
visible without interaction and visually attached to the Kustomization.

So `Result` and `Node` gain `Dependencies` alongside `Children`, and the engine
treats them differently: a dependency is **resolved one level deep** — enough for
its health and fields — and its own children are never enqueued.

That asymmetry is what makes `spec.dependsOn` safe to include. A dependency of a
Kustomization is usually another Kustomization; descending into it would splice
its entire subtree in, and on this cluster `cluster-ready` → `cert-issuer` →
`cert-manager` → `alloy` would drag four full inventories into one tree. Showing
the dependency node with its health answers "what is it waiting on?" — which is
the actual question — at a bounded cost of one resolve per dependency.

Dependencies are deliberately **excluded from the engine's visited map**. Adding
them would let a dependency leaf poison a later real expansion: the second
occurrence of the same reference as a genuine child would be attached as an
already-visited pointer and lose its subtree. Keeping them out means a shared
GitRepository is resolved once per Kustomization that names it — a handful of
cheap GETs, in exchange for every card being complete on its own.

*Alternative rejected — a `Role` field on `Ref`:* one list, each entry tagged
child or dependency. It avoids touching the engine's shape, but `Ref` is the
graph's identity type — `Key()` is built from it — and a presentational role has
no business there; two refs identical but for their role would be two different
nodes.

### The Kustomization declares source and dependsOn as dependencies

`KustomizationResolver` already fetches `spec.sourceRef` to compute freshness;
the object was simply discarded afterwards. It now becomes the first dependency,
followed by the `spec.dependsOn` entries in declaration order. The inventory
stays in `Children`.

A source that cannot be fetched must not break the Kustomization: the existing
fetch already degrades to nil for freshness, and the dependency is simply omitted
rather than emitted as an error node. A `dependsOn` entry, by contrast, is
emitted from the spec without pre-fetching — if it does not exist, resolving it
produces an error node, which is exactly the signal wanted: a Kustomization
waiting on something absent.

*Alternative rejected — flattening the source into the Kustomization's own
fields:* cheaper still, but it erases the source's own health. A GitRepository
failing to fetch its artifact is a distinct, actionable state, and the card is
precisely where that distinction should stay visible.

### Fields without a second fetch

Flux fields must appear inline wherever a Flux object is listed, not only on the
node the user opened. The UI's child rows already fetch each object to compute a
cheap health, so the fields are derivable from an object it is holding.

`FieldsForFetched(ref, obj)` exposes exactly that: fields computable from an
already-retrieved object, empty for kinds that have none. It costs no call, and
it keeps the extraction rules in the resolver package instead of leaking Flux
schema paths into the UI.

*Alternative rejected — running the full resolver per child row:* it would return
fields uniformly, but a HelmRelease resolve fetches and gunzips its storage
Secret, and a Kustomization resolve fetches its source. That is the probing cost
the whole design set out to avoid.

### A dedicated resolver for Flux source and image kinds

`FluxObjectResolver` claims `source.toolkit.fluxcd.io` and
`image.toolkit.fluxcd.io` kinds, reports health from the Ready condition exactly
as the fallback does, declares itself non-expandable, and adds per-kind fields:
repository URL, branch/tag/semver and interval for a GitRepository; the
equivalent for OCIRepository, Bucket and HelmRepository; the scanned image and
last scan for an ImageRepository; the policy rule and selected tag for an
ImagePolicy.

It is registered ahead of the generic fallback and reuses the existing `Field`
carrier, so both surfaces render it with no new plumbing — the CLI's inline
`fieldsSummary` and the UI's fields panel already handle it.

### Stable ordering

`OrderChildren` MUST be a stable partition, not a comparison sort: expandable
children keep their mutual order, then the rest keep theirs. Inventory order is
meaningful (it is roughly apply order) and today's output is deterministic;
a comparison sort on a boolean key would be free to shuffle equal elements and
make the tree churn between runs.

### Marker rendering

The CLI reserves a fixed two-character slot between the health glyph and the
label: `▸ ` for an expandable node, two spaces otherwise, so labels stay
column-aligned. In the CLI the whole subtree is already printed, so the chevron's
real job is to distinguish a node that is a leaf *by nature* (ConfigMap) from one
that is expandable but came back empty — while the ordering does the heavy
lifting.

The UI exposes the flag as `expandable` on `NodeDTO` — set both for the expanded
node and for each child — so the frontend renders the chevron and the accent
without re-deriving type rules in JavaScript. That mirrors how `freshness` and
`fields` are already handed to the frontend rather than recomputed there.

## Risks / Trade-offs

**A container can open onto nothing** → A Deployment whose ReplicaSets are all
scaled to zero, or a Kustomization with an empty inventory, is marked expandable
and expands to an empty list. This is inherent to a type-level hint and is the
price of not probing. Mitigated by keeping the marker modest (a chevron, not a
count) so it promises "worth a click", not "has N children"; the spec states the
hint semantics explicitly so it is not read as a bug later.

**The declaration can drift from the implementation** → `Expandable` says true
while `Resolve` never returns children, or the reverse. Nothing enforces
agreement at compile time. Mitigated by a table-driven test per resolver that
asserts the predicate against the kinds the resolver actually descends into, and
by the `Pod` case being covered explicitly since it is the one intra-resolver
exception.

**A shared source is re-resolved per Kustomization** → Because dependencies skip
the visited map, a GitRepository named by twenty Kustomizations is fetched twenty
times in a full traversal. The cost is one GET each and the benefit is that every
card is self-contained. Mitigated in the UI by expanding a single layer at a
time; if it ever bites in the CLI, the fix is a per-traversal fetch cache in the
resolve context, not a change to the traversal rule.

**A dependency's own subtree stays invisible** → Showing a `dependsOn`
Kustomization without descending means the user cannot see why *it* is failing
without navigating to it. This is the deliberate trade against tree explosion;
the node carries its health and message, and in the UI it is one click away.

**Field extraction depends on Flux schema paths** → Reading `spec.ref.branch`,
`spec.policy`, `status.lastScanResult` couples the resolver to Flux's CRD shapes,
which shift between API versions. Mitigated by treating every field as optional:
a path that is absent yields no field rather than an error or an empty label, so
an unexpected schema degrades to today's behaviour instead of breaking the node.

**Interface change breaks out-of-tree implementations** → Only in-tree
implementations exist today (four resolvers, two test doubles), so the blast
radius is the compiler pointing at six files in one commit.
