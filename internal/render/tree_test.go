package render

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

func k8sRef(typ, ns, name string) engine.Ref {
	display := typ + " " + name
	if ns != "" {
		display = typ + " " + ns + "/" + name
	}
	return engine.Ref{Domain: "kubernetes", Type: typ, Coords: map[string]string{"namespace": ns, "name": name}, Display: display}
}

func TestTree_ShowsNestedNodesWithHealth(t *testing.T) {
	root := &engine.Node{
		Ref:    k8sRef("Kustomization", "flux-system", "apps"),
		Health: engine.Healthy,
		Children: []*engine.Node{
			{Ref: k8sRef("ConfigMap", "team-a", "cfg"), Health: engine.Healthy},
			{Ref: k8sRef("Deployment", "team-a", "web"), Health: engine.Unhealthy, Detail: "0/1 ready"},
		},
	}
	out := Tree(root)

	for _, want := range []string{
		"Kustomization flux-system/apps",
		"[healthy]",
		"ConfigMap team-a/cfg",
		"Deployment team-a/web",
		"[unhealthy]",
		"0/1 ready",
		"(kubernetes)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	// Two children -> a branch and a last connector.
	if !strings.Contains(out, "├─") || !strings.Contains(out, "└─") {
		t.Fatalf("expected tree connectors:\n%s", out)
	}
}

func TestTree_ErrorNodeShowsReason(t *testing.T) {
	root := &engine.Node{
		Ref:    k8sRef("Kustomization", "flux-system", "apps"),
		Health: engine.Healthy,
		Children: []*engine.Node{
			{Ref: k8sRef("Secret", "team-a", "tok"), Health: engine.Error, Err: "not found"},
		},
	}
	out := Tree(root)
	if !strings.Contains(out, "[error]") || !strings.Contains(out, "not found") {
		t.Fatalf("error node must show reason:\n%s", out)
	}
}

// A pending node must not be mistaken for any of its siblings at a glance,
// which is the whole reason it is a status of its own rather than a detail.
func TestTree_PendingNodeHasItsOwnGlyph(t *testing.T) {
	seen := map[string]engine.Health{}
	for _, h := range []engine.Health{engine.Healthy, engine.Unhealthy, engine.Pending, engine.Unknown, engine.Error} {
		g := glyph(h)
		if other, clash := seen[g]; clash {
			t.Fatalf("health %q and %q share the glyph %q", h, other, g)
		}
		seen[g] = h
	}

	root := &engine.Node{
		Ref:    k8sRef("Kustomization", "flux-system", "apps"),
		Health: engine.Healthy,
		Children: []*engine.Node{
			{Ref: k8sRef("Deployment", "team-a", "web"), Health: engine.Pending, Detail: "generation 4, observed 3"},
		},
	}
	out := Tree(root)
	if !strings.Contains(out, "[pending]") || !strings.Contains(out, "generation 4, observed 3") {
		t.Fatalf("pending node must show its status and detail:\n%s", out)
	}
}

func TestTree_VisitedAnnotated(t *testing.T) {
	root := &engine.Node{
		Ref:    k8sRef("Node", "", "a"),
		Health: engine.Healthy,
		Children: []*engine.Node{
			{Ref: k8sRef("Node", "", "a"), Health: engine.Healthy, Visited: true},
		},
	}
	if out := Tree(root); !strings.Contains(out, "already visited") {
		t.Fatalf("visited leaf must be annotated:\n%s", out)
	}
}

func TestTree_NilRootIsEmpty(t *testing.T) {
	if Tree(nil) != "" {
		t.Fatal("nil root should render empty")
	}
}

func TestTree_ShowsFreshnessAndFields(t *testing.T) {
	root := &engine.Node{
		Ref:       k8sRef("Kustomization", "flux", "alloy"),
		Health:    engine.Healthy,
		Freshness: engine.UpToDate,
		Fields: []engine.Field{
			{Label: "Applied", Value: "alloy@f799f03"},
			{Label: "Synced", Value: "3m ago"},
		},
	}
	out := Tree(root)
	for _, want := range []string{"[healthy]", "[up-to-date]", "applied alloy@f799f03", "synced 3m ago"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTree_NonFreshnessNodeUnchanged(t *testing.T) {
	root := &engine.Node{Ref: k8sRef("Deployment", "ns", "web"), Health: engine.Healthy, Detail: "1/1 ready"}
	out := Tree(root)
	if strings.Contains(out, "[up-to-date]") || strings.Contains(out, "[behind]") {
		t.Fatalf("a non-freshness node must not show freshness:\n%s", out)
	}
	if !strings.Contains(out, "1/1 ready") {
		t.Fatalf("detail lost:\n%s", out)
	}
}

func TestTree_ExpandableNodeCarriesChevron(t *testing.T) {
	root := &engine.Node{
		Ref:        k8sRef("Kustomization", "flux", "apps"),
		Health:     engine.Healthy,
		Expandable: true,
		Children: []*engine.Node{
			{Ref: k8sRef("HelmRelease", "team-a", "web"), Health: engine.Healthy, Expandable: true},
			{Ref: k8sRef("ConfigMap", "team-a", "cfg"), Health: engine.Healthy},
		},
	}
	out := Tree(root)

	if !strings.Contains(out, "▸ Kustomization flux/apps") {
		t.Errorf("expandable root should carry the chevron:\n%s", out)
	}
	if !strings.Contains(out, "▸ HelmRelease team-a/web") {
		t.Errorf("expandable child should carry the chevron:\n%s", out)
	}
	if strings.Contains(out, "▸ ConfigMap") {
		t.Errorf("a leaf must not carry the chevron:\n%s", out)
	}
}

func TestTree_LeafPaddingKeepsLabelsAligned(t *testing.T) {
	root := &engine.Node{
		Ref:    k8sRef("Kustomization", "flux", "apps"),
		Health: engine.Healthy,
		Children: []*engine.Node{
			{Ref: k8sRef("HelmRelease", "ns", "web"), Health: engine.Healthy, Expandable: true},
			{Ref: k8sRef("ConfigMap", "ns", "cfg"), Health: engine.Healthy},
		},
	}
	lines := strings.Split(strings.TrimRight(Tree(root), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// Both children share the same connector, so their labels must start at the
	// same visual column whether or not they carry a chevron. The chevron is
	// multi-byte, so the column must be counted in runes, not bytes.
	col := func(line, label string) int {
		return utf8.RuneCountInString(line[:strings.Index(line, label)])
	}
	if got, want := col(lines[1], "HelmRelease"), col(lines[2], "ConfigMap"); got != want {
		t.Errorf("labels misaligned: chevron line starts at %d, padded line at %d\n%s", got, want, strings.Join(lines, "\n"))
	}
}

func TestTree_ChildrenRenderInResolverOrder(t *testing.T) {
	// The registry orders children containers-first before the engine builds the
	// tree; the renderer must not re-sort, only render what it is given.
	root := &engine.Node{
		Ref:    k8sRef("Kustomization", "flux", "apps"),
		Health: engine.Healthy,
		Children: []*engine.Node{
			{Ref: k8sRef("HelmRelease", "ns", "web"), Health: engine.Healthy, Expandable: true},
			{Ref: k8sRef("ConfigMap", "ns", "a"), Health: engine.Healthy},
			{Ref: k8sRef("ConfigMap", "ns", "b"), Health: engine.Healthy},
		},
	}
	out := Tree(root)
	hr, a, b := strings.Index(out, "HelmRelease"), strings.Index(out, "ConfigMap ns/a"), strings.Index(out, "ConfigMap ns/b")
	if !(hr < a && a < b) {
		t.Errorf("renderer must preserve the order it receives:\n%s", out)
	}
}

func TestTree_DependenciesRenderFirstWithTheirMarker(t *testing.T) {
	root := &engine.Node{
		Ref:        k8sRef("Kustomization", "flux", "traefik"),
		Health:     engine.Healthy,
		Expandable: true,
		Dependencies: []*engine.Node{{
			Ref:        k8sRef("GitRepository", "flux", "flux"),
			Health:     engine.Healthy,
			Dependency: true,
			Fields: []engine.Field{
				{Label: "Repo", Value: "gitlab/infra/fleet"},
				{Label: "Branch", Value: "main"},
				{Label: "Interval", Value: "1m"},
			},
		}},
		Children: []*engine.Node{
			{Ref: k8sRef("HelmRelease", "traefik", "traefik"), Health: engine.Healthy, Expandable: true},
			{Ref: k8sRef("ConfigMap", "traefik", "values"), Health: engine.Unknown},
		},
	}
	lines := strings.Split(strings.TrimRight(Tree(root), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}

	// The dependency comes before any child.
	if !strings.Contains(lines[1], "GitRepository flux/flux") {
		t.Errorf("the dependency must be printed first:\n%s", strings.Join(lines, "\n"))
	}
	// It carries the dependency marker, not the expandable chevron.
	if !strings.Contains(lines[1], "⇢ GitRepository") {
		t.Errorf("dependency must carry the ⇢ marker:\n%s", lines[1])
	}
	if strings.Contains(lines[1], "▸ GitRepository") {
		t.Errorf("dependency must not carry the expandable chevron:\n%s", lines[1])
	}
	// Its fields are inline, no interaction needed.
	for _, want := range []string{"repo gitlab/infra/fleet", "branch main", "interval 1m"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("dependency line missing %q:\n%s", want, lines[1])
		}
	}
	// The last child still closes the branch.
	if !strings.HasPrefix(lines[3], "└─ ") {
		t.Errorf("last child should close the branch, got %q", lines[3])
	}
}

func TestTree_NodeWithoutDependenciesUnchanged(t *testing.T) {
	root := &engine.Node{
		Ref:      k8sRef("Kustomization", "flux", "apps"),
		Health:   engine.Healthy,
		Children: []*engine.Node{{Ref: k8sRef("ConfigMap", "ns", "cfg"), Health: engine.Healthy}},
	}
	out := Tree(root)
	if strings.Contains(out, "⇢") {
		t.Errorf("no dependency marker should appear:\n%s", out)
	}
	if !strings.HasPrefix(strings.Split(out, "\n")[1], "└─ ") {
		t.Errorf("single child should close the branch:\n%s", out)
	}
}

func TestTree_ChildDependenciesNestUnderTheChild(t *testing.T) {
	// The engine resolves dependencies for every node, not just the root, so a
	// child Kustomization must render its own dependency block indented under it.
	root := &engine.Node{
		Ref: k8sRef("Kustomization", "flux", "flux"), Health: engine.Healthy, Expandable: true,
		Dependencies: []*engine.Node{
			{Ref: k8sRef("GitRepository", "flux", "flux"), Health: engine.Healthy, Dependency: true},
		},
		Children: []*engine.Node{{
			Ref: k8sRef("Kustomization", "flux", "alloy"), Health: engine.Unhealthy, Expandable: true,
			Dependencies: []*engine.Node{
				{Ref: k8sRef("GitRepository", "flux", "flux"), Health: engine.Healthy, Dependency: true},
				{Ref: k8sRef("Kustomization", "flux", "vault-operator"), Health: engine.Healthy, Dependency: true},
			},
			Children: []*engine.Node{{Ref: k8sRef("HelmRelease", "grafana", "grafana"), Health: engine.Unhealthy}},
		}},
	}
	lines := strings.Split(strings.TrimRight(Tree(root), "\n"), "\n")

	// Find the child Kustomization, then check the two lines after it are its
	// dependencies, indented deeper than it.
	var idx = -1
	for i, l := range lines {
		if strings.Contains(l, "Kustomization flux/alloy") {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("child Kustomization not rendered:\n%s", strings.Join(lines, "\n"))
	}
	for _, want := range []string{"GitRepository flux/flux", "Kustomization flux/vault-operator"} {
		found := false
		for _, l := range lines[idx+1 : idx+3] {
			if strings.Contains(l, want) && strings.Contains(l, "⇢") {
				found = true
			}
		}
		if !found {
			t.Errorf("child's dependency %q must render right under it:\n%s", want, strings.Join(lines, "\n"))
		}
	}
	// Nested under the child: deeper indentation than the child's own line.
	depth := func(s string) int { return len(s) - len(strings.TrimLeft(s, "│ ")) }
	if depth(lines[idx+1]) <= depth(lines[idx]) {
		t.Errorf("the child's dependency must be indented under it:\n%s", strings.Join(lines[idx:idx+2], "\n"))
	}
}
