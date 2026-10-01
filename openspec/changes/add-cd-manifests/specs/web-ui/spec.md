## MODIFIED Requirements

### Requirement: Local no-auth portal

The system SHALL serve a web portal from an HTTP server that requires no
authentication and acts with the credentials of the process running it. The server
MUST bind to a loopback address **by default**, so the unauthenticated API is not
reachable off-host unless an operator explicitly asks for a wider bind. It MUST be
read-only: no request may mutate cluster state.

The same binary MAY be run inside a cluster with a wider bind address, in which
case it acts with its ServiceAccount's credentials and the unauthenticated API is
reachable by anything that can reach it. That is a property of how it is deployed,
not a change of default: whoever widens the bind takes on restricting who can
reach it.

#### Scenario: Server binds to loopback by default

- **WHEN** the portal server starts without an explicit bind address
- **THEN** it listens on a loopback address (e.g. `127.0.0.1`) and serves the portal there

#### Scenario: Portal reads local state without credentials prompt

- **WHEN** a user opens the portal
- **THEN** it uses the local kubeconfig to talk to the cluster, with no login step

#### Scenario: A wider bind is only ever explicit

- **WHEN** the portal is reachable from another host
- **THEN** it is because an address beyond loopback was requested, never because a default changed

#### Scenario: Deployed in a cluster it acts as its ServiceAccount

- **WHEN** the portal runs in a pod with no kubeconfig
- **THEN** it serves the same read-only portal, acting with the pod's ServiceAccount credentials

### Requirement: Context listing and selection

The portal SHALL list the kube contexts available to it, indicate the current one,
and let the user select which context subsequent requests target. All data shown
MUST reflect the selected context.

When no kubeconfig is readable and in-cluster credentials are available, the portal
MUST report exactly one context, named to identify it as the cluster the portal
runs in and marked current, rather than an empty list. An empty selector is a dead
end that says nothing about why: a missing kubeconfig is not an error to report,
it is a different, single-cluster mode to name.

Selecting that context MUST resolve through the in-cluster credentials. In this
mode there is one cluster and the selector offers no alternative — the portal sees
the cluster it runs in.

When neither a kubeconfig nor in-cluster credentials are available, the portal MUST
report the failure rather than presenting an empty selector.

#### Scenario: Contexts are listed with the current one marked

- **WHEN** the portal loads with a kubeconfig containing contexts
- **THEN** it shows those contexts and marks the current-context

#### Scenario: Switching context changes the data source

- **WHEN** the user selects a different context
- **THEN** subsequent listings and expansions target the newly selected context

#### Scenario: In a pod, one named context is offered

- **WHEN** the portal loads with no readable kubeconfig and in-cluster credentials available
- **THEN** the selector shows a single context identifying the cluster the portal runs in, marked current

#### Scenario: The in-cluster context resolves

- **WHEN** roots are listed or a node is expanded under that context
- **THEN** the request is served with the in-cluster credentials

#### Scenario: No credentials at all is reported, not shown as empty

- **WHEN** the portal loads with neither a kubeconfig nor in-cluster credentials
- **THEN** it reports that it cannot list contexts, instead of rendering an empty selector
