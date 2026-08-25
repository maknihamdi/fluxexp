## Why

When someone pushes a new commit to a Git source, they need to see quickly
whether Flux has actually applied it. Today `fluxexp` shows a Kustomization's
health (Ready) but not whether its applied revision matches the source's — so a
"healthy" Kustomization that is still one commit behind looks fine. This
increment adds a **freshness** signal (distinct from health) plus the concrete
commit ids and sync times, so a user can tell at a glance if a Kustomization is
up to date, and confirm their pushed commit by its sha.

## What Changes

- A second, health-independent status — **freshness** — computed for
  Kustomizations from cluster state only (no git access):
  - **up-to-date**: Ready and `status.lastAppliedRevision` ==
    the source's `status.artifact.revision`.
  - **behind**: the source has a newer revision than what was applied.
  - **failed**: Ready is False (the latest apply failed).
  - **suspended**: `spec.suspend` is true.
- The Kustomization resolver fetches its **source** (the `spec.sourceRef` object,
  e.g. GitRepository) to read the fetched revision and time, and surfaces
  structured **fields**: applied revision (short + full), source revision when
  behind, synced time (relative + absolute), source fetched time, and — on
  failure — the attempted revision and message.
- The engine's node model gains two generic carriers so this stays domain-blind:
  a secondary `Freshness` status and an ordered list of `Fields{Label,Value}`.
  The engine computes neither; resolvers set them, exactly as they already set
  Health.
- The CLI tree shows freshness next to health and folds the key fields into the
  node detail; the web UI shows a freshness badge and a fields panel, and the
  roots home shows each Kustomization's freshness badge + short applied sha +
  synced time for a fast glance.

Out of scope: comparing the source's fetched revision to the real git remote
HEAD (would need `git ls-remote` / creds) — a possible later opt-in; HelmRelease
freshness (chart version) — a follow-up.

## Capabilities

### New Capabilities

- `reconciliation-freshness`: the freshness status model (generic carriers on
  the node) and the Kustomization freshness computation from applied vs source
  revision.

### Modified Capabilities

- `cli`: the traverse tree renders freshness and the key revision/sync fields.
- `web-ui`: the node view and the roots home render freshness and fields.

## Impact

- `internal/engine`: add `Freshness` (enum) and `Field{Label,Value}` to `Result`
  and `Node`; the engine copies them through (no logic).
- `internal/resolver`: the Kustomization resolver fetches the source and computes
  freshness/fields via a shared `KustomizationFreshness(...)` helper (also used
  by the UI roots summary); a small revision-shortening + relative-time helper.
- `internal/render`: tree shows freshness + fields.
- `internal/ui`: `RootDTO`/`NodeDTO` gain freshness + fields; `Roots` fetches
  sources (cached) to compute freshness; frontend renders badges + a fields
  panel.
- Reuses the existing single `List`/`Get`; no new external dependency.
