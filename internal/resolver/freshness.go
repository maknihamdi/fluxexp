package resolver

import (
	"strings"
	"time"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// now is overridable in tests for deterministic relative times.
var now = time.Now

// shortRevision renders a Flux revision (`<ref>@sha1:<sha>`, `<ref>@<sha>`, or
// `<ref>/<sha>`) as `<ref>@<sha[:7]>`, keeping it readable. Unrecognized formats
// are returned unchanged.
func shortRevision(rev string) string {
	if rev == "" {
		return ""
	}
	ref, sha := rev, ""
	if i := strings.LastIndex(rev, "@"); i >= 0 {
		ref, sha = rev[:i], rev[i+1:]
	} else if i := strings.LastIndex(rev, "/"); i >= 0 {
		ref, sha = rev[:i], rev[i+1:]
	} else {
		return rev
	}
	sha = strings.TrimPrefix(sha, "sha1:")
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return ref + "@" + sha
}

// humanizeSince renders an RFC3339 timestamp as a relative "<n> ago" string.
// Returns "" for an unparseable/empty timestamp.
func humanizeSince(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	d := now().Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + "h ago"
	default:
		return itoa(int(d.Hours()/24)) + "d ago"
	}
}

func itoa(n int) string {
	if n < 0 {
		n = 0
	}
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// KustomizationFreshness computes a Kustomization's freshness from cluster state
// (applied vs source-fetched revision) and the structured fields to surface. The
// source object may be nil (fetch failed): freshness then degrades to a
// health-only value. It never errors.
func KustomizationFreshness(ks, source *unstructured.Unstructured) (engine.Freshness, []engine.Field) {
	applied, _, _ := unstructured.NestedString(ks.Object, "status", "lastAppliedRevision")
	attempted, _, _ := unstructured.NestedString(ks.Object, "status", "lastAttemptedRevision")
	readyStatus, readyMsg, _ := readyCondition(ks)
	syncedAt := readyTransitionTime(ks)
	suspend, _, _ := unstructured.NestedBool(ks.Object, "spec", "suspend")

	var sourceRev, sourceFetched string
	if source != nil {
		sourceRev, _, _ = unstructured.NestedString(source.Object, "status", "artifact", "revision")
		sourceFetched, _, _ = unstructured.NestedString(source.Object, "status", "artifact", "lastUpdateTime")
	}

	fields := []engine.Field{}
	if applied != "" {
		fields = append(fields, engine.Field{Label: "Applied", Value: shortRevision(applied), Full: applied})
	}
	if syncedAt != "" {
		fields = append(fields, engine.Field{Label: "Synced", Value: humanizeSince(syncedAt), Full: syncedAt})
	}

	switch {
	case suspend:
		return engine.Suspended, fields

	case readyStatus == "False":
		if attempted != "" {
			fields = append(fields, engine.Field{Label: "Attempted", Value: shortRevision(attempted), Full: attempted})
		}
		if readyMsg != "" {
			fields = append(fields, engine.Field{Label: "Error", Value: readyMsg})
		}
		return engine.Failed, fields

	case source == nil || sourceRev == "":
		// No source revision to compare against: fall back to health.
		if readyStatus == "True" {
			return engine.UpToDate, fields
		}
		return "", fields

	case applied == sourceRev:
		return engine.UpToDate, fields

	default:
		src := shortRevision(sourceRev)
		if f := humanizeSince(sourceFetched); f != "" {
			src += " · fetched " + f
		}
		fields = append(fields, engine.Field{Label: "Source", Value: src, Full: sourceRev})
		return engine.Behind, fields
	}
}

// readyTransitionTime returns the lastTransitionTime of the object's Ready
// condition, if present.
func readyTransitionTime(obj *unstructured.Unstructured) string {
	conds, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return ""
	}
	for _, c := range conds {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := cond["type"].(string); t == "Ready" {
			v, _ := cond["lastTransitionTime"].(string)
			return v
		}
	}
	return ""
}
