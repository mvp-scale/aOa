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
	"strings"

	"github.com/corey/aoa/internal/kglab"
)

func main() { os.Exit(run(os.Args[1:])) }

// runBlueprint reads a .aoa authoring file and prints the full readable
// structure: CURRENT STATE (what the code is) · TARGET (what you declared) ·
// DRIFT (the gap). This is the authoring-structure → view loop, end to end.
func runBlueprint(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: kglab blueprint <file.aoa>")
		return 2
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "kglab: %v\n", err)
		return 2
	}
	spec, err := kglab.ParseEstate(string(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "kglab: parse error: %v\n", err)
		return 2
	}

	units, deps := kglab.SampleGraph()

	fmt.Printf("╔═ BLUEPRINT: %s  (authored in %s)\n", spec.Name, args[0])

	// 1. CURRENT STATE — render the declared view over the real code.
	fmt.Printf("║\n║ ① CURRENT STATE — what the code IS  (view: %s)\n", spec.View)
	shard, err := kglab.Compile(kglab.ViewQuery{Scope: spec.Name, Render: kglab.RenderSpec{Kind: spec.View}}, units, deps)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kglab: %v\n", err)
		return 2
	}
	for _, b := range shard.Buckets {
		names := make([]string, len(b.Members))
		for i, m := range b.Members {
			names[i] = m.Label
		}
		fmt.Printf("║    %-9s : %s\n", b.Label, strings.Join(names, ", "))
	}
	for _, e := range shard.Edges {
		fmt.Printf("║      %s → %s  ×%d\n", e.Source, e.Target, e.Count)
	}

	// 2. TARGET — the edges the author declared should exist.
	fmt.Printf("║\n║ ② TARGET — what you DECLARED should be true  (%d allowed edges)\n", len(spec.Allowed))
	for _, t := range spec.Allowed {
		fmt.Printf("║    allow  %s → %s\n", t.FromUnit, t.ToUnit)
	}
	for _, f := range spec.Forbid {
		fmt.Printf("║    forbid %s\n", f)
	}

	// 3. DRIFT — real vs target.
	real := kglab.FactSetFromDeps("real", deps)
	target, err := kglab.LoadTarget("target", spec.Allowed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kglab: %v\n", err)
		return 2
	}
	// Friendly-name map (unit ID -> readable label) so output isn't raw IDs.
	labelOf := map[string]string{}
	for _, u := range units {
		labelOf[u.ID] = u.Label
	}
	name := func(id string) string {
		if l := labelOf[id]; l != "" {
			return l
		}
		return id
	}

	d := kglab.DriftDiff(real, target)
	fmt.Printf("║\n║ ③ DRIFT — current vs target:  %d VIOLATION · %d MISSING · %d CONFORMANT\n", d.Violations, d.Missing, d.Conformant)
	for _, it := range d.Items {
		from, to := name(it.Fact.FromUnit), name(it.Fact.ToUnit)
		switch it.Alignment {
		case kglab.AlignViolation:
			fmt.Printf("║    ✗ VIOLATION  %s → %s\n", from, to)
			fmt.Printf("║        WHY : your code has this import, but the target does not allow it\n")
			fmt.Printf("║        WHERE: %s:%d\n", it.Fact.File, it.Fact.Line)
			fmt.Printf("║        FIX : remove the import of %q from %s (or invert it via a ports interface)\n", to, from)
		case kglab.AlignMissing:
			fmt.Printf("║    ○ MISSING    %s → %s\n", from, to)
			fmt.Printf("║        WHY : the target declares this edge, but the code has not built it\n")
			fmt.Printf("║        FIX : add an import so %s depends on %s\n", from, to)
		}
	}
	fmt.Printf("╚═ %d conformant edges hidden. exit %d\n", d.Conformant, boolToExit(d.Violations > 0))
	if d.Violations > 0 {
		return 1
	}
	return 0
}

func boolToExit(b bool) int {
	if b {
		return 1
	}
	return 0
}

// run is the testable entry point (main() only wraps os.Exit).
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: kglab {graph|target|drift|ledger} [--json] [--render R] [--seed ID --dir reverse]")
		return 2
	}
	verb := args[0]
	if verb == "blueprint" {
		return runBlueprint(args[1:])
	}
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
