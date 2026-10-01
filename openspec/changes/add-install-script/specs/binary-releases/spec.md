## MODIFIED Requirements

### Requirement: Installation is discoverable from the README

The README SHALL tell a newcomer how to obtain and run the tool without cloning
the repository, and MUST NOT make them supply what their machine already knows.

Specifically:

- It MUST NOT name a version. A version written in prose goes stale at the next
  release and then instructs the reader to fetch an older artifact, or one that no
  longer exists. Every documented path MUST resolve the current version itself.
- Detecting the operating system and the processor architecture MUST NOT be the
  reader's job. A reader choosing their own architecture gets it wrong in the one
  case that matters: an Apple Silicon machine whose Go toolchain is an amd64
  build reports `amd64` through the Go environment while the hardware is `arm64`.
- Verification MUST be shown in a form that works on every platform the releases
  target. A single checksum command does not: the tool is named `sha256sum` on
  Linux and `shasum -a 256` on macOS, so a recipe naming only one fails half the
  audience at the step that matters most.
- The toolchain install MUST be offered first where it applies, because for a
  reader who already has the Go toolchain it is one command that needs no
  platform detection and covers every platform, including the one the shell
  installer does not.
- A platform with no published archive MUST be named as such rather than left to
  produce a download failure.

The README SHALL also show the pipeline's state and the version currently
released.

The pipeline-state indicator MUST be described for what it reports — the latest
run on the default branch — because it cannot report the run of a tag, and a
reader would otherwise take a green indicator as proof that the released version
built.

#### Scenario: A newcomer installs without cloning

- **WHEN** someone reads the README having never seen the repository
- **THEN** they find the download, verification and toolchain instructions for their platform

#### Scenario: No instruction names a version

- **WHEN** a new version is released and the README is not edited
- **THEN** every documented install path still obtains the newest version

#### Scenario: The reader is not asked for their platform

- **WHEN** a reader on Linux or macOS follows the primary instruction
- **THEN** the operating system and architecture are determined for them, from the machine rather than from the Go environment

#### Scenario: Verification works on macOS as well as Linux

- **WHEN** a reader on either platform follows the verification step
- **THEN** it succeeds using a checksum tool that exists on that platform

#### Scenario: An unsupported platform is told so

- **WHEN** the primary instruction runs on a platform with no published archive
- **THEN** it reports that the platform is unsupported and lists the published ones, rather than failing on a download

#### Scenario: The released version is visible

- **WHEN** the README is displayed
- **THEN** it shows the version currently released and the pipeline's state

#### Scenario: The indicator does not overstate what it covers

- **WHEN** a reader looks at the pipeline-state indicator
- **THEN** the README states that it reflects the default branch, not the released tag

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

The release SHALL also carry the **installer script** as an asset, covered by that
release's checksum file like every other asset. The convenience of a one-line
install requires fetching the script from a mutable branch; publishing it per
release gives an immutable copy whose integrity can be checked, so the script that
verifies downloads is itself verifiable, and a pinned installation can fetch a
script that will not change under it.

**Exactly one job MUST create the release**, after every target has been built.
Per-target publication is forbidden: concurrent jobs creating the same release
lose assets, and a release missing two of its five archives is
indistinguishable from a complete one.

The binaries MUST be statically linked and cross-compiled from a single runner —
no target may require a native runner or a C toolchain.

#### Scenario: A version tag publishes five archives

- **WHEN** the tag `v1.4.2` is pushed
- **THEN** a GitHub Release for `v1.4.2` carries five archives, one per supported target

#### Scenario: The installer is published with the release

- **WHEN** a release's assets are listed
- **THEN** the installer script is among them, and its checksum is in that release's checksum file

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

## ADDED Requirements

### Requirement: The installer refuses rather than degrades

The installer script SHALL verify the archive it downloaded against the release's
published checksum before installing anything, and MUST abort when it cannot.

It MUST NOT fall back to installing an unverified binary when no checksum tool is
available: this binary reads the caller's kubeconfig and talks to their clusters
with their credentials, so verification is the part of the distribution that
matters most, and a step that silently stops happening is worse than one that
fails loudly.

Verification MUST compare only the entry for the archive actually downloaded.
Checking the whole checksum file fails on the archives that were not downloaded,
which turns a correct download into an error.

The script MUST leave nothing behind on failure: downloads go to a temporary
directory removed on exit, never into the caller's working directory.

It MUST NOT require privilege it does not need: elevation is for writing to the
install directory, and only when that directory is not already writable.

#### Scenario: A verified archive installs

- **WHEN** the downloaded archive matches its published checksum
- **THEN** the binary is installed and reports the released version

#### Scenario: A corrupted download does not install

- **WHEN** the downloaded archive does not match its published checksum
- **THEN** the script aborts and installs nothing

#### Scenario: No checksum tool means no install

- **WHEN** neither checksum tool is available on the machine
- **THEN** the script aborts rather than installing an unverified binary

#### Scenario: Only the relevant checksum entry is checked

- **WHEN** the checksum file lists every archive in the release and one has been downloaded
- **THEN** verification succeeds, rather than failing over the archives that were not downloaded

#### Scenario: A user-writable destination needs no privilege

- **WHEN** the install directory is writable by the user
- **THEN** the installation completes without requesting elevation

#### Scenario: Nothing is left in the working directory

- **WHEN** the script finishes, successfully or not
- **THEN** no archive, checksum file or extracted binary remains in the directory it was run from
