package resolver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// encodeRelease reproduces the Helm Secrets-driver encoding of a release JSON:
// gzip, then Helm base64, then the Kubernetes base64 that the API layer adds.
func encodeRelease(t *testing.T, releaseJSON string) string {
	t.Helper()
	var gzBuf bytes.Buffer
	w := gzip.NewWriter(&gzBuf)
	if _, err := w.Write([]byte(releaseJSON)); err != nil {
		t.Fatal(err)
	}
	w.Close()
	helmB64 := base64.StdEncoding.EncodeToString(gzBuf.Bytes())
	return base64.StdEncoding.EncodeToString([]byte(helmB64))
}

func TestDecodeHelmRelease_RoundTrip(t *testing.T) {
	encoded := encodeRelease(t, `{"name":"web","namespace":"apps","version":7,"manifest":"x","info":{"status":"deployed"}}`)
	rel, err := decodeHelmRelease(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rel.Name != "web" || rel.Namespace != "apps" || rel.Version != 7 || rel.Info.Status != "deployed" {
		t.Fatalf("unexpected release: %+v", rel)
	}
}

func TestDecodeHelmRelease_Corrupt(t *testing.T) {
	if _, err := decodeHelmRelease("!!!not base64!!!"); err == nil {
		t.Fatal("expected error on corrupt payload")
	}
}

// allNamespaced treats every kind as namespaced.
func allNamespaced(_ /*apiVersion*/, _ /*kind*/ string) (bool, error) { return true, nil }

// scopeBy reports cluster scope for the kinds in the set (namespaced otherwise).
func scopeBy(clusterScoped map[string]bool) func(string, string) (bool, error) {
	return func(_ /*apiVersion*/, kind string) (bool, error) {
		return !clusterScoped[kind], nil
	}
}

func TestManifestChildren(t *testing.T) {
	manifest := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: apps
---
apiVersion: v1
kind: Service
metadata:
  name: web
---
# an empty doc below
---
`
	refs, err := manifestChildren(manifest, "apps", allNamespaced)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("want 2 children, got %d: %+v", len(refs), refs)
	}
	// Deployment keeps its explicit namespace.
	if refs[0].Type != "apps/v1|Deployment" || refs[0].Coords["namespace"] != "apps" || refs[0].Coords["name"] != "web" {
		t.Fatalf("deployment ref wrong: %+v", refs[0])
	}
	// Service has no namespace in the manifest -> inherits the release namespace.
	if refs[1].Type != "v1|Service" || refs[1].Coords["namespace"] != "apps" {
		t.Fatalf("service should inherit release ns: %+v", refs[1])
	}
}

func TestManifestChildren_ClusterScopedStaysNamespaceless(t *testing.T) {
	manifest := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: viewer
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
`
	refs, err := manifestChildren(manifest, "apps", scopeBy(map[string]bool{"ClusterRole": true}))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("want 2 children, got %d", len(refs))
	}
	// Cluster-scoped ClusterRole must NOT inherit the release namespace.
	if refs[0].Coords["namespace"] != "" {
		t.Fatalf("ClusterRole should stay namespace-less, got %q", refs[0].Coords["namespace"])
	}
	// Namespaced ConfigMap without a namespace inherits the release namespace.
	if refs[1].Coords["namespace"] != "apps" {
		t.Fatalf("ConfigMap should inherit release ns, got %q", refs[1].Coords["namespace"])
	}
}

func TestHelmReleaseResolver_Matches(t *testing.T) {
	r := HelmReleaseResolver{}
	hr := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "web")
	dep := K8sRef("apps/v1", "Deployment", "apps", "web")
	if !r.Matches(hr) {
		t.Fatal("must claim HelmRelease")
	}
	if r.Matches(dep) {
		t.Fatal("must not claim Deployment")
	}
}

// helmReleaseObj builds a HelmRelease with a history entry and a Ready condition.
func helmReleaseObj(name, ns string, version int64, ready string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": ready, "message": "ok"},
			},
			"history": []interface{}{
				map[string]interface{}{"name": name, "namespace": ns, "version": version},
			},
		},
	}}
}

func TestHelmReleaseResolver_ResolveExpandsManifest(t *testing.T) {
	manifest := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: apps\n---\napiVersion: v1\nkind: Service\nmetadata:\n  name: web\n"
	releaseJSON := `{"name":"web","namespace":"apps","version":7,"manifest":` + jsonString(manifest) + `,"info":{"status":"deployed"}}`
	secret := &unstructured.Unstructured{Object: map[string]interface{}{
		"data": map[string]interface{}{"release": encodeRelease(t, releaseJSON)},
	}}

	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{
		"HelmRelease|apps|web":                  helmReleaseObj("web", "apps", 7, "True"),
		"Secret|apps|sh.helm.release.v1.web.v7": secret,
	}}
	rc := &ResolveContext{K8s: getter}
	ref := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "web")

	res, err := HelmReleaseResolver{}.Resolve(context.Background(), rc, ref)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Health != engine.Healthy {
		t.Fatalf("health = %q", res.Health)
	}
	if len(res.Children) != 2 {
		t.Fatalf("want 2 children, got %d", len(res.Children))
	}
	if res.Detail != "release deployed" {
		t.Fatalf("detail = %q", res.Detail)
	}
}

func TestHelmReleaseResolver_NoHistoryNoChildren(t *testing.T) {
	hr := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "Unknown"},
			},
		},
	}}
	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{"HelmRelease|apps|web": hr}}
	rc := &ResolveContext{K8s: getter}
	ref := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "web")

	res, err := HelmReleaseResolver{}.Resolve(context.Background(), rc, ref)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(res.Children) != 0 {
		t.Fatalf("want no children, got %d", len(res.Children))
	}
}

