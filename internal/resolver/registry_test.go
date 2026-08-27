package resolver

import (
	"context"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

// stubResolver matches refs whose Type equals want and tags its detail so tests
// can tell which resolver ran.
type stubResolver struct {
	want       string
	tag        string
	expandable bool
}

func (s stubResolver) Matches(ref engine.Ref) bool { return ref.Type == s.want }
func (s stubResolver) Resolve(_ context.Context, _ *ResolveContext, _ engine.Ref) (engine.Result, error) {
	return engine.Result{Health: engine.Healthy, Detail: s.tag}, nil
}
func (s stubResolver) Expandable(_ engine.Ref) bool { return s.expandable }

func TestRegistry_SpecificBeforeFallback(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubResolver{want: "Special", tag: "specific"})
	reg.RegisterFallback(DomainK8s, stubResolver{want: "", tag: "fallback"})

	got := reg.For(engine.Ref{Domain: DomainK8s, Type: "Special"})
	if s, ok := got.(stubResolver); !ok || s.tag != "specific" {
		t.Fatalf("expected specific resolver, got %#v", got)
	}
}

func TestRegistry_FallbackWhenNoMatch(t *testing.T) {
	reg := NewRegistry()
	reg.Register(stubResolver{want: "Special", tag: "specific"})
	fb := stubResolver{tag: "fallback"}
	reg.RegisterFallback(DomainK8s, fb)

	got := reg.For(engine.Ref{Domain: DomainK8s, Type: "Other"})
	if s, ok := got.(stubResolver); !ok || s.tag != "fallback" {
		t.Fatalf("expected fallback resolver, got %#v", got)
	}
}

func TestRegistry_NoResolverForUnknownDomain(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterFallback(DomainK8s, stubResolver{tag: "fallback"})

	if got := reg.For(engine.Ref{Domain: "gcp", Type: "sql.Instance"}); got != nil {
		t.Fatalf("expected nil for domain with no fallback, got %#v", got)
	}
}

func TestRegistry_ResolveFuncSurfacesMissingResolver(t *testing.T) {
	reg := NewRegistry()
	fn := reg.ResolveFunc(context.Background(), &ResolveContext{})
	if _, err := fn(engine.Ref{Domain: "gcp", Type: "sql.Instance"}); err == nil {
		t.Fatal("expected an error when no resolver handles the ref")
	}
}
