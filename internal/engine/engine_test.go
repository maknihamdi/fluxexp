package engine

import (
	"errors"
	"testing"
)

// ref is a small helper to build a kubernetes-domain-ish reference for tests.
func ref(typ, name string) Ref {
	return Ref{Domain: "kubernetes", Type: typ, Coords: map[string]string{"name": name}}
}

// graphResolver builds a ResolveFunc from a static adjacency map plus a set of
// refs whose resolution should fail, recording how many times each ref is asked.
func graphResolver(children map[string][]Ref, fail map[string]bool, calls map[string]int) ResolveFunc {
	return func(r Ref) (Result, error) {
		calls[r.Key()]++
		if fail[r.Key()] {
			return Result{}, errors.New("boom")
		}
		return Result{Health: Healthy, Children: children[r.Key()]}, nil
	}
}

func findChild(n *Node, name string) *Node {
	for _, c := range n.Children {
		if c.Ref.Coords["name"] == name {
			return c
		}
	}
	return nil
}

func TestTraverse_BuildsTree(t *testing.T) {
	root := ref("Root", "r")
	c1, c2 := ref("Child", "a"), ref("Child", "b")
	calls := map[string]int{}
	resolve := graphResolver(map[string][]Ref{root.Key(): {c1, c2}}, nil, calls)

	got := Traverse(root, resolve)
	if len(got.Children) != 2 {
		t.Fatalf("want 2 children, got %d", len(got.Children))
	}
	if got.Health != Healthy {
		t.Fatalf("root health = %q", got.Health)
	}
}

func TestTraverse_MixesDomains(t *testing.T) {
	root := ref("Kustomization", "apps")
	gcp := Ref{Domain: "gcp", Type: "sql.Instance", Coords: map[string]string{"name": "db"}}
	calls := map[string]int{}
	resolve := graphResolver(map[string][]Ref{root.Key(): {gcp}}, nil, calls)

	got := Traverse(root, resolve)
	if len(got.Children) != 1 || got.Children[0].Ref.Domain != "gcp" {
		t.Fatalf("expected a single gcp child, got %+v", got.Children)
	}
}

func TestTraverse_ChainsSameType(t *testing.T) {
	k1 := ref("Kustomization", "root")
	k2 := ref("Kustomization", "child")
	leaf := ref("ConfigMap", "cm")
	calls := map[string]int{}
	resolve := graphResolver(map[string][]Ref{
		k1.Key(): {k2},
		k2.Key(): {leaf},
	}, nil, calls)

	got := Traverse(k1, resolve)
	k2Node := findChild(got, "child")
	if k2Node == nil || findChild(k2Node, "cm") == nil {
		t.Fatalf("expected root->child->cm chain, got %+v", got)
	}
}

func TestTraverse_CycleTerminatesAndResolvesOnce(t *testing.T) {
	a := ref("Node", "a")
	b := ref("Node", "b")
	calls := map[string]int{}
	resolve := graphResolver(map[string][]Ref{
		a.Key(): {b},
		b.Key(): {a},
	}, nil, calls)

	got := Traverse(a, resolve)
	if calls[a.Key()] != 1 || calls[b.Key()] != 1 {
		t.Fatalf("each node must resolve once: a=%d b=%d", calls[a.Key()], calls[b.Key()])
	}
	// b's child points back to a, which must be an already-visited leaf.
	bNode := findChild(got, "b")
	back := findChild(bNode, "a")
	if back == nil || !back.Visited {
		t.Fatalf("expected already-visited leaf back to a, got %+v", back)
	}
}

func TestTraverse_PartialFailureContinues(t *testing.T) {
	root := ref("Root", "r")
	c1, c2, c3 := ref("Child", "a"), ref("Child", "b"), ref("Child", "c")
	calls := map[string]int{}
	resolve := graphResolver(
		map[string][]Ref{root.Key(): {c1, c2, c3}},
		map[string]bool{c2.Key(): true},
		calls,
	)

	got := Traverse(root, resolve)
	if len(got.Children) != 3 {
		t.Fatalf("want all 3 children present, got %d", len(got.Children))
	}
	bad := findChild(got, "b")
	if bad.Health != Error || bad.Err == "" {
		t.Fatalf("second child must be an error node, got %+v", bad)
	}
	if findChild(got, "a").Health != Healthy || findChild(got, "c").Health != Healthy {
		t.Fatalf("siblings of the failed child must still resolve")
	}
}

func TestTraverse_CarriesFreshnessAndFields(t *testing.T) {
	root := ref("Kustomization", "apps")
	resolve := func(r Ref) (Result, error) {
		return Result{
			Health:    Healthy,
			Freshness: Behind,
			Fields:    []Field{{Label: "Applied", Value: "main@abc"}, {Label: "Source", Value: "main@def"}},
		}, nil
	}
	got := Traverse(root, resolve)
	if got.Freshness != Behind {
		t.Fatalf("freshness = %q, want behind", got.Freshness)
	}
	if len(got.Fields) != 2 || got.Fields[0].Label != "Applied" || got.Fields[1].Label != "Source" {
		t.Fatalf("fields not carried in order: %+v", got.Fields)
	}
}

