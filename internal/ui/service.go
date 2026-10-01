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
	// The in-cluster sentinel is a name for a mode, not a kubeconfig context: an
	// empty context is what resolves in-cluster, and passing the sentinel through
	// would fail validation instead. This is the single place the UI turns a
	// context name into a client, so it is the only place that needs to know.
	requested := contextName
	if requested == k8s.InClusterContext {
		requested = ""
	}
	c, err := s.newClient(requested)
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
	srcCache := map[string]*unstructured.Unstructured{}
	for i := range objs {
		ks := &objs[i]
		roots = append(roots, rootSummary(ks, s.sourceFor(c, ks, srcCache)))
	}
	return roots, nil
}

// sourceFor fetches a Kustomization's source object, caching by identity so a
// shared source (e.g. the "flux" GitRepository) is fetched once per listing.
func (s *Service) sourceFor(c cluster, ks *unstructured.Unstructured, cache map[string]*unstructured.Unstructured) *unstructured.Unstructured {
	kind, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "kind")
	name, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "name")
	if kind == "" || name == "" {
		return nil
	}
	ns, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "namespace")
	if ns == "" {
		ns = ks.GetNamespace()
	}
	key := kind + "|" + ns + "|" + name
	if src, ok := cache[key]; ok {
		return src
	}
	src, err := c.Get(context.Background(), "source.toolkit.fluxcd.io/v1", kind, ns, name)
	if err != nil {
		src = nil
	}
	cache[key] = src
	return src
}

// Expand resolves one node fully (its health/detail and its child refs) and
// computes a cheap health for each child, without expanding the children.
func (s *Service) Expand(contextName string, ref engine.Ref) (NodeDTO, error) {
	node := NodeDTO{Ref: refToDTO(ref), Context: contextName}

	c, err := s.clientFor(contextName)
	if err != nil {
		return node, err
	}
	// One layer asks for the same objects repeatedly — every listed Kustomization
	// resolves its own source, and they usually share one. Memoize for the
	// duration of this request only.
	rc := &resolver.ResolveContext{K8s: newMemoGetter(c)}

	// Resolve through the registry's own entry point, the one the engine uses, so
	// tier ordering, de-duplication against dependencies and the expandable flag
	// are applied here exactly as they are for the CLI — rather than being
	// re-implemented, and drifting.
	res, err := s.registry.ResolveFunc(context.Background(), rc)(ref)
	if err != nil {
		node.Health = string(engine.Error)
		node.Err = err.Error()
		return node, nil
	}
	node.Health = string(res.Health)
	node.Freshness = string(res.Freshness)
	node.Detail = res.Detail
	node.Fields = fieldsToDTO(res.Fields)
	node.Expandable = res.Expandable

	// Dependencies are resolved one level deep, like the engine does: enough for
	// the card to show each one's health and fields, never descending into them.
	node.Dependencies = s.resolveLayer(rc, res.Dependencies, contextName)
	node.Children = s.resolveLayer(rc, res.Children, contextName)

	// A reference can be applied by this node *and* be the source of one of its
	// own children — Flux project layouts routinely apply a Kustomization next to
	// the GitRepository it reconciles from. It is then already shown grouped
	// under that child, so drop the redundant top-level row.
	node.Children = dropNestedDuplicates(node.Children, node.Dependencies)
	return node, nil
}

