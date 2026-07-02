# Knowledge Graph & Architecture Visualization — the Hand-in-Hand Deep Dive

**Status:** architectural position — DRAFT for green-light. No code changes
prescribed. **This is a falsifiable document:** every load-bearing claim cites a
`file:line` or a source doc; if a cited anchor is wrong, the claim built on it is
void.

**What this doc answers (the brief):**
1. What a knowledge graph buys aOa *beyond* the O(1) token index — a per-query
   verdict on which classes genuinely beat grep and which degrade into a worse,
   stale grep (§2).
2. Why mockup-style architecture visualization matters — the blind-judge moat no
   competitor renderer has (§3).
3. The mechanism by which the two reinforce each other: one substrate → an agent
   *query face* and a human *view face*, the viewer rendering exactly what the
   query answers, provenance as the shared spine (§1, §4).
4. Could-do vs should-do, run through the scope-law ladder (§2, §5).

**Binding law (read before relitigating anything here):**
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
  (the three-layer ladder: derive REAL / infer-leashed MIXED / declare-and-diff)
- **Goals** — `.context/GOALS.md` (G0 Speed, G2 Two-Binary split, G3 Agent-First,
  G4 Hexagonal — non-negotiable)
- **Rendering law** — `playbook/standards/view-standards.json` + quality gate
  `playbook/standards/MODEL-STANDARD.md`
- **Prior research verdict** — `.context/details/2026-06-19-graphify-plus-mcp-research.md`
  (treated as established; sharpened here, not relitigated)

**Sibling specs (implementation grade, per plane):** `integration/01-facts-substrate.md`
(data plane), `integration/02-arch-service.md` (service plane),
`integration/03-visualization.md` (presentation plane). This doc is the *why they
are one product*; those are the *how each plane is built*.

---

## 1. The spine — one substrate, two faces

The central claim: **a knowledge graph and the mockup-style architecture viewer
are not two features that happen to coexist — they are one product, because they
read one substrate.** The knowledge graph is that substrate's *machine face*; the
viewer is its *human face*. Build the substrate once and both faces light up;
build either face without the substrate and you have a demo, not a product.

