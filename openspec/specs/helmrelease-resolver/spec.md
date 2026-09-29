# helmrelease-resolver Specification

## Purpose
TBD - created by archiving change add-helmrelease-resolver. Update Purpose after archive.
## Requirements
### Requirement: HelmRelease matcher

The system SHALL provide a resolver for the Flux HelmRelease type in the
`kubernetes` domain (`helm.toolkit.fluxcd.io`, kind `HelmRelease`). Its matcher
MUST claim references whose domain is `kubernetes` and whose type is a
HelmRelease, and MUST NOT claim other references.

#### Scenario: Matcher claims HelmRelease references only

- **WHEN** the registry tests a `kubernetes` HelmRelease reference and, separately, a Deployment reference against this resolver
- **THEN** the matcher claims the HelmRelease reference and rejects the Deployment reference

### Requirement: Locate the Helm release storage

The HelmRelease resolver MUST locate the current Helm release from the
HelmRelease object's `.status.history`, using the newest entry to obtain the
release `name`, storage `namespace`, and `version`. From these it MUST target
the storage Secret named `sh.helm.release.v1.<name>.v<version>` in the storage
namespace. When the history is absent or empty, the resolver MUST return no
children without erroring (the release has not been stored yet).

#### Scenario: Newest history entry selects the storage secret

- **WHEN** a HelmRelease has history whose newest entry is name `web`, namespace `apps`, version `7`
- **THEN** the resolver targets the Secret `sh.helm.release.v1.web.v7` in namespace `apps`

#### Scenario: No history yields no children

- **WHEN** a HelmRelease has no `.status.history` entries
- **THEN** the resolver returns no children and does not error

### Requirement: Decode the Helm release payload

The resolver MUST decode the storage Secret's `release` value through the Helm
Secrets-driver encoding: Kubernetes base64, then Helm base64, then gzip
decompression, yielding the release JSON. A payload that cannot be decoded MUST
surface as a resolve error for that node.

#### Scenario: Encoded payload is decoded to the release object

- **WHEN** the storage Secret holds a `release` value encoded as base64(base64(gzip(json)))
- **THEN** the resolver decodes it to the release object and reads its `manifest`

#### Scenario: Corrupt payload is an error

- **WHEN** the storage Secret's `release` value is not valid base64/gzip/JSON
- **THEN** the resolver returns an error for that node (which the engine renders as an error node)

### Requirement: Manifest objects become children

The resolver MUST parse the release `manifest` as a multi-document YAML stream
and emit one `kubernetes` child reference per rendered object, carrying the
object's type (apiVersion/kind) and coordinates (namespace, name). Namespace
defaulting MUST be scope-aware: a **namespaced** object with no explicit
namespace MUST inherit the release namespace, while a **cluster-scoped** object
MUST keep an empty namespace. When the scope cannot be determined, the object
MUST be treated as namespaced. Empty documents MUST be skipped.

#### Scenario: Rendered objects become child references

- **WHEN** the release manifest renders a Deployment and a Service in namespace `apps`
- **THEN** the resolver returns two `kubernetes` child references identifying that Deployment and Service in `apps`

#### Scenario: Namespaced object inherits the release namespace

- **WHEN** a namespaced rendered object declares no namespace and the release namespace is `apps`
- **THEN** the corresponding child reference has namespace `apps`

#### Scenario: Cluster-scoped object stays namespace-less

- **WHEN** a cluster-scoped rendered object (e.g. a ClusterRole or CustomResourceDefinition) declares no namespace
- **THEN** the corresponding child reference has an empty namespace and does not inherit the release namespace

### Requirement: HelmRelease health

The resolver MUST report the HelmRelease's health from its `.status.conditions`
`Ready` condition (`True` healthy, `False` unhealthy, absent unknown), and SHOULD
surface the release status as detail.

#### Scenario: Ready True is healthy

- **WHEN** the HelmRelease's `Ready` condition has `status: "True"`
- **THEN** the resolver reports the node as healthy

#### Scenario: Ready False is unhealthy

- **WHEN** the HelmRelease's `Ready` condition has `status: "False"`
- **THEN** the resolver reports the node as unhealthy and surfaces the condition message as detail

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
