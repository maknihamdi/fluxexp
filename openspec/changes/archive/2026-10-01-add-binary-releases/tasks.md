## 1. Licence

- [x] 1.1 Add `LICENSE` at the repo root: MIT, copyright `2026 Hamdi MAKNI`, verbatim text from the SPDX MIT template (no edits to the body — a modified MIT is not MIT)
- [x] 1.2 Confirm `.dockerignore` does not exclude `LICENSE` (it excludes `README.md` and `CLAUDE.md`, so check this one explicitly). Not needed by the archives — those are built on the runner with `go build`, outside any Docker context — but the exclusion list should not silently decide it for a later image change

## 2. `--version`

- [x] 2.1 In `cmd/fluxexp/command/root.go`, change `NewRootCmd()` to `NewRootCmd(version string)` and set `cmd.Version = version`. Declare no flag by hand: cobra adds `--version` on its own once `Version` is non-empty
- [x] 2.2 In `cmd/fluxexp/main.go`, add `var version string` (the `-X main.version` target) and pass it through. It is the only caller
- [x] 2.3 Add the fallback in `main.go`: injected value first, else the module version from `runtime/debug.ReadBuildInfo()`, else `dev`. This is what makes `go install …@v0.1.0` report a version instead of `dev`
- [x] 2.4 Table test in `cmd/fluxexp/command` asserting `NewRootCmd("1.2.3")` yields a command whose `Version` is `1.2.3` and which accepts `--version`; and that an empty version leaves cobra's flag absent rather than printing an empty version
- [x] 2.5 `make vet` and `make test` green
- [x] 2.6 Verify by hand: `go build -ldflags "-X main.version=9.9.9" -o /tmp/fx ./cmd/fluxexp && /tmp/fx --version` prints `9.9.9`, and a plain `make build` prints `dev` or `(devel)`
- [x] 2.7 Verify the fallback path without publishing: `go install` from the local module and confirm the build-info branch is reached (a dirty tree reports `(devel)`, which is the branch working, not a failure)

## 3. The version reaches the image

- [x] 3.1 In `Dockerfile`, add `ARG VERSION=dev` in the build stage and pass `-ldflags "-X main.version=${VERSION}"` to the existing `go build`
- [x] 3.2 In the `publish` job, pass `build-args: VERSION=${{ steps.meta.outputs.version }}` to `docker/build-push-action`
- [x] 3.3 Confirm a plain `docker build -t fluxexp:dev .` still works and `docker run --rm fluxexp:dev --version` prints `dev`
- [x] 3.4 Confirm an explicit `docker build --build-arg VERSION=9.9.9 .` produces a binary reporting `9.9.9`

## 4. Build the binaries

- [x] 4.1 Add a `build-binaries` job: `needs: validate`, `if: github.ref_type == 'tag'`, `ubuntu-latest`, matrix over the five targets (`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`). No `permissions` block — it keeps the read-only default
- [x] 4.2 Steps: checkout, `setup-go` with `go-version-file: go.mod`, then `CGO_ENABLED=0 GOOS=… GOARCH=… go build -ldflags "-X main.version=<tag without the v>" -o …`. The binary is `fluxexp.exe` for Windows
- [x] 4.3 Archive per target: `tar czf fluxexp_<version>_<os>_<arch>.tar.gz fluxexp LICENSE`, or `zip` for Windows. The archive holds the binary and `LICENSE`, nothing else
- [x] 4.4 Smoke-test the native target only: on `linux/amd64`, run the built binary with `--version` and assert it prints the release version — catches a broken ldflags path, which is the failure that is *not* platform-specific
- [x] 4.5 `actions/upload-artifact` per matrix entry, so the release job can collect them

## 5. Create the release

- [x] 5.1 Add a `release` job: `needs: build-binaries`, `if: github.ref_type == 'tag'`, and `permissions: contents: write` **on the job** — the workflow-level `contents: read` must stay as it is
- [x] 5.2 `actions/download-artifact` with no name, collecting every matrix artifact into one directory, flattened so the archives sit side by side
- [x] 5.3 Generate `SHA256SUMS` over all five archives in one file, in the format `sha256sum -c` accepts
- [x] 5.4 `softprops/action-gh-release` with the five archives plus `SHA256SUMS`, and `generate_release_notes: true`. Exactly this one job creates the release — no `action-gh-release` inside the matrix
- [x] 5.5 `actionlint` on the workflow reports nothing
- [x] 5.6 Re-read the job graph and confirm the two gates: nothing is built without `validate`, and nothing is released without all five builds

## 6. README and docs

- [x] 6.1 Add the two badges at the top of the README: the native Actions badge with `?branch=main`, and a shields.io release badge linking to `/releases/latest`
- [x] 6.2 Add an "Install" section before "Usage": download per platform, `sha256sum -c` verification, `go install github.com/maknihamdi/fluxexp/cmd/fluxexp@latest`, and the container image as the third option
- [x] 6.3 State in the README what the CI badge reflects — the latest run on `main`, not the released tag's build. Without that line the badge reads as a claim about the release
- [x] 6.4 Mention the licence in the README
- [x] 6.5 Add to `docs/commands.md`: the local cross-build one-liner, `sha256sum -c SHA256SUMS`, `gh release view --json assets`, `gh release download`
- [x] 6.6 Add the increment to the README roadmap

## 7. Release and verify

- [x] 7.1 Tag **`v0.2.0`**, not `v0.1.0` — `v0.1.0` already has a published image, so re-tagging it would leave the image and the release describing different commits
- [x] 7.2 Confirm the job order on the run: `validate` → `publish` and `build-binaries` → `release`
- [x] 7.3 Confirm the release carries five archives and `SHA256SUMS` (`gh release view <tag> --json assets`)
- [x] 7.4 Download the `linux/amd64` archive and `SHA256SUMS`, verify with the targeted form (`grep <archive> SHA256SUMS | sha256sum -c -` — a plain `-c SHA256SUMS` fails on the four archives not downloaded), extract it, and confirm the binary reports the release version and the archive contains `LICENSE`
- [x] 7.5 Confirm `docker run --rm quay.io/hamdi_makni/fluxexp:0.2.0 --version` reports `0.2.0`, agreeing with `org.opencontainers.image.version`
- [x] 7.6 Confirm `org.opencontainers.image.licenses` is now `MIT` — it was empty on `v0.1.0` because there was no `LICENSE`
- [x] 7.7 Confirm both README badges render and point at the right places

**Where this was verified.** Group 7 ran across three tags and two repositories,
because the release job failed on its first attempt and the repository was
republished mid-increment:

- `v0.2.0` — image published; the `release` job failed (`download-artifact`
  swept `build-push-action`'s `.dockerbuild` records). 7.1, 7.2, 7.5, 7.6 were
  verified here: `licenses` went from empty to `MIT`.
- `v0.2.1` — the fix; first complete release. 7.3 and 7.4 verified here: five
  archives, checksums, `LICENSE` in the archive, the `darwin/arm64` binary run
  natively and the `linux/amd64` one under emulation.
- `v0.3.0` — the first release of the scrubbed, public repository. 7.3 re-verified,
  and 7.7 closed: the release badge went from `no releases or repo not found` to
  `release: v0.3.0`, which it could never have shown while the repository was
  private.
