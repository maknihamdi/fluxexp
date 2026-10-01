# cli Specification

## Purpose
TBD - created by archiving change add-traversal-engine. Update Purpose after archive.
## Requirements
### Requirement: List command

The CLI SHALL provide a `list` command that lists Flux resources of a given kind
together with their health, so a user can discover starting points before
traversing. The user MUST be able to choose the kind and the scope (all
namespaces or a single namespace), and SHALL be able to filter the output to only
unhealthy resources. Each row MUST show the resource's namespace/name and health.

#### Scenario: List resources of a kind with health

- **WHEN** the user runs the list command for a kind across all namespaces
- **THEN** the command prints one row per resource, each showing its namespace/name and health status

#### Scenario: Filter to unhealthy only

- **WHEN** the user runs the list command with the unhealthy-only filter
- **THEN** only resources whose health is not healthy are printed

#### Scenario: Unresolvable kind is reported

- **WHEN** the user requests a kind that cannot be resolved to a cluster resource
- **THEN** the command exits non-zero with a clear message

### Requirement: Traverse command

The system SHALL provide a `fluxexp` CLI with a command that traverses the
resource graph from a starting resource. The user MUST be able to specify the
start resource by kind, namespace, and name; the command builds the root
reference in the `kubernetes` domain from those inputs. The command SHALL run the
traversal engine and print the resulting resource tree.

#### Scenario: User traverses from a Kustomization

- **WHEN** the user runs the traverse command specifying a Kustomization's kind, namespace, and name
- **THEN** the command connects to the cluster, resolves the graph starting from that Kustomization, and prints the resource tree

#### Scenario: Missing start resource is reported

- **WHEN** the user specifies a start resource that does not exist in the cluster
- **THEN** the command exits non-zero with a clear message naming the resource it could not find

### Requirement: Cluster connection via kubeconfig

The CLI MUST connect to the cluster using the standard kubeconfig resolution
(the `KUBECONFIG` environment variable or the default path), and SHALL allow
selecting the context. It MUST fail with a clear message when no cluster is
reachable.

#### Scenario: Default kubeconfig is used

- **WHEN** the user runs a command without specifying a kubeconfig path
- **THEN** the CLI resolves credentials from the standard kubeconfig location

#### Scenario: Unreachable cluster reported clearly

- **WHEN** the target cluster cannot be reached
- **THEN** the command exits non-zero with a message indicating the connection failure

### Requirement: Tree rendering with health

The command SHALL render the resolved graph as an indented tree. Each node MUST
show its domain, type, coordinates (e.g. namespace/name), and health status. Each
health value — healthy, unhealthy, **pending**, unknown and error — MUST carry its own
glyph, so pending is not confused with any other status. Error nodes MUST be visually
distinguishable and MUST show their failure reason. An **expandable** node MUST carry a
leading chevron marker, and non-expandable nodes MUST reserve the same width so that
labels stay column-aligned. A node's children MUST be listed in the shared three-tier
order: Flux objects, then other expandable children, then the rest. A node's
**dependencies** MUST be rendered before its children, each carrying a marker
distinguishing it from an expandable child, so a reader can tell what the node
requires from what it produces.

#### Scenario: Tree shows nodes with health

- **WHEN** the traversal produces a root with nested children of varying health
- **THEN** the output is an indented tree where each line shows the node's domain, type, coordinates, and health status

#### Scenario: Pending node has its own glyph

- **WHEN** the tree contains a pending node alongside healthy, unhealthy and unknown siblings
- **THEN** the pending node's glyph differs from all three

#### Scenario: Error node shows its reason

- **WHEN** the tree contains an error node
- **THEN** that node is rendered distinctly and its failure reason is shown inline

#### Scenario: Expandable node carries the chevron

- **WHEN** a rendered node is expandable
- **THEN** its line shows a chevron marker before the label, and a non-expandable sibling shows blank padding of the same width in its place

#### Scenario: Children are printed in tier order

- **WHEN** a node's children mix Flux objects, other expandable references, and leaves
- **THEN** the Flux objects are printed first, then the other containers, then the leaves, each tier keeping its original relative order

