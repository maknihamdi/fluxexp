## MODIFIED Requirements

### Requirement: Root Kustomizations home

The portal's entry point SHALL be the root level of the tree: the root Flux Kustomizations
for the selected context (by default those in the `flux` namespace, with an option to widen
the scope), each shown with its health. Selecting one MUST open it in the node view, where
its useful information is shown: source reference, path, last applied revision, reconcile
interval, last transition time, and the Ready message.

#### Scenario: The root level lists the root Kustomizations with their health

- **WHEN** the user opens the portal for a context
- **THEN** the tree's root level shows the root Kustomizations, each with its health

#### Scenario: Selecting a root shows its information

- **WHEN** the user selects a root Kustomization
- **THEN** the node view shows its health and its source, path, applied revision, interval, last transition time and message

#### Scenario: Unhealthy root is visually distinct

- **WHEN** a root Kustomization is not Ready
- **THEN** it is rendered as unhealthy at the root level, and its failure message is visible when it is selected

### Requirement: Drill-in trail and new exploration

The portal SHALL support two navigation modes. In **drill-in** mode the user stays on the
current page: the path traversed is kept in the tree, where every level from the root down
to the selected node stays visible and selectable, so they can see and return to where they
came from. That path MUST be reflected in the page URL, so it survives a reload and can be
shared. Alternatively the user MAY **open a new exploration**, which starts a fresh page —
addressed by that resource's own exploration URL — and leaves the current one.

#### Scenario: Drill-in keeps the path visible

- **WHEN** the user drills from a root into a child and then a grandchild on the same page
- **THEN** the tree shows root → child → grandchild as a path, and selecting any of them returns to that level

#### Scenario: New exploration starts fresh

- **WHEN** the user chooses to open a new exploration from a resource
- **THEN** a new exploration page begins from that resource, separate from the current page's tree

#### Scenario: A new exploration is itself addressable

- **WHEN** a new exploration is opened from a resource
- **THEN** its page carries that resource's exploration URL, so it can be reloaded and shared like any other

### Requirement: Freshness on the roots home

The root level SHALL show each root Kustomization's freshness distinctly from its health,
and selecting a root SHALL show its freshness together with the short applied revision and
the synced time, so a user can tell whether it is up to date.

#### Scenario: Root shows a freshness badge and applied sha

- **WHEN** the user selects a root Kustomization that is up to date
- **THEN** the node view shows an up-to-date freshness badge alongside the health, plus the short applied revision and the synced time

#### Scenario: Behind root is visually distinct

- **WHEN** a root Kustomization is behind its source
- **THEN** its freshness reads behind and is visually distinct from up-to-date, and from its health

### Requirement: Expandable children are visually marked

The portal SHALL mark an expandable reference — in the tree and in the node view's list of
children — with an expansion control and an emphasis distinct from the health and freshness
marks, so a user can tell at a glance which references are worth opening. A non-expandable
reference MUST NOT carry the marker, and its expansion control MUST NOT be offered.

#### Scenario: Container child is marked

- **WHEN** a layer contains an expandable child
- **THEN** that child's row shows the expansion control and the emphasis

#### Scenario: Leaf child is not marked

- **WHEN** a layer contains a non-expandable child such as a ConfigMap
- **THEN** that child's row shows neither, and offers no way to expand it

### Requirement: Pending status is visually distinct

The portal SHALL render a **pending** reference with a health mark visually distinct from
healthy, unhealthy, unknown and error, wherever health appears — the tree, the node view's
header, its list of children, and a listed dependency. A pending reference MUST NOT read as
healthy, so a reader scanning a layer can tell an object that is still converging from one
that is working.

#### Scenario: A pending object is distinguishable in a layer

- **WHEN** a layer contains a pending object alongside healthy and unhealthy ones
- **THEN** the pending object's mark differs from both, and it does not read as healthy

#### Scenario: A pending root is distinguishable in the tree

- **WHEN** a root Kustomization's status is pending
- **THEN** its tree row carries the pending mark, and its detail message is shown when it is selected

### Requirement: Browser history navigates the exploration trail

The portal SHALL add a browser history entry for each navigation it performs: selecting a
row in the tree, selecting a child or a dependency in the node view, moving up to the
parent, switching context. Expanding or collapsing a branch is not a navigation and MUST
NOT add a history entry. Using the browser's Back and Forward controls MUST move along the
exploration trail and MUST NOT leave the portal, with the rendered view always matching the
URL being restored.

#### Scenario: Back returns to the previous node

- **WHEN** the user has drilled two levels deep and presses the browser's Back control
- **THEN** the portal renders the previous level, still inside the portal

#### Scenario: Forward returns to the node just left

- **WHEN** the user presses Back and then Forward
- **THEN** the portal renders the node they had left

#### Scenario: Expanding does not enter history

- **WHEN** the user expands three branches in the tree and then presses Back
- **THEN** the portal returns to the node visited before the current one, not to an earlier expansion state

#### Scenario: History and URL never disagree

- **WHEN** any history entry is restored
- **THEN** the rendered path and selected node are those encoded in that entry's URL

## ADDED Requirements

### Requirement: Health is encoded by shape as well as colour

Wherever the portal shows health, the mark SHALL differ in **shape** between the five values
as well as in colour, so the values remain distinguishable to a colour-blind reader and in a
greyscale rendering. The same mark MUST be used for a given health value everywhere it
appears.

#### Scenario: Health values differ without colour

- **WHEN** a layer mixing healthy, pending and unhealthy references is rendered without colour
- **THEN** the three remain distinguishable by the shape of their marks

#### Scenario: One mark per value across the portal

- **WHEN** the same reference appears in the tree, in a list of children and as a dependency
- **THEN** it carries the same health mark in all three places

### Requirement: Interaction colour is separate from health colour

The portal SHALL reserve its accent colour for interaction — selection, focus, hover, links
— and MUST NOT use the health palette for any of them, so a coloured element always means
what it appears to mean.

#### Scenario: A selected healthy row is not confused with a state

- **WHEN** the user selects a row
- **THEN** the selection is shown in the accent colour, distinct from any health colour on that row

#### Scenario: Focus is visible

- **WHEN** the user moves keyboard focus onto a row or a control
- **THEN** it carries a visible focus indicator in the accent colour