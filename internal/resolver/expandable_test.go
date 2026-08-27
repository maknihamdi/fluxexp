package resolver

import (
	"context"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// TestResolverExpandableDeclarations pins each resolver's declaration against
// the kinds it actually descends into, so a declaration cannot silently drift
// away from what Resolve returns.
func TestResolverExpandableDeclarations(t *testing.T) {
	tests := []struct {
		name string
		res  Resolver
		ref  engine.Ref
		want bool
	}{
		{"kustomization", KustomizationResolver{}, K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"), true},
		{"helmrelease", HelmReleaseResolver{}, K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "team", "web"), true},
		{"deployment", WorkloadResolver{}, K8sRef("apps/v1", "Deployment", "ns", "web"), true},
		{"statefulset", WorkloadResolver{}, K8sRef("apps/v1", "StatefulSet", "ns", "db"), true},
		{"daemonset", WorkloadResolver{}, K8sRef("apps/v1", "DaemonSet", "ns", "agent"), true},
		{"replicaset", WorkloadResolver{}, K8sRef("apps/v1", "ReplicaSet", "ns", "web-abc"), true},
		{"job", WorkloadResolver{}, K8sRef("batch/v1", "Job", "ns", "migrate"), true},
		{"pod is a leaf", WorkloadResolver{}, K8sRef("v1", "Pod", "ns", "web-abc-1"), false},
		{"workload resolver ignores foreign kinds", WorkloadResolver{}, K8sRef("v1", "ConfigMap", "ns", "cfg"), false},
		{"generic fallback", GenericK8sResolver{}, K8sRef("v1", "ConfigMap", "ns", "cfg"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.res.Expandable(tt.ref); got != tt.want {
				t.Errorf("Expandable = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRegistryExpandable_FollowsResolverSelection checks the registry answers
// through the same specific-before-fallback selection used for resolution.
func TestRegistryExpandable_FollowsResolverSelection(t *testing.T) {
	reg := NewDefaultRegistry()

	expandable := []engine.Ref{
		K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"),
		K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "team", "web"),
		K8sRef("apps/v1", "Deployment", "ns", "web"),
	}
	for _, ref := range expandable {
		if !reg.Expandable(ref) {
			t.Errorf("%s: want expandable", ref.Label())
		}
	}

	leaves := []engine.Ref{
		K8sRef("v1", "Pod", "ns", "web-1"),
		K8sRef("v1", "ConfigMap", "ns", "cfg"),
		K8sRef("v1", "Secret", "ns", "creds"),
	}
	for _, ref := range leaves {
		if reg.Expandable(ref) {
			t.Errorf("%s: want not expandable", ref.Label())
		}
	}
}

func TestRegistryExpandable_UnresolvableIsNotExpandable(t *testing.T) {
	reg := NewDefaultRegistry()
	// No resolver and no fallback registered for domain "gcp".
	ref := engine.Ref{Domain: "gcp", Type: "sqladmin/Instance", Coords: map[string]string{"name": "db"}}
	if reg.Expandable(ref) {
		t.Error("reference with no resolver and no fallback must not be expandable")
	}
}

func TestOrderChildren_ContainersFirstAndStable(t *testing.T) {
	reg := NewDefaultRegistry()

	cfg := K8sRef("v1", "ConfigMap", "ns", "values")
	sa := K8sRef("v1", "ServiceAccount", "ns", "infra")
	hr := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "ns", "traefik")
	ks := K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "ns", "child")
	pod := K8sRef("v1", "Pod", "ns", "web-1")

	// Interleaved input, mirroring a real inventory order.
	got := reg.OrderChildren([]engine.Ref{cfg, hr, sa, ks, pod})
	want := []engine.Ref{hr, ks, cfg, sa, pod}

	if len(got) != len(want) {
		t.Fatalf("length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Key() != want[i].Key() {
			t.Errorf("position %d = %s, want %s", i, got[i].Label(), want[i].Label())
		}
	}
}

func TestOrderChildren_UniformListsUnchanged(t *testing.T) {
	reg := NewDefaultRegistry()

	allLeaves := []engine.Ref{
		K8sRef("v1", "ConfigMap", "ns", "a"),
		K8sRef("v1", "Secret", "ns", "b"),
		K8sRef("v1", "Pod", "ns", "c"),
	}
	allContainers := []engine.Ref{
		K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "ns", "a"),
		K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "ns", "b"),
		K8sRef("apps/v1", "Deployment", "ns", "c"),
	}

	for name, in := range map[string][]engine.Ref{"leaves": allLeaves, "containers": allContainers} {
		t.Run(name, func(t *testing.T) {
			got := reg.OrderChildren(in)
			for i := range in {
				if got[i].Key() != in[i].Key() {
					t.Errorf("position %d = %s, want %s (order must be untouched)", i, got[i].Label(), in[i].Label())
				}
			}
		})
	}
}

func TestOrderChildren_EmptyAndSingle(t *testing.T) {
	reg := NewDefaultRegistry()
	if got := reg.OrderChildren(nil); len(got) != 0 {
		t.Errorf("nil input: got %d refs, want 0", len(got))
	}
	one := []engine.Ref{K8sRef("v1", "ConfigMap", "ns", "a")}
	if got := reg.OrderChildren(one); len(got) != 1 || got[0].Key() != one[0].Key() {
		t.Error("single-element input must be returned as-is")
	}
}

// childEmittingResolver is a stub that emits a fixed child list, so ordering can
// be exercised end-to-end through ResolveFunc.
type childEmittingResolver struct {
	children []engine.Ref
}

func (childEmittingResolver) Matches(ref engine.Ref) bool {
	_, kind, _, _, err := DecodeK8sRef(ref)
	return err == nil && kind == "Root"
}

func (c childEmittingResolver) Resolve(_ context.Context, _ *ResolveContext, _ engine.Ref) (engine.Result, error) {
	return engine.Result{Health: engine.Healthy, Children: c.children}, nil
}

func (childEmittingResolver) Expandable(engine.Ref) bool { return true }

// TestResolveFunc_OrdersChildren checks the rule reaches the engine (and so the
// CLI) without the engine knowing about it.
func TestResolveFunc_OrdersChildren(t *testing.T) {
	cfg := K8sRef("v1", "ConfigMap", "ns", "values")
	hr := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "ns", "web")

	reg := NewRegistry()
	reg.Register(childEmittingResolver{children: []engine.Ref{cfg, hr}})
	reg.Register(HelmReleaseResolver{})
	reg.RegisterFallback(DomainK8s, GenericK8sResolver{})

	res, err := reg.ResolveFunc(context.Background(), &ResolveContext{})(K8sRef("test/v1", "Root", "ns", "root"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(res.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(res.Children))
	}
	if res.Children[0].Key() != hr.Key() {
		t.Errorf("first child = %s, want the HelmRelease (containers first)", res.Children[0].Label())
	}
	if res.Children[1].Key() != cfg.Key() {
		t.Errorf("second child = %s, want the ConfigMap", res.Children[1].Label())
	}
}

func TestIsFluxRef(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		want       bool
	}{
		{"source group", "source.toolkit.fluxcd.io/v1", true},
		{"kustomize group", "kustomize.toolkit.fluxcd.io/v1", true},
		{"helm group", "helm.toolkit.fluxcd.io/v2", true},
		{"image group", "image.toolkit.fluxcd.io/v1beta2", true},
		{"notification group", "notification.toolkit.fluxcd.io/v1beta3", true},
		{"bare toolkit group", "toolkit.fluxcd.io/v1", true},
		{"core group", "v1", false},
		{"apps group", "apps/v1", false},
		{"lookalike suffix", "nottoolkit.fluxcd.io/v1", false},
		{"flux in the name only", "fluxcd.example.com/v1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFluxRef(K8sRef(tt.apiVersion, "Thing", "ns", "n")); got != tt.want {
				t.Errorf("IsFluxRef(%q) = %v, want %v", tt.apiVersion, got, tt.want)
			}
		})
	}
}

