# aOa Architecture Views — Enhancement & Integration Guide

**Status:** research & integration plan — no code changes prescribed until each
phase is green-lit. **Scope law:** `.context/decisions/2026-06-11-core-competence-and-scope-line.md`.
**Rendering law:** `playbook/standards/view-standards.json`.

The playbook proved the presentation layer: 16 standard views, provenance
stamps, findings, journeys, quality gates. This guide maps every view, pattern,
and capability onto **aOa as it stands** — the most performant, realistic
integration point for each — so the mockup backs into the product.

---

## 1. What aOa has today (integration surfaces)

| Surface | Reality | Relevance |
|---|---|---|
| `internal/adapters/treesitter` | 28-language parser; walker extracts symbols, signatures, method boundaries; imports are *counted* (walker.go:567) but **not stored as edges** | The keystone gap: one walker pass already touches every import node |
| `internal/ports/storage.go` | `Index{Tokens, Metadata(SymbolMeta), Files(FileMeta)}` — FileMeta carries `Language` and `Domain` (atlas-assigned) | Facts substrate slots beside `Index` as a sibling structure, same bbolt persistence |
| `internal/domain/enricher` + `atlas/v1` | 134 semantic domains, O(1) keyword→term→domain | The layer-2 "what it MEANS" engine — already built, already honest |
| `internal/adapters/bbolt` | Project-scoped buckets, JSON serialization, `{root}/.aoa/aoa.db` | Facts store home: append-friendly, zero new dependencies |
| `internal/adapters/socket` | JSON-over-unix-socket daemon, concurrent clients | The query plane for views, journeys, focus flows |
| `internal/adapters/web` | Embedded HTML dashboard, JSON API, localhost | Where the playbook viewer ships in-product |
| `cmd/aoa/cmd` | grep/egrep/find/locate/tree/peek/daemon/init/health… | The `aoa arch` command family lands here |
| recon (`--recon`, parked) + git overlays | Findings bitmask + churn already feed the LOCAL estate views | The findings pipeline's real-data inputs exist |

**The one structural gap:** the index answers "where is symbol X" in O(1) but
cannot answer "what does package A import." Everything in §3 hangs off closing
that gap **inside the existing walker pass** (no second parse, G0-safe).

## 2. The facts substrate (checkpoint #4 — the enabler)

```
walker pass ──emit──▶ facts JSONL (append-only)      {root}/.aoa/facts/*.jsonl
                          │ compact
                          ▼
                    bbolt facts buckets               (edges, units, configs, deltas)
                          │ O(1)/O(edges) reads
                          ▼
        renditions: views · findings · journeys · focus flows · evidence packs
```

- **Fact shape:** `{kind, subject, object?, attrs{}, source{file,line,commit}, prov}` —
  kinds: `unit` (pkg/module/class), `dep` (import edge), `route`, `schema`,
  `deploy`, `owner`, `delta`. Every fact carries its source pointer: this is
  what makes evidence packs auditable later.
- **Emission cost:** the walker already visits import/route/schema nodes; emitting
  a fact is a struct append during the same pass. Index build time budget
  unchanged (G0). Facts compact into bbolt on the same autotune-style cadence.
- **Mock first:** the playbook implements this contract with per-scope JSONL +
  snapshot substrate (no Go changes) to prove the touch-one-package demo and
  derive one journey from facts. The Go implementation then matches the contract.

## 3. View-by-view integration matrix

Provenance ceiling = the honest maximum on an arbitrary repo (ladder: REAL
derive / MIXED infer / DECLARED). Effort: S < 1d · M ≈ 2–4d · L ≈ 1–2wk.
Phase: ① substrate mock ② keystone+GREEN views ③ live estate ④ evidence/governance.

