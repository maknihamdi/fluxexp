#!/bin/sh
# Install fluxexp on Linux or macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/maknihamdi/fluxexp/main/install.sh | sh
#
# Environment:
#   FLUXEXP_VERSION   install this version instead of the latest (e.g. 0.4.0).
#                     Pin it in CI: it is reproducible and it skips the API call,
#                     which is rate-limited to 60 requests an hour per address.
#   FLUXEXP_BIN_DIR   where to install (default /usr/local/bin). A directory you
#                     own needs no sudo.
#
# Windows is not covered: this is a POSIX script. Use the Go toolchain
# (`go install github.com/maknihamdi/fluxexp/cmd/fluxexp@latest`) or the .zip from
# the releases page.

set -eu

REPO=maknihamdi/fluxexp
BIN_DIR="${FLUXEXP_BIN_DIR:-/usr/local/bin}"
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# --- platform -----------------------------------------------------------------
# From uname, never from `go env`: on an Apple Silicon machine whose Go toolchain
# is an amd64 build, `go env GOARCH` reports amd64 while the hardware is arm64,
# and the wrong binary installs and runs, slowly, under emulation.
case "$(uname -s)" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported operating system '$(uname -s)'. Published: $PLATFORMS" ;;
esac

case "$(uname -m)" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture '$(uname -m)'. Published: $PLATFORMS" ;;
esac

# --- version ------------------------------------------------------------------
version="${FLUXEXP_VERSION:-}"
if [ -z "$version" ]; then
  # The asset name carries the version, so /releases/latest/download/<asset> is
  # not a stable URL — it resolves only while `latest` happens to be the version
  # written in the name. One API call is the reliable way.
  # sed rather than jq, which is not installed everywhere.
  version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
            | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$version" ] || die "could not determine the latest version. Set FLUXEXP_VERSION to install a specific one."
fi

archive="fluxexp_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/v${version}"

# --- checksum tool ------------------------------------------------------------
# sha256sum on Linux, shasum on macOS. No fallback to skipping the check: this
# binary reads the caller's kubeconfig and talks to their clusters with their
# credentials, so an unverified install is the part that matters most.
if command -v sha256sum >/dev/null 2>&1; then
  sum() { sha256sum "$@"; }
elif command -v shasum >/dev/null 2>&1; then
  sum() { shasum -a 256 "$@"; }
else
  die "neither sha256sum nor shasum found; refusing to install without verifying the download"
fi

# --- download, verify, install ------------------------------------------------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

printf 'fluxexp %s (%s/%s)\n' "$version" "$os" "$arch"

curl -fsSL -o "$tmp/$archive" "$base/$archive" \
  || die "could not download $archive. Is $version a released version?"
curl -fsSL -o "$tmp/SHA256SUMS" "$base/SHA256SUMS" \
  || die "could not download SHA256SUMS for $version"

# Only the line for this archive: SHA256SUMS covers all five, and checking the
# whole file fails on the four that were not downloaded.
( cd "$tmp" && grep " $archive\$" SHA256SUMS | sum -c - >/dev/null ) \
  || die "checksum mismatch for $archive; nothing was installed"

tar xzf "$tmp/$archive" -C "$tmp"

if [ -w "$BIN_DIR" ]; then
  install -m 0755 "$tmp/fluxexp" "$BIN_DIR/fluxexp"
  printf 'installed %s/fluxexp\n' "$BIN_DIR"
elif [ -d "$BIN_DIR" ]; then
  printf '%s is not writable, using sudo\n' "$BIN_DIR"
  sudo install -m 0755 "$tmp/fluxexp" "$BIN_DIR/fluxexp"
  printf 'installed %s/fluxexp\n' "$BIN_DIR"
else
  die "$BIN_DIR does not exist. Create it, or set FLUXEXP_BIN_DIR to a directory that does."
fi

# Prove it rather than announce it.
"$BIN_DIR/fluxexp" --version