func TestIsFluxRef_NonKubernetesDomain(t *testing.T) {
	ref := engine.Ref{Domain: "gcp", Type: "sqladmin/Instance", Coords: map[string]string{"name": "db"}}
	if IsFluxRef(ref) {
		t.Error("a non-kubernetes reference cannot be a Flux object")
	}
}

func TestOrderChildren_FluxLeafOutranksNonFluxContainer(t *testing.T) {
	reg := NewDefaultRegistry()

	git := K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux") // Flux, leaf
	deploy := K8sRef("apps/v1", "Deployment", "ns", "web")                        // not Flux, container
	cfg := K8sRef("v1", "ConfigMap", "ns", "values")                              // leaf

	got := reg.OrderChildren([]engine.Ref{cfg, deploy, git})
	want := []engine.Ref{git, deploy, cfg}
	for i := range want {
		if got[i].Key() != want[i].Key() {
			t.Errorf("position %d = %s, want %s", i, got[i].Label(), want[i].Label())
		}
	}
}

func TestOrderChildren_ThreeTiersStable(t *testing.T) {
	reg := NewDefaultRegistry()

	git := K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux")
	hr := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "ns", "web")
	imgRepo := K8sRef("image.toolkit.fluxcd.io/v1beta2", "ImageRepository", "ns", "app")
	deploy := K8sRef("apps/v1", "Deployment", "ns", "api")
	sts := K8sRef("apps/v1", "StatefulSet", "ns", "db")
	cfg := K8sRef("v1", "ConfigMap", "ns", "a")
	sa := K8sRef("v1", "ServiceAccount", "ns", "b")

	// Deliberately interleaved so a tier-blind implementation cannot pass.
	in := []engine.Ref{cfg, deploy, git, sa, hr, sts, imgRepo}
	want := []engine.Ref{git, hr, imgRepo, deploy, sts, cfg, sa}

	got := reg.OrderChildren(in)
	if len(got) != len(want) {
		t.Fatalf("length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Key() != want[i].Key() {
			t.Errorf("position %d = %s, want %s", i, got[i].Label(), want[i].Label())
		}
	}
}

