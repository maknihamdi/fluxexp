package resolver

import (
	"context"
	"regexp"
	"strings"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fluxSourceGroup and fluxImageGroup are the Flux API groups whose objects carry
// configuration worth surfacing (where the code comes from, what is scanned).
const (
	fluxSourceGroup = "source.toolkit.fluxcd.io"
	fluxImageGroup  = "image.toolkit.fluxcd.io"
)

// FluxObjectResolver handles Flux source and image-automation objects. They are
// leaves — nothing descends from a GitRepository — but they answer the question
// "where does this come from?", so it surfaces their useful fields instead of
// letting them fall to the generic fallback, which shows only a Ready condition.
type FluxObjectResolver struct{}

// Matches claims the Flux source and image-automation groups.
func (FluxObjectResolver) Matches(ref engine.Ref) bool {
	apiVersion, _, _, _, err := DecodeK8sRef(ref)
	if err != nil {
		return false
	}
	group := groupOf(apiVersion)
	return group == fluxSourceGroup || group == fluxImageGroup
}

// Expandable reports false: these objects are the end of the chain.
func (FluxObjectResolver) Expandable(engine.Ref) bool { return false }

// Resolve fetches the object, derives health from its Ready condition, and adds
// the fields that matter for its kind.
func (FluxObjectResolver) Resolve(ctx context.Context, rc *ResolveContext, ref engine.Ref) (engine.Result, error) {
	obj, err := rc.GetK8s(ctx, ref)
	if err != nil {
		return engine.Result{}, err
	}
	_, kind, _, _, _ := DecodeK8sRef(ref)
	health, detail := healthFromReady(obj)

	return engine.Result{
		Health: health,
		Detail: detail,
		Fields: fluxObjectFields(obj, kind),
	}, nil
}

// FieldsForFetched returns the fields computable from an object a caller has
// already retrieved, so a surface holding it for another purpose — the portal's
// child rows, which fetch each object for its health — can render them without a
// second call. Kinds with no fields yield an empty list.
func FieldsForFetched(ref engine.Ref, obj *unstructured.Unstructured) []engine.Field {
	if obj == nil {
		return nil
	}
	_, kind, _, _, err := DecodeK8sRef(ref)
	if err != nil {
		return nil
	}
	if !(FluxObjectResolver{}).Matches(ref) {
		return nil
	}
	return fluxObjectFields(obj, kind)
}

// fluxObjectFields returns the fields worth showing for a Flux source or image
// object. Every field is optional: an absent path yields no field at all, so an
// unconfigured option — or a schema shift between Flux API versions — degrades
// to health-only rather than showing an empty label or failing.
func fluxObjectFields(obj *unstructured.Unstructured, kind string) []engine.Field {
	var fields []engine.Field
	add := func(label, value, full string) {
		if value == "" {
			return
		}
		fields = append(fields, engine.Field{Label: label, Value: value, Full: full})
	}
	spec := func(path ...string) string {
		v, _, _ := unstructured.NestedString(obj.Object, append([]string{"spec"}, path...)...)
		return v
	}

	switch kind {
	case "GitRepository":
		url := spec("url")
		add("Repo", shortURL(url), url)
		label, value := trackedRef(obj)
		add(label, value, "")
		add("Interval", humanizeInterval(spec("interval")), "")

	case "OCIRepository":
		url := spec("url")
		add("Repo", shortURL(url), url)
		label, value := trackedRef(obj)
		add(label, value, "")
		add("Interval", humanizeInterval(spec("interval")), "")

	case "Bucket":
		add("Bucket", spec("bucketName"), "")
		add("Endpoint", spec("endpoint"), "")
		add("Interval", humanizeInterval(spec("interval")), "")

	case "HelmRepository":
		url := spec("url")
		add("Repo", shortURL(url), url)
		add("Type", spec("type"), "")
		add("Interval", humanizeInterval(spec("interval")), "")

	case "ImageRepository":
		image := spec("image")
		add("Image", shortURL(image), image)
		add("Interval", humanizeInterval(spec("interval")), "")
		scan, _, _ := unstructured.NestedString(obj.Object, "status", "lastScanResult", "scanTime")
		add("Last scan", humanizeSince(scan), scan)

	case "ImagePolicy":
		add("Policy", policyRule(obj), "")
		tag, _, _ := unstructured.NestedString(obj.Object, "status", "latestRef", "tag")
		if tag == "" {
			// Older Flux versions expose the full image reference instead.
			latest, _, _ := unstructured.NestedString(obj.Object, "status", "latestImage")
			if _, after, found := strings.Cut(latest, ":"); found {
				tag = after
			}
		}
		add("Selected", tag, "")
	}
	return fields
}

// trackedRef returns the label and value of whichever git/OCI reference the
// object tracks. Flux allows exactly one of these to be set.
func trackedRef(obj *unstructured.Unstructured) (label, value string) {
	for _, candidate := range []struct{ field, label string }{
		{"branch", "Branch"},
		{"tag", "Tag"},
		{"semver", "Semver"},
		{"digest", "Digest"},
		{"commit", "Commit"},
	} {
		if v, _, _ := unstructured.NestedString(obj.Object, "spec", "ref", candidate.field); v != "" {
			return candidate.label, v
		}
	}
	return "Ref", ""
}

// policyRule renders an ImagePolicy's rule, e.g. "semver >=1.0.0" or
// "alphabetical asc". Returns "" when the policy is absent or unrecognized.
func policyRule(obj *unstructured.Unstructured) string {
	policy, ok, _ := unstructured.NestedMap(obj.Object, "spec", "policy")
	if !ok {
		return ""
	}
	for _, kind := range []struct{ name, field string }{
		{"semver", "range"},
		{"alphabetical", "order"},
		{"numerical", "order"},
	} {
		inner, ok := policy[kind.name].(map[string]interface{})
		if !ok {
			continue
		}
		if v, _ := inner[kind.field].(string); v != "" {
			return kind.name + " " + v
		}
		return kind.name
	}
	return ""
}

// shortURL trims the scheme and a trailing ".git" so a repository reads as
// "host/org/repo". The complete value is kept as the field's Full.
func shortURL(url string) string {
	if url == "" {
		return ""
	}
	short := url
	for _, scheme := range []string{"https://", "http://", "ssh://", "oci://"} {
		short = strings.TrimPrefix(short, scheme)
	}
	if _, after, found := strings.Cut(short, "@"); found {
		short = after // ssh form: git@host/org/repo
	}
	return strings.TrimSuffix(short, ".git")
}

// zeroTail matches a trailing zero-valued duration component that is preceded by
// a larger one — the "0s" of "1m0s", the "0m" of "1h0m". Anchoring on the
// preceding unit is what keeps "10m" intact: a plain TrimSuffix("0m") would
// chew its way into the digits and leave "1".
var zeroTail = regexp.MustCompile(`([hm])0[ms]$`)

// humanizeInterval drops the zero-valued tails Go duration strings carry, so
// "1m0s" reads as "1m" and "1h0m0s" as "1h", while "10m" and "30s" pass through
// untouched.
func humanizeInterval(interval string) string {
	for {
		trimmed := zeroTail.ReplaceAllString(interval, "$1")
		if trimmed == interval {
			return interval
		}
		interval = trimmed
	}
}
