## Why

`add-ci-pipeline` ships an image, and an image serves a cluster. But the primary
way `fluxexp` is used is a person running `fluxexp list --kind Kustomization`
against their own kubeconfig — and that person currently has to clone the repo and
install Go 1.26 to get a binary. `docker run` is a poor substitute for a CLI whose
whole job is to read the caller's kubeconfig.

That change deferred this explicitly ("binaries for humans are a separate
decision"). This is that decision.

It also closes two gaps the first release exposed: the binary cannot say which
version it is, and the repository has no licence — which for downloadable
artifacts is not a formality but a prohibition, since no licence means no right to
use them.

## What Changes

- A **`release` job** on `v*` tags builds `fluxexp` for five targets —
  `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64` —
  and attaches the archives to a **GitHub Release** for that tag.
- Archives are `.tar.gz` (Linux, macOS) and `.zip` (Windows), named
  `fluxexp_<version>_<os>_<arch>.<ext>`, each containing the binary and the
  licence.
- A single **`SHA256SUMS`** file covers every archive in the release. Without it a
  download cannot be verified, which for a binary that reads cluster credentials
  is the difference between a distribution and an invitation.
- The release is created **once**, by one job, after all five builds have
  succeeded — not by each build as it finishes. Five jobs racing to create the
  same release is how a release ends up with three of its five assets.
- **BREAKING (spec only)**: the `ci-pipeline` requirement forbidding any
  repository write permission no longer holds. Creating a release needs
  `contents: write`. The workflow default stays `contents: read`; the elevation is
  granted **at the job level**, to that one job.
- **`fluxexp --version`**. Cobra creates the flag on its own once
  `cmd.Version` is set; the value is injected at build time. It falls back to the
  module version recorded by the Go toolchain, so a binary obtained with
  `go install …@v0.1.0` reports `v0.1.0` rather than `dev`.
- The **image gets the same version injected** through a build argument.
  `docker run … --version` answering `dev` while the image labels announce `0.1.0`
  would make the flag a liar, and a `--version` that lies is worse than none.
- A **MIT `LICENSE`**, which also fills the `org.opencontainers.image.licenses`
  label — it came out empty on `v0.1.0` for exactly this reason.
- **README**: a CI badge and a released-version badge, plus an install section
  with per-platform download, checksum verification, and the `go install`
  alternative.

Deliberately excluded from this increment:
- **GoReleaser.** It is the ecosystem standard and it would do all of the above
  from one config file — but it also wants to publish the container image, so
  adopting it means either running two release mechanisms or rewriting the image
  publication that was just built and verified end to end. Revisit when the
  release grows a Homebrew tap or a changelog worth generating.
- **`windows/arm64`, `linux/386`, FreeBSD.** Five targets cover servers, both Mac
  generations and Windows. A sixth is one matrix line away.
- **Signing and provenance for the binaries** (cosign, SLSA attestations). The
  image already carries a provenance attestation from buildx; the binaries get
  checksums only. Signing is a key-management decision, not a YAML line.
- **A maintained changelog.** GitHub generates release notes from the commits.
- **Packaging** (Homebrew, apt, winget, Scoop, Nix). A tarball and a checksum
  first; a tap is worth it once someone asks twice.
- **`fluxexp version` as a subcommand.** `--version` is what cobra gives for free
  and what people type.

## Capabilities

### New Capabilities
- `binary-releases`: which platforms are built for a released tag, how the
  artifacts are named and verified, that one release carries them all, and how a
  newcomer discovers and installs them.

### Modified Capabilities
- `ci-pipeline`: the requirement stating that an image is published for a released
  tag only asserts the pipeline requests **no repository write permission**.
  Creating a GitHub Release requires it, so the requirement must instead say where
  that permission is granted and how narrowly.
- `cli`: gains a requirement that the binary reports its own version, including
  what it reports when built outside the release pipeline.

## Impact

- `.github/workflows/ci.yml` — a `build-binaries` matrix job and a `release` job;
  the image `publish` job is untouched.
- `Dockerfile` — a `VERSION` build argument threaded into the existing `ldflags`,
  defaulting so that a plain `docker build .` still works.
- `cmd/fluxexp/main.go` and `cmd/fluxexp/command/root.go` — a `version` variable
  and `NewRootCmd(version string)`. `main.go` is the only caller, so the signature
  change is contained.
- New `LICENSE` (MIT, Hamdi MAKNI, 2026).
- `README.md` — badges and an install section. `docs/commands.md` — the release
  and verification commands.
- No change to the engine, the resolvers, health derivation, the portal, or any
  traversal behaviour. No new Go dependency.
- Depends on `add-ci-pipeline` (archived): it extends that workflow rather than
  adding a second one.
