## Why

The portal's home is hardwired to the `flux` namespace. A user who cannot read that
namespace gets a 4-second toast, a blank node pane and a tree stuck on "loading…", with no
way forward: the rest of the tool is unreachable even when they hold rights on the
namespaces of their own team.

## What Changes

- A namespace selector in the rail, right after the context selector, with autocompletion.
  It defaults to `flux` and accepts any namespace, typed or picked.
- A new endpoint, `GET /api/namespaces`, returning the cluster's namespaces when the
  credentials may list them, and nothing otherwise. No namespace is probed: suggestions are
  not filtered, and typing a name always stays possible.
- Each selection lists that namespace. The listing is held in the page with its age, and
  selecting the namespace again shows it without a call until the user re-reads it.
- The roots home lists **Kustomizations and HelmReleases** of the selected namespace,
  because a team namespace often holds only HelmReleases.
- A namespace that cannot be read, or holds no Flux resource, is stated in place and
  persistently. The selector stays usable, so the user is never stuck.
- The selected namespace is part of the URL when it is not the default; changing it clears
  the path and the tree, as changing context does.
- **BREAKING** (`GET /api/roots`): the response may carry HelmRelease entries, and a home
  in `flux` now also lists the HelmReleases of that namespace.

## Capabilities

### New Capabilities

- `namespace-selection`: choosing the namespace the portal starts from — default, discovery
  of the suggestions, free entry, held listings, and what an unusable namespace shows.

### Modified Capabilities

- `web-ui`: the roots home is scoped by the selected namespace and lists HelmReleases as
  well as Kustomizations; the exploration URL carries the namespace.

## Impact

- **Backend**: `internal/ui/service.go` (`Roots`, new `Namespaces`, HelmRelease summary),
  `internal/ui/server.go` (new route, `unreadable` in the roots response), tests.
- **Frontend**: `internal/ui/web/index.html`, `app.js`, `styles.css`.
- **API**: `/api/roots` gains an `unreadable` field and honours the `namespace` parameter
  the frontend never sent; `/api/namespaces` is new.
- **Cluster load**: one `Namespace` list per context, in the background of the home; two
  lists per namespace actually selected, and none for one already held.
- **Unchanged**: `deploy/rbac.yaml` (the wildcard read already covers namespaces), the CLI,
  the resolvers. In-cluster, the ServiceAccount reads everything, so the selector is a
  convenience there and not an access mechanism.
