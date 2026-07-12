// Command kglab is the stand-aside "Angle of Attack" demo: the knowledge graph
// as a driving force — where-we-are (graph), where-to-be (target), the vector
// between (drift), and the eat-the-elephant completeness ledger.
//
// It is agent-agnostic on purpose: plain stdlib JSON + POSIX exit codes over
// os.Args, no Claude/MCP/cobra dependency. Any agent (GPT, Gemini, a bash
// script) drives it identically.
//
//	go run ./cmd/kglab graph  [--json] [--render component|cycles|dsm] [--seed ID --dir reverse]
//	go run ./cmd/kglab target [--json]
//	go run ./cmd/kglab drift  [--json]
//	go run ./cmd/kglab ledger [--json]
//
// Exit codes: 0 ok · 2 usage/error.
package main

import (
	"fmt"
	"os"

	"github.com/corey/aoa/internal/kglab"
)

func main() { os.Exit(run(os.Args[1:])) }

// run is the testable entry point (main() only wraps os.Exit).
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: kglab {graph|target|drift|ledger} [--json] [--render R] [--seed ID --dir reverse]")
		return 2
	}
	verb := args[0]
	json := false
	render, seed, dir := "component", "", "forward"
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			json = true
		case "--render":
			if i+1 < len(args) {
				i++
				render = args[i]
			}
		case "--seed":
			if i+1 < len(args) {
				i++
				seed = args[i]
			}
		case "--dir":
			if i+1 < len(args) {
				i++
				dir = args[i]
			}
		}
	}

	var payload interface{}
	var caption string
	var ok bool
	switch verb {
	case "graph":
		r := kglab.ComputeGraph(render, seed, dir)
		payload, caption, ok = r, r.Caption, r.OK
	case "target":
		r := kglab.ComputeTarget()
		payload, caption, ok = r, r.Caption, r.OK
	case "drift":
		r := kglab.ComputeDrift()
		payload, caption, ok = r, r.Caption, r.OK
	case "ledger":
		r := kglab.ComputeLedger("")
		payload, caption, ok = r, r.Caption, r.OK
	default:
		fmt.Fprintf(os.Stderr, "kglab: unknown verb %q (want graph|target|drift|ledger)\n", verb)
		return 2
	}

	if json {
		b, err := kglab.RenderResponseJSON(payload)
		if err != nil {
			fmt.Fprintf(os.Stderr, "kglab: json error: %v\n", err)
			return 2
		}
		fmt.Println(string(b))
	} else {
		fmt.Println(caption)
	}
	if !ok {
		return 2
	}
	return 0
}
