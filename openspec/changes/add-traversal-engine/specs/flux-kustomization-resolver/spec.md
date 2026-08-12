## ADDED Requirements

### Requirement: Kustomization matcher

The system SHALL provide a resolver for the Flux Kustomization type in the
`kubernetes` domain (`kustomize.toolkit.fluxcd.io`, kind `Kustomization`). Its
matcher MUST claim references whose domain is `kubernetes` and whose type is a
Kustomization, and MUST NOT claim other references.

#### Scenario: Matcher claims Kustomization references only

- **WHEN** the registry tests a `kubernetes` Kustomization reference and, separately, a Deployment reference against this resolver
- **THEN** the matcher claims the Kustomization reference and rejects the Deployment reference

### Requirement: Kustomization children from inventory

The Kustomization resolver MUST retrieve the Kustomization object via the shared
Kubernetes client and discover children by reading `.status.inventory.entries`.
Each entry encodes an object's identifier and apiVersion; the resolver MUST
decode every entry into a child reference in the `kubernetes` domain, carrying
the object's type (GVK) and coordinates (namespace, name).

#### Scenario: Inventory entries become children

- **WHEN** the resolver processes a Kustomization whose `.status.inventory.entries` lists three objects
- **THEN** it returns three `kubernetes` child references, each with the type (GVK) and coordinates decoded from its entry

#### Scenario: Empty or absent inventory yields no children

- **WHEN** the resolver processes a Kustomization with no `.status.inventory` or an empty entries list
- **THEN** it returns no children and does not error

### Requirement: Kustomization health

The Kustomization resolver MUST report the object's health from its
`.status.conditions` `Ready` condition: `True` is healthy, `False` is unhealthy,
and an absent `Ready` condition is unknown.

#### Scenario: Ready True is healthy

- **WHEN** the Kustomization's `Ready` condition has `status: "True"`
- **THEN** the resolver reports the node as healthy

#### Scenario: Ready False is unhealthy

- **WHEN** the Kustomization's `Ready` condition has `status: "False"`
- **THEN** the resolver reports the node as unhealthy and surfaces the condition message as the node's detail
