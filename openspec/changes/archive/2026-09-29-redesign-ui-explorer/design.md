## Context

The portal today is a single column that renders exactly one layer: `renderHome` or
`renderExplore` clears `#view` and draws the node plus its children. The path is kept in
`state.trail`, projected from `?p=`, and displayed as a breadcrumb. Everything the user
opened before is gone from the screen.

Two constraints frame the redesign, both recorded in `CLAUDE.md`:

- **One hop per call.** `/api/expand` resolves a single reference: health, fields,
  dependencies and one level of children. The server memo lives for one request only, and
  the frontend deliberately caches nothing, "because caching a visited layer client-side
  would serve stale health".
- **The URL is the source of truth for where the user is.** `state.trail` is a projection
  of `?p=<hop>~<hop>…`, every navigation pushes a history entry, and `popstate` re-reads
  the URL.

A persistent tree is in direct tension with the first: it keeps several fetched layers on
screen at once. That tension is the substance of this design.

A clickable mockup of the target UI lives beside this document (`mockup.html`, published
at https://claude.ai/artifact/6jmSHin2eWn996HcS1vuEt). It uses a static in-file graph in
place of `/api/expand` and is the reference for layout, density and the visual language.

## Goals / Non-Goals

**Goals:**

- Keep the shape of the explored graph on screen: what contains what, and where the
  selected node sits within it.
- Never display a health value without saying how old it is.
- Preserve every existing URL: the same `?context=` and `?p=` addresses must open the same
  node after the redesign.
- Stay vanilla — no build step, no framework, no runtime dependency, still `//go:embed`.
- No server change: same routes, same DTOs, same one-hop resolution.

**Non-Goals:**

- Cluster-wide search. Filtering applies to what has been fetched; searching a cluster
  would need a new endpoint and a different cost model.
- Automatic polling or live updates. Re-reading stays an explicit act.
- Any write action. The portal stays read-only.
- Rendering the whole subtree at once. Expansion remains one hop per interaction.

## Decisions

### D1 — Two panes: a tree that holds branches, a detail pane that holds one node

The left pane holds every branch the user has opened, with a health mark on each row. The
right pane shows the selected node in full: header, fields, dependencies, and the list of
what it applies.

*Alternative considered: a single nested tree with inline detail.* Rejected — a
Kustomization's fields, message and dependency block are too tall to inline in a tree row
without the tree ceasing to be scannable; a 67-entry inventory would be unreadable.

*Alternative considered: keeping one column and adding a mini-map.* Rejected — it adds a
second representation of the same graph without removing the need to navigate the first.

### D2 — Expansion and selection are separate actions

The twisty expands a branch in place and leaves the detail pane alone. The row label
selects: it loads that node into the detail pane and reveals it in the tree. Both are
available on the same row, in the same place on every row.

This is what lets a user compare branches — open two subtrees, then inspect one node —
which the current UI cannot do at all.

### D3 — The URL keeps carrying the selection trail, and nothing else

`?p=` stays exactly as it is: the ordered hops from the roots home to the **selected**
node. Open branches are not in the address.

*Why:* the URL's job is "where the user is", and that is the selected node. Encoding a set
of open branches would make two users' links differ for the same node, and would grow
without bound. On opening a link, the tree shows the trail's hops expanded down to the
selected node — which costs nothing extra, since a trail's hops are already derivable from
the URL, and only the selected node is resolved (the existing "restoring a trail costs one
expansion" requirement is unchanged).

The URL/history half of `app.js` — `encodeHop`, `decodeHop`, `urlFor`, `rawParam`,
`readURL`, `navigateTo`, `popstate` — is kept as-is. The rewrite is confined to rendering.

### D4 — A loaded branch states when it was read; the detail pane always re-fetches

The no-cache rule exists so that nothing claims a health it can no longer vouch for. A
tree that holds branches cannot re-fetch everything on every interaction, so the rule is
restated rather than broken:

1. Selecting a node **always** calls `/api/expand` for it, even if its children are
   already in the tree. The detail pane is never rendered from held material.
2. A branch's rows in the tree keep the health from the call that loaded them, and the
   branch shows the age of that read.
3. A re-read control on a branch and on the detail header re-runs that one call.
4. Collapsing a branch discards what it held, so re-expanding re-fetches.

The invariant moves from "never hold a fetched layer" to "never show held material as
current". That is a genuine change in behaviour, and it is the price of a persistent tree.

*Alternative considered: re-fetching every open branch on each selection.* Rejected — a
user with six branches open would pay six resolves per click, and `/api/expand` on a
HelmRelease gunzips its Helm storage Secret.

### D5 — Filtering is client-side, over material already fetched, and keeps ancestors

The name filter and the health filter narrow the tree and the current layer. A branch is
kept when it matches **or when something under it does**, so filtering never hides the
path to a match. The layer header states how many of the total are shown.

No cluster call. Nothing is fetched to answer a filter, which keeps the feature honest:
it narrows what the user already pulled, and never implies cluster-wide coverage.

### D6 — Visual language: ruled rows, one card, health as shape and colour

- **Structure**: the card treatment (border, radius, fill) is reserved for the detail
  header — the one object on screen that is a thing rather than a list entry. Children are
  ruled rows in a table-like grid with aligned kind / name / detail columns, which is what
  makes 67 entries scannable. Today every block is a card, so nothing is emphasised.
- **Health** is a mark whose **shape** differs per value (disc, half-ring, triangle, hollow
  ring, diamond) as well as its colour, so the five values survive a colour-blind reader
  and a greyscale screenshot. The current five outlined pills differ only by hue and repeat
  the word on every row.
- **Freshness** stays a separate chip, as specified.
- **Palette**: cool slate neutrals; one steel-blue accent for interaction only; green /
  amber / red reserved for health, never for interaction. Tokens are defined on bare
  `:root` and redefined for dark under both `prefers-color-scheme` and `[data-theme]`.
- **Type**: the mockup pairs IBM Plex Sans with IBM Plex Mono (identifiers, revisions and
  counts are monospace, prose is not). **The implementation uses the system sans and
  `ui-monospace` stacks instead**: the portal is served by a local binary and must render
  with no network, and embedding four font files to gain a typeface is not a trade this
  tool needs. The pairing that matters — proportional for prose, monospace for
  identifiers — survives the substitution.

### D7 — Layout: a fixed shell, with the tree as a drawer under 760px

`html, body { height: 100% }` and a two-column grid; each pane scrolls independently, so
the header rail and the tree stay put while a long inventory scrolls. Below 760px the grid
becomes one column and the tree slides in as a drawer from a toggle in the rail, because a
tree pane and a detail pane cannot both be usable at 400px.

## Risks / Trade-offs

- **Held health goes stale silently** → every branch shows its read age, the detail pane
  always re-fetches, and collapsing discards. The user is never told a value is current
  when it is not.
- **Rewriting `app.js` regresses URL and history behaviour**, which has seven spec
  scenarios and no automated test → the URL/history functions are moved unchanged, not
  rewritten, and the change is verified by walking those scenarios by hand against
  `dev` (deep link, reload, Back/Forward, up, context switch, bad path).
- **Many open branches cost many resolves** → expansion stays one branch per user action;
  nothing expands on its own. A Kustomization with a 67-entry inventory loads exactly as it
  does today, through the existing bounded pool.
- **A deep tree indents off the pane** → indentation is capped and rows ellipsize; the
  kind is shown before the name so a truncated row still says what it is.
- **The frontend grows** (three files, more DOM construction) → `app.js` stays one file
  with no build step; the rendering half is replaced, not added to.

## Migration Plan

Not applicable: the portal is served from the binary and holds no persisted state. The
address format is unchanged, so links minted by the old UI open in the new one. Rollback is
`git revert` of the frontend files.

## Open Questions

- Should the tree pane's width be user-adjustable (a drag handle, persisted in
  `localStorage`)? Deferred: a fixed `minmax(240px, 22rem)` column is enough until it
  demonstrably is not.
- Should the roots home widen beyond namespace `flux` from the rail (the existing spec
  mentions "an option to widen the scope", never implemented)? Out of scope here; it is a
  data question, not a layout one.