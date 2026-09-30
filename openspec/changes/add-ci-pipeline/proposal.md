## Why

Ten increments have shipped with no automation whatsoever: `make vet` and
`make test` are run by hand, on one machine, and nothing proves that what is on
`main` compiles. There is also no artifact — `fluxexp` exists only as a binary in
someone's `bin/`, so the portal cannot be deployed anywhere. Both gaps have the
same fix, and it is the prerequisite for the CD half: a container image built
from a tag is what the manifests will reference.

## What Changes

- A **GitHub Actions workflow** runs on every push to `main` and every pull
  request: formatting check, `go vet`, `go test ./...`, `go build`. A red check
  blocks nothing today (no branch protection is claimed here) but it is visible
  on the PR, which is the whole point.
- The Go toolchain version is taken **from `go.mod`**, not pinned a second time
  in the workflow. Two places to bump is one place too many.
- A **`Dockerfile`** at the repo root: multi-stage, `CGO_ENABLED=0` static
  binary, final image from a distroless static base running as non-root. The
  binary talks to an API server and reads nothing from disk except a kubeconfig
  that will not exist in a pod, so there is nothing for a shell to be useful for.
- On a **tag** matching `v*`, the same workflow builds the image and pushes it to
  `quay.io/hamdi_makni/fluxexp`, tagged from the semver tag (`1.2.3`, `1.2`,
  `latest`). Nothing is pushed on a branch or a pull request — an image exists
  because someone tagged a release, not because someone merged.
- The image carries **OCI labels** (source revision, version, created) so a
  running pod can be traced back to a commit without the binary learning a
  `--version` flag it does not have.
- Registry credentials come from repository secrets (`QUAY_USERNAME`,
  `QUAY_TOKEN`), expected to be a Quay **robot account** scoped to write that one
  repository.

Deliberately excluded from this increment:
- **Multi-architecture images.** `linux/amd64` only. Cross-building `arm64`
  through QEMU multiplies the build time for a platform no target cluster uses
  today; `docker/build-push-action` gains it by one `platforms:` line when one
  does.
- **A release job** (GitHub Release, binary attachments, changelog). The ask is
  an image; binaries for humans are a separate decision.
- **Coverage reporting, linters beyond `go vet`, dependency scanning.** `vet` and
  the tests are what the repo already runs; adding golangci-lint to eleven
  increments of existing code means triaging its backlog in this change.
- **Branch protection / required checks.** Repository settings, not a file in the
  repo — worth doing, but not something this change can deliver.

## Capabilities

### New Capabilities
- `ci-pipeline`: what the automated pipeline verifies on a change, when a
  container image is produced, where it is published and how it is tagged, and
  what that image contains.

### Modified Capabilities
<!-- none: no existing behaviour of the CLI, the engine or the portal changes -->

## Impact

- New `.github/workflows/ci.yml` — the only workflow.
- New `Dockerfile` and `.dockerignore` at the repo root.
- No Go code changes, no dependency changes, no behaviour change in the CLI or
  the portal. `make build` / `make test` keep working untouched; the workflow
  calls the same targets rather than restating the commands.
- Requires two repository secrets to exist before the first tag is pushed;
  without them the validation jobs still pass and only the publish job fails.
- Consumed by the follow-up change `add-cd-manifests`, which references
  `quay.io/hamdi_makni/fluxexp` at a released tag.