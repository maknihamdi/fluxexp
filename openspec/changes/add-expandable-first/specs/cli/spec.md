## MODIFIED Requirements

### Requirement: Tree rendering with health

The command SHALL render the resolved graph as an indented tree. Each node MUST
show its domain, type, coordinates (e.g. namespace/name), and health status.
Error nodes MUST be visually distinguishable and MUST show their failure reason.
An **expandable** node MUST carry a leading chevron marker, and non-expandable
nodes MUST reserve the same width so that labels stay column-aligned. A node's
children MUST be listed in the shared three-tier order: Flux objects, then other
expandable children, then the rest. A node's **dependencies** MUST be rendered
before its children, each carrying a marker distinguishing it from an expandable
child, so a reader can tell what the node requires from what it produces.

#### Scenario: Tree shows nodes with health

- **WHEN** the traversal produces a root with nested children of varying health
- **THEN** the output is an indented tree where each line shows the node's domain, type, coordinates, and health status

#### Scenario: Error node shows its reason

- **WHEN** the tree contains an error node
- **THEN** that node is rendered distinctly and its failure reason is shown inline

#### Scenario: Expandable node carries the chevron

- **WHEN** a rendered node is expandable
- **THEN** its line shows a chevron marker before the label, and a non-expandable sibling shows blank padding of the same width in its place

#### Scenario: Children are printed in tier order

- **WHEN** a node's children mix Flux objects, other expandable references, and leaves
- **THEN** the Flux objects are printed first, then the other containers, then the leaves, each tier keeping its original relative order



#### Scenario: Dependencies are rendered before children with their own marker

- **WHEN** a Kustomization declares a source dependency and applies inventory children
- **THEN** the source is printed first, carrying the dependency marker rather than the expandable chevron, followed by the children in tier order

#### Scenario: Dependency fields are shown inline

- **WHEN** a rendered dependency is a GitRepository carrying repository, branch and interval fields
- **THEN** those fields appear on its line without any further interaction
