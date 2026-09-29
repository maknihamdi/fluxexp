package engine

// Health is a domain-independent status for a node in the resolved graph.
type Health string

const (
	// Healthy means the resource reports itself as ready/working.
	Healthy Health = "healthy"
	// Unhealthy means the resource reports a not-ready/failed state.
	Unhealthy Health = "unhealthy"
	// Pending means the backend has not caught up with the resource's declared
	// spec, so its reported state describes a superseded one. It is neither a
	// working resource nor a broken one, and it is distinct from Unknown: the
	// state is perfectly readable, it is simply not the state of what was asked
	// for.
	Pending Health = "pending"
	// Unknown means health could not be determined: the resource publishes a
	// state that cannot be interpreted. It is a last resort, not a default.
	Unknown Health = "unknown"
	// Error means the node itself could not be retrieved or resolved.
	Error Health = "error"
)
