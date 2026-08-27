## MODIFIED Requirements

### Requirement: Layer-by-layer exploration

From any resource the portal SHALL let the user expand **one layer at a time**:
selecting a resource reveals its immediate children (each with health), and any
child that itself has children can be expanded in turn. Expansion MUST resolve a
single hop per interaction (not the whole subtree at once), reusing the same
resolvers as the CLI. A revealed layer MUST follow the shared three-tier order —
Flux objects, then other expandable children, then the rest — using the same
ordering rule as the CLI. A resource that could not be resolved MUST be shown as
an error node with its reason.

#### Scenario: Expanding a Kustomization reveals its inventory children

- **WHEN** the user expands a Kustomization
- **THEN** the portal shows that Kustomization's immediate children (its inventory objects) with health, without expanding deeper levels

#### Scenario: Expanding a HelmRelease reveals its deployed objects

- **WHEN** the user expands a HelmRelease
- **THEN** the portal shows the objects the release deployed, with health

#### Scenario: Layer follows the tier order

- **WHEN** a revealed layer mixes a GitRepository (Flux), a Deployment (expandable) and a ConfigMap (leaf)
- **THEN** they appear in that order, each tier keeping its original relative order

#### Scenario: Unresolvable node shown as error

- **WHEN** expanding a reference fails to resolve
- **THEN** that node is shown as an error with its reason, and sibling expansion still works

## ADDED Requirements

### Requirement: Expandability exposed by the node API

The portal's node API SHALL expose an **expandable** flag for the resolved node
and for each of its children, so the frontend does not re-derive type rules. The
flag MUST come from the shared registry lookup, the same one the CLI uses. The
API SHALL additionally expose the node's **dependencies** as a list separate from
its children, each carrying health, detail and fields.

#### Scenario: Flag is present on node and children

- **WHEN** a node is expanded through the API
- **THEN** the response carries the expandable flag for the node itself and for every child

#### Scenario: Flag matches the resolver declaration

- **WHEN** a child is a Pod and a sibling is a HelmRelease
- **THEN** the Pod's flag is false and the HelmRelease's flag is true

### Requirement: Expandable children are visually marked

The portal SHALL mark an expandable child with a chevron before its name and an
accent on its card, visually distinct from the health and freshness badges, so a
user can tell at a glance which children are worth opening. A non-expandable
child MUST NOT carry the marker.

#### Scenario: Container child is marked

- **WHEN** a revealed layer contains an expandable child
- **THEN** that child's card shows the chevron and the accent

#### Scenario: Leaf child is not marked

- **WHEN** a revealed layer contains a non-expandable child such as a ConfigMap
- **THEN** that child's card shows neither the chevron nor the accent

### Requirement: Kustomization and its dependencies render as one card

The portal SHALL render a node and its dependencies as a **single card**: the
dependencies appear nested inside the node's own card, visually subordinate to
it, and MUST NOT also appear in the list of children. The children list SHALL be
presented as what the node applies, separate from the card.

#### Scenario: The source is inside the card, not in the children list

- **WHEN** the user opens a Kustomization that declares a GitRepository dependency
- **THEN** the GitRepository appears nested within the Kustomization's card, and does not appear among the children below

#### Scenario: Dependencies carry their own health

- **WHEN** a Kustomization depends on another Kustomization that is not ready
- **THEN** that dependency shows inside the card with its own unhealthy status and message

#### Scenario: A node without dependencies shows no dependency block

- **WHEN** the user opens a node that declares no dependencies
- **THEN** the card shows the node alone, with no empty dependency section

### Requirement: Dependencies are reachable from the card

Each dependency rendered inside a card SHALL be selectable, opening it as the
current node so the user can continue from it.

#### Scenario: Clicking a dependency drills into it

- **WHEN** the user selects a dependency shown inside a card
- **THEN** the portal opens that object as the current node

### Requirement: Flux fields are shown inline in lists

The portal SHALL show a Flux source or image-automation object's fields inline
wherever it appears in a list — as a dependency or as a child — without requiring
the user to open it. Those fields MUST be derived from the object already
retrieved for its health, without an additional call.

#### Scenario: A listed HelmRepository shows its repository and interval

- **WHEN** a children list contains a HelmRepository
- **THEN** its repository and interval are visible on its row

#### Scenario: A non-Flux child shows no fields

- **WHEN** a children list contains a ConfigMap
- **THEN** its row shows health and detail only, with no field block

### Requirement: Listed nodes show their own dependencies

The portal SHALL group a listed node's own dependencies with it, not only those
of the node currently opened. When a listed entry declares dependencies in its
own manifest — a Kustomization with a `spec.sourceRef` or `spec.dependsOn` — they
MUST appear grouped under it, each with its health and fields.

The portal MUST derive those references from the object it **already fetched for
that entry's health**, and MUST NOT resolve the entry or compute its children to
obtain them. Each derived dependency MUST then be retrieved once, for its health
and fields only.

#### Scenario: A child Kustomization shows its source in the list

- **WHEN** a revealed layer contains a Kustomization that references a GitRepository
- **THEN** that GitRepository appears grouped under the child Kustomization, with its repository, branch and interval

#### Scenario: A child Kustomization shows what it waits on

- **WHEN** a listed Kustomization declares `dependsOn` entries and one of them is not ready
- **THEN** that entry appears grouped under it with its unhealthy status

#### Scenario: A listed entry's children are never computed

- **WHEN** a revealed layer contains a Kustomization with a large inventory
- **THEN** its dependencies are shown from its manifest, and its inventory is not read or resolved

#### Scenario: Kinds without dependencies show no group

- **WHEN** a revealed layer contains ConfigMaps and a HelmRelease
- **THEN** none of them shows a dependency group

#### Scenario: Nesting stops at one level

- **WHEN** a dependency shown under a listed child would itself have dependencies
- **THEN** they are not resolved, so the list nests at most one level deep

### Requirement: A layer shows each reference once

The portal SHALL show a reference at most once per rendered layer. When an entry
is already displayed as another entry's nested dependency, it MUST NOT also
appear as a top-level row of that same layer — the grouped occurrence wins,
because it is the one that says why the reference is there.

An entry that itself carries a dependency group MUST be kept regardless, so that
two entries depending on each other cannot make both disappear.

#### Scenario: A source applied alongside its Kustomization appears once

- **WHEN** a Kustomization applies both a child Kustomization and the GitRepository that child reconciles from
- **THEN** the GitRepository appears only nested under the child Kustomization, not as a separate row of the same layer

#### Scenario: An entry carrying its own group is never dropped

- **WHEN** a listed entry is also another entry's dependency but carries a dependency group of its own
- **THEN** it is kept as a top-level row

#### Scenario: References used only once are untouched

- **WHEN** no entry of a layer duplicates another entry's dependency
- **THEN** every entry is rendered

