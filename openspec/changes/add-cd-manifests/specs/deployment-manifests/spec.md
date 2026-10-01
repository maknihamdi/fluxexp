## ADDED Requirements

### Requirement: A kustomize base deploys the portal in-cluster

The repository SHALL provide a kustomize base that deploys the portal into the
cluster it inspects, applicable with a single `kubectl apply -k` and requiring no
patch, no overlay and no external chart.

The base MUST be self-contained: it declares its own namespace, and every
selector — Deployment, Service, PodDisruptionBudget — MUST be generated from one
label definition rather than written three times, so a selector cannot silently
match nothing.

The deployed container MUST run the portal bound to all interfaces, passed as an
explicit argument. The default bind address MUST remain loopback, so reachability
inside a cluster is something a manifest asks for and never something a version
bump grants.

The image reference MUST name a published version, never a moving tag.

#### Scenario: The base applies cleanly

- **WHEN** the base is applied to a cluster with `kubectl apply -k`
- **THEN** every object is accepted and the portal becomes reachable on its Service

#### Scenario: Selectors are consistent by construction

- **WHEN** the rendered manifests are inspected
- **THEN** the Deployment's pod labels, the Service's selector and the PodDisruptionBudget's selector all match, having been generated from one definition

#### Scenario: The portal binds beyond loopback only because the manifest says so

- **WHEN** the Deployment's container arguments are read
- **THEN** they contain an explicit address on all interfaces, and the binary's own default remains loopback

#### Scenario: The image is pinned

- **WHEN** the Deployment's image is read
- **THEN** it names a specific released version and not a moving tag

### Requirement: The pod's permissions are cluster-wide read, and nothing else

The deployment SHALL run under its own ServiceAccount, bound to a ClusterRole
granting `get` and `list` on all resources in all API groups. No write verb and no
`watch` may be granted.

Read breadth is a requirement, not a convenience: the traversal follows a
Kustomization's inventory into kinds that are not knowable when the ClusterRole is
written, including CRDs of operators this build has never seen, and an unlisted
kind would surface as a failure rather than as a permission gap. Expanding a
HelmRelease MUST work, which requires reading its Helm release storage Secret —
so Secret read access is part of the requirement and the built-in `view` role is
insufficient.

The consequence MUST be stated where the permission is granted: the pod can read
every Secret in the cluster, and the portal is unauthenticated, so anyone who
reaches it reads through that permission.

The Secret grant MUST be expressed as a rule separate from the rest, so that an
operator who refuses it can remove it in place, accepting that HelmRelease
expansion stops working. Narrowing it further is not possible: authorization
cannot restrict reads to Secrets matching a name pattern while still permitting
`list`.

The absence of write verbs MUST make the read-only invariant enforceable by the
API server, not only by the code.

#### Scenario: An arbitrary CRD in an inventory is readable

- **WHEN** a Kustomization's inventory names a custom resource of an operator the build does not know
- **THEN** the pod can read it and it is reported with a health status, not as a permission error

#### Scenario: A HelmRelease expands

- **WHEN** a HelmRelease is expanded through the deployed portal
- **THEN** its Helm storage Secret is read and the objects its chart deployed are listed

#### Scenario: No mutation is possible

- **WHEN** the granted permissions are inspected
- **THEN** they contain no create, update, patch, delete or watch verb for any resource

#### Scenario: The Secret grant is removable

- **WHEN** an operator deletes the rule granting Secret access
- **THEN** every other kind still resolves and only HelmRelease expansion stops working

### Requirement: Availability and scaling are declared coherently

The deployment SHALL declare a PodDisruptionBudget requiring at least one
available pod, and a HorizontalPodAutoscaler whose minimum is at least two
replicas. The two numbers are one decision: at a single replica, requiring one
available pod makes a voluntary eviction impossible forever, so the budget is only
meaningful above one.

The Deployment MUST NOT declare a replica count. The autoscaler owns it, and a
Deployment that also declares it is reset on every apply and scaled back by the
autoscaler — a flap that costs a rollout each time, permanently, when a
reconciler re-applies the manifest.

