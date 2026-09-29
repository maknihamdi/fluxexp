## 1. Visual language

- [x] 1.1 Rewrite `styles.css` tokens: cool slate neutrals, one steel-blue accent, the health palette kept separate — every token on bare `:root`, redefined under `@media (prefers-color-scheme: dark) :root:not([data-theme="light"])` and `:root[data-theme="dark"]` → verify: the portal is legible in both OS themes with no literal colour left in a component rule (`grep -nE '#[0-9a-fA-F]{3,6}' styles.css` returns only the token blocks)
- [x] 1.2 Replace the five outlined health pills with marks that differ by shape (disc, half-ring, triangle, hollow ring, diamond) as well as colour → verify: a greyscale screenshot of a mixed layer still distinguishes healthy / pending / unhealthy
- [x] 1.3 Restyle children as ruled rows in an aligned kind / name / detail grid, keeping the card treatment for the node header only → verify: a 40+ entry layer scans as a table, column edges aligned
- [x] 1.4 Keep freshness as its own chip, distinct from health, with the system sans and `ui-monospace` stacks (identifiers monospace, prose proportional) → verify: no webfont is requested (the portal renders identically with the network off)

## 2. Shell

- [x] 2.1 Rewrite `index.html` as the shell: header rail (brand, context, filter), tree pane, node pane; `html, body { height: 100% }`, each pane scrolling on its own → verify: with a long inventory open, the rail and the tree stay put
- [x] 2.2 Collapse to one column below 760px with the tree as a drawer opened from the rail → verify: at 400px the node view fills the width, the drawer opens and closes, and the page never scrolls sideways
- [ ] 2.3 Give every row and control a visible focus state and keyboard operation, and respect `prefers-reduced-motion` → verify: the whole shell is usable with Tab / Enter alone

## 3. Tree pane

- [x] 3.1 Hold the tree as a map of reference key → `{ node, loadedAt }` plus the set of open keys; render rows with kind, namespace/name, health mark, expansion control and child count → verify: opening a Kustomization then a child HelmRelease leaves both branches on screen
- [x] 3.2 Split the two actions: the expansion control resolves that one reference and reveals its children without touching the node pane; the row label selects → verify: expanding a sibling while a node is open leaves the node pane unchanged
- [x] 3.3 Mark the selected row as current, and reveal its branch when selection comes from elsewhere (a child row, a dependency, Back/Forward) → verify: after pressing Back, the tree marks the restored node
- [x] 3.4 Discard a branch's held material on collapse → verify: collapsing and re-expanding issues a new `/api/expand` call (visible in the network panel)
- [x] 3.5 Expand the URL trail's hops down to the selected node on arrival, without resolving them → verify: opening a five-hop URL issues exactly one `/api/expand` call and shows the five hops in the tree

## 4. Node pane

- [x] 4.1 Rebuild the node header: kind, namespace/name, health, freshness, message, fields — always from a fresh resolve of the selected reference, never from what the tree holds → verify: selecting an already-expanded branch issues a new call
- [x] 4.2 Keep dependencies inside the header block, above the children list, each selectable → verify: a Kustomization's GitRepository shows in its header and not among its children
- [x] 4.3 Render the children list with `dropNestedDuplicates`, nested dependencies and inline Flux fields preserved from the current behaviour → verify: the existing web-ui scenarios for one-reference-per-layer and inline fields still hold
- [x] 4.4 Add the layer summary: `applies · N`, per-health tally and a proportional bar → verify: the tally sums to the child count

## 5. Read age and re-read

- [x] 5.1 Record the read time of every resolved branch and of the selected node; display it as an age on the node header and on each open branch → verify: a branch open for two minutes says so
- [x] 5.2 Add a re-read control on the node header and on each open branch, resolving that one reference → verify: re-reading one branch issues exactly one call and leaves the other open branches untouched

## 6. Filter

- [x] 6.1 Add the name filter over kind / namespace / name, case-insensitive, applied to the tree and the children list at once → verify: typing `cert` narrows both panes
- [x] 6.2 Add the health filter, combining with the name filter → verify: excluding `healthy` on a 67-entry layer leaves the failures
- [x] 6.3 Keep a branch whose descendant matches, so the path to a match is never hidden → verify: filtering on a Pod name keeps its Kustomization and Deployment visible
- [x] 6.4 State the filtered count in the layer header, and distinguish "nothing matches the filter" from "applies nothing" → verify: both messages appear in their own case, and no call is made while filtering

## 7. Verification

- [x] 7. Move `encodeHop`, `decodeHop`, `urlFor`, `rawParam`, `readURL`, `navigateTo`, `goUp`, `newExploration` and the `popstate` handler unchanged; add a history entry for every navigation and none for an expansion → verify: expanding three branches then pressing Back returns to the previously selected node
- [ ] 7.2 Walk the seven URL and history scenarios of `web-ui` by hand against `dev`: deep link, reload, Back, Forward, up-to-parent, context switch, unparseable path → verify: each renders what its scenario states
- [x] 7. Run `make vet` and `make test` → verify: both pass (the Go side is untouched, so any failure is a regression)
- [x] 7. Smoke-test against `dev` and a cluster with suspended Kustomizations (`dev-alt` or `staging`): open a large inventory, filter it, expand three branches, re-read one → verify: health and freshness match `flux get`, and the call count per action is what the specs state (assert the payload, not just that a request returned — an expired gcloud token fails fast and looks like a win)
- [ ] 7.5 Delete `mockup.html` from the change directory once the implementation matches it → verify: nothing in the repo references it