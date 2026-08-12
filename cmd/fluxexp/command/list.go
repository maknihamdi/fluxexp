package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"
	"github.com/maknihamdi/fluxexp/internal/k8s"
	"github.com/maknihamdi/fluxexp/internal/render"
	"github.com/maknihamdi/fluxexp/internal/resolver"

	"github.com/spf13/cobra"
)

// knownKinds maps common Flux kinds to their apiVersion so `list --kind X` works
// without an explicit --api-version. Override with --api-version for any other
// kind. Keyed by lower-cased kind for case-insensitive lookup.
var knownKinds = map[string]string{
	"kustomization":  "kustomize.toolkit.fluxcd.io/v1",
	"helmrelease":    "helm.toolkit.fluxcd.io/v2",
	"gitrepository":  "source.toolkit.fluxcd.io/v1",
	"helmrepository": "source.toolkit.fluxcd.io/v1",
	"helmchart":      "source.toolkit.fluxcd.io/v1",
	"ocirepository":  "source.toolkit.fluxcd.io/v1",
	"bucket":         "source.toolkit.fluxcd.io/v1",
}

func newListCmd() *cobra.Command {
	var (
		kind          string
		apiVersion    string
		namespace     string
		allNamespaces bool
		unhealthyOnly bool
		kubeconfig    string
		context       string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Flux resources of a kind with their health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolvedKind, resolvedAPIVersion, err := resolveKind(kind, apiVersion)
			if err != nil {
				return err
			}

			client, err := k8s.LoadClient(kubeconfig, context)
			if err != nil {
				return fmt.Errorf("connecting to cluster: %w", err)
			}

			// -n selects a single namespace; otherwise list across all.
			ns := ""
			if namespace != "" && !allNamespaces {
				ns = namespace
			}

			objs, err := client.List(cmd.Context(), resolvedAPIVersion, resolvedKind, ns)
			if err != nil {
				return err
			}

			rows := make([]render.ResourceStatus, 0, len(objs))
			for i := range objs {
				obj := &objs[i]
				health, detail := resolver.K8sHealth(obj)
				if unhealthyOnly && health == engine.Healthy {
					continue
				}
				rows = append(rows, render.ResourceStatus{
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
					Health:    health,
					Detail:    detail,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Namespace != rows[j].Namespace {
					return rows[i].Namespace < rows[j].Namespace
				}
				return rows[i].Name < rows[j].Name
			})

			fmt.Fprint(cmd.OutOrStdout(), render.List(rows))
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d %s(s)\n", len(rows), resolvedKind)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&kind, "kind", "Kustomization", "Flux kind to list")
	f.StringVar(&apiVersion, "api-version", "", "apiVersion of the kind (auto for known Flux kinds)")
	f.StringVarP(&namespace, "namespace", "n", "", "list a single namespace")
	f.BoolVarP(&allNamespaces, "all-namespaces", "A", true, "list across all namespaces (default)")
	f.BoolVar(&unhealthyOnly, "unhealthy", false, "show only resources whose health is not healthy")
	f.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig (default: KUBECONFIG or ~/.kube/config)")
	f.StringVar(&context, "context", "", "kubeconfig context to use (default: current-context)")

	return cmd
}

// resolveKind determines the (kind, apiVersion) to list. An explicit
// --api-version wins; otherwise the kind must be a known Flux kind.
func resolveKind(kind, apiVersion string) (string, string, error) {
	if apiVersion != "" {
		return kind, apiVersion, nil
	}
	if av, ok := knownKinds[strings.ToLower(kind)]; ok {
		return kind, av, nil
	}
	return "", "", fmt.Errorf("unknown kind %q: pass --api-version, or use one of %s", kind, knownKindNames())
}

func knownKindNames() string {
	names := make([]string, 0, len(knownKinds))
	for k := range knownKinds {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
