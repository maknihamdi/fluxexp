package k8s

import (
	"os"
	"path/filepath"
	"testing"
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
