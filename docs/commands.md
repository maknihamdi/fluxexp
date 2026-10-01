# Command reference

Commands that were needed at least twice, with what they are for. Everything here
is read-only unless it says otherwise.

## Build and test

```bash
make build        # build ./bin/fluxexp
make vet          # go vet ./...
make test         # go test ./... — no cluster needed, the fixtures are in-memory
make tidy         # go mod tidy
```

```bash
# What CI checks that `make` does not: formatting. Prints the offending files.
gofmt -l .
```

The toolchain version comes from `go.mod` (`go 1.26.0`). A local `go` older than
that downloads the right toolchain on first use, which needs network access and a
writable module cache — worth knowing when a shell has neither and `go` reports
`go.mod requires go >= 1.26.0`.

## Container image

```bash
# Build locally. The frontend is embedded in the binary, so this is a Go build.
docker build -t fluxexp:dev .

# Run the portal. --address is required: the binary binds loopback by default,
# which from inside a container means unreachable from outside it.
docker run --rm -p 8765:8765 fluxexp:dev ui --address 0.0.0.0:8765
curl -s localhost:8765 | head        # should be the SPA's HTML
```

Checks that the image is what the deployment assumes it is:

```bash
# Runs as nonroot (uid 65532), not root.
docker inspect --format '{{.Config.User}}' fluxexp:dev

# There is no shell. This MUST fail — a success means the base image changed.
docker run --rm --entrypoint /bin/sh fluxexp:dev -c true

# Size: tens of MB. Hundreds means the final stage picked up the build stage.
docker images fluxexp:dev
```

Reading a published image back, to tie a running container to a commit:

```bash
# The OCI labels: .source, .revision, .version, .created.
docker buildx imagetools inspect quay.io/hamdi_makni/fluxexp:0.3.0 --raw

# Which tags exist.
skopeo list-tags docker://quay.io/hamdi_makni/fluxexp
```

## CI

```bash
gh workflow view ci.yml           # parses the workflow; fails on a syntax error
gh run list --workflow ci.yml     # recent runs
gh run watch                      # follow the run for the current branch
gh run view --log-failed          # the failing step's log, without the rest
```

```bash
# Set the registry credentials (a Quay robot account with write on that one
# repository). WRITES to the repository's secrets.
gh secret set QUAY_USERNAME
gh secret set QUAY_TOKEN
```

```bash
# Release. Publishing happens here and nowhere else.
git tag v0.3.1 && git push origin v0.3.1
```

## Binaries

```bash
# What the release matrix does, for one target. Useful for reproducing a build
# locally or checking a target still compiles before tagging.
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
  go build -ldflags "-X main.version=0.3.0" -o /tmp/fluxexp ./cmd/fluxexp

# What version a binary claims. Injected value, else the version the toolchain
# recorded (a dirty tree is marked), else `dev`.
fluxexp --version
```

```bash
# Install the current release: detects the platform, verifies the checksum,
# installs to /usr/local/bin.
curl -fsSL https://raw.githubusercontent.com/maknihamdi/fluxexp/main/install.sh | sh

# Pinned and user-local — the form for CI: reproducible, no sudo, and no call to
# the release API (unauthenticated, 60 requests an hour per address).
curl -fsSL https://raw.githubusercontent.com/maknihamdi/fluxexp/main/install.sh \
  | FLUXEXP_VERSION=0.4.0 FLUXEXP_BIN_DIR="$HOME/.local/bin" sh
```

```bash
# The release's assets, without opening a browser.
gh release view v0.3.0 --json assets --jq '.assets[].name'

# Download them all, or one.
gh release download v0.3.0
gh release download v0.3.0 --pattern 'fluxexp_*_linux_amd64.tar.gz'

# Verify, on either platform: macOS has no sha256sum, it has shasum -a 256.
# SHA256SUMS covers every asset, so check the line for the file you have rather
# than the whole file, which would fail on everything you did not download.
sum=$(command -v sha256sum || echo 'shasum -a 256')
grep 'fluxexp_0.3.0_linux_amd64' SHA256SUMS | $sum -c -
```

```bash
# Which tag `latest` would follow — the same computation the publish job makes.
git tag -l 'v*' --sort=-v:refname | sed '/-/d' | head -n1
```

## Cluster

```bash
fluxexp list --kind Kustomization --unhealthy    # discover broken starting points
fluxexp traverse -n flux-system --name apps      # the whole tree from one root
fluxexp ui                                       # portal on 127.0.0.1:8765
```

## Deploying the portal

Always pass `--context` explicitly. The current context is whatever it last was,
and this applies a ClusterRole granting cluster-wide read.

```bash
# Render and read before applying anything.
kubectl kustomize deploy/

# Check the selectors agree — the Deployment's and the Service's are generated,
# the PDB's and the spread constraint's are written by hand because kustomize's
# label transformer does not reach them.
kubectl kustomize deploy/ | grep -A2 -E "^  selector:|labelSelector:"
```

```bash
# Validate against a real API server. The cluster-scoped objects only: a server
# dry-run does not create the Namespace, so the namespaced ones fail with
# `namespaces "fluxexp" not found`. That is the dry-run's limitation, not a
# manifest defect.
kubectl --context <ctx> apply -k deploy/ --dry-run=server
```

```bash
# Apply. WRITES: a Namespace, a ServiceAccount, a ClusterRole with cluster-wide
# read (Secrets included) and a ClusterRoleBinding, plus the workload.
kubectl --context <ctx> apply -k deploy/

kubectl --context <ctx> -n fluxexp port-forward svc/fluxexp 8765:8765
```

```bash
# The HPA owns the replica count; the Deployment declares none. Re-applying must
# not reset it.
kubectl --context <ctx> -n fluxexp get hpa,deploy

# How many evictions the budget allows right now. With minAvailable 1 and two
# replicas this reads 1; at one replica it reads 0 and a drain would hang.
kubectl --context <ctx> -n fluxexp get pdb

# Where to revise the resource requests from.
kubectl --context <ctx> -n fluxexp top pod
```

```bash
# Remove everything this base created.
kubectl --context <ctx> delete -k deploy/
```