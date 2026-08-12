package command

import (
	"fmt"

	"github.com/maknihamdi/fluxexp/internal/engine"
	"github.com/maknihamdi/fluxexp/internal/k8s"
	"github.com/maknihamdi/fluxexp/internal/render"
	"github.com/maknihamdi/fluxexp/internal/resolver"

	"github.com/spf13/cobra"
)

// defaultAPIVersion is the Flux Kustomization apiVersion, the primary starting
// point; override with --api-version to start from any other Kubernetes type.
const defaultAPIVersion = "kustomize.toolkit.fluxcd.io/v1"

func newTraverseCmd() *cobra.Command {
	var (
		apiVersion string
		kind       string
		namespace  string
		name       string
		kubeconfig string
		context    string
	)

	cmd := &cobra.Command{
		Use:   "traverse",
		Short: "Traverse the resource graph starting from a resource",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			client, err := k8s.LoadClient(kubeconfig, context)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			reg := resolver.NewRegistry()
			reg.Register(resolver.KustomizationResolver{})
			reg.Register(resolver.HelmReleaseResolver{})
			reg.RegisterFallback(resolver.DomainK8s, resolver.GenericK8sResolver{})

			rc := &resolver.ResolveContext{K8s: client}
			root := resolver.K8sRef(apiVersion, kind, namespace, name)
			node := engine.Traverse(root, reg.ResolveFunc(cmd.Context(), rc))

			// A failure to resolve the *root* means the start resource is
			// missing/unreachable: report it and exit non-zero. Errors deeper in
			// the tree are tolerated (partial failure) and rendered inline.
			if node.Health == engine.Error {
				return fmt.Errorf("cannot resolve start resource %s: %s", root.Label(), node.Err)
			}

			fmt.Fprint(cmd.OutOrStdout(), render.Tree(node))
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&apiVersion, "api-version", defaultAPIVersion, "apiVersion of the start resource")
	f.StringVar(&kind, "kind", "Kustomization", "kind of the start resource")
	f.StringVarP(&namespace, "namespace", "n", "", "namespace of the start resource (empty for cluster-scoped)")
	f.StringVar(&name, "name", "", "name of the start resource (required)")
	f.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig (default: KUBECONFIG or ~/.kube/config)")
	f.StringVar(&context, "context", "", "kubeconfig context to use (default: current-context)")

	return cmd
}
