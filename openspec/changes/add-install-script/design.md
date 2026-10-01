## Context

What the releases already provide, which the script consumes rather than invents:

- Five archives per release, named `fluxexp_<version>_<os>_<arch>` with `.tar.gz`
  for Linux and macOS and `.zip` for Windows, each containing the binary and
  `LICENSE`.
- One `SHA256SUMS` per release covering all of them, in the format
  `sha256sum -c` consumes.
- Published platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`,
  `darwin/arm64`, `windows/amd64`.

Measurements that shape the design, taken rather than assumed:

- `uname -m` on an Apple Silicon machine reports `arm64`; `go env GOARCH` on the
  same machine reports `amd64`, because the installed toolchain is an amd64 build
  under Rosetta. **Detection must not go through the Go environment.**
- `sha256sum` is not on a stock macOS; `shasum` is. This machine happens to have
  both, which is exactly how the current README recipe passed review and would
  still have failed for a colleague.
- `https://github.com/<repo>/releases/latest/download/<asset>` is not a stable
  URL here, because the asset name carries the version: it resolves today only
  because `latest` happens to be the version in the name, and 404s on the next
  release.
- The repository is public, so the release API needs no credentials.

## Goals / Non-Goals

**Goals:**

- One command installs a verified `fluxexp` on Linux and macOS, on amd64 and
  arm64, without the reader supplying a version or a platform.
- Instructions that cannot go stale, because no version is written down.
- Verification that works on the platforms the releases target, or refuses.
- A readable script: someone asked to pipe it into a shell should be able to read
  it in a minute.

**Non-Goals:**

- Windows coverage from this script, PowerShell, Homebrew, package managers.
- Signatures, self-update, completions.
- Any change to the CLI or its behaviour.

## Decisions

### `go install` is documented first, and the script second

For anyone with a Go toolchain, `go install github.com/maknihamdi/fluxexp/cmd/fluxexp@latest`
is already one line, already resolves the version, already detects the platform,
and already works on Windows. It is strictly simpler than anything this change can
write. It is also verified: it reports `v0.4.0` through `debug.ReadBuildInfo`.

So the script exists for the case `go install` does not cover — no Go toolchain —
and the README says so in that order. Leading with the script would be selling the
more complicated path to people who do not need it.

### Resolve the version from the API, never from a URL alias

```sh
version=$(curl -fsSL https://api.github.com/repos/maknihamdi/fluxexp/releases/latest \
          | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n1)
```

`sed` rather than `jq`, which is not installed everywhere and would be a
dependency the script cannot assume. One field out of a known-shape response does
not justify a JSON parser.

Rejected: `/releases/latest/download/<asset>`. Measured above — it cannot work
while asset names carry the version. Rejected also: publishing version-less
duplicate assets to make that URL work. That doubles the release's assets to
work around a problem one API call solves.

`FLUXEXP_VERSION` skips the call, which matters for a pinned install in CI and for
anyone behind a proxy that blocks the API but not the downloads.

### Detect with `uname`, and map explicitly

`uname -s` → `linux` / `darwin`; `uname -m` → `amd64` for `x86_64|amd64`, `arm64`
for `arm64|aarch64`. Anything else exits with the list of published platforms.

The mapping is a `case`, not a lowercase-and-hope: `uname -m` says `x86_64` where
Go says `amd64`, and `aarch64` on Linux where macOS says `arm64`. Four lines that
have to be right.

An explicit failure matters more than it looks: without it, an unsupported
platform produces a 404 from `curl` on a URL the user never typed, which reads as
"the project is broken" rather than "that platform is not published".

### Verify, or refuse

```sh
if command -v sha256sum >/dev/null; then  sum() { sha256sum "$@"; }
elif command -v shasum   >/dev/null; then  sum() { shasum -a 256 "$@"; }
else exit 1
fi
```

