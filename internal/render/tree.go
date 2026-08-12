// Package render turns a resolved *engine.Node tree into human-readable text.
// It is a pure function of the tree, so it is unit-testable and reusable by a
// future UI (which would serialize the same tree instead).
package render

import (
	"fmt"
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// Tree renders the resolved graph as an indented tree. Each line shows the
// node's health, domain, type and coordinates. Error nodes are marked and carry
// their failure reason; already-visited references are annotated as leaves.
func Tree(root *engine.Node) string {
	if root == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(line(root))
	b.WriteString("\n")
	renderChildren(&b, root, "")
	return b.String()
}

func renderChildren(b *strings.Builder, node *engine.Node, prefix string) {
	for i, child := range node.Children {
		last := i == len(node.Children)-1
		connector := "├─ "
		childPrefix := prefix + "│  "
		if last {
			connector = "└─ "
			childPrefix = prefix + "   "
		}
		b.WriteString(prefix)
		b.WriteString(connector)
		b.WriteString(line(child))
		b.WriteString("\n")
		renderChildren(b, child, childPrefix)
	}
}

// line formats a single node.
func line(n *engine.Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s [%s]", glyph(n.Health), n.Ref.Label(), n.Health)
	b.WriteString(" (" + n.Ref.Domain + ")")

	switch {
	case n.Health == engine.Error && n.Err != "":
		b.WriteString(" — " + n.Err)
	case n.Detail != "":
		b.WriteString(" — " + n.Detail)
	}
	if n.Visited {
		b.WriteString(" (already visited)")
	}
	return b.String()
}

// glyph returns a status marker for a health value.
func glyph(h engine.Health) string {
	switch h {
	case engine.Healthy:
		return "✔"
	case engine.Unhealthy:
		return "✖"
	case engine.Error:
		return "!"
	default:
		return "?"
	}
}
