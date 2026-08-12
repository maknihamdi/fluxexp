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
