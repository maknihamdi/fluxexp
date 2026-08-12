package resolver

import (
	"context"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestKustomization_MatchesOnlyKustomizations(t *testing.T) {
	r := KustomizationResolver{}
	kust := K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux-system", "apps")
	deploy := K8sRef("apps/v1", "Deployment", "ns", "web")
	if !r.Matches(kust) {
		t.Fatal("must claim Kustomization ref")
	}
	if r.Matches(deploy) {
		t.Fatal("must not claim Deployment ref")
	}
	// A same-kind object in a different group must not match.
	other := K8sRef("example.com/v1", "Kustomization", "ns", "x")
	if r.Matches(other) {
		t.Fatal("must not claim Kustomization from a foreign group")
	}
}

func TestKustomization_InventoryBecomesChildren(t *testing.T) {
	obj := kustomizationObj("True", "", "v1",
		"team-a_web_apps_Deployment",
		"team-a_web-cfg__ConfigMap", // core group is empty
		"_cluster-role_rbac.authorization.k8s.io_ClusterRole",
	)
	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{
		"Kustomization|flux-system|apps": obj,
	}}
	rc := &ResolveContext{K8s: getter}
	ref := K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux-system", "apps")

	res, err := KustomizationResolver{}.Resolve(context.Background(), rc, ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Children) != 3 {
		t.Fatalf("want 3 children, got %d", len(res.Children))
	}

	// Deployment (apps/v1)
	assertChild(t, res.Children[0], DomainK8s, "apps/v1|Deployment", "team-a", "web")
	// ConfigMap (core -> version only)
	assertChild(t, res.Children[1], DomainK8s, "v1|ConfigMap", "team-a", "web-cfg")
	// ClusterRole (cluster-scoped -> empty namespace)
	assertChild(t, res.Children[2], DomainK8s, "rbac.authorization.k8s.io/v1|ClusterRole", "", "cluster-role")
}

func TestKustomization_EmptyInventoryNoChildren(t *testing.T) {
	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{
		"Kustomization|flux-system|apps": kustomizationObj("True", "", "v1"), // no ids
	}}
	rc := &ResolveContext{K8s: getter}
	ref := K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux-system", "apps")

	res, err := KustomizationResolver{}.Resolve(context.Background(), rc, ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Children) != 0 {
		t.Fatalf("want no children, got %d", len(res.Children))
	}
}

func TestKustomization_Health(t *testing.T) {
	cases := []struct {
		status string
		want   engine.Health
	}{
		{"True", engine.Healthy},
		{"False", engine.Unhealthy},
	}
	for _, tc := range cases {
		getter := fakeGetter{objs: map[string]*unstructured.Unstructured{
			"Kustomization|flux-system|apps": kustomizationObj(tc.status, "boom", "v1"),
		}}
		rc := &ResolveContext{K8s: getter}
		ref := K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux-system", "apps")
		res, err := KustomizationResolver{}.Resolve(context.Background(), rc, ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Health != tc.want {
			t.Fatalf("status %q -> health %q, want %q", tc.status, res.Health, tc.want)
		}
	}
}

func assertChild(t *testing.T, ref engine.Ref, domain, typ, ns, name string) {
	t.Helper()
	if ref.Domain != domain || ref.Type != typ {
		t.Fatalf("child domain/type = %q/%q, want %q/%q", ref.Domain, ref.Type, domain, typ)
	}
	if ref.Coords["namespace"] != ns || ref.Coords["name"] != name {
		t.Fatalf("child coords = %q/%q, want %q/%q", ref.Coords["namespace"], ref.Coords["name"], ns, name)
	}
}
