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
	Fields   []Field
	Children []Ref
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
	// Visited is true when this reference was already expanded elsewhere in the
	// graph; such a node is a leaf pointer and is not re-expanded.
	Visited  bool
	Children []*Node
}

// ResolveFunc resolves a single reference into a Result. The engine treats a
// non-nil error as a per-node failure (it becomes an Error node) and keeps
// traversing siblings. It performs no backend I/O itself.
type ResolveFunc func(ref Ref) (Result, error)
