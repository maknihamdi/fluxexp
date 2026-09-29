package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"
	"github.com/maknihamdi/fluxexp/internal/resolver"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// mixedKustomization is a Kustomization whose inventory interleaves containers
// and leaves, so ordering has something real to reorder. Inventory ids are
// "namespace_name_group_kind"; the core group is empty.
func mixedKustomization(ns, name string) *unstructured.Unstructured {
	ks := kustomization(ns, name)
	_ = unstructured.SetNestedSlice(ks.Object, []interface{}{
		map[string]interface{}{"id": ns + "_values__ConfigMap", "v": "v1"},
		map[string]interface{}{"id": ns + "_web_helm.toolkit.fluxcd.io_HelmRelease", "v": "v2"},
		map[string]interface{}{"id": ns + "_infra__ServiceAccount", "v": "v1"},
		map[string]interface{}{"id": ns + "_api_apps_Deployment", "v": "v1"},
	}, "status", "inventory", "entries")
	return ks
}

func bare(ns, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"namespace": ns, "name": name},
	}}
}

func newMixedService() *Service {
	fc := &fakeCluster{
		objs: map[string]*unstructured.Unstructured{
			"Kustomization|flux|apps":   mixedKustomization("flux", "apps"),
			"GitRepository|flux|flux":   gitRepo("flux", "flux", "main@sha1:abc"),
			"ConfigMap|flux|values":     bare("flux", "values"),
			"HelmRelease|flux|web":      bare("flux", "web"),
			"ServiceAccount|flux|infra": bare("flux", "infra"),
			"Deployment|flux|api":       bare("flux", "api"),
			"Pod|flux|api-1":            bare("flux", "api-1"),
		},
	}
	s := newService(func(string) (cluster, error) { return fc, nil })
	return s
}

// k8sRefFor mirrors resolver.K8sRef for the test's convenience.
func k8sRefFor(apiVersion, kind, ns, name string) engine.Ref {
	return resolver.K8sRef(apiVersion, kind, ns, name)
}

func TestExpand_ChildrenAreOrderedContainersFirst(t *testing.T) {
	s := newMixedService()

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(node.Children) != 4 {
		t.Fatalf("got %d children, want the 4 inventory objects", len(node.Children))
	}
	// The source is a dependency now, so it must not be among the children.
	for _, c := range node.Children {
		if c.Ref.Name == "flux" {
			t.Error("the source must not appear among the children")
		}
	}

	// Tier 1, Flux: the HelmRelease. Tier 2: the Deployment. Tier 3: the leaves,
	// in inventory order.
	want := []string{"web", "api", "values", "infra"}
	for i, name := range want {
		if node.Children[i].Ref.Name != name {
			t.Errorf("child %d = %q, want %q (Flux, then containers, then leaves)", i, node.Children[i].Ref.Name, name)
		}
	}
}

func TestExpand_ExposesExpandableOnNodeAndChildren(t *testing.T) {
	s := newMixedService()

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if !node.Expandable {
		t.Error("the expanded Kustomization must report expandable")
	}

	got := map[string]bool{}
	for _, c := range node.Children {
		got[c.Ref.Name] = c.Expandable
	}
	want := map[string]bool{"web": true, "api": true, "values": false, "infra": false}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("child %q expandable = %v, want %v", name, got[name], w)
		}
	}
}

