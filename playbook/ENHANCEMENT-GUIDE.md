# aOa Architecture Views — Enhancement & Integration Guide

**Status:** integrated architectural position — DRAFT for red-team. No code
changes prescribed until each phase is green-lit. **This is a falsifiable
document:** every load-bearing claim cites a file:line or a source doc; if a
cited anchor is wrong, the claim built on it is void.

**Binding law (read before relitigating anything here):**
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md` (the three-layer ladder: derive REAL / infer-leashed MIXED / declare-and-diff)
- **Goals** — `.context/GOALS.md` (G0 Speed, G2 Two-Binary split, G3 Agent-First, G4 Hexagonal — non-negotiable)
- **Rendering law** — `playbook/standards/view-standards.json` + quality gate `playbook/standards/MODEL-STANDARD.md`
- **Prior research verdict** — `.context/details/2026-06-19-graphify-plus-mcp-research.md` (treated as established; sharpened here, not relitigated)

**Implementation-grade specs** (solutions-architecture detail per plane):
- `integration/01-facts-substrate.md` — data plane: fact model, ports, keystone import-edge extraction per language, bbolt layout, incremental/deltas, performance gates, graphify parity+ (17 rows), 11 tasks
- `integration/02-arch-service.md` — service plane: `aoa arch` family, rendition engine (byte-compatible with the viewer's shard contract), detectors, conformance + baseline, derive A→B, agent guidance, 14 tasks
- `integration/03-visualization.md` — presentation plane: viewer extraction to web adapter (fork-guard), daemon data feed, vendored bundle, governance surfaces, evidence-pack export, graphify keep/improve/drop, 13 tasks

---

## 0. The hand-in-hand thesis: knowledge graph + visualization = one substrate, two faces

The central architectural claim of this guide: **a knowledge graph and the
mockup-style architecture viewer are not two features that happen to coexist —
they are one product, because they read one substrate.** The knowledge graph is
that substrate's *machine face*; the viewer is its *human face*. Build the
substrate once and both faces light up; build either face without the substrate
and you have a demo, not a product.

This is not a slogan retrofitted onto two unrelated builds. It is forced by the
data contract that already exists in the playbook. The viewer fetches a
content-hashed **manifest → shard** chain (`architecture-c4.html:32-37`); each
shard is a self-contained `{buckets[], members[], edges[], findings[]}` JSON
object for exactly one view. **That same shard JSON is the agent's query
result.** When `aoa arch view component` returns a shard to an agent and the
browser fetches the same shard to render, they are reading one truth
(`00-OVERVIEW.md:90-94`: *"aOa maintains the substrate… and the views, journeys,
conformance checks are all renditions of it"*; `03-visualization.md`: the viewer
*"consumes the same shard contract unchanged"*).

The two faces are *mutually load-bearing*, and the reinforcement runs in both
directions:

1. **The graph gives the viewer evidence-backed, leash-bounded, fresh content.**
   Every rendered pixel traces to a fact with a `source{file,line,commit}`
   pointer. The leash rule as the viewer spec states it (`03-visualization.md:343`:
   *"agent may name/group, never add a node/edge"*) forbids the picture from
   saying anything the facts don't — note the binding ADR text is narrower
   ("never add a **node**", `2026-06-11:26`); "/edge" is the derived corollary of
   §2.1, carried by the spec but not in the ADR's words —
   graphify's LLM `semantically_similar_to` edges and force-directed physics are
   explicitly **dropped** for exactly this reason.

2. **The viewer's blind-judge gate gives the graph a falsifiable output-quality
   test that disciplines the substrate, not just the CSS.** The gate
   (`MODEL-STANDARD.md:43-53`, the "Blind judge" section) hands a judge agent
   *only* a screenshot, the view's `question`, and the `pass` criterion — no JSON,
   no context (`:45-50`) — and asks
   "can you answer the question from the image alone?" A view rendered at member
   grain — e.g. the faulted event-platform component view at *66 components*
   (`manifest.json:366`, an `8 buckets · 66 components · 10 edges` count) — risks
   failing that readability gate, which is exactly why the **deriver** aggregates
   to bucket grain *before* it renders (`03-visualization.md:298-305`); note that
   shard is itself `prov: simulated` (`manifest.json:369`), so it stands as the
   illustrative member-grain risk, not a recorded judge failure.
   The visual quality bar reaches back and constrains how the graph is shaped.
   No competitor renderer has a falsifiable, automatable visual acceptance test;
   graphify's bar is one word — *"eyeball"* (`00-OVERVIEW.md:85`).

3. **Freshness is shared** — *provided the edges are emitted on the always-on
   `ParseFileToMeta` index path (§5.1 site (a))*. Both faces then ride the live
   index: fsnotify → `onFileChanged` reindex → revision bump; the agent's sub-ms
   socket read and the human's ETag-polled auto-refreshing canvas are fed by the
   *same* invalidation event (`03-visualization.md:139-148`). graphify is a
   hand-run script over a stale `graph.json` build artifact (`serve.py:19
   _load_graph`, mtime-poll); aOa's two faces stay fresh together because they
   share one substrate and one freshness signal. (This shared freshness is a
   consequence of the site-(a) keystone choice, not an automatic property — see
   §5.1/§5.2.)

**The spine, stated once:** the rest of this guide is the engineering of that
single sentence — *aOa turns any codebase into a deterministic,
provenance-stamped fact substrate the instant you run `aoa init`, and serves it
through one contract to both an agent (CLI/socket/MCP, sub-ms, file:line:commit
on every answer) and a human (16 architect-trusted views that pass a
blind-readability gate) — so the diagram and the agent's answer are the same
truth, never a seller's drawing* (`2026-06-19-graphify-plus-mcp-research.md:180-184`).

Everything below — substrate shape, could-vs-should, access surface, the
language ladder, and the integration seams — hangs off this spine.

---

## 1. The structural gap — three node-maps, zero relations

aOa's entire persisted code model is `ports.Index` (`internal/ports/storage.go:59-63`):
three maps, all keyed off a compact `TokenRef{FileID uint32, Line uint16}`.

| Map | Type | Holds | Source |
|-----|------|-------|--------|
| `Tokens` | `map[string][]TokenRef` | token → locations (the O(1) search spine) | storage.go:60 |
| `Metadata` | `map[TokenRef]*SymbolMeta` | location → symbol | storage.go:61 |
| `Files` | `map[uint32]*FileMeta` | fileID → file (carries `Language`, atlas `Domain`) | storage.go:62 |

`SymbolMeta` (storage.go:72-80) carries `Name, Signature, Kind, StartLine,
EndLine, Parent, Tags` — the per-method `[start-end]` byte range that powers
`peek`.

**The decisive structural fact: every entry is NODE-shaped.** Each describes one
thing (a symbol, a file) or maps a name *to* a thing. There is no map whose key
and value are both symbols — **no relation/edge is representable in the current
model.** `SymbolMeta.Parent` (storage.go:78) is the single proto-edge: it
encodes method→receiver containment, but only as an un-indexed display *string*,
not a traversable reference. `grep -niE "edge|graph|reachab|shortest|affected|cycle|dsm"`
over `ports/` + `domain/index/` returns **zero** relationship primitives.

So the whole question "what does a KG add?" reduces to: **a KG adds exactly one
new shape the index cannot express — a typed relation `(src, kind, dst)`.**
Minimal sibling to `Index`:

```
Edges []Edge   where Edge = {Src TokenRef, Dst TokenRef|DstPath, Kind string ("imports"|"contains"|...), prov}
```

- Reuses the **existing `TokenRef` identity** (storage.go:66-69) as endpoints —
  no new node-identity scheme; edges point at symbols/files already in
  `Metadata`/`Files`.
- `imports` is nearly free today (the AST nodes are already visited — §5).
  `contains` is *already latent* in `SymbolMeta.Parent` and could be promoted to
  a real edge with zero new parsing.
- This single shape is what unlocks the only query classes that genuinely beat
  grep (§2). None are expressible against three node-maps; all are trivial
  against an adjacency list.

> **`[]Edge` is the LOGICAL shape, not the storage layout.** The flat slice above
> describes the *fact* — a typed triple — and is how the rendition engine and the
> agent see an edge set. It is **not** how the store must key edges on disk or in
> memory. The write path forces a different physical layout: per-file
> invalidation on every keystroke (§5.2 seam 6) means the store must be able to
> drop and re-emit *one file's* outbound edges without scanning the whole edge
> set. So the storage layout is keyed for per-file deletion — `map[FileID][]Edge`
> (outbound-by-src), with an inbound index built for reachability/blast queries —
> never a single flat `[]Edge` the watcher would have to linear-scan per edit
> (the G0 reason is spelled out in §5.1/§5.2). Logical shape: a triple set.
> Physical shape: keyed by source file.

---

## 2. Could do vs should do — the scope line over the graph

A graph is technically capable of an enormous query surface. The scope law and
G0–G6 cut that surface down to what aOa **should** build. The cut runs per query
class, and each verdict cites graphify's own implementation as the falsifiable
reference for "what this would actually be."

### 2.1 The provenance ladder is the cut, not a label

The scope law's three-layer ladder (`2026-06-11-core-competence-and-scope-line.md:23-31`)
is the gate every proposed edge passes through:

| Layer | What it is | Move | Stamp |
|---|---|---|---|
| 1 | What the code **IS** — imports, structure, configs, churn | DERIVE (tree-sitter, git, manifests) | **REAL** |
| 2 | What the code **MEANS** — names, groupings, domains, verbs | INFER, **leashed**: agent may name/group/annotate extracted facts, **NEVER add a node** (ADR `:26`, verbatim) | **MIXED** |
| 3 | What was **INTENDED** / what the world does — trust policy, runtime | DECLARE or INGEST, then **diff layer 3 against layer 1** | **DECLARED / SIMULATED / OBSERVED** |

> **Corollary (the draft's own, not the ADR's words):** the ADR leash says
> "never add a node." Edges are layer-1 REAL facts (derived structure), so an
> agent *adding an edge* would be inventing structure the code does not contain —
> out of bounds for the same reason a node is. We carry the leash as "never add a
> node **or edge**," but that "/edge" is our derived corollary; the binding ADR
> text says only "node" (`2026-06-11-core-competence-and-scope-line.md:26`).

> **Productization note (`00-OVERVIEW.md:63-66`):** the REAL/MIXED/SIMULATED
> *pills* were playbook scaffolding to pin the format. The derive/infer
> discipline stays as engineering law; the pills do not ship as a headline UI.
> Provenance stays internal on every fact — file:line:commit — but it plays
> **two distinct roles, and conflating them overstates the argument:**
> - **On layer-1 REAL edges (the keystone import edge), the `source` pointer is
>   an audit/freshness anchor, not an inference leash.** There is no inference to
>   discipline — the edge is deterministic AST output — so the stamp's job is to
>   make the fact *auditable and re-derivable*: it powers `aoa arch facts`,
>   tells you the commit the edge was last derived at, and lets the substrate
>   recompute on change. On this row a red-teamer is right that provenance is
>   *closer to* decoration than to a safety mechanism — its value is freshness
>   and audit, not leashing.
> - **On layer-2 MIXED content (the §6 grouping/naming/verb rows), the stamp is
>   the load-bearing mechanism that makes inference safe to ship.** Here an agent
>   *is* adding meaning on top of extracted facts; the provenance stamp is what
>   pins each named bucket or inferred verb back to the REAL facts it sits on, so
>   the leash is checkable. This is the row where "provenance makes inference
>   safe to ship" is literally true.
>
> Keep both — but do not claim the second role for the first. The keystone edge's
> stamp earns its keep as an audit/freshness anchor, not as an inference leash.

### 2.2 Per-query verdict — SHOULD BUILD

The four classes below are *transitive-closure or global-topology shaped*. Each
asks a question about the structure **between** nodes, which a token→location
index cannot represent and which agentic grep can only fake through unbounded
recursive round-trips (and cannot prove a negative).

| Query class | Verdict | Why grep structurally can't | graphify evidence |
|---|---|---|---|
| **Reachability / `shortest_path`** ("how does A connect to B?") | **BUILD** | A transitive-closure question; answer length unbounded a priori. An agent simulates it only by recursively grepping each callee N round-trips deep, re-deciding the frontier each time — and **cannot prove "no path exists."** | `serve.py:822` `nx.shortest_path`, hop chain `:828-847`, "No path found" `:823-824` |
| **Affected-set / reverse-deps / PR blast-radius** ("what breaks if I change X / which PRs collide?") | **BUILD — ELEVATE (graphify's single best idea)** | grep finds *forward* literal occurrences. Reverse transitive dependency, and *set-intersection across changesets* ("which in-flight PRs touch overlapping subsystems"), are edge-closure operations grep has no notion of. **Cheap, and never stale because the PR file-list is a git feature.** | `get_pr_impact`/`triage_prs` `serve.py:862-932`, blast-radius rank `:919-926` |
| **Cycles / DSM** ("circular deps / layering violations?") | **BUILD (Tier-1, mechanical)** | A cycle `A→B→C→A` is a topological property — there is no string to match; it exists only in the edge structure. DSM is the adjacency matrix itself. Note graphify does **not** expose these (`serve.py:564-684` lists 10 tools, no cycles/dsm) — aOa-native opportunity. | aOa-native; SCC/Tarjan over the dep edge set |
| **Architecture orientation — `god_nodes`** ("most-connected abstractions?") | **BUILD god_nodes; community detection OPTIONAL** | Degree-centrality is a global graph property; grep counts textual occurrences, not structural in/out-degree. | `god_nodes` `serve.py:775-780`, `graph_stats` `:782-792` |

**Freshness is the decisive asymmetry on every "build" row.** In graphify these
ride a stale `graph.json` artifact; aOa derives the same edges from the AST
inside the live, fsnotify-reindexed parse pass — so the grep-can't classes become
**graph-capable AND never-stale**, capturing graphify's real value (which is
narrow) while eliminating its structural defect.

### 2.3 Per-query verdict — DO NOT BUILD (and explicitly OUT)

| Anti-pattern | Verdict | Why |
|---|---|---|
| **`query_graph` / `get_node` / `get_neighbors`** (1-hop "show me X and its neighbors") | **DO NOT BUILD — the stale-grep trap** | graphify's seed step is an **un-indexed full node scan** (`_score_nodes serve.py:144 for nid,data in G.nodes()`, `get_node` linear scan `:717-718`) over a snapshot only as fresh as the last build. That is literally a slower grep over a stale graph. **The tell:** graphify's own output must *nudge agents off grep* (`serve.py:415-418`, `:670-675`). For "show this function and what it calls," fresh `grep→peek` wins on freshness + precision + per-method `[start-end]` granularity (graphify has only file/function nodes — GAP.md:39). |
| **Cross-modal / LLM-inferred edges** (`semantically_similar_to`, pdf/image/audio) | **OUT** | Directly conflicts with the determinism thesis; LLM `calls` edges needed guards to drop phantoms from shared names (`render`/`parse`). Build *only* with explicit inferred-provenance, if ever. Scope law layer-3 territory at best. |
| **Heavyweight standing/stale graph as the agent's primary retrieval** | **OUT** | The frontier CLI agents (Claude Code, Codex, Aider) deliberately rejected indexes/graphs for agentic grep, citing exactly aOa's strengths. Don't chase the bet they lost. |
| **Automatic pattern DETECTION** ("this is 73% hexagonal") | **OUT (scope law)** | 30 years of research caps architecture-level detection at unusable precision (F1 0.09–0.70). aOa does **declare-and-diff** instead (conformance, §4), never detect. |
| **Full community detection (Leiden)** | **OPTIONAL TAIL — lowest leverage** | atlas 134-domain `@domain` enrichment (`enricher/atlas.go`) already covers much of the orientation need as a keyword→domain *classification* — a fair grouping substitute, but **not** topology-based community detection (a *different* thing, `research:49`). Resist building until Tier-1 lenses are solid. |

**The one-paragraph could-vs-should:** a knowledge graph *could* answer
everything from 1-hop neighbor lookups to LLM semantic similarity; it *should*
answer exactly the four transitive/topological classes grep can't — reachability,
affected-set/blast-radius, cycles/DSM, god_nodes — all REAL-derivable and
never-stale, and it should ship **nothing** for the 1-hop neighbor class where a
graph degrades into a worse, stale grep.

---

## 3. Access surface — native-first, MCP as a thin late adapter

**Recommendation (order is the recommendation): ① daemon-socket method →
② `aoa arch` CLI → ③ MCP adapter last.** Never replace fresh grep→peek with a
stale graph scan.

### 3.1 Why this order (falsifiable rationale)

- **G3 is binding precedent for native-first.** `GOALS.md:10` is literally
  "Agent-First — Drop-in shim for grep/egrep/find… Agents never know it's not GNU
  grep." The agent-access contract is *already won* via the shim; MCP is a second
  optional way to reach the same service, not the primary one.
- **The latency gap is real and one-directional.** The daemon answers over a unix
  socket with a flat JSON method switch — `socket/server.go:207 handleRequest`
  dispatches `MethodSearch`/`MethodPeek`/etc. with no handshake, no session, no
  JSON-RPC envelope (cases at `:208-227`, switch closes `:230`). Adding
  `case MethodArchDerive:` (spec method; `reach` is its CLI alias, ADR 2026-07-02) is a
  one-line addition that inherits the sub-ms read path G0 mandates. **MCP's stdio
  + JSON-RPC handshake/session overhead sits structurally *above* that** — you
  cannot make MCP faster than the socket it would wrap. The socket is the floor,
  so you build the floor first.
- **The market split confirms it** (`research:96-103`): CLI/agent tools chose
  grep; IDE tools chose indexes; `gh` CLI measured 7–32× cheaper than GitHub's
  MCP for bulk ops. aOa is CLI-first → aligned with the camp that won the
  CLI-agent segment.

### 3.2 So why build MCP at all? — and the direct answer to "MCP vs faster"

Because the *interface* bet is low-regret and rides a real wave — MCP the
protocol has won (OpenAI Mar'25, Google Apr'25, MS VS Code GA Jul'25, Linux
Foundation Dec'25; `research:93-95`), and hexagonal architecture makes the
adapter nearly free *later* (one more adapter beside socket/web, wrapping the
same `ArchQuerier` 1:1, zero duplicated logic). The discipline is **scope**: MCP
exposes *only* the §2.2 grep-beating queries — reachability, affected-set/PR-
blast-radius, orientation — and **never** a `query_graph` grep-replacement.

**The load-bearing synthesis (the actual answer to "is MCP a distraction, or is
building it faster the move?"):** *building MCP is not the faster move, and is not
meant to be — it is the **wider** move, late and thin.* Because MCP structurally
cannot beat the socket (§3.1), it is justified **ONLY** as a reach/compatibility
surface for MCP-only agents and IDEs — **never** as the path a latency-sensitive
agent takes. If an agent can shell out, it should use the CLI/socket; MCP is the
fallback for those that can't. "Faster" is the socket's job (it is the floor and
already exists in pattern); MCP buys *reach*, never *speed*. So MCP is correctly
scoped precisely when it never fronts a latency-sensitive query — that is the
single condition under which it is not a distraction.

| | CLI / socket (`aoa arch …`) | MCP server |
|---|---|---|
| Latency | daemon-socket sub-ms; process-exec ~ms | JSON-RPC over stdio + handshake/session overhead — structurally above the socket |
| Buys you | speed + freshness (the hot path) | reach (MCP-only agents/IDEs), never speed |
| Precedent | G3 "agent-first CLI" already proven via grep shim | new protocol surface to maintain |
| Reach | any agent that can shell out (all of them) | agents/IDEs with MCP support only |
| Architecture | new cobra commands + socket cases over the same app service | one more adapter beside socket/web — cheap *later* |
| When it's the right surface | always, for latency-sensitive retrieval | only as fallback for agents that cannot shell out |

Proposed command family (JSON to stdout, mirrors the shard contract exactly):

```
aoa arch views                      # catalog + status per view (live/mixed/declared/planned)
aoa arch view <id> [--scope p]      # one view's rendition JSON (= a shard)
aoa arch reach A B                  # reachability / shortest-path between two anchors
aoa arch blast <ref|PR>             # affected-set / PR blast-radius (graphify's best idea)
aoa arch findings [--new]           # findings, baseline-aware
aoa arch journey <id> | derive A B  # stored journey / focus-flow derivation
aoa arch facts <subject>            # raw facts + source pointers (the audit trail)
aoa arch pack <dd|pci|delta>        # evidence pack export (§7)
```

### 3.3 The verdict on "do grep/peek stay useful atop a KG?" — YES, they stay primary

This is the load-bearing question for the whole access surface. The honest case
both ways, then the landing:

- **Case that they're superseded:** once typed edges exist, "show me X and what
  it references" *could* be one graph hop instead of grep→peek. If the graph were
  always fresh and complete, the agent would prefer the structured answer.
- **Case that they stay primary (the stronger case):**
  1. **Freshness.** grep reads the live, fsnotify-reindexed index; any KG is a
     build artifact — between builds answers can be silently wrong. The frontier
     agents rejected the graph for exactly this.
  2. **Granularity.** peek's per-method `[start-end]` byte ranges operate *below*
     the grain a `unit`/`dep` graph models. The KG's coarsest node is a
     module/package (DSM n=modules ≤~50); peek delivers the actual method body.
     **Different altitudes, not competitors.**
  3. **Scope law forbids the rich edges that would threaten grep.** Tier-1
     refuses call/inheritance/LLM-semantic edges; the KG is deliberately narrow
     (imports + derived DSM/cycles). It was never going to answer the questions
     grep answers — it answers a different, smaller, graph-shaped set.

**Landing:** grep→peek is the **freshness-and-precision layer** and stays the
default verb; the KG is the **connectivity-and-orientation layer** that adds the
three verbs grep structurally can't form. aOa's edge is that *both layers read
one substrate* — so the agent's grep answer and its reachability answer cite the
*same* `file:line:commit`. The strongest market conclusion is **hybrid**
(structural query via graph, fallback to grep/file, beats either alone,
`research:107`); aOa is that hybrid by construction.

---

## 4. The language edge — honest ladder, not a flat "400+"

aOa's language reach genuinely beats graphify's 36-grammar single Python
codebase — but it **must be stated as a ladder**, because the tiers carry
different provenance. The architecture is three concentric rings (verified in
code):

| Ring | Count | Mechanism | Evidence |
|---|---|---|---|
| **A. Tuned (walker concept-resolution)** | **10** | Hand-verified per-language node-kind overrides (go, python, js, ts, tsx, rust, java, c, cpp, ruby) — these tune the *walker's* dimensions/concept resolution, NOT symbol extraction | `internal/domain/analyzer/lang_map.go:48-143` `langOverrides` — **ten map keys** (the count is grounded in the map literal, not in a test assertion); `SupportedLanguages()` returns those keys (`:173`) |
| **B. Default extraction** | **509** | Every compiled-in grammar gets `conceptDefaults` universal node-kinds; `Resolve()` falls back for any unlisted language | `languages_forest.go:5` literally says "Languages: 509"; walker is language-agnostic via `analyzer.IsNodeKind` (`walker.go:658-659`) |
| **C. Dispatch-reachable** | **400+ more / unbounded** | Dynamic `.so`/`.dylib` loader (purego), no recompile | `gen_forest.go` scans ~505 grammars; `--core`/`--lean` load dynamically with no compiled imports |

**Critical: two different tuning subsystems — do not conflate them.** The "10"
above tunes the *walker's* concept resolution (the dimensions engine), which is
a different code path from the one the keystone rides. Symbol — and therefore
import-edge — **extraction** lives in `parser.go`'s `extractSymbols`
(parser.go:235), whose switch has only **three** hand-written branches:
`extractGo` (parser.go:347), `extractPython` (:458), and `extractJavaScript`
(:532, shared by `javascript`/`typescript`/`tsx`); rust, java, c, cpp, ruby —
and everything else — fall through to `extractGeneric` (:250, driven by the
`symbolRules` table). So the two tiers are:

- **(a) walker concept-resolution tuning = 10** `langOverrides` — the dimensions
  engine, not the keystone's path.
- **(b) symbol/import EXTRACTION tuning = 3** hand-written extractors
  (`go`, `python`, `js`/`ts`/`tsx`) + `extractGeneric` for the rest.

**The keystone import edge inherits tier (b), the parser.go extraction tier — not
the walker's 10.** REAL-stamped import-edge parity is therefore initially
**Go / Python / JS-TS-TSX** (the three tuned extractors); every other language
gets best-effort `extractGeneric` extraction that works for common grammar
conventions but is unverified per-language. The provenance ladder keeps this
honest — the three tuned extractors emit REAL; generic extraction stamps the gap
rather than hiding it. This is also why claiming "509 languages of
architect-grade analysis" is doubly wrong: 509 is the *registration* count, and
even tuned *extraction* parity is three, not ten.

**Why this beats graphify (honest framing):**

| | graphify | aOa arch views |
|---|---|---|
| Languages | Python (+partial JS/Java), 36 grammars, hand-grouped | 3 tuned extractors (Go/Python/JS-TS-TSX) / 509 registered / reachable forest, one walker |
| Views | 1 import graph (+tree, callflow) | 16 architect-trusted standard types |
| Freshness | manual script run over stale `graph.json` | rides the live index — regenerates as you type |
| Evidence | none (LLM-guessed edges, force-directed grouping) | every edge carries file:line:commit (`aoa arch facts`) |
| Quality bar | *"eyeball"* | lint + **blind-judge gate** (answer the view's question from the image alone) |
| For AI agents | stale graph that must nudge agents off grep | CLI-first, sub-ms daemon reads, zero standing token cost |
| Cost to adopt | clone, pip, run | already inside the tool indexing your repo |

**State the edge as:** *3 languages with tuned, parity-verified symbol/import
extraction (Go, Python, JS-TS-TSX); 509 grammars compiled in with working
default extraction; a dynamic loader architected to reach the full ~500+ public
grammar forest without a rebuild* — broader and faster than graphify, kept
honest by the provenance stamp. The "10" is real but belongs to a *different*
subsystem (walker concept resolution), so do not borrow it for the extraction
claim. Do **not** claim "509 languages of architect-grade analysis," and do
**not** claim REAL import-edge parity beyond the three tuned extractors.

> **Stale-doc flag for the board (including this guide's own sources):**
> README.md:544 and CLAUDE.md both say "28-language structural analysis"; the
> scope law says "28 active languages"; and — flagged for honesty — **the
> draft's own primary integration spec carries it too: `00-OVERVIEW.md:81` says
> "28, one walker."** So the guide leans on `00-OVERVIEW.md` repeatedly as
> authority for the spine and the graphify comparison while silently
> contradicting that same doc's language count; this §4 ladder *is* the
> correction, and the reconciliation must reach back to `00-OVERVIEW.md:81`, not
> only to README/CLAUDE. The code shows **10** walker overrides, **3** tuned
> symbol extractors, and **509** registered grammars — the "28" figure is not
> grounded in `internal/domain/analyzer/lang_map.go` or `languages_forest.go` and
> appears legacy. Reconcile **all four sources** (README.md:544, CLAUDE.md,
> scope law, `00-OVERVIEW.md:81`) to the tiered reality (3 tuned extractors / 10
> walker overrides / 509 registered / reachable forest) **before** any external
> 400+ claim ships.

---

## 5. Integration touchpoints — six seams and the keystone that gates everything

Six pre-existing extension points. Five (1, 2, 4, 5, 6) are **additive to
existing machinery — zero architectural inversion.** The entire feature pivots on
**seam 3, the keystone** — and everything downstream inherits it for free.

### 5.1 The keystone — pinned precisely (the one gate)

The prior research says "the walker visits import nodes but never emits edges."
Grounding that against the code sharpens it into **two genuinely different
tree-sitter passes** — and the choice between them is the whole G0 question,
because only one of them is the always-on index-build pass.

**(a) The always-on index-build pass never visits imports at all — and this is
the keystone's home.** `ParseFileToMeta` (`parser.go:108`) calls **only**
`extractSymbols` (parser.go:104→235); it never invokes the walker. This is the
pass `internal/app/indexer.go:140` always runs to build the search index, and
it is the pass the watcher re-runs on every edit (§5.2 seam 6). `extractSymbols`
dispatches to `extractGo` (parser.go:347), `extractPython`, `extractJavaScript`,
or `extractGeneric` — and the Go extractor switches on exactly three node kinds
(`function_declaration`, `method_declaration`, `type_declaration`);
`import_declaration` is silently skipped. So on the always-on path, imports are
never seen — **no edge is constructed because no relation node is traversed.**

**(b) The dimensions pass DOES traverse imports — but it is a SEPARATE,
recon-gated walk, and it only COUNTS them.** `countImportSpecs`
(`internal/adapters/treesitter/walker.go:567-583`) walks
`import_spec_list`/`import_spec` children and returns `importCount int`,
discarding the package names. But `countImportSpecs` is reached *only* via
`walkContext.walk` (walker.go:54) — the **dimensions engine**, whose persistence
runs through `SaveAllDimensions` (`internal/app/dim_engine.go:200`). That is a
**different traversal from the index build**, and it is recon/dimensions-gated,
**not** part of the standard always-on index pass. This is the literal "visits
imports, emits no edges" site — but it is not the index pass.

**This distinction is the keystone, and the two sites are NOT interchangeable:**

- **Site (a) — `parser.go` `extractSymbols` — is the ONLY G0-free choice, and is
  the recommendation.** Add an `import_declaration` case to the per-language
  extractors (Go ~parser.go:347; the switch already iterates every top-level
  child) and emit `(importerFileID → importPath)` edge records alongside the
  symbols already produced at `indexer.go:142`. The edge is born **inside the
  existing, always-on `ParseFileToMeta` pass** — no second walk, no second file
  read — which is what keeps it G0-safe (≤+3% build budget, `00-OVERVIEW.md:99`)
  *and* keeps freshness free (§5.2: the watcher re-runs exactly this function).

- **Site (b) — `countImportSpecs` in the dimensions walk — is the richer but
  NOT-always-on option, and is the WRONG choice for the base product.** Choosing
  it means one of two failures: (i) the dimensions pass must run, which is
  recon-gated — **violating G2's "aoa must never depend on aoa-recon"** for the
  base binary — or (ii) a second walk on the index path, costing a second
  traversal G0 forbids. It also persists through `SaveAllDimensions`, the
  off-interface concrete shortcut flagged in §5.3 — so it is the G4-dirtier path
  too. Reserve site (b) only for a recon-gated *enrichment* tier, never for the
  always-on import edge.

**Do not collapse these into "either way, same pass."** Only site (a) is
genuinely always-on, G0-free, and G4-clean once an `EdgeStore` port exists
(§5.3). The "no second walk / freshness for free" guarantee attaches to **site
(a) alone**, not to either site.

**The hot-path cost site (a) inherits — and the storage-layout constraint it
forces (the G0-relevant write number).** Riding `onFileChanged` is what makes
freshness free, but it also inherits that callback's existing hot-path
characteristic: `onFileChanged` already does **two full-map linear scans of
`a.Index.Files` per edit** — one to find the changed file's existing ID
(`watcher.go:65 for id, fm := range a.Index.Files`) and one to allocate a new ID
(`watcher.go:110 for id := range a.Index.Files`). The per-file edge
upsert/delete the §5.2 table calls "nothing structural" is only nothing
structural **if the edge store is keyed for per-file deletion.** To re-emit one
file's edges, the watcher must first *delete* that file's outbound edges — and if
edges are stored as the flat `[]Edge` slice of §1, that delete is **O(all
edges)**: every keystroke-driven reindex would scan the entire estate's edge set.
That is the number that blows the ≤+3% G0 budget on a large estate, and it is on
the **write/invalidation** path, not the read path.

> **G0 constraint on the edge store (write path):** the edge store's *physical*
> layout must support per-file deletion in O(edges-for-that-file), not O(all
> edges) — e.g. `map[FileID][]Edge` (outbound-by-src) so `onFileChanged` drops
> and re-emits exactly one file's edges, with a separately maintained inbound
> index for reachability/blast reads. The flat `[]Edge` of §1 is the **logical
> shape** the rendition engine and agent see; it is **not** the storage layout.
> Stating this explicitly closes the one unbacked perf gap: the read cost is
> "O(edges) read; cache laid-out shard" (§6), and the **per-file invalidation
> cost is O(edges-for-that-file)** — both must hold for the keystone to stay
> inside G0.

**Reconciling with the locking law — the import edge is index data, not an "arch
write."** A reader red-teaming on G4 will reach for the locking law
(`00-OVERVIEW.md:101`: *"no arch/facts write ever holds App.mu; daemon-first
reads"*) and ask how the keystone can ride `App.mu` (it must — `onFileChanged`
holds `a.mu.Lock` at `watcher.go:43-44` and `ParseFileToMeta` runs inside that
locked section, `watcher.go:132`). The answer is a line the draft must draw, not
elide: there are **two distinct write classes**, and the locking law governs
only the second.

1. **The import-edge FACT is INDEX DATA.** It is produced by `extractSymbols`
   alongside the symbols already written there, and it rides the *existing*
   `SaveIndex`/index write — the same write that already holds `App.mu` today for
   every symbol. The keystone is **not a new "arch write"**; it is one more field
   on the index write that already runs under the lock. *This is exactly what
   makes freshness free* — the edge invalidates and re-derives on the same
   `onFileChanged` tick as every token, because it lives in the same write.
2. **Derived renditions and detector output are the "arch/facts writes" the
   locking law means** — laid-out shards, DSM matrices, cycle/SCC findings,
   conformance diffs, evidence packs. *These* never hold `App.mu`: they are
   computed off the hot path (compact-time detectors, §7) and served
   daemon-first as reads. The locking law keeps the **expensive derivations** off
   `App.mu`, not the cheap in-pass fact emission that is already index-write
   shaped.

So `00-OVERVIEW.md:101` and the keystone are *not* contradictory: the law says
"arch/facts **writes**" — meaning the derived layer — and the import edge is not
in that layer. (Wording reconciliation noted for the board: `00-OVERVIEW.md:101`
should read "no **derived** arch/facts write holds App.mu" to make the boundary
explicit and stop a careful reader from landing on a false contradiction.)

### 5.2 The six seams

| # | Seam | File:line | Net-new | Build/hex constraint |
|---|------|-----------|---------|----------------------|
| 1 | Socket method switch | `socket/server.go:225-248`; `protocol.go:39-48` | the six spec `MethodArch*` `case` arms + handlers (reach/blast = CLI-only aliases, ADR 2026-07-02); MCP rides as a sibling `case`/server calling the same handlers | Additive dispatch; delegate, don't reach into domain |
| 2 | Web route table | `web/server.go:77-113` | `GET /api/arch/*` (reuse `withETag`) + embed viewer at `GET /arch` (reuse `//go:embed`) | Localhost-only; build-neutral static bytes; `/api/recon*` is the precedent |
| 3 | **Index parse pass (KEYSTONE)** | `parser.go:104-108, 235-245, 347` (`extractSymbols`/`extractGo`); `indexer.go:138-159`; *(site b, NOT chosen: `walker.go:567-583`)* | **Emit edges from the import nodes in the always-on `extractSymbols` pass (site a)**; thread `[]Edge` through `ParseFileToMeta`; persist beside `Metadata`/`Tokens` **keyed for per-file deletion** | **Only build-cost seam**; site (a) is in-pass → ≤+3% G0 *and* G2-clean (no recon); **precondition: define `ports.EdgeStore` (or extend `Storage`) first**, and its layout must be keyed-by-file (not flat `[]Edge`) so seam 6's delete is O(edges-for-that-file) — adapter implements the port, never the reverse |
| 4 | bbolt buckets | `store.go:32-37, 98-177` | `keyEdges` in the existing `index` bucket (rides `SaveIndex`/`LoadIndex` — cheapest) **or** sibling `bucketArch` (mirrors `dimensions` replace-all lifecycle) | Additive key; `_version` byte → old DBs self-recover (recompute on next index); behind `ports.Store` |
| 5 | cobra surface | `root.go:32-52`; pattern `grammar_cgo.go:16-60` | `archCmd` + children (`reach`/`blast`/`view`/`facts`), structurally identical to the `grammar` parent/child | Thin delegate to daemon/App; mirror `_cgo`/`_nocgo` tags so `--light` degrades gracefully |
| 6 | fsnotify reindex | reindex callback `internal/app/watcher.go:20 onFileChanged` (serialized at `watcher.go:43 a.mu.Lock`; two full-map `a.Index.Files` scans at `:65` and `:110`; `ParseFileToMeta` runs inside at `:132`); wired at `app.go:698` | Per-file edge upsert/delete (mirror `removeFileFromIndex`) — **"nothing structural" only if the edge store is keyed-by-file** so the per-file delete is O(edges-for-that-file), not O(all edges) | The **edge FACT** rides `a.mu` exactly as the symbol write already does (it is index data, not a derived "arch write" — see §5.1 reconciliation); only **derived** renditions/detectors stay off `a.mu` per the locking law (`00-OVERVIEW.md:101`). Edges inherit freshness for free **because `onFileChanged` re-runs `ParseFileToMeta` — i.e. only if the keystone landed at site (a)**; the per-file invalidation must not add a full edge-set scan to this already-linear callback |

**The load-bearing fact (contingent on site (a)):** *because the keystone lands
at site (a)*, edges are emitted *inside* `ParseFileToMeta` (seam 3); a file edit
already re-runs that function via `onFileChanged` (seam 6, watcher.go:20) and
re-derives that file's edges — so **freshness, the exact axis graphify's
full-scan loses, is free** — *provided* the per-file edge delete stays
O(edges-for-that-file) (the keyed-by-file layout above), so the keystone does
not add a third full-map scan to a callback that already does two
(`watcher.go:65, :110`). This "freshness for free" is a *consequence of the
site-(a) choice*, not a property of either site: at site (b) the dimensions pass
has its own recon-gated update path (`dim_engine.go:222 updateDimForFile`) and
freshness would not be free. Storage, CLI, socket, and web all inherit the
keystone through machinery aOa already runs — provided the edge is born on the
always-on `ParseFileToMeta` path and stored keyed-by-file.

### 5.3 Two precedents worth exploiting (and a clean-architecture caveat)

- **`dimensions/` is the copy-paste template.** It was added as an independent
  transactional sub-bucket with a delete-then-recreate replace-all lifecycle
  (`store.go:461-484`); an `edges/`/`facts/` sibling drops in identically.
- **Caveat (flag under G4) — and it compounds the §5.1 site-(b) cost.**
  `SaveAllDimensions`/`LoadAllDimensions` live on the concrete `*Store`
  (`bbolt/store.go:461`/`:488`) but are **NOT declared in the `Storage`
  interface** (`storage.go:12-56`) — recon reaches them by concrete type,
  bypassing the port. This is exactly the path §5.1 site (b) would ride (the
  dimensions engine persists through `SaveAllDimensions`), so site (b) is not
  only a second pass / recon-gated (the §5.1 blocker) but **also** the
  G4-dirtier route — two independent reasons it is the wrong keystone home.
  Site (a) avoids both.
- **Precondition on the keystone task (not a caveat):** **define `ports.EdgeStore`
  (or extend `Storage`) first; the bbolt adapter then implements it.** The edge
  methods must enter through the port, never repeat the off-interface shortcut.
  This is a hard gate on seam 3, listed as a precondition in the §5.2 table.
  The port's contract must include a per-file delete (`DeleteEdgesForFile(FileID)`)
  so the keyed-by-file storage constraint (§5.1) is enforced at the interface,
  not left to the adapter.

---

## 6. View-by-view integration matrix

*(Preserved from the prior guide — the substrate above is what makes each row
real. Provenance ceiling = the honest maximum on an arbitrary repo. Effort:
S < 1d · M ≈ 2–4d · L ≈ 1–2wk. Phase: ① substrate mock ② keystone+minimum-lovable
GREEN views ②b remaining GREEN views ③ live estate ④ evidence/governance — see
§10 for the explicit ②/②b split.)*

The 16 standard views collapse onto **5 structural shard `kind`s** read by one
rendering engine (`view-standards.json:55-267`; `manifest.json`): `simple`
(context/container/dataflow/sequence/statemachine), `buckets`
(component/domains/deployment/trust), `entity` (datamodel/code), `table`
(glossary/techportfolio/sbom/cycles), `matrix` (dsm). **N questions, 5 layouts, 1
engine** — add a question that fits an existing shape and it renders for free.

| View | Facts needed | aOa source | Integration pattern | Perf notes | Ceiling | Effort | Phase |
|---|---|---|---|---|---|---|---|
| component | `unit`, `dep` | walker import queries (NEW — keystone) | emit during parse; group by path-prefix + atlas domain | O(edges) read; cache laid-out shard; **per-file invalidation O(edges-for-file)** | REAL (grouping MIXED) | M | **②** |
| dsm | same `dep` edges | derived from component facts | matrix rendition, zero new data | O(n²) render only, n=modules ≤ ~50 | REAL | S | **②** |
| cycles | same | Tarjan SCC over `dep` | findings pipeline entry | O(V+E), trivial at module grain | REAL | S | **②** |
| code (L4) | `unit` + symbols | **exists today** (SymbolMeta, peek) | render critical-path subset; agent picks subset | O(1) symbol reads | REAL (subset choice MIXED) | S | **②b** |
| techportfolio | manifests + `Language` per FileMeta | exists + lockfile parse (NEW, small) | table rendition; EOL/CVE joins external feeds later | parse-at-index-time | REAL | S | **③** |
| sbom | lockfiles | lockfile parser (NEW, syft-pattern) | table rendition; CycloneDX export | parse-at-index-time | REAL (unpinned → flagged) | M | **③** |
| datamodel | `schema` facts | ORM/DDL/migration tree-sitter queries (NEW per stack) | entity rendition; verbs inferred by agent | per-stack extractors, lazy | REAL fields / MIXED verbs | M | ③ |
| container | `deploy` facts | compose/k8s/Dockerfile parsers (NEW, config not code) | simple rendition | config parse trivial | REAL if IaC in repo, else MIXED | M | ③ |
| context | `dep` on external SDKs + env/config | SDK-dependency heuristics + agent naming | simple rendition; ALWAYS stamped MIXED | cheap | MIXED | M | ③ |
| domains | `unit` + atlas Domain per file | **enricher exists** | buckets rendition; agent names buckets, never adds | O(files) | MIXED (honest strength) | S | **②b** |
| dataflow | `route` + store/queue clients | source/sink tree-sitter queries + agent verbs | simple rendition | per-stack queries | MIXED | M | ③ |
| sequence | call-chain from entrypoint | symbol graph walk + agent narration | each step must cite a symbol or marked inferred | bounded depth walk | MIXED | L | ③ |
| statemachine | enum + transition writes | explicit-machine extractors (XState/Spring) else DECLARED | render extracted; declared otherwise | niche extractors | MIXED/DECLARED | L | ④ |
| trust | zone DECLARATIONS + `dep`/`dataflow` crossings | `.aoa/arch.yaml` declarations; detectors diff | conformance machinery (§7) | diff is O(edges) | DECLARED + REAL diff | M | ④ |
| glossary | term DECLARATIONS; atlas candidates | agent harvests candidates → human ratifies | table; MIXED until approved | — | DECLARED | S | ④ |
| API surface (NEW) | `route` facts | route tree-sitter queries (Spring/Express/Go mux…) + OpenAPI files | table+graph rendition | per-stack queries | REAL routes / MIXED consumers | M | ③ |
| ownership (NEW) | CODEOWNERS + git authorship | git adapter (churn exists) + CODEOWNERS parse | overlay + table | cheap | REAL | S | ③ |
| decision log (NEW) | `docs/adr/*.md` + git | repo scan; drift = ADR-touched-files × churn | table + drift findings | cheap | REAL | M | ④ |
| event catalog (NEW) | AsyncAPI/broker config + producer/consumer symbols | config parse + symbol query | table+graph | cheap | REAL/MIXED | M | ④ |
| estate landscape (NEW) | cross-repo `unit`+`dep` rollup | multi-root substrate union | scope-level rendition of existing facts | needs multi-project keying | REAL | M | ④ |

**Phase-② column reconciled with §10 (this is the fix for the prior
self-contradiction).** Phase ② is now the **minimum-lovable cut: exactly the
three views that share one edge set — `component` + `dsm` + `cycles`.** Those
three are the only views that fall out of the keystone import edge with zero
additional extractors (`dsm` and `cycles` are pure derivations of the same `dep`
set the `component` view emits), which is what makes them lovable on day one and
keeps ② honest at ~1–2 wk. `code` and `domains` are demoted to **②b** — both are
GREEN and cheap (S) but neither needs the keystone (`code` rides today's
SymbolMeta/peek; `domains` rides today's enricher), so they are *additive after*
the keystone proves out, not part of the minimum cut. `techportfolio` and `sbom`
are **demoted out of ② entirely to ③** (they were never edge-derived — they need a
NEW lockfile parser — so bundling them into the keystone phase was the source of
the stuffing the reviewer flagged). The §10 table and this matrix now encode the
same scope.

---

## 7. Patterns, findings, conformance, and the trust surfaces

- **Detectors run at substrate-compact time, not render time** — cycles, god
  (fan-in/out thresholds), orphans, dead-code candidates (`unit` with zero
  inbound `dep`/reference — always "candidate," reflection caveat stated).
  Findings are facts (`kind:finding`) with severity + source pointers; the viewer
  and the dock read them like any other rendition.
- **Conformance = declared template diffed against derived edges** (scope-law
  layer-3-vs-layer-1). Declaration lives in `{root}/.aoa/arch.yaml`: pattern name
  (layered | hexagonal | onion | custom) + role→path mapping. Output:
  convergent / divergent / **absent** edge classes through the findings pipeline.
  **Baseline/freeze** (ArchUnit pattern) stored in bbolt: report only NEW drift.
  This is the 17th view and the Sonar-validated market. Note: this is
  **declare-and-diff, never pattern DETECTION** (§2.3 OUT).
- **The findings dock** (`view-standards.json:44`) has three FIXED segments —
  VIEW (the derived caption: question · pass · source) | SELECTION (clicked
  element's stat + relations table, violations first) | FINDINGS (diagnostics).
  In-product, findings become facts emitted at compact-time and shipped in the
  shard as `findings:[{id,severity,text,anchors,rule,source}]`
  (`03-visualization.md:198-208`); the faulted estates carry the violation signal
  today (`supply/deployment.json` edge `tag:"cross-version"` → red-dashed + ⚠),
  making them **regression tests for findings rendering**.
- **Journeys & focus flow** are post-substrate renditions. Journey = stored or
  derived step list, each step anchored to `(scope, view, sel)` and selected on
  arrival so the dock shows its record (`bopis-click-to-curbside.json`); focus
  flow = k-shortest-path over `dep`+`dataflow` facts between two anchors. Path
  queries are O(E log V) at module grain — sub-ms for any sane estate. Anchors
  must resolve in **both** clean and faulted estate variants.

---

## 8. Governance & evidence packs

Renditions over the substrate, exported as self-contained documents (all Phase ④
— no new derivation, only assembly + export):
1. **DD exhibit set** — current-state views per system + findings scorecard +
   SBOM; every figure carries provenance + commit stamp (the anti-"seller diagram").
2. **PCI/SOC2 evidence bundle** — trust + dataflow views, asset inventory, SBOM;
   regenerated-on-change satisfies update-on-change obligations.
3. **"What changed since \<ref\>" pack** — `delta` facts between two commits,
   scoped to a focus area: affected closure, view diffs, new findings. Market
   white space; only credible from code. **This diff renderer (current-vs-future,
   SHA-snapshot edges) is the wedge graphify structurally cannot match** —
   derived from AST, free to compute, no doc to maintain. Worth elevating from
   "Phase ③ someday" to a named first-class target once the keystone lands
   (`research:174-176`).

---

## 9. Versus graphify (the honest competitive frame)

Graphify renders one import graph for one Python codebase, hand-grouped, over a
stale `graph.json`. After the keystone, aOa derives the same view across the
language ladder with provenance, findings, DSM/cycles, and an agent-queryable
substrate behind it — and graphify itself becomes just another estate in the
dropdown. **Calibration (don't overstate the threat):** graphify's "YC S26" badge
is unverified/self-applied (YC profile 404s; S26 not announced until Sept 2026);
the *project* is real (~69K stars, 36 grammars, Leiden, MCP). Treat traction as
real, the YC label as marketing. graphify's genuine value is **narrow** — only 3
of its 10 tools beat grep (reachability, affected-set/blast-radius, orientation),
and even those carry staleness risk its build-artifact architecture can't shed.
aOa's freshness + provenance + scale removes the ceiling at the keystone and
never reopens it.

---

## 10. Phase plan

| Phase | Deliverable | Proof | Size |
|---|---|---|---|
| ① substrate mock (playbook) | facts JSONL + snapshot substrate + renditions contract | touch-one-package demo; one journey derived from facts | days |
| **② keystone + minimum-lovable views (Go)** | import-edge facts from the parse pass (keyed-by-file store); **exactly `aoa arch view component/dsm/cycles`** (the three that share one edge set); `aoa arch facts`; the 3 detectors those views need (cycles/god/orphan); socket `arch.*`; **the cross-repo self-test invariant** | the three views on a **stranger's** repo passing the **same blind-judge gate**, REAL-stamped; edit one package → only affected shards change, with per-file edge invalidation O(edges-for-file) | **~1–2 wk** (honest now that ② is the three edge-derived views only) |
| **②b remaining GREEN views (no new edges)** | `aoa arch view code` (rides today's SymbolMeta/peek) + `aoa arch view domains` (rides today's enricher) | both pass the blind-judge gate; neither required keystone changes | ~3–5 d |
| ③ live estate | AMBER extractors (techportfolio, sbom, routes, schemas, deploy configs, ownership) + leashed-agent naming; viewer reads substrate via daemon; **diff renderer / blast-radius elevated** | real-repo PoC: clone → derive → judge → delete | ~2 wk |
| ④ governance | conformance view + baseline; evidence packs; **MCP adapter (thin, reachability/affected-set/orientation only — reach, not speed)**; remaining views | a DD exhibit pack generated end-to-end on a live repo | ~2–3 wk |

**Scope note on the ②/②b split (this resolves the prior table-vs-prose
contradiction).** The earlier draft's ② estimate (~1–2 wk) and its view
assignments (five views: component/dsm/cycles/code/domains, plus
techportfolio/sbom) could not both be true — a reviewer correctly flagged ~10
eng-wk if ② meant five-to-seven views. The fix is **scope, not estimate**: ② is
now *only* the three keystone-derived views that share one edge set
(component/dsm/cycles), which genuinely is ~1–2 wk; `code`/`domains` move to ②b
(cheap, GREEN, but keystone-independent); `techportfolio`/`sbom` move to ③ (they
need a new lockfile parser, never the keystone). §6's Phase column now matches
this table exactly.

**The single demo that proves the spine** (`research:151-154`, `00-OVERVIEW.md:59-61`):
clone a stranger's repo → `aoa init` → component/DSM/cycles render in the
viewer, REAL-stamped → edit one package → only the affected shards change. That
demo proves the two faces are one product. **Until it exists, aOa is a proven
face (the judged viewer) waiting for a substrate (the keystone edges).**

**Scope guard on every line above:** rendition of derived facts, or diff against
a declaration. Anything else gets a provenance stamp that says what it is — or it
doesn't ship.

---

## 11. Deep dives — where the exhaustive detail lives

This guide is the **single-entry readable position**: the thesis (§0), the
could-vs-should cut (§2), the access-surface verdict (§3), the language ladder
(§4), and the keystone (§5). Implementation-grade detail — every fact field,
every port signature, every per-language extractor, the byte-level shard
contract, and the per-task breakdowns — is deliberately pushed into the
companions below so this page stays the map, not the territory. Each companion is
plane-scoped and carries its own task list; read them in order when building, not
to understand the position.

| Companion | Plane | What it pins down (read it for) |
|---|---|---|
| `integration/00-OVERVIEW.md` | orientation | One-page integration map: what lands where, the parse-pass → index → renderer flow, why the result is graphify-plus. Start here before the per-plane specs. |
| `integration/01-facts-substrate.md` | **data plane** | The fact model, `ports.EdgeStore`/`Storage` signatures, **keystone import-edge extraction per language**, bbolt layout (keyed-by-file), incremental/delta derivation, performance gates, graphify parity+ (17 rows), 11 tasks. This is the home of §1/§5's substrate detail. |
| `integration/02-arch-service.md` | **service plane** | The `aoa arch` command family, the rendition engine (byte-compatible with the viewer's shard contract), detectors (cycle/god/orphan), conformance + baseline, derive A→B, agent guidance, 14 tasks. The home of §3/§7's service detail. |
| `integration/03-visualization.md` | **presentation plane** | Viewer extraction into the web adapter (fork-guard), the daemon data feed, the vendored bundle, governance surfaces, evidence-pack export, graphify keep/improve/drop, 13 tasks. The home of §0's two-faces contract and §8's packs. |
| `integration/04-review-findings.md` | pre-build review | The consolidated review pass over 01–03 before any code lands — the gate between "position agreed" and "phase ② green-lit". |
| `standards/MODEL-STANDARD.md` | quality law | The 3-step gate (lint → render → **blind judge**, `:43-53`) every view must pass. The falsifiable visual-acceptance test §0 point 2 leans on. |
| `standards/view-standards.json` | rendering law | The machine-readable shard `kind`s, label budgets, vital fields, and dock layout the rendition engine and viewer both obey. |

> **Companion not yet written (flagged, not faked):** the round-1→3 **red-team
> record** behind this guide's revisions currently lives inline (the revision
> summary at the foot of this doc). It should be curated into a standalone
> `integration/05-red-team-record.md` so the falsifiability trail is auditable
> separately from the position it produced; until that file exists, the revision
> summary below is the record.

---

## Appendix — falsifiable anchor index (red-team this list first)

| Claim | Anchor |
|---|---|
| Three node-maps, zero relations | `internal/ports/storage.go:59-63`; `Parent` proto-edge `:78` |
| Keystone site (a) = ONLY G0-free choice: always-on index pass, never visits imports | `ParseFileToMeta` `internal/adapters/treesitter/parser.go:104,108`→`extractSymbols:235-245`→`extractGo:347` (3 node kinds, no `import_declaration`) |
| Keystone site (b) = recon-gated dimensions walk, counts+discards (NOT chosen) | `walker.go:567-583 countImportSpecs`, reached only via `walkContext.walk` `walker.go:54` → `dim_engine.go:200 SaveAllDimensions` |
| Edge emission rides the always-on index pass (site a) | `internal/app/indexer.go:138-159` |
| Hot-path write cost the edge upsert inherits → store must be keyed-by-file (not flat `[]Edge`) | `onFileChanged` already does two full-map scans of `a.Index.Files` per edit: find-ID `internal/app/watcher.go:65`, alloc-ID `:110`; per-file edge delete must be O(edges-for-file), not O(all edges) |
| Flat socket method switch (MCP rides alongside) | `internal/adapters/socket/server.go:224-249` (cases `:226-245`); `protocol.go:39-48` |
| `dimensions/` bucket precedent + off-interface caveat (site-b compounder) | `internal/adapters/bbolt/store.go:32-37, 461 SaveAllDimensions / :488 LoadAllDimensions`; interface `storage.go:12-56` (neither declared) |
| Web ETag + embed precedent (`/api/recon*`) | `internal/adapters/web/server.go:77-113` |
| fsnotify → reindex (freshness for free, *iff site a*) | reindex callback `internal/app/watcher.go:20 onFileChanged` (serialized `:43-44 a.mu.Lock`, two full-map scans `:65`/`:110`, `ParseFileToMeta` inside at `:132`); wired `app.go:698` |
| Locking law governs DERIVED writes, not the in-pass index fact | `00-OVERVIEW.md:101` ("no arch/facts write holds App.mu") reconciled in §5.1: import edge is index data (rides `SaveIndex` under `a.mu`, like every symbol); only laid-out shards / DSM / cycles / conformance stay off `a.mu` |
| cobra parent/child precedent | `cmd/aoa/cmd/root.go:32-52`; `grammar_cgo.go:16-60` |
| Language ladder: walker tuning (10) ≠ extraction tuning (3) | walker: `internal/domain/analyzer/lang_map.go:48-143 langOverrides` — **ten map keys** (count from the literal, not a test); `SupportedLanguages()` returns those keys (`:172-174`); extraction: `parser.go:235-245 extractSymbols` (3: `extractGo`/`extractPython`/`extractJavaScript` + `extractGeneric`); `languages_forest.go:5` ("Languages: 509"); `gen_forest.go`, `loader.go` |
| graphify wins narrow (3 of 10 tools) | `serve.py:822` (shortest_path), `:862-932` (PR impact), `:775-792` (god/stats); anti-pattern `:144, :717-718, :415-418` |
| Blind-judge gate (the moat) | `playbook/standards/MODEL-STANDARD.md:43-53` ("### 3. Blind judge"; judge receives only screenshot/question/pass, no JSON, `:45-50`) |
| The 3-step gate process (lint → render → judge) | `playbook/standards/MODEL-STANDARD.md:18-54` (lint `:18-27`, render `:29-41`, judge `:43-53`) |
| Manifest → shard → viewer contract | `architecture-c4.html:32-37`; `manifest.json:24-37`; `03-visualization.md:108-124` |
| Member-grain readability risk (illustrative, simulated prov) | `manifest.json:366` (`8 buckets · 66 components · 10 edges`), `:369` (`prov: simulated`); deriver aggregates to bucket grain `03-visualization.md:298-305` |
| Provenance-pills are scaffolding, not headline UI | `00-OVERVIEW.md:63-66` |
| Provenance role splits by layer (audit-anchor on REAL edges; inference-leash on MIXED) | §2.1 productization note; ADR ladder `2026-06-11-core-competence-and-scope-line.md:23-31` |
| Scope law ladder + OUT list | `2026-06-11-core-competence-and-scope-line.md:23-58` (ADR leash text is "**never add a node**" at `:26`; "/edge" is the draft's derived corollary, §2.1 — not ADR words) |
| Stale "28" carried by the guide's own source | `00-OVERVIEW.md:81` ("28, one walker") — flagged alongside README.md:544 + CLAUDE.md; §4 ladder is the correction |
| Prior research verdict (sharpened, not relitigated) | `2026-06-19-graphify-plus-mcp-research.md` (esp. `:74-115, 160-184`) |

---

**Revision summary — round 3 (red-team findings, all verified against live code/source):**

- **[major] Hot-path write cost of the incremental edge upsert surfaced and bounded (§1, §5.1, §5.2 seams 3 & 6, §6 component row, appendix).** Verified `onFileChanged` already runs **two full-map linear scans of `a.Index.Files` per edit** (`watcher.go:65` find-ID, `:110` alloc-ID). Added the G0-relevant **write/invalidation** number the prior draft omitted: a flat `[]Edge` (§1) makes per-file edge delete **O(all edges)** on every keystroke. New constraint stated explicitly — `[]Edge` is the *logical* shape, the *storage* layout must be keyed-by-file (`map[FileID][]Edge` + inbound index) so per-file delete is O(edges-for-that-file); enforced at the `EdgeStore` port via a `DeleteEdgesForFile` contract (§5.3).
- **[minor] "MCP vs faster" answered directly (§3.2).** Added the load-bearing synthesis the prior draft left implicit: MCP is *not* the faster move and is not meant to be — it is the **wider** move (reach, not speed), late and thin. Closing sentence states MCP is justified ONLY as a reach/compatibility surface for agents that can't shell out, and must never front a latency-sensitive query. Added a "buys you / when it's the right surface" row to the comparison table.
- **[minor] Phase-② scope contradiction RESOLVED, not just flagged (§6, §10).** Made tables agree with prose by **scoping, not re-estimating**: ② is now exactly the three keystone-derived views that share one edge set (component/dsm/cycles, honestly ~1–2 wk); `code`/`domains` demoted to new phase **②b** (cheap, GREEN, keystone-independent); `techportfolio`/`sbom` demoted to **③** (need a new lockfile parser, never the keystone). §6's Phase column rewritten to match §10 exactly; added a §6 reconciliation note and a §10 scope note.
- **[minor] Provenance claim split by layer (§2.1 productization note, appendix).** On layer-1 REAL edges (the keystone) the `source` stamp is an **audit/freshness anchor** (powers `aoa arch facts` + re-derivation), explicitly *not* an inference leash — conceding a red-teamer is right that here it is closer to decoration. On layer-2 MIXED content the stamp **is** the load-bearing mechanism that makes agent naming/grouping safe to ship. Both kept, no longer conflated.
- **[minor] Blind-judge gate citation tightened (§0 point 2, appendix).** Corrected `MODEL-STANDARD.md:44-54` → `:43-53` (the "### 3. Blind judge" section; line 44 blank, content `:45-50`). The appendix's broader `:18-54` is retained but **relabeled** "the 3-step gate process (lint → render → judge)" since `:18-42` is lint+render, not the judge.
- **[housekeeping] Deep dives section added (§11).** Linked the plane-scoped companions (`integration/00-04`, `standards/`) and flagged the not-yet-written `05-red-team-record.md` honestly rather than citing a file that does not exist.

No code/source files were edited — markdown only, per the constraint. File:
`/home/corey/aOa-go/playbook/ENHANCEMENT-GUIDE.md`.
