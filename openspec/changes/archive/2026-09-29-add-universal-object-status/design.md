## Context

`K8sHealth` in `internal/resolver/resolver.go` is the single derivation of Kubernetes
health, shared by every surface. Its whole body is `healthFromReady`: find
`.status.conditions[type=Ready]`, map `True`/`False`, and answer `unknown` otherwise.
That was right for Flux objects, which all publish `Ready`. It is wrong for what a
Kustomization actually applies — see the measured breakdown in `proposal.md`: 172 of
255 inventory entries answer `unknown`.

Two constraints shape the design.

The **first** is that `K8sHealth` must stay the only place this logic lives.
`internal/ui/service.go` (twice), `flux_object.go`, `helmrelease.go`,
`kustomization.go`, `generic_k8s.go` and `cmd/fluxexp/command/list.go` all call into
it today, which is why the change is a body swap rather than a sweep. The invariant
already exists in `CLAUDE.md`; the risk is diluting it, not establishing it.

The **second** is that health is an `engine` value. `engine.Health` is deliberately
domain-independent — the engine carries it and never computes it — so anything added
there must make sense for a future GCP resolver, not just for Kubernetes.

## Goals / Non-Goals

**Goals:**

- Every Kubernetes object reachable through traversal carries a meaningful status.
  `unknown` becomes a real answer ("this object publishes a verdict I cannot read")
  rather than the default for anything without a `Ready` condition.
- Distinguish *nothing to know* from *not reconciled yet* from *cannot read*, which
  today are one word.
- Do not introduce false greens while removing false unknowns.
- Keep verdicts aligned with `flux` where alignment is honest, and diverge explicitly,
  in one documented place, where it is not.

**Non-Goals:**

