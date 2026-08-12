package engine

// Health is a domain-independent status for a node in the resolved graph.
type Health string

const (
	// Healthy means the resource reports itself as ready/working.
	Healthy Health = "healthy"
	// Unhealthy means the resource reports a not-ready/failed state.
	Unhealthy Health = "unhealthy"
	// Unknown means health could not be determined (e.g. no Ready condition).
	Unknown Health = "unknown"
	// Error means the node itself could not be retrieved or resolved.
	Error Health = "error"
)
