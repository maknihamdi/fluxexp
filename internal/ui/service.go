// Package ui serves the local, read-only exploration portal: kube-context
// listing/selection, the root Kustomizations home, and layer-by-layer expansion
// that reuses the CLI's resolvers (one hop per call). No authentication; it acts
// with the local kubeconfig's credentials.
package ui

import (
	"context"
	"sync"

	"github.com/maknihamdi/fluxexp/internal/engine"
	"github.com/maknihamdi/fluxexp/internal/k8s"
	"github.com/maknihamdi/fluxexp/internal/resolver"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// cluster is the read-only cluster access the service needs. *k8s.Client
// satisfies it; tests provide a fake. K8sGetter already covers Get/List/Namespaced.
type cluster interface {
	resolver.K8sGetter
}

// Service resolves portal requests. It caches one cluster client per context and
// shares a single resolver registry (identical behavior to the CLI).
type Service struct {
	mu        sync.Mutex
	clients   map[string]cluster
	newClient func(contextName string) (cluster, error)
	registry  *resolver.Registry
}

// NewService builds a service that talks to real clusters via the kubeconfig.
func NewService() *Service {
	return newService(func(contextName string) (cluster, error) {
		return k8s.LoadClient("", contextName)
	})
}

// newService is the injectable constructor used by tests.
func newService(newClient func(contextName string) (cluster, error)) *Service {
	return &Service{
		clients:   map[string]cluster{},
		newClient: newClient,
		registry:  resolver.NewDefaultRegistry(),
	}
}

// Contexts lists the kube contexts from the local kubeconfig.
func (s *Service) Contexts() ([]k8s.ContextInfo, error) {
	return k8s.ListContexts()
}

func (s *Service) clientFor(contextName string) (cluster, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.clients[contextName]; ok {
		return c, nil
	}
	c, err := s.newClient(contextName)
	if err != nil {
		return nil, err
	}
	s.clients[contextName] = c
	return c, nil
}

// Roots lists the root Flux Kustomizations for a context. An empty namespace
// defaults to "flux".
func (s *Service) Roots(contextName, namespace string) ([]RootDTO, error) {
	if namespace == "" {
		namespace = "flux"
	}
	c, err := s.clientFor(contextName)
	if err != nil {
		return nil, err
	}
	objs, err := c.List(context.Background(), "kustomize.toolkit.fluxcd.io/v1", "Kustomization", namespace)
	if err != nil {
		return nil, err
	}
	roots := make([]RootDTO, 0, len(objs))
	for i := range objs {
		roots = append(roots, rootSummary(&objs[i]))
	}
	return roots, nil
}

// Expand resolves one node fully (its health/detail and its child refs) and
// computes a cheap health for each child, without expanding the children.
func (s *Service) Expand(contextName string, ref engine.Ref) (NodeDTO, error) {
	node := NodeDTO{Ref: refToDTO(ref), Context: contextName}

	c, err := s.clientFor(contextName)
	if err != nil {
		return node, err
	}
	rc := &resolver.ResolveContext{K8s: c}

	r := s.registry.For(ref)
	if r == nil {
		node.Health = string(engine.Error)
		node.Err = "no resolver for " + ref.Domain + "/" + ref.Type
		return node, nil
	}
	res, err := r.Resolve(context.Background(), rc, ref)
	if err != nil {
		node.Health = string(engine.Error)
		node.Err = err.Error()
		return node, nil
	}
	node.Health = string(res.Health)
	node.Detail = res.Detail
	for _, child := range res.Children {
		node.Children = append(node.Children, s.childHealth(rc, child, contextName))
	}
	return node, nil
}

// childHealth computes a cheap health for a child (fetch + Ready condition),
// without running the child's full resolver.
func (s *Service) childHealth(rc *resolver.ResolveContext, ref engine.Ref, contextName string) NodeDTO {
	child := NodeDTO{Ref: refToDTO(ref), Context: contextName}
	obj, err := rc.GetK8s(context.Background(), ref)
	if err != nil {
		child.Health = string(engine.Error)
		child.Err = err.Error()
		return child
	}
	h, d := resolver.K8sHealth(obj)
	child.Health = string(h)
	child.Detail = d
	return child
}

// rootSummary extracts the home-page summary from a Kustomization object.
func rootSummary(obj *unstructured.Unstructured) RootDTO {
	ns := obj.GetNamespace()
	name := obj.GetName()
	ref := resolver.K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", ns, name)

	h, _ := resolver.K8sHealth(obj)
	dto := RootDTO{Ref: refToDTO(ref), Health: string(h)}

	dto.SourceKind, _, _ = unstructured.NestedString(obj.Object, "spec", "sourceRef", "kind")
	dto.SourceName, _, _ = unstructured.NestedString(obj.Object, "spec", "sourceRef", "name")
	dto.Path, _, _ = unstructured.NestedString(obj.Object, "spec", "path")
	dto.Interval, _, _ = unstructured.NestedString(obj.Object, "spec", "interval")
	dto.Revision, _, _ = unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")

	if conds, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions"); found {
		for _, c := range conds {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if t, _ := cond["type"].(string); t != "Ready" {
				continue
			}
			dto.Message, _ = cond["message"].(string)
			dto.LastTransition, _ = cond["lastTransitionTime"].(string)
			break
		}
	}
	return dto
}