// dropNestedDuplicates removes a top-level entry that is already displayed as
// another entry's nested dependency in the same layer. The grouped occurrence
// wins: it is the one that says why the reference is there.
//
// An entry carrying a dependency group of its own is always kept, so two entries
// that reference each other cannot make both vanish.
func dropNestedDuplicates(children, cardDeps []NodeDTO) []NodeDTO {
	nested := map[string]struct{}{}
	collect := func(entries []NodeDTO) {
		for _, e := range entries {
			for _, dep := range e.Dependencies {
				nested[dtoKey(dep.Ref)] = struct{}{}
			}
		}
	}
	collect(children)
	collect(cardDeps)
	if len(nested) == 0 {
		return children
	}

	kept := children[:0:0]
	for _, c := range children {
		_, dup := nested[dtoKey(c.Ref)]
		if dup && len(c.Dependencies) == 0 {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}

// dtoKey identifies a reference for comparison within a layer.
func dtoKey(r RefDTO) string {
	return r.Domain + "|" + r.Type + "|" + r.Namespace + "|" + r.Name
}

// layerConcurrency bounds how many entries of one layer are resolved at once.
// The entries are independent read-only lookups dominated by round-trip latency,
// so a small pool turns a serial walk into a near-constant wait without flooding
// the API server.
const layerConcurrency = 8

// resolveLayer resolves every entry of a layer, concurrently but order-preserving:
// results are written by index, so the tier ordering computed upstream survives.
func (s *Service) resolveLayer(rc *resolver.ResolveContext, refs []engine.Ref, contextName string) []NodeDTO {
	if len(refs) == 0 {
		return nil
	}
	out := make([]NodeDTO, len(refs))
	sem := make(chan struct{}, layerConcurrency)
	var wg sync.WaitGroup

	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref engine.Ref) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = s.listedNode(rc, ref, contextName)
		}(i, ref)
	}
	wg.Wait()
	return out
}

// listedNode renders one entry of a list: the object itself, plus the
// dependencies it declares in its own manifest, grouped with it.
//
// It never resolves the entry. A Kustomization's spec.sourceRef and
// spec.dependsOn are in the YAML fetched for its health, so reading them costs
// nothing — and its inventory, which is what resolving would compute, is
// irrelevant to the group. Each derived dependency is then retrieved once, for
// its health and fields only.
func (s *Service) listedNode(rc *resolver.ResolveContext, ref engine.Ref, contextName string) NodeDTO {
	node, obj := s.fetchedNode(rc, ref, contextName)
	if obj == nil {
		return node
	}
	for _, dep := range resolver.DependencyRefsForFetched(ref, obj) {
		node.Dependencies = append(node.Dependencies, s.childHealth(rc, dep, contextName))
	}
	return node
}

// childHealth computes a cheap health for a child (fetch + Ready condition),
// without running the child's full resolver.
func (s *Service) childHealth(rc *resolver.ResolveContext, ref engine.Ref, contextName string) NodeDTO {
	node, _ := s.fetchedNode(rc, ref, contextName)
	return node
}

// fetchedNode retrieves the object once and derives everything obtainable from
// it: health, detail and fields. It returns the object too, so a caller can read
// more from it without a second call. A failed retrieval yields an error node
// and a nil object.
func (s *Service) fetchedNode(rc *resolver.ResolveContext, ref engine.Ref, contextName string) (NodeDTO, *unstructured.Unstructured) {
	node := NodeDTO{Ref: refToDTO(ref), Context: contextName, Expandable: s.registry.Expandable(ref)}
	obj, err := rc.GetK8s(context.Background(), ref)
	if err != nil {
		node.Health = string(engine.Error)
		node.Err = err.Error()
		return node, nil
	}
	h, d := resolver.K8sHealth(obj)
	node.Health = string(h)
	node.Detail = d
	// The object is already in hand, so its fields cost nothing: a Flux source or
	// image row shows repo/branch/interval without the user opening it.
	node.Fields = fieldsToDTO(resolver.FieldsForFetched(ref, obj))
	return node, obj
}

// rootSummary extracts the home-page summary from a Kustomization object and its
// source (may be nil), including reconciliation freshness.
func rootSummary(obj, source *unstructured.Unstructured) RootDTO {
	ns := obj.GetNamespace()
	name := obj.GetName()
	ref := resolver.K8sRef("kustomize.toolkit.fluxcd.io/v1", "Kustomization", ns, name)

	h, _ := resolver.K8sHealth(obj)
	freshness, fields := resolver.KustomizationFreshness(obj, source)
	dto := RootDTO{
		Ref:       refToDTO(ref),
		Health:    string(h),
		Freshness: string(freshness),
		Fields:    fieldsToDTO(fields),
	}

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
