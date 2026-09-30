# The frontend is compiled into the binary (//go:embed web/* in internal/ui), so
# the image build is a Go build and nothing else — no node toolchain, no asset
# step.
#
# This tag is the second place the Go version is declared; go.mod is the first.
# A toolchain bump touches both, and a drift the wrong way fails loudly here
# because Go refuses a go.mod requiring a newer toolchain than the image has.
FROM golang:1.26 AS build

WORKDIR /src

# Dependencies first: a change touching only Go source reuses this layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# The version the binary reports through --version. Defaulted so a plain
# `docker build .` still works; the pipeline passes the released version, because
# a --version answering `dev` while the image labels announce 0.1.0 would make
# the flag a liar.
ARG VERSION=dev

# CGO off is required, not incidental: the final base is statically linked and
# carries no libc for cgo's resolver to call. It also gives the pure-Go resolver,
# which reads /etc/resolv.conf the way a cluster expects.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o /out/fluxexp ./cmd/fluxexp

# distroless static carries exactly what this binary needs: CA certificates (its
# only outbound traffic is HTTPS to an API server), /etc/passwd with the nonroot
# user 65532, and tzdata. No shell, no package manager.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/fluxexp /usr/local/bin/fluxexp

# No CMD: the caller picks the subcommand (ui, list, traverse).
ENTRYPOINT ["/usr/local/bin/fluxexp"]