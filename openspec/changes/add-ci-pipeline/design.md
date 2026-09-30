## Context

The repo has a `Makefile` with `build`, `test`, `vet`, `tidy` and no automation.
Constraints that shape everything below:

- **Go 1.26.0**, declared in `go.mod`. That declaration is the only version
  statement in the repo and must stay the only one.
- **The frontend is embedded** (`//go:embed web/*` in `internal/ui`). There is no
  node toolchain, no asset pipeline, no build step before `go build` — the image
  build is a Go build and nothing else.
- **The binary needs no shell, no libc, no files.** It talks to an API server
  over HTTPS and serves an embedded SPA. Its only filesystem read is a kubeconfig,
  which does not exist in a pod (`clientcmd`'s deferred loader falls back to the
  in-cluster config when it finds none — see `internal/k8s/client.go`).
- **Tests need no cluster.** `internal/resolver/fake_test.go` holds an in-memory
  getter; the UI injects a fake cluster through `newService`. A runner with no
  kubeconfig runs the whole suite.
- The publish target is `quay.io/maknihamdi`, i.e. a personal Quay namespace,
  which means a robot account and a token secret rather than any OIDC federation.

## Goals / Non-Goals

**Goals:**

- Every push and pull request proves the code formats, vets, tests and builds.
- Every `v*` tag produces one immutable image at `quay.io/maknihamdi/fluxexp`,
  traceable back to its commit.
- One workflow file, one Dockerfile, no Go code touched.

**Non-Goals:**

- Multi-arch images, GitHub Releases, binary artifacts, changelogs.
- Linters beyond `go vet`, coverage gates, dependency or image scanning.
- Deploying anything. The image is the handoff point to `add-cd-manifests`.
- Branch protection and required-check configuration (repository settings).

## Decisions

### One workflow, two jobs, gated by the ref

`.github/workflows/ci.yml` with `validate` (always) and `publish` (only when
`github.ref_type == 'tag'`, `needs: validate`).

Alternative: two files, `ci.yml` and `release.yml`. Rejected — a tag push would
then run validation in one workflow and publication in another with no ordering
between them, so an image could be pushed while the tests it was built from were
still running, or failing. `needs:` in one file is the cheap way to say "never
publish something that did not pass".

### The toolchain version comes from `go.mod`

`actions/setup-go` with `go-version-file: go.mod`, plus its built-in module and
build caching (`cache: true` is the default when a `go.sum` is found). Pinning
`go-version: '1.26'` in the workflow would be a second place to bump, and the two
would drift the first time someone bumps only one.

The Dockerfile has the same problem and does **not** get the same solution: a
`FROM golang:...` tag cannot read `go.mod`. It is pinned there once, and the
tasks make bumping it part of any toolchain bump.

### The workflow calls `make`, not `go`

`make vet`, `make test`, `make build`. The Makefile already defines what those
mean; restating `go test ./...` in YAML creates a second definition that can
disagree with the one a developer runs locally. The one command with no Make
target is the formatting check, which is a shell line:

```
test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }
```

Alternative: `gofmt -d` and let a diff be the output. Rejected as noisier for the
same signal; the file list is what a developer needs to act.

Alternative: skip formatting entirely, since `go vet` does not care. Rejected —
`gofmt` is the one style question that is never a matter of opinion in Go, and
it is one line.

### Build with `docker/build-push-action`, tag with `docker/metadata-action`

The tag derivation is the part worth not writing by hand: `metadata-action` with
`type=semver,pattern={{version}}` and `{{major}}.{{minor}}` turns the tag `v1.4.2`
into `1.4.2` and `1.4`, and it also emits the OCI labels
(`org.opencontainers.image.revision`, `.version`, `.created`, `.source`) that let
a running pod be traced to a commit. Hand-rolled `${GITHUB_REF#refs/tags/v}`
string surgery gets the simple case right and the prerelease case wrong.

**`latest` is the one tag the action cannot decide.** `metadata-action` sees only
the ref the run was triggered by; it queries neither the other tags in the
repository nor the registry, so it has no way to know that `v1.3.5` is a patch on
an older line than an already-released `v1.4.2`. Whatever its own `latest` rule
does, it cannot be that comparison.