func TestExpand_PodIsNotExpandable(t *testing.T) {
	s := newMixedService()

	node, err := s.Expand("ctx", k8sRefFor("v1", "Pod", "flux", "api-1"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if node.Expandable {
		t.Error("a Pod is the end of the chain and must not report expandable")
	}
}

func TestHandlerExpand_SerializesExpandable(t *testing.T) {
	srv := httptest.NewServer(Handler(newMixedService()))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/expand?context=ctx&type=kustomize.toolkit.fluxcd.io/v1%7CKustomization&ns=flux&name=apps")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	var node NodeDTO
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !node.Expandable {
		t.Error("node.expandable must survive JSON encoding")
	}
	if len(node.Dependencies) == 0 || node.Dependencies[0].Ref.Name != "flux" {
		t.Fatalf("the source must be serialized as a dependency, got %+v", node.Dependencies)
	}
	// The source is a leaf, so it also proves false survives encoding.
	if node.Dependencies[0].Expandable {
		t.Error("the GitRepository source is a leaf and must serialize expandable=false")
	}
	if len(node.Children) == 0 || !node.Children[0].Expandable {
		t.Error("the first child (HelmRelease) must serialize expandable=true")
	}
}

func TestExpand_DependenciesAreSeparateFromChildren(t *testing.T) {
	s := newMixedService()

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(node.Dependencies) != 1 {
		t.Fatalf("want the source as the single dependency, got %+v", node.Dependencies)
	}
	dep := node.Dependencies[0]
	if dep.Ref.Name != "flux" || dep.Ref.Type != "source.toolkit.fluxcd.io/v1|GitRepository" {
		t.Errorf("dependency = %+v, want the GitRepository source", dep.Ref)
	}
	// Resolved one level deep: it carries its own health and its fields.
	if dep.Health == "" {
		t.Error("a dependency must carry its own health")
	}
	if len(dep.Fields) == 0 {
		t.Error("a GitRepository dependency must carry its fields without a click")
	}
}

func TestChildRows_FluxObjectsCarryFieldsInline(t *testing.T) {
	s := newMixedService()

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	byName := map[string]NodeDTO{}
	for _, c := range node.Children {
		byName[c.Ref.Name] = c
	}
	// A non-Flux child carries no fields.
	if len(byName["values"].Fields) != 0 {
		t.Errorf("a ConfigMap row must carry no fields, got %+v", byName["values"].Fields)
	}
	if len(byName["api"].Fields) != 0 {
		t.Errorf("a Deployment row must carry no fields, got %+v", byName["api"].Fields)
	}
}

func TestChildRows_FluxSourceChildCarriesFields(t *testing.T) {
	// A HelmRepository sitting in the inventory must show its fields on its row.
	fc := &fakeCluster{objs: map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": func() *unstructured.Unstructured {
			ks := kustomization("flux", "apps")
			_ = unstructured.SetNestedSlice(ks.Object, []interface{}{
				map[string]interface{}{"id": "flux_charts_source.toolkit.fluxcd.io_HelmRepository", "v": "v1"},
			}, "status", "inventory", "entries")
			return ks
		}(),
		"GitRepository|flux|flux": gitRepo("flux", "flux", "main@sha1:abc"),
		"HelmRepository|flux|charts": {Object: map[string]interface{}{
			"metadata": map[string]interface{}{"namespace": "flux", "name": "charts"},
			"spec":     map[string]interface{}{"url": "https://charts.jetstack.io", "interval": "1h"},
		}},
	}}
	s := newService(func(string) (cluster, error) { return fc, nil })

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(node.Children) != 1 {
		t.Fatalf("want 1 child, got %d", len(node.Children))
	}
	fields := map[string]string{}
	for _, f := range node.Children[0].Fields {
		fields[f.Label] = f.Value
	}
	if fields["Repo"] != "charts.jetstack.io" || fields["Interval"] != "1h" {
		t.Errorf("HelmRepository row fields = %v, want repo and interval inline", fields)
	}
}

// nestedService builds a Kustomization "parent" whose inventory contains another
// Kustomization "child" (itself with a source and a dependsOn), plus a ConfigMap
// and a HelmRelease that must not be resolved.
func nestedService() *fakeCluster {
	parent := kustomization("flux", "parent")
	_ = unstructured.SetNestedSlice(parent.Object, []interface{}{
		map[string]interface{}{"id": "flux_values__ConfigMap", "v": "v1"},
		map[string]interface{}{"id": "flux_child_kustomize.toolkit.fluxcd.io_Kustomization", "v": "v1"},
		map[string]interface{}{"id": "flux_web_helm.toolkit.fluxcd.io_HelmRelease", "v": "v2"},
	}, "status", "inventory", "entries")

	child := kustomization("flux", "child")
	_ = unstructured.SetNestedMap(child.Object, map[string]interface{}{
		"kind": "GitRepository", "name": "flux",
	}, "spec", "sourceRef")
	_ = unstructured.SetNestedSlice(child.Object, []interface{}{
		map[string]interface{}{"name": "base"},
	}, "spec", "dependsOn")

	return &fakeCluster{objs: map[string]*unstructured.Unstructured{
		"Kustomization|flux|parent": parent,
		"Kustomization|flux|child":  child,
		"Kustomization|flux|base":   kustomization("flux", "base"),
		"GitRepository|flux|flux":   gitRepo("flux", "flux", "main@sha1:abc"),
		"ConfigMap|flux|values":     bare("flux", "values"),
		"HelmRelease|flux|web":      bare("flux", "web"),
	}}
}

