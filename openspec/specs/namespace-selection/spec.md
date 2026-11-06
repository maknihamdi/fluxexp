# namespace-selection Specification

## Purpose
How the portal chooses the namespace it starts from: the default, the suggestions, free entry, the listings held per namespace, and what a namespace that cannot be used shows.
## Requirements
### Requirement: Namespace selector

The portal SHALL offer a labelled namespace selector in the rail, immediately after the
context selector. It MUST default to `flux`, MUST offer autocompletion from the suggested
namespaces, and MUST accept a namespace typed by hand whether or not it is suggested.
Committing a namespace MUST show the roots of that namespace.

#### Scenario: The default namespace

- **WHEN** the portal opens on a URL that names no namespace
- **THEN** the selector shows `flux` and the roots home lists that namespace

#### Scenario: Picking a suggestion

- **WHEN** the user focuses the selector and picks a suggested namespace
- **THEN** the roots home lists that namespace

#### Scenario: The suggestions are reachable from the default

- **WHEN** the user focuses the selector while it shows `flux`
- **THEN** every suggested namespace is offered, not only those matching `flux`

#### Scenario: Autocompletion narrows the suggestions

- **WHEN** the user types part of a namespace name
- **THEN** only the suggestions matching what was typed are offered

#### Scenario: Typing a namespace that is not suggested

- **WHEN** the user types a namespace absent from the suggestions and commits it
- **THEN** the roots home lists that namespace

#### Scenario: Leaving the selector without choosing

- **WHEN** the user focuses the selector and leaves it without entering a namespace
- **THEN** the selector shows the current namespace again and the view is unchanged

### Requirement: Selecting a namespace starts a new exploration

Committing a different namespace MUST clear the exploration path and every branch held in
the tree, as switching context does. Switching context MUST reset the namespace to `flux`
and refresh the suggestions, since a namespace belongs to a cluster.

#### Scenario: Changing namespace clears the path

- **WHEN** the user commits another namespace while exploring
- **THEN** the portal shows the roots home of that namespace and no branch opened in the previous one stays in the tree

#### Scenario: Switching context resets the namespace

- **WHEN** the user selects another kube context
- **THEN** the selector shows `flux` and the suggestions are those of the new context

#### Scenario: A late answer is ignored

- **WHEN** the user commits a namespace while the roots of the previous one are still being read
- **THEN** the roots of the previous one are never displayed

### Requirement: Namespace discovery

`GET /api/namespaces?context=<name>` SHALL return, sorted, the namespaces of the cluster
when the current credentials may list them. It MUST issue a single list and MUST NOT query
any namespace individually, so a suggestion carries no promise that the namespace is readable
or holds Flux resources. When listing namespaces is refused it MUST return an empty list and
no error. Discovery MUST NOT delay the roots home.

#### Scenario: Namespaces can be listed

- **WHEN** the credentials may list namespaces
- **THEN** every namespace of the cluster is returned, and no namespace is queried individually

#### Scenario: Listing namespaces is refused

- **WHEN** listing namespaces is refused
- **THEN** an empty list is returned, and the selector still accepts a typed namespace

#### Scenario: Discovery fails

- **WHEN** discovery fails for a reason other than a refusal
- **THEN** the selector offers no suggestions and still accepts a typed namespace

### Requirement: A namespace's listing is held until re-read

Selecting a namespace SHALL list it once. The portal MUST hold that listing in the page and
show it again, with the age of its read, when the namespace is selected again, without
issuing a call. The re-read control MUST replace the held listing of the current namespace
only. A failed read MUST NOT be held. Switching context or reloading the page MUST discard
every held listing.

#### Scenario: Selecting a namespace lists it

- **WHEN** the user commits a namespace for the first time
- **THEN** its roots are read from the cluster

#### Scenario: Returning to a namespace costs no call

- **WHEN** the user commits a namespace already read in this page
- **THEN** its roots are shown with the age of that read and nothing is requested from the cluster

#### Scenario: Re-reading refreshes one namespace

- **WHEN** the user activates the re-read control on the roots
- **THEN** that namespace is read again, its age resets, and the listings held for other namespaces are untouched

#### Scenario: A refusal is not held

- **WHEN** the user commits a namespace whose read was refused earlier
- **THEN** the namespace is read again

#### Scenario: Switching context discards the listings

- **WHEN** the user switches context and comes back
- **THEN** the namespaces are read again

### Requirement: An unusable namespace is stated and recoverable

When the selected namespace cannot be read, or holds no Flux resource, the portal SHALL say
so persistently in the node pane and in the tree, naming the namespace, instead of leaving
an empty view. The selector MUST stay usable. A failure to read roots MUST NOT prevent an
exploration URL from rendering its node.

#### Scenario: Default namespace refused

- **WHEN** the portal opens and reading `flux` is refused
- **THEN** the page shows that `flux` cannot be read with the reason given by the cluster, and the message stays until another namespace is selected

#### Scenario: The user moves on from a refusal

- **WHEN** the user then commits a namespace they can read
- **THEN** its roots are listed without a reload

#### Scenario: No Flux resources

- **WHEN** the selected namespace holds neither a Kustomization nor a HelmRelease
- **THEN** the page states that the namespace holds no Flux resources for this context

#### Scenario: Deep link in a namespace whose roots are refused

- **WHEN** an exploration URL is opened and reading the roots of its namespace is refused
- **THEN** the selected node renders with its path, and the root level states the refusal

