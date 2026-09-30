## Context

What exists, measured on the `v0.1.0` release rather than assumed:

- `.github/workflows/ci.yml` has `validate` (gofmt, vet, test, build, unpushed
  image build) and `publish` (`needs: validate`, `if: github.ref_type == 'tag'`).
  Workflow-level `permissions: contents: read`.
- The tag run behaved as specified: `validate` 08:40:38→08:42:37, `publish`
  started 08:42:41, three tags on one digest
  `sha256:6b5e5585…`, `org.opencontainers.image.revision` equal to the tagged
  commit. The `latest` gate is a shell step, because `metadata-action` sees only
  the triggering ref.
- The published artifact is an **OCI image index**: `linux/amd64` plus an
  attestation manifest, from `build-push-action`'s default provenance.
- `org.opencontainers.image.licenses` came back **empty** — there is no `LICENSE`
  in the repository.
- `cmd/fluxexp/command/root.go` builds the root command with no `Version` field.
  `NewRootCmd` has exactly **one caller**, `cmd/fluxexp/main.go:14`.
- `CGO_ENABLED=0 GOOS=linux go build` produces a 36 MB statically linked binary,
  so nothing in the dependency tree drags in cgo and cross-compilation needs no C
  toolchain for any target.
- The frontend is embedded, so every target is a plain `go build` — there is no
  per-platform asset step and no reason a Windows build would differ.

## Goals / Non-Goals

**Goals:**

- A person on Linux, macOS (Intel or Apple Silicon) or Windows downloads one
  archive from a release and runs `fluxexp`.
- They can verify what they downloaded, and ask the binary what it is.
- The repository states a licence, so the artifacts are legally usable.
- One release per tag, carrying all five archives and one checksum file.

**Non-Goals:**

- GoReleaser, signing, SBOMs, package managers, a maintained changelog.
- More than five targets.
- Any change to traversal, health derivation or the portal.

## Decisions

### A matrix of `go build`, not GoReleaser

GoReleaser would replace all of the YAML below with one config file, and it is
what the Go ecosystem reaches for. It is not chosen here for one reason: it also
publishes container images, so adopting it means either **two release
mechanisms** in the same workflow or rewriting the image publication that was
just verified end to end on `v0.1.0` — including the `latest` gate that had to be
hand-written because no action could compute it.

The cost of the decision is about thirty lines of YAML that GoReleaser would have
owned. The benefit is that the release path stays one mechanism with one gate.
Revisit when there is a Homebrew tap or a generated changelog to justify it.

### Build in a matrix, release in one job

```
build-binaries (matrix ×5)  →  upload-artifact
                                     ↓
release (needs: build-binaries)  →  download-artifact → SHA256SUMS → one release
```

The tempting shape is `softprops/action-gh-release` inside the matrix, each job
attaching its own archive. Rejected: five jobs then race to *create* the same
release, and the loser either errors or attaches to a release the winner is still
assembling. The failure is not theoretical and it is ugly — a release with three
of five assets looks complete.

One job creating the release also means the checksum file covers **all** archives.
Computed per matrix job it would be five one-line files, which is not what
`sha256sum -c` expects.

The collecting job must **name what it collects**. Measured on the `v0.2.0` run:
the workflow produced seven artifacts, not five — `docker/build-push-action`
uploads a `.dockerbuild` build record of its own, once from `validate` and once
from `publish`. Downloading every artifact in the run therefore tried to fetch
those records and failed outright (`Artifact download failed after 5 retries`),
taking the whole release with it. A name pattern restricted to the archives fixes
it, and is the right shape regardless: what this job releases must not depend on
what other jobs happen to upload.

`build-binaries` needs `needs: validate` as well: the same rule as the image —
nothing is released from a commit whose tests have not passed.

### Cross-compilation needs no per-platform runner

All five targets build on `ubuntu-latest`. `CGO_ENABLED=0` is already how the
image is built and the binary is statically linked, so `GOOS`/`GOARCH` is the
whole of it. A macOS runner would be needed only for cgo or for code signing,
and there is neither.

Alternative: a matrix over `runs-on` with native builds. Rejected as five times
slower and five times the runner cost for an identical artifact.

### `contents: write` at the job, not the workflow

The workflow keeps `permissions: contents: read`. The `release` job declares
`permissions: contents: write` for itself. The `validate`, `publish` and
`build-binaries` jobs keep the read-only default.

This is the whole substance of the `ci-pipeline` spec change. The old requirement
said the pipeline requests no write permission at all, which was true and worth
stating; now that one job must write, the requirement has to say **where** the
elevation lives, because "the workflow can write to the repository" and "one job
that only creates a release can write" are very different claims.

### The version variable lives in `main`, and falls back to build info

`main.version` with `command.NewRootCmd(version string)` setting `cmd.Version`.
Cobra then creates `--version` itself; no flag is declared by hand.

Alternative: put the variable in the `command` package and inject
`-X github.com/maknihamdi/fluxexp/cmd/fluxexp/command.version=…`. Rejected — an
ldflags path that long is a string nobody validates, and it breaks silently if the
package moves. `-X main.version=…` is short enough to read. `NewRootCmd` has one
caller, so the signature change costs two lines.

