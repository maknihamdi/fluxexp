package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"
	"github.com/maknihamdi/fluxexp/internal/k8s"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeCluster implements the ui.cluster interface for tests. It records the keys
// asked for, so a test can pin what a layer does *not* retrieve.
type fakeCluster struct {
	objs          map[string]*unstructured.Unstructured  // "kind|ns|name"
	lists         map[string][]unstructured.Unstructured // "kind|ns"
	clusterScoped map[string]bool
	listErrs      map[string]error // "kind|ns"

	mu     sync.Mutex // a layer's entries are fetched concurrently
	gets   []string
	listed []string
}

func (f *fakeCluster) Get(_ context.Context, _ /*apiVersion*/, kind, ns, name string) (*unstructured.Unstructured, error) {
	key := kind + "|" + ns + "|" + name
	f.mu.Lock()
	f.gets = append(f.gets, key)
	f.mu.Unlock()
	if o, ok := f.objs[key]; ok {
		return o, nil
	}
	return nil, fmt.Errorf("not found: %s/%s/%s", kind, ns, name)
}

// fetched reports whether the cluster was asked for a "kind|ns|name" key.
func (f *fakeCluster) fetched(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, g := range f.gets {
		if g == key {
			return true
		}
	}
	return false
}

func (f *fakeCluster) Namespaced(_ /*apiVersion*/, kind string) (bool, error) {
	return !f.clusterScoped[kind], nil
}

func (f *fakeCluster) List(_ context.Context, _ /*apiVersion*/, kind, ns string) ([]unstructured.Unstructured, error) {
	key := kind + "|" + ns
	f.mu.Lock()
	f.listed = append(f.listed, key)
	f.mu.Unlock()
	if err := f.listErrs[key]; err != nil {
		return nil, fmt.Errorf("listing %s: %w", kind, err) // as k8s.Client.List wraps it
	}
	return f.lists[key], nil
}

func forbidden(resource, ns string) error {
	return apierrors.NewForbidden(schema.GroupResource{Resource: resource}, "", errors.New("no rights in "+ns))
}

func noKind(kind string) error {
	return &meta.NoKindMatchError{GroupKind: schema.GroupKind{Kind: kind}}
}

func helmRelease(ns, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata":   map[string]interface{}{"namespace": ns, "name": name},
		"spec":       map[string]interface{}{"interval": "5m"},
		"status": map[string]interface{}{"conditions": []interface{}{
			map[string]interface{}{
				"type": "Ready", "status": "True", "message": "Helm install succeeded",
				"lastTransitionTime": "2026-08-12T10:05:20Z",
			},
		}},
	}}
}

func namespaceObj(name string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]interface{}{"name": name},
	}}
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

func gitRepo(ns, name, revision string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"namespace": ns, "name": name},
		"spec": map[string]interface{}{
			"url":      "https://gitlab.example.com/infra/fleet.git",
			"ref":      map[string]interface{}{"branch": "main"},
			"interval": "1m0s",
		},
		"status": map[string]interface{}{"artifact": map[string]interface{}{
			"revision": revision, "lastUpdateTime": "2026-08-13T09:00:00Z",
		}},
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
			// Source matches the applied revision -> up-to-date.
			"GitRepository|flux|flux": gitRepo("flux", "flux", "main@sha1:abc"),
		},
		lists: map[string][]unstructured.Unstructured{
			"Kustomization|flux": {*kustomization("flux", "apps")},
		},
	}
	s := newService(func(string) (cluster, error) { return fc, nil })
	return s, fc
}

// The in-cluster sentinel names a mode, not a kubeconfig context. It has to
// reach the client factory as the empty context — that is what clientcmd
// resolves in-cluster — because the sentinel itself would fail context
// validation and never reach the fallback.
func TestClientFor_InClusterSentinelBecomesEmptyContext(t *testing.T) {
	var asked []string
	s := newService(func(name string) (cluster, error) {
		asked = append(asked, name)
		return &fakeCluster{}, nil
	})

	if _, err := s.clientFor(k8s.InClusterContext); err != nil {
		t.Fatalf("clientFor: %v", err)
	}
	if len(asked) != 1 || asked[0] != "" {
		t.Fatalf("factory called with %q, want the empty context", asked)
	}

	// Cached under the name the caller used, so a second call does not rebuild.
	if _, err := s.clientFor(k8s.InClusterContext); err != nil {
		t.Fatalf("clientFor (cached): %v", err)
	}
	if len(asked) != 1 {
		t.Fatalf("factory called %d times, want 1", len(asked))
	}
}

