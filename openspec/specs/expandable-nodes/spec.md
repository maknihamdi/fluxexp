# expandable-nodes Specification

## Purpose
Declaring which references can descend, ordering children so the path deeper into the graph reads first, and treating expandability as a hint about a type rather than a measurement of an instance.

## Requirements
### Requirement: Expandability predicate

The system SHALL let a resolver declare, for a reference it matches, whether that
reference **can descend** (has a child layer). The declaration MUST be derivable
from the reference alone: it MUST NOT fetch the object, MUST NOT require a
resolve context, and MUST NOT return an error.

#### Scenario: Predicate answers without retrieval

- **WHEN** the expandability of a reference is queried
- **THEN** the answer is produced from the reference's domain, type and coordinates alone, performing no backend call

#### Scenario: Predicate is total

- **WHEN** the predicate is invoked for any reference the resolver matches
- **THEN** it returns a boolean, never an error

### Requirement: Per-resolver expandability declarations

Each resolver SHALL declare its own expandability. The Flux `Kustomization` and
`HelmRelease` resolvers MUST declare their references expandable. The workload
resolver MUST declare its kinds expandable **except `Pod`**, which MUST be
declared non-expandable. The generic Kubernetes fallback MUST declare every
reference non-expandable.

#### Scenario: Flux objects are expandable

- **WHEN** the predicate is queried for a Kustomization or a HelmRelease reference
- **THEN** it reports expandable

#### Scenario: Workloads are expandable except Pods

- **WHEN** the predicate is queried for a Deployment, StatefulSet, DaemonSet, ReplicaSet or Job reference
- **THEN** it reports expandable

#### Scenario: A Pod is not expandable

- **WHEN** the predicate is queried for a Pod reference
- **THEN** it reports not expandable

#### Scenario: Unknown kinds are not expandable

- **WHEN** the predicate is queried for a reference handled only by the generic Kubernetes fallback, such as a ConfigMap
- **THEN** it reports not expandable

### Requirement: Registry-level expandability lookup

The registry SHALL answer the expandability of any reference by delegating to the
resolver its matcher selects, applying the same specific-before-fallback
selection used for resolution. A reference for which no resolver and no domain
fallback exists MUST report not expandable.

#### Scenario: Lookup follows resolver selection

- **WHEN** the registry is asked whether a reference is expandable
- **THEN** it returns the declaration of the resolver that would resolve it

#### Scenario: Unresolvable reference is not expandable

- **WHEN** the registry is asked about a reference in a domain with no registered resolver and no fallback
- **THEN** it reports not expandable

### Requirement: Flux objects rank first

The system SHALL treat a reference whose type belongs to a Flux API group — any
group ending in `toolkit.fluxcd.io` — as the highest-ranking tier when ordering
children, **regardless of whether it is expandable**. The test MUST be derived
from the reference alone.

#### Scenario: A Flux leaf outranks a non-Flux container

- **WHEN** a child list contains a GitRepository (Flux, not expandable) and a Deployment (not Flux, expandable)
- **THEN** the GitRepository is ordered before the Deployment

#### Scenario: Every Flux group qualifies

- **WHEN** the tier is computed for `source.toolkit.fluxcd.io`, `kustomize.toolkit.fluxcd.io`, `helm.toolkit.fluxcd.io`, `image.toolkit.fluxcd.io` and `notification.toolkit.fluxcd.io` references
- **THEN** each is placed in the Flux tier

#### Scenario: A lookalike group does not qualify

- **WHEN** the tier is computed for a reference in a group that merely contains the word `flux` but does not end in `toolkit.fluxcd.io`
- **THEN** it is not placed in the Flux tier

### Requirement: Three-tier child ordering

The system SHALL provide a single ordering operation that returns child
references in three tiers: Flux objects first, then the remaining **expandable**
references, then all others. The ordering MUST be **stable**: within each tier,
children MUST keep the relative order in which the resolver produced them. The
operation MUST be the only place this rule is implemented, so that every surface
consuming children shares it.

#### Scenario: Tiers are applied in order

- **WHEN** a resolver returns a HelmRelease (Flux), a Deployment (expandable), and a ConfigMap (leaf), in that reversed order
- **THEN** the ordered list is the HelmRelease, then the Deployment, then the ConfigMap

#### Scenario: Relative order is preserved within each tier

- **WHEN** children are ordered
- **THEN** two children of the same tier appear in their original relative order

#### Scenario: Uniform lists are unchanged

- **WHEN** every child belongs to the same tier
- **THEN** the ordered list is identical to the input

### Requirement: Expandability is a hint

Expandability SHALL be understood as a property of a reference's **type**, not a
measurement of the instance. A reference declared expandable MAY resolve to an
empty list of children, and this MUST NOT be treated as an error by any surface.

#### Scenario: An expandable node resolves to no children

- **WHEN** a node declared expandable is resolved and returns zero children
- **THEN** it is rendered as an ordinary node with no children, and no error is reported
