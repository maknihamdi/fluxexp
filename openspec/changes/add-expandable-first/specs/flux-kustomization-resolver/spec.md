## ADDED Requirements

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
