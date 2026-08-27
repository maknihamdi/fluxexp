# flux-kustomization-resolver Specification

## Purpose
TBD - created by archiving change add-traversal-engine. Update Purpose after archive.
## Requirements
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

### Requirement: Kustomization declares its source and dependsOn as dependencies

The Kustomization resolver MUST declare as **dependencies** — not children — the
object referenced by `spec.sourceRef`, followed by each Kustomization listed in
`spec.dependsOn`, in declaration order. The inventory objects MUST remain
children. A `dependsOn` entry's namespace MUST default to the Kustomization's own
namespace when unset, as MUST the source's.

Both are read from the Kustomization's own manifest, so obtaining them MUST NOT
require any retrieval. Every declared reference MUST be emitted without
pre-fetching: one that cannot be retrieved surfaces as an error node, because a
Kustomization pointing at a missing source or a missing dependency is exactly
what the reader needs to see. Only an **unconfigured** reference is absent from
the list.

#### Scenario: Source leads the dependencies

- **WHEN** a Kustomization references a GitRepository and lists two entries in `spec.dependsOn`
- **THEN** it returns three dependencies, the GitRepository first, then the two entries in declaration order

#### Scenario: Inventory stays in children

- **WHEN** a Kustomization references a source and its inventory lists three objects
- **THEN** the three inventory objects are children and the source is not among them

#### Scenario: Namespaces default to the Kustomization's

- **WHEN** neither `spec.sourceRef.namespace` nor a `spec.dependsOn` entry's namespace is set
- **THEN** both emitted references use the Kustomization's own namespace

#### Scenario: Unfetchable source surfaces as an error node

- **WHEN** a Kustomization's source object cannot be retrieved
- **THEN** the source dependency is still emitted and resolves to an error node, and the Kustomization keeps its own health

#### Scenario: Missing dependsOn target surfaces as an error node

- **WHEN** a Kustomization lists a `spec.dependsOn` entry that does not exist
- **THEN** that dependency resolves to an error node, and the Kustomization keeps its own health

#### Scenario: Sourceless Kustomization is unaffected

- **WHEN** a Kustomization has no `spec.sourceRef` and no `spec.dependsOn`
- **THEN** it returns no dependencies and only its inventory children
