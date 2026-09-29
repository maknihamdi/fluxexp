package resolver

import (
	"strings"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestK8sHealth_InventoryFamilies covers one case per family measured across a
// real cluster's Kustomization inventories. Families A, B and D used to report
// unknown wholesale — 172 of 255 entries — which is what this change exists to
// fix; family C was already right and must stay right.
func TestK8sHealth_InventoryFamilies(t *testing.T) {
	cases := []struct {
		name string
		obj  *unstructured.Unstructured
		want engine.Health
	}{
		// Family A — no status at all. Existing is the whole of what these can do.
		{"A: ConfigMap", statusObj("v1", "ConfigMap"), engine.Healthy},
		{"A: ClusterRole", statusObj("rbac.authorization.k8s.io/v1", "ClusterRole"), engine.Healthy},
		{"A: ServiceAccount", statusObj("v1", "ServiceAccount"), engine.Healthy},

		// Family B — conditions, but no Ready. kstatus alone reports every one
		// of these as current; the polarity table is what makes them readable.
		{"B: Bundle Synced=True", statusObj("trust.cert-manager.io/v1alpha1", "Bundle",
			condition("Synced", "True", "synced to 3 namespaces")), engine.Healthy},
		{"B: Bundle Synced=False", statusObj("trust.cert-manager.io/v1alpha1", "Bundle",
			condition("Synced", "False", "vault unreachable")), engine.Unhealthy},
		{"B: KubernetesRole Configured=False", statusObj("auth.toolkit.vault.hopopops.com/v1", "KubernetesRole",
			condition("Configured", "False", "denied")), engine.Unhealthy},
		{"B: ComputeClass Health=True", statusObj("cloud.google.com/v1", "ComputeClass",
			condition("Health", "True", "")), engine.Healthy},

		// Family C — Ready present. Already correct before this change.
		{"C: Certificate Ready=True", statusObj("cert-manager.io/v1", "Certificate",
			condition("Ready", "True", "certificate issued")), engine.Healthy},
		{"C: Certificate Ready=False", statusObj("cert-manager.io/v1", "Certificate",
			condition("Ready", "False", "issuance failed")), engine.Unhealthy},
		{"C: ExternalSecret Ready=True", statusObj("external-secrets.io/v1", "ExternalSecret",
			condition("Ready", "True", "")), engine.Healthy},

		// Family D — a status, but no conditions to read in it.
		{"D: Namespace", statusObj("v1", "Namespace"), engine.Healthy},
		{"D: operator CR with an opaque status", statusObj("nifi.konpyutaika.com/v1", "NifiCluster"), engine.Healthy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := K8sHealth(tc.obj)
			if got != tc.want {
				t.Fatalf("health = %q, want %q", got, tc.want)
			}
			if got == engine.Unknown {
				t.Fatal("no object may be left without a status")
			}
		})
	}
}

// TestK8sHealth_ConditionPolarity pins the layer that keeps kstatus's generic
// "current" from painting a broken object green. A condition's sign cannot be
// read off its status: Degraded=False is good news, Synced=False is bad news.
func TestK8sHealth_ConditionPolarity(t *testing.T) {
	cases := []struct {
		name       string
		obj        *unstructured.Unstructured
		want       engine.Health
		wantDetail string
	}{
		{
			name:       "positive condition false is unhealthy",
			obj:        statusObj("example.io/v1", "Thing", condition("Synced", "False", "vault unreachable")),
			want:       engine.Unhealthy,
			wantDetail: "vault unreachable",
		},
		{
			name:       "negative condition false is good news",
			obj:        statusObj("example.io/v1", "Thing", condition("Degraded", "False", "no degradation")),
			want:       engine.Healthy,
			wantDetail: "no degradation",
		},
		{
			name: "negative condition true is unhealthy",
			obj:  statusObj("example.io/v1", "Thing", condition("Degraded", "True", "half the shards are down")),
			want: engine.Unhealthy,
		},
		{
			name: "a violated condition outranks a satisfied one",
			obj: statusObj("example.io/v1", "Thing",
				condition("Available", "True", "serving"),
				condition("Synced", "False", "drifted")),
			want:       engine.Unhealthy,
			wantDetail: "drifted",
		},
		{
			name: "an unknown condition type alone leaves the verdict alone",
			obj:  statusObj("example.io/v1", "Thing", condition("Frobnicated", "False", "who knows")),
			want: engine.Healthy,
		},
		{
			name: "a condition with no message still decides",
			obj:  statusObj("example.io/v1", "Thing", condition("Configured", "False", "")),
			want: engine.Unhealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, detail := K8sHealth(tc.obj)
			if got != tc.want {
				t.Fatalf("health = %q, want %q (detail %q)", got, tc.want, detail)
			}
			if tc.wantDetail != "" && detail != tc.wantDetail {
				t.Fatalf("detail = %q, want %q", detail, tc.wantDetail)
			}
		})
	}
}

