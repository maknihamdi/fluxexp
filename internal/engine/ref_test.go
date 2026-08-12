package engine

import "testing"

func TestRefKey_StableAcrossCoordOrdering(t *testing.T) {
	a := Ref{Domain: "kubernetes", Type: "v1|ConfigMap", Coords: map[string]string{"namespace": "ns", "name": "cm"}}
	b := Ref{Domain: "kubernetes", Type: "v1|ConfigMap", Coords: map[string]string{"name": "cm", "namespace": "ns"}}
	if a.Key() != b.Key() {
		t.Fatalf("equal refs produced different keys:\n a=%q\n b=%q", a.Key(), b.Key())
	}
}

func TestRefKey_DistinguishesDomain(t *testing.T) {
	k8s := Ref{Domain: "kubernetes", Type: "SQLInstance", Coords: map[string]string{"name": "db"}}
	gcp := Ref{Domain: "gcp", Type: "SQLInstance", Coords: map[string]string{"name": "db"}}
	if k8s.Key() == gcp.Key() {
		t.Fatalf("refs in different domains share a key: %q", k8s.Key())
	}
}

func TestRefKey_DistinguishesCoords(t *testing.T) {
	a := Ref{Domain: "kubernetes", Type: "v1|ConfigMap", Coords: map[string]string{"name": "a"}}
	b := Ref{Domain: "kubernetes", Type: "v1|ConfigMap", Coords: map[string]string{"name": "b"}}
	if a.Key() == b.Key() {
		t.Fatalf("distinct refs share a key: %q", a.Key())
	}
}

func TestRefLabel_FallsBackToTypeAndCoords(t *testing.T) {
	r := Ref{Domain: "gcp", Type: "sql.Instance", Coords: map[string]string{"name": "db"}}
	if got := r.Label(); got != "sql.Instance db" {
		t.Fatalf("Label() = %q", got)
	}
	r.Display = "Cloud SQL db"
	if got := r.Label(); got != "Cloud SQL db" {
		t.Fatalf("Label() with Display = %q", got)
	}
}
