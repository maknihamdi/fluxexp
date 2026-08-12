## Context

I4 computes workload health but returns no children, so exploration stops before
the running Pods. Kubernetes links a workload to what it created via
`.metadata.ownerReferences` on the *child* (child → parent, by UID). This
increment follows that link downward (Deployment → ReplicaSet → Pod, and
StatefulSet/DaemonSet/Job → Pod), the first ownerReference-based descent. The
mechanism was verified on the live cluster (a Deployment owns several ReplicaSets;
only one is active).

## Goals / Non-Goals

**Goals:**
- Descend workloads to owned objects by UID-matched ownerReferences.
- Deployment shows only active ReplicaSets; omitted old-revision count surfaced.
- A small reusable owner-descent helper.
- Works in CLI and UI without engine changes.

**Non-Goals:**
- Descending arbitrary operator CRDs (proves the mechanism on workloads first).
- Cross-namespace ownership (these kinds own within their namespace).
- Container-level detail under a Pod (Pod stays a leaf).

## Decisions

### D1: Descent by listing + UID owner-filter
To find a parent's children we `List` the child kind in the parent's namespace
and keep those whose `ownerReferences[].uid == parent.uid`. UID (not name) is the
reliable owner key. **Alternative:** label-selector matching — rejected: labels
are convention, ownerReferences are the authoritative link and are uniform across
kinds.

### D2: Child kind per parent
- Deployment → `apps/v1` ReplicaSet
- ReplicaSet / StatefulSet / DaemonSet / Job → `v1` Pod
- Pod → none
Fixed per kind (no discovery needed); each parent has exactly one child kind in
this increment.

### D3: Deployment shows active ReplicaSets only, count the rest
A Deployment keeps every old ReplicaSet (revision history) scaled to 0. Showing
them all is noise. The resolver returns only ReplicaSets with `status.replicas`
> 0 (active/rolling revisions) and appends `· N old revisions` to the Deployment
detail when any are omitted — visible, not silently dropped (per the no-silent-caps
rule). **Alternative:** show all ReplicaSets — rejected: 5–6 empty children per
Deployment drowns the running one.

### D4: `List` joins the resolver cluster contract
`resolver.K8sGetter` gains `List(ctx, apiVersion, kind, namespace)`. `*k8s.Client`
already implements it; the UI's `cluster` interface already required it (now it
just embeds `K8sGetter`). Resolver test fakes gain a `List`.

### D5: Reusable helper
`ownedChildren(ctx, rc, parent, ns, childAPIVersion, childKind, keep func) []Ref`
lists + owner-filters + optional `keep` predicate (used for the ReplicaSet
active filter). This is the seed of a general operator-descent helper later.

## Risks / Trade-offs

- **List cost** (one List per workload expansion) → bounded and lazy; the UI
  expands one hop at a time, the CLI walks the tree once. Acceptable; add caching
  later if needed.
- **RBAC**: listing ReplicaSets/Pods requires read access; a denied List becomes
  a per-node resolve error (engine tolerates it), not a crash.
- **Many Pods** under a big ReplicaSet/DaemonSet → shown as-is; a future cap with
  a visible "N more" note can be added if it becomes unwieldy.
- **Adopted/orphaned children** (ownerReference removed) won't appear — correct:
  they are no longer owned.

## Open Questions

- Should a rolling Deployment (two active ReplicaSets) show both? Yes — both have
  replicas > 0, so both appear, which is the accurate in-progress picture.
- Do we want Pod → container/events later? Out of scope; Pod stays a leaf for now.
