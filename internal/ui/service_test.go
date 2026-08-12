package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fakeCluster implements the ui.cluster interface for tests.
type fakeCluster struct {
	objs          map[string]*unstructured.Unstructured  // "kind|ns|name"
	lists         map[string][]unstructured.Unstructured // "kind|ns"
	clusterScoped map[string]bool
}

func (f *fakeCluster) Get(_ context.Context, _ /*apiVersion*/, kind, ns, name string) (*unstructured.Unstructured, error) {
	if o, ok := f.objs[kind+"|"+ns+"|"+name]; ok {
		return o, nil
	}
	return nil, fmt.Errorf("not found: %s/%s/%s", kind, ns, name)
}

func (f *fakeCluster) Namespaced(_ /*apiVersion*/, kind string) (bool, error) {
	return !f.clusterScoped[kind], nil
}

func (f *fakeCluster) List(_ context.Context, _ /*apiVersion*/, kind, ns string) ([]unstructured.Unstructured, error) {
	return f.lists[kind+"|"+ns], nil
}

func kustomization(ns, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"namespace": ns, "name": name},
		"spec": map[string]interface{}{
			"path":      "./infra/" + name,
			"interval":  "10m",
			"sourceRef": map[string]interface{}{"kind": "GitRepository", "name": "flux"},
		},
		"status": map[string]interface{}{
			"lastAppliedRevision": "main@sha1:abc",
			"conditions": []interface{}{
				map[string]interface{}{
					"type": "Ready", "status": "True", "message": "Applied revision: main@sha1:abc",
					"lastTransitionTime": "2026-08-12T10:05:20Z",
				},
			},
			"inventory": map[string]interface{}{
				"entries": []interface{}{
					map[string]interface{}{"id": ns + "_web_apps_Deployment", "v": "v1"},
				},
			},
		},
	}}
}

func newFakeService() (*Service, *fakeCluster) {
	deploy := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"namespace": "flux", "name": "web"},
	}}
	fc := &fakeCluster{
		objs: map[string]*unstructured.Unstructured{
			"Kustomization|flux|apps": kustomization("flux", "apps"),
			"Deployment|flux|web":     deploy,
		},
		lists: map[string][]unstructured.Unstructured{
			"Kustomization|flux": {*kustomization("flux", "apps")},
		},
	}
	s := newService(func(string) (cluster, error) { return fc, nil })
	return s, fc
}

func TestRoots_ExtractsSummary(t *testing.T) {
	s, _ := newFakeService()
	roots, err := s.Roots("dev", "flux")
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Fatalf("want 1 root, got %d", len(roots))
	}
	r := roots[0]
	if r.Health != string(engine.Healthy) || r.SourceKind != "GitRepository" || r.SourceName != "flux" {
		t.Fatalf("summary wrong: %+v", r)
	}
	if r.Path != "./infra/apps" || r.Interval != "10m" || r.Revision != "main@sha1:abc" {
		t.Fatalf("summary fields wrong: %+v", r)
	}
	if r.LastTransition == "" || r.Message == "" {
		t.Fatalf("expected transition time and message: %+v", r)
	}
}

func TestExpand_ReturnsChildrenWithHealth(t *testing.T) {
	s, _ := newFakeService()
	ref := engine.Ref{
		Domain: "kubernetes",
		Type:   "kustomize.toolkit.fluxcd.io/v1|Kustomization",
		Coords: map[string]string{"namespace": "flux", "name": "apps"},
	}
	node, err := s.Expand("dev", ref)
	if err != nil {
		t.Fatal(err)
	}
	if node.Health != string(engine.Healthy) {
		t.Fatalf("node health = %q", node.Health)
	}
	if len(node.Children) != 1 {
		t.Fatalf("want 1 child, got %d", len(node.Children))
	}
	if node.Children[0].Ref.Name != "web" || node.Children[0].Context != "dev" {
		t.Fatalf("child wrong: %+v", node.Children[0])
	}
}

func TestExpand_UnresolvableChildIsError(t *testing.T) {
	s, fc := newFakeService()
	delete(fc.objs, "Deployment|flux|web") // child fetch will fail
	ref := engine.Ref{
		Domain: "kubernetes",
		Type:   "kustomize.toolkit.fluxcd.io/v1|Kustomization",
		Coords: map[string]string{"namespace": "flux", "name": "apps"},
	}
	node, err := s.Expand("dev", ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Children) != 1 || node.Children[0].Health != string(engine.Error) {
		t.Fatalf("expected one error child, got %+v", node.Children)
	}
}

func TestHandlers_RootsAndExpand(t *testing.T) {
	s, _ := newFakeService()
	srv := httptest.NewServer(Handler(s))
	defer srv.Close()

	// /api/roots
	resp, err := http.Get(srv.URL + "/api/roots?context=dev&namespace=flux")
	if err != nil {
		t.Fatal(err)
	}
	var rootsBody struct{ Roots []RootDTO }
	json.NewDecoder(resp.Body).Decode(&rootsBody)
	resp.Body.Close()
	if len(rootsBody.Roots) != 1 {
		t.Fatalf("roots endpoint: want 1, got %d", len(rootsBody.Roots))
	}

	// /api/expand missing params -> 400
	bad, _ := http.Get(srv.URL + "/api/expand?context=dev")
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("expand without params: want 400, got %d", bad.StatusCode)
	}
	bad.Body.Close()

	// /api/expand valid
	ok, err := http.Get(srv.URL + "/api/expand?context=dev&type=" +
		"kustomize.toolkit.fluxcd.io%2Fv1%7CKustomization&ns=flux&name=apps")
	if err != nil {
		t.Fatal(err)
	}
	var node NodeDTO
	json.NewDecoder(ok.Body).Decode(&node)
	ok.Body.Close()
	if len(node.Children) != 1 {
		t.Fatalf("expand endpoint: want 1 child, got %d", len(node.Children))
	}
}
