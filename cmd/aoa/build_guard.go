//go:build !lean && !core && !recon && !testing

package main

func init() {
	// This file exists solely to block bare "go build ./cmd/aoa/".
	// The standard build uses: CGO_ENABLED=0 -tags lean  (no recon, no CGo)
	// The core build uses:     -tags core                 (tree-sitter runtime, no compiled grammars)
	// The recon build uses:    -tags recon                (opt-in only)
	// Integration tests use:   -tags testing              (test harness only)
	//
	// Use ./build.sh or make build. Never run go build directly.
	panic("BLOCKED: bare 'go build' is not allowed. Use ./build.sh or make build.")
}
