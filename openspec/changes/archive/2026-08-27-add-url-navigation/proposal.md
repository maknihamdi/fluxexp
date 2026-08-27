## Why

The portal's exploration path lives only in memory: `state.trail` is a JavaScript
array, and the URL never changes while the user drills down. So a view cannot be
shared or bookmarked, a reload drops the user back on the roots home, and the
browser's Back button leaves the application entirely instead of stepping back up
the trail. The only way back is to re-click in the breadcrumb — and only within
the same page session.

## What Changes

- The full exploration path is encoded in the URL as a query parameter, so every
  node reached by drilling is addressable: `/?context=<ctx>&p=<hop>~<hop>~…`.
- Opening or reloading a deep URL rebuilds the whole trail — breadcrumb included —
  and renders the last hop, whether the URL was pasted, bookmarked or shared.
- Every navigation (drilling into a row, jumping in the breadcrumb, going up)
  pushes a history entry, and a `popstate` handler rebuilds the state from the
  URL, so browser Back/Forward walk the exploration trail.
- The breadcrumb bar gains an **up-to-parent** control, absent on the roots home.
- Switching kube context resets the trail and updates the URL.
- **BREAKING** (URL surface only): the "new exploration" link emits the new `p=`
  format; the `e_domain`/`e_type`/`e_ns`/`e_name`/`e_display` seed parameters are
  replaced. Previously opened tabs keep working until reloaded; there is no
  persisted state to migrate.

Deliberately excluded: any client-side cache of already-visited layers. Going back
re-resolves, because serving cached health would contradict the reason the
server-side memo lives for a single request — never showing stale health.

## Capabilities

### New Capabilities
<!-- none: this changes how the existing portal navigates, not what it explores -->

### Modified Capabilities
- `web-ui`: the drill-in trail requirement gains URL addressability, browser
  history integration and an up-to-parent control; the "new exploration" mode is
  restated in terms of the new URL form.

## Impact

- `internal/ui/web/app.js` — trail encoding/decoding, `pushState`/`popstate`
  wiring, breadcrumb and parent control, `newExploration`.
- `internal/ui/web/index.html`, `internal/ui/web/styles.css` — the parent control.
- No server-side change: the API stays `/api/contexts`, `/api/roots`,
  `/api/expand`, and `/` keeps being served by `http.FileServer` — which is
  precisely why the path is carried in a query parameter rather than a sub-path.