func TestListedKustomization_CarriesItsOwnDependencies(t *testing.T) {
	fc := nestedService()
	s := newService(func(string) (cluster, error) { return fc, nil })

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "parent"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	var child *NodeDTO
	for i := range node.Children {
		if node.Children[i].Ref.Name == "child" {
			child = &node.Children[i]
		}
	}
	if child == nil {
		t.Fatalf("child Kustomization missing from %+v", node.Children)
	}
	if len(child.Dependencies) != 2 {
		t.Fatalf("child should carry its source and its dependsOn, got %+v", child.Dependencies)
	}
	if child.Dependencies[0].Ref.Name != "flux" || child.Dependencies[0].Ref.Type != "source.toolkit.fluxcd.io/v1|GitRepository" {
		t.Errorf("first nested dependency = %+v, want the GitRepository source", child.Dependencies[0].Ref)
	}
	if len(child.Dependencies[0].Fields) == 0 {
		t.Error("the nested source must carry its fields without a click")
	}
	if child.Dependencies[1].Ref.Name != "base" {
		t.Errorf("second nested dependency = %q, want the dependsOn entry", child.Dependencies[1].Ref.Name)
	}
	// The entry is not resolved, so no freshness is claimed for it: computing one
	// without its source would risk showing "up-to-date" for something behind.
	if child.Freshness != "" {
		t.Errorf("a listed entry must not claim freshness, got %q", child.Freshness)
	}
}

// TestListedKustomization_InventoryIsNeverRead pins that listing an entry does
// not resolve it: the child's inventory names an object the fake does not hold,
// so reading it would surface as an extra dependency or an error.
func TestListedKustomization_InventoryIsNeverRead(t *testing.T) {
	fc := nestedService()
	child := fc.objs["Kustomization|flux|child"]
	_ = unstructured.SetNestedSlice(child.Object, []interface{}{
		map[string]interface{}{"id": "flux_absent__ConfigMap", "v": "v1"},
	}, "status", "inventory", "entries")

	s := newService(func(string) (cluster, error) { return fc, nil })
	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "parent"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, c := range node.Children {
		if c.Ref.Name != "child" {
			continue
		}
		if c.Health == "error" {
			t.Errorf("the listed entry must not be resolved: %s", c.Err)
		}
		for _, dep := range c.Dependencies {
			if dep.Ref.Name == "absent" {
				t.Error("an inventory entry leaked into the dependency group")
			}
		}
	}
}

func TestListedNonBearingChildrenCarryNoDependencies(t *testing.T) {
	fc := nestedService()
	s := newService(func(string) (cluster, error) { return fc, nil })

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "parent"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, c := range node.Children {
		if c.Ref.Name == "child" {
			continue
		}
		if len(c.Dependencies) != 0 {
			t.Errorf("%s must not carry dependencies", c.Ref.Display)
		}
	}
}

func TestNestedDependenciesStopAtOneLevel(t *testing.T) {
	fc := nestedService()
	// Give the dependsOn target its own source; it must NOT be resolved.
	base := fc.objs["Kustomization|flux|base"]
	_ = unstructured.SetNestedMap(base.Object, map[string]interface{}{
		"kind": "GitRepository", "name": "flux",
	}, "spec", "sourceRef")

	s := newService(func(string) (cluster, error) { return fc, nil })
	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "parent"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for _, c := range node.Children {
		if c.Ref.Name != "child" {
			continue
		}
		for _, dep := range c.Dependencies {
			if len(dep.Dependencies) != 0 {
				t.Errorf("nesting must stop at one level; %s carries %d", dep.Ref.Display, len(dep.Dependencies))
			}
		}
	}
}

// TestExpand_SourceAppearsOnceWhenAlsoApplied reproduces the real bootstrap
// shape: the Kustomization reconciles from a GitRepository and also applies it.
func TestExpand_SourceAppearsOnceWhenAlsoApplied(t *testing.T) {
	ks := kustomization("flux", "apps")
	_ = unstructured.SetNestedSlice(ks.Object, []interface{}{
		map[string]interface{}{"id": "flux_flux_source.toolkit.fluxcd.io_GitRepository", "v": "v1"},
		map[string]interface{}{"id": "flux_values__ConfigMap", "v": "v1"},
	}, "status", "inventory", "entries")

	fc := &fakeCluster{objs: map[string]*unstructured.Unstructured{
		"Kustomization|flux|apps": ks,
		"GitRepository|flux|flux": gitRepo("flux", "flux", "main@sha1:abc"),
		"ConfigMap|flux|values":   bare("flux", "values"),
	}}
	s := newService(func(string) (cluster, error) { return fc, nil })

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "apps"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	if len(node.Dependencies) != 1 || node.Dependencies[0].Ref.Name != "flux" {
		t.Fatalf("the source must be kept as a dependency, got %+v", node.Dependencies)
	}
	for _, c := range node.Children {
		if c.Ref.Type == "source.toolkit.fluxcd.io/v1|GitRepository" {
			t.Error("the source must not also appear among the applied children")
		}
	}
	if len(node.Children) != 1 || node.Children[0].Ref.Name != "values" {
		t.Errorf("the other inventory entries must survive, got %+v", node.Children)
	}
}

