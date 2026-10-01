## 1. The script

- [x] 1.1 Create `install.sh` at the repo root, `#!/bin/sh` and `set -eu`. No `pipefail` — it is a bashism and this is `sh`; the one pipeline that matters (version resolution) is checked by testing its result for emptiness
- [x] 1.2 Platform detection: `uname -s` → `linux`/`darwin`, `uname -m` → `amd64` for `x86_64|amd64` and `arm64` for `arm64|aarch64`. **Never** `go env GOARCH`, which reports `amd64` on an Apple Silicon machine whose toolchain is an amd64 build. Anything unmatched exits naming the published platforms
- [x] 1.3 Version: `FLUXEXP_VERSION` when set, else `tag_name` from the releases API, parsed with `sed` (no `jq` dependency). An empty result is a failure, not a value carried into a URL
- [x] 1.4 Download the archive and `SHA256SUMS` into `mktemp -d`, with `trap 'rm -rf "$tmp"' EXIT` so nothing survives a failure and nothing lands in the caller's directory
- [x] 1.5 Checksum: define a `sum()` wrapper over `sha256sum` or `shasum -a 256`, whichever exists, and **exit non-zero when neither does**. Verify only the line for the downloaded archive — a whole-file `-c` fails on the four archives not downloaded
- [x] 1.6 Install to `${FLUXEXP_BIN_DIR:-/usr/local/bin}` with `install -m 0755`, using `sudo` only when the directory is not writable, and print which destination and whether elevation was used
- [x] 1.7 Print the installed version at the end by running the binary, so success is proven rather than announced
- [x] 1.8 `shellcheck install.sh` clean if available; otherwise `sh -n install.sh` for a syntax check, and say which was used

## 2. Exercise it

- [x] 2.1 Run with `FLUXEXP_BIN_DIR` pointing at a scratch directory: it installs without `sudo` and the binary reports the current release version
- [x] 2.2 Run with `FLUXEXP_VERSION` pinned to an older release and confirm that version is installed, proving the pin bypasses the API
- [x] 2.3 Corrupt the downloaded archive mid-flight (or point the checksum at a wrong line) and confirm the script aborts and installs nothing
- [x] 2.4 Simulate no checksum tool (run with a `PATH` lacking both) and confirm it refuses rather than installing unverified
- [x] 2.5 Simulate an unsupported platform by overriding the detection inputs and confirm the error names the published platforms instead of failing on a download
- [x] 2.6 Confirm the working directory is clean afterwards in every case above — no archive, no `SHA256SUMS`, no stray binary

## 3. Publish it with the release

- [x] 3.1 Add `install.sh` to the `release` job's `files`, so it lands among the assets and in `SHA256SUMS` with everything else
- [x] 3.2 Confirm the checksum step still covers only the intended files: it globs `fluxexp_*`, so the script has to be added to the sum explicitly or the glob widened — decide which and make the file list and the sum agree
- [x] 3.3 Exclude `install.sh` in `.dockerignore`; it has no business in the image build context
- [ ] 3.4 `actionlint` clean

## 4. Rewrite the instructions

- [x] 4.1 README install section, in this order: `go install` first (one line, every platform, no detection — and it already reports the right version through build info), then the shell installer for Linux and macOS, then the `.zip` for Windows without Go
- [x] 4.2 Show both forms of the installer: the one-liner, and download-read-run beside it. State plainly that the first executes code from the network, and that `FLUXEXP_BIN_DIR` into a user directory needs no `sudo` — without arguing the reader out of their caution
- [x] 4.3 **Remove every hardcoded version from the install instructions.** This is the defect the change exists for: the section has said `0.2.0`, then `0.3.0`, each time one release behind
- [x] 4.4 Document `FLUXEXP_VERSION` and `FLUXEXP_BIN_DIR`, and that pinning the version is what CI should do — both to be reproducible and to avoid the unauthenticated API rate limit
- [x] 4.5 Keep the Windows path honest: no PowerShell installer is shipped, so say what to do instead rather than implying coverage
- [x] 4.6 `docs/commands.md`: the install commands, the pinned form, and the portable verification one-liner
- [x] 4.7 Add the increment to the README roadmap

## 5. Verify on a real release

- [ ] 5.1 After merge, tag the next version and confirm `install.sh` appears among the release assets with its checksum in `SHA256SUMS`
- [ ] 5.2 Run the README's one-liner verbatim, from a directory that is not the repo, and confirm it installs the new version
- [ ] 5.3 Fetch the published `install.sh` asset and confirm its checksum matches the entry in that release's `SHA256SUMS`
