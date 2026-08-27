package ui

import (
	"context"
	"sync"

	"github.com/maknihamdi/fluxexp/internal/resolver"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// memoGetter wraps a cluster for the duration of a single request, returning a
// previously fetched object instead of calling the API again.
//
// It exists because one layer legitimately asks for the same object many times:
// every Kustomization in a list resolves its own source, and on a real cluster
// they overwhelmingly share one GitRepository. Without this, a 23-entry layer
// spent most of its time re-fetching the same handful of objects.
//
// Its lifetime is one request on purpose: caching across requests would serve
// stale health, which is the one thing this tool must not do.
type memoGetter struct {
	inner resolver.K8sGetter

	mu   sync.Mutex
	objs map[string]memoEntry
}

type memoEntry struct {
	obj *unstructured.Unstructured
	err error
}

func newMemoGetter(inner resolver.K8sGetter) *memoGetter {
	return &memoGetter{inner: inner, objs: map[string]memoEntry{}}
}

// Get returns the cached result when the same object was already requested in
// this request. Failures are cached too: a missing dependsOn target is looked up
// once, not once per dependent.
func (m *memoGetter) Get(ctx context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
	key := apiVersion + "|" + kind + "|" + namespace + "|" + name

	m.mu.Lock()
	entry, ok := m.objs[key]
	m.mu.Unlock()
	if ok {
		return entry.obj, entry.err
	}

	obj, err := m.inner.Get(ctx, apiVersion, kind, namespace, name)

	m.mu.Lock()
	m.objs[key] = memoEntry{obj: obj, err: err}
	m.mu.Unlock()
	return obj, err
}

// List is not memoized: listings are used once per request, and caching them
// would buy nothing while risking a stale set.
func (m *memoGetter) List(ctx context.Context, apiVersion, kind, namespace string) ([]unstructured.Unstructured, error) {
	return m.inner.List(ctx, apiVersion, kind, namespace)
}

// Namespaced delegates; the underlying client already answers it from its
// discovery-backed RESTMapper without a call.
func (m *memoGetter) Namespaced(apiVersion, kind string) (bool, error) {
	return m.inner.Namespaced(apiVersion, kind)
}
