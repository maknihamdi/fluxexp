package render

import (
	"strings"
	"testing"

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
