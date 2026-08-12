// Command fluxexp traverses the dependency graph of resources reconciled by
// FluxCD (and, in future increments, whatever they ultimately produce) and
// prints the resolved resource tree with per-node health.
package main

import (
	"fmt"
	"os"

	"github.com/maknihamdi/fluxexp/cmd/fluxexp/command"
)

func main() {
	if err := command.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
