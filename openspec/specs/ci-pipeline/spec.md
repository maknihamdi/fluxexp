# ci-pipeline Specification

## Purpose
TBD - created by archiving change add-ci-pipeline. Update Purpose after archive.
## Requirements
### Requirement: Every change is validated automatically

The repository SHALL run an automated pipeline on every push to the default
branch and on every pull request targeting it, verifying — in this order —
that the code is `gofmt`-clean, that `go vet` reports nothing, that the whole
test suite passes, and that the binary builds.

The pipeline MUST obtain its Go toolchain version from `go.mod`, which is the
single declaration of that version in the repository. It MUST NOT restate the
build, vet or test commands: it invokes the same `Makefile` targets a developer
invokes locally, so the pipeline and the local workflow cannot diverge.

The suite MUST run with no cluster and no kubeconfig available, as the tests are
built to.

#### Scenario: A pull request is validated

- **WHEN** a pull request is opened or updated against the default branch
- **THEN** the pipeline runs the formatting check, `vet`, the tests and the build, and its result is reported on the pull request

#### Scenario: A push to the default branch is validated

- **WHEN** a commit is pushed to the default branch
- **THEN** the same checks run against it

#### Scenario: Unformatted code fails the pipeline

- **WHEN** any Go file in the repository is not `gofmt`-clean
- **THEN** the pipeline fails and names the offending files

#### Scenario: A failing test fails the pipeline

- **WHEN** any test in `go test ./...` fails
- **THEN** the pipeline fails and no image is published

#### Scenario: The toolchain version is not declared twice

- **WHEN** the Go version required by `go.mod` changes
- **THEN** the pipeline uses the new version with no edit to the pipeline definition

### Requirement: The container image

The repository SHALL define a container image built from a multi-stage build that
compiles a **statically linked** binary (`CGO_ENABLED=0`) and ships it on a
minimal base carrying CA certificates and a non-root user, with no shell and no
package manager.

The base MUST provide CA certificates: the binary's only outbound traffic is
HTTPS to a Kubernetes API server, and a base without them turns every request
into a certificate error. `CGO_ENABLED=0` is required rather than incidental,
because a static base has no system C library for cgo's resolver to call.

The image MUST run as a non-root user by default, and MUST contain the embedded
web assets — they are compiled into the binary, so the image build MUST require
no frontend toolchain or asset step.

The image MUST carry OCI labels recording the source repository, the commit
revision, the version and the build time, so a running container can be traced
back to the commit it was built from.

#### Scenario: The image runs the portal

- **WHEN** the image is run with the `ui` command and an address on all interfaces
- **THEN** the portal serves its embedded frontend

#### Scenario: The image runs as non-root

- **WHEN** the image is inspected
- **THEN** its default user is not root

#### Scenario: The image has no shell

- **WHEN** a shell is invoked inside the image
- **THEN** there is none to invoke

#### Scenario: The image can reach an HTTPS API server

- **WHEN** the binary in the image connects to a Kubernetes API server over HTTPS
- **THEN** the certificate chain verifies, because the base carries CA certificates

#### Scenario: A running image is traceable to a commit

- **WHEN** the labels of a published image are read
- **THEN** they name the source repository, the commit revision and the version it was built from

#### Scenario: The image build is verified without publishing

- **WHEN** the pipeline validates a pull request or a branch push
- **THEN** the image is built to prove it still builds, and is not pushed anywhere

### Requirement: An image is published for a released tag only

The pipeline SHALL publish a container image to `quay.io/hamdi_makni/fluxexp`
when, and only when, a tag matching `v*` is pushed. Publication MUST be
conditional on the validation checks having passed for that same ref.

No image may be published for a branch push or a pull request: an image exists
because a release was tagged, so that anything deployed can be reproduced from a
tag.

The published tags MUST be derived from the semantic version of the git tag — the
full version, its major.minor prefix, and a `latest` pointer moved only when the
tag is the highest version released. Registry credentials MUST come from
repository secrets, and the pipeline MUST request no repository write permission,
since it writes only to the registry.

#### Scenario: A version tag publishes an image

- **WHEN** the tag `v1.4.2` is pushed
- **THEN** the image is pushed to `quay.io/hamdi_makni/fluxexp` tagged `1.4.2`, `1.4` and `latest`

#### Scenario: Validation gates publication

- **WHEN** a tag is pushed and a validation check fails for that commit
- **THEN** no image is pushed

#### Scenario: A merge to the default branch publishes nothing

- **WHEN** a commit is pushed to the default branch without a tag
- **THEN** the checks run and no image is pushed

#### Scenario: A patch tagged on an older version does not move latest

- **WHEN** `v1.3.5` is tagged while `v1.4.2` has already been released
- **THEN** the image is published as `1.3.5` and `1.3`, and `latest` keeps pointing at `1.4.2`

#### Scenario: Missing credentials fail only the publication

- **WHEN** the registry secrets are absent and a tag is pushed
- **THEN** the validation checks still pass and the publication step fails with a login error