The pods MUST be spread across nodes on a best-effort basis, without blocking
scheduling when the cluster cannot satisfy it: two replicas on one node make the
budget decorative, since one drain violates it.

The autoscaler's maximum MUST be bounded as a limit on aggregate API-server
pressure rather than sized for capacity. The rate limiter that makes a single
portal layer fast is per process, so each replica multiplies the requests the
cluster's API server receives.

The autoscaler target MUST be documented as a compromise: the workload waits on
the API server rather than on local compute, so CPU utilisation stays low exactly
when the portal feels slow. It is what ships because it needs no metric adapter
and no application metrics, and it does give capacity for concurrent users.

#### Scenario: A node drain evicts one pod at a time

- **WHEN** a node hosting one of the pods is drained
- **THEN** the eviction is allowed and a second pod remains available

#### Scenario: A single replica would never be evictable

- **WHEN** the autoscaler's minimum and the budget are read together
- **THEN** the minimum is at least two, so the budget cannot block every eviction

#### Scenario: The autoscaler is the only owner of the replica count

- **WHEN** the Deployment manifest is read
- **THEN** it declares no replica count, and re-applying it does not change the running replica count

#### Scenario: Scaling out is bounded

- **WHEN** the autoscaler's maximum is read
- **THEN** it is bounded, with its rationale recorded as a limit on total API-server pressure

### Requirement: The container asserts its own confinement

The pod SHALL declare a security context that asserts, rather than assumes, what
the image provides: running as a non-root user with an explicit uid, a read-only
root filesystem, no privilege escalation, all capabilities dropped, and the
runtime's default seccomp profile.

These assertions are redundant against the image as built, and that is their
purpose: they fail loudly if the base image ever stops providing them, instead of
a property being lost unnoticed.

The read-only root filesystem MUST hold without a writable volume. Nothing in the
process writes to disk — in particular, discovery is performed by a client that
keeps no on-disk cache. No scratch volume may be added pre-emptively, because it
would mask the moment something starts writing.

#### Scenario: The pod runs as non-root with a read-only filesystem

- **WHEN** the pod is running
- **THEN** its process runs under a non-root uid and cannot write to its root filesystem

#### Scenario: A base image losing non-root fails the pod

- **WHEN** the image is rebuilt on a base that runs as root
- **THEN** the pod fails to start rather than running as root

#### Scenario: No scratch volume is mounted

- **WHEN** the Deployment's volumes are read
- **THEN** there is none, and the portal serves correctly without one

### Requirement: Probes assert the server, not the cluster

The container SHALL declare readiness and liveness probes that request the
portal's root path, which the embedded file server answers.

The probes MUST NOT check cluster reachability. A probe that did would take every
replica down during an API-server disruption, whereas the portal's specified
behaviour then is to serve and report the failure per node — partial failure is a
first-class result. No dedicated health endpoint may be added, because it would
assert exactly what the root path already asserts.

#### Scenario: A serving portal is ready

- **WHEN** the HTTP server is listening
- **THEN** the readiness probe succeeds and the pod receives traffic

#### Scenario: An API-server disruption does not restart the pods

- **WHEN** the Kubernetes API server is unreachable from the pod
- **THEN** the probes keep succeeding and the portal reports the failure in its results

### Requirement: The deployment is not exposed outside the cluster

The base SHALL expose the portal on a cluster-internal Service only. It MUST NOT
include an Ingress, a LoadBalancer Service, or any other external entry point.

The portal has no authentication and reads the whole cluster: exposing it on a
hostname is a decision about an authenticating proxy, not a manifest, and MUST
remain a separate, explicit change. Until then, access requires credentials for
the cluster itself, by port-forwarding to the Service.

#### Scenario: The Service is cluster-internal

- **WHEN** the Service's type is read
- **THEN** it is cluster-internal, with no external address allocated

#### Scenario: No external entry point is created

- **WHEN** the rendered manifests are inspected
- **THEN** they contain no Ingress and no LoadBalancer

#### Scenario: Viewing the portal requires cluster credentials

- **WHEN** a user wants to open the portal
- **THEN** they port-forward to the Service, which requires them to already be authorized against the cluster
