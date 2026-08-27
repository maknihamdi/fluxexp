## ADDED Requirements

### Requirement: Exploration state is addressable by URL

The portal SHALL encode the current exploration path in the page URL, so that
every node reached by drilling has an address that can be copied, bookmarked and
shared. The URL MUST carry the selected kube context and the ordered sequence of
hops from the roots home to the current node, each hop identified by enough
information to resolve it (its domain when not the default, its type, its
namespace and its name).

Opening such a URL — by reload, by pasting it in another tab, or from someone
else's message — MUST restore the same view: the current node rendered and the
full breadcrumb trail available. Restoring a trail MUST NOT resolve the
intermediate hops; only the current node is expanded.

The display label MUST NOT be carried in the URL: it is derived from the hop's
type, so a shared link cannot show a stale label.

#### Scenario: Drilling changes the URL

- **WHEN** the user drills from the roots home into a Kustomization and then into one of its children
- **THEN** the URL reflects the two-hop path, alongside the selected context

#### Scenario: Reloading a deep URL restores the view

- **WHEN** the user reloads the page on a deep exploration URL
- **THEN** the portal renders the same current node with its full breadcrumb, instead of returning to the roots home

#### Scenario: A shared URL opens on the same node

- **WHEN** a user opens an exploration URL produced by another session against the same cluster
- **THEN** the portal opens on that node with its breadcrumb, without the sender having to describe the path

#### Scenario: Restoring a trail costs one expansion

- **WHEN** a five-hop exploration URL is opened
- **THEN** only the last hop is expanded, the earlier hops being shown in the breadcrumb from their own identifiers

#### Scenario: The roots home has its own address

- **WHEN** the user is on the roots home
- **THEN** the URL carries the context and no exploration path, so reloading returns to the roots home

#### Scenario: An unparseable path falls back to the home

- **WHEN** the page is opened with an exploration path that cannot be decoded
- **THEN** the portal shows the roots home and reports the problem, rather than rendering a broken trail

#### Scenario: Switching context clears the path

- **WHEN** the user selects a different kube context while exploring
- **THEN** the portal returns to the roots home for that context and the URL no longer carries the previous trail

### Requirement: Browser history navigates the exploration trail

The portal SHALL add a browser history entry for each navigation it performs:
drilling into a row or a dependency, jumping to an entry of the breadcrumb,
moving up to the parent, switching context. Using the browser's Back and Forward
controls MUST move along the exploration trail and MUST NOT leave the portal,
with the rendered view always matching the URL being restored.

#### Scenario: Back returns to the previous node

- **WHEN** the user has drilled two levels deep and presses the browser's Back control
- **THEN** the portal renders the previous level, still inside the portal

#### Scenario: Forward returns to the node just left

- **WHEN** the user presses Back and then Forward
- **THEN** the portal renders the node they had left

#### Scenario: History and URL never disagree

- **WHEN** any history entry is restored
- **THEN** the rendered breadcrumb and current node are those encoded in that entry's URL

### Requirement: Up-to-parent control

While a node is open, the portal SHALL offer a control that moves up one level,
to the node the current one was reached from. Activating it MUST be a forward
navigation — reached whatever the previous history entry was — not a replay of
the browser's Back. On the roots home, where there is no parent, the control MUST
NOT be offered.

#### Scenario: Moving up one level

- **WHEN** the user activates the up control on a node two levels deep
- **THEN** the portal renders its parent, and the URL and breadcrumb reflect the shorter path

#### Scenario: Up from the first level reaches the roots home

- **WHEN** the user activates the up control on a node opened from the roots home
- **THEN** the portal renders the roots home

#### Scenario: No parent control on the home

- **WHEN** the user is on the roots home
- **THEN** no up control is shown

#### Scenario: Up is independent of how the node was reached

- **WHEN** the user jumps to a node from the breadcrumb and then activates the up control
- **THEN** the portal renders that node's parent in the trail, not the node visited before the jump

## MODIFIED Requirements

### Requirement: Drill-in trail and new exploration

The portal SHALL support two navigation modes. In **drill-in** mode the user
stays on the current page and the path traversed is kept as a breadcrumb trail
so they can see and return to where they came from; that trail MUST be reflected
in the page URL, so it survives a reload and can be shared. Alternatively the
user MAY **open a new exploration**, which starts a fresh page — addressed by
that resource's own exploration URL — and leaves the current one.

#### Scenario: Drill-in keeps a breadcrumb trail

- **WHEN** the user drills from a root into a child and then a grandchild on the same page
- **THEN** a breadcrumb shows the path root → child → grandchild and lets the user navigate back to any prior level

#### Scenario: New exploration starts fresh

- **WHEN** the user chooses to open a new exploration from a resource
- **THEN** a new exploration page begins from that resource, separate from the current page's trail

#### Scenario: A new exploration is itself addressable

- **WHEN** a new exploration is opened from a resource
- **THEN** its page carries that resource's exploration URL, so it can be reloaded and shared like any other
