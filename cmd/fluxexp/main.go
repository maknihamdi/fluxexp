// Command fluxexp traverses the dependency graph of resources reconciled by
// FluxCD (and, in future increments, whatever they ultimately produce) and
// prints the resolved resource tree with per-node health.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/maknihamdi/fluxexp/cmd/fluxexp/command"
)

// version is injected at build time with -ldflags "-X main.version=…", which is
// how the released archives and the container image are built.
var version string

func main() {
	if err := command.NewRootCmd(resolveVersion()).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// resolveVersion answers what this binary is. The injected value comes first;
// failing that, the module version the toolchain recorded, which is what
// `go install <module>@v1.2.3` leaves behind and the only version such a binary
// has. A build from a working copy has no version to name, hence the placeholder.
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}
