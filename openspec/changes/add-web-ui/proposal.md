## Why

Everything `fluxexp` can show on the CLI (root Kustomizations, health, layer
expansion) is more useful explored visually. This increment initiates the web
UI: a **local, no-auth portal** that reads the current machine's state (the
kubeconfig, and later `gcloud`) and lets a user pick a context, list the root
Kustomizations, and drill into the resource graph **layer by layer, click by
click** — reusing the traversal resolvers already built for the CLI.

## What Changes

- New **`fluxexp ui`** command that starts a local HTTP server (bound to
  loopback) and serves an embedded single-page app. No authentication; the
  server acts with the local kubeconfig's credentials.
- **Backend API** (JSON over HTTP), reusing the existing engine/resolvers:
  - list kube **contexts** from the kubeconfig (with the current one) and let
    the UI select which context a request targets;
  - list **root Kustomizations** for a context (default namespace `flux`, with
    an option to widen), each with health and useful info (source ref, path,
    applied revision, interval, last transition time, message);
  - **resolve a single node** (one hop): given a resource reference, return its
    health/detail and its immediate children — the primitive that powers
    click-by-click expansion (no full-tree BFS).
- **Frontend** (embedded, dependency-free): a context switcher, a home page
  listing root Kustomizations with state/info, and **layer-by-layer drill-in**
  that keeps a breadcrumb of where you came from. A user can either drill in on
  the same page (keeping the trail) or **open a new exploration** (leaving the
  current page). When a hop would change kube context, the UI flags it.

Out of scope for this increment: any cloud/GCP (`gcloud`) integration (later),
authentication, editing/triggering reconciles (read-only), and multi-cluster
context switching mid-graph (only a top-level context selector for now).

## Capabilities

### New Capabilities

- `web-ui`: the local no-auth portal — context listing/selection, the root
  Kustomizations home with state and info, and layer-by-layer click-through
  exploration with breadcrumb and "new exploration", all read-only from the
  local kubeconfig.

### Modified Capabilities

- `cli`: add the `ui` command that starts the local server and serves the
  portal.

## Impact

- New packages: `internal/ui` (HTTP server, handlers, embedded assets) and a
  small `internal/k8s` addition to list kubeconfig contexts. New
  `cmd/fluxexp/command/ui.go`.
- Reuses `internal/engine`, `internal/resolver`, `internal/k8s` unchanged for
  the actual resolution; adds a single-hop resolution entry point (already
  available via `Registry` + `ResolveContext`).
- Frontend embedded via Go `embed` (no JS build step, no external runtime deps),
  keeping the binary self-contained.
- Server binds to `127.0.0.1` by default (the API exposes cluster data with no
  auth, so it must not be reachable off-host).