// projectService reproduces the reported shape: a parent Kustomization that
// applies both a child Kustomization and the GitRepository that child
// reconciles from, plus an unrelated ExternalSecret.
func projectService() *fakeCluster {
	parent := kustomization("flux", "project")
	_ = unstructured.SetNestedSlice(parent.Object, []interface{}{
		map[string]interface{}{"id": "app_energies_kustomize.toolkit.fluxcd.io_Kustomization", "v": "v1"},
		map[string]interface{}{"id": "app_energies_source.toolkit.fluxcd.io_GitRepository", "v": "v1"},
		map[string]interface{}{"id": "app_token_external-secrets.io_ExternalSecret", "v": "v1"},
	}, "status", "inventory", "entries")

	child := kustomization("app", "energies")
	_ = unstructured.SetNestedMap(child.Object, map[string]interface{}{
		"kind": "GitRepository", "name": "energies",
	}, "spec", "sourceRef")
	// The child applies nothing, to keep the layer focused.
	unstructured.RemoveNestedField(child.Object, "status", "inventory")

	return &fakeCluster{objs: map[string]*unstructured.Unstructured{
		"Kustomization|flux|project": parent,
		"Kustomization|app|energies": child,
		"GitRepository|app|energies": gitRepo("app", "energies", "main@sha1:abc"),
		"GitRepository|flux|flux":    gitRepo("flux", "flux", "main@sha1:abc"),
		"ExternalSecret|app|token":   bare("app", "token"),
	}}
}

func TestExpand_LayerShowsEachReferenceOnce(t *testing.T) {
	s := newService(func(string) (cluster, error) { return projectService(), nil })

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux", "project"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}

	// The GitRepository must appear only nested under the child Kustomization.
	var topLevelGit int
	var nestedGit int
	for _, c := range node.Children {
		if c.Ref.Type == "source.toolkit.fluxcd.io/v1|GitRepository" && c.Ref.Name == "energies" {
			topLevelGit++
		}
		for _, dep := range c.Dependencies {
			if dep.Ref.Name == "energies" && dep.Ref.Type == "source.toolkit.fluxcd.io/v1|GitRepository" {
				nestedGit++
			}
		}
	}
	if nestedGit != 1 {
		t.Errorf("the GitRepository must be grouped under its Kustomization, found %d", nestedGit)
	}
	if topLevelGit != 0 {
		t.Errorf("the GitRepository must not also be a top-level row, found %d", topLevelGit)
	}
	// The unrelated entries survive: the child Kustomization and the secret.
	if len(node.Children) != 2 {
		t.Errorf("want 2 remaining children, got %d: %+v", len(node.Children), node.Children)
	}
}

func TestDropNestedDuplicates_KeepsEntriesCarryingTheirOwnGroup(t *testing.T) {
	ref := func(name string) RefDTO {
		return RefDTO{Domain: "kubernetes", Type: "kustomize.toolkit.fluxcd.io/v1|Kustomization", Namespace: "ns", Name: name}
	}
	a := NodeDTO{Ref: ref("a"), Dependencies: []NodeDTO{{Ref: ref("b")}}}
	b := NodeDTO{Ref: ref("b"), Dependencies: []NodeDTO{{Ref: ref("a")}}}

	got := dropNestedDuplicates([]NodeDTO{a, b}, nil)
	if len(got) != 2 {
		t.Errorf("mutually referencing entries must both survive, got %d", len(got))
	}
}

func TestDropNestedDuplicates_NoOverlapIsUntouched(t *testing.T) {
	mk := func(name string) NodeDTO {
		return NodeDTO{Ref: RefDTO{Domain: "kubernetes", Type: "v1|ConfigMap", Namespace: "ns", Name: name}}
	}
	in := []NodeDTO{mk("a"), mk("b"), mk("c")}
	got := dropNestedDuplicates(in, nil)
	if len(got) != 3 {
		t.Errorf("a layer with no overlap must be untouched, got %d", len(got))
	}
}

