package resolver

import (
	"context"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// GenericK8sResolver is the fallback resolver for the kubernetes domain. It
// handles any Kubernetes reference not claimed by a more specific resolver,
// reporting health through the shared derivation and returning no children.
// Every kind reaches a verdict there — objects with no status, conditions other
// than Ready, and generation drift included — so this resolver owns no health
// logic of its own.
//
// Health semantics are Kubernetes-specific, which is why this is a per-domain
// fallback rather than a universal one.
type GenericK8sResolver struct{}

// Matches claims any reference in the kubernetes domain.
func (GenericK8sResolver) Matches(ref engine.Ref) bool {
	return ref.Domain == DomainK8s
}

// Expandable reports false: the fallback only reads status and never returns
// children, so an unknown kind is always a leaf.
func (GenericK8sResolver) Expandable(engine.Ref) bool { return false }

// Resolve fetches the object and derives its health.
func (GenericK8sResolver) Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error) {
	obj, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return engine.Result{}, err
	}
	health, detail := K8sHealth(obj)
	return engine.Result{Health: health, Detail: detail}, nil
}
