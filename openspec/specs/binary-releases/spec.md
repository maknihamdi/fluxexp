# binary-releases Specification

## Purpose
TBD - created by archiving change add-binary-releases. Update Purpose after archive.
## Requirements
### Requirement: Binaries are published for a released tag

The pipeline SHALL build `fluxexp` for `linux/amd64`, `linux/arm64`,
`darwin/amd64`, `darwin/arm64` and `windows/amd64` when a `v*` tag is pushed, and
attach the results to a GitHub Release for that tag.

Building MUST be conditional on the validation checks having passed for that same
ref: nothing is released from a commit whose tests did not pass.

Each target MUST be delivered as one archive named
`fluxexp_<version>_<os>_<arch>` with a `.tar.gz` extension for Linux and macOS
and `.zip` for Windows, containing the binary and the licence. The licence MUST
travel inside the archive, because a downloaded artifact carries no link back to
the repository.

**Exactly one job MUST create the release**, after every target has been built.
Per-target publication is forbidden: concurrent jobs creating the same release
lose assets, and a release missing two of its five archives is
indistinguishable from a complete one.

The binaries MUST be statically linked and cross-compiled from a single runner —
no target may require a native runner or a C toolchain.

#### Scenario: A version tag publishes five archives

- **WHEN** the tag `v1.4.2` is pushed
- **THEN** a GitHub Release for `v1.4.2` carries five archives, one per supported target

#### Scenario: Windows is delivered as a zip

- **WHEN** the Windows archive of a release is inspected
- **THEN** it is a `.zip` and contains `fluxexp.exe`

#### Scenario: The licence travels with the binary

- **WHEN** any release archive is extracted
- **THEN** it contains the licence alongside the binary

#### Scenario: Validation gates the release

- **WHEN** a tag is pushed and a validation check fails for that commit
- **THEN** no release is created and no binary is published

#### Scenario: A merge to the default branch releases nothing

- **WHEN** a commit is pushed to the default branch without a tag
- **THEN** the checks run and no release is created

#### Scenario: One release holds every archive

- **WHEN** the five builds finish at different times
- **THEN** a single release is created once they have all succeeded, carrying all five archives

### Requirement: A download can be verified

Every release SHALL carry one checksum file covering all of its archives, in the
format the standard `sha256sum -c` accepts.

One file for the whole release is required rather than one per archive: a
per-target file would be a single line, which is not what the verification tool
consumes, and it leaves the person comparing five files instead of running one
command.

This is not optional hygiene for this tool. The binary reads the caller's
kubeconfig and talks to their clusters with their credentials, so an unverifiable
download is the part of the distribution that matters most.

#### Scenario: Checksums cover the whole release

- **WHEN** a release is published
- **THEN** it carries one checksum file listing every archive in that release

#### Scenario: A downloaded archive verifies

- **WHEN** a user downloads an archive and the checksum file and runs the standard verification command
- **THEN** it reports the archive as matching

#### Scenario: A tampered archive fails verification

- **WHEN** a downloaded archive differs by one byte from the published one
- **THEN** verification reports a mismatch

### Requirement: Installation is discoverable from the README

The README SHALL tell a newcomer how to obtain and run the tool without cloning
the repository: where the releases are, how to download the archive for their
platform, how to verify it, and how to install it with the Go toolchain instead.

The README SHALL also show the pipeline's state and the version currently
released.

The pipeline-state indicator MUST be described for what it reports — the latest
run on the default branch — because it cannot report the run of a tag, and a
reader would otherwise take a green indicator as proof that the released version
built.

#### Scenario: A newcomer installs without cloning

- **WHEN** someone reads the README having never seen the repository
- **THEN** they find the download, verification and `go install` instructions for their platform

#### Scenario: The released version is visible

- **WHEN** the README is displayed
- **THEN** it shows the version currently released and the pipeline's state

#### Scenario: The indicator does not overstate what it covers

- **WHEN** a reader looks at the pipeline-state indicator
- **THEN** the README states that it reflects the default branch, not the released tag
