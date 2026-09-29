# node-dependencies Specification

## Purpose
The dependency edge class: what a node requires in order to reconcile, shown with its health and fields but never descended into, and kept distinct from what the node produces.

## Requirements
### Requirement: Dependency edge class

The system SHALL distinguish two kinds of outgoing edge from a node:
**children**, the objects the node produces or applies, and **dependencies**, the
objects the node requires in order to reconcile. A resolver MAY return
dependencies alongside children; both MUST be domain-agnostic references, and a
dependency MAY belong to a different domain than the node that declares it.

#### Scenario: A resolver returns both kinds

- **WHEN** a resolver resolves a reference that both requires a source and applies objects
- **THEN** it returns the source among the dependencies and the applied objects among the children, in two separate lists

#### Scenario: Dependencies are optional

- **WHEN** a resolver returns no dependencies
- **THEN** the resolved node carries an empty dependency list and behaves exactly as before

### Requirement: Dependencies are resolved but never descended

The engine SHALL resolve each dependency reference **one level deep** — enough to
obtain its health, detail and fields — and MUST NOT enqueue that dependency's own
children for traversal. A dependency that fails to resolve MUST become an error
node without affecting its parent's health or the traversal of anything else.

#### Scenario: A dependency's subtree is not traversed

- **WHEN** a node declares a dependency that would itself return children
- **THEN** the resolved dependency node carries its own health but has no children

#### Scenario: Dependency chains terminate

- **WHEN** a node depends on a second node that depends on a third
- **THEN** only the first level is resolved, and the traversal does not follow the chain

#### Scenario: An unresolvable dependency is an error node

- **WHEN** a declared dependency cannot be retrieved
- **THEN** it appears as an error node carrying its reason, and its parent keeps its own health

### Requirement: Dependencies are excluded from deduplication

Dependencies SHALL NOT participate in the engine's visited-reference tracking:
resolving a reference as a dependency MUST NOT prevent the same reference from
being fully expanded later as a child, and MUST NOT be replaced by an
already-visited pointer.

#### Scenario: A dependency does not suppress a later child expansion

- **WHEN** a reference is first resolved as one node's dependency and later appears as another node's child
- **THEN** the child occurrence is expanded normally, with its own children

#### Scenario: A shared dependency is shown under each dependent

- **WHEN** two nodes declare the same dependency
- **THEN** both carry a fully resolved dependency node rather than one of them carrying a visited pointer

### Requirement: Fields derivable without a second retrieval

The system SHALL expose the fields computable from an **already-retrieved**
object, so a surface holding an object for another purpose can render its fields
without issuing a second call. Kinds with no fields MUST yield an empty list
rather than an error.

#### Scenario: Fields come from an object already in hand

- **WHEN** a caller holds a fetched GitRepository and asks for its fields
- **THEN** it receives the repository, tracked reference and interval fields without any additional retrieval

#### Scenario: A kind with no fields yields nothing

- **WHEN** the same operation is applied to a ConfigMap
- **THEN** it yields an empty field list and no error

### Requirement: Dependency references are derivable from a fetched object

The system SHALL expose the dependency references derivable from an
**already-fetched** object, so a surface holding it can group them with it
without resolving the object and without computing its children. The operation
MUST require no retrieval, MUST NOT error, and MUST yield an empty list for kinds
that declare no dependencies.

A Kustomization's `spec.sourceRef` and `spec.dependsOn`, and a HelmRelease's
chart source and `spec.dependsOn`, live in their own manifest, so obtaining them
MUST cost nothing beyond the object already in hand. For a HelmRelease this is
not merely an optimisation: resolving one reads and decompresses its Helm storage
Secret, so a listed entry MUST obtain its dependency references without being
resolved.

The derivation MUST remain the single source of these references for every kind
that declares them — each resolver obtains them the same way — so that what the
engine traverses and what a listed row shows cannot drift apart.

#### Scenario: A fetched Kustomization yields its dependency references

- **WHEN** a caller holds a fetched Kustomization declaring a source and a dependsOn entry
- **THEN** it receives both references, source first, without any retrieval and without the object's children being computed

#### Scenario: A fetched HelmRelease yields its dependency references

- **WHEN** a caller holds a fetched HelmRelease declaring a chart source and a dependsOn entry
- **THEN** it receives both references, source first, without any retrieval and without the Helm storage Secret being read

#### Scenario: Kinds without dependencies yield nothing

- **WHEN** the operation is applied to a fetched ConfigMap or Pod
- **THEN** it yields an empty list and no error

#### Scenario: The same rule serves resolution and listing

- **WHEN** a Kustomization or a HelmRelease is resolved through its resolver and, separately, its dependency references are derived from the fetched object
- **THEN** both produce the same references in the same order

### Requirement: A declared dependency is not repeated among children

A reference a node declares as a dependency SHALL NOT also appear among that
node's children. An object can legitimately be both — a Flux Kustomization
routinely applies the very GitRepository it reconciles from — but showing it in
both places states the same fact twice. The dependency grouping wins: the entry
is kept with the node it belongs to and removed from the children.

The rule MUST be applied where both surfaces share it, so the CLI tree and the
portal cannot disagree about which entries a node lists.

#### Scenario: An object that is both source and applied appears once

- **WHEN** a Kustomization declares a GitRepository as its source and also lists it in its inventory
- **THEN** it appears once, grouped with the Kustomization as a dependency, and not among the applied children

#### Scenario: Unrelated children are untouched

- **WHEN** a node's children include references it does not depend on
- **THEN** all of them remain, in their tier order
