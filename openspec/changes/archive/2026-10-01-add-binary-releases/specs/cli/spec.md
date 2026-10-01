## ADDED Requirements

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
