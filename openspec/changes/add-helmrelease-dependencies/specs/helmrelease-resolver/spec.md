## ADDED Requirements

### Requirement: HelmRelease dependencies

The HelmRelease resolver SHALL report, as dependencies, the references the
HelmRelease declares in its own manifest: its **source** first, then its
`spec.dependsOn` entries in declaration order.

The source MUST be the one declared. When `spec.chartRef` is present it is the
source, whatever kind it names (OCIRepository, HelmChart); otherwise the source
is `spec.chart.spec.sourceRef` (HelmRepository, GitRepository, Bucket). When both
are present — which the API treats as mutually exclusive — `spec.chartRef` MUST
win. When neither is present the HelmRelease MUST declare no source rather than
failing.

Each `spec.dependsOn` entry MUST be taken as another HelmRelease. On the source
reference and on every `dependsOn` entry, an omitted namespace MUST default to
the HelmRelease's own.

No declared reference may be pre-fetched to decide whether to report it: a
HelmRelease pointing at a missing source MUST surface that source as an error
node downstream, not omit it.

#### Scenario: The chart's repository is grouped with the release

- **WHEN** a HelmRelease declaring `spec.chart.spec.sourceRef` to a HelmRepository is resolved
- **THEN** that HelmRepository is reported as its dependency, with its own health

#### Scenario: dependsOn entries follow the source

- **WHEN** a HelmRelease declares a source and two `dependsOn` entries
- **THEN** its dependencies are the source first, then the two entries in declaration order

#### Scenario: chartRef takes precedence

- **WHEN** a HelmRelease declares both `spec.chartRef` to an OCIRepository and `spec.chart.spec.sourceRef` to a HelmRepository
- **THEN** the OCIRepository is reported as its source, and the HelmRepository is not

#### Scenario: Omitted namespaces default to the release's own

- **WHEN** a HelmRelease in namespace `apps` declares a source and a `dependsOn` entry, neither carrying a namespace
- **THEN** both references are reported in namespace `apps`

#### Scenario: Explicit namespaces are honoured

- **WHEN** a HelmRelease declares a source whose reference names another namespace
- **THEN** the source reference is reported in that namespace

#### Scenario: A missing source is reported, not omitted

- **WHEN** a HelmRelease names a HelmRepository that does not exist
- **THEN** the reference is still reported and surfaces downstream as an error node with its reason

#### Scenario: A HelmRelease without declarations has no dependencies

- **WHEN** a HelmRelease declares neither a chart source nor `dependsOn` entries
- **THEN** it reports no dependencies and resolves normally
