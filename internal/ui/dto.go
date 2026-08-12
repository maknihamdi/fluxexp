package ui

import (
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// RefDTO is the JSON encoding of an engine.Ref (kubernetes domain in this
// increment). It round-trips to/from the API so the frontend can request
// expansion of a specific node.
type RefDTO struct {
	Domain    string `json:"domain"`
	Type      string `json:"type"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Display   string `json:"display"`
}

// NodeDTO is a resolved node plus its immediate (unexpanded) children.
type NodeDTO struct {
	Ref      RefDTO    `json:"ref"`
	Health   string    `json:"health"`
	Detail   string    `json:"detail,omitempty"`
	Err      string    `json:"error,omitempty"`
	Children []NodeDTO `json:"children,omitempty"`
	// Context is the kube context this node was resolved under.
	Context string `json:"context"`
}

// RootDTO is a root Kustomization summary for the home page.
type RootDTO struct {
	Ref            RefDTO `json:"ref"`
	Health         string `json:"health"`
	SourceKind     string `json:"sourceKind,omitempty"`
	SourceName     string `json:"sourceName,omitempty"`
	Path           string `json:"path,omitempty"`
	Interval       string `json:"interval,omitempty"`
	Revision       string `json:"revision,omitempty"`
	LastTransition string `json:"lastTransition,omitempty"`
	Message        string `json:"message,omitempty"`
}

// refToDTO converts an engine.Ref (kubernetes) to its JSON form with a friendly
// display label ("<Kind> <namespace>/<name>"), independent of how the ref was
// constructed (so a ref rebuilt from query params still reads well).
func refToDTO(r engine.Ref) RefDTO {
	ns := r.Coords["namespace"]
	name := r.Coords["name"]
	return RefDTO{
		Domain:    r.Domain,
		Type:      r.Type,
		Namespace: ns,
		Name:      name,
		Display:   friendlyLabel(r.Type, ns, name),
	}
}

// friendlyLabel renders "<Kind> <namespace>/<name>" (or "<Kind> <name>" when
// cluster-scoped). Kind is the part of a kubernetes type after "<apiVersion>|".
func friendlyLabel(typ, namespace, name string) string {
	kind := typ
	if i := strings.LastIndex(typ, "|"); i >= 0 {
		kind = typ[i+1:]
	}
	if namespace != "" {
		return kind + " " + namespace + "/" + name
	}
	if name != "" {
		return kind + " " + name
	}
	return kind
}

// dtoToRef rebuilds an engine.Ref from its JSON form.
func dtoToRef(d RefDTO) engine.Ref {
	return engine.Ref{
		Domain:  d.Domain,
		Type:    d.Type,
		Coords:  map[string]string{"namespace": d.Namespace, "name": d.Name},
		Display: d.Display,
	}
}
