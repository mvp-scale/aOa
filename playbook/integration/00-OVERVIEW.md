# aOa Architecture Views — Integration Overview

One page: what integrates, where it lands, how it flows, and why the result is
graphify-plus. Detail: specs 01–03 · pre-build review: 04 · plan: ../ENHANCEMENT-GUIDE.md.

## The integration map

```
                          ┌─ aOa today ─────────────┐   ┌─ NEW (first slice) ──────────┐
  any repo                │                          │   │                               │
  ┌──────┐   parse pass   │  treesitter walker       │   │ + import-edge emission        │
  │ code ├───────────────▶│  (28 languages)          ├──▶│   (same pass, ≤+3% gated)     │
  └──────┘                │  internal/adapters/      │   │   parser.go extractors        │
                          │  treesitter/parser.go    │   │                               │
                          └──────────┬───────────────┘   └──────────────┬────────────────┘
                                     ▼                                  ▼
                          ┌──────────────────────────┐   ┌───────────────────────────────┐
                          │  Index{Tokens,Metadata,  │   │ + FileMeta.Imports            │
                          │  Files} · bbolt · .aoa/  ├──▶│ + DepAdjacency (resolved at   │
                          │  internal/ports/storage  │   │   debounce, in-memory)        │
                          └──────────┬───────────────┘   └──────────────┬────────────────┘
                                     │                                  ▼
                                     │                   ┌───────────────────────────────┐
                          atlas 134 domains ────────────▶│ internal/domain/arch/         │
                          git overlays ─────────────────▶│  renderers: facts → shard JSON│
                                     │                   │  detectors: cycle·god·orphan  │
                                     │                   │  (compact-time, never render) │
                                     ▼                   └──────────────┬────────────────┘
                          ┌──────────────────────────┐                  ▼
                          │  daemon (socket) · web   │   ┌───────────────────────────────┐
                          │  dashboard · CLI (cobra) ├──▶│ `aoa arch view|findings|facts`│
                          └──────────────────────────┘   │ socket: arch.* · later:       │
                                                         │ /arch/ page + /api/arch/*     │
                                                         └──────────────┬────────────────┘
                                                                        ▼
                                                         ┌───────────────────────────────┐
                                                         │  THE VIEWER (already built &   │
                                                         │  judged in playbook/): 16 view │
                                                         │  types · findings dock ·       │
                                                         │  captions · journeys —         │
                                                         │  consumes the same shard       │
                                                         │  contract unchanged            │
                                                         └───────────────────────────────┘
```

**Where each piece lands**

| Component | Lands in | Phase |
|---|---|---|
| Import-edge emission | existing extractors in `internal/adapters/treesitter/parser.go` | ② |
| Imports + adjacency | extension of `ports.Index`/`FileMeta` (NOT a parallel store) | ② |
| Renderers + detectors | new `internal/domain/arch/` (dependency-free, hexagonal) | ② |
| `aoa arch` commands | `cmd/aoa/cmd/` + socket methods (daemon-first) | ② |
| Agent guidance | `aoa init` CLAUDE.md block (grep→peek pattern) | ② |
| Generic Fact model (route/schema/deploy/owner) | `ports.FactStore` generalization | ③ |
| Viewer in-product | `internal/adapters/web/static/arch/` (vendored bundle, tagged out of --light) | ③ |
| Conformance (arch.yaml), evidence packs, MCP adapter | domain/arch + web | ④ |

**First slice exit gate (= checkpoint #4 proof, upgraded):** clone a stranger's
repo → `aoa init` → component/DSM/cycles/domains render in the viewer → edit
one package → only the affected shards change. Three honest weeks.

> Note: the REAL/MIXED/SIMULATED provenance stamps were playbook REQUIREMENTS
> scaffolding — used to pin the format, structure, and limits for rendering any
> model — not a product deliverable. The derive/infer discipline stays as
> internal engineering law; the pills do not ship as a headline UI.

## The story: everything graphify gives us, plus

graphify proved the idea: parse a Python codebase's imports, draw the graph,
find the tangles. Its ceiling is structural — one language, one view, a
hand-run script, edges with no evidence trail, grouping by force-directed
physics and LLM guesses.

aOa absorbs its strengths (the tsconfig/workspace resolvers, the affected-set
idea, confidence labels — and we fixed its Java resolver defect on paper) and
removes the ceiling:

| | graphify | aOa arch views |
|---|---|---|
| Languages | Python (+partial JS/Java) | 28, one walker |
| Views | 1 import graph (+tree, callflow) | 16 standard types architects already trust |
| Freshness | manual script run | rides the index — regenerates as you type |
| Evidence | none | every edge carries file:line:commit (`aoa arch facts`) |
| Quality bar | eyeball | lint + blind-judge gate (a model must answer each view's question from the image alone) |
| Findings | cycles list | cycles · god · orphan · dead-candidates · band violations → findings dock, baseline/--new |
| For AI agents | none | CLI-first (`aoa arch`), sub-ms daemon reads, zero standing token cost |
| Cost to adopt | clone, pip, run | already inside the tool indexing your repo |

The deeper difference: graphify renders *a* graph; aOa maintains *the
substrate* — symbols, edges, domains, churn, findings with sources — and the
views, journeys, conformance checks, and (later) evidence packs are all
renditions of it. That's the same capability the market leaders charge $10K–$800K
per app for (CAST), require production agents for (vFunction), or cap at five
languages for (Sonar) — and none of them are free at the point of `aoa init`.

## Guardrails (what keeps this from stuffing aOa)

- **G0 is benchmark-gated**: ≤+3% index build, max-RSS-during-compact measured
  on a 30k-file fixture, <200ms startup untouched (verified: nothing eager).
- **Locking law**: no arch/facts write ever holds App.mu; daemon-first reads.
- **Scope law** (ADR 2026-06-11): derive what code IS, infer-with-leash what it
  MEANS, declare-and-diff intent. Nothing else ships.
- **Build split**: `--light` stays lean — no parser → "dep facts unavailable",
  viewer bundle tagged out.
- **Phases gate on proof**, not plans: ② ends on a stranger's repo passing the
  blind judge, or it doesn't end.
