## Why

The portal shows **one layer at a time**: opening a child replaces the whole view, so the
shape of the graph — what contains what, how deep the current node sits, which sibling
branch is failing — exists only in the breadcrumb, one line of text. Exploring a Flux
tree is a back-and-forth between levels, and the current UI makes the user rebuild that
mental picture on every hop. The visual language has the same flatness: every block is a
card with the same radius, border and shadow, so a node header, a child row and a
dependency all carry equal weight.

This change gives the portal the shape the domain already has — a tree — and a visual
direction with an actual hierarchy.

## What Changes

- **A two-pane shell replaces the single-column view.** A persistent tree on the left
  holds the branches the user has opened, with health on every node; a detail pane on the
  right shows the selected node — its header, its dependencies, what it applies.
- **The tree keeps several layers on screen at once.** Expanding a branch in the tree
  reveals its children in place and does not change the detail pane; selecting a node
  loads it in the detail pane and reveals it in the tree.
- **Freshness is explicit about age.** Because a layer now stays on screen after it was
  fetched, every loaded branch carries the time it was read and can be re-read on demand;
  nothing on screen silently claims a health it can no longer vouch for.
- **A filter narrows what is on screen** — by name fragment and by health — across the
  loaded tree and the current layer, without a new cluster call.
- **The visual language is rebuilt**: one type scale, a deliberate neutral, health encoded
  as a coloured stripe and dot rather than five outlined pills, and card styling reserved
  for the detail header instead of applied to every block.
- **The URL still addresses the selected node** exactly as today (`context` + `p=` trail),
  so every existing link keeps working; the tree's open branches are derived from the
  trail and from what the user opened, and are not part of the address.
- **No new server endpoint, no new dependency.** The shell resolves through the same
  `/api/expand`, one hop per call, and the frontend stays vanilla HTML/CSS/JS embedded by
  `//go:embed`.

## Capabilities

### New Capabilities

- `ui-tree-navigation`: the persistent tree pane — expansion independent of selection,
  multiple layers on screen, per-branch load state, staleness and refresh, and how the
  tree relates to the URL trail.
- `ui-layer-filter`: narrowing what is displayed by name fragment and by health, over
  material already fetched, with no additional cluster call.

### Modified Capabilities

- `web-ui`: the drill-in trail requirement becomes the tree + detail shell (the breadcrumb
  is no longer the only record of the path); the roots home becomes the tree's root level;
  the visual requirements for expandable children, pending status and freshness badges are
  restated against the new visual language.

## Impact

- `internal/ui/web/index.html`, `styles.css`, `app.js` — rewritten. `app.js` is where the
  work concentrates: the rendering half is replaced, the URL/history half is kept.
- `internal/ui/service.go`, `dto.go`, `server.go` — unchanged. No API change, no new route.
- `internal/resolver`, `internal/engine`, `internal/k8s`, the CLI — untouched.
- `openspec/specs/web-ui/spec.md` — several requirements restated.
- Deliberately out of scope: cluster-wide search (would need a new endpoint), live polling,
  and any write action (the portal stays read-only).