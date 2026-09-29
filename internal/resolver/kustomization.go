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

// kustomizationAPIVersion is the Flux API version used for spec.dependsOn
// entries, which are always Kustomizations.
const kustomizationAPIVersion = "kustomize.toolkit.fluxcd.io/v1"

// sourceAPIVersion is the Flux source API version used for spec.sourceRef
// objects (GitRepository, OCIRepository, Bucket, HelmRepository).
const sourceAPIVersion = "source.toolkit.fluxcd.io/v1"

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

// Expandable reports true: a Kustomization always descends into its inventory
// (which may be empty — expandability describes the type, not the instance).
func (KustomizationResolver) Expandable(engine.Ref) bool { return true }

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

	health, detail := K8sHealth(obj)

	// Compute reconciliation freshness by comparing the applied revision to the
	// source's fetched revision (cluster-only). A failed source fetch degrades
	// gracefully inside KustomizationFreshness.
	_, _, ns, _, _ := DecodeK8sRef(ref)
	source := fetchKustomizationSource(ctx, rc, obj, ns)
	freshness, fields := KustomizationFreshness(obj, source)

	return engine.Result{
		Health:       health,
		Detail:       detail,
		Freshness:    freshness,
		Fields:       fields,
		Dependencies: DependencyRefsForFetched(ref, obj),
		Children:     children,
	}, nil
}

// kustomizationDependencies lists what the Kustomization needs in order to
// reconcile: its source first, then its spec.dependsOn entries in declaration
// order.
//
// Every configured reference is emitted without pre-fetching. One that cannot be
// retrieved becomes an error node downstream, which is the right signal: a
// Kustomization pointing at a missing source or waiting on a missing dependency
// is exactly what the reader needs to see. Only an unconfigured reference is
// absent.
func kustomizationDependencies(ks *unstructured.Unstructured, ns string) []engine.Ref {
	var deps []engine.Ref
	if srcRef, ok := kustomizationSourceRef(ks, ns); ok {
		deps = append(deps, srcRef)
	}
	return append(deps, dependsOnRefs(ks, kustomizationAPIVersion, "Kustomization", ns)...)
}

// kustomizationSourceRef builds the reference to the Kustomization's source
// (spec.sourceRef), defaulting the namespace to the Kustomization's own. The
// second result is false when no source is configured.
func kustomizationSourceRef(ks *unstructured.Unstructured, ns string) (engine.Ref, bool) {
	return sourceRefAt(ks, ns, "spec", "sourceRef")
}

// fetchKustomizationSource fetches the Kustomization's source object
// (spec.sourceRef). Returns nil when it is unspecified or cannot be fetched.
func fetchKustomizationSource(ctx context.Context, rc *ResolveContext, ks *unstructured.Unstructured, ns string) *unstructured.Unstructured {
	ref, ok := kustomizationSourceRef(ks, ns)
	if !ok {
		return nil
	}
	src, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return nil
	}
	return src
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
