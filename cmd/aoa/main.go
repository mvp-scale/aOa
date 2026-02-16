// aOa is a semantic code search engine.
// Single binary, zero config — fast symbol lookup, domain-aware results.
package main

import (
	"os"

	"github.com/corey/aoa/cmd/aoa/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