// helmReleaseWithSource builds a HelmRelease that pulls its chart from a
// HelmRepository and waits on another release, with a stored release in history
// (so resolving it would read a Helm storage Secret).
func helmReleaseWithSource(ns, name, repo string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"namespace": ns, "name": name},
		"spec": map[string]interface{}{
			"chart": map[string]interface{}{
				"spec": map[string]interface{}{
					"chart":     name,
					"sourceRef": map[string]interface{}{"kind": "HelmRepository", "name": repo},
				},
			},
			"dependsOn": []interface{}{map[string]interface{}{"name": "base"}},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True", "message": "release deployed"},
			},
			"history": []interface{}{
				map[string]interface{}{"name": name, "namespace": ns, "version": int64(3)},
			},
		},
	}}
}

func helmRepo(ns, name, url string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"namespace": ns, "name": name},
		"spec":     map[string]interface{}{"url": url, "interval": "1m"},
		"status": map[string]interface{}{
			"conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}},
		},
	}}
}

// helmService is a Kustomization applying a HelmRelease, plus everything the
// release refers to and the Helm storage Secret a resolve would read.
func helmService() *fakeCluster {
	ks := kustomization("apps", "charts")
	_ = unstructured.SetNestedSlice(ks.Object, []interface{}{
		map[string]interface{}{"id": "apps_web_helm.toolkit.fluxcd.io_HelmRelease", "v": "v2"},
	}, "status", "inventory", "entries")

	return &fakeCluster{objs: map[string]*unstructured.Unstructured{
		"Kustomization|apps|charts":             ks,
		"GitRepository|apps|flux":               gitRepo("apps", "flux", "main@sha1:abc"),
		"HelmRelease|apps|web":                  helmReleaseWithSource("apps", "web", "jetstack"),
		"HelmRepository|apps|jetstack":          helmRepo("apps", "jetstack", "https://charts.jetstack.io"),
		"HelmRelease|apps|base":                 helmReleaseWithSource("apps", "base", "jetstack"),
		"Secret|apps|sh.helm.release.v1.web.v3": bare("apps", "sh.helm.release.v1.web.v3"),
	}}
}

// TestListedHelmRelease_CarriesItsOwnDependencies pins that a HelmRelease is
// grouped with what it needs, exactly as a Kustomization is: its chart source
// first, then what it waits on.
func TestListedHelmRelease_CarriesItsOwnDependencies(t *testing.T) {
	fc := helmService()
	s := newService(func(string) (cluster, error) { return fc, nil })

	node, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "apps", "charts"))
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(node.Children) != 1 {
		t.Fatalf("want the HelmRelease as the only child, got %+v", node.Children)
	}
	hr := node.Children[0]
	if len(hr.Dependencies) != 2 {
		t.Fatalf("the listed HelmRelease should carry its source and its dependsOn, got %+v", hr.Dependencies)
	}
	if hr.Dependencies[0].Ref.Name != "jetstack" || hr.Dependencies[0].Ref.Type != "source.toolkit.fluxcd.io/v1|HelmRepository" {
		t.Errorf("first dependency = %+v, want the HelmRepository source", hr.Dependencies[0].Ref)
	}
	if len(hr.Dependencies[0].Fields) == 0 {
		t.Error("the nested HelmRepository must carry its fields without a click")
	}
	if hr.Dependencies[1].Ref.Name != "base" {
		t.Errorf("second dependency = %q, want the dependsOn entry", hr.Dependencies[1].Ref.Name)
	}
}

// TestListedHelmRelease_StorageSecretIsNeverRead pins the reason the derivation
// works from a fetched object: resolving a HelmRelease reads and gunzips its Helm
// storage Secret, which listing one must never do.
func TestListedHelmRelease_StorageSecretIsNeverRead(t *testing.T) {
	fc := helmService()
	s := newService(func(string) (cluster, error) { return fc, nil })

	if _, err := s.Expand("ctx", k8sRefFor("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "apps", "charts")); err != nil {
		t.Fatalf("expand: %v", err)
	}
	if fc.fetched("Secret|apps|sh.helm.release.v1.web.v3") {
		t.Error("listing a HelmRelease must not read its Helm storage Secret")
	}
}