func TestResolveFunc_DependencyIsNotRepeatedAmongChildren(t *testing.T) {
	// A Flux Kustomization routinely applies the very GitRepository it
	// reconciles from, so the same reference arrives in both lists.
	git := K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux")
	cfg := K8sRef("v1", "ConfigMap", "flux", "values")
	hr := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "flux", "web")

	reg := NewRegistry()
	reg.Register(dependingResolver{deps: []engine.Ref{git}, children: []engine.Ref{cfg, git, hr}})
	reg.RegisterFallback(DomainK8s, GenericK8sResolver{})

	res, err := reg.ResolveFunc(context.Background(), &ResolveContext{})(K8sRef("test/v1", "Root", "flux", "root"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(res.Dependencies) != 1 || res.Dependencies[0].Key() != git.Key() {
		t.Fatalf("the dependency must be kept, got %+v", res.Dependencies)
	}
	for _, c := range res.Children {
		if c.Key() == git.Key() {
			t.Error("a declared dependency must not be repeated among the children")
		}
	}
	// The unrelated children survive, in tier order (HelmRelease is Flux).
	if len(res.Children) != 2 || res.Children[0].Key() != hr.Key() || res.Children[1].Key() != cfg.Key() {
		t.Errorf("unrelated children must be untouched and ordered, got %+v", res.Children)
	}
}

func TestResolveFunc_ChildrenUntouchedWithoutOverlap(t *testing.T) {
	git := K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux")
	cfg := K8sRef("v1", "ConfigMap", "flux", "values")
	sa := K8sRef("v1", "ServiceAccount", "flux", "infra")

	reg := NewRegistry()
	reg.Register(dependingResolver{deps: []engine.Ref{git}, children: []engine.Ref{cfg, sa}})
	reg.RegisterFallback(DomainK8s, GenericK8sResolver{})

	res, err := reg.ResolveFunc(context.Background(), &ResolveContext{})(K8sRef("test/v1", "Root", "flux", "root"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(res.Children) != 2 {
		t.Errorf("children with no overlap must all survive, got %+v", res.Children)
	}
}

// dependingResolver emits a fixed dependency and child list.
type dependingResolver struct {
	deps     []engine.Ref
	children []engine.Ref
}

func (dependingResolver) Matches(ref engine.Ref) bool {
	_, kind, _, _, err := DecodeK8sRef(ref)
	return err == nil && kind == "Root"
}

func (d dependingResolver) Resolve(_ context.Context, _ *ResolveContext, _ engine.Ref) (engine.Result, error) {
	return engine.Result{Health: engine.Healthy, Dependencies: d.deps, Children: d.children}, nil
}

func (dependingResolver) Expandable(engine.Ref) bool { return true }
