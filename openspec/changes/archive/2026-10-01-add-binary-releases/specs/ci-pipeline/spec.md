## MODIFIED Requirements

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
repository secrets.

The pipeline's **default permission MUST remain read-only**. Publishing the image
requires no repository write access, since it writes only to the registry, and the
image jobs MUST NOT request any. Where a write permission is genuinely required —
creating a release — it MUST be granted at the level of the single job that needs
it and never at the workflow level, so that the jobs handling registry
credentials and the jobs able to write to the repository remain disjoint sets.

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

#### Scenario: The workflow default stays read-only

- **WHEN** the pipeline definition is inspected
- **THEN** its workflow-level permission is read-only, and the image jobs request no write access

#### Scenario: Write access is scoped to the job that needs it

- **WHEN** a job must write to the repository to create a release
- **THEN** that permission is declared on that job alone, and every other job keeps the read-only default