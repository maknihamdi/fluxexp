package engine

// Result is what a resolver returns for a single reference: the object's health,
// an optional human-readable detail, and the child references to traverse next.
// Children MAY belong to a different domain than the resolved reference.
type Result struct {
	Health Health
	Detail string
	// Freshness is an optional second status (reconciliation freshness); empty
	// when a resolver does not set it.
	Freshness Freshness
	// Fields are optional ordered label/value extras surfaced by the resolver.
	Fields []Field
	// Expandable reports whether this reference can descend to a child layer.
	// Like Health and Freshness it is set by the resolver side and only carried
	// by the engine, which never interprets it.
	Expandable bool
	// Dependencies are references this node requires in order to reconcile, as
	// opposed to Children, which it produces. The engine resolves them one level
	// deep and never descends into them, so declaring a dependency on a node
	// that has a large subtree of its own stays cheap.
	Dependencies []Ref
	Children     []Ref
}

// Node is the engine's output: one vertex of the resolved tree. It wraps the
// reference identity, resolved health/detail, an optional error (set when the
// node could not be retrieved or resolved), and the resolved child nodes.
type Node struct {
	Ref    Ref
	Health Health
	Detail string
	// Err holds the failure reason for an Error node; empty otherwise.
	Err string
	// Freshness is the optional second status carried from the resolver.
	Freshness Freshness
	// Fields are the optional ordered label/value extras from the resolver.
	Fields []Field
	// Expandable is carried from the resolver: the node can descend to a child
	// layer. It describes the reference's type, so an expandable node may still
	// have no children.
	Expandable bool
	// Visited is true when this reference was already expanded elsewhere in the
	// graph; such a node is a leaf pointer and is not re-expanded.
	Visited bool
	// Dependency is true when this node was reached as something its parent
	// requires rather than something it produces.
	Dependency bool
	// Dependencies are the resolved nodes this one requires. They carry health,
	// detail and fields but never children: the engine does not descend into a
	// dependency.
	Dependencies []*Node
	Children     []*Node

	// deps carries the dependency references between resolution and their own
	// resolution; it is cleared once Dependencies is populated.
	deps []Ref
}

// ResolveFunc resolves a single reference into a Result. The engine treats a
// non-nil error as a per-node failure (it becomes an Error node) and keeps
// traversing siblings. It performs no backend I/O itself.
type ResolveFunc func(ref Ref) (Result, error)
