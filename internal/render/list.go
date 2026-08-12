package render

import (
	"fmt"
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// ResourceStatus is one row for the list command.
type ResourceStatus struct {
	Namespace string
	Name      string
	Health    engine.Health
	Detail    string
}

// List renders resource statuses as aligned rows: "<glyph> <health>  <ns/name>  <detail>".
// An empty slice yields a single "no resources" line.
func List(items []ResourceStatus) string {
	if len(items) == 0 {
		return "no resources\n"
	}

	// Width of the namespace/name column for alignment.
	width := 0
	for _, it := range items {
		if l := len(it.Namespace + "/" + it.Name); l > width {
			width = l
		}
	}

	var b strings.Builder
	for _, it := range items {
		nsName := it.Namespace + "/" + it.Name
		fmt.Fprintf(&b, "%s %-11s %-*s", glyph(it.Health), "["+string(it.Health)+"]", width, nsName)
		if it.Detail != "" {
			b.WriteString("  " + it.Detail)
		}
		b.WriteString("\n")
	}
	return b.String()
}
