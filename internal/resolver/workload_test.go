package resolver

import (
	"context"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func replicaObj(spec *int64, ready int64) *unstructured.Unstructured {
	status := map[string]interface{}{"readyReplicas": ready}
	specMap := map[string]interface{}{}
	if spec != nil {
		specMap["replicas"] = *spec
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{"spec": specMap, "status": status}}
}

func i64(v int64) *int64 { return &v }

func TestReplicaHealth(t *testing.T) {
	cases := []struct {
		name   string
		spec   *int64
		ready  int64
		want   engine.Health
		detail string
	}{
		{"fully ready", i64(2), 2, engine.Healthy, "2/2 ready"},
		{"under ready", i64(3), 1, engine.Unhealthy, "1/3 ready"},
		{"zero desired", i64(0), 0, engine.Healthy, "0/0 ready"},
		{"default desired 1 ready", nil, 1, engine.Healthy, "1/1 ready"},
		{"default desired 0 ready", nil, 0, engine.Unhealthy, "0/1 ready"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, d := replicaHealth(replicaObj(tc.spec, tc.ready))
			if h != tc.want || d != tc.detail {
				t.Fatalf("got %q %q, want %q %q", h, d, tc.want, tc.detail)
			}
		})
	}
}

func TestDaemonSetHealth(t *testing.T) {
	ds := func(desired, ready int64) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"status": map[string]interface{}{"desiredNumberScheduled": desired, "numberReady": ready},
		}}
	}
	if h, _ := daemonSetHealth(ds(4, 4)); h != engine.Healthy {
		t.Fatalf("4/4 should be healthy, got %q", h)
	}
	if h, d := daemonSetHealth(ds(4, 2)); h != engine.Unhealthy || d != "2/4 ready" {
		t.Fatalf("2/4 should be unhealthy, got %q %q", h, d)
	}
	if h, _ := daemonSetHealth(ds(0, 0)); h != engine.Healthy {
		t.Fatalf("0/0 should be healthy, got %q", h)
	}
}

func podObj(phase, ready string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"phase":      phase,
			"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": ready}},
		},
	}}
}

func TestPodHealth(t *testing.T) {
	cases := []struct {
		phase, ready string
		want         engine.Health
	}{
		{"Running", "True", engine.Healthy},
		{"Running", "False", engine.Unhealthy},
		{"Succeeded", "", engine.Healthy},
		{"Failed", "", engine.Unhealthy},
		{"Pending", "", engine.Unknown},
	}
	for _, tc := range cases {
		if h, _ := podHealth(podObj(tc.phase, tc.ready)); h != tc.want {
			t.Fatalf("phase %s ready %s -> %q, want %q", tc.phase, tc.ready, h, tc.want)
		}
	}
}

func TestJobHealth(t *testing.T) {
	job := func(condType, status string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{map[string]interface{}{"type": condType, "status": status}},
			},
		}}
	}
	if h, _ := jobHealth(job("Complete", "True")); h != engine.Healthy {
		t.Fatalf("complete job should be healthy, got %q", h)
	}
	if h, _ := jobHealth(job("Failed", "True")); h != engine.Unhealthy {
		t.Fatalf("failed job should be unhealthy, got %q", h)
	}
	if h, _ := jobHealth(job("Complete", "False")); h != engine.Unknown {
		t.Fatalf("running job should be unknown, got %q", h)
	}
}

func TestWorkloadResolver_Matches(t *testing.T) {
	r := WorkloadResolver{}
	for _, ref := range []engine.Ref{
		K8sRef("apps/v1", "Deployment", "ns", "web"),
		K8sRef("apps/v1", "StatefulSet", "ns", "db"),
		K8sRef("apps/v1", "DaemonSet", "ns", "agent"),
		K8sRef("v1", "Pod", "ns", "web-abc"),
		K8sRef("batch/v1", "Job", "ns", "migrate"),
	} {
		if !r.Matches(ref) {
			t.Fatalf("should claim %s", ref.Type)
		}
	}
	if r.Matches(K8sRef("v1", "ConfigMap", "ns", "cfg")) {
		t.Fatal("must not claim ConfigMap")
	}
	if r.Matches(K8sRef("v1", "Service", "ns", "svc")) {
		t.Fatal("must not claim Service")
	}
}

func TestWorkloadResolver_ResolveNoChildren(t *testing.T) {
	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{
		"Deployment|ns|web": replicaObj(i64(2), 2),
	}}
	rc := &ResolveContext{K8s: getter}
	res, err := WorkloadResolver{}.Resolve(context.Background(), rc, K8sRef("apps/v1", "Deployment", "ns", "web"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Health != engine.Healthy || res.Detail != "2/2 ready" || len(res.Children) != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestNewDefaultRegistry_Selection(t *testing.T) {
	reg := NewDefaultRegistry()
	if _, ok := reg.For(K8sRef("apps/v1", "Deployment", "ns", "web")).(WorkloadResolver); !ok {
		t.Fatal("Deployment should select the workload resolver")
	}
	if _, ok := reg.For(K8sRef("v1", "ConfigMap", "ns", "cfg")).(GenericK8sResolver); !ok {
		t.Fatal("ConfigMap should fall back to the generic resolver")
	}
	if _, ok := reg.For(K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps")).(KustomizationResolver); !ok {
		t.Fatal("Kustomization should select the kustomization resolver")
	}
}
