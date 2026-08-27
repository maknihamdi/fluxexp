package resolver

import (
	"context"
	"fmt"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// Registry selects a resolver for a reference. Specific resolvers are tried in
// registration order (first matcher wins); when none match, the fallback
// registered for the reference's domain is used.
type Registry struct {
	resolvers []Resolver
	fallbacks map[string]Resolver
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{fallbacks: map[string]Resolver{}}
}

// NewDefaultRegistry returns the standard resolver set shared by the CLI and the
// web UI: specific resolvers (Kustomization, HelmRelease, workloads) ahead of
// the generic Kubernetes fallback. Add new resolvers here so both surfaces
// resolve identically.
func NewDefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(KustomizationResolver{})
	reg.Register(HelmReleaseResolver{})
	reg.Register(WorkloadResolver{})
	reg.Register(FluxObjectResolver{})
	reg.RegisterFallback(DomainK8s, GenericK8sResolver{})
	return reg
}

// Register adds a specific resolver. Order matters: earlier registrations take
// precedence when multiple matchers claim the same reference.
func (r *Registry) Register(res Resolver) {
	r.resolvers = append(r.resolvers, res)
}

// RegisterFallback sets the fallback resolver for a domain, used when no
// specific resolver matches a reference in that domain.
func (r *Registry) RegisterFallback(domain string, res Resolver) {
	r.fallbacks[domain] = res
}

// For returns the resolver that should handle ref: the first specific matcher,
// otherwise the domain fallback, otherwise nil.
func (r *Registry) For(ref engine.Ref) Resolver {
	for _, res := range r.resolvers {
		if res.Matches(ref) {
			return res
		}
	}
	if fb, ok := r.fallbacks[ref.Domain]; ok {
		return fb
	}
	return nil
}

// Expandable reports whether ref can descend to a child layer, by delegating to
// the resolver that would resolve it (same specific-before-fallback selection).
// A reference no resolver claims is not expandable.
func (r *Registry) Expandable(ref engine.Ref) bool {
	res := r.For(ref)
	if res == nil {
		return false
	}
	return res.Expandable(ref)
}

// OrderChildren returns refs in three tiers: Flux objects, then the remaining
// expandable references, then everything else. A Flux object outranks a
// non-Flux container even when it is itself a leaf — reading a Kustomization,
// "what drives this?" comes before "what does it run?".
//
// The partition is stable: each tier keeps the order the resolver produced
// (roughly apply order for an inventory), so output stays deterministic.
//
// This is the single implementation of the rule: ResolveFunc applies it for the
// engine (and so the CLI), and the web UI applies it to the children it renders.
func (r *Registry) OrderChildren(refs []engine.Ref) []engine.Ref {
	if len(refs) < 2 {
		return refs
	}
	flux := make([]engine.Ref, 0, len(refs))
	var containers, rest []engine.Ref
	for _, ref := range refs {
		switch {
		case IsFluxRef(ref):
			flux = append(flux, ref)
		case r.Expandable(ref):
			containers = append(containers, ref)
		default:
			rest = append(rest, ref)
		}
	}
	return append(append(flux, containers...), rest...)
}

// ResolveFunc adapts the registry to the engine's ResolveFunc, binding the
// resolve context. A reference with no matching resolver and no domain fallback
// is surfaced as a resolve error (which the engine turns into an Error node).
func (r *Registry) ResolveFunc(ctx context.Context, rc *ResolveContext) engine.ResolveFunc {
	return func(ref engine.Ref) (engine.Result, error) {
		res := r.For(ref)
		if res == nil {
			return engine.Result{}, fmt.Errorf("no resolver for domain %q type %q", ref.Domain, ref.Type)
		}
		result, err := res.Resolve(ctx, rc, ref)
		if err != nil {
			return engine.Result{}, err
		}
		result.Children = withoutDependencies(r.OrderChildren(result.Children), result.Dependencies)
		result.Expandable = res.Expandable(ref)
		return result, nil
	}
}

// withoutDependencies removes from children any reference already declared as a
// dependency. An object can genuinely be both — a Flux Kustomization routinely
// applies the very GitRepository it reconciles from — but listing it under both
// headings states the same fact twice. The dependency grouping wins, because
// that is where the reference explains something about its parent.
func withoutDependencies(children, deps []engine.Ref) []engine.Ref {
	if len(children) == 0 || len(deps) == 0 {
		return children
	}
	declared := make(map[string]struct{}, len(deps))
	for _, d := range deps {
		declared[d.Key()] = struct{}{}
	}
	kept := children[:0:0]
	for _, c := range children {
		if _, dup := declared[c.Key()]; dup {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}
