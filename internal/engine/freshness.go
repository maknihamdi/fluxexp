package engine

// Freshness is a second, health-independent status a resolver MAY set to express
// whether a node's applied state matches its desired source (a GitOps concept).
// The engine never computes it — it carries whatever a resolver sets, exactly as
// it carries Health. An empty value means "not applicable".
type Freshness string

const (
	// UpToDate: the applied revision matches the source's available revision.
	UpToDate Freshness = "up-to-date"
	// Behind: the source has a revision that has not been applied yet.
	Behind Freshness = "behind"
	// Failed: the latest reconciliation/apply failed.
	Failed Freshness = "failed"
	// Suspended: reconciliation is suspended.
	Suspended Freshness = "suspended"
)

// Field is an ordered label/value pair a resolver may attach to a node to surface
// extra information (revisions, timestamps, …). Value is the display form
// (possibly shortened); Full is the optional complete value (e.g. the full
// revision) for tooltips. It is opaque to the engine.
type Field struct {
	Label string
	Value string
	Full  string
}