- Per-kind status readers for operator CRDs (NifiCluster's `state`, Service's
  `loadBalancer`, ResourceQuota's `hard`/`used`). Family D is served by the generic
  rules here; bespoke readers are a later, separate decision per operator.
- Replacing `Freshness`. Freshness answers "is this reconciled against the latest
  revision of its source"; the statuses here answer "is this object working". They
  stay distinct fields, as they are today.
- Rewriting `WorkloadResolver`'s health arithmetic (see Decision 5).
- Any change to how children or dependencies are discovered.

## Decisions

### 1. kstatus as the derivation base

Use `sigs.k8s.io/cli-utils/pkg/kstatus/status.Compute`, which returns one of
`Current` / `InProgress` / `Failed` / `Terminating` / `NotFound` plus a message.

It is the library Flux uses for its own health checks, it already encodes
`observedGeneration` handling and the `Reconciling`/`Stalled`/`Ready` convention, and
it ships per-kind rules for Deployment, StatefulSet, DaemonSet, Job, Pod, PVC, CRD and
Service. Verified against 20 real object kinds pulled from `dev`: 20/20
produce a verdict, none `unknown`.

Cost is one new module. `k8s.io/api`, `k8s.io/utils` and `k8s.io/kube-openapi` were
verified to be in the build graph already via client-go, so the addition to a `go.mod`
holding only cobra, apimachinery and client-go is narrower than it looks.

*Alternative rejected — hand-rolled generic rules (~150 lines: ordered positive
condition list, negative list, `observedGeneration`).* Zero dependencies and fully in
the repo's idiom, but it is a reimplementation of kstatus that would drift from what
Flux considers healthy. The polarity layer in Decision 3 is that hand-rolled logic,
reduced to the part kstatus genuinely lacks.

*Alternative rejected — kstatus in `GenericK8sResolver` only.* Smaller blast radius,
but it leaves two health derivations in the codebase, which is the exact duplication
the single-`K8sHealth` invariant exists to prevent.

### 2. Status mapping, and `pending` as a fifth health value

| kstatus | `engine.Health` |
|---|---|
| `Current` | `healthy` |
| `InProgress` | `pending` |
| `Failed` | `unhealthy` |
| `Terminating` | `pending` |
| `NotFound` | `unhealthy` |

`pending` is new. Its main source is generation drift: kstatus reports
`InProgress` with `"<Kind> generation is 4, but latest observed generation is 1"` when
`metadata.generation` runs ahead of `status.observedGeneration`. That is a precise,
useful thing to say and it has no home in the current four values — it is neither
healthy, nor broken, nor unknown.

Adding a value to `engine.Health` is not a domain leak: "the backend has not caught up
with the declared spec" is as meaningful for a GCP resource as for a Kubernetes one.

*Alternative rejected — put drift in `Detail` text.* Cheapest, but the information
would not be filterable, sortable or visible at a glance, which is most of its value.

*Alternative rejected — extend `Freshness` to any object carrying
`observedGeneration`.* `Freshness` is source-relative (`up-to-date` / `behind` a
revision) and is deliberately never claimed for a listed entry, because computing it
without the entry's source can wrongly report `up-to-date`. Generation drift is
self-contained and always computable. Folding one into the other would make
`Freshness` mean two things.

### 3. A condition-polarity layer above kstatus

**This is the load-bearing decision.** kstatus recognises exactly three condition
types — `Reconciling`, `Stalled`, `Ready`. Anything else falls through to its generic
rule: nothing recognised and generation current → `Current`. Verified:

```
Bundle          Synced=False       → Current    ← false green
KubernetesRole  Configured=False   → Current    ← false green
Thing           Ready=False        → InProgress
Thing           Stalled=True       → Failed
Thing           generation 4 / observed 1 → InProgress
Thing           (no status at all) → Current
```

Family B is 39 entries — CRD `Established`, Bundle `Synced`, KubernetesRole and Policy
`Configured`, PodDisruptionBudget `DisruptionAllowed`, ComputeClass `Health`. Taking
kstatus unqualified would turn 39 noisy `unknown`s into 39 green lies. That is a
regression, not a fix: an `unknown` invites a look, a green tells the reader to move
on.

So before a generic `Current` is accepted, fluxexp reads the object's conditions
through a polarity table:

- **positive** (`False` → unhealthy): `Synced`, `Configured`, `Established`,
  `Available`, `Healthy`, `Succeeded`, `Complete`, …
- **negative** (`True` → unhealthy): `Degraded`, `Stalled`, `Failed`, …

A polarity table is unavoidable in any case: `Degraded=False` is good news and
`Synced=False` is bad news, so "any condition that is False means unhealthy" is not a
rule that can be written. The table only overrides a *generic* `Current` — where
kstatus has a per-kind rule or recognises a condition, kstatus wins.

*Alternative rejected — a polarity table plus an `unknown` guard*, where an object
carrying only conditions absent from the table refuses kstatus's `Current` and reports
`unknown` with the condition type in the detail. It makes false greens structurally
impossible, at the price of residual `unknown`s on exotic CRDs. Rejected as contrary
to the goal of the change; revisit if false greens are observed in practice.

*Alternative rejected — accept kstatus's `Current` verbatim*, for strict alignment
with `flux`. Rejected: alignment is not worth a green Bundle that is not syncing.

### 4. `Ready=False` stays unhealthy

kstatus classes `Ready=False` as `InProgress` unless `Stalled` is also true, on the
reasoning that most controllers pass through not-ready while converging. fluxexp
overrides that: `Ready=False` → `unhealthy`, as today.

The 83 entries in family C are correct today and a broken Certificate must stay red,
not become amber. This is the one deliberate divergence from `flux`; it lives in one
function and is called out here so it is not later "fixed" into alignment.

*Alternative rejected — a temporal threshold* (`Ready=False` is `pending` until
`lastTransitionTime` is older than, say, five minutes, then `unhealthy`). It models
reality best, but it puts an arbitrary clock inside health derivation and makes the
same object report differently on two consecutive reads for reasons the reader cannot
see.

### 5. `WorkloadResolver` keeps its own arithmetic, gains the drift check

kstatus has Deployment/StatefulSet/DaemonSet/Job/Pod rules, so `WorkloadResolver`
could be deleted. It is kept:

- Its details are better for this tool's purpose — `"0/2 ready"` against kstatus's
  `"Deployment is not available. Replicas: 0"`.
- Its behaviour is already specified and tested in `workload-health-resolver`;
  replacing it would rewrite a stable capability for no user-visible gain.

It does gain one thing: the generation-drift check runs **before** the replica
arithmetic. A Deployment whose controller has not yet observed the new spec reports
`pending`, not `unhealthy` on a replica count that describes the previous spec. That
check is the shared one, not a second implementation.

### 6. Objects with no `.status` report healthy

Family A is 101 entries, 40% of the inventory. A ClusterRole cannot be unhealthy; it
can only exist or not, and if it was fetched, it exists. kstatus already answers
`Current` / `"Resource is always ready"` for these, so this decision is mostly a
matter of not overriding it.

*Alternative rejected — a `present` health value* distinct from `healthy`, with a
neutral glyph. It is arguably more honest, and it would keep 40% of the tree from
going green and diluting the signal. Rejected for simplicity: it is a fifth value on
top of `pending`'s, every renderer and filter would have to learn it, and the
distinction it draws ("exists" vs "working") is one the detail text can carry.

Note this makes *fetchability* the status: a missing object is already an error node
via the engine's partial-failure handling, so "healthy" here reads as "applied and
present", which is exactly what a reader wants to know about a ConfigMap.

## Risks / Trade-offs

- **A false green is worse than an `unknown`, and the polarity table is a list that
  can be incomplete.** → The table only overrides a generic `Current`; new condition
  types are one line each and belong beside the derivation with a test. Decision 3's
  rejected `unknown`-guard variant is the escape hatch if incompleteness bites.

- **Behaviour visibly shifts for anyone reading the tree today**: most `unknown`s
  become `healthy`, and some mid-rollout workloads move from `unhealthy` to `pending`.
  → Intended, and stated in the proposal. Worth a line in the README when this lands.

- **Divergence from `flux` on `Ready=False`** could confuse someone comparing
  `fluxexp` and `flux get` side by side. → One documented divergence, in one function,
  recorded in Decision 4.

- **kstatus's per-kind rules could contradict a resolver's own reading** as kstatus
  evolves (Service, CRD and PVC all have built-in rules). → Specific resolvers keep
  their own derivation where they have one; kstatus governs the fallback path.

- **`status.Compute` requires `metadata.generation` as an `int64`.** The dynamic
  client produces `int64`; `encoding/json` produces `float64` and makes `Compute`
  fail with `".metadata.generation accessor error: 1 is of the type float64, expected
  int64"`. This was hit while validating the design. → Test fixtures and builders MUST
  use `int64` literals (or decode with `k8s.io/apimachinery/pkg/util/json`, never
  `encoding/json`). Worth a comment in `fake_test.go` where the builders live.

- **Dependency surface grows** for the first time beyond cobra + apimachinery +
  client-go. → Verified to be a single module; `cli-utils` pins older `k8s.io/api`
  versions but MVS resolves upward to the versions client-go already requires.

## Migration Plan

Not applicable — read-only tool, no persisted state, no API contract. `pending` is an
additive value in an existing enumeration; the UI badge derives its CSS class from the
value, so an unstyled `pending` degrades to a default badge rather than breaking.
Rollback is reverting the commit.

## Open Questions

- Should the polarity table be extendable from configuration rather than compiled in?
  Deferred until a real cluster produces a condition type worth adding that fluxexp
  does not already know.
- Family D's operator-specific status fields (NifiCluster `state`, Service
  `loadBalancer`) currently resolve to `healthy` via the generic path. Whether any of
  them deserves a bespoke reader is a per-operator question, out of scope here.