func TestClientFor_NamedContextIsPassedThrough(t *testing.T) {
	var asked []string
	s := newService(func(name string) (cluster, error) {
		asked = append(asked, name)
		return &fakeCluster{}, nil
	})

	if _, err := s.clientFor("prod"); err != nil {
		t.Fatalf("clientFor: %v", err)
	}
	if len(asked) != 1 || asked[0] != "prod" {
		t.Fatalf("factory called with %q, want \"prod\"", asked)
	}
}

func TestRoots_ExtractsSummary(t *testing.T) {
	s, _ := newFakeService()
	roots, unreadable, err := s.Roots("dev", "flux")
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || unreadable != nil {
		t.Fatalf("want 1 root and nothing unreadable, got %d, %v", len(roots), unreadable)
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
	if r.Freshness != "up-to-date" {
		t.Fatalf("freshness = %q, want up-to-date", r.Freshness)
	}
	if len(r.Fields) == 0 {
		t.Fatalf("expected freshness fields on the root summary")
	}
}

func TestRoots_ByKind(t *testing.T) {
	refused := forbidden("x", "team-a")
	cases := []struct {
		name           string
		lists          map[string][]unstructured.Unstructured
		errs           map[string]error
		want           []string // kinds, in order
		wantUnreadable []string
		wantErr        bool
	}{
		{
			name:  "HelmRelease-only namespace",
			lists: map[string][]unstructured.Unstructured{"HelmRelease|team-a": {*helmRelease("team-a", "web")}},
			want:  []string{"HelmRelease"},
		},
		{
			name: "Kustomizations come first",
			lists: map[string][]unstructured.Unstructured{
				"HelmRelease|team-a":   {*helmRelease("team-a", "web")},
				"Kustomization|team-a": {*kustomization("team-a", "apps")},
			},
			want: []string{"Kustomization", "HelmRelease"},
		},
		{
			name:  "no HelmRelease kind on the cluster",
			lists: map[string][]unstructured.Unstructured{"Kustomization|team-a": {*kustomization("team-a", "apps")}},
			errs:  map[string]error{"HelmRelease|team-a": noKind("HelmRelease")},
			want:  []string{"Kustomization"},
		},
		{
			name:           "one kind refused",
			lists:          map[string][]unstructured.Unstructured{"HelmRelease|team-a": {*helmRelease("team-a", "web")}},
			errs:           map[string]error{"Kustomization|team-a": refused},
			want:           []string{"HelmRelease"},
			wantUnreadable: []string{"Kustomization"},
		},
		{
			name:    "every kind refused",
			errs:    map[string]error{"Kustomization|team-a": refused, "HelmRelease|team-a": refused},
			wantErr: true,
		},
		{
			name:    "refused, the other kind being absent",
			errs:    map[string]error{"Kustomization|team-a": refused, "HelmRelease|team-a": noKind("HelmRelease")},
			wantErr: true,
		},
		{
			name:    "any other failure",
			errs:    map[string]error{"HelmRelease|team-a": errors.New("boom")},
			wantErr: true,
		},
		{
			name: "no Flux kind at all reads as empty",
			errs: map[string]error{"Kustomization|team-a": noKind("Kustomization"), "HelmRelease|team-a": noKind("HelmRelease")},
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeCluster{lists: tc.lists, listErrs: tc.errs}
			s := newService(func(string) (cluster, error) { return fc, nil })
			roots, unreadable, err := s.Roots("dev", "team-a")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			kinds := []string{}
			for _, r := range roots {
				kinds = append(kinds, r.Ref.Type[strings.LastIndex(r.Ref.Type, "|")+1:])
			}
			if !reflect.DeepEqual(kinds, tc.want) {
				t.Fatalf("kinds = %v, want %v", kinds, tc.want)
			}
			if !reflect.DeepEqual(unreadable, tc.wantUnreadable) {
				t.Fatalf("unreadable = %v, want %v", unreadable, tc.wantUnreadable)
			}
		})
	}
}

