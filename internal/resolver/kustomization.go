package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// kustomizationGroup is the Flux API group for Kustomization objects.
const kustomizationGroup = "kustomize.toolkit.fluxcd.io"

// KustomizationResolver resolves Flux Kustomization objects. It discovers
// children from .status.inventory.entries and reports health from the
// Kustomization's Ready condition.
type KustomizationResolver struct{}

// Matches claims kubernetes references of kind Kustomization in the Flux group.
func (KustomizationResolver) Matches(ref engine.Ref) bool {
	apiVersion, kind, _, _, err := DecodeK8sRef(ref)
	if err != nil {
		return false
	}
	if kind != "Kustomization" {
		return false
	}
	// apiVersion is "<group>/<version>"; the group must be the Flux group.
	group, _, _ := strings.Cut(apiVersion, "/")
	return group == kustomizationGroup
}

// Resolve fetches the Kustomization, decodes its inventory into child
// references, and derives health from its Ready condition.
func (KustomizationResolver) Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error) {
	obj, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return engine.Result{}, err
	}

	children, err := inventoryChildren(obj)
	if err != nil {
		return engine.Result{}, err
	}

	health, detail := healthFromReady(obj)
	return engine.Result{Health: health, Detail: detail, Children: children}, nil
}

// inventoryChildren decodes .status.inventory.entries into child references.
// Each entry has an id "<namespace>_<name>_<group>_<kind>" and a version "v".
// An absent or empty inventory yields no children (not an error).
func inventoryChildren(obj *unstructured.Unstructured) ([]engine.Ref, error) {
	entries, found, err := unstructured.NestedSlice(obj.Object, "status", "inventory", "entries")
	if err != nil {
		return nil, fmt.Errorf("reading inventory: %w", err)
	}
	if !found || len(entries) == 0 {
		return nil, nil
	}

	refs := make([]engine.Ref, 0, len(entries))
	for _, e := range entries {
		entry, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := entry["id"].(string)
		version, _ := entry["v"].(string)

		namespace, name, group, kind, err := parseInventoryID(id)
		if err != nil {
			return nil, err
		}

		apiVersion := version
		if group != "" {
			apiVersion = group + "/" + version
		}
		refs = append(refs, K8sRef(apiVersion, kind, namespace, name))
	}
	return refs, nil
}

// parseInventoryID splits a Flux inventory id into its parts. Kubernetes names,
// namespaces, groups and kinds never contain "_", so a 4-way split is exact.
func parseInventoryID(id string) (namespace, name, group, kind string, err error) {
	parts := strings.Split(id, "_")
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("malformed inventory id %q (want namespace_name_group_kind)", id)
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}
