# web-ui Specification

## Purpose
TBD - created by archiving change add-web-ui. Update Purpose after archive.
## Requirements
### Requirement: Local no-auth portal

The system SHALL serve a web portal from a local HTTP server that requires no
authentication and acts with the local kubeconfig's credentials. The server MUST
bind to a loopback address by default so the unauthenticated API is not reachable
off-host. It MUST be read-only: no request may mutate cluster state.

#### Scenario: Server binds to loopback by default

- **WHEN** the portal server starts without an explicit bind address
- **THEN** it listens on a loopback address (e.g. `127.0.0.1`) and serves the portal there

#### Scenario: Portal reads local state without credentials prompt

- **WHEN** a user opens the portal
- **THEN** it uses the local kubeconfig to talk to the cluster, with no login step

### Requirement: Context listing and selection

The portal SHALL list the kube contexts from the local kubeconfig, indicate the
current context, and let the user select which context subsequent requests
target. All data shown MUST reflect the selected context.

#### Scenario: Contexts are listed with the current one marked

- **WHEN** the portal loads
- **THEN** it shows the kubeconfig's contexts and marks the current-context

#### Scenario: Switching context changes the data source

- **WHEN** the user selects a different context
- **THEN** subsequent listings and expansions target the newly selected context

### Requirement: Root Kustomizations home

The portal's home SHALL list the root Flux Kustomizations for the selected
context (by default those in the `flux` namespace, with an option to widen the
scope). Each entry MUST show its health and useful information: source reference,
path, last applied revision, reconcile interval, last transition time, and the
Ready message.

#### Scenario: Home lists root Kustomizations with state and info

- **WHEN** the user opens the home for a context
- **THEN** each root Kustomization is shown with its health and its source, path, applied revision, interval, last transition time and message

#### Scenario: Unhealthy root is visually distinct

- **WHEN** a root Kustomization is not Ready
- **THEN** it is rendered as unhealthy with its failure message visible

### Requirement: Layer-by-layer exploration

From any resource the portal SHALL let the user expand **one layer at a time**:
selecting a resource reveals its immediate children (each with health), and any
child that itself has children can be expanded in turn. Expansion MUST resolve a
single hop per interaction (not the whole subtree at once), reusing the same
resolvers as the CLI. A resource that could not be resolved MUST be shown as an
error node with its reason.

#### Scenario: Expanding a Kustomization reveals its inventory children

- **WHEN** the user expands a Kustomization
- **THEN** the portal shows that Kustomization's immediate children (its inventory objects) with health, without expanding deeper levels

#### Scenario: Expanding a HelmRelease reveals its deployed objects

- **WHEN** the user expands a HelmRelease
- **THEN** the portal shows the objects the release deployed, with health

#### Scenario: Unresolvable node shown as error

- **WHEN** expanding a reference fails to resolve
- **THEN** that node is shown as an error with its reason, and sibling expansion still works

### Requirement: Drill-in trail and new exploration

The portal SHALL support two navigation modes. In **drill-in** mode the user
stays on the current page and the path traversed is kept as a breadcrumb trail
so they can see and return to where they came from. Alternatively the user MAY
**open a new exploration**, which starts a fresh page and leaves the current one.

#### Scenario: Drill-in keeps a breadcrumb trail

- **WHEN** the user drills from a root into a child and then a grandchild on the same page
- **THEN** a breadcrumb shows the path root → child → grandchild and lets the user navigate back to any prior level

#### Scenario: New exploration starts fresh

- **WHEN** the user chooses to open a new exploration from a resource
- **THEN** a new exploration page begins from that resource, separate from the current page's trail

### Requirement: Context-change indication

The portal MUST indicate when following a hop would require a different kube
context than the one being explored, rather than silently crossing contexts.

#### Scenario: Cross-context hop is flagged

- **WHEN** a resource references an object that belongs to a different kube context
- **THEN** the portal indicates that continuing requires switching context, instead of resolving it silently under the current context

### Requirement: Freshness on the roots home

The roots home SHALL show each root Kustomization's freshness as a badge distinct
from its health badge, together with the short applied revision and the synced
time, so a user can tell at a glance whether it is up to date.

#### Scenario: Root shows a freshness badge and applied sha

- **WHEN** the roots home lists a Kustomization that is up to date
- **THEN** it shows an up-to-date freshness badge alongside the health badge, plus the short applied revision and the synced time

#### Scenario: Behind root is visually distinct

- **WHEN** a root Kustomization is behind its source
- **THEN** its freshness badge reads behind and is visually distinct from up-to-date

### Requirement: Freshness and fields in the node view

When a node exposes freshness and fields, the node view SHALL render a freshness
badge next to the health badge and a panel listing the fields (label and value),
with the full revision available even when a short form is shown.

#### Scenario: Node view shows freshness and fields

- **WHEN** the user opens a Kustomization in the portal
- **THEN** the node view shows its freshness badge and a fields panel with the applied revision, synced time and source state

#### Scenario: Full revision available

- **WHEN** a field shows a shortened revision
- **THEN** the full revision is available to the user (e.g. via the field value / title)

