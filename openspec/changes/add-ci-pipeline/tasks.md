## 1. Make the tree pipeline-clean first

- [x] 1.1 Run `gofmt -l .` locally and commit any reformatting it asks for, so the first pipeline run fails for a real reason or not at all
- [x] 1.2 Run `make vet` and `make test` locally and confirm both are green on the current `main`

## 2. The image

- [x] 2.1 Add `.dockerignore` excluding at least `bin/`, `.git/`, `.idea/`, `.claude/`, `openspec/`, `.DS_Store` — the build context is a Go module and a `Dockerfile`, nothing else
- [x] 2.2 Add `Dockerfile`: builder stage `FROM golang:1.26`, `go mod download` before copying the source (so a dependency-only change reuses the layer), then `CGO_ENABLED=0 GOOS=linux go build -o /out/fluxexp ./cmd/fluxexp`
- [x] 2.3 Final stage `FROM gcr.io/distroless/static-debian12:nonroot`, copy the binary to `/usr/local/bin/fluxexp`, `ENTRYPOINT ["/usr/local/bin/fluxexp"]`, no `CMD` — the command is chosen by the caller (`ui`, `list`, `traverse`)
- [x] 2.4 `docker build -t fluxexp:dev .` and check the size is in the tens of MB, not hundreds
- [x] 2.5 Verify non-root: `docker inspect --format '{{.Config.User}}' fluxexp:dev` returns `nonroot:nonroot` (or uid 65532)
- [x] 2.6 Verify there is no shell: `docker run --rm --entrypoint /bin/sh fluxexp:dev -c true` must fail to start
- [x] 2.7 Verify the embedded frontend ships: `docker run --rm -p 8765:8765 fluxexp:dev ui --address 0.0.0.0:8765` and `curl -s localhost:8765` returns the SPA HTML (it will have no cluster to talk to — the HTML is what this step checks)

## 3. The validate job

- [x] 3.1 Create `.github/workflows/ci.yml`: name, `on: push` (branches `main`, tags `v*`) and `on: pull_request` (branches `main`), and `permissions: contents: read` at workflow level
- [x] 3.2 Job `validate` on `ubuntu-latest`: `actions/checkout`, then `actions/setup-go` with `go-version-file: go.mod` and its default module/build caching
- [x] 3.3 Steps in order: the `gofmt -l` check (fail listing the files), `make vet`, `make test`, `make build`
- [x] 3.4 Add an image build step to `validate` using `docker/build-push-action` with `push: false`, so a broken `Dockerfile` is caught on a pull request rather than at tag time
- [x] 3.5 Push the branch and confirm the job runs green on the pull request; check that the Go cache is populated on the second run

## 4. The publish job

- [x] 4.1 Create a Quay robot account with write on `hamdi_makni/fluxexp` only, and set the repository secrets `QUAY_USERNAME` and `QUAY_TOKEN`
- [x] 4.2 Job `publish` with `needs: validate` and `if: github.ref_type == 'tag'` — the gate must be on the ref, not on the event name — checking out with `fetch-depth: 0` so the other tags are present
- [x] 4.3 Add the `latest` decision as its own step: take the highest non-prerelease tag (`git tag -l 'v*' --sort=-v:refname`, drop tags containing `-`, first line) and emit `latest=true` only when it equals the tag being built. `metadata-action` cannot make this call — it sees only the current ref and queries neither the other tags nor the registry
- [x] 4.4 `docker/metadata-action` with `images: quay.io/hamdi_makni/fluxexp`, `flavor: latest=false` (so the action's own rule cannot add the tag behind 4.3's decision), and tag rules `type=semver,pattern={{version}}`, `type=semver,pattern={{major}}.{{minor}}`, `type=raw,value=latest,enable=<4.3's output>`
- [x] 4.5 `docker/login-action` against `quay.io` with the two secrets, then `docker/build-push-action` with `push: true`, passing the metadata step's `tags` and `labels` outputs
- [x] 4.6 Confirm the workflow file is valid before tagging: `gh workflow view ci.yml` (or `actionlint` if available) reports no parse error

## 5. First release

- [ ] 5.1 Tag `v0.1.0` on `main` and push it
- [ ] 5.2 Confirm `validate` runs and `publish` waits for it, then confirm the image appears at `quay.io/hamdi_makni/fluxexp` with tags `0.1.0`, `0.1`, `latest`
- [ ] 5.3 Read the labels back (`docker buildx imagetools inspect quay.io/hamdi_makni/fluxexp:0.1.0`) and confirm `org.opencontainers.image.revision` matches the tagged commit
- [ ] 5.4 Confirm no image was pushed by the earlier branch and pull-request runs

## 6. Documentation

- [x] 6.1 Add a "Container image" line to the README Development section: where the image lives, that it is published on `v*` tags only, and the `ui --address 0.0.0.0:8765` invocation
- [x] 6.2 Record the useful commands in a project command reference (`docs/commands.md` or the existing equivalent): local image build, the non-root/no-shell checks, `imagetools inspect`, `gh run watch`
- [x] 6.3 Note in `CLAUDE.md` that the Go version lives in `go.mod` and is pinned a second time in the `Dockerfile`'s builder tag, so a toolchain bump touches both
