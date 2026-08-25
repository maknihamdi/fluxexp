# workload-health-resolver Specification

## Purpose
TBD - created by archiving change add-workload-health. Update Purpose after archive.
## Requirements
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

### Requirement: Replica-based health

For Deployment, StatefulSet and ReplicaSet the resolver MUST report health by
comparing ready replicas to the desired replica count (defaulting a missing
desired count to 1, and treating a desired count of 0 as healthy). It MUST report
**healthy** when ready ≥ desired and **unhealthy** otherwise, surfacing a
`<ready>/<desired> ready` detail.

#### Scenario: Fully-ready Deployment is healthy

- **WHEN** a Deployment has desired 2 and ready 2
- **THEN** the resolver reports healthy with detail `2/2 ready`

#### Scenario: Under-ready Deployment is unhealthy

- **WHEN** a Deployment has desired 3 and ready 1
- **THEN** the resolver reports unhealthy with detail `1/3 ready`

#### Scenario: Zero desired replicas is healthy

- **WHEN** a Deployment has desired 0
- **THEN** the resolver reports healthy

### Requirement: DaemonSet health

For a DaemonSet the resolver MUST compare `numberReady` to
`desiredNumberScheduled`, reporting **healthy** when ready ≥ desired (including
`0/0`) and **unhealthy** otherwise, with a `<ready>/<desired> ready` detail.

#### Scenario: DaemonSet all ready is healthy

- **WHEN** a DaemonSet has desiredNumberScheduled 4 and numberReady 4
- **THEN** the resolver reports healthy

#### Scenario: DaemonSet missing pods is unhealthy

- **WHEN** a DaemonSet has desiredNumberScheduled 4 and numberReady 2
- **THEN** the resolver reports unhealthy with detail `2/4 ready`

### Requirement: Pod health

For a Pod the resolver MUST report health from `.status.phase` and the `Ready`
condition: a `Running` pod is **healthy** only when its `Ready` condition is
`True` (otherwise **unhealthy**); `Succeeded` is **healthy**; `Failed` is
**unhealthy**; `Pending` (or unknown phase) is **unknown**. The phase MUST be
surfaced as detail.

#### Scenario: Running and ready Pod is healthy

- **WHEN** a Pod has phase `Running` and its `Ready` condition is `True`
- **THEN** the resolver reports healthy

#### Scenario: Running but not ready Pod is unhealthy

- **WHEN** a Pod has phase `Running` and its `Ready` condition is `False`
- **THEN** the resolver reports unhealthy

#### Scenario: Pending Pod is unknown

- **WHEN** a Pod has phase `Pending`
- **THEN** the resolver reports unknown

### Requirement: Job health

For a Job the resolver MUST report **healthy** when a `Complete` condition is
`True`, **unhealthy** when a `Failed` condition is `True`, and **unknown**
otherwise (still running).

#### Scenario: Completed Job is healthy

- **WHEN** a Job has a `Complete` condition with status `True`
- **THEN** the resolver reports healthy

#### Scenario: Failed Job is unhealthy

- **WHEN** a Job has a `Failed` condition with status `True`
- **THEN** the resolver reports unhealthy

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

