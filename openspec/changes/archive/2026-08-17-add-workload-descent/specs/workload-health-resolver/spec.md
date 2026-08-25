## MODIFIED Requirements

### Requirement: Workload matcher

The system SHALL provide a resolver claiming the core Kubernetes workload kinds
in the `kubernetes` domain: `apps/v1` Deployment, StatefulSet, DaemonSet and
ReplicaSet; `v1` Pod; `batch/v1` Job. Its matcher MUST claim exactly those kinds
and MUST NOT claim other references. The resolver MUST be registered ahead of the
generic Kubernetes fallback. The resolver descends to owned objects (it is no
longer a leaf), except a Pod which MUST remain a leaf.

#### Scenario: Matcher claims workload kinds only

- **WHEN** the registry tests a Deployment reference and a ConfigMap reference against this resolver
- **THEN** the matcher claims the Deployment and rejects the ConfigMap

#### Scenario: Workload resolver preferred over the fallback

- **WHEN** a Deployment reference is resolved through the default registry
- **THEN** the workload resolver handles it rather than the generic fallback

## ADDED Requirements

### Requirement: Owner-reference descent

The resolver MUST discover a workload's children by ownership: it lists the
candidate child kind in the parent's namespace and keeps only objects whose
`.metadata.ownerReferences` include an entry with the parent's UID. Each kept
object becomes a `kubernetes` child reference.

#### Scenario: Only owned objects become children

- **WHEN** the namespace contains two ReplicaSets, one owned by the Deployment (matching UID) and one owned by a different Deployment
- **THEN** only the owned ReplicaSet is returned as a child

### Requirement: Deployment descends to active ReplicaSets

For a Deployment the resolver MUST return the owned ReplicaSets that have
replicas greater than 0 (the active revisions), omitting scaled-to-0 old
revisions. When any old revisions are omitted, their count MUST be surfaced on
the Deployment node's detail (not silently dropped).

#### Scenario: Active ReplicaSet is a child, old ones are omitted

- **WHEN** a Deployment owns one ReplicaSet with replicas 1 and three ReplicaSets with replicas 0
- **THEN** only the replicas-1 ReplicaSet is returned as a child, and the Deployment detail notes 3 omitted old revisions

### Requirement: Pod-owning workloads descend to Pods

For a ReplicaSet, StatefulSet, DaemonSet or Job the resolver MUST return the Pods
it owns (by UID) as children.

#### Scenario: ReplicaSet descends to its Pods

- **WHEN** a ReplicaSet owns two Pods
- **THEN** the resolver returns those two Pods as children

#### Scenario: Pod is a leaf

- **WHEN** a Pod is resolved
- **THEN** it returns no children
