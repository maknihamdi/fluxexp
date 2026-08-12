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

