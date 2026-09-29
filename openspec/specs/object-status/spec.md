# object-status Specification

## Purpose
TBD - created by archiving change add-universal-object-status. Update Purpose after archive.
## Requirements

### Requirement: Every Kubernetes object carries a status

The system SHALL derive a status for **every** Kubernetes object it retrieves,
whatever its kind. An object MUST NOT be reported as **unknown** merely because it
carries no `Ready` condition. `unknown` SHALL be reserved for an object that
publishes state the system cannot interpret, never used as the default answer for
kinds the system has no rule for.

#### Scenario: An object without a Ready condition still gets a status

- **WHEN** the system derives health for an object whose `.status.conditions` contains no entry of type `Ready`
- **THEN** it reports healthy, unhealthy or pending according to the rules of this capability, and does not report unknown solely because `Ready` is absent

#### Scenario: A kind the system has no specific rule for still gets a status

- **WHEN** the system derives health for a custom resource of a kind it has no dedicated resolver for
- **THEN** it reports a status derived from the object's generic status shape, not unknown by default

### Requirement: Single shared status derivation

The Kubernetes status derivation SHALL live in exactly one function in the resolver
package, and every surface — the traversal tree, the `list` command, and the web
portal — MUST obtain health through it. No call site may reimplement condition
reading, replica arithmetic or generation comparison.

#### Scenario: All surfaces agree on an object's status

- **WHEN** the same object is shown by the CLI tree, by the `list` command and by the web portal
- **THEN** all three report the same health value and the same detail

### Requirement: Status derivation is based on kstatus

The derivation SHALL compute a base verdict with the `kstatus` library used by Flux
for its own health checks, mapping its outcomes as follows:

| kstatus outcome | health |
|---|---|
| `Current` | healthy |
| `InProgress` | pending |
| `Failed` | unhealthy |
| `Terminating` | pending |
| `NotFound` | unhealthy |

The message kstatus produces MUST be surfaced as the node's detail, so a reader sees
why the status is what it is.

#### Scenario: A standard workload verdict comes from kstatus

- **WHEN** the derivation runs on an available Deployment
- **THEN** it reports healthy with kstatus's message as the detail

#### Scenario: A failed object is unhealthy

- **WHEN** the derivation runs on an object whose `Stalled` condition is `True`
- **THEN** it reports unhealthy, with the condition's message as the detail

### Requirement: Objects with no status are healthy

An object carrying no `.status` at all MUST be reported **healthy** — a ConfigMap,
Secret, ServiceAccount, Role, RoleBinding, ClusterRole, ClusterRoleBinding,
NetworkPolicy, webhook configuration or comparable static resource. Such an object
cannot be unhealthy: having been retrieved, it exists and is applied, which is the
whole of what can be known about it.

#### Scenario: A ConfigMap is healthy

- **WHEN** the derivation runs on a ConfigMap that has no `.status` field
- **THEN** it reports healthy

#### Scenario: A ClusterRole is healthy

- **WHEN** the derivation runs on a ClusterRole that has no `.status` field
- **THEN** it reports healthy, not unknown

### Requirement: Generation drift is pending

An object whose `metadata.generation` is ahead of its `status.observedGeneration` MUST
be reported **pending**, with a detail naming both generations, because its status
describes a superseded spec. It MUST NOT be reported unhealthy on the strength of a
status the controller has not yet refreshed.

#### Scenario: Unobserved spec change reads pending

- **WHEN** the derivation runs on an object whose `metadata.generation` is 4 and whose `status.observedGeneration` is 1
- **THEN** it reports pending with a detail naming generation 4 and observed generation 1

#### Scenario: An observed spec change does not read pending

- **WHEN** the derivation runs on an object whose `metadata.generation` equals its `status.observedGeneration`
- **THEN** the generation comparison contributes nothing and the status comes from the object's conditions

#### Scenario: A reconciling object reads pending

- **WHEN** the derivation runs on an object whose `Reconciling` condition is `True`
- **THEN** it reports pending

### Requirement: A Ready condition that is False is unhealthy

`Ready` with status `False` MUST be reported **unhealthy**, overriding the base
verdict, which classifies it as in-progress. This is a deliberate divergence from
`flux`, confined to this rule: an object that declares itself not ready is shown as
broken, not as still converging.

#### Scenario: A failing Certificate is unhealthy

- **WHEN** the derivation runs on a Certificate whose `Ready` condition is `False` with a failure message
- **THEN** it reports unhealthy with that message as the detail

#### Scenario: A ready object is healthy

- **WHEN** the derivation runs on an object whose `Ready` condition is `True`
- **THEN** it reports healthy

### Requirement: Unrecognised conditions are read by polarity

The system MUST read an object's conditions through a polarity table before accepting
a **generic** current verdict, because the base verdict recognises only the
`Reconciling`, `Stalled` and `Ready` condition types and reports anything else as
current. The table classifies:

- **positive-polarity** types (`Synced`, `Configured`, `Established`, `Available`,
  `Healthy`, `Succeeded`, `Complete`, and comparable types) are unhealthy when their
  status is `False`;
- **negative-polarity** types (`Degraded`, `Stalled`, `Failed`, and comparable types)
  are unhealthy when their status is `True`.

A condition's message MUST be surfaced as the detail when it determines the status.
The table MUST only override a *generic* current verdict: where the base verdict comes
from a kind-specific rule or from a condition it recognises, that verdict stands.
Polarity is required because the sign of a condition is not derivable from its status:
`Degraded=False` is good news and `Synced=False` is bad news.

#### Scenario: A Bundle that is not synced is unhealthy

- **WHEN** the derivation runs on a Bundle whose only condition is `Synced` with status `False`
- **THEN** it reports unhealthy with that condition's message, rather than healthy

#### Scenario: A resource that is not configured is unhealthy

- **WHEN** the derivation runs on a custom resource whose only condition is `Configured` with status `False`
- **THEN** it reports unhealthy

#### Scenario: A negative-polarity condition that is False is good news

- **WHEN** the derivation runs on an object whose only condition is `Degraded` with status `False`
- **THEN** it reports healthy, because a negative-polarity condition is only unhealthy when true

#### Scenario: An established CRD is healthy

- **WHEN** the derivation runs on a CustomResourceDefinition whose `Established` condition is `True`
- **THEN** it reports healthy

#### Scenario: Polarity does not override a kind-specific verdict

- **WHEN** the base verdict for an object comes from a kind-specific rule rather than from the generic current fallback
- **THEN** that verdict stands and the polarity table is not consulted
