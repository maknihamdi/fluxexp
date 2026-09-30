package main

import "testing"

// The precedence is the whole of resolveVersion: an injected value must win over
// whatever the toolchain recorded, and the chain must never end up empty — a
// binary that cannot name itself is the failure this exists to prevent.
func TestResolveVersion(t *testing.T) {
	t.Run("the injected value wins", func(t *testing.T) {
		t.Cleanup(func() { version = "" })
		version = "9.9.9"

		if got := resolveVersion(); got != "9.9.9" {
			t.Fatalf("resolveVersion() = %q, want 9.9.9", got)
		}
	})

	t.Run("without injection it still names something", func(t *testing.T) {
		t.Cleanup(func() { version = "" })
		version = ""

		if got := resolveVersion(); got == "" {
			t.Fatal("resolveVersion() = \"\", want the build-info version or the placeholder")
		}
	})
}