This is forced, not retrofitted. The data contract already exists in the
playbook. The viewer fetches a content-hashed **manifest → shard** chain
(`mockups/architecture-c4.html:32-37`); each shard is a self-contained
`{buckets[], members[], edges[], findings[]}` JSON object for exactly one view.
**That same shard JSON is the agent's query result.** When `aoa arch view component`
returns a shard to an agent and the browser fetches the same shard to render, they
are reading one truth (`integration/00-OVERVIEW.md:90-94`: *"aOa maintains the
substrate… and the views, journeys, conformance checks are all renditions of it"*;
`integration/03-visualization.md`: the viewer *"consumes the same shard contract
unchanged"*).

### 1.1 What a graph adds that the index cannot express

aOa's entire persisted code model is `ports.Index` (`internal/ports/storage.go:59-63`):
three maps, all keyed off a compact `TokenRef{FileID, Line}`.

| Map | Holds | Source |
|-----|-------|--------|
| `Tokens` | token → locations (the O(1) search spine) | storage.go:60 |
| `Metadata` | location → symbol (`SymbolMeta`: name, signature, kind, `[start-end]`) | storage.go:61 |
| `Files` | fileID → file (carries `Language`, atlas `Domain`) | storage.go:62 |

**Every entry is NODE-shaped.** Each describes one thing or maps a name *to* a
thing. There is no map whose key and value are both symbols — **no relation/edge
is representable.** `SymbolMeta.Parent` (storage.go:78) is the single proto-edge:
method→receiver containment, but only as an un-indexed display *string*, not a
traversable reference.

So "what does a KG add beyond the O(1) index?" reduces to exactly one new shape:
**a typed relation `(src, kind, dst)`.** Minimal sibling to `Index`:

```
Edge = { Src TokenRef, Dst TokenRef|DstPath, Kind ("imports"|"contains"|…), prov }
```

It reuses the existing `TokenRef` identity as endpoints — no new node-identity
scheme — and `imports` is nearly free to derive (the import AST nodes are already
visited during parse; see `integration/01-facts-substrate.md` for the keystone).
This single shape is the *only* thing that unlocks the query classes grep
structurally can't form (§2). None are expressible against three node-maps; all
are trivial against an adjacency list.

> **Logical shape ≠ storage layout.** The flat `Edge` above is the *fact* — the
> triple the rendition engine and the agent see. It is **not** how the store must
> key edges on disk. Per-file invalidation on every keystroke (the watcher
> re-derives one file's edges per edit) forces the physical layout to be keyed for
> per-file deletion — `map[FileID][]Edge` (outbound-by-src) plus an inbound index
> for reachability — never a flat slice the watcher would linear-scan per edit.
> Logical shape: a triple set. Physical shape: keyed by source file. (G0 detail in
> `integration/01-facts-substrate.md`.)

### 1.2 The reinforcement runs both directions

The two faces are *mutually load-bearing*:

1. **The graph gives the viewer evidence-backed, leash-bounded, fresh content.**
   Every rendered pixel traces to a fact with a `source{file,line,commit}`
   pointer. The leash as the viewer spec states it
   (`integration/03-visualization.md:343`: *"agent may name/group, never add a
   node/edge"*) forbids the picture from saying anything the facts don't — the
   binding ADR text is narrower (*"never add a **node**"*,
   `2026-06-11-core-competence-and-scope-line.md:26`); "/edge" is the derived
   corollary (§2.1). graphify's LLM `semantically_similar_to` edges and
   force-directed physics are explicitly **dropped** for exactly this reason.

2. **The viewer's blind-judge gate gives the graph a falsifiable output-quality
   test.** The gate (`standards/MODEL-STANDARD.md:43-53`, the "Blind judge"
   section) hands a judge agent *only* a screenshot, the view's `question`, and the
   `pass` criterion — no JSON, no context (`:45-50`) — and asks "can you answer the
   question from the image alone?" A view rendered at member grain — e.g. a faulted
   component view at *66 components* (`mockups/archmodel/manifest.json:366`, an
   `8 buckets · 66 components · 10 edges` count) — risks failing that readability
   gate, which is exactly why the **deriver** aggregates to bucket grain *before* it
   renders (`integration/03-visualization.md:298-305`); that shard is itself
   `prov: simulated` (`manifest.json:369`), so it is the illustrative member-grain
   risk, not a recorded judge failure. **The visual bar reaches back and constrains
   how the graph is shaped** — readability disciplines the substrate, not just the
   CSS. No competitor renderer has this. graphify's bar is one word — *"eyeball"*
   (`integration/00-OVERVIEW.md:85`).

3. **Freshness is shared** — *provided the edges ride the always-on index pass.*
   Both faces then ride the live index: fsnotify → reindex → revision bump; the
   agent's sub-ms socket read and the human's ETag-polled auto-refreshing canvas
   are fed by the *same* invalidation event (`integration/03-visualization.md:139-148`).
   graphify is a hand-run script over a stale `graph.json` build artifact
   (`repo/graphify/serve.py` `_load_graph`, mtime-poll); aOa's two faces stay fresh
   together because they share one substrate and one freshness signal. (This is a
   *consequence* of the keystone landing on the always-on parse pass, not an
   automatic property of any graph — see `integration/01-facts-substrate.md`.)

**The spine, stated once:** *aOa turns any codebase into a deterministic,
provenance-stamped fact substrate the instant you run `aoa init`, and serves it
through one contract to both an agent (CLI/socket/MCP, sub-ms, file:line:commit on
every answer) and a human (architect-trusted views that pass a blind-readability
gate) — so the diagram and the agent's answer are the same truth, never a
seller's drawing* (`2026-06-19-graphify-plus-mcp-research.md:180-184`).

---

## 2. Could do vs should do — the scope line over the graph

A graph is technically capable of an enormous query surface. The scope law and
G0–G6 cut that surface to what aOa **should** build. The cut runs per query
class, each citing graphify's own implementation as the falsifiable reference for
"what this would actually be."

### 2.1 The provenance ladder is the cut

The scope-law ladder (`2026-06-11-core-competence-and-scope-line.md:23-31`) is the
gate every proposed edge passes through:

| Layer | What it is | Move | Stamp |
|---|---|---|---|
| 1 | What the code **IS** — imports, structure, configs, churn | DERIVE (tree-sitter, git, manifests) | **REAL** |
| 2 | What the code **MEANS** — names, groupings, domains, verbs | INFER, **leashed**: agent may name/group/annotate facts, **never add a node** (ADR `:26`, verbatim) | **MIXED** |
| 3 | What was **INTENDED** / what the world does — trust, runtime | DECLARE or INGEST, then **diff layer 3 against layer 1** | **DECLARED / SIMULATED / OBSERVED** |

> **Corollary (the draft's own, not the ADR's words):** the ADR leash says "never
> add a node." Edges are layer-1 REAL facts (derived structure), so an agent
> *adding an edge* would be inventing structure the code does not contain — out of
> bounds for the same reason a node is. We carry the leash as "never add a node **or
> edge**," but that "/edge" is our derived corollary; the binding ADR text says only
> "node" (`:26`).

> **Provenance plays two distinct roles — do not conflate them**
> (`integration/00-OVERVIEW.md:63-66`: the REAL/MIXED pills were scaffolding to pin
> the format; the derive/infer discipline stays as engineering law, the pills do not
> ship as headline UI):
> - **On layer-1 REAL edges (the keystone import edge), the `source` pointer is an
>   audit/freshness anchor, not an inference leash.** There is no inference to
>   discipline — the edge is deterministic AST output — so the stamp's job is to make
>   the fact *auditable and re-derivable*: it powers `aoa arch facts`, records the
>   commit the edge was last derived at, and lets the substrate recompute on change.
>   On this row a red-teamer is right that provenance is *closer to* decoration than
>   safety — its value is freshness and audit.
> - **On layer-2 MIXED content (grouping/naming/verb inference), the stamp is the
>   load-bearing mechanism that makes inference safe to ship.** Here an agent *is*
>   adding meaning on top of extracted facts; the stamp pins each named bucket or
>   inferred verb back to the REAL facts it sits on, so the leash is checkable. This
>   is the row where "provenance makes inference safe to ship" is literally true.
>
> Keep both; don't claim the second role for the first.

### 2.2 Per-query verdict — SHOULD BUILD

These four classes are *transitive-closure or global-topology shaped*. Each asks
about the structure **between** nodes, which a token→location index cannot
represent and which agentic grep can only fake through unbounded recursive
round-trips (and cannot prove a negative).

| Query class | Verdict | Why grep structurally can't | graphify evidence |
|---|---|---|---|
| **Reachability / `shortest_path`** ("how does A connect to B?") | **BUILD** | A transitive-closure question; answer length unbounded a priori. An agent simulates it only by recursively grepping each callee N hops deep, re-deciding the frontier each time — and **cannot prove "no path exists."** | `serve.py:822` `nx.shortest_path`, "No path found" `:823-824` |
| **Affected-set / reverse-deps / PR blast-radius** ("what breaks if I change X / which PRs collide?") | **BUILD — ELEVATE (graphify's single best idea)** | grep finds *forward* literal occurrences. Reverse transitive dependency, and *set-intersection across changesets*, are edge-closure operations grep has no notion of. **Cheap, and never stale because the PR file-list is a git feature.** | `get_pr_impact`/`triage_prs` `serve.py:862-932` |
| **Cycles / DSM** ("circular deps / layering violations?") | **BUILD (Tier-1, mechanical)** | A cycle `A→B→C→A` is a topological property — no string to match; it exists only in the edge structure. DSM *is* the adjacency matrix. graphify does **not** expose these (`serve.py:564-684` lists 10 tools, no cycles/dsm) — aOa-native opportunity. | aOa-native; SCC/Tarjan over the dep edge set |
| **Architecture orientation — `god_nodes`** ("most-connected abstractions?") | **BUILD god_nodes; community detection OPTIONAL** | Degree-centrality is a global graph property; grep counts textual occurrences, not structural in/out-degree. | `god_nodes` `serve.py:775-780`, `graph_stats` `:782-792` |

**Freshness is the decisive asymmetry on every "build" row.** In graphify these
ride a stale `graph.json`; aOa derives the same edges from the AST inside the
live, fsnotify-reindexed parse pass — so the grep-can't classes become
**graph-capable AND never-stale**, capturing graphify's narrow real value while
eliminating its structural defect.

### 2.3 Per-query verdict — DO NOT BUILD (explicitly OUT)

| Anti-pattern | Verdict | Why |
|---|---|---|
| **`query_graph` / `get_node` / `get_neighbors`** (1-hop "show me X and its neighbors") | **DO NOT BUILD — the stale-grep trap** | graphify's seed step is an **un-indexed full node scan** (`_score_nodes serve.py:144`, `get_node` linear scan `:717-718`) over a snapshot only as fresh as the last build — a slower grep over a stale graph. **The tell:** graphify's own output must *nudge agents off grep* (`serve.py:415-418`, `:670-675`). For "show this function and what it calls," fresh `grep→peek` wins on freshness + precision + per-method `[start-end]` granularity (graphify has only file/function nodes — `repo/GAP.md:39`). |
| **Cross-modal / LLM-inferred edges** (`semantically_similar_to`, pdf/image/audio) | **OUT** | Conflicts with the determinism thesis; LLM `calls` edges needed guards to drop phantoms from shared names. Scope-law layer-3 at best, with explicit inferred-provenance if ever. |
| **Heavyweight standing/stale graph as the agent's primary retrieval** | **OUT** | Frontier CLI agents (Claude Code, Codex, Aider) deliberately rejected indexes/graphs for agentic grep, citing exactly aOa's strengths. Don't chase the bet they lost. |
| **Automatic pattern DETECTION** ("this is 73% hexagonal") | **OUT (scope law)** | 30 years of research caps architecture-level detection at unusable precision (F1 0.09–0.70). aOa does **declare-and-diff** (conformance), never detect. |
| **Full community detection (Leiden)** | **OPTIONAL TAIL — lowest leverage** | atlas 134-domain `@domain` enrichment (`enricher/atlas.go`) already covers much orientation need as keyword→domain *classification* — a fair grouping substitute, but **not** topology-based community detection (a *different* thing). Resist building until Tier-1 lenses are solid. |

**The one-paragraph could-vs-should:** a graph *could* answer everything from
1-hop neighbor lookups to LLM semantic similarity; it *should* answer exactly the
four transitive/topological classes grep can't — reachability,
affected-set/blast-radius, cycles/DSM, god_nodes — all REAL-derivable and
never-stale, and it should ship **nothing** for the 1-hop neighbor class where a
graph degrades into a worse, stale grep.

### 2.4 Do grep/peek stay useful atop a KG? — YES, they stay primary

The honest case both ways, then the landing.

- **Case they're superseded:** once typed edges exist, "show me X and what it
  references" *could* be one graph hop. If the graph were always fresh and
  complete, the agent would prefer the structured answer.
- **Case they stay primary (stronger):**
  1. **Freshness.** grep reads the live, fsnotify-reindexed index; any KG is a
     build artifact — between builds, answers can be silently wrong. The frontier
     agents rejected the graph for exactly this.
  2. **Granularity.** peek's per-method `[start-end]` byte ranges operate *below*
     the grain a `unit`/`dep` graph models. The KG's coarsest node is a
     module/package (DSM n=modules ≤ ~50); peek delivers the actual method body.
     **Different altitudes, not competitors.**
  3. **Scope law forbids the rich edges that would threaten grep.** Tier-1 refuses
     call/inheritance/LLM-semantic edges; the KG is deliberately narrow (imports +
     derived DSM/cycles). It was never going to answer the questions grep answers.

**Landing:** grep→peek is the **freshness-and-precision layer** and stays the
default verb; the KG is the **connectivity-and-orientation layer** that adds the
three verbs grep structurally can't form. aOa's edge is that *both layers read one
substrate* — so the agent's grep answer and its reachability answer cite the
*same* `file:line:commit`. The strongest market conclusion is **hybrid**
(structural query via graph, fallback to grep/file, beats either alone); aOa is
that hybrid by construction.

---

## 3. Why the visualization matters — the blind-judge moat

The viewer is not a nicety on top of the graph; it is the face that makes the
substrate *trustworthy to a human* and the only place a falsifiable quality bar
can live.

### 3.1 The moat: a falsifiable visual acceptance test

Every architecture diagram tool on the market — graphify included — has the same
quality bar: it renders, and a human eyeballs it
(`integration/00-OVERVIEW.md:85`). aOa's viewer is gated by a three-step process
(`standards/MODEL-STANDARD.md:18-53`): **lint** the label/field budget (`:18-27`),
**render** to a screenshot (`:29-41`), then the **blind judge** (`:43-53`) — a
judge agent receives *only* the screenshot, the view's `question`, and the `pass`
criterion, with no JSON and no context (`:45-50`), and must answer the question
from the image alone.

This is a moat for two reasons:
1. **It is automatable and falsifiable.** "Eyeball" is not a CI gate; "can a blind
   agent answer the view's question from the pixels?" is. No competitor renderer
   has this.
2. **It disciplines the substrate, not just the CSS.** As §1.2 point 2 shows, a
   member-grain view (66 components, `manifest.json:366`) risks failing the gate, so
   the deriver aggregates to bucket grain *before* render
   (`integration/03-visualization.md:298-305`). The readability requirement reaches
   back into how the graph is shaped — the picture's quality bar constrains the
   data, not the other way round.

### 3.2 Why mockup-style (architect-trusted) views, not a force-directed blob

graphify renders one import graph with force-directed physics — a hairball that
looks impressive and answers nothing specific. The mockups
(`mockups/archmodel/manifest.json`, `mockups/blueprint-viewer.html`) are instead a
fixed catalog of **architect-trusted view types** (C4 context/container/component,
DSM, dataflow, sequence, trust, …), each with a *question it must answer* and a
*pass criterion*. The five structural shard `kind`s (`view-standards.json`) —
`simple`, `buckets`, `entity`, `table`, `matrix` — collapse N questions onto one
rendering engine: **add a question that fits an existing shape and it renders for
free.** That is the inverse of a blob: every view earns its place by passing the
gate, and the substrate is shaped to make views passable.

### 3.3 The findings dock — the diagram carries its own diagnostics

The viewer's findings dock (`view-standards.json:44`) has three fixed segments —
VIEW (the derived caption: question · pass · source), SELECTION (clicked element's
stats + relations, violations first), FINDINGS (diagnostics). Findings are facts
(`kind:finding`) emitted at substrate-compact time and shipped in the shard
(`integration/03-visualization.md:198-208`); the faulted mockup estates already
carry the violation signal (`supply/deployment.json` edge `tag:"cross-version"` →
red-dashed + ⚠), making them **regression tests for findings rendering.** The
human face doesn't just draw the graph — it draws the graph *and what's wrong with
it*, both traced to source.

---

## 4. The mechanism of reinforcement — provenance as the shared spine

This section names, precisely, *how* the two faces reinforce rather than merely
coexist.

1. **One substrate, two read paths, one contract.** The agent calls
   `aoa arch view component`; the browser fetches the same shard. Both deserialize
   `{buckets, members, edges, findings}`. There is no second data model, no
   translation layer — the query result *is* the render input
   (`integration/00-OVERVIEW.md:90-94`).
2. **Provenance is the spine both faces hang off.** Every fact carries
   `source{file,line,commit}`. The agent surfaces it as the audit trail
   (`aoa arch facts <subject>` returns raw facts + pointers); the viewer surfaces it
   as the SELECTION/VIEW caption and the findings anchors. **Same pointer, two
   presentations** — so when an agent reasons over a reachability path and a human
   inspects the same path in the canvas, they are pointing at identical
   `file:line:commit`. This is what makes "the diagram and the agent's answer are
   the same truth" literally true rather than aspirational.
3. **The viewer renders exactly what the query answers.** A reachability query
   returns a path; the viewer's focus-flow renders that path. A cycles detector
   emits SCCs; the DSM/cycles view renders them. A blast-radius query returns an
   affected set; the viewer highlights that set. The viewer is not a separate
   visualization of the codebase — it is the *rendering of the query layer's
   outputs*. Add a query class, and (if it fits one of the five shard kinds) the
   viewer shows it for free.
4. **Freshness flows to both at once.** One fsnotify event → one reindex → one
   revision bump → both the socket read and the ETag-polled canvas invalidate
   together (`integration/03-visualization.md:139-148`). Neither face can drift from
   the other, because there is only one freshness signal.

The reinforcement is therefore *structural*: the graph gives the viewer
evidence-backed fresh content; the viewer's gate gives the graph a quality bar;
provenance lets both cite the same truth; and one freshness signal keeps them in
lockstep. Remove the substrate and neither face has anything to read; remove
either face and the substrate is half-used.

---

## 5. The could-vs-should ladder, applied — the scope guard

Pulling §2 and §3 together against the scope law gives the binding ladder for what
ships:

| Tier | What | Provenance | Verdict |
|---|---|---|---|
| **DERIVE (REAL)** | import edges → component/DSM/cycles views; reachability; affected-set/blast-radius; god_nodes; ownership (CODEOWNERS+git); techportfolio (`Language`+lockfiles) | REAL, `file:line:commit` | **BUILD** — these are the keystone and its direct derivations |
| **INFER, leashed (MIXED)** | bucket naming, domain grouping (atlas), dataflow verbs, context-map naming | MIXED, stamped, agent may name/group **never add a node** | **BUILD with the leash** — every named element pins back to a REAL fact |
| **DECLARE & DIFF (layer-3 vs layer-1)** | conformance (declared pattern in `.aoa/arch.yaml` diffed against derived edges), trust zones, glossary | DECLARED + REAL diff | **BUILD as declare-and-diff** — never as detection |
| **OUT** | 1-hop `query_graph` retrieval, cross-modal/LLM edges, automatic pattern detection, heavyweight standing graph, full Leiden community detection | — | **DO NOT BUILD** (§2.3) |

**The scope guard on every line:** rendition of derived facts, or diff against a
declaration. Anything else gets a provenance stamp that says what it is — or it
doesn't ship.

**The one demo that proves the spine** (`2026-06-19-graphify-plus-mcp-research.md:151-154`,
`integration/00-OVERVIEW.md:59-61`): clone a stranger's repo → `aoa init` →
component/DSM/cycles render in the viewer, REAL-stamped → edit one package → only
the affected shards change. That demo proves the two faces are one product. **Until
it exists, aOa is a proven face (the judged viewer) waiting for a substrate (the
keystone edges).** The keystone itself — emitting import edges inside the always-on
parse pass, G0-gated at ≤+3% build — is specified in `integration/01-facts-substrate.md`;
this doc establishes *why* closing that one gap lights up both faces at once.

---

## Appendix — falsifiable anchor index (red-team this first)

| Claim | Anchor |
|---|---|
| Three node-maps, zero relations | `internal/ports/storage.go:59-63`; `Parent` proto-edge `:78` |
| KG adds exactly one shape: a typed relation | derived from storage.go (no map with symbol→symbol) |
| Manifest → shard → viewer contract (= agent query result) | `mockups/architecture-c4.html:32-37`; `integration/00-OVERVIEW.md:90-94`; `integration/03-visualization.md` |
| Leash: agent may name/group, never add a node | ADR `2026-06-11-core-competence-and-scope-line.md:26` ("never add a node"); "/edge" is the §2.1 corollary, carried by `integration/03-visualization.md:343` |
| Scope-law ladder (REAL/MIXED/DECLARED) | `2026-06-11-core-competence-and-scope-line.md:23-31` |
| Provenance pills are scaffolding, not headline UI | `integration/00-OVERVIEW.md:63-66` |
| Blind-judge gate (the moat) | `standards/MODEL-STANDARD.md:43-53` (judge gets only screenshot/question/pass, no JSON, `:45-50`) |
| 3-step gate (lint → render → judge) | `standards/MODEL-STANDARD.md:18-53` (lint `:18-27`, render `:29-41`, judge `:43-53`) |
| Member-grain readability risk (illustrative, simulated prov) | `mockups/archmodel/manifest.json:366` (`8 buckets · 66 components · 10 edges`), `:369` (`prov: simulated`); deriver aggregates `integration/03-visualization.md:298-305` |
| Findings dock + faulted regression signal | `view-standards.json:44`; `integration/03-visualization.md:198-208`; `supply/deployment.json` `tag:"cross-version"` |
| graphify wins narrow (3 of 10 tools) | `serve.py:822` (shortest_path), `:862-932` (PR impact), `:775-792` (god/stats); anti-pattern `:144`, `:717-718`, `:415-418`, `:670-675`; file/function-only nodes `GAP.md:39` |
| graphify "eyeball" bar | `integration/00-OVERVIEW.md:85` |
| Prior research verdict + spine sentence | `2026-06-19-graphify-plus-mcp-research.md` (esp. `:160-184`, `:180-184`) |
| Keystone + G0 budget (specified elsewhere) | `integration/01-facts-substrate.md`; `integration/00-OVERVIEW.md:99` (≤+3%) |
