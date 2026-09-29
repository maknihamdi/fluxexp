## ADDED Requirements

### Requirement: A HelmChart declares the repository it pulls from

A HelmChart SHALL report its `spec.sourceRef` as a dependency, defaulting an
omitted namespace to the HelmChart's own. It is the one Flux source kind with a
source of its own: a HelmRelease using `spec.chartRef` points at a HelmChart, and
the repository is one hop further — without this the chain stops at the chart and
the reader never learns where it comes from.

The HelmChart MUST remain a leaf: declaring a dependency does not give it
children and does not make it expandable.

#### Scenario: The chart's repository is grouped with it

- **WHEN** a HelmChart declaring `spec.sourceRef` to a HelmRepository is opened
- **THEN** that HelmRepository is shown as its dependency, with its own health and fields

#### Scenario: A chartRef release reaches its repository

- **WHEN** a HelmRelease declares `spec.chartRef` to a HelmChart which itself declares a HelmRepository
- **THEN** opening the release shows the HelmChart grouped with it, and the HelmRepository grouped with the HelmChart

#### Scenario: An omitted namespace defaults to the chart's own

- **WHEN** a HelmChart in namespace `apps` declares a source carrying no namespace
- **THEN** the reference is reported in namespace `apps`

#### Scenario: Declaring a dependency does not make it expandable

- **WHEN** a HelmChart with a source is resolved
- **THEN** it still returns no children and still reports not expandable

#### Scenario: A chart without a source declares nothing

- **WHEN** a HelmChart declares no `spec.sourceRef`
- **THEN** it reports no dependencies and resolves normally

### Requirement: HelmChart fields

A HelmChart SHALL surface the chart it pulls, its version constraint, the source
it pulls from (as kind and name) and its interval, from the object already
retrieved for its health.

The source field matters where the dependency grouping stops: a HelmChart listed
one level down shows no nested dependency of its own, so this field is what tells
the reader which repository the chart comes from.

#### Scenario: A HelmChart shows its chart, version and source

- **WHEN** a HelmChart pulling chart `cluster` at version `*` from a HelmRepository named `infra-repo` is shown
- **THEN** its row carries the chart, the version, the source as `HelmRepository/infra-repo`, and the interval

#### Scenario: An unconfigured field is absent

- **WHEN** a HelmChart declares no version
- **THEN** no version field is shown, and the other fields are unaffected
