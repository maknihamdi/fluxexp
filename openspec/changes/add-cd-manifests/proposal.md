## Why

`add-ci-pipeline` produces an image; nothing consumes it. The portal is the half
of `fluxexp` that a team would actually share — a read-only view of what Flux
reconciled, with health down to the Pod — and today it only exists on the laptop
of whoever ran `fluxexp ui`. Running it in the cluster it inspects removes the
kubeconfig from the equation entirely: the pod's ServiceAccount is the
credential.

That move exposes a gap the tool has carried since I3. The portal was built for a
laptop: it lists the kube contexts from the local kubeconfig and asks the user to
pick one. In a pod there is no kubeconfig, so the selector comes back **empty**
and the user is left with a context bar offering nothing. The traversal itself
works — `internal/k8s.LoadClient` goes through `clientcmd`'s deferred loader,
which falls back to the in-cluster config when it finds no kubeconfig — so this
is a one-selector gap, not a port.

## What Changes

- A **kustomize base** under `deploy/` holding the objects needed to run the
  portal in-cluster: Namespace, ServiceAccount, ClusterRole, ClusterRoleBinding,
  Deployment, Service, PodDisruptionBudget, HorizontalPodAutoscaler.
- The **Deployment** runs `ui --address 0.0.0.0:8765`: the default bind stays
  loopback, and being reachable inside the cluster is an explicit argument, not a
  changed default.
- It declares **no `replicas`**. An HPA owns the replica count, and a Deployment
  that also declares it fights the autoscaler on every apply — with Flux
  re-applying the manifest, that fight is a permanent one.
- The **ClusterRole grants `get`/`list` on every resource in every group**,
  including Secrets. This is not laziness: the traversal follows a Kustomization's
  inventory into kinds nobody enumerated in advance (that is the whole design),
  and expanding a HelmRelease means reading its Helm storage Secret
  (`sh.helm.release.v1.<name>.v<n>`). The built-in `view` role excludes Secrets,
  so the HelmRelease hop would break under it. `watch` is not granted — nothing
  in the code watches — and no write verb is granted anywhere.
- The **PDB** uses `minAvailable: 1` against an HPA `minReplicas: 2`. With one
  replica, `minAvailable: 1` deadlocks a node drain forever; two replicas is what
  makes the budget meaningful rather than obstructive.
- The **HPA** targets CPU utilisation, with the resource requests it requires on
  the container. See the design for why CPU is the wrong signal for this workload
  and why it is still what ships.
- A **`securityContext`** asserting what the image already is (non-root, uid
  65532, read-only root filesystem, no privilege escalation, all capabilities
  dropped) rather than trusting the base image to stay that way.
- **Probes on `/`**, which the embedded file server answers. There is no health
  endpoint and this change does not add one: `/` returning the SPA proves the
  HTTP server is up, which is all a probe on this process can honestly assert.
- The portal's **context listing gains the in-cluster case**: when no kubeconfig
  is readable and the in-cluster config is available, it reports a single
  `in-cluster` context instead of an empty list, and selecting it resolves
  through the pod's ServiceAccount.

Deliberately excluded from this increment:
- **An Ingress, or any external exposure.** The portal has no authentication, and
  it reads the whole cluster. Putting it behind a hostname is a decision about an
  authenticating proxy, not a YAML file, and it does not belong in the change that
  first makes the thing deployable. A `ClusterIP` Service plus `kubectl
  port-forward` is how it gets looked at until then.
- **A NetworkPolicy.** Same reasoning inverted: worth having, but which ingress it
  should permit depends on how the cluster exposes things, which this change does
  not decide.
- **Overlays (`deploy/overlays/<env>`).** One base, one environment. An overlay is
  one directory away the day there are two.
- **A Flux `Kustomization` + `GitRepository` pointing at `deploy/`.** The
  manifests come first; how they get applied — Flux, `kubectl apply -k`, Argo — is
  the next decision, and `kubectl apply -k deploy/` is what validates this change.
- **A ServiceMonitor, dashboards, or any metrics.** The binary exposes none.
- **Multi-cluster.** A pod reads the cluster it runs in. The multi-context portal
  keeps working on a laptop; in-cluster it sees one cluster, which is what the
  context listing change says out loud.

## Capabilities

### New Capabilities
- `deployment-manifests`: what the in-cluster deployment consists of, the
  permissions it requires and why, how availability and scaling are declared, and
  what the change deliberately does not expose.

### Modified Capabilities
- `cli`: the requirement describing cluster connection is kubeconfig-only. It must
  state the in-cluster fallback, which is the credential path the deployment
  depends on.
- `web-ui`: two requirements change. The portal requirement's "loopback so the
  unauthenticated API is not reachable off-host" no longer tells the whole truth
  once the same binary runs behind a Service — the default must stay loopback and
  the in-cluster case must be stated with its consequence. The context-listing
  requirement must stop assuming a kubeconfig exists.

## Impact

- New `deploy/` directory: `kustomization.yaml` plus one file per object.
- `internal/k8s/contexts.go` — `ListContexts` reports the in-cluster case; it
  currently returns an empty list with no error, because a missing kubeconfig is
  not an error to `clientcmd`.
- `internal/ui/service.go` — `clientFor` maps the in-cluster context name to the
  empty context that `LoadClient` already resolves in-cluster.
- `internal/ui/web/app.js` — the context option label, which today renders
  `" (current)"` for a nameless context.
- No change to the engine, the resolvers, health derivation, or any traversal
  behaviour. No new Go dependency.
- Depends on `add-ci-pipeline`: the Deployment references
  `quay.io/hamdi_makni/fluxexp` at a published tag, and asserts the non-root uid
  that image provides.
