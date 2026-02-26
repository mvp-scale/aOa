//go:build !lean && !recon

package main

func init() {
	// This file exists solely to block bare "go build ./cmd/aoa/".
	// The standard build uses: CGO_ENABLED=0 -tags lean  (no recon, no CGo)
	// The recon build uses:    -tags recon                (opt-in only)
	//
	// Use ./build.sh or make build. Never run go build directly.
	panic("BLOCKED: bare 'go build' is not allowed. Use ./build.sh or make build.")
}
