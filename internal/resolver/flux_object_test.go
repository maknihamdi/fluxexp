package resolver

import (
	"context"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fluxObj builds a Flux object with the given spec and status maps.
func fluxObj(spec, status map[string]interface{}) *unstructured.Unstructured {
	o := map[string]interface{}{}
	if spec != nil {
		o["spec"] = spec
	}
	if status != nil {
		o["status"] = status
	}
	return &unstructured.Unstructured{Object: o}
}

func readyStatus(msg string) map[string]interface{} {
	return map[string]interface{}{"conditions": []interface{}{
		map[string]interface{}{"type": "Ready", "status": "True", "message": msg},
	}}
}

// fieldMap flattens fields to label -> value for assertions.
func fieldMap(fields []engine.Field) map[string]string {
	m := map[string]string{}
	for _, f := range fields {
		m[f.Label] = f.Value
	}
	return m
}

func TestFluxObjectResolver_Matches(t *testing.T) {
	r := FluxObjectResolver{}
	claimed := []engine.Ref{
		K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux"),
		K8sRef("source.toolkit.fluxcd.io/v1", "HelmRepository", "ns", "charts"),
		K8sRef("source.toolkit.fluxcd.io/v1", "Bucket", "ns", "b"),
		K8sRef("image.toolkit.fluxcd.io/v1beta2", "ImageRepository", "ns", "app"),
		K8sRef("image.toolkit.fluxcd.io/v1beta2", "ImagePolicy", "ns", "app"),
	}
	for _, ref := range claimed {
		if !r.Matches(ref) {
			t.Errorf("%s should be claimed", ref.Label())
		}
		if r.Expandable(ref) {
			t.Errorf("%s is a leaf and must not be expandable", ref.Label())
		}
	}

	rejected := []engine.Ref{
		K8sRef("v1", "ConfigMap", "ns", "cfg"),
		K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"),
		K8sRef("helm.toolkit.fluxcd.io/v2", "HelmRelease", "ns", "web"),
	}
	for _, ref := range rejected {
		if r.Matches(ref) {
			t.Errorf("%s must not be claimed (it has its own resolver, or is not Flux)", ref.Label())
		}
	}
}

func TestFluxObject_GitRepositoryFields(t *testing.T) {
	obj := fluxObj(map[string]interface{}{
		"url":      "https://gitlab.example.com/infra/fleet.git",
		"ref":      map[string]interface{}{"branch": "main"},
		"interval": "1m0s",
	}, readyStatus("stored artifact"))

	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{"GitRepository|flux|flux": obj}}
	res, err := FluxObjectResolver{}.Resolve(context.Background(), &ResolveContext{K8s: getter},
		K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Health != engine.Healthy {
		t.Errorf("health = %q, want healthy (still from the Ready condition)", res.Health)
	}

	got := fieldMap(res.Fields)
	want := map[string]string{
		"Repo":     "gitlab.example.com/infra/fleet",
		"Branch":   "main",
		"Interval": "1m",
	}
	for label, value := range want {
		if got[label] != value {
			t.Errorf("field %q = %q, want %q", label, got[label], value)
		}
	}
	// The full URL must survive for the tooltip.
	for _, f := range res.Fields {
		if f.Label == "Repo" && f.Full != "https://gitlab.example.com/infra/fleet.git" {
			t.Errorf("Repo.Full = %q, want the complete URL", f.Full)
		}
	}
}

func TestFluxObject_TrackedRefVariants(t *testing.T) {
	tests := []struct {
		name      string
		ref       map[string]interface{}
		wantLabel string
		wantValue string
	}{
		{"branch", map[string]interface{}{"branch": "main"}, "Branch", "main"},
		{"tag", map[string]interface{}{"tag": "v1.2.3"}, "Tag", "v1.2.3"},
		{"semver", map[string]interface{}{"semver": ">=1.0.0"}, "Semver", ">=1.0.0"},
		{"commit", map[string]interface{}{"commit": "abc1234"}, "Commit", "abc1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := fluxObj(map[string]interface{}{"url": "https://h/o/r.git", "ref": tt.ref}, nil)
			got := fieldMap(fluxObjectFields(obj, "GitRepository"))
			if got[tt.wantLabel] != tt.wantValue {
				t.Errorf("field %q = %q, want %q", tt.wantLabel, got[tt.wantLabel], tt.wantValue)
			}
		})
	}
}

func TestFluxObject_BucketAndHelmRepositoryFields(t *testing.T) {
	bucket := fluxObj(map[string]interface{}{
		"bucketName": "artifacts", "endpoint": "storage.googleapis.com", "interval": "5m0s",
	}, nil)
	got := fieldMap(fluxObjectFields(bucket, "Bucket"))
	if got["Bucket"] != "artifacts" || got["Endpoint"] != "storage.googleapis.com" || got["Interval"] != "5m" {
		t.Errorf("bucket fields = %v", got)
	}

	helm := fluxObj(map[string]interface{}{
		"url": "https://charts.jetstack.io", "type": "default", "interval": "1h",
	}, nil)
	got = fieldMap(fluxObjectFields(helm, "HelmRepository"))
	if got["Repo"] != "charts.jetstack.io" || got["Type"] != "default" || got["Interval"] != "1h" {
		t.Errorf("helmrepository fields = %v", got)
	}
}

