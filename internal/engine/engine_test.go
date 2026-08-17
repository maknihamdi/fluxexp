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
