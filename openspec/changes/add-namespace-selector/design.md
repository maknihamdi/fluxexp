## Context

`Service.Roots(context, namespace)` already takes a namespace and defaults it to `flux`, but
`loadRoots()` in `app.js` never sends one, so the backend half of the scope exists and
nothing reaches it. Roots are Kustomizations only.

The block itself comes from the frontend. `renderDetail` catches the `/api/roots` failure,
empties the pane and shows a toast that disappears after 4 seconds; `tree.roots` stays
`null`, so `renderTree` shows "loading…" forever. On a deep link, the failure is swallowed
by `.catch(() => {})`.

The portal acts with the caller's kubeconfig locally. In-cluster it acts as the
ServiceAccount, whose wildcard read makes the problem moot, and the portal has no notion of
who is looking. So "the rights of the user" exist only in local mode.

The crux: the user this change serves is, almost by definition, one with namespaced rights,
and `list namespaces` may be refused to them. Autocompletion cannot be assumed to have data.
Nor can it mean "the namespaces I have rights on": Kubernetes authorizes a `list` as a whole
request and does not filter its result, so a caller allowed to list namespaces sees all of
them. Only asking namespace by namespace could tell them apart, and that cost was rejected.

## Goals / Non-Goals

**Goals:**

- A user without rights on `flux` reaches any namespace they can read.
- Suggestions cost one call and their absence blocks nothing.
- Moving between namespaces already visited costs no call.
- A namespace that cannot be used says so and stays recoverable, without a reload.
- The namespace is part of the address, so a link keeps working for the person it was sent to.

**Non-Goals:**

- Remembering the last namespace across visits. The URL covers a bookmark; a stored
  preference adds a second source of truth next to it.
- Selecting several namespaces, or an "all namespaces" view.
- Per-user identity in-cluster.
- A `flux-system` default. `flux` is the deployment convention here and stays the default,
  though it is not Flux's own default.
- Filtering suggestions to namespaces that are readable or hold Flux resources.
- Computing freshness for HelmRelease roots: it stays unset, as for any entry a resolver
  cannot compute it for.

## Decisions

### Discovery is one list of namespaces, unfiltered

`Service.Namespaces(context)` lists `v1 Namespace` and returns the sorted names. If that list
is refused it returns an empty list and no error. Any other failure is an error: the frontend
then has no suggestions and says nothing more, because free entry still works.

The cost is bounded by construction: one call per context, whatever the cluster's size.

**Alternative rejected — probing each namespace.** Suggestions would be exact, but at up to
2×N lists of full objects (`K8sGetter.List` has no `limit`) for a list that is only a hint.

**Alternative rejected — deriving namespaces from a cluster-wide list of Flux objects.** One
or two calls, and it would hide namespaces without Flux resources, but only for callers with
cluster-wide read, who are not the ones this change is for. Two sources for one list is not
worth it; a namespace without Flux resources says so when selected.

### Suggestions are advisory; selecting is what verifies

Every selection, suggested or typed, goes through `/api/roots`, and only its answer decides
what the home shows. This one path makes three things hold at once: free entry needs no
validation of its own, a suggestion that leads nowhere is stated at once, and
`/api/namespaces` needs no filtering.

### A namespace's listing is held in the page until re-read

Selecting a namespace reads it once. Its listing goes into `tree.listings` (namespace →
`{roots, at, unreadable}`), and selecting it again restores that entry without a call. This
extends what the roots home already does: `tree.roots` is held, shows its age, and is only
re-read by its control. The re-read control replaces the entry of the current namespace only.

The bounds, all following the existing rule "never show held material as current":

- a restored listing shows the age of its read, so it never reads as current;
- only a successful read is held. A refused namespace is retried on each selection, since
  the likely next step after a refusal is being granted the right;
- the branches open in the tree are not held per namespace: changing namespace discards
  them, because `tree.nodes` are what collapsing already discards, and the node pane always
  re-resolves regardless;
- changing context discards every listing, and so does a reload: the map lives in the page.

Holding the whole tree per namespace was rejected: it would add open and selected state per
namespace for a benefit nobody asked for.

### The selector is a native `<input list>` with a `<datalist>`

