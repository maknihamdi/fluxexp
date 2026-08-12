# resource-traversal Specification

## Purpose
TBD - created by archiving change add-traversal-engine. Update Purpose after archive.
## Requirements
### Requirement: Domain-agnostic resource reference

The system SHALL identify every node by a **domain-agnostic reference** (`Ref`)
that MUST NOT assume Kubernetes. A `Ref` MUST carry: a `domain` (e.g.
`kubernetes`, `gcp`, or any future backend), a `type` (an opaque type identifier
meaningful within that domain, such as a GVK string for Kubernetes or a resource
type for a cloud), and a set of `coordinates` locating the object within its
domain. A `Ref` MUST expose a **stable key** derived from its domain, type, and
coordinates, used for deduplication and cycle detection.

#### Scenario: Reference identifies a non-Kubernetes object

- **WHEN** a reference is constructed for a cloud object in domain `gcp`
- **THEN** the reference carries `domain=gcp`, its cloud type, and the coordinates needed to locate it, and yields a stable key distinct from any Kubernetes reference

#### Scenario: Stable key deduplicates equal references

- **WHEN** two references have the same domain, type, and coordinates
- **THEN** they produce the same key

### Requirement: Resolver contract

The system SHALL define a resolver contract that is not tied to any single
domain. A resolver MUST declare, via a **matcher**, which references it handles.
Given a reference it handles, a resolver MUST retrieve the underlying object
**itself** (through shared clients provided in a resolve context) and return that
object's **health** plus a list of **child references** to traverse next.
Resolvers MUST be read-only. Child references MAY belong to a **different
domain** than the resolver that produced them.

#### Scenario: Resolver retrieves and reports children and health

- **WHEN** the engine invokes a resolver with a reference it matches
- **THEN** the resolver retrieves the object via the resolve context and returns a health status and a (possibly empty) list of child references

#### Scenario: Resolver claims references via its matcher

- **WHEN** the registry tests a reference against a resolver's matcher
- **THEN** the matcher decides, from the reference's domain and type, whether that resolver handles it

#### Scenario: Children may switch domains

- **WHEN** a resolver in domain `kubernetes` determines its object maps to an object in domain `gcp`
- **THEN** it returns child references in domain `gcp`, and the engine traverses them with a `gcp` resolver without any special-casing in the engine

### Requirement: Resolver registry with fallback

The system SHALL maintain a registry that selects, for a given reference, the
first registered resolver whose matcher claims it. When no registered resolver
matches, the registry MUST return a **fallback resolver** for the reference's
domain. The registry MUST support registering resolvers for any domain and MUST
allow a per-domain fallback.

#### Scenario: Specific resolver selected before fallback

- **WHEN** a reference matches a registered resolver
- **THEN** the registry returns that resolver rather than a fallback

#### Scenario: Fallback used when nothing matches

- **WHEN** a reference matches no registered resolver
- **THEN** the registry returns the fallback resolver registered for that reference's domain

### Requirement: Generic Kubernetes fallback resolver

The system SHALL provide a generic fallback resolver for the `kubernetes` domain
that handles any Kubernetes reference not claimed by a more specific resolver. It
MUST report health from the object's `.status.conditions` entry of type `Ready`
(`True` → healthy, `False` → unhealthy), treat an absent `Ready` condition as
**unknown**, and return no children.

#### Scenario: Generic fallback reads the Ready condition

- **WHEN** the generic Kubernetes fallback processes an object whose `Ready` condition has `status: "True"`
- **THEN** it reports the node as healthy and returns no children

#### Scenario: Generic fallback handles missing Ready condition

- **WHEN** the generic Kubernetes fallback processes an object with no `Ready` condition
- **THEN** it reports health as unknown and returns no children

### Requirement: Traversal engine

The system SHALL provide an engine that, starting from one root reference,
recursively resolves children into a result tree. The engine MUST be
domain-agnostic: for each node it selects a resolver via the registry, delegates
retrieval and interpretation entirely to that resolver, and recurses into the
returned child references. The engine MUST NOT itself perform any
domain-specific retrieval, and MUST be read-only.

#### Scenario: Engine builds a tree from the root

- **WHEN** the engine is run against a root reference whose resolver returns two children
- **THEN** the result is a tree whose root node has two child nodes, each carrying its own resolved health

#### Scenario: Engine mixes domains in one tree

- **WHEN** a root in domain `kubernetes` yields, several hops down, a child in domain `gcp`
- **THEN** the single result tree contains both the Kubernetes and the `gcp` nodes, each resolved by its own domain's resolver

### Requirement: Arbitrary chaining

The engine MUST support arbitrary-depth chains of the same or different types,
including a resolver whose children are of the same type as itself (e.g. a
Kustomization that applies another Kustomization). Depth MUST be bounded only by
the graph itself and cycle detection, not by any hardcoded hop limit.

#### Scenario: Kustomization chains into another Kustomization

- **WHEN** a Kustomization's children include a second Kustomization, which in turn has its own children
- **THEN** the engine resolves the second Kustomization with the same resolver and continues expanding its children to full depth

### Requirement: Cycle safety

The traversal engine MUST track visited nodes by each reference's stable key and
MUST NOT resolve the same node twice, so that ownership or reference cycles
cannot cause infinite traversal.

#### Scenario: Cycle does not loop forever

- **WHEN** reference A lists B as a child and B lists A as a child
- **THEN** the engine visits A and B once each and terminates, marking the repeated reference as already-visited rather than re-expanding it

### Requirement: Partial-failure tolerance

The engine MUST tolerate partial failure: if a resolver cannot retrieve its
object or returns an error, the engine records that node as an **error node**
carrying the failure reason and continues traversing the remaining siblings and
subtrees.

#### Scenario: One failed child does not abort the walk

- **WHEN** a node has three children and resolving the second one fails
- **THEN** the result tree contains all three child nodes, the second marked as an error node with its failure reason, and the first and third fully resolved

### Requirement: Per-node health model

Every node in the result tree MUST carry a health status drawn from a fixed,
domain-independent set: at minimum **healthy**, **unhealthy**, **unknown**, and
**error** (error meaning the node itself could not be retrieved or resolved).

#### Scenario: Node exposes a health status

- **WHEN** the result tree is produced
- **THEN** each node exposes exactly one health status value from the fixed set

