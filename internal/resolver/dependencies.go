package resolver

import (
	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DependencyRefsForFetched returns the references an already-fetched object
// declares as dependencies, read straight from its own manifest. It performs no
// retrieval, never errors, and never computes the object's children — what a
// Kustomization or a HelmRelease needs in order to reconcile is right there in
// the YAML a caller already holds.
//
// For a HelmRelease that is not merely an optimisation: resolving one reads and
// gunzips its Helm storage Secret, so a listed row must reach its dependencies
// without going through the resolver.
//
// It is the single source of these references — each resolver's Resolve calls it
// too — so what the engine traverses and what a listed row shows cannot drift
// apart. A kind absent from the dispatch below yields nothing.
func DependencyRefsForFetched(ref engine.Ref, obj *unstructured.Unstructured) []engine.Ref {
	if obj == nil {
		return nil
	}
	apiVersion, kind, ns, _, err := DecodeK8sRef(ref)
	if err != nil {
		return nil
	}
	switch group := groupOf(apiVersion); {
	case kind == "Kustomization" && group == kustomizationGroup:
		return kustomizationDependencies(obj, ns)
	case kind == "HelmRelease" && group == helmReleaseGroup:
		return helmReleaseDependencies(obj, apiVersion, ns)
	case kind == "HelmChart" && group == fluxSourceGroup:
		return helmChartDependencies(obj, ns)
	}
	return nil
}

// dependsOnRefs turns a spec.dependsOn list into references, in declaration
// order, defaulting an omitted namespace to the declaring object's own. Flux's
// dependsOn always names objects of the declaring kind, so the caller passes
// that kind and the apiVersion serving it.
//
// Entries are emitted without pre-fetching: an object waiting on something that
// does not exist is exactly what the reader needs to see, as an error node.
func dependsOnRefs(obj *unstructured.Unstructured, apiVersion, kind, ns string) []engine.Ref {
	entries, found, _ := unstructured.NestedSlice(obj.Object, "spec", "dependsOn")
	if !found {
		return nil
	}
	var refs []engine.Ref
	for _, e := range entries {
		entry, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			continue
		}
		depNS, _ := entry["namespace"].(string)
		if depNS == "" {
			depNS = ns
		}
		refs = append(refs, K8sRef(apiVersion, kind, depNS, name))
	}
	return refs
}

// sourceRefAt reads a {kind, name, namespace} reference at the given path and
// builds a Flux source reference from it, defaulting an omitted namespace to
// defaultNS. Every source kind a Flux object can name — GitRepository, Bucket,
// HelmRepository, OCIRepository, HelmChart — is served under the same source
// apiVersion, so no kind-to-version table is needed. ok is false when the
// reference is not declared.
func sourceRefAt(obj *unstructured.Unstructured, defaultNS string, path ...string) (engine.Ref, bool) {
	at := func(field string) string {
		v, _, _ := unstructured.NestedString(obj.Object, append(append([]string{}, path...), field)...)
		return v
	}
	kind, name := at("kind"), at("name")
	if kind == "" || name == "" {
		return engine.Ref{}, false
	}
	ns := at("namespace")
	if ns == "" {
		ns = defaultNS
	}
	return K8sRef(sourceAPIVersion, kind, ns, name), true
}
