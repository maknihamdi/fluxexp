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
	// Dependencies come first — what the node needs before what it produces —
	// and are never descended into, so they render as leaves.
	total := len(node.Dependencies) + len(node.Children)
	for i, dep := range node.Dependencies {
		connector, _ := connectors(i, total)
		b.WriteString(prefix)
		b.WriteString(connector)
		b.WriteString(line(dep))
		b.WriteString("\n")
	}
	for i, child := range node.Children {
		connector, suffix := connectors(len(node.Dependencies)+i, total)
		childPrefix := prefix + suffix
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
	fmt.Fprintf(&b, "%s %s%s [%s]", glyph(n.Health), marker(n), n.Ref.Label(), n.Health)
	if n.Freshness != "" {
		b.WriteString(" [" + string(n.Freshness) + "]")
	}
	b.WriteString(" (" + n.Ref.Domain + ")")

	detail := n.Detail
	if len(n.Fields) > 0 {
		detail = fieldsSummary(n.Fields)
	}
	switch {
	case n.Health == engine.Error && n.Err != "":
		b.WriteString(" — " + n.Err)
	case detail != "":
		b.WriteString(" — " + detail)
	}
	if n.Visited {
		b.WriteString(" (already visited)")
	}
	return b.String()
}

// fieldsSummary renders fields compactly as "label value · label value".
func fieldsSummary(fields []engine.Field) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, strings.ToLower(f.Label)+" "+f.Value)
	}
	return strings.Join(parts, " · ")
}

// connectors returns the branch connector for position i of n.
func connectors(i, n int) (connector, childPrefix string) {
	if i == n-1 {
		return "└─ ", "   "
	}
	return "├─ ", "│  "
}

// marker returns the two-character slot placed between the health glyph and the
// label. It is three-state: a dependency marker for something the node requires,
// a chevron for an expandable child, blank padding otherwise — so labels stay
// column-aligned whatever the mix.
func marker(n *engine.Node) string {
	switch {
	case n.Dependency:
		return "⇢ "
	case n.Expandable:
		return "▸ "
	default:
		return "  "
	}
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
