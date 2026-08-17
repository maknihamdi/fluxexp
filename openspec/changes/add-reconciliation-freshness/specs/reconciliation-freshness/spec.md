## ADDED Requirements

### Requirement: Freshness status carrier

The node model SHALL carry a **freshness** status independent of health, drawn
from a fixed set: **up-to-date**, **behind**, **failed**, **suspended**, and an
empty value meaning "not applicable". The engine MUST NOT compute freshness; it
carries whatever a resolver sets, exactly as it carries health. Resolvers that do
not set freshness leave it empty.

#### Scenario: Freshness is carried through to the node

- **WHEN** a resolver returns a result with freshness `behind`
- **THEN** the resolved node exposes freshness `behind`, independent of its health value

#### Scenario: Resolvers that don't set freshness leave it empty

- **WHEN** a resolver returns a result without setting freshness
- **THEN** the node's freshness is empty and renderers omit any freshness indicator

### Requirement: Structured fields carrier

The node model SHALL carry an ordered list of **fields** (each a label and a
value) that a resolver MAY attach to surface extra information (e.g. revisions,
timestamps). The engine MUST preserve their order and carry them unchanged.

#### Scenario: Fields are carried in order

- **WHEN** a resolver attaches fields `[Applied, Synced, Source]` in that order
- **THEN** the node exposes exactly those fields in that order

### Requirement: Kustomization freshness

The system SHALL compute a Kustomization's freshness from cluster state only, by
fetching its source (the `spec.sourceRef` object) and comparing revisions:

- **suspended** when `spec.suspend` is true;
- otherwise **failed** when the Kustomization's Ready condition is False;
- otherwise **up-to-date** when `status.lastAppliedRevision` equals the source's
  `status.artifact.revision`;
- otherwise **behind** when the source has a revision that differs from the
  applied one.

When the source cannot be fetched, freshness MUST fall back to a health-only
value (up-to-date if Ready, else failed) rather than erroring.

#### Scenario: Applied equals source is up-to-date

- **WHEN** a Ready Kustomization's `lastAppliedRevision` equals its source's `artifact.revision`
- **THEN** its freshness is up-to-date

#### Scenario: Source ahead of applied is behind

- **WHEN** a Ready Kustomization's `lastAppliedRevision` differs from a newer source `artifact.revision`
- **THEN** its freshness is behind, and a field exposes both the applied and the source revision

#### Scenario: Not-Ready is failed

- **WHEN** a Kustomization's Ready condition is False
- **THEN** its freshness is failed and a field exposes the attempted revision and the message

#### Scenario: Suspended Kustomization

- **WHEN** a Kustomization has `spec.suspend` true
- **THEN** its freshness is suspended

### Requirement: Revision and time presentation

The system SHALL present revisions in a short human form (branch + short commit
sha) while keeping the full revision available, and SHALL present sync times both
relative (e.g. "3m ago") and absolute.

#### Scenario: Revision shortened but full retained

- **WHEN** an applied revision is `main@sha1:0123456789abcdef...`
- **THEN** the short form shown is `main@0123456` and the full revision remains available in the field value
