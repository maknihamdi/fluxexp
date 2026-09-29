## ADDED Requirements

### Requirement: Pending status is visually distinct

The portal SHALL render a **pending** node with a health badge visually distinct from
healthy, unhealthy, unknown and error, wherever a health badge appears — the roots
home, a revealed layer, a node's own header, and a listed dependency. A pending node
MUST NOT read as healthy, so a reader scanning a layer can tell an object that is
still converging from one that is working.

#### Scenario: A pending object is distinguishable in a layer

- **WHEN** a revealed layer contains a pending object alongside healthy and unhealthy ones
- **THEN** the pending object's badge differs from both, and it does not read as healthy

#### Scenario: A pending root shows its badge on the home

- **WHEN** a root Kustomization's status is pending
- **THEN** the roots home shows it with the pending badge and its detail message
