# flux-object-details Specification

## Purpose
Surfacing what Flux source and image-automation objects actually say — repository, tracked ref, interval, scanned image, selected tag — instead of leaving them to the generic Ready-condition fallback.

## Requirements
### Requirement: Flux source and image resolver

The system SHALL provide a resolver claiming the Flux **source**
(`source.toolkit.fluxcd.io`) and **image automation**
(`image.toolkit.fluxcd.io`) kinds in the `kubernetes` domain. It MUST report
health from the object's Ready condition, MUST declare itself non-expandable,
MUST return no children, and MUST be registered ahead of the generic Kubernetes
fallback so those kinds no longer fall through to it.

#### Scenario: Resolver claims source and image kinds

- **WHEN** the registry tests a GitRepository reference and, separately, a ConfigMap reference against this resolver
- **THEN** the matcher claims the GitRepository and rejects the ConfigMap

#### Scenario: Health still comes from the Ready condition

- **WHEN** the resolver resolves a GitRepository whose Ready condition is False with a message
- **THEN** it reports unhealthy and carries that message

#### Scenario: Source objects are leaves

- **WHEN** a source or image object is resolved
- **THEN** it returns no children and reports not expandable

### Requirement: Source repository fields

For a `GitRepository` the resolver MUST surface the repository URL, the tracked
git reference, and the reconciliation interval as fields. The tracked reference
MUST be whichever of branch, tag, semver or commit is configured. For an
`OCIRepository` it MUST surface the URL, the tracked tag/semver/digest and the
interval; for a `Bucket`, the bucket name, the endpoint and the interval; for a
`HelmRepository`, the URL, the repository type and the interval.

#### Scenario: GitRepository exposes repo, branch and interval

- **WHEN** a GitRepository specifies a URL, a branch, and an interval
- **THEN** the node carries a field for the repository URL, one for the branch, and one for the interval

#### Scenario: Tag-tracking GitRepository exposes the tag

- **WHEN** a GitRepository tracks a tag rather than a branch
- **THEN** the tracked-reference field reports the tag

#### Scenario: Bucket exposes its bucket and endpoint

- **WHEN** a Bucket specifies a bucket name and an endpoint
- **THEN** the node carries a field for each, plus the interval

### Requirement: Image automation fields

For an `ImageRepository` the resolver MUST surface the scanned image and, when
present, the last scan result. For an `ImagePolicy` it MUST surface the policy
rule and, when present, the currently selected tag.

#### Scenario: ImageRepository exposes the scanned image

- **WHEN** an ImageRepository specifies an image and its status reports a last scan
- **THEN** the node carries a field for the image and one for the last scan

#### Scenario: ImagePolicy exposes rule and selected tag

- **WHEN** an ImagePolicy defines a policy rule and its status reports a selected tag
- **THEN** the node carries a field for the rule and one for the selected tag

### Requirement: Missing fields degrade silently

Every field SHALL be optional. When the underlying path is absent — an
unconfigured option, or a schema shift between Flux API versions — the resolver
MUST omit that field rather than emitting an empty value or an error, so the
node degrades to health-only rather than breaking.

#### Scenario: Absent paths produce no field

- **WHEN** a GitRepository has no interval and no ref configured
- **THEN** the node carries only the fields that are present, and resolution succeeds

#### Scenario: Unknown Flux kind still resolves

- **WHEN** a claimed kind exposes none of the expected paths
- **THEN** the resolver reports health with no fields and no error

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