#### Scenario: Dependencies are rendered before children with their own marker

- **WHEN** a Kustomization declares a source dependency and applies inventory children
- **THEN** the source is printed first, carrying the dependency marker rather than the expandable chevron, followed by the children in tier order

#### Scenario: Dependency fields are shown inline

- **WHEN** a rendered dependency is a GitRepository carrying repository, branch and interval fields
- **THEN** those fields appear on its line without any further interaction

### Requirement: UI command

The CLI SHALL provide a `ui` command that starts the local portal server and
serves the embedded web app. It MUST allow choosing the bind address/port
(defaulting to a loopback address) and SHOULD print the URL to open. Like the
other commands it resolves the cluster via the standard kubeconfig.

#### Scenario: `ui` starts the local portal

- **WHEN** the user runs the `ui` command
- **THEN** a local HTTP server starts on the default loopback address and the command prints the URL to open

#### Scenario: Bind address is configurable

- **WHEN** the user runs the `ui` command with an explicit address/port
- **THEN** the portal is served on that address

### Requirement: Freshness in the tree

The traverse command SHALL render a node's freshness next to its health when
present, and fold the node's key fields (applied revision, synced time, and — on
failure — the attempted revision) into the rendered line. A node with no
freshness MUST render as before (health only).

#### Scenario: Kustomization line shows health and freshness

- **WHEN** a healthy, up-to-date Kustomization is rendered
- **THEN** the line shows both its health and an up-to-date freshness indicator, plus the applied short revision and synced time

#### Scenario: Behind Kustomization shows both revisions

- **WHEN** a Kustomization is behind its source
- **THEN** the line shows a behind freshness indicator and both the applied and source short revisions

#### Scenario: Non-freshness node unchanged

- **WHEN** a node without freshness (e.g. a Deployment) is rendered
- **THEN** the line shows only its health, unchanged from before
### Requirement: The binary reports its own version

The CLI SHALL report its version on request, without a subcommand and without
reaching a cluster.

A distributed binary is the only artifact that carries no metadata a user can
read: the container image has its OCI labels, but a downloaded executable has
only its filename, which is lost the moment it is renamed or moved onto a PATH.
The version MUST therefore come from the binary itself.

The reported version MUST be the released version for an artifact produced by the
release pipeline. For any other binary it MUST report the version the Go
toolchain recorded for the module, rather than a placeholder — that covers both a
`go install <module>@v1.2.3`, where the recorded version is the published one, and
a build from a working copy, where the toolchain derives it from the version
control state and marks a modified tree.

A build carrying no recorded version at all MUST report a placeholder. It MUST
NOT report an empty string: a binary that cannot name itself is the failure this
requirement exists to prevent.

The precedence MUST be: the value injected at build time, then the version
recorded by the toolchain, then the placeholder.

A version reported for anything other than a released artifact MUST NOT be
mistakable for a released one. The toolchain's own marking of a modified tree
satisfies this, and is preferred over substituting a placeholder, which would
discard the information about which commit the binary came from.

#### Scenario: A released binary reports its version

- **WHEN** a binary from a release archive for `v1.4.2` is asked for its version
- **THEN** it reports `1.4.2`

#### Scenario: A toolchain install reports its version

- **WHEN** the CLI is installed with the Go toolchain from the published version `v1.4.2` and asked for its version
- **THEN** it reports `v1.4.2`, taken from the module version the toolchain recorded

#### Scenario: A local build reports the version control state

- **WHEN** the CLI is built from a working copy with no version injected
- **THEN** it reports the version the toolchain derived from version control, marked as modified when the tree has uncommitted changes, so it cannot be mistaken for a released artifact

#### Scenario: A build with no recorded version falls back to the placeholder

- **WHEN** the CLI is built with no injected version and no version information recorded at all
- **THEN** it reports the placeholder, and never an empty string

#### Scenario: The version needs no cluster

- **WHEN** the version is requested with no kubeconfig and no cluster reachable
- **THEN** it is reported and the command exits successfully

#### Scenario: The image agrees with its own labels

- **WHEN** the CLI inside a published image is asked for its version
- **THEN** it reports the same version as the image's OCI version label
