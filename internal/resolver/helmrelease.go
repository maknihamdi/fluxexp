package resolver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

// helmReleaseGroup is the Flux API group for HelmRelease objects.
const helmReleaseGroup = "helm.toolkit.fluxcd.io"

// HelmReleaseResolver expands a Flux HelmRelease into the objects its chart
// deployed, discovered from the Helm release storage Secret, and reports the
// HelmRelease's health.
type HelmReleaseResolver struct{}

// Matches claims kubernetes references of kind HelmRelease in the Flux group.
func (HelmReleaseResolver) Matches(ref engine.Ref) bool {
	apiVersion, kind, _, _, err := DecodeK8sRef(ref)
	if err != nil || kind != "HelmRelease" {
		return false
	}
	group, _, _ := strings.Cut(apiVersion, "/")
	return group == helmReleaseGroup
}

// Resolve fetches the HelmRelease, locates and decodes its Helm release storage
// Secret, and turns the rendered manifest into child references.
func (HelmReleaseResolver) Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error) {
	obj, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return engine.Result{}, err
	}

	health, detail := K8sHealth(obj)

	name, namespace, version, ok := newestHistory(obj)
	if !ok {
		// Release not stored yet: report health, no children.
		return engine.Result{Health: health, Detail: detail}, nil
	}

	secretName := fmt.Sprintf("sh.helm.release.v1.%s.v%d", name, version)
	secret, err := rc.GetK8s(ctx, K8sRef("v1", "Secret", namespace, secretName))
	if err != nil {
		return engine.Result{}, fmt.Errorf("reading helm storage secret %s/%s: %w", namespace, secretName, err)
	}

	encoded, _, _ := unstructured.NestedString(secret.Object, "data", "release")
	if encoded == "" {
		return engine.Result{}, fmt.Errorf("helm storage secret %s/%s has no release data", namespace, secretName)
	}

	rel, err := decodeHelmRelease(encoded)
	if err != nil {
		return engine.Result{}, fmt.Errorf("decoding helm release %s: %w", name, err)
	}

	children, err := manifestChildren(rel.Manifest, rel.Namespace, rc.K8s.Namespaced)
	if err != nil {
		return engine.Result{}, fmt.Errorf("parsing helm manifest for %s: %w", name, err)
	}

	if rel.Info.Status != "" {
		detail = "release " + rel.Info.Status
	}
	return engine.Result{Health: health, Detail: detail, Children: children}, nil
}

// helmRelease is the minimal shape we need from the decoded release JSON.
type helmRelease struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   int    `json:"version"`
	Manifest  string `json:"manifest"`
	Info      struct {
		Status string `json:"status"`
	} `json:"info"`
}

// newestHistory returns the release name, storage namespace and version of the
// newest entry in the HelmRelease's .status.history (the highest version).
func newestHistory(obj *unstructured.Unstructured) (name, namespace string, version int, ok bool) {
	entries, found, err := unstructured.NestedSlice(obj.Object, "status", "history")
	if err != nil || !found {
		return "", "", 0, false
	}
	best := -1
	for _, e := range entries {
		entry, isMap := e.(map[string]interface{})
		if !isMap {
			continue
		}
		v := toInt(entry["version"])
		if v > best {
			best = v
			name, _ = entry["name"].(string)
			namespace, _ = entry["namespace"].(string)
			version = v
			ok = true
		}
	}
	return name, namespace, version, ok && name != "" && namespace != ""
}

// toInt coerces an unstructured numeric value (int64 or float64) to int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case float64:
		return int(n)
	case int:
		return n
	default:
		return -1
	}
}

// decodeHelmRelease decodes the Helm Secrets-driver payload: Kubernetes base64,
// then Helm base64, then gzip, then JSON.
func decodeHelmRelease(k8sBase64 string) (*helmRelease, error) {
	helmB64, err := base64.StdEncoding.DecodeString(k8sBase64)
	if err != nil {
		return nil, fmt.Errorf("kubernetes base64: %w", err)
	}
	gzipped, err := base64.StdEncoding.DecodeString(string(helmB64))
	if err != nil {
		return nil, fmt.Errorf("helm base64: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(gzipped))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	var rel helmRelease
	if err := json.Unmarshal(raw, &rel); err != nil {
		return nil, fmt.Errorf("release json: %w", err)
	}
	return &rel, nil
}

// manifestChildren parses the release manifest (a multi-document YAML stream)
// into child references. A namespaced object with no explicit namespace inherits
// releaseNamespace; a cluster-scoped object keeps an empty namespace. Scope is
// resolved via namespaced(); when it cannot be determined, the object is treated
// as namespaced (defaults to releaseNamespace). Empty documents are skipped.
func manifestChildren(manifest, releaseNamespace string, namespaced func(apiVersion, kind string) (bool, error)) ([]engine.Ref, error) {
	dec := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	var refs []engine.Ref
	for {
		var doc map[string]interface{}
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		apiVersion, _ := doc["apiVersion"].(string)
		kind, _ := doc["kind"].(string)
		if apiVersion == "" || kind == "" {
			continue
		}
		meta, _ := doc["metadata"].(map[string]interface{})
		name, _ := meta["name"].(string)
		namespace, _ := meta["namespace"].(string)
		if namespace == "" {
			// Only namespaced kinds inherit the release namespace; cluster-scoped
			// kinds stay namespace-less. If scope is unknown, assume namespaced.
			if isNs, err := namespaced(apiVersion, kind); err != nil || isNs {
				namespace = releaseNamespace
			}
		}
		refs = append(refs, K8sRef(apiVersion, kind, namespace, name))
	}
	return refs, nil
}
