// Package resolver holds the resolver contract, the matcher-based registry, the
// shared resolve context, and the concrete resolvers. Resolvers own their own
// retrieval (through the shared clients in ResolveContext) and may emit children
// in a different domain than their own. The traversal engine stays domain-blind.
package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DomainK8s is the domain name for Kubernetes-hosted objects.
const DomainK8s = "kubernetes"

// K8sGetter fetches Kubernetes objects, lists them, and answers resource-scope
// questions. *k8s.Client satisfies it; tests provide a fake.
type K8sGetter interface {
	Get(ctx context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error)
	// List returns all objects of apiVersion/kind in a namespace (empty = all).
	List(ctx context.Context, apiVersion, kind, namespace string) ([]unstructured.Unstructured, error)
	// Namespaced reports whether the apiVersion/kind is a namespaced resource.
	Namespaced(apiVersion, kind string) (bool, error)
}

// ResolveContext carries the shared, lazily-built clients per domain. In this
// increment only the Kubernetes client is present; future domains (e.g. gcp)
// add their own client here without changing the engine or existing resolvers.
type ResolveContext struct {
	K8s K8sGetter
}

// Resolver interprets references of the type(s) it matches. It retrieves the
// underlying object itself (via rc) and returns the object's health plus child
// references. Implementations MUST be read-only.
type Resolver interface {
	// Matches reports whether this resolver handles the given reference.
	Matches(ref engine.Ref) bool
	// Resolve retrieves and interprets the reference.
	Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error)
}

// K8sRef builds a Kubernetes-domain reference. Type encodes apiVersion and kind;
// Coords carry namespace and name (namespace empty for cluster-scoped).
func K8sRef(apiVersion, kind, namespace, name string) engine.Ref {
	display := kind + " " + name
	if namespace != "" {
		display = kind + " " + namespace + "/" + name
	}
	return engine.Ref{
		Domain:  DomainK8s,
		Type:    apiVersion + "|" + kind,
		Coords:  map[string]string{"namespace": namespace, "name": name},
		Display: display,
	}
}

// DecodeK8sRef extracts apiVersion, kind, namespace and name from a
// Kubernetes-domain reference.
func DecodeK8sRef(ref engine.Ref) (apiVersion, kind, namespace, name string, err error) {
	if ref.Domain != DomainK8s {
		return "", "", "", "", fmt.Errorf("not a kubernetes reference: domain %q", ref.Domain)
	}
	parts := strings.SplitN(ref.Type, "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", "", "", fmt.Errorf("malformed kubernetes type %q (want apiVersion|Kind)", ref.Type)
	}
	return parts[0], parts[1], ref.Coords["namespace"], ref.Coords["name"], nil
}

// GetK8s fetches the object referenced by a Kubernetes-domain reference.
func (rc *ResolveContext) GetK8s(ctx context.Context, ref engine.Ref) (*unstructured.Unstructured, error) {
	apiVersion, kind, namespace, name, err := DecodeK8sRef(ref)
	if err != nil {
		return nil, err
	}
	if rc == nil || rc.K8s == nil {
		return nil, fmt.Errorf("no kubernetes client configured")
	}
	return rc.K8s.Get(ctx, apiVersion, kind, namespace, name)
}

// readyCondition returns the status and message of the object's Ready condition.
func readyCondition(obj *unstructured.Unstructured) (status, message string, found bool) {
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !ok {
		return "", "", false
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := cond["type"].(string); t != "Ready" {
			continue
		}
		status, _ = cond["status"].(string)
		message, _ = cond["message"].(string)
		return status, message, true
	}
	return "", "", false
}

// K8sHealth derives a domain-independent Health from a Kubernetes object's
// Ready condition. It is the single source of the Ready-condition health logic,
// shared by the generic resolver and the list command.
func K8sHealth(obj *unstructured.Unstructured) (engine.Health, string) {
	return healthFromReady(obj)
}

// healthFromReady maps a Ready condition status to a domain-independent Health.
func healthFromReady(obj *unstructured.Unstructured) (engine.Health, string) {
	status, message, found := readyCondition(obj)
	if !found {
		return engine.Unknown, ""
	}
	switch status {
	case "True":
		return engine.Healthy, message
	case "False":
		return engine.Unhealthy, message
	default:
		return engine.Unknown, message
	}
}
