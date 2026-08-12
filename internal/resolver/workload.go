package resolver

import (
	"context"
	"fmt"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// workloadKinds maps API group → the workload kinds handled in that group.
var workloadKinds = map[string]map[string]bool{
	"apps":  {"Deployment": true, "StatefulSet": true, "DaemonSet": true, "ReplicaSet": true},
	"batch": {"Job": true},
	"":      {"Pod": true}, // core group
}

// WorkloadResolver reports health for core Kubernetes workload kinds from their
// status (replica counts, phase, conditions). Workloads are leaves here.
type WorkloadResolver struct{}

// Matches claims the workload kinds in the kubernetes domain.
func (WorkloadResolver) Matches(ref engine.Ref) bool {
	apiVersion, kind, _, _, err := DecodeK8sRef(ref)
	if err != nil {
		return false
	}
	kinds, ok := workloadKinds[groupOf(apiVersion)]
	return ok && kinds[kind]
}

// Resolve fetches the workload and computes its health; it returns no children.
func (WorkloadResolver) Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error) {
	obj, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return engine.Result{}, err
	}
	_, kind, _, _, _ := DecodeK8sRef(ref)

	var health engine.Health
	var detail string
	switch kind {
	case "Deployment", "StatefulSet", "ReplicaSet":
		health, detail = replicaHealth(obj)
	case "DaemonSet":
		health, detail = daemonSetHealth(obj)
	case "Pod":
		health, detail = podHealth(obj)
	case "Job":
		health, detail = jobHealth(obj)
	default:
		health = engine.Unknown
	}
	return engine.Result{Health: health, Detail: detail}, nil
}

// groupOf returns the API group of an apiVersion ("" for the core group).
func groupOf(apiVersion string) string {
	for i := 0; i < len(apiVersion); i++ {
		if apiVersion[i] == '/' {
			return apiVersion[:i]
		}
	}
	return ""
}

// replicaHealth compares ready replicas to the desired count.
func replicaHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	desired := int64(1)
	if v, ok := nestedInt(obj, "spec", "replicas"); ok {
		desired = v
	}
	ready, _ := nestedInt(obj, "status", "readyReplicas")
	if desired == 0 {
		return engine.Healthy, "0/0 ready"
	}
	detail := fmt.Sprintf("%d/%d ready", ready, desired)
	if ready >= desired {
		return engine.Healthy, detail
	}
	return engine.Unhealthy, detail
}

// daemonSetHealth compares numberReady to desiredNumberScheduled.
func daemonSetHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	desired, _ := nestedInt(obj, "status", "desiredNumberScheduled")
	ready, _ := nestedInt(obj, "status", "numberReady")
	detail := fmt.Sprintf("%d/%d ready", ready, desired)
	if ready >= desired {
		return engine.Healthy, detail
	}
	return engine.Unhealthy, detail
}

// podHealth derives health from the pod phase and Ready condition.
func podHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	switch phase {
	case "Running":
		if s, _, ok := readyCondition(obj); ok && s == "True" {
			return engine.Healthy, phase
		}
		return engine.Unhealthy, phase
	case "Succeeded":
		return engine.Healthy, phase
	case "Failed":
		return engine.Unhealthy, phase
	default:
		return engine.Unknown, phase
	}
}

// jobHealth reads the Complete/Failed conditions.
func jobHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	if conditionTrue(obj, "Complete") {
		return engine.Healthy, "complete"
	}
	if conditionTrue(obj, "Failed") {
		return engine.Unhealthy, "failed"
	}
	return engine.Unknown, "running"
}

// nestedInt reads an integer status/spec field tolerantly (int64 in unstructured).
func nestedInt(obj *unstructured.Unstructured, fields ...string) (int64, bool) {
	v, found, err := unstructured.NestedInt64(obj.Object, fields...)
	if err != nil || !found {
		return 0, false
	}
	return v, true
}

// conditionTrue reports whether the object has a condition of condType with
// status "True".
func conditionTrue(obj *unstructured.Unstructured, condType string) bool {
	conds, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, c := range conds {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := cond["type"].(string); t == condType {
			s, _ := cond["status"].(string)
			return s == "True"
		}
	}
	return false
}
