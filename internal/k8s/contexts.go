package k8s

import (
	"fmt"
	"sort"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// InClusterContext is the name reported for the cluster the process runs in,
// when there is no kubeconfig to name contexts from. It is a sentinel: the UI
// maps it back to the empty context, which is what clientcmd resolves
// in-cluster. It cannot be passed through as a context override — a name absent
// from the kubeconfig fails validation with "context was not found", which is
// not an empty-config error and so never reaches the in-cluster fallback.
const InClusterContext = "in-cluster"

// inClusterConfig is a variable so both branches of ListContexts are testable
// without a cluster, the same reason freshness.go makes `now` overridable.
var inClusterConfig = rest.InClusterConfig

// ContextInfo describes one kube context from the local kubeconfig.
type ContextInfo struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Cluster string `json:"cluster"`
}

// ListContexts reads the standard kubeconfig and returns its contexts sorted by
// name, marking the current-context.
//
// A missing kubeconfig is not an error to clientcmd — it returns an empty config
// — so with no contexts and in-cluster credentials available this reports the one
// cluster the process runs in. An empty list would be a dead end that says
// nothing about why; a single named context says which mode this is.
func ListContexts() ([]ContextInfo, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	if len(cfg.Contexts) == 0 {
		icc, iccErr := inClusterConfig()
		if iccErr != nil {
			return nil, fmt.Errorf("no kubeconfig context found and not running in a cluster: %w", iccErr)
		}
		return []ContextInfo{{
			Name:    InClusterContext,
			Current: true,
			Cluster: icc.Host,
		}}, nil
	}

	out := make([]ContextInfo, 0, len(cfg.Contexts))
	for name, c := range cfg.Contexts {
		out = append(out, ContextInfo{
			Name:    name,
			Current: name == cfg.CurrentContext,
			Cluster: c.Cluster,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
