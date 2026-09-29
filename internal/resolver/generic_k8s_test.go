package resolver

import (
	"context"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGenericK8s_Matches(t *testing.T) {
	r := GenericK8sResolver{}
	if !r.Matches(engine.Ref{Domain: DomainK8s, Type: "v1|ConfigMap"}) {
		t.Fatal("should match any kubernetes ref")
	}
	if r.Matches(engine.Ref{Domain: "gcp", Type: "sql.Instance"}) {
		t.Fatal("should not match non-kubernetes ref")
	}
}

func TestGenericK8s_ReadyStates(t *testing.T) {
	cases := []struct {
		name string
		obj  string // key in fake
		want engine.Health
	}{
		{"ready true -> healthy", "true", engine.Healthy},
		{"ready false -> unhealthy", "false", engine.Unhealthy},
		// An object with nothing to report is applied and present, which is the
		// whole of what can be known about it. It used to read unknown, which
		// said "look at me" about 40% of a Kustomization's inventory.
		{"no condition -> healthy", "none", engine.Healthy},
	}

	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{
		"ConfigMap|ns|true":  objWithReady("True", ""),
		"ConfigMap|ns|false": objWithReady("False", "degraded"),
		"ConfigMap|ns|none":  objNoConditions(),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc := &ResolveContext{K8s: getter}
			ref := K8sRef("v1", "ConfigMap", "ns", tc.obj)
			res, err := GenericK8sResolver{}.Resolve(context.Background(), rc, ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Health != tc.want {
				t.Fatalf("health = %q, want %q", res.Health, tc.want)
			}
			if len(res.Children) != 0 {
				t.Fatalf("generic fallback must return no children, got %d", len(res.Children))
			}
		})
	}
}

func TestGenericK8s_FetchErrorPropagates(t *testing.T) {
	rc := &ResolveContext{K8s: fakeGetter{objs: map[string]*unstructured.Unstructured{}}}
	ref := K8sRef("v1", "ConfigMap", "ns", "missing")
	_, err := (GenericK8sResolver{}).Resolve(context.Background(), rc, ref)
	if err == nil {
		t.Fatal("expected fetch error to propagate")
	}
}
