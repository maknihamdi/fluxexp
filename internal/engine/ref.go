package engine

import (
	"sort"
	"strings"
)

// Ref is a domain-agnostic reference to a node in the resource graph. It makes
// no assumption about Kubernetes: a node may live in domain "kubernetes",
// "gcp", or any future backend. The engine only ever compares Refs by Key().
type Ref struct {
	// Domain names the backend the object lives in, e.g. "kubernetes", "gcp".
	Domain string
	// Type is an opaque type identifier meaningful within Domain (a GVK string
	// for kubernetes, a resource type for a cloud, ...).
	Type string
	// Coords locate the object within its domain (e.g. namespace/name).
	Coords map[string]string
	// Display is an optional human-friendly label used for rendering.
	Display string
}

// Key returns a stable identity for the reference, independent of Coords map
// ordering. It is used for deduplication and cycle detection.
func (r Ref) Key() string {
	keys := make([]string, 0, len(r.Coords))
	for k := range r.Coords {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(r.Domain)
	b.WriteString("|")
	b.WriteString(r.Type)
	for _, k := range keys {
		b.WriteString("|")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(r.Coords[k])
	}
	return b.String()
}

// Label returns a human-friendly identifier: the explicit Display when set,
// otherwise a compact "type coords" form.
func (r Ref) Label() string {
	if r.Display != "" {
		return r.Display
	}
	keys := make([]string, 0, len(r.Coords))
	for k := range r.Coords {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, r.Coords[k])
	}
	coords := strings.Join(parts, "/")
	if coords == "" {
		return r.Type
	}
	return r.Type + " " + coords
}
