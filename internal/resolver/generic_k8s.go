package resolver

import (
	"context"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// GenericK8sResolver is the fallback resolver for the kubernetes domain. It
// handles any Kubernetes reference not claimed by a more specific resolver,
// reporting health from the object's Ready condition and returning no children.
//
// Health semantics ("Ready") are Kubernetes-specific, which is why this is a
// per-domain fallback rather than a universal one.
type GenericK8sResolver struct{}

// Matches claims any reference in the kubernetes domain.
func (GenericK8sResolver) Matches(ref engine.Ref) bool {
	return ref.Domain == DomainK8s
}

// Expandable reports false: the fallback reads a Ready condition and never
// returns children, so an unknown kind is always a leaf.
func (GenericK8sResolver) Expandable(engine.Ref) bool { return false }

// Resolve fetches the object and derives health from its Ready condition.
func (GenericK8sResolver) Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error) {
	obj, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return engine.Result{}, err
	}
	health, detail := healthFromReady(obj)
	return engine.Result{Health: health, Detail: detail}, nil
}
