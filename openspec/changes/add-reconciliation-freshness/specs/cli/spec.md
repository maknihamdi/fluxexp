## ADDED Requirements

### Requirement: Freshness in the tree

The traverse command SHALL render a node's freshness next to its health when
present, and fold the node's key fields (applied revision, synced time, and — on
failure — the attempted revision) into the rendered line. A node with no
freshness MUST render as before (health only).

#### Scenario: Kustomization line shows health and freshness

- **WHEN** a healthy, up-to-date Kustomization is rendered
- **THEN** the line shows both its health and an up-to-date freshness indicator, plus the applied short revision and synced time

#### Scenario: Behind Kustomization shows both revisions

- **WHEN** a Kustomization is behind its source
- **THEN** the line shows a behind freshness indicator and both the applied and source short revisions

#### Scenario: Non-freshness node unchanged

- **WHEN** a node without freshness (e.g. a Deployment) is rendered
- **THEN** the line shows only its health, unchanged from before
