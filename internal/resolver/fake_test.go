package resolver

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fakeGetter is a K8sGetter backed by an in-memory object map keyed by
// "kind|namespace|name". An empty map with err set simulates fetch failures.
// clusterScoped names kinds that should report as cluster-scoped; anything else
// is treated as namespaced.
type fakeGetter struct {
	objs          map[string]*unstructured.Unstructured
	lists         map[string][]unstructured.Unstructured // "kind|namespace"
	err           error
	clusterScoped map[string]bool
}

func (f fakeGetter) Get(_ context.Context, _ /*apiVersion*/, kind, namespace, name string) (*unstructured.Unstructured, error) {
	if f.err != nil {
		return nil, f.err
	}
	key := kind + "|" + namespace + "|" + name
	obj, ok := f.objs[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return obj, nil
}

func (f fakeGetter) List(_ context.Context, _ /*apiVersion*/, kind, namespace string) ([]unstructured.Unstructured, error) {
	return f.lists[kind+"|"+namespace], nil
}

func (f fakeGetter) Namespaced(_ /*apiVersion*/, kind string) (bool, error) {
	return !f.clusterScoped[kind], nil
}

// A note on numbers in these builders: every integer MUST be an int64 literal.
// Health derivation runs through kstatus, which reads metadata.generation and
// status.observedGeneration with unstructured's typed accessors and fails hard
// on anything else — "accessor error: 1 is of the type float64, expected
// int64". A fixture decoded with encoding/json produces exactly that float64,
// so decode fixtures with k8s.io/apimachinery/pkg/util/json if one is ever
// added here.

// objWithReady builds an object carrying a Ready condition.
func objWithReady(status, message string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": status, "message": message},
			},
		},
	}}
}

// objNoConditions builds an object with no status conditions.
func objNoConditions() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{}}
}

// condition builds one status condition entry.
func condition(condType, status, message string) interface{} {
	return map[string]interface{}{"type": condType, "status": status, "message": message}
}

// statusObj builds an apiVersion/kind-bearing object with the given status
// conditions. The kind matters: kstatus has built-in rules for some of them.
func statusObj(apiVersion, kind string, conds ...interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]interface{}{"name": "x", "namespace": "ns"},
	}}
	if len(conds) > 0 {
		obj.Object["status"] = map[string]interface{}{"conditions": conds}
	}
	return obj
}

// withGenerations sets metadata.generation and status.observedGeneration.
// Both are int64 for the reason given at the top of this file.
func withGenerations(obj *unstructured.Unstructured, generation, observed int64) *unstructured.Unstructured {
	_ = unstructured.SetNestedField(obj.Object, generation, "metadata", "generation")
	_ = unstructured.SetNestedField(obj.Object, observed, "status", "observedGeneration")
	return obj
}

// kustomizationObj builds a Kustomization with the given Ready status and
// inventory ids ("namespace_name_group_kind"); version applies to all entries.
func kustomizationObj(readyStatus, readyMsg, version string, ids ...string) *unstructured.Unstructured {
	entries := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, map[string]interface{}{"id": id, "v": version})
	}
	status := map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"type": "Ready", "status": readyStatus, "message": readyMsg},
		},
	}
	if len(entries) > 0 {
		status["inventory"] = map[string]interface{}{"entries": entries}
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{"status": status}}
}
