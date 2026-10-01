## MODIFIED Requirements

### Requirement: Cluster connection via kubeconfig

The CLI MUST connect to the cluster using the standard kubeconfig resolution
(the `KUBECONFIG` environment variable or the default path), and SHALL allow
selecting the context. When no kubeconfig can be resolved and the process is
running inside a cluster, it MUST fall back to the in-cluster credentials — the
ServiceAccount token mounted into the pod — so the same binary works unchanged
when deployed. It MUST fail with a clear message when no cluster is reachable by
either path.

An explicitly requested context MUST NOT be silently ignored: asking for a context
that does not exist is an error, not a reason to fall back to the in-cluster
credentials.

#### Scenario: Default kubeconfig is used

- **WHEN** the user runs a command without specifying a kubeconfig path
- **THEN** the CLI resolves credentials from the standard kubeconfig location

#### Scenario: In-cluster credentials are used when there is no kubeconfig

- **WHEN** the binary runs in a pod with no kubeconfig available and no context requested
- **THEN** it connects with the pod's ServiceAccount credentials

#### Scenario: A named context that does not exist is an error

- **WHEN** a context is explicitly requested and the resolved kubeconfig does not contain it
- **THEN** the command fails naming the missing context, rather than connecting to anything else

#### Scenario: Unreachable cluster reported clearly

- **WHEN** the target cluster cannot be reached
- **THEN** the command exits non-zero with a message indicating the connection failure
