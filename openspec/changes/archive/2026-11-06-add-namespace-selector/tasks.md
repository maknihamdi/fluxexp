## 1. Roots across both kinds

- [x] 1.1 Give `fakeCluster` per-key list errors (`kind|namespace`), so a test can refuse a list or report a kind as absent, always wrapped with `%w` as `k8s.Client.List` does
- [x] 1.2 Confirm `apierrors.IsForbidden` and `meta.IsNoMatchError` see through that wrap in the pinned apimachinery, with a test each. If either does not, unwrap in the one helper that classifies a list error
- [x] 1.3 Make `rootSummary` take the apiVersion and kind explicitly. Freshness and source apply to Kustomizations only; health, interval and Ready message stay shared
- [x] 1.4 `Roots` lists Kustomizations then HelmReleases (`helm.toolkit.fluxcd.io/v2`) and returns the kinds refused: absent kind skipped, refused kind recorded, any other error fails the call, every present kind refused returns that refusal
- [x] 1.5 `handleRoots` adds `unreadable` to the response. Update `TestRoots_ExtractsSummary` and `TestHandlers_RootsAndExpand` for the new signature
- [x] 1.6 Tests: HelmRelease-only namespace, no HelmRelease kind, one kind refused, both refused, and Kustomizations ordered first

## 2. Namespace discovery

- [x] 2.1 `Service.Namespaces(context)`: list `v1 Namespace`, return the sorted names. Refused: empty result, no error. Any other failure: error
- [x] 2.2 Route `GET /api/namespaces`, in `Handler`, returning `{"namespaces": [...]}`, never `null`
- [x] 2.3 Tests, one per scenario of `Namespace discovery`, including that exactly one list is issued

## 3. Selector and URL

- [x] 3.1 `index.html`: `<label>` + `<input id="namespace" class="field" list="namespaces">` + `<datalist id="namespaces">` inside a `.ctx` block right after the context selector. Check the rail's narrow layout in `styles.css` still fits
- [x] 3.2 `state.namespace`, defaulting to `flux`; `loadRoots` sends it. `urlFor` writes `namespace=` when not the default, and every caller passes it, `newExploration` included; `readURL` reads it, and falls back to `flux` when the parameter is absent
- [x] 3.3 `loadNamespaces()` fills the datalist in the background: not awaited at boot, called again on context change, and a failure leaves it empty with no toast
- [x] 3.4 `tree.listings`: namespace → `{roots, at, unreadable}`, written by `loadRoots` on success only, emptied by `resetTree()`. A helper switches namespace: it clears `tree.nodes`, `tree.open` and `tree.loading`, and restores the held entry or leaves `tree.roots` null so the read happens
- [x] 3.5 `change` on the input: an empty value keeps the current namespace; otherwise that helper then `navigateTo([])`. Focus moves the value to the placeholder, blur restores it
- [x] 3.6 Context `onchange` resets the namespace to `flux` and reloads the suggestions
- [x] 3.7 `popstate`: a different context in the URL runs `resetTree()`, a different namespace runs the helper, both before rendering

## 4. Stated failure

- [x] 4.1 `loadRoots` stops throwing: it stores `tree.rootsError`, sets `tree.roots = []`, clears the error on success, and drops an answer whose context or namespace is no longer current. `refreshRoots` and `renderDetail` lose their now-dead failure paths
- [x] 4.2 Home and tree show the persistent refusal with the namespace name; an empty namespace reads "No Flux resources in namespace '<ns>' for this context."; `unreadable` kinds are named on the home
- [x] 4.3 `rootRow` shows `kindOf(r.ref)` in place of the hardcoded `Kustomization`; the spinner, reread titles and "root kustomizations" headings become "roots"

## 5. Verify

- [x] 5.1 `make test`, `make vet`, `gofmt -l .`
- [x] 5.2 In a browser, against a cluster or the UI fake: from `flux`, focusing the selector offers every suggestion; typing narrows them; Enter, picking and blur each commit once; leaving it empty restores the namespace; returning to a namespace issues no `/api/roots` request (network tab) and shows the age of its read
- [x] 5.3 With a kubeconfig limited to one namespace (a ServiceAccount token with a namespaced `Role`, no rights on `flux`): the home states the refusal, typing the namespace lists its roots, a HelmRelease opens, Back and reload behave
- [x] 5.4 Same check on a role that allows `list namespaces`: suggestions show every namespace, and selecting an unreadable one states the refusal

## 6. Docs

- [x] 6.1 `CLAUDE.md`: the `internal/ui` section (namespace in the URL, `/api/namespaces`, held listings per namespace, `loadRoots` never throwing) and the "Root" wording
- [x] 6.2 `README.md`: the portal's description and roadmap entry
