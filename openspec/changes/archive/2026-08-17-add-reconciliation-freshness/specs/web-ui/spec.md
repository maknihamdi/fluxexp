## ADDED Requirements

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
