# ui-tree-navigation Specification

## Purpose
TBD - created by archiving change redesign-ui-explorer. Update Purpose after archive.
## Requirements
### Requirement: Persistent tree pane

The portal SHALL display a persistent tree pane alongside the node view. The pane SHALL
hold every branch the user has opened during the session, each row carrying the reference's
kind, its namespace and name, and its health. A row whose reference is expandable SHALL be
marked as such, and a row holding children SHALL show how many.

#### Scenario: Opened branches stay on screen

- **WHEN** the user opens a Kustomization, then one of its child HelmReleases
- **THEN** the tree shows the root level, the Kustomization with its children, and the HelmRelease with its children, all at once

#### Scenario: Every tree row carries health

- **WHEN** a branch is shown in the tree
- **THEN** each of its rows shows the health of that reference

#### Scenario: The selected node is marked in the tree

- **WHEN** a node is shown in the node view
- **THEN** its row in the tree is marked as the current one

### Requirement: Expansion is independent of selection

The tree SHALL offer two distinct actions on a row: **expanding** it, which reveals its
children in the tree and MUST NOT change which node the node view shows, and **selecting**
it, which opens it in the node view and reveals it in the tree. Expanding a row MUST resolve
that one reference only — a single hop, as everywhere else in the portal.

#### Scenario: Expanding leaves the node view alone

- **WHEN** the user expands a sibling branch while a node is open in the node view
- **THEN** the sibling's children appear in the tree and the node view still shows the same node

#### Scenario: Selecting opens and reveals

- **WHEN** the user selects a row in the tree
- **THEN** that reference is shown in the node view and its branch is revealed in the tree

#### Scenario: Expanding costs one hop

- **WHEN** the user expands a Kustomization in the tree
- **THEN** only that Kustomization is resolved, and its children's own children are not

### Requirement: The node view is never rendered from held material

Selecting a reference SHALL resolve it, even when its children are already held in the tree
from an earlier expansion. The node view MUST always show the result of that resolution, so
a value it displays was read when it was shown.

#### Scenario: Selecting a branch already expanded re-resolves it

- **WHEN** the user expands a Kustomization, explores elsewhere, then selects that Kustomization
- **THEN** it is resolved again and the node view shows the result of that call, not what the tree held

### Requirement: Held material states its age and can be re-read

Every branch held in the tree SHALL show how long ago it was read, and the portal SHALL
offer a control that re-reads it. The node view SHALL likewise show the age of its own read
with a re-read control. Held material MUST NOT be presented as current.

#### Scenario: A branch says when it was read

- **WHEN** a branch has been on screen for some minutes
- **THEN** the portal shows how long ago that branch was read

#### Scenario: Re-reading refreshes one branch

- **WHEN** the user activates the re-read control on a branch
- **THEN** that branch is resolved again, its rows show the new health, and its age resets

#### Scenario: Other branches are untouched by a re-read

- **WHEN** the user re-reads one branch while three others are open
- **THEN** only that branch is resolved

### Requirement: Collapsing discards what a branch held

Collapsing a branch SHALL discard the material held for it, so that expanding it again
resolves it afresh rather than restoring what was read earlier.

#### Scenario: Re-expanding a collapsed branch re-resolves it

- **WHEN** the user collapses a branch and expands it again
- **THEN** the branch is resolved again and its rows show health read at that moment

### Requirement: The tree reflects the URL trail on arrival

Opening an exploration URL SHALL render the tree with that trail's hops expanded down to
the selected node, which is shown in the node view. Restoring a trail MUST NOT resolve its
intermediate hops: only the selected node is resolved, and the intermediate rows are shown
from their own identifiers until the user expands them.

#### Scenario: A deep link opens with its path visible in the tree

- **WHEN** a user opens a three-hop exploration URL
- **THEN** the tree shows the three hops as a path down to the selected node, which is shown in the node view

#### Scenario: Restoring a trail still costs one resolution

- **WHEN** a five-hop exploration URL is opened
- **THEN** only the last hop is resolved

#### Scenario: Open branches are not part of the address

- **WHEN** the user expands two sibling branches without selecting anything
- **THEN** the URL is unchanged, because the selected node has not changed

### Requirement: A reference repeated on a path is a leaf pointer

A reference already present on the path from the root to the current row SHALL be shown
as a marked leaf and MUST NOT be descended into again, mirroring the engine's visited
node. Without it the Flux bootstrap Kustomization, which applies itself, makes the tree
walk unbounded.

#### Scenario: A self-applying Kustomization terminates the branch

- **WHEN** the user expands a Kustomization whose inventory contains that same Kustomization
- **THEN** the repeat is shown as a marked leaf under it, with no expansion control, and the tree renders

#### Scenario: The same reference under a different path still expands

- **WHEN** a reference appears under two different branches, on neither of which it is its own ancestor
- **THEN** it is expandable in both

### Requirement: The tree pane adapts to a narrow viewport

On a viewport too narrow for two panes, the portal SHALL show the node view full width and
make the tree reachable through a control in the header, without losing the tree's state.
Selecting a row from there SHALL return the user to the node view.

#### Scenario: Tree is reachable on a phone-width viewport

- **WHEN** the portal is opened at phone width
- **THEN** the node view fills the width and a control opens the tree over it

#### Scenario: Selecting from the narrow tree returns to the node

- **WHEN** the user selects a row from the tree on a phone-width viewport
- **THEN** the tree closes and the node view shows the selected reference