And the comparison is on the **one** line for the downloaded archive, not the
whole file — `-c SHA256SUMS` fails on the four archives that were not downloaded,
which is the bug the README carried until it was caught by actually running it.

No fallback to "skip the check if no tool is available". This binary reads the
caller's kubeconfig and talks to their clusters with their credentials; an
unverified install is the part of the distribution that matters most. A script
that silently degrades its only security step is worse than one that stops.

### Install without `sudo` when it can

Target is `${FLUXEXP_BIN_DIR:-/usr/local/bin}`. `sudo` is used only when that
directory exists and is not writable, and the script says which it chose. A
`FLUXEXP_BIN_DIR=~/.local/bin` install asks for no privileges at all, which is
also the form to recommend to anyone who dislikes `curl | sh`.

Rejected: always `sudo`. It trains people to hand root to a piped script for no
reason.

### A temp directory, cleaned on every exit

`mktemp -d` plus `trap 'rm -rf "$tmp"' EXIT`, so a failed download leaves nothing
behind and the archive is never extracted into the caller's working directory —
the current README recipe drops `fluxexp`, `LICENSE` and `SHA256SUMS` wherever the
reader happened to be.

### Shipped as a release asset as well as on `main`

The convenience one-liner fetches `install.sh` from `main`, which is the only
honest way to offer one line. Adding it to the release's files gives a second,
immutable copy whose checksum is in that release's `SHA256SUMS` — so the script
that verifies downloads is itself verifiable, and a pinned deployment can fetch a
script that will not change under it.

### `set -eu`, and no `pipefail`

`set -eu` catches the unset-variable and failed-command classes. `pipefail` is not
POSIX — it is a bashism — and the script is `sh`. The one pipeline whose failure
matters (the version resolution) is checked by testing the result for emptiness
instead.

## Risks / Trade-offs

- **`curl | sh` executes code from the network.** Stated as what it is, with the
  download-read-run form given beside it, and the user-local install needing no
  `sudo`. Not argued away: the README shows both and the script is short enough to
  read.
- **The API call is a new dependency at install time** → `FLUXEXP_VERSION` skips
  it entirely, and the failure is explicit rather than a confusing 404.
- **Unauthenticated API calls are rate-limited** (60/hour per IP). Irrelevant for
  a human, possible in CI that installs repeatedly from one egress address — which
  is exactly where `FLUXEXP_VERSION` should be used anyway.
- **`sed` parsing of JSON is fragile if the response shape changes** → it reads
  `tag_name` out of a stable, documented payload, and an empty result is treated
  as a failure rather than carried forward into a URL.
- **The script is not covered by `make test`.** It is shell, and the suite is Go.
  It is exercised the only way that proves anything: run on this machine for the
  native platform, and run with `FLUXEXP_BIN_DIR` pointing at a scratch directory
  for the cases that do not need root. Cross-platform behaviour is argued from the
  `uname` mapping, not claimed as tested.
- **Windows remains the worst path.** Accepted and written down rather than
  disguised.

## Migration Plan

Nothing to migrate. The current README recipe keeps working for anyone who
already copied it, since the asset naming does not change.

1. Add `install.sh`, run it locally into a scratch directory, confirm the binary
   it installs reports the release version.
2. Add it to the release job's files, rewrite the README and `docs/commands.md`.
3. On the next tag, confirm `install.sh` appears among the release assets and in
   `SHA256SUMS`, then run the one-liner from the README verbatim.

Rollback is deleting the script and restoring the README section; nothing depends
on it.

## Open Questions

- Whether to offer `FLUXEXP_VERSION=latest` as an explicit synonym for the default,
  for symmetry in CI configuration. Left out as a second spelling of the default.
- Whether the script should refuse to overwrite an existing `fluxexp` that came
  from somewhere else (a `go install` into `$GOBIN`, a package manager). It
  currently installs over the target path it was given. Worth deciding if anyone
  reports a surprise.