func TestTraverse_EveryNodeHasHealth(t *testing.T) {
	root := ref("Root", "r")
	c1 := ref("Child", "a")
	calls := map[string]int{}
	resolve := graphResolver(map[string][]Ref{root.Key(): {c1}}, map[string]bool{c1.Key(): true}, calls)

	got := Traverse(root, resolve)
	valid := map[Health]bool{Healthy: true, Unhealthy: true, Unknown: true, Error: true}
	var check func(*Node)
	check = func(n *Node) {
		if !valid[n.Health] {
			t.Fatalf("node %s has invalid health %q", n.Ref.Label(), n.Health)
		}
		for _, c := range n.Children {
			check(c)
		}
	}
	check(got)
}

func TestTraverse_DependencyIsResolvedButNotDescended(t *testing.T) {
	// dep would return a child of its own; the engine must not follow it.
	resolve := func(ref Ref) (Result, error) {
		switch ref.Type {
		case "root":
			return Result{Health: Healthy, Dependencies: []Ref{{Domain: "k", Type: "dep"}}}, nil
		case "dep":
			return Result{Health: Healthy, Children: []Ref{{Domain: "k", Type: "deep"}}}, nil
		default:
			t.Errorf("engine descended into a dependency: resolved %q", ref.Type)
			return Result{Health: Healthy}, nil
		}
	}
	root := Traverse(Ref{Domain: "k", Type: "root"}, resolve)

	if len(root.Dependencies) != 1 {
		t.Fatalf("got %d dependencies, want 1", len(root.Dependencies))
	}
	if len(root.Dependencies[0].Children) != 0 {
		t.Errorf("a dependency must carry no children, got %d", len(root.Dependencies[0].Children))
	}
	if root.Dependencies[0].Health != Healthy {
		t.Errorf("dependency health = %q, want it resolved", root.Dependencies[0].Health)
	}
}

func TestTraverse_DependencyChainDoesNotFollow(t *testing.T) {
	resolved := map[string]int{}
	resolve := func(ref Ref) (Result, error) {
		resolved[ref.Type]++
		switch ref.Type {
		case "a":
			return Result{Health: Healthy, Dependencies: []Ref{{Domain: "k", Type: "b"}}}, nil
		case "b":
			return Result{Health: Healthy, Dependencies: []Ref{{Domain: "k", Type: "c"}}}, nil
		default:
			return Result{Health: Healthy}, nil
		}
	}
	Traverse(Ref{Domain: "k", Type: "a"}, resolve)

	if resolved["c"] != 0 {
		t.Errorf("the chain must stop at the first level; %q was resolved %d time(s)", "c", resolved["c"])
	}
}

func TestTraverse_UnresolvableDependencyIsErrorNode(t *testing.T) {
	resolve := func(ref Ref) (Result, error) {
		if ref.Type == "missing" {
			return Result{}, errors.New("not found")
		}
		return Result{Health: Healthy, Dependencies: []Ref{{Domain: "k", Type: "missing"}}}, nil
	}
	root := Traverse(Ref{Domain: "k", Type: "root"}, resolve)

	if root.Health != Healthy {
		t.Errorf("parent health = %q, want it untouched by a failing dependency", root.Health)
	}
	if len(root.Dependencies) != 1 || root.Dependencies[0].Health != Error {
		t.Fatalf("want one error dependency, got %+v", root.Dependencies)
	}
	if root.Dependencies[0].Err == "" {
		t.Error("the error dependency must carry its reason")
	}
}

func TestTraverse_DependencyDoesNotSuppressLaterChildExpansion(t *testing.T) {
	// "shared" is first seen as a dependency, then as a real child. The child
	// occurrence must still expand.
	resolve := func(ref Ref) (Result, error) {
		switch ref.Type {
		case "root":
			return Result{Health: Healthy,
				Dependencies: []Ref{{Domain: "k", Type: "shared"}},
				Children:     []Ref{{Domain: "k", Type: "branch"}}}, nil
		case "branch":
			return Result{Health: Healthy, Children: []Ref{{Domain: "k", Type: "shared"}}}, nil
		case "shared":
			return Result{Health: Healthy, Children: []Ref{{Domain: "k", Type: "leaf"}}}, nil
		default:
			return Result{Health: Healthy}, nil
		}
	}
	root := Traverse(Ref{Domain: "k", Type: "root"}, resolve)

	shared := root.Children[0].Children[0]
	if shared.Visited {
		t.Error("the child occurrence must not be marked visited by the earlier dependency")
	}
	if len(shared.Children) != 1 {
		t.Errorf("the child occurrence must expand, got %d children", len(shared.Children))
	}
}
