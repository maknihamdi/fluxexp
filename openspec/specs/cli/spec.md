# cli Specification

## Purpose
TBD - created by archiving change add-traversal-engine. Update Purpose after archive.
## Requirements
### Requirement: List command

The CLI SHALL provide a `list` command that lists Flux resources of a given kind
together with their health, so a user can discover starting points before
traversing. The user MUST be able to choose the kind and the scope (all
namespaces or a single namespace), and SHALL be able to filter the output to only
unhealthy resources. Each row MUST show the resource's namespace/name and health.

#### Scenario: List resources of a kind with health

- **WHEN** the user runs the list command for a kind across all namespaces
- **THEN** the command prints one row per resource, each showing its namespace/name and health status

#### Scenario: Filter to unhealthy only

- **WHEN** the user runs the list command with the unhealthy-only filter
- **THEN** only resources whose health is not healthy are printed

#### Scenario: Unresolvable kind is reported

- **WHEN** the user requests a kind that cannot be resolved to a cluster resource
- **THEN** the command exits non-zero with a clear message

### Requirement: Traverse command

The system SHALL provide a `fluxexp` CLI with a command that traverses the
resource graph from a starting resource. The user MUST be able to specify the
start resource by kind, namespace, and name; the command builds the root
reference in the `kubernetes` domain from those inputs. The command SHALL run the
traversal engine and print the resulting resource tree.

#### Scenario: User traverses from a Kustomization

- **WHEN** the user runs the traverse command specifying a Kustomization's kind, namespace, and name
- **THEN** the command connects to the cluster, resolves the graph starting from that Kustomization, and prints the resource tree

#### Scenario: Missing start resource is reported

- **WHEN** the user specifies a start resource that does not exist in the cluster
- **THEN** the command exits non-zero with a clear message naming the resource it could not find

### Requirement: Cluster connection via kubeconfig

The CLI MUST connect to the cluster using the standard kubeconfig resolution
(the `KUBECONFIG` environment variable or the default path), and SHALL allow
selecting the context. It MUST fail with a clear message when no cluster is
reachable.

#### Scenario: Default kubeconfig is used

- **WHEN** the user runs a command without specifying a kubeconfig path
- **THEN** the CLI resolves credentials from the standard kubeconfig location

#### Scenario: Unreachable cluster reported clearly

- **WHEN** the target cluster cannot be reached
- **THEN** the command exits non-zero with a message indicating the connection failure

### Requirement: Tree rendering with health

The command SHALL render the resolved graph as an indented tree. Each node MUST
show its domain, type, coordinates (e.g. namespace/name), and health status.
Error nodes MUST be visually distinguishable and MUST show their failure reason.

#### Scenario: Tree shows nodes with health

- **WHEN** the traversal produces a root with nested children of varying health
- **THEN** the output is an indented tree where each line shows the node's domain, type, coordinates, and health status

#### Scenario: Error node shows its reason

- **WHEN** the tree contains an error node
- **THEN** that node is rendered distinctly and its failure reason is shown inline

