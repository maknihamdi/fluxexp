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

// ksWithSource builds a Kustomization with a spec.sourceRef, optional dependsOn
// entries, and one inventory entry, so dependencies and children are separable.
func ksWithSource(srcKind, srcName, srcNS string, dependsOn ...map[string]interface{}) *unstructured.Unstructured {
	ks := kustomizationObj("True", "ok", "v1", "ns_web_apps_Deployment")
	spec := map[string]interface{}{}
	if srcName != "" {
		src := map[string]interface{}{"kind": srcKind, "name": srcName}
		if srcNS != "" {
			src["namespace"] = srcNS
		}
		spec["sourceRef"] = src
	}
	if len(dependsOn) > 0 {
		entries := make([]interface{}, 0, len(dependsOn))
		for _, d := range dependsOn {
			entries = append(entries, d)
		}
		spec["dependsOn"] = entries
	}
	ks.Object["spec"] = spec
	return ks
}

func resolveKs(t *testing.T, objs map[string]*unstructured.Unstructured, ns, name string) engine.Result {
	t.Helper()
	rc := &ResolveContext{K8s: fakeGetter{objs: objs}}
	res, err := KustomizationResolver{}.Resolve(context.Background(), rc,
		K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", ns, name))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return res
}

func TestKustomization_SourceLeadsDependencies(t *testing.T) {
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": ksWithSource("GitRepository", "flux", "",
			map[string]interface{}{"name": "cert-manager"},
			map[string]interface{}{"name": "alloy"}),
		"GitRepository|flux|flux": objWithReady("True", "stored artifact"),
	}, "flux", "apps")

	if len(res.Dependencies) != 3 {
		t.Fatalf("got %d dependencies, want 3 (source + 2 dependsOn)", len(res.Dependencies))
	}
	_, kind, ns, name, _ := DecodeK8sRef(res.Dependencies[0])
	if kind != "GitRepository" || name != "flux" || ns != "flux" {
		t.Errorf("first dependency = %s %s/%s, want GitRepository flux/flux", kind, ns, name)
	}
	// dependsOn entries keep declaration order.
	for i, want := range []string{"cert-manager", "alloy"} {
		_, kind, _, name, _ := DecodeK8sRef(res.Dependencies[i+1])
		if kind != "Kustomization" || name != want {
			t.Errorf("dependency %d = %s %s, want Kustomization %s", i+1, kind, name, want)
		}
	}
}

func TestKustomization_InventoryStaysInChildren(t *testing.T) {
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": ksWithSource("GitRepository", "flux", ""),
		"GitRepository|flux|flux": objWithReady("True", ""),
	}, "flux", "apps")

	if len(res.Children) != 1 {
		t.Fatalf("got %d children, want only the inventory entry", len(res.Children))
	}
	for _, c := range res.Children {
		if _, kind, _, _, _ := DecodeK8sRef(c); kind == "GitRepository" {
			t.Error("the source must not appear among the children")
		}
	}
}

func TestKustomization_DependencyNamespacesDefault(t *testing.T) {
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|team|apps": ksWithSource("GitRepository", "repo", "",
			map[string]interface{}{"name": "base"}),
		"GitRepository|team|repo": objWithReady("True", ""),
	}, "team", "apps")

	for i, dep := range res.Dependencies {
		if _, _, ns, _, _ := DecodeK8sRef(dep); ns != "team" {
			t.Errorf("dependency %d namespace = %q, want the Kustomization's own", i, ns)
		}
	}
}

func TestKustomization_ExplicitDependencyNamespaces(t *testing.T) {
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|team|apps": ksWithSource("GitRepository", "shared", "flux-system",
			map[string]interface{}{"name": "base", "namespace": "platform"}),
		"GitRepository|flux-system|shared": objWithReady("True", ""),
	}, "team", "apps")

	if _, _, ns, _, _ := DecodeK8sRef(res.Dependencies[0]); ns != "flux-system" {
		t.Errorf("source namespace = %q, want the explicit sourceRef namespace", ns)
	}
	if _, _, ns, _, _ := DecodeK8sRef(res.Dependencies[1]); ns != "platform" {
		t.Errorf("dependsOn namespace = %q, want the explicit entry namespace", ns)
	}
}

