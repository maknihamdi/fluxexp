package k8s

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/rest"
)

const kubeconfigFixture = `
apiVersion: v1
kind: Config
current-context: dev
contexts:
- name: dev
  context: {cluster: dev-cluster, user: dev}
- name: prod
  context: {cluster: prod-cluster, user: prod}
clusters:
- name: dev-cluster
  cluster: {server: https://dev.example}
- name: prod-cluster
  cluster: {server: https://prod.example}
users:
- name: dev
  user: {}
- name: prod
  user: {}
`

func TestListContexts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(kubeconfigFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)

	ctxs, err := ListContexts()
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if len(ctxs) != 2 {
		t.Fatalf("want 2 contexts, got %d", len(ctxs))
	}
	// Sorted by name: dev, prod.
	if ctxs[0].Name != "dev" || !ctxs[0].Current {
		t.Fatalf("dev should be first and current: %+v", ctxs[0])
	}
	if ctxs[1].Name != "prod" || ctxs[1].Current {
		t.Fatalf("prod should be second and not current: %+v", ctxs[1])
	}
	if ctxs[0].Cluster != "dev-cluster" {
		t.Fatalf("dev cluster = %q", ctxs[0].Cluster)
	}
}

// stubInCluster replaces the in-cluster probe for one test. Running in a pod is
// the one thing a unit test cannot be, so the detection is injected.
func stubInCluster(t *testing.T, cfg *rest.Config, err error) {
	t.Helper()
	prev := inClusterConfig
	inClusterConfig = func() (*rest.Config, error) { return cfg, err }
	t.Cleanup(func() { inClusterConfig = prev })
}

// A kubeconfig with contexts must behave exactly as before, whether or not the
// process also happens to have in-cluster credentials. The in-cluster branch is
// a fallback for having nothing to name, not a mode that can pre-empt a real
// kubeconfig.
func TestListContexts_KubeconfigWinsOverInCluster(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(kubeconfigFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	stubInCluster(t, &rest.Config{Host: "https://in.cluster"}, nil)

	ctxs, err := ListContexts()
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if len(ctxs) != 2 || ctxs[0].Name != "dev" || ctxs[1].Name != "prod" {
		t.Fatalf("kubeconfig contexts should be unchanged, got %+v", ctxs)
	}
}

func TestListContexts_InCluster(t *testing.T) {
	// An empty KUBECONFIG path yields no contexts and, deliberately, no error:
	// clientcmd does not treat a missing kubeconfig as a failure.
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "absent"))
	stubInCluster(t, &rest.Config{Host: "https://kubernetes.default.svc"}, nil)

	ctxs, err := ListContexts()
	if err != nil {
		t.Fatalf("ListContexts: %v", err)
	}
	if len(ctxs) != 1 {
		t.Fatalf("want exactly one context, got %d: %+v", len(ctxs), ctxs)
	}
	got := ctxs[0]
	if got.Name != InClusterContext {
		t.Fatalf("name = %q, want %q", got.Name, InClusterContext)
	}
	if !got.Current {
		t.Fatal("the only context must be marked current, or the UI selects nothing")
	}
	if got.Cluster != "https://kubernetes.default.svc" {
		t.Fatalf("cluster = %q, want the API server host", got.Cluster)
	}
}

// Neither a kubeconfig nor in-cluster credentials must be reported, not rendered
// as an empty selector the reader cannot interpret.
func TestListContexts_NoCredentialsAtAll(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "absent"))
	stubInCluster(t, nil, rest.ErrNotInCluster)

	ctxs, err := ListContexts()
	if err == nil {
		t.Fatalf("want an error, got %d contexts", len(ctxs))
	}
	if !errors.Is(err, rest.ErrNotInCluster) {
		t.Fatalf("error should wrap the in-cluster failure, got %v", err)
	}
}