func TestFluxObject_ImageRepositoryAndPolicyFields(t *testing.T) {
	repo := fluxObj(map[string]interface{}{
		"image": "europe-docker.pkg.dev/proj/repo/app", "interval": "10m",
	}, map[string]interface{}{
		"lastScanResult": map[string]interface{}{"scanTime": "2026-08-26T09:00:00Z", "tagCount": int64(12)},
	})
	got := fieldMap(fluxObjectFields(repo, "ImageRepository"))
	if got["Image"] != "europe-docker.pkg.dev/proj/repo/app" {
		t.Errorf("Image = %q", got["Image"])
	}
	if got["Interval"] != "10m" {
		t.Errorf("Interval = %q", got["Interval"])
	}
	if got["Last scan"] == "" {
		t.Error("a scan time in the status must produce a Last scan field")
	}

	policy := fluxObj(map[string]interface{}{
		"policy": map[string]interface{}{"semver": map[string]interface{}{"range": ">=1.0.0"}},
	}, map[string]interface{}{
		"latestRef": map[string]interface{}{"tag": "1.4.2"},
	})
	got = fieldMap(fluxObjectFields(policy, "ImagePolicy"))
	if got["Policy"] != "semver >=1.0.0" {
		t.Errorf("Policy = %q, want %q", got["Policy"], "semver >=1.0.0")
	}
	if got["Selected"] != "1.4.2" {
		t.Errorf("Selected = %q, want %q", got["Selected"], "1.4.2")
	}
}

func TestFluxObject_ImagePolicyFallsBackToLatestImage(t *testing.T) {
	policy := fluxObj(map[string]interface{}{
		"policy": map[string]interface{}{"alphabetical": map[string]interface{}{"order": "asc"}},
	}, map[string]interface{}{
		"latestImage": "registry.io/app:2.0.1",
	})
	got := fieldMap(fluxObjectFields(policy, "ImagePolicy"))
	if got["Policy"] != "alphabetical asc" {
		t.Errorf("Policy = %q", got["Policy"])
	}
	if got["Selected"] != "2.0.1" {
		t.Errorf("Selected = %q, want the tag parsed out of latestImage", got["Selected"])
	}
}

func TestFluxObject_MissingPathsDegradeSilently(t *testing.T) {
	// A GitRepository with nothing configured beyond its URL.
	bare := fluxObj(map[string]interface{}{"url": "https://h/o/r.git"}, nil)
	fields := fluxObjectFields(bare, "GitRepository")
	for _, f := range fields {
		if f.Value == "" {
			t.Errorf("field %q has an empty value; absent paths must emit no field at all", f.Label)
		}
	}
	if got := fieldMap(fields); got["Branch"] != "" || got["Interval"] != "" {
		t.Errorf("absent paths must not appear: %v", got)
	}

	// A claimed kind with no recognized paths at all.
	unknown := fluxObj(map[string]interface{}{"whatever": "x"}, nil)
	if fields := fluxObjectFields(unknown, "ImageUpdateAutomation"); len(fields) != 0 {
		t.Errorf("an unrecognized kind must produce no fields, got %v", fields)
	}
}

func TestFluxObject_HealthFromReadyCondition(t *testing.T) {
	obj := fluxObj(map[string]interface{}{"url": "https://h/o/r.git"}, map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"type": "Ready", "status": "False", "message": "failed to checkout"},
		},
	})
	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{"GitRepository|flux|flux": obj}}
	res, err := FluxObjectResolver{}.Resolve(context.Background(), &ResolveContext{K8s: getter},
		K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Health != engine.Unhealthy || res.Detail != "failed to checkout" {
		t.Errorf("health = %q detail = %q, want unhealthy with the Ready message", res.Health, res.Detail)
	}
	if len(res.Children) != 0 {
		t.Errorf("source objects are leaves, got %d children", len(res.Children))
	}
}

// TestRegistry_FluxObjectBeatsFallback pins the registration order: without it,
// GitRepository would silently fall back to the generic resolver and lose its
// fields.
func TestRegistry_FluxObjectBeatsFallback(t *testing.T) {
	reg := NewDefaultRegistry()
	got := reg.For(K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux"))
	if _, ok := got.(FluxObjectResolver); !ok {
		t.Errorf("GitRepository resolved by %T, want FluxObjectResolver", got)
	}
}

func TestHumanizeInterval(t *testing.T) {
	tests := map[string]string{
		"1m0s":   "1m",
		"1h0m0s": "1h",
		"1h30m":  "1h30m",
		"10m":    "10m", // must not be eaten into "1"
		"100m":   "100m",
		"30s":    "30s",
		"1h":     "1h",
		"":       "",
	}
	for in, want := range tests {
		if got := humanizeInterval(in); got != want {
			t.Errorf("humanizeInterval(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFieldsForFetched(t *testing.T) {
	git := fluxObj(map[string]interface{}{
		"url": "https://gitlab.example.com/infra/fleet.git",
		"ref": map[string]interface{}{"branch": "main"}, "interval": "1m0s",
	}, nil)

	got := fieldMap(FieldsForFetched(K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux"), git))
	if got["Repo"] != "gitlab.example.com/infra/fleet" || got["Branch"] != "main" || got["Interval"] != "1m" {
		t.Errorf("fields from an object in hand = %v", got)
	}

	// A non-Flux kind yields nothing rather than erroring.
	cfg := &unstructured.Unstructured{Object: map[string]interface{}{"data": map[string]interface{}{"k": "v"}}}
	if f := FieldsForFetched(K8sRef("v1", "ConfigMap", "ns", "cfg"), cfg); len(f) != 0 {
		t.Errorf("a ConfigMap must yield no fields, got %v", f)
	}
	// A nil object is tolerated.
	if f := FieldsForFetched(K8sRef("source.toolkit.fluxcd.io/v1", "GitRepository", "flux", "flux"), nil); len(f) != 0 {
		t.Errorf("a nil object must yield no fields, got %v", f)
	}
}
