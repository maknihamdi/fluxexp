// Package k8s wires a read-only Kubernetes dynamic client. It knows nothing
// about the traversal engine: it fetches unstructured objects by
// apiVersion/kind/namespace/name, mapping the GVK to a resource via live
// discovery. Domain-specific reference encoding lives in the resolver package.
package k8s

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is a read-only dynamic Kubernetes client with a discovery-backed
// RESTMapper for resolving arbitrary (including CRD) GVKs.
type Client struct {
	dyn    dynamic.Interface
	mapper meta.RESTMapper
}

// LoadClient builds a Client from the standard kubeconfig resolution. An empty
// kubeconfig path falls back to KUBECONFIG / the default path; an empty context
// uses the current-context. It returns a clear error when no cluster config can
// be assembled.
func LoadClient(kubeconfigPath, contextName string) (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		rules.ExplicitPath = kubeconfigPath
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}
	// Traversal is read-only and inventory routinely references deprecated API
	// versions; suppress server deprecation warnings so the tree stays clean.
	cfg.WarningHandler = rest.NoWarnings{}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building dynamic client: %w", err)
	}

	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building discovery client: %w", err)
	}
	groups, err := restmapper.GetAPIGroupResources(disco)
	if err != nil {
		return nil, fmt.Errorf("discovering API groups (is the cluster reachable?): %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)

	return &Client{dyn: dyn, mapper: mapper}, nil
}

// Get fetches a single object by apiVersion/kind and namespace/name. It maps the
// GVK to a resource via the RESTMapper and branches on namespaced vs
// cluster-scoped. A nil/empty namespace is treated as cluster-scoped.
func (c *Client) Get(ctx context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing apiVersion %q: %w", apiVersion, err)
	}
	gvk := gv.WithKind(kind)

	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("mapping %s: %w", gvk.String(), err)
	}

	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace && namespace != "" {
		ri = c.dyn.Resource(mapping.Resource).Namespace(namespace)
	} else {
		ri = c.dyn.Resource(mapping.Resource)
	}

	obj, err := ri.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting %s %s/%s: %w", kind, namespace, name, err)
	}
	return obj, nil
}

// Namespaced reports whether the given apiVersion/kind is a namespaced resource,
// resolved via the discovery-backed RESTMapper.
func (c *Client) Namespaced(apiVersion, kind string) (bool, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false, fmt.Errorf("parsing apiVersion %q: %w", apiVersion, err)
	}
	gvk := gv.WithKind(kind)
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return false, fmt.Errorf("mapping %s: %w", gvk.String(), err)
	}
	return mapping.Scope.Name() == meta.RESTScopeNameNamespace, nil
}

// List returns all objects of the given apiVersion/kind. An empty namespace
// lists across all namespaces (for namespaced kinds) or cluster-wide.
func (c *Client) List(ctx context.Context, apiVersion, kind, namespace string) ([]unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing apiVersion %q: %w", apiVersion, err)
	}
	gvk := gv.WithKind(kind)

	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("mapping %s: %w", gvk.String(), err)
	}

	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace && namespace != "" {
		ri = c.dyn.Resource(mapping.Resource).Namespace(namespace)
	} else {
		ri = c.dyn.Resource(mapping.Resource)
	}

	list, err := ri.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", kind, err)
	}
	return list.Items, nil
}
