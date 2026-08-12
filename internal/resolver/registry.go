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

// ResolveFunc adapts the registry to the engine's ResolveFunc, binding the
// resolve context. A reference with no matching resolver and no domain fallback
// is surfaced as a resolve error (which the engine turns into an Error node).
func (r *Registry) ResolveFunc(ctx context.Context, rc *ResolveContext) engine.ResolveFunc {
	return func(ref engine.Ref) (engine.Result, error) {
		res := r.For(ref)
		if res == nil {
			return engine.Result{}, fmt.Errorf("no resolver for domain %q type %q", ref.Domain, ref.Type)
		}
		return res.Resolve(ctx, rc, ref)
	}
}
