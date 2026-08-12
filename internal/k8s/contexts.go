package k8s

import (
	"fmt"
	"sort"

	"k8s.io/client-go/tools/clientcmd"
)

// ContextInfo describes one kube context from the local kubeconfig.
type ContextInfo struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Cluster string `json:"cluster"`
}

// ListContexts reads the standard kubeconfig and returns its contexts sorted by
// name, marking the current-context.
func ListContexts() ([]ContextInfo, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
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
