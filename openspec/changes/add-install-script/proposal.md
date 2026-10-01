## Why

The README's install recipe asks the reader for three facts their machine already
knows — the version, the OS, the architecture — and then hardcodes the version in
the document. That last part has gone stale **three times** in three releases: it
said `0.2.0` after `v0.2.1` shipped, `0.3.0` after `v0.3.0`, and it still says
`0.3.0` now that `v0.4.0` is out. A newcomer following it downloads a version
that is not the latest, or gets a 404 once that asset is garbage.

The recipe is also wrong on two platforms in ways that are invisible until they
bite:

- It ends with `sha256sum -c -`. **macOS has no `sha256sum`**, it has
  `shasum -a 256`. The verification step — the one step that matters for a binary
  that reads cluster credentials — fails on a stock Mac.
- Anyone writing their own detection is a step away from `go env GOARCH`, which
  **lies**: measured on an Apple Silicon machine with an amd64 Go toolchain, it
  reports `amd64` while `uname -m` correctly reports `arm64`. That mistake
  installs the wrong binary and it runs, slowly, under emulation.

## What Changes

- An **`install.sh`** at the repo root, POSIX shell, that does what the reader
  should not have to: detect the platform, resolve the current version, download,
  verify, install.
  - OS and architecture from `uname -s` / `uname -m`, never from the Go
    environment.
  - The version resolved from the release API, so **nothing is hardcoded** and the
    instructions cannot go stale. `FLUXEXP_VERSION` pins it when needed.
  - Checksum verified with whichever of `sha256sum` or `shasum -a 256` exists, and
    the script **refuses to install** if neither does rather than skipping the
    check.
  - Installs to `/usr/local/bin`, or `FLUXEXP_BIN_DIR`; `sudo` only when the
    target is not writable, so a user-local install needs no privileges.
  - An unsupported platform fails with the list of what is published, not with a
    404 from `curl`.
- The script is also **published as a release asset** and covered by that
  release's `SHA256SUMS`, so there is a pinned, verifiable copy of the thing that
  does the verifying.
- The **README install section is rewritten**: `go install` first, because for
  anyone with a Go toolchain it is already one line and already covers every
  platform including Windows; then the script for Linux and macOS; then the
  `.zip` for Windows without Go. The hardcoded version disappears.

Deliberately excluded from this increment:
- **A PowerShell installer.** A POSIX script does not cover Windows and pretending
  otherwise is how a "universal" installer becomes a support thread. Windows users
  with Go have a one-liner; without Go they get a `.zip` and two lines. Worth
  revisiting if someone asks twice.
- **A Homebrew tap.** A separate repository with a formula to bump on every
  release — real infrastructure for an internal tool.
- **Package managers** (apt, winget, Scoop, Nix), and **shell completions**.
- **Signature verification.** The release carries checksums, not signatures;
  teaching the script to verify a signature that does not exist would be theatre.
- **Self-update** (`fluxexp upgrade`). The install line is idempotent; a tool that
  rewrites its own binary is a different risk profile.

## Capabilities

### New Capabilities
<!-- none: this is how an existing capability is delivered, not a new one -->

### Modified Capabilities
- `binary-releases`: the requirement covering discoverability from the README
  currently only asks that instructions exist. It must state that the instructions
  carry no hardcoded version, that platform detection is not the reader's job, and
  that verification works on the platforms the releases target. The release-content
  requirement also gains the installer as a published, checksummed asset.

## Impact

- New `install.sh` at the repo root.
- `.github/workflows/ci.yml` — the `release` job includes `install.sh` among the
  files it uploads, so it lands in `SHA256SUMS` with everything else.
- `README.md` — the install section, rewritten; `docs/commands.md` — the install
  and verification commands.
- `.dockerignore` — exclude `install.sh`; it has no business in the image build
  context.
- No Go code, no change to the CLI, the engine, the resolvers or the portal.
- Depends on `binary-releases` (archived): the script consumes the asset naming
  and the `SHA256SUMS` format that capability defines.