// TestK8sHealth_PolarityDoesNotOverrideKindSpecificVerdict guards the boundary:
// where kstatus has a rule for the kind, its verdict stands. A CRD publishes
// Established, which is in the polarity table, but crdConditions already read
// it — the table must not get a second, possibly contradictory, say.
func TestK8sHealth_PolarityDoesNotOverrideKindSpecificVerdict(t *testing.T) {
	crd := statusObj("apiextensions.k8s.io/v1", "CustomResourceDefinition",
		condition("NamesAccepted", "True", ""),
		condition("Established", "True", "the initial names have been accepted"))

	got, detail := K8sHealth(crd)
	if got != engine.Healthy {
		t.Fatalf("health = %q, want healthy (detail %q)", got, detail)
	}
	if detail != "CRD is established" {
		t.Fatalf("detail = %q, want kstatus's own CRD message", detail)
	}
}

// TestK8sHealth_GenerationDrift covers the status that used to have no home:
// the object is neither working nor broken, its controller simply has not
// looked at the current spec yet.
func TestK8sHealth_GenerationDrift(t *testing.T) {
	t.Run("generation ahead of observed is pending", func(t *testing.T) {
		obj := withGenerations(statusObj("example.io/v1", "Thing",
			condition("Ready", "False", "not yet")), 4, 1)

		got, detail := K8sHealth(obj)
		if got != engine.Pending {
			t.Fatalf("health = %q, want pending", got)
		}
		if !strings.Contains(detail, "4") || !strings.Contains(detail, "1") {
			t.Fatalf("detail = %q, want both generations named", detail)
		}
	})

	t.Run("an observed spec change reads its conditions", func(t *testing.T) {
		obj := withGenerations(statusObj("example.io/v1", "Thing",
			condition("Ready", "False", "boom")), 4, 4)

		got, detail := K8sHealth(obj)
		if got != engine.Unhealthy {
			t.Fatalf("health = %q, want unhealthy once the spec is observed", got)
		}
		if detail != "boom" {
			t.Fatalf("detail = %q, want the condition message", detail)
		}
	})

	t.Run("no observedGeneration means no drift", func(t *testing.T) {
		obj := statusObj("example.io/v1", "Thing", condition("Ready", "True", "ok"))
		_ = unstructured.SetNestedField(obj.Object, int64(7), "metadata", "generation")

		if got, _ := K8sHealth(obj); got != engine.Healthy {
			t.Fatalf("health = %q, want healthy", got)
		}
	})

	t.Run("reconciling is pending", func(t *testing.T) {
		obj := statusObj("example.io/v1", "Thing", condition("Reconciling", "True", "working on it"))

		if got, _ := K8sHealth(obj); got != engine.Pending {
			t.Fatalf("health = %q, want pending", got)
		}
	})

	t.Run("stalled is unhealthy", func(t *testing.T) {
		obj := statusObj("example.io/v1", "Thing", condition("Stalled", "True", "stuck"))

		got, detail := K8sHealth(obj)
		if got != engine.Unhealthy {
			t.Fatalf("health = %q, want unhealthy", got)
		}
		if detail != "stuck" {
			t.Fatalf("detail = %q, want the condition message", detail)
		}
	})
}

// TestK8sHealth_UsesWorkloadRules pins the agreement between the two paths that
// can ask about the same object: the CLI resolves a Pod through
// WorkloadResolver, a UI layer merely lists it and derives health from the
// fetched object. Both go through K8sHealth, so both must get the workload
// rules — a Pod that reads pending in one and unhealthy in the other is the
// kind of drift this single entry point exists to prevent.
func TestK8sHealth_UsesWorkloadRules(t *testing.T) {
	cases := []struct {
		name string
		obj  *unstructured.Unstructured
		want engine.Health
	}{
		{"pending pod", podObj("Pending", "False"), engine.Pending},
		{"running unready pod", podObj("Running", "False"), engine.Unhealthy},
		{"under-ready deployment", replicaObj(i64(3), 1), engine.Unhealthy},
		{"running job", statusObj("batch/v1", "Job"), engine.Pending},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shared, sharedDetail := K8sHealth(tc.obj)
			if shared != tc.want {
				t.Fatalf("K8sHealth = %q, want %q", shared, tc.want)
			}
			direct, directDetail, ok := workloadHealth(tc.obj)
			if !ok {
				t.Fatal("workloadHealth should claim this object")
			}
			if direct != shared || directDetail != sharedDetail {
				t.Fatalf("paths disagree: shared %q/%q, workload %q/%q",
					shared, sharedDetail, direct, directDetail)
			}
		})
	}

	// A kind the workload rules do not claim keeps the generic derivation.
	if _, _, ok := workloadHealth(statusObj("v1", "ConfigMap")); ok {
		t.Fatal("workloadHealth must not claim a ConfigMap")
	}
}

// TestK8sHealth_ReadyFalseIsUnhealthy pins the one place fluxexp knowingly
// disagrees with flux: kstatus calls Ready=False in-progress, we call it broken.
// Without this, every failing Certificate in a cluster turns amber.
func TestK8sHealth_ReadyFalseIsUnhealthy(t *testing.T) {
	obj := statusObj("cert-manager.io/v1", "Certificate", condition("Ready", "False", "issuance failed"))

	got, detail := K8sHealth(obj)
	if got != engine.Unhealthy {
		t.Fatalf("health = %q, want unhealthy", got)
	}
	if detail != "issuance failed" {
		t.Fatalf("detail = %q, want the condition message", detail)
	}
}
