## MODIFIED Requirements

### Requirement: Replica-based health

For Deployment, StatefulSet and ReplicaSet the resolver MUST first check whether the
controller has observed the object's current spec, reporting **pending** when
`metadata.generation` is ahead of `status.observedGeneration`, through the shared
status derivation rather than its own comparison. Otherwise it MUST report health by
comparing ready replicas to the desired replica count (defaulting a missing desired
count to 1, and treating a desired count of 0 as healthy). It MUST report **healthy**
when ready ≥ desired and **unhealthy** otherwise, surfacing a `<ready>/<desired> ready`
detail.

#### Scenario: Fully-ready Deployment is healthy

- **WHEN** a Deployment has desired 2 and ready 2
- **THEN** the resolver reports healthy with detail `2/2 ready`

#### Scenario: Under-ready Deployment is unhealthy

- **WHEN** a Deployment has desired 3 and ready 1
- **THEN** the resolver reports unhealthy with detail `1/3 ready`

#### Scenario: Zero desired replicas is healthy

- **WHEN** a Deployment has desired 0
- **THEN** the resolver reports healthy

#### Scenario: A Deployment whose new spec is unobserved is pending

- **WHEN** a Deployment has desired 3, ready 0, `metadata.generation` 4 and `status.observedGeneration` 3
- **THEN** the resolver reports pending, because the replica counts describe the previous spec

### Requirement: Pod health

For a Pod the resolver MUST report health from `.status.phase` and the `Ready`
condition: a `Running` pod is **healthy** only when its `Ready` condition is
`True` (otherwise **unhealthy**); `Succeeded` is **healthy**; `Failed` is
**unhealthy**; `Pending` is **pending**. Only the `Unknown` phase — the API
server has lost contact with the node, so the pod's state cannot be read — MAY
report **unknown**. The phase MUST be surfaced as detail.

#### Scenario: Running and ready Pod is healthy

- **WHEN** a Pod has phase `Running` and its `Ready` condition is `True`
- **THEN** the resolver reports healthy

#### Scenario: Running but not ready Pod is unhealthy

- **WHEN** a Pod has phase `Running` and its `Ready` condition is `False`
- **THEN** the resolver reports unhealthy

#### Scenario: Pending Pod is pending

- **WHEN** a Pod has phase `Pending`
- **THEN** the resolver reports pending, because a pod that is scheduling or pulling images has a readable state and is simply not running yet

#### Scenario: Pod on an unreachable node is unknown

- **WHEN** a Pod has phase `Unknown`
- **THEN** the resolver reports unknown, because its state genuinely cannot be read

### Requirement: Job health

For a Job the resolver MUST report **healthy** when a `Complete` condition is
`True`, **unhealthy** when a `Failed` condition is `True`, and **pending**
otherwise, because a job with neither condition is still running rather than
unreadable.

#### Scenario: Completed Job is healthy

- **WHEN** a Job has a `Complete` condition with status `True`
- **THEN** the resolver reports healthy

#### Scenario: Failed Job is unhealthy

- **WHEN** a Job has a `Failed` condition with status `True`
- **THEN** the resolver reports unhealthy

#### Scenario: Running Job is pending

- **WHEN** a Job has neither a `Complete` nor a `Failed` condition set to `True`
- **THEN** the resolver reports pending