The fallback matters more than it looks. Injection covers the release path, but
`go install github.com/maknihamdi/fluxexp/cmd/fluxexp@v0.1.0` passes no ldflags,
and that is a documented way to get the tool. `runtime/debug.ReadBuildInfo()`
carries the module version the toolchain recorded, so the order is: injected
value, else build-info version, else `dev`. Six lines, and it is the difference
between a flag that answers and a flag that says `dev` to the install path the
README recommends second.

### Archive layout and naming

`fluxexp_<version>_<os>_<arch>.tar.gz`, `.zip` for Windows, binary named
`fluxexp` (`fluxexp.exe` on Windows), with `LICENSE` alongside it in the archive.

- The version in the filename means a downloaded file is still identifiable a
  month later, which a bare `fluxexp_linux_amd64` is not.
- `.zip` for Windows because `tar.gz` on Windows is a second tool for many people.
- `LICENSE` in the archive because the licence has to travel with the artifact to
  do anything; the README does not, it lives online.

### Badges: what they can and cannot say

```
[![ci](…/actions/workflows/ci.yml/badge.svg?branch=main)](…/actions)
[![release](https://img.shields.io/github/v/release/maknihamdi/fluxexp)](…/releases/latest)
```

The native Actions badge reports the **latest run on `main`**. It cannot report
the run of a tag: the badge accepts `branch` and `event`, and there is no
parameter for a tag or for "the most recent release build". So "the build state of
the latest version" is served by two badges, not one — CI health from the first,
which version is out from the second. That limitation is stated here because the
obvious reading of the badge is wrong: a green `ci` badge says `main` is healthy,
not that `v0.1.0` built.

The release badge is the one dependency on a third-party service (shields.io). If
that is unacceptable later, it is one line to drop and the CI badge stays native.

### The image gets the version too

A `VERSION` build argument, defaulted to `dev`, threaded into the same `ldflags`
as the binaries, passed by `build-push-action` from the metadata step's version
output.

Without it the image's binary answers `--version` with `dev` while its own OCI
labels say `0.1.0` — two answers from one artifact, and the one a user can reach
from inside the container is the wrong one. The default keeps a bare
`docker build .` working.

It does cost a layer-cache miss per version, which is correct: a different version
string is a different binary.

### Release notes are generated, not written

`generate_release_notes: true`. A hand-maintained changelog is a file that rots;
the commit history is already the record, and this repository's history is
readable because the OpenSpec flow forces a proposal per increment.

## Risks / Trade-offs

- **Five jobs racing to create one release** → the matrix only builds and uploads
  artifacts; exactly one job creates the release. This is the main structural
  decision above, not a mitigation added afterwards.
- **A release published from a commit that failed its tests** → `build-binaries`
  takes `needs: validate`, the same gate the image publication uses.
- **Cross-compiled binaries are never executed before being published.** A
  `darwin/arm64` build that compiles but crashes on start would ship. Accepted for
  the four non-native targets; the `linux/amd64` archive is smoke-tested in the
  job (`--version`), which catches the class of failure that is not
  platform-specific. Running the other four would need four more runners.
- **`contents: write` on a job in a workflow triggered by tags** → the job runs
  only when `github.ref_type == 'tag'`, it uses the default `GITHUB_TOKEN`, and
  the elevation is job-scoped. A pull request from a fork cannot reach it.
- **The CI badge does not mean what it appears to mean** → documented above and in
  the README, which says which branch it reflects.
- **`shields.io` is a third-party dependency in the README** → one line, droppable.
- **MIT was chosen by the repository owner**, not derived. It is recorded here
  because the `licenses` label and the archive contents both follow from it.
- **The `dev` placeholder is nearly unreachable, which is better than designed.**
  Measured on this toolchain, a plain `make build` reports `v0.1.0+dirty` and a
  local `go install` the same: `debug.ReadBuildInfo().Main.Version` carries the
  version derived from version control — the nearest tag, suffixed when the tree
  is modified — not the `(devel)` this design first assumed. So the placeholder
  only appears when no build information was recorded at all. The outcome is more
  useful than the placeholder would have been, and the suffix already prevents a
  local build from passing as a release, so no code forces the placeholder. The
  `cli` spec was corrected to state the observed behaviour rather than the
  assumed one.

## Migration Plan

1. `LICENSE`, the `--version` plumbing and its test land first — they are the only
   Go changes, and `make test` must stay green.
2. The workflow jobs and the README follow. A pull request exercises
   `build-binaries` only if it is made to run there; by default it will not,
   because the release jobs are tag-gated. So the first real exercise is a tag.
3. Tag `v0.2.0` — not `v0.1.0` again. `v0.1.0` has a published image, so rewriting
   it would leave an image and a release describing different commits. The rewrite
   done during `add-ci-pipeline` was safe only because nothing had been published.
4. Verify: five archives plus `SHA256SUMS` on the release, `sha256sum -c` passes,
   the `linux/amd64` binary reports `0.2.0`, `docker run … --version` agrees, and
   `org.opencontainers.image.licenses` is now `MIT`.

Rollback is deleting the release and the two jobs; the image path is untouched by
all of it.

## Open Questions

- Whether the four non-native archives should ever be smoke-tested. It needs
  either more runners or QEMU, and the failure it guards against (a target that
  compiles but does not start) has not been seen in a pure-Go static binary.
- Whether `latest` should also exist as a release alias (a moving `latest` tag on
  the release, as some projects do). Not included: GitHub already serves
  `/releases/latest`, so it would be a second name for something that has one.
