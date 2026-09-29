## MODIFIED Requirements

### Requirement: Per-node health model

Every node in the result tree MUST carry a health status drawn from a fixed,
domain-independent set: at minimum **healthy**, **unhealthy**, **pending**,
**unknown**, and **error** (error meaning the node itself could not be retrieved or
resolved). **pending** means the backend has not yet caught up with the object's
declared spec — it is neither a working object nor a broken one — and is distinct from
**unknown**, which means the node's state could not be interpreted.

#### Scenario: Node exposes a health status

- **WHEN** the result tree is produced
- **THEN** each node exposes exactly one health status value from the fixed set

#### Scenario: Pending is distinct from unknown and unhealthy

- **WHEN** a node's backend has not yet observed its declared spec
- **THEN** the node carries the pending status, distinguishable from both unknown and unhealthy

### Requirement: Generic Kubernetes fallback resolver

The system SHALL provide a generic fallback resolver for the `kubernetes` domain
that handles any Kubernetes reference not claimed by a more specific resolver. It
MUST report health through the shared Kubernetes status derivation — which covers
objects with no status, conditions other than `Ready`, and generation drift — and
return no children. It MUST NOT implement its own condition reading.

#### Scenario: Generic fallback reads the Ready condition

- **WHEN** the generic Kubernetes fallback processes an object whose `Ready` condition has `status: "True"`
- **THEN** it reports the node as healthy and returns no children

#### Scenario: Generic fallback handles an object with no conditions

- **WHEN** the generic Kubernetes fallback processes an object with no `Ready` condition
- **THEN** it reports the status the shared derivation produces for that object, not unknown by default, and returns no children
