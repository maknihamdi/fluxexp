# ui-layer-filter Specification

## Purpose
TBD - created by archiving change redesign-ui-explorer. Update Purpose after archive.
## Requirements
### Requirement: Filter by name fragment

The portal SHALL offer a filter that narrows what is displayed to the references whose kind,
namespace or name contains a given fragment, matched case-insensitively. The filter SHALL
apply to the tree and to the node view's list of children at once.

#### Scenario: Typing a fragment narrows both panes

- **WHEN** the user types `cert` in the filter
- **THEN** the tree and the current list show only references matching `cert`, and the rest are hidden

#### Scenario: Clearing the filter restores everything

- **WHEN** the user clears the filter
- **THEN** every reference held in the tree and every child of the open node is shown again

### Requirement: Filter by health

The portal SHALL let the user exclude health values from what is displayed, so a layer can
be reduced to what is not healthy. Health filtering and name filtering SHALL combine.

#### Scenario: Hiding healthy references leaves the failures

- **WHEN** the user excludes `healthy` on a layer of 67 entries of which 3 are unhealthy
- **THEN** the list shows those 3, and the layer header states that 3 of 67 are shown

#### Scenario: Health and name filters combine

- **WHEN** the user excludes `healthy` and types `team-a`
- **THEN** only non-healthy references matching `team-a` are shown

### Requirement: Filtering keeps the path to a match

A branch SHALL be kept when it matches the filter **or when any reference beneath it
matches**, so that filtering never hides the path leading to a match.

#### Scenario: An ancestor that does not match is kept

- **WHEN** the filter matches a Pod three levels down
- **THEN** the Kustomization and Deployment above it remain visible in the tree as its path

#### Scenario: A branch with no match is hidden

- **WHEN** no reference in a branch matches the filter
- **THEN** that whole branch is hidden

### Requirement: Filtering never resolves anything

Applying or changing a filter MUST NOT resolve any reference and MUST NOT issue any cluster
call: it narrows material already fetched. The portal SHALL state how many of the available
entries are shown, so the user can tell a filtered view from a complete one.

#### Scenario: No call is made while filtering

- **WHEN** the user types in the filter and changes the health selection
- **THEN** no request is sent to the portal's API

#### Scenario: A filtered layer says so

- **WHEN** a filter reduces a layer of 42 children to 5
- **THEN** the layer header shows that 5 of 42 are displayed

#### Scenario: A filter matching nothing is stated, not empty

- **WHEN** a filter matches no entry of the open layer
- **THEN** the portal states that nothing in the layer matches the filter, distinct from a node that applies nothing
