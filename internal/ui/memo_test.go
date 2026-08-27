package ui

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// countingGetter records how many times each object was actually fetched.
type countingGetter struct {
	*fakeCluster
	calls map[string]int
}

func (c *countingGetter) Get(ctx context.Context, apiVersion, kind, ns, name string) (*unstructured.Unstructured, error) {
	c.calls[kind+"|"+ns+"|"+name]++
	return c.fakeCluster.Get(ctx, apiVersion, kind, ns, name)
}

func TestMemoGetter_FetchesOncePerObject(t *testing.T) {
	inner := &countingGetter{
		fakeCluster: &fakeCluster{objs: map[string]*unstructured.Unstructured{
			"GitRepository|flux|flux": gitRepo("flux", "flux", "main@sha1:abc"),
		}},
		calls: map[string]int{},
	}
	m := newMemoGetter(inner)

	for i := 0; i < 5; i++ {
		obj, err := m.Get(context.Background(), "source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux")
		if err != nil || obj == nil {
			t.Fatalf("get %d: %v", i, err)
		}
	}
	if got := inner.calls["GitRepository|flux|flux"]; got != 1 {
		t.Errorf("fetched %d times, want 1", got)
	}
}

func TestMemoGetter_CachesFailures(t *testing.T) {
	inner := &countingGetter{
		fakeCluster: &fakeCluster{objs: map[string]*unstructured.Unstructured{}},
		calls:       map[string]int{},
	}
	m := newMemoGetter(inner)

	for i := 0; i < 3; i++ {
		if _, err := m.Get(context.Background(), "kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "ghost"); err == nil {
			t.Fatal("expected the miss to surface as an error")
		}
	}
	if got := inner.calls["Kustomization|flux|ghost"]; got != 1 {
		t.Errorf("a missing object was looked up %d times, want 1", got)
	}
}

func TestMemoGetter_DistinguishesObjects(t *testing.T) {
	inner := &countingGetter{
		fakeCluster: &fakeCluster{objs: map[string]*unstructured.Unstructured{
			"GitRepository|flux|a": gitRepo("flux", "a", "r"),
			"GitRepository|flux|b": gitRepo("flux", "b", "r"),
		}},
		calls: map[string]int{},
	}
	m := newMemoGetter(inner)

	for _, name := range []string{"a", "b", "a", "b"} {
		if _, err := m.Get(context.Background(), "source.toolkit.fluxcd.io/v1", "GitRepository", "flux", name); err != nil {
			t.Fatalf("get %s: %v", name, err)
		}
	}
	for _, name := range []string{"a", "b"} {
		if got := inner.calls["GitRepository|flux|"+name]; got != 1 {
			t.Errorf("%s fetched %d times, want 1", name, got)
		}
	}
}

// TestExpand_DoesNotCacheAcrossRequests pins that the memo lives for one request:
// a second Expand must see the cluster's current state, not the first call's.
func TestExpand_DoesNotCacheAcrossRequests(t *testing.T) {
	fc := nestedService()
	s := newService(func(string) (cluster, error) { return fc, nil })
	ref := k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "parent")

	if _, err := s.Expand("ctx", ref); err != nil {
		t.Fatalf("first expand: %v", err)
	}
	// The source disappears between the two requests.
	delete(fc.objs, "GitRepository|flux|flux")

	node, err := s.Expand("ctx", ref)
	if err != nil {
		t.Fatalf("second expand: %v", err)
	}
	// The reference is still declared, so it is still listed — but the second
	// request must have gone back to the cluster and found it gone.
	var found bool
	for _, dep := range node.Dependencies {
		if dep.Ref.Name != "flux" {
			continue
		}
		found = true
		if dep.Health != "error" {
			t.Errorf("the second request served a stale source (health %q); the memo must not outlive one request", dep.Health)
		}
	}
	if !found {
		t.Fatal("the declared source should still be listed")
	}
}
