package resolver

import (
	"testing"
	"time"

	"github.com/maknihamdi/fluxexp/internal/engine"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestShortRevision(t *testing.T) {
	cases := map[string]string{
		"main@sha1:0123456789abcdef": "main@0123456",
		"alloy@sha1:f799f031c9d2":    "alloy@f799f03",
		"main/0123456789abcdef":      "main@0123456",
		"main@0123456789":            "main@0123456",
		"weirdformat":                "weirdformat",
		"":                           "",
	}
	for in, want := range cases {
		if got := shortRevision(in); got != want {
			t.Fatalf("shortRevision(%q) = %q, want %q", in, got, want)
		}
	}
}

func ksObj(applied, readyStatus, msg, attempted string, suspend bool) *unstructured.Unstructured {
	status := map[string]interface{}{
		"lastAppliedRevision":   applied,
		"lastAttemptedRevision": attempted,
		"conditions": []interface{}{map[string]interface{}{
			"type": "Ready", "status": readyStatus, "message": msg,
			"lastTransitionTime": "2026-08-13T10:00:00Z",
		}},
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"spec":   map[string]interface{}{"suspend": suspend},
		"status": status,
	}}
}

func srcObj(revision string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{"artifact": map[string]interface{}{
			"revision": revision, "lastUpdateTime": "2026-08-13T09:00:00Z",
		}},
	}}
}

func field(fields []engine.Field, label string) (engine.Field, bool) {
	for _, f := range fields {
		if f.Label == label {
			return f, true
		}
	}
	return engine.Field{}, false
}

func TestKustomizationFreshness(t *testing.T) {
	now = func() time.Time { return time.Date(2026, 8, 13, 10, 5, 0, 0, time.UTC) }
	defer func() { now = time.Now }()

	rev := "main@sha1:0123456789abcdef"

	t.Run("up-to-date", func(t *testing.T) {
		f, fields := KustomizationFreshness(ksObj(rev, "True", "ok", rev, false), srcObj(rev))
		if f != engine.UpToDate {
			t.Fatalf("got %q", f)
		}
		if a, ok := field(fields, "Applied"); !ok || a.Value != "main@0123456" || a.Full != rev {
			t.Fatalf("applied field wrong: %+v", a)
		}
	})

	t.Run("behind", func(t *testing.T) {
		src := "main@sha1:fedcba9876543210"
		f, fields := KustomizationFreshness(ksObj(rev, "True", "ok", rev, false), srcObj(src))
		if f != engine.Behind {
			t.Fatalf("got %q", f)
		}
		if s, ok := field(fields, "Source"); !ok || s.Full != src {
			t.Fatalf("expected a Source field with the newer revision, got %+v", s)
		}
	})

	t.Run("failed", func(t *testing.T) {
		f, fields := KustomizationFreshness(ksObj(rev, "False", "boom", "main@sha1:deadbeef0000", false), srcObj(rev))
		if f != engine.Failed {
			t.Fatalf("got %q", f)
		}
		if _, ok := field(fields, "Attempted"); !ok {
			t.Fatal("expected Attempted field")
		}
		if e, ok := field(fields, "Error"); !ok || e.Value != "boom" {
			t.Fatalf("expected Error field, got %+v", e)
		}
	})

	t.Run("suspended", func(t *testing.T) {
		f, _ := KustomizationFreshness(ksObj(rev, "True", "ok", rev, true), srcObj(rev))
		if f != engine.Suspended {
			t.Fatalf("got %q", f)
		}
	})

	t.Run("nil source falls back to health", func(t *testing.T) {
		if f, _ := KustomizationFreshness(ksObj(rev, "True", "ok", rev, false), nil); f != engine.UpToDate {
			t.Fatalf("ready + no source should be up-to-date, got %q", f)
		}
	})
}