So the comparison is computed, and the action's rule is turned off
(`flavor: latest=false`) so it cannot add the tag behind that decision's back. A
shell step takes the highest non-prerelease tag in the repository
(`git tag -l 'v*' --sort=-v:refname`, prereleases filtered out, first line) and
emits `latest=true` only when it is the tag being built; the `type=raw,value=latest`
rule is enabled from that output. It needs `fetch-depth: 0` on the checkout, which
is the only reason the publish job clones full history.

Six lines of shell, and they are the reason a patch release on an old line does
not silently repoint `latest` at older code. The manifests in `add-cd-manifests`
pin a version anyway; `latest` is for a human poking at the image, which is
exactly the reader who would be misled.

Alternative: `docker build` + `docker push` in two `run:` steps. Rejected for the
tagging reason above, not for the build itself.

### The image: distroless static, non-root, `CGO_ENABLED=0`

```
FROM golang:1.26 AS build       → CGO_ENABLED=0 go build -o /out/fluxexp ./cmd/fluxexp
FROM gcr.io/distroless/static-debian12:nonroot
```

`CGO_ENABLED=0` matters specifically here: with cgo, Go uses the system resolver
through glibc, and a distroless *static* base has no glibc. The pure-Go resolver
is also the one that reads `/etc/resolv.conf` the way a cluster expects.

Alternative bases considered:
- `alpine` — needs `GOFLAGS` nothing, but ships a shell and apk for no reason
  this binary has, and adds musl DNS quirks.
- `scratch` — one step further, but then CA certificates must be copied in by
  hand. The binary calls an HTTPS API server; distroless `static` already carries
  `ca-certificates`, `/etc/passwd` with a nonroot user, and tzdata. That is
  exactly the set this binary needs and nothing else.

`:nonroot` rather than adding a `USER` line: the base already defines uid 65532,
and the manifests in the CD change will assert it in `securityContext` rather
than trusting the image.

### Nothing is pushed outside a tag

No `main`-branch image, no PR image, no `sha-` tag on a branch build. The rule is
"an image exists because someone tagged a release". A `main` image invites
someone to deploy an untagged commit, and then the manifests point at something
that cannot be reproduced from a tag.

Consequence accepted: a broken Dockerfile is only discovered at tag time, since
the validate job does not build the image. Mitigated below.

### Credentials

`QUAY_USERNAME` / `QUAY_TOKEN` repository secrets, a Quay robot account with
write on `maknihamdi/fluxexp` only. `permissions: contents: read` at the workflow
level — nothing here writes to the repository, and the default token grant is
wider than that.

## Risks / Trade-offs

- **A Dockerfile that does not build is found only at tag time** → the validate
  job builds the image (`push: false`) on every run. It costs a build and removes
  the entire class of "the tag failed to publish". This is the one place where
  duplicating work is cheaper than the failure it prevents.
- **`gofmt -l` on the whole tree may fail on the first run** → `make vet` and the
  formatting check are run once locally before the workflow is committed, and any
  reformatting lands in this change rather than surprising the first PR.
- **A pinned `golang:1.26` in the Dockerfile drifts from `go.mod`** → the build
  fails loudly, because Go refuses a `go.mod` requiring a newer toolchain than
  the image provides. A drift the other way (image newer) is harmless.
- **`latest` on a personal namespace is a public, mutable pointer** → the CD
  manifests pin a version, never `latest`.
- **Secrets missing at the first tag** → the validate job still passes and only
  publication fails, with an explicit login error. The tasks make setting the
  secrets a step before the first tag.
- **No supply-chain attestation, no SBOM, no image signing.** Accepted for a
  personal-namespace tool; `build-push-action` gains provenance and SBOM by two
  input lines if the image ever goes somewhere that asks.

## Migration Plan

Nothing to migrate: no existing pipeline, no existing image. Order of operations:

1. Merge the workflow and the Dockerfile. The validate job starts running on PRs
   immediately and builds the image without pushing.
2. Create the Quay robot account and set the two secrets.
3. Push `v0.1.0` and confirm the image lands with its tags and labels.

Rollback is deleting the workflow file; nothing in the Go code depends on it.

## Open Questions

- The first tag's version number: `v0.1.0` (the tool is usable but the roadmap is
  open) versus `v1.0.0` (ten increments are shipped). Assumed `v0.1.0`; it only
  determines a string.
- Whether `main` should later get a moving image for a staging deployment. Left
  open deliberately — it is a CD question, and the CD change can answer it by
  asking for a `main` tag if it needs one.
