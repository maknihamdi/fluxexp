## Context

A Kustomization's commit travels in stages: pushed commit → GitRepository
`status.artifact.revision` (fetched) → Kustomization `status.lastAppliedRevision`
(applied). Health (Ready) does not tell you if the applied revision is the latest
the source knows. This increment adds a freshness signal from cluster state only,
and surfaces the concrete revisions/times. Fields verified live: Kustomization
`lastAppliedRevision`/`lastAttemptedRevision`, GitRepository
`status.artifact.revision`/`lastUpdateTime`; format `<ref>@sha1:<sha40>`.

## Goals / Non-Goals

**Goals:**
- A freshness status distinct from health: up-to-date / behind / failed /
  suspended.
- Compute it from applied vs source-fetched revision (no git access).
- Surface applied/source short revisions (full retained), synced + fetched times.
- Show it in the CLI tree, the UI node view, and the UI roots home.
- Keep the engine domain-blind (generic carriers only).

**Non-Goals:**
- Comparing to the real git remote HEAD (`git ls-remote`) — later opt-in.
- HelmRelease chart freshness — follow-up.
- Triggering reconciles (read-only).

## Decisions

### D1: Generic carriers on the node, not Flux logic in the engine
Add to `engine`: `Freshness` (enum: `up-to-date`, `behind`, `failed`,
`suspended`, "") and `Field{Label, Value}`. `Result` and `Node` gain
`Freshness` and `Fields []Field`; `resolveNode` copies them across. The engine
computes nothing — it carries what a resolver sets, exactly like `Health`. This
keeps "the engine knows nothing about Flux" intact: freshness is a generic
second status (any GitOps-style resolver may set it), and fields are opaque
label/value pairs. **Alternative:** stuff everything into the `Detail` string —
rejected: the UI needs structured data for badges and the full-vs-short sha.

### D2: Freshness computed in a shared helper
`resolver.KustomizationFreshness(ks, source *unstructured) (engine.Freshness, []engine.Field)`
implements the rules (suspend → Ready False → applied==source → behind). It is
called by both the Kustomization resolver (traverse/expand) and the UI roots
summary, so the logic lives once. The caller fetches the source object first
(the resolver via `GetK8s`, the UI `Roots` via a per-name cache since many
Kustomizations share one source).

### D3: Fetch the source generically
The source is `spec.sourceRef.{kind,name}` in `spec.sourceRef.namespace` (default
the Kustomization's namespace), apiVersion `source.toolkit.fluxcd.io/v1`. All
source kinds (GitRepository/OCIRepository/Bucket) expose
`status.artifact.revision` + `lastUpdateTime`, so the comparison is kind-generic.
A failed source fetch degrades gracefully to health-only freshness.

### D4: Fields set by the Kustomization resolver
Ordered fields: `Applied` (short, full in value/title), `Synced`
(relative + absolute), `Source` (the source object + its fetched short revision +
fetched time, revealing the newer revision when behind), and on failure
`Attempted` + `Error`. Short form: parse `<ref>@sha1:<sha>` (or `<ref>/<sha>`,
`<ref>@<sha>`) → `<ref>@<sha[:7]>`.

### D5: Rendering
- CLI (`render.Tree`): after `[health]`, add `[freshness]` when set; append the
  key fields to the line detail. Non-freshness nodes unchanged.
- UI node view: a freshness badge next to health + a fields panel (full revision
  in the value's `title`).
- UI roots home: freshness badge + short applied sha + synced time per card.

### D6: Relative time
A small `humanizeSince(ts)` uses `time.Now()` (real Go runtime — allowed here)
and `time.Parse(time.RFC3339, ...)` to render "3m ago"; the absolute timestamp is
kept alongside.

## Risks / Trade-offs

- **False "up-to-date" right after a push** (source hasn't polled the remote yet)
  → documented; mitigated by showing the applied **sha** prominently so the user
  confirms their own commit. The git-remote check (Non-Goal) would remove this
  gap later.
- **Extra source fetch per Kustomization** → one `Get`; the UI caches sources by
  name (the shared `flux` source is fetched once). Acceptable.
- **Revision format drift across Flux versions** → the shortener tolerates
  `@sha1:`, `@`, and `/` separators; comparison is exact-string on the full
  revision, unaffected by shortening.
- **Model growth** (two new node fields) → generic and small; other resolvers
  ignore them (empty), renderers omit when empty.

## Open Questions

- Should "behind" also show *how far* behind (commit distance)? Needs git;
  deferred with the remote check.
- Should freshness roll up to a parent (a Kustomization tree "mostly up to date")?
  Not now; per-node is enough for the stated use case.
