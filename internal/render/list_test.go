package render

import (
	"strings"
	"testing"

	"github.com/maknihamdi/fluxexp/internal/engine"
)

func TestList_RendersRowsWithHealth(t *testing.T) {
	out := List([]ResourceStatus{
		{Namespace: "flux-system", Name: "apps", Health: engine.Healthy},
		{Namespace: "team-a", Name: "web", Health: engine.Unhealthy, Detail: "source not found"},
	})
	for _, want := range []string{
		"flux-system/apps", "[healthy]",
		"team-a/web", "[unhealthy]", "source not found",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestList_EmptyIsReported(t *testing.T) {
	if got := List(nil); !strings.Contains(got, "no resources") {
		t.Fatalf("empty list should say so, got %q", got)
	}
}