// jsonString quotes a string as a JSON literal (for embedding the manifest).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// hrWithSpec builds a HelmRelease carrying only a spec: the dependency
// derivation reads the manifest a caller already holds and never looks at
// status, let alone at the Helm storage Secret.
func hrWithSpec(spec map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
}

// chartSource builds the classic spec.chart.spec.sourceRef form; an empty ns
// leaves the namespace undeclared.
func chartSource(kind, name, ns string) map[string]interface{} {
	src := map[string]interface{}{"kind": kind, "name": name}
	if ns != "" {
		src["namespace"] = ns
	}
	return map[string]interface{}{
		"chart": map[string]interface{}{
			"spec": map[string]interface{}{"chart": "cert-manager", "sourceRef": src},
		},
	}
}

func TestHelmReleaseDependencies(t *testing.T) {
	hrRef := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "web")

	cases := []struct {
		name string
		spec map[string]interface{}
		want []engine.Ref
	}{
		{
			name: "chart repository is grouped with the release",
			spec: chartSource("HelmRepository", "jetstack", ""),
			want: []engine.Ref{K8sRef(sourceAPIVersion, "HelmRepository", "apps", "jetstack")},
		},
		{
			name: "dependsOn entries follow the source, in declaration order",
			spec: withDependsOn(chartSource("HelmRepository", "jetstack", ""),
				map[string]interface{}{"name": "cert-manager"},
				map[string]interface{}{"name": "alloy", "namespace": "monitoring"}),
			want: []engine.Ref{
				K8sRef(sourceAPIVersion, "HelmRepository", "apps", "jetstack"),
				K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "cert-manager"),
				K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "monitoring", "alloy"),
			},
		},
		{
			name: "chartRef takes precedence over the chart's sourceRef",
			spec: merge(chartSource("HelmRepository", "jetstack", ""), map[string]interface{}{
				"chartRef": map[string]interface{}{"kind": "OCIRepository", "name": "podinfo"},
			}),
			want: []engine.Ref{K8sRef(sourceAPIVersion, "OCIRepository", "apps", "podinfo")},
		},
		{
			name: "an explicit source namespace is honoured",
			spec: chartSource("HelmRepository", "jetstack", "flux"),
			want: []engine.Ref{K8sRef(sourceAPIVersion, "HelmRepository", "flux", "jetstack")},
		},
		{
			name: "dependsOn alone, without a source",
			spec: withDependsOn(map[string]interface{}{}, map[string]interface{}{"name": "cert-manager"}),
			want: []engine.Ref{K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "cert-manager")},
		},
		{
			name: "a release declaring nothing has no dependencies",
			spec: map[string]interface{}{"interval": "10m"},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DependencyRefsForFetched(hrRef, hrWithSpec(tc.spec))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d refs %v, want %d", len(got), labels(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i].Key() != tc.want[i].Key() {
					t.Errorf("position %d: got %s, want %s", i, got[i].Label(), tc.want[i].Label())
				}
			}
		})
	}
}

// withDependsOn adds dependsOn entries to a spec.
func withDependsOn(spec map[string]interface{}, entries ...map[string]interface{}) map[string]interface{} {
	list := make([]interface{}, 0, len(entries))
	for _, e := range entries {
		list = append(list, e)
	}
	spec["dependsOn"] = list
	return spec
}

// merge copies extra keys into a spec.
func merge(spec, extra map[string]interface{}) map[string]interface{} {
	for k, v := range extra {
		spec[k] = v
	}
	return spec
}

func labels(refs []engine.Ref) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Label())
	}
	return out
}

// TestHelmRelease_MissingSourceIsStillEmitted pins that no declared reference is
// pre-fetched to decide whether to report it: a release pointing at a source
// that does not exist must surface it as an error node downstream, not hide it.
func TestHelmRelease_MissingSourceIsStillEmitted(t *testing.T) {
	hr := hrWithSpec(chartSource("HelmRepository", "gone", ""))
	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{"HelmRelease|apps|web": hr}}
	rc := &ResolveContext{K8s: getter}
	ref := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "web")

	res, err := HelmReleaseResolver{}.Resolve(context.Background(), rc, ref)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(res.Dependencies) != 1 || res.Dependencies[0].Coords["name"] != "gone" {
		t.Fatalf("want the missing HelmRepository still reported, got %v", labels(res.Dependencies))
	}
}

// TestHelmReleaseDependencyRefs_AgreeWithResolver pins the single-source rule for
// the HelmRelease, as its Kustomization counterpart does: what a listed row
// derives from the fetched object must equal what the resolver returns.
func TestHelmReleaseDependencyRefs_AgreeWithResolver(t *testing.T) {
	hr := hrWithSpec(withDependsOn(chartSource("HelmRepository", "jetstack", ""),
		map[string]interface{}{"name": "cert-manager"}))
	ref := K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "apps", "web")

	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{"HelmRelease|apps|web": hr}}
	res, err := HelmReleaseResolver{}.Resolve(context.Background(), &ResolveContext{K8s: getter}, ref)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	derived := DependencyRefsForFetched(ref, hr)
	if len(derived) != len(res.Dependencies) {
		t.Fatalf("derived %d refs, resolver returned %d", len(derived), len(res.Dependencies))
	}
	for i := range derived {
		if derived[i].Key() != res.Dependencies[i].Key() {
			t.Errorf("position %d: derived %s, resolver %s", i, derived[i].Label(), res.Dependencies[i].Label())
		}
	}
}