It gives autocompletion, free entry, keyboard handling and a label association with no
code. `change` fires on Enter, on blur after an edit, and on picking a suggestion, which is
exactly the commit point needed.

Browsers filter a datalist by the input's current value, so with `flux` in the field only
`flux` is offered, which hides the list from the very user who needs it. Focusing the input
therefore moves its value to the placeholder, and blurring restores it. An empty commit
keeps the current namespace. To be checked in a real browser: this is the one behaviour
tests cannot pin.

**Alternative rejected — a custom combobox.** The accessibility work (roles, active
descendant, keyboard) is real code for a list of plain strings. Worth revisiting only if
suggestions ever need to carry more than a name.

### `loadRoots` never throws; failure is state

On failure it stores the error in `tree.rootsError` and sets `tree.roots = []`. The tree and the home
then render a stated message from ordinary state, instead of relying on a toast. The empty
array is also what lets a deep link's spine render: that branch of `renderTree` sits after
the `tree.roots === null` return, which is why a forbidden deep link would otherwise show
"loading…" forever.

The server's message goes to the user as is. Kubernetes already says who was refused what,
so `/api/roots` keeps its status codes and no forbidden-specific branch is added.

A response that arrives after the context or namespace changed is discarded. With a free
text field this race becomes easy to hit, and `seq` guards only the detail pane.

### HelmReleases are roots; a kind unavailable is not a failure

`Roots` lists both kinds and returns Kustomizations first. Per kind:

- no such kind on the cluster (`meta.IsNoMatchError`) — skip it. A Flux install without
  helm-controller is valid and must not break the home;
- refused (`apierrors.IsForbidden`) — record it and continue;
- anything else — fail the call, as today.

If every kind present was refused, `Roots` returns that refusal, so the home shows an error
and not an empty namespace. If only some were, the response carries them in `unreadable`
and the home names them: showing half a namespace as if it were whole would break the
partial-failure invariant in the other direction.

A HelmRelease summary reuses `rootSummary`, whose health, interval and Ready message reading
is not Kustomization-specific. `rootSummary` takes the apiVersion and kind explicitly:
freshness and source lookup apply to Kustomizations only. A HelmRelease row is built from the
listed object alone: resolving it would gunzip a storage Secret for every row.

A side effect: a HelmRelease applied by a Kustomization in the same namespace now appears at
the root level too. Nested Kustomizations already behave that way, since roots are
"everything of the kind in the namespace".

### The namespace lives in the URL, omitted at the default

`?context=…&namespace=…&p=…`, written by `urlFor` and read by `readURL`. Omitted when it is
`flux`, so existing links keep their meaning and stay short. A trail's hops already name
their own namespaces, so this parameter only scopes the root level.

Changing namespace clears the branches and restores the held listing, if any, then
`navigateTo([])`. Changing context runs `resetTree()`, which now also empties `tree.listings`,
and resets the namespace to `flux`, since a namespace belongs to a cluster.

`popstate` applies the same two transitions when the context or namespace in the URL differs
from the state's. It does not today for context, so Back across a context switch shows the
previous cluster's roots. The namespace needs that comparison anyway, so both are covered.

## Risks / Trade-offs

- [Suggestions include namespaces the user cannot read or that hold no Flux resources] →
  Accepted, inherent to refusing per-namespace checks. Selecting one states it, and costs two
  lists.
- [A user refused `list namespaces` gets no autocompletion, and this is the likely case for
  the audience of this change] → Accepted. They type the name once; the URL then keeps it.
- [The namespace list goes stale for the life of the page] → It is a hint; a reload refreshes it.
- [A held listing goes stale] → It shows its age and has a re-read control.
- [`meta.IsNoMatchError` / `IsForbidden` must see through `k8s.Client.List`'s `%w` wrap; not
  verified against the pinned apimachinery v0.36] → Task 1.2 pins both with a wrapped error.
- [Clearing the field on focus surprises a user who wanted to edit one character] → Accepted:
  the current value stays visible as the placeholder and is restored on blur.
- [A typo reads as "no Flux resources in namespace X"] → Accepted. Kubernetes answers an empty
  list for a namespace that does not exist, so the portal cannot tell them apart without a
  further `get` that many users may not be allowed either.
