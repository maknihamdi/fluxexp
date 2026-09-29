package resolver

import (
	"context"
	"fmt"
	"strings"

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

// Expandable reports whether the workload descends to owned objects. Every kind
// this resolver claims descends except a Pod, which is the end of the chain.
func (r WorkloadResolver) Expandable(ref engine.Ref) bool {
	if !r.Matches(ref) {
		return false
	}
	_, kind, _, _, _ := DecodeK8sRef(ref)
	return kind != "Pod"
}

// Resolve fetches the workload and computes its health; it returns no children.
func (WorkloadResolver) Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error) {
	obj, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return engine.Result{}, err
	}
	_, kind, _, _, _ := DecodeK8sRef(ref)

	// The kind-specific rules below live in workloadHealth, which K8sHealth also
	// consults. Going through the shared entry point is what keeps a Pod from
	// reading pending here and unhealthy in a UI layer that only lists it.
	health, detail := K8sHealth(obj)

	children, extra, err := workloadChildren(ctx, rc, obj, ref, kind)
	if err != nil {
		return engine.Result{}, err
	}
	if extra != "" {
		detail = strings.TrimSpace(detail + " · " + extra)
	}
	return engine.Result{Health: health, Detail: detail, Children: children}, nil
}

// workloadChildren descends a workload to the objects it owns. It returns the
// child references and an optional extra detail (e.g. omitted old revisions).
func workloadChildren(ctx context.Context, rc *ResolveContext, obj *unstructured.Unstructured, ref engine.Ref, kind string) ([]engine.Ref, string, error) {
	ns := ref.Coords["namespace"]
	uid := string(obj.GetUID())

	switch kind {
	case "Deployment":
		// Owned ReplicaSets, keeping only active revisions (replicas > 0).
		var omitted int
		children, err := ownedChildren(ctx, rc, uid, ns, "apps/v1", "ReplicaSet", func(rs *unstructured.Unstructured) bool {
			if n, _ := nestedInt(rs, "status", "replicas"); n > 0 {
				return true
			}
			omitted++
			return false
		})
		extra := ""
		if omitted > 0 {
			extra = fmt.Sprintf("%d old revisions", omitted)
		}
		return children, extra, err
	case "ReplicaSet", "StatefulSet", "DaemonSet", "Job":
		children, err := ownedChildren(ctx, rc, uid, ns, "v1", "Pod", nil)
		return children, "", err
	default: // Pod: leaf
		return nil, "", nil
	}
}

// ownedChildren lists childKind in namespace ns and returns references to those
// owned by parentUID (via ownerReferences), optionally filtered by keep.
func ownedChildren(ctx context.Context, rc *ResolveContext, parentUID, ns, childAPIVersion, childKind string, keep func(*unstructured.Unstructured) bool) ([]engine.Ref, error) {
	if rc == nil || rc.K8s == nil {
		return nil, fmt.Errorf("no kubernetes client configured")
	}
	objs, err := rc.K8s.List(ctx, childAPIVersion, childKind, ns)
	if err != nil {
		return nil, fmt.Errorf("listing %s in %s: %w", childKind, ns, err)
	}
	var refs []engine.Ref
	for i := range objs {
		child := &objs[i]
		if !ownedBy(child, parentUID) {
			continue
		}
		if keep != nil && !keep(child) {
			continue
		}
		refs = append(refs, K8sRef(childAPIVersion, childKind, child.GetNamespace(), child.GetName()))
	}
	return refs, nil
}

// ownedBy reports whether obj has an ownerReference with the given UID.
func ownedBy(obj *unstructured.Unstructured, parentUID string) bool {
	if parentUID == "" {
		return false
	}
	for _, o := range obj.GetOwnerReferences() {
		if string(o.UID) == parentUID {
			return true
		}
	}
	return false
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
//
// It asks the shared generation check first: until the controller has observed
// the current spec, the replica counts still describe the previous one, and
// reporting "0/3 ready" as broken would blame a rollout for not having started.
func replicaHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	if detail, drifting := GenerationDrift(obj); drifting {
		return engine.Pending, detail
	}
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

// workloadHealth answers for the kinds this resolver claims, reporting ok=false
// for anything else so the caller keeps its own verdict.
//
// It dispatches on the object rather than on a reference because K8sHealth is
// also reached from surfaces that hold an object and no resolver — the UI's
// listed entries. Both paths must reach the same rules: these counts and phases
// say more than kstatus's generic phrasing ("0/3 ready" against "Deployment is
// not available"), and a kind must not have two verdicts depending on which
// surface asked.
func workloadHealth(obj *unstructured.Unstructured) (engine.Health, string, bool) {
	kinds, known := workloadKinds[groupOf(obj.GetAPIVersion())]
	kind := obj.GetKind()
	if !known || !kinds[kind] {
		return "", "", false
	}
	switch kind {
	case "Deployment", "StatefulSet", "ReplicaSet":
		health, detail := replicaHealth(obj)
		return health, detail, true
	case "DaemonSet":
		health, detail := daemonSetHealth(obj)
		return health, detail, true
	case "Pod":
		health, detail := podHealth(obj)
		return health, detail, true
	case "Job":
		health, detail := jobHealth(obj)
		return health, detail, true
	}
	return "", "", false
}

// podHealth derives health from the pod phase and Ready condition.
//
// Only the `Unknown` phase yields unknown, and it earns it: that phase means
// the API server lost contact with the node, so the pod's state genuinely
// cannot be read. A `Pending` pod is readable — it is scheduling or pulling
// images — and is reported as pending.
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
	case "Pending":
		return engine.Pending, phase
	default:
		return engine.Unknown, phase
	}
}

// jobHealth reads the Complete/Failed conditions. A job that has neither is
// still running: work in flight, not an unreadable state.
func jobHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	if conditionTrue(obj, "Complete") {
		return engine.Healthy, "complete"
	}
	if conditionTrue(obj, "Failed") {
		return engine.Unhealthy, "failed"
	}
	return engine.Pending, "running"
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