func TestKustomization_UnfetchableSourceIsStillEmitted(t *testing.T) {
	// The Kustomization exists; its source does not. The reference is declared in
	// the manifest, so it is emitted and becomes an error node downstream —
	// pointing at a missing source is exactly what the reader needs to see.
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": ksWithSource("GitRepository", "missing", ""),
	}, "flux", "apps")

	if len(res.Dependencies) != 1 {
		t.Fatalf("the declared source must be emitted, got %+v", res.Dependencies)
	}
	if _, _, _, name, _ := DecodeK8sRef(res.Dependencies[0]); name != "missing" {
		t.Errorf("dependency = %q, want the declared source", name)
	}
	if res.Health != engine.Healthy {
		t.Errorf("health = %q, want the Kustomization's own health untouched", res.Health)
	}
}

// TestDependencyRefsForFetched_AgreesWithResolver pins the single-source rule:
// what a listed row derives from the fetched object must equal what the resolver
// returns, or the two surfaces would show different dependencies.
func TestDependencyRefsForFetched_AgreesWithResolver(t *testing.T) {
	ks := ksWithSource("GitRepository", "flux", "",
		map[string]interface{}{"name": "cert-manager"},
		map[string]interface{}{"name": "alloy", "namespace": "other"})
	ref := K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps")

	derived := DependencyRefsForFetched(ref, ks)
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": ks,
		"GitRepository|flux|flux": objWithReady("True", ""),
	}, "flux", "apps")

	if len(derived) != len(res.Dependencies) {
		t.Fatalf("derived %d refs, resolver returned %d", len(derived), len(res.Dependencies))
	}
	for i := range derived {
		if derived[i].Key() != res.Dependencies[i].Key() {
			t.Errorf("position %d: derived %s, resolver %s", i, derived[i].Label(), res.Dependencies[i].Label())
		}
	}
}

// TestDependencyRefsForFetched_KindsWithoutDeclarations covers the kinds the
// dispatch does not know: they yield nothing even when their manifest happens to
// carry a sourceRef-shaped field. (Kustomizations and HelmReleases, which do
// declare dependencies, are covered by their own tests.)
func TestDependencyRefsForFetched_KindsWithoutDeclarations(t *testing.T) {
	plain := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{"sourceRef": map[string]interface{}{"kind": "GitRepository", "name": "x"}},
	}}
	for _, ref := range []engine.Ref{
		K8sRef("v1", "ConfigMap", "ns", "cfg"),
		K8sRef("v1", "Pod", "ns", "p"),
		K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "ns", "flux"),
	} {
		if got := DependencyRefsForFetched(ref, plain); len(got) != 0 {
			t.Errorf("%s must yield no dependency refs, got %v", ref.Label(), got)
		}
	}
	for _, ref := range []engine.Ref{
		K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "ns", "k"),
		K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "ns", "web"),
	} {
		if got := DependencyRefsForFetched(ref, nil); len(got) != 0 {
			t.Errorf("a nil object must yield nothing for %s, got %v", ref.Label(), got)
		}
	}
}

func TestKustomization_MissingDependsOnIsStillEmitted(t *testing.T) {
	// dependsOn is emitted from the spec without pre-fetching, so a missing
	// target surfaces later as an error node rather than vanishing.
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": ksWithSource("", "", "", map[string]interface{}{"name": "ghost"}),
	}, "flux", "apps")

	if len(res.Dependencies) != 1 {
		t.Fatalf("got %d dependencies, want the dependsOn entry emitted", len(res.Dependencies))
	}
	if _, _, _, name, _ := DecodeK8sRef(res.Dependencies[0]); name != "ghost" {
		t.Errorf("dependency = %q, want %q", name, "ghost")
	}
}

func TestKustomization_NoSourceNoDependsOn(t *testing.T) {
	res := resolveKs(t, map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": kustomizationObj("True", "ok", "v1", "ns_web_apps_Deployment"),
	}, "flux", "apps")

	if len(res.Dependencies) != 0 {
		t.Errorf("want no dependencies, got %d", len(res.Dependencies))
	}
	if len(res.Children) != 1 {
		t.Errorf("want the inventory child, got %d", len(res.Children))
	}
}