| View | Facts needed | aOa source | Integration pattern | Perf notes | Ceiling | Effort | Phase |
|---|---|---|---|---|---|---|---|
| component | `unit`, `dep` | walker import queries (NEW — keystone) | emit during parse; render groups by path-prefix + atlas domain | O(edges) read; cache laid-out shard | REAL (grouping MIXED) | M | ② |
| dsm | same `dep` edges | derived from component facts | matrix rendition, zero new data | O(n²) render only, n=modules ≤ ~50 | REAL | S | ② |
| cycles | same | Tarjan SCC over `dep` | findings pipeline entry | O(V+E), trivial at module grain | REAL | S | ② |
| code (L4) | `unit` + symbols | **exists today** (SymbolMeta, peek) | render critical-path subset; agent picks subset | O(1) symbol reads | REAL (subset choice MIXED) | S | ② |
| techportfolio | manifests + `Language` per FileMeta | exists + lockfile parse (NEW, small) | table rendition; EOL/CVE joins external feeds later | parse-at-index-time | REAL | S | ② |
| sbom | lockfiles | lockfile parser (NEW, syft-pattern) | table rendition; CycloneDX export | parse-at-index-time | REAL (unpinned → flagged) | M | ② |
| datamodel | `schema` facts | ORM/DDL/migration tree-sitter queries (NEW per stack) | entity rendition; verbs inferred by agent | per-stack extractors, lazy | REAL fields / MIXED verbs | M | ③ |
| container | `deploy` facts | compose/k8s/Dockerfile parsers (NEW, config not code) | buckets rendition | config parse trivial | REAL if IaC in repo, else MIXED | M | ③ |
| context | `dep` on external SDKs + env/config | SDK-dependency heuristics + agent naming | simple rendition; ALWAYS stamped MIXED | cheap | MIXED | M | ③ |
| domains | `unit` + atlas Domain per file | **enricher exists** | buckets rendition; agent names buckets, never adds | O(files) | MIXED (honest strength) | S | ② |
| dataflow | `route` + store/queue clients | source/sink tree-sitter queries + agent verbs | simple rendition | per-stack queries | MIXED | M | ③ |
| sequence | call-chain from entrypoint | symbol graph walk + agent narration | each step must cite a symbol or marked inferred | bounded depth walk | MIXED | L | ③ |
| statemachine | enum + transition writes | explicit-machine extractors (XState/Spring) else DECLARED | render extracted; declared otherwise | niche extractors | MIXED/DECLARED | L | ④ |
| trust | zone DECLARATIONS + `dep`/`dataflow` crossings | `.aoa/arch.yaml` declarations; detectors diff | conformance machinery (§4) | diff is O(edges) | DECLARED + REAL diff | M | ④ |
| glossary | term DECLARATIONS; atlas candidates | agent harvests candidates → human ratifies | table; MIXED until approved | — | DECLARED | S | ④ |
| API surface (NEW) | `route` facts | route tree-sitter queries (Spring/Express/Go mux…) + OpenAPI files | table+graph rendition | per-stack queries | REAL routes / MIXED consumers | M | ③ |
| ownership (NEW) | CODEOWNERS + git authorship | git adapter (churn exists) + CODEOWNERS parse | overlay + table | cheap | REAL | S | ③ |
| decision log (NEW) | `docs/adr/*.md` + git | repo scan; drift = ADR-touched-files × churn | table + drift findings | cheap | REAL | M | ④ |
| event catalog (NEW) | AsyncAPI/broker config + producer/consumer symbols | config parse + symbol query | table+graph | cheap | REAL/MIXED | M | ④ |
| estate landscape (NEW) | cross-repo `unit`+`dep` rollup | multi-root substrate union | scope-level rendition of existing facts | needs multi-project keying | REAL | M | ④ |

## 4. Patterns, findings, conformance

- **Detectors run at substrate-compact time, not render time** — cycles, god
  (fan-in/out thresholds), orphans, dead-code candidates (`unit` with zero
  inbound `dep`/reference facts — always "candidate," reflection caveat stated).
  Findings are facts (`kind:finding`) with severity + source pointers; the
  viewer and the dock read them like any other rendition.
- **Conformance = declared template diffed against derived edges.** Declaration
  lives in `{root}/.aoa/arch.yaml`: pattern name (layered | hexagonal | onion |
  custom) + role→path mapping (aOa's own CLAUDE.md hexagonal description is the
  size of one). Output: convergent / divergent / absent edge classes through
  the findings pipeline. **Baseline/freeze** (ArchUnit pattern) stored in bbolt:
  report only NEW drift. This is the 17th view and the Sonar-validated market.
