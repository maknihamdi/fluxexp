## MODIFIED Requirements

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
