package resolver

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fakeGetter is a K8sGetter backed by an in-memory object map keyed by
// "kind|namespace|name". An empty map with err set simulates fetch failures.
type fakeGetter struct {
	objs map[string]*unstructured.Unstructured
	err  error
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