- **Journeys & focus flow:** post-substrate renditions. Journey = stored or
  derived step list (contract shipped in the playbook, 2026-06-11); focus flow =
  k-shortest-path over `dep`+`dataflow` facts between two anchors, each hop
  rendered with its view fragment + deltas + findings. Path queries are
  O(E log V) at module grain — sub-ms for any sane estate.

## 5. Agent interface: CLI-first, MCP as an adapter

**Recommendation: direct CLI commands are primary; MCP is a thin optional
adapter added later.** Rationale:

| | CLI (`aoa arch …`) | MCP server |
|---|---|---|
| Latency | process exec ~ms; daemon-socket variant sub-ms | JSON-RPC over stdio + handshake/session overhead |
| Precedent | G3 is literally "agent-first CLI" — agents already speak aOa via the grep shim; CLAUDE.md guidance pattern is proven | new protocol surface to maintain |
| Reach | any agent that can shell out (all of them) | agents/IDEs with MCP support only |
| Architecture | new cobra commands over the same app service | one more adapter beside socket/web — hexagonal makes it cheap **later** |

Proposed command family (JSON to stdout, mirrors the shard contract exactly):

```
aoa arch views                      # catalog + status per view (live/mixed/declared/planned)
aoa arch view <id> [--scope p]      # one view's rendition JSON (= a shard)
aoa arch findings [--new]           # findings, baseline-aware
aoa arch journey <id> | derive A B  # stored journey / focus-flow derivation
aoa arch facts <subject>            # raw facts + source pointers (the audit trail)
aoa arch pack <dd|pci|delta>        # evidence pack export (§6)
```

CAST and vFunction both shipped MCP servers in 2025–26 — the ecosystem signal
is real, so the MCP adapter stays on the roadmap (Phase ④, wrapping the same
service the CLI calls; zero duplicated logic under hexagonal rules).

## 6. Governance & evidence packs (where the budget research points)

Renditions over the substrate, exported as self-contained documents:
1. **DD exhibit set** — current-state views per system + findings scorecard +
   SBOM; every figure carries provenance + commit stamp (the anti-"seller diagram").
2. **PCI/SOC2 evidence bundle** — trust + dataflow views, asset inventory,
   SBOM; regenerated-on-change satisfies the update-on-change obligations.
3. **"What changed since \<ref\>" pack** — `delta` facts between two commits,
   scoped to a focus area: affected closure, view diffs, new findings.
   Market white space; only credible from code.

All three are Phase ④ renderers — no new derivation, only assembly + export of
facts and views that earlier phases produce.

## 7. Versus graphify

Graphify renders one import graph for one Python codebase, hand-grouped. After
Phase ②, aOa derives the same view across 28 languages with provenance,
findings, DSM/cycles analysis, and an agent-queryable substrate behind it —
and graphify itself becomes just another estate in the dropdown (it already
is). The gap closes at the keystone and never reopens.

## 8. Phase plan

| Phase | Deliverable | Proof | Size |
|---|---|---|---|
| ① substrate mock (playbook) | facts JSONL + snapshot substrate + renditions contract | touch-one-package demo; one journey derived from facts | days |
| ② keystone + GREEN views (Go) | import-edge facts from walker; `aoa arch view component/dsm/cycles/code/techportfolio`; findings detectors | the 5 GREEN views on a stranger's repo, REAL-stamped | ~1–2 wk |
| ③ live estate | AMBER extractors (routes, schemas, deploy configs, ownership) + leashed-agent naming; viewer reads substrate | real-repo PoC campaign: clone → derive → judge → delete | ~2 wk |
| ④ governance | conformance view + baseline; evidence packs; MCP adapter; remaining views | a DD exhibit pack generated end-to-end on a live repo | ~2–3 wk |

**Scope guard on every line above:** rendition of derived facts, or diff
against a declaration. Anything else gets a provenance stamp that says what it
is — or it doesn't ship.
