package resolver

import (
	"fmt"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

// K8sHealth derives a domain-independent Health from any Kubernetes object. It
// is the single source of Kubernetes health derivation, shared by every
// resolver, the list command and the UI: no call site may read conditions,
// compare generations or count replicas on its own.
//
// The base verdict comes from kstatus, the library Flux uses for its own health
// checks. Two things sit on top of it, and both are deliberate — see
// openspec/changes/.../add-universal-object-status/design.md:
//
//   - Ready=False is unhealthy, where kstatus says in-progress (Decision 4).
//   - A *generic* Current is re-read through a condition-polarity table
//     (Decision 3), because kstatus only knows Reconciling, Stalled and Ready
//     and reports every other condition type as current — which would show a
//     Bundle with Synced=False in green.
//
// The order below mirrors kstatus's own decision order so each override lands
// at the point where kstatus would otherwise have decided.
func K8sHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	// The controller has not observed the spec it is being asked to reconcile,
	// so everything else its status says describes a superseded one. This runs
	// first: a stale "not ready" must not be reported as broken.
	if detail, drifting := GenerationDrift(obj); drifting {
		return engine.Pending, detail
	}

	// Core workloads have rules of their own, in workload.go beside the resolver
	// that descends them. They are consulted here rather than only there so that
	// a Pod listed in a UI layer and the same Pod resolved by the CLI cannot
	// disagree.
	if health, detail, ok := workloadHealth(obj); ok {
		return health, detail
	}

	res, err := status.Compute(obj)
	if err != nil {
		// The object publishes a status that cannot be interpreted. This is
		// what unknown is for, and the only thing it is for.
		return engine.Unknown, err.Error()
	}

	// Being deleted is neither working nor broken.
	if res.Status == status.TerminatingStatus {
		return engine.Pending, res.Message
	}

	// An object that declares itself not ready is shown as broken, not as still
	// converging. kstatus classifies this as in-progress; this is the one place
	// fluxexp knowingly disagrees with flux.
	if s, message, ok := readyCondition(obj); ok && s == "False" {
		return engine.Unhealthy, message
	}

	// GetLegacyConditionsFn is nil when kstatus has no rule for this kind, so a
	// Current here means it recognised nothing at all: not the kind, not a
	// condition type. Only then may the polarity table have the last word.
	if res.Status == status.CurrentStatus && status.GetLegacyConditionsFn(obj) == nil {
		if health, detail, ok := polarityHealth(obj); ok {
			return health, detail
		}
	}

	switch res.Status {
	case status.CurrentStatus:
		return engine.Healthy, res.Message
	case status.InProgressStatus:
		return engine.Pending, res.Message
	case status.FailedStatus, status.NotFoundStatus:
		return engine.Unhealthy, res.Message
	default:
		return engine.Unknown, res.Message
	}
}

// GenerationDrift reports whether obj's controller has yet to observe the spec
// it is being asked to reconcile, and a detail naming both generations.
//
// It is exported so WorkloadResolver can ask the same question before trusting
// replica counts, rather than comparing generations a second time.
func GenerationDrift(obj *unstructured.Unstructured) (string, bool) {
	observed, found := nestedInt(obj, "status", "observedGeneration")
	if !found {
		return "", false
	}
	generation := obj.GetGeneration()
	if generation <= observed {
		return "", false
	}
	return fmt.Sprintf("generation %d, observed %d", generation, observed), true
}

// conditionPolarity maps a condition type to its polarity: true when the
// condition reports success (so False is bad news), false when it reports a
// problem (so True is bad news).
//
// The table cannot be replaced by a rule. Nothing in a condition's shape says
// which way it points: Degraded=False is good news and Synced=False is bad
// news, and both are just a type and a status. Kubernetes never standardised
// beyond Reconciling/Stalled/Ready, which is exactly the set kstatus covers, so
// everything below is what fluxexp adds. New types are one line each.
var conditionPolarity = map[string]bool{
	"Available":   true,
	"Complete":    true,
	"Configured":  true,
	"Established": true,
	"Healthy":     true,
	"Succeeded":   true,
	"Synced":      true,

	"Degraded": false,
	"Failed":   false,
	"Stalled":  false,
}

// polarityHealth reads obj's conditions through conditionPolarity. It reports
// ok=false when the object carries no condition the table knows, leaving the
// caller's verdict untouched. A violated condition wins over a satisfied one,
// and whichever decides carries its message as the detail — far more useful
// than kstatus's generic "Resource is current".
func polarityHealth(obj *unstructured.Unstructured) (health engine.Health, detail string, ok bool) {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return "", "", false
	}
	for _, c := range conditions {
		cond, isMap := c.(map[string]interface{})
		if !isMap {
			continue
		}
		condType, _ := cond["type"].(string)
		positive, known := conditionPolarity[condType]
		if !known {
			continue
		}
		condStatus, _ := cond["status"].(string)
		message, _ := cond["message"].(string)
		if message == "" {
			message = condType + "=" + condStatus
		}
		if condStatus != "True" && condStatus != "False" {
			continue
		}
		if positive == (condStatus == "True") {
			health, detail, ok = engine.Healthy, message, true
			continue // keep looking: a violated condition outranks this one
		}
		return engine.Unhealthy, message, true
	}
	return health, detail, ok
}