// A HelmRelease row comes from the listed object alone: no freshness is claimed,
// and resolving it would gunzip a storage Secret.
func TestRoots_HelmReleaseSummary(t *testing.T) {
	fc := &fakeCluster{lists: map[string][]unstructured.Unstructured{
		"HelmRelease|team-a": {*helmRelease("team-a", "web")},
	}}
	s := newService(func(string) (cluster, error) { return fc, nil })
	roots, _, err := s.Roots("dev", "team-a")
	if err != nil {
		t.Fatal(err)
	}
	r := roots[0]
	if r.Health != string(engine.Healthy) || r.Interval != "5m" || r.Message == "" {
		t.Fatalf("summary wrong: %+v", r)
	}
	if r.Freshness != "" || len(r.Fields) != 0 {
		t.Fatalf("no freshness may be claimed for a HelmRelease: %+v", r)
	}
	if len(fc.gets) != 0 {
		t.Fatalf("a listed HelmRelease must cost no fetch, got %v", fc.gets)
	}
}

func TestNamespaces(t *testing.T) {
	t.Run("listed sorted, with exactly one call", func(t *testing.T) {
		fc := &fakeCluster{lists: map[string][]unstructured.Unstructured{
			"Namespace|": {namespaceObj("team-b"), namespaceObj("flux"), namespaceObj("team-a")},
		}}
		s := newService(func(string) (cluster, error) { return fc, nil })
		got, err := s.Namespaces("dev")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"flux", "team-a", "team-b"}) {
			t.Fatalf("namespaces = %v", got)
		}
		if !reflect.DeepEqual(fc.listed, []string{"Namespace|"}) {
			t.Fatalf("calls = %v, want the namespace list alone", fc.listed)
		}
	})
	t.Run("refused means no suggestions", func(t *testing.T) {
		fc := &fakeCluster{listErrs: map[string]error{"Namespace|": forbidden("namespaces", "")}}
		s := newService(func(string) (cluster, error) { return fc, nil })
		got, err := s.Namespaces("dev")
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %v, %v; want an empty list and no error", got, err)
		}
	})
	t.Run("another failure is an error", func(t *testing.T) {
		fc := &fakeCluster{listErrs: map[string]error{"Namespace|": errors.New("boom")}}
		s := newService(func(string) (cluster, error) { return fc, nil })
		if _, err := s.Namespaces("dev"); err == nil {
			t.Fatal("want an error")
		}
	})
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
	if node.Freshness != "up-to-date" || len(node.Fields) == 0 {
		t.Fatalf("expand should carry freshness + fields, got freshness=%q fields=%d", node.Freshness, len(node.Fields))
	}
	// The source is a dependency, not a child; children are the inventory only.
	if len(node.Dependencies) != 1 || node.Dependencies[0].Ref.Name != "flux" {
		t.Fatalf("want the GitRepository as the single dependency, got %+v", node.Dependencies)
	}
	if len(node.Children) != 1 {
		t.Fatalf("want 1 child (the inventory entry), got %d", len(node.Children))
	}
	if node.Children[0].Ref.Name != "web" || node.Children[0].Context != "dev" {
		t.Fatalf("inventory child wrong: %+v", node.Children[0])
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
	// The source still resolves as a dependency; only the inventory child fails.
	if len(node.Children) != 1 || node.Children[0].Health != string(engine.Error) {
		t.Fatalf("expected one error child, got %+v", node.Children)
	}
	if len(node.Dependencies) != 1 || node.Dependencies[0].Health == string(engine.Error) {
		t.Fatalf("the source dependency should still resolve, got %+v", node.Dependencies)
	}
}

func TestHandlers_RootsAndExpand(t *testing.T) {
	s, fc := newFakeService()
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

	// /api/namespaces answers a list, never null, even when refused.
	fc.listErrs = map[string]error{"Namespace|": forbidden("namespaces", "")}
	nsResp, err := http.Get(srv.URL + "/api/namespaces?context=dev")
	if err != nil {
		t.Fatal(err)
	}
	var nsBody map[string]json.RawMessage
	json.NewDecoder(nsResp.Body).Decode(&nsBody)
	nsResp.Body.Close()
	if string(nsBody["namespaces"]) != "[]" {
		t.Fatalf("namespaces endpoint: want [], got %s", nsBody["namespaces"])
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
		t.Fatalf("expand endpoint: want 1 child (inventory), got %d", len(node.Children))
	}
	if len(node.Dependencies) != 1 {
		t.Fatalf("expand endpoint: want the source as a dependency, got %d", len(node.Dependencies))
	}
}
