package resolver

import (
	"context"
	"strings"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Every builder here sets apiVersion and kind. Health derivation dispatches on
// them to find the workload rules, exactly as it does for an object returned by
// the dynamic client, so a fixture without them is not a workload at all.
func replicaObj(spec *int64, ready int64) *unstructured.Unstructured {
	status := map[string]interface{}{"readyReplicas": ready}
	specMap := map[string]interface{}{}
	if spec != nil {
		specMap["replicas"] = *spec
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"spec": specMap, "status": status,
	}}
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

// A rollout that has just been declared has stale replica counts: they describe
// the spec the controller last observed, not the one it is being asked for.
// Reporting that as unhealthy blames a rollout for not having started.
func TestReplicaHealth_UnobservedSpecIsPending(t *testing.T) {
	obj := replicaObj(i64(3), 0)
	withGenerations(obj, 4, 3)

	h, d := replicaHealth(obj)
	if h != engine.Pending {
		t.Fatalf("got %q %q, want pending", h, d)
	}
	if !strings.Contains(d, "4") || !strings.Contains(d, "3") {
		t.Fatalf("detail = %q, want both generations named", d)
	}

	// Once the controller has caught up, the counts mean what they say.
	withGenerations(obj, 4, 4)
	if h, d := replicaHealth(obj); h != engine.Unhealthy || d != "0/3 ready" {
		t.Fatalf("got %q %q, want unhealthy 0/3 ready once observed", h, d)
	}
}

func TestDaemonSetHealth(t *testing.T) {
	ds := func(desired, ready int64) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apps/v1", "kind": "DaemonSet",
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
		"apiVersion": "v1", "kind": "Pod",
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
		// A pod waiting to be scheduled or pulling images is readable: it is
		// pending. Only the Unknown phase — the node stopped reporting — is
		// genuinely unreadable.
		{"Pending", "", engine.Pending},
		{"Unknown", "", engine.Unknown},
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
			"apiVersion": "batch/v1", "kind": "Job",
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
	if h, _ := jobHealth(job("Complete", "False")); h != engine.Pending {
		t.Fatalf("running job should be pending, got %q", h)
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

// --- descent (ownerReferences) -------------------------------------------

func ownerRefs(kind, ownerUID string) []interface{} {
	return []interface{}{map[string]interface{}{
		"apiVersion": "apps/v1", "kind": kind, "name": "owner", "uid": ownerUID,
	}}
}

func rsObj(ns, name, uid, ownerUID string, replicas int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "ReplicaSet",
		"metadata": map[string]interface{}{
			"namespace": ns, "name": name, "uid": uid,
			"ownerReferences": ownerRefs("Deployment", ownerUID),
		},
		"status": map[string]interface{}{"replicas": replicas},
	}}
}

func podOwned(ns, name, ownerUID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": map[string]interface{}{
			"namespace": ns, "name": name,
			"ownerReferences": ownerRefs("ReplicaSet", ownerUID),
		},
		"status": map[string]interface{}{
			"phase":      "Running",
			"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}},
		},
	}}
}

func deployObj(ns, name, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"namespace": ns, "name": name, "uid": uid},
		"spec":     map[string]interface{}{"replicas": int64(1)},
		"status":   map[string]interface{}{"readyReplicas": int64(1)},
	}}
}

func TestWorkload_DeploymentDescendsToActiveReplicaSets(t *testing.T) {
	getter := fakeGetter{
		objs: map[string]*unstructured.Unstructured{
			"Deployment|ns|web": deployObj("ns", "web", "dep-uid"),
		},
		lists: map[string][]unstructured.Unstructured{
			"ReplicaSet|ns": {
				*rsObj("ns", "web-active", "rs1", "dep-uid", 1), // owned + active
				*rsObj("ns", "web-old", "rs2", "dep-uid", 0),    // owned + scaled to 0
				*rsObj("ns", "other", "rs3", "other-uid", 1),    // not owned
			},
		},
	}
	rc := &ResolveContext{K8s: getter}
	res, err := WorkloadResolver{}.Resolve(context.Background(), rc, K8sRef("apps/v1", "Deployment", "ns", "web"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Health != engine.Healthy {
		t.Fatalf("health = %q", res.Health)
	}
	if len(res.Children) != 1 || res.Children[0].Coords["name"] != "web-active" {
		t.Fatalf("want only the active ReplicaSet, got %+v", res.Children)
	}
	if !contains(res.Detail, "1 old revisions") {
		t.Fatalf("detail should note omitted revisions, got %q", res.Detail)
	}
}

func TestWorkload_ReplicaSetDescendsToPods(t *testing.T) {
	getter := fakeGetter{
		objs: map[string]*unstructured.Unstructured{
			"ReplicaSet|ns|web": rsObj("ns", "web", "rs-uid", "dep-uid", 1),
		},
		lists: map[string][]unstructured.Unstructured{
			"Pod|ns": {
				*podOwned("ns", "p1", "rs-uid"),
				*podOwned("ns", "p2", "rs-uid"),
				*podOwned("ns", "p3", "other-uid"), // not owned
			},
		},
	}
	rc := &ResolveContext{K8s: getter}
	res, err := WorkloadResolver{}.Resolve(context.Background(), rc, K8sRef("apps/v1", "ReplicaSet", "ns", "web"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Children) != 2 {
		t.Fatalf("want 2 owned pods, got %d: %+v", len(res.Children), res.Children)
	}
}

func TestWorkload_PodIsLeaf(t *testing.T) {
	getter := fakeGetter{objs: map[string]*unstructured.Unstructured{
		"Pod|ns|p1": podOwned("ns", "p1", "rs-uid"),
	}}
	rc := &ResolveContext{K8s: getter}
	res, err := WorkloadResolver{}.Resolve(context.Background(), rc, K8sRef("v1", "Pod", "ns", "p1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Children) != 0 {
		t.Fatalf("pod must be a leaf, got %d children", len(res.Children))
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

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
