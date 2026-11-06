## RENAMED Requirements

- FROM: `### Requirement: Root Kustomizations home`
- TO: `### Requirement: Roots home`

## MODIFIED Requirements

### Requirement: Roots home

The portal's entry point SHALL be the root level of the tree: the Flux Kustomizations and
HelmReleases of the selected namespace for the selected context, Kustomizations first, each
shown with its kind and health. Selecting one MUST open it in the node view, where its
useful information is shown: for a Kustomization its source reference, path, last applied
revision, reconcile interval, last transition time and Ready message; for a HelmRelease its
reconcile interval, last transition time and Ready message.

A kind absent from the cluster MUST be skipped silently. A kind that cannot be read while
the other can MUST be named on the home, so a partial listing does not read as complete.

#### Scenario: The root level lists the roots with their health

- **WHEN** the user opens the portal for a context
- **THEN** the tree's root level shows the Kustomizations and HelmReleases of the selected namespace, each with its health

#### Scenario: A namespace holding only HelmReleases

- **WHEN** the selected namespace holds HelmReleases and no Kustomization
- **THEN** the root level lists the HelmReleases, and selecting one opens it in the node view

#### Scenario: Cluster without helm-controller

- **WHEN** the HelmRelease kind does not exist on the cluster
- **THEN** the root level lists the Kustomizations and reports no problem

#### Scenario: One kind is refused

- **WHEN** reading Kustomizations in the namespace is refused and reading HelmReleases is not
- **THEN** the root level lists the HelmReleases and the home states that Kustomizations could not be read

#### Scenario: Selecting a root shows its information

- **WHEN** the user selects a root Kustomization
- **THEN** the node view shows its health and its source, path, applied revision, interval, last transition time and message

#### Scenario: Unhealthy root is visually distinct

- **WHEN** a root is not Ready
- **THEN** it is rendered as unhealthy at the root level, and its failure message is visible when it is selected

### Requirement: Exploration state is addressable by URL

The portal SHALL encode the current exploration path in the page URL, so that
every node reached by drilling has an address that can be copied, bookmarked and
shared. The URL MUST carry the selected kube context, the selected namespace when it
is not the default, and the ordered sequence of
hops from the roots home to the current node, each hop identified by enough
information to resolve it (its domain when not the default, its type, its
namespace and its name).

Opening such a URL — by reload, by pasting it in another tab, or from someone
else's message — MUST restore the same view: the current node rendered and the
full breadcrumb trail available. Restoring a trail MUST NOT resolve the
intermediate hops; only the current node is expanded.

The display label MUST NOT be carried in the URL: it is derived from the hop's
type, so a shared link cannot show a stale label.

#### Scenario: Drilling changes the URL

- **WHEN** the user drills from the roots home into a Kustomization and then into one of its children
- **THEN** the URL reflects the two-hop path, alongside the selected context

#### Scenario: Reloading a deep URL restores the view

- **WHEN** the user reloads the page on a deep exploration URL
- **THEN** the portal renders the same current node with its full breadcrumb, instead of returning to the roots home

#### Scenario: A shared URL opens on the same node

- **WHEN** a user opens an exploration URL produced by another session against the same cluster
- **THEN** the portal opens on that node with its breadcrumb, without the sender having to describe the path

#### Scenario: Restoring a trail costs one expansion

- **WHEN** a five-hop exploration URL is opened
- **THEN** only the last hop is expanded, the earlier hops being shown in the breadcrumb from their own identifiers

#### Scenario: The roots home has its own address

- **WHEN** the user is on the roots home
- **THEN** the URL carries the context and no exploration path, so reloading returns to the roots home

#### Scenario: The namespace is carried unless it is the default

- **WHEN** the user selects `team-a`
- **THEN** the URL carries `namespace=team-a`, and reloading it shows `team-a` in the selector with its roots

#### Scenario: The default namespace is not written

- **WHEN** the selected namespace is `flux`
- **THEN** the URL names no namespace, so addresses produced before this change keep their meaning

#### Scenario: Back across a namespace change

- **WHEN** the user changes namespace and presses Back
- **THEN** the selector and the root level show the previous namespace, from its held listing with the age of that read

#### Scenario: An unparseable path falls back to the home

- **WHEN** the page is opened with an exploration path that cannot be decoded
- **THEN** the portal shows the roots home and reports the problem, rather than rendering a broken trail

#### Scenario: Switching context clears the path

- **WHEN** the user selects a different kube context while exploring
- **THEN** the portal returns to the roots home for that context and the URL no longer carries the previous trail