## Context

The portal is a single-page app (`internal/ui/web/app.js`) served by
`http.FileServer` at `/`, talking to three read-only endpoints. Exploration state
is `state.trail`, an array of ref DTOs; `render()` dispatches to the roots home
when it is empty and to the node view otherwise. `syncURL()` writes only
`context`, with `history.replaceState` — so the address bar never reflects where
the user is, and Back exits the app.

A ref is `{domain, type, namespace, name}` where a Kubernetes `type` is
`"<apiVersion>|<Kind>"` (`kustomize.toolkit.fluxcd.io/v1|Kustomization`). The
`display` label is not primary data: the server derives it from the type in
`friendlyLabel` (`internal/ui/dto.go`), and the same derivation works in the
browser.

## Goals / Non-Goals

**Goals:**
- Every explored node is addressable: its URL can be copied, bookmarked, shared,
  and reopened in another tab or by another person on the same cluster.
- Browser Back/Forward walk the exploration trail instead of leaving the app.
- An explicit control moves up one level from the current node.
- No server change: the API and the static-file serving stay as they are.

**Non-Goals:**
- Caching visited layers client-side. Back re-resolves.
- Persisting exploration across restarts (no localStorage, no server session).
- Deep-linking into a node's *scroll position* or an expanded sub-section.
- Encoding the display label in the URL.

## Decisions

### The trail goes in a query parameter, not a path

`/` is served by `http.FileServer`; a URL like `/explore/flux/apps` would return
404 on reload — the very failure being fixed — unless a catch-all handler
rewrites unknown paths to `index.html`. That is server machinery bought for
nothing, since a Kubernetes `type` already contains `/` (`…fluxcd.io/v1|Kind`)
and would have to be percent-encoded inside a path segment anyway. Depth is
unbounded (root → Kustomization → HelmRelease → Deployment → ReplicaSet → Pod);
four segments per hop produces a path no one reads.

A hash fragment (`#/…`) was also considered: it needs no server change either,
but it is invisible to any future server-side handling and reads as a legacy SPA
idiom. The query parameter keeps `context` exactly where it already is.

Chosen form: `/?context=<ctx>&p=<hop>~<hop>~…`

### Hop encoding: `type:ns:name`, or `domain:type:ns:name`

`domain` is omitted when it is `kubernetes`, which is every hop today; a future
`gcp` hop carries it explicitly. Splitting a hop on `:` yields 3 or 4 parts,
which disambiguates without a marker.

Each component is percent-encoded individually, so a name containing a separator
can never break parsing — Kubernetes names cannot today, but a future backend's
coordinates might. To keep the address bar readable, the encoder restores `/` and
`|` after encoding (both are legal in a query string); `decodeURIComponent`
accepts either form, so the decoder needs no special case. The result reads:

```
/?context=prod&p=kustomize.toolkit.fluxcd.io/v1|Kustomization:flux:apps~apps/v1|Deployment:team-a:web
```

The query string is assembled by hand rather than through `URLSearchParams`,
whose form-urlencoded serializer would escape `~` and `:` into `%7E` and `%3A`
and lose that readability.

### The URL is the single source of truth for the trail

`state.trail` becomes a projection of the URL rather than a parallel copy.
Navigation writes the URL with `pushState` and then renders from what was
written; `popstate` re-reads the URL and renders. One decoder is used by boot,
by `popstate`, and by every navigation, so a shared link and a Back press land in
exactly the same state.

Intermediate hops are never fetched: only the last hop is expanded, and the
breadcrumb labels come from each hop's type. Reopening a five-level URL therefore
costs one API call, consistent with the portal's one-fetch-per-hop rule.

### `display` stays out of the URL

It is derivable (`kind` = the part after the last `|`, then
`kind ns/name`). Keeping it out makes links shorter and prevents a shared link
from carrying a label that no longer matches the object.

### Up-to-parent pushes a new entry, it is not `history.back()`

Back and "up" are different operations: the previous history entry may be a
sibling the user jumped to, or another exploration entirely. The control
navigates to `trail.slice(0, -1)` and pushes it, so the trail stays a property of
the URL rather than of where the user happened to come from. On the roots home
there is no parent, so the control is not rendered.

### Context change resets the trail

A trail is a path through one cluster; carrying it into another context would
address objects that may not exist there. Switching context clears `p` and
returns to the roots home, pushing that as a history entry.

## Risks / Trade-offs

- **A malformed or hand-edited `p` cannot be parsed** → the app falls back to the
  roots home and surfaces a toast, rather than rendering a broken trail.
- **A shared link may point at an object that no longer exists** → the last hop
  resolves to an error node, which the portal already renders inline, with the
  breadcrumb intact so the user can step back up.
- **Back re-resolves instead of restoring instantly** → accepted: cached health
  is misleading health, and the server-side memo is deliberately scoped to a
  single request for the same reason. The cost is one hop's fetch.
- **Old "new exploration" links (`e_type`…) stop working once reloaded** → the
  portal is a local tool with no persisted links; nothing outside a currently
  open tab holds that form.
