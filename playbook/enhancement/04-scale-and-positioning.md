# 04 — Scale & Positioning: the honest 400+ story and the wedge graphify can't match

**Status:** positioning deep-dive — DRAFT companion to `../ENHANCEMENT-GUIDE.md`.
No code changes prescribed. **This is a falsifiable document:** every
load-bearing claim cites a `file:line` or a source doc; if a cited anchor is
wrong, the claim built on it is void. This file zooms into one slice of the
guide — **scale, freshness, provenance, and the competitive frame** — and exists
to be the answer when someone asks *"is the 400+-language claim real, and is aOa
actually faster/better than graphify, or is that marketing?"*

**Binding law (do not relitigate here):**
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md` (three-layer ladder; leash text at `:26` is "**NEVER add a node**")
- **Goals** — `.context/GOALS.md` (G0 Speed, G2 Two-Binary, G3 Agent-First, G4 Hexagonal)
- **Prior research verdict** — `.context/details/2026-06-19-graphify-plus-mcp-research.md` (established; sharpened, not relitigated)
- **Sibling deep-dives** — `../ENHANCEMENT-GUIDE.md` (the full position), `../integration/00-OVERVIEW.md` (the integration map)

**The one-line position this document substantiates:** *faster (sub-ms socket
reads on a live, fsnotify-reindexed index), better (per-method granularity +
file:line:commit provenance on every answer + a falsifiable blind-judge quality
gate), and our own thing (the current-vs-future diff renderer graphify's
build-artifact architecture structurally cannot produce) — and the honest
language story is a ladder, not a flat "400+".*

---

## 1. The honest language ladder — 3 tuned, 10 walker-tuned, 509 registered, forest reachable

The single most abused number in this space is the language count. graphify says
"36 grammars"; the lazy aOa pitch says "509 languages" or "400+". Both are
misleading the same way: **registration is not extraction is not parity.** State
the reach as a ladder, because each rung carries a different provenance and a
different honesty obligation.

| Rung | Count | What it actually means | Evidence |
|---|---|---|---|
| **A — symbol/import EXTRACTION tuned (parity-grade)** | **3** | Hand-written extractors that emit symbols (and, post-keystone, import edges) the parity fixtures verify: `extractGo`, `extractPython`, `extractJavaScript` (shared by `javascript`/`typescript`/`tsx`) | `internal/adapters/treesitter/parser.go:235 extractSymbols` switch → `:347 extractGo`, `:458 extractPython`, `:532 extractJavaScript`; everything else falls to `:250 extractGeneric` |
| **B — walker concept-resolution tuned** | **10** | Per-language node-kind overrides that tune the *walker's* dimensions/concept engine (NOT symbol extraction): go, python, javascript, typescript, tsx, rust, java, c, cpp, ruby | `internal/domain/analyzer/lang_map.go:48 langOverrides` — **ten map keys** (count grounded in the map literal, not a test assertion); `SupportedLanguages()` returns those keys (`:172-174`) |
| **C — grammars REGISTERED (compiled-in, default extraction)** | **509** | Every compiled-in grammar gets `conceptDefaults` universal node-kinds and falls through to `extractGeneric` — working best-effort extraction, unverified per language | `internal/adapters/treesitter/languages_forest.go:5` literally reads `// Languages: 509` |
| **D — grammars REACHABLE (dynamic, no recompile)** | **~500+ / forest** | Dynamic `.so`/`.dylib` loader (purego) over the public tree-sitter grammar forest; `--core`/`--lean` builds load grammars dynamically with no compiled imports | `gen_forest.go` scans ~505 grammars; dispatcher architecture documented in `../ENHANCEMENT-GUIDE.md §4` |

**Two tuning subsystems — never conflate them.** The "10" (rung B) tunes the
*walker's* concept resolution — the dimensions engine, a different code path
entirely. The keystone import edge does **not** ride the walker; it rides
`parser.go`'s `extractSymbols` (rung A). So:

- **Walker concept-resolution tuning = 10** (`lang_map.go:48`) — the dimensions engine.
- **Symbol/import EXTRACTION tuning = 3** hand-written extractors + `extractGeneric` for the rest.

**The keystone inherits rung A, the 3-extractor reality — not the 10.**
REAL-stamped import-edge parity is therefore initially **Go / Python /
JS-TS-TSX**. Every other language gets best-effort `extractGeneric` extraction
that works for common grammar conventions but is unverified per-language. The
provenance stamp keeps this honest: the three tuned extractors emit REAL;
generic extraction stamps the gap rather than hiding it.

**How to state the edge externally (the honest sentence):** *3 languages with
tuned, parity-verified symbol/import extraction (Go, Python, JS-TS-TSX); 509
grammars compiled in with working default extraction; a dynamic loader
architected to reach the full ~500+ public grammar forest without a rebuild* —
broader and faster than graphify's 36, kept honest by the provenance stamp.

**What you may NOT claim:**
- ❌ "509 languages of architect-grade analysis" — 509 is the *registration*
  count; even tuned *extraction* parity is three, not ten.
- ❌ "10 languages of tuned import extraction" — the 10 belongs to the *walker*
  subsystem, not the extraction path; borrowing it for the keystone is the
  round-1 error this ladder corrects.
- ❌ REAL import-edge parity beyond Go/Python/JS-TS-TSX before the per-language
  extractors are verified against fixtures.

> **Stale-doc flag for the board.** `README.md:544`, `CLAUDE.md`, the scope law,
> **and the integration spec's own `../integration/00-OVERVIEW.md:81`** all say
> "28 active languages" / "28, one walker." The "28" figure is not grounded in
> `lang_map.go` (10) or `languages_forest.go` (509) and appears legacy. This §1
> ladder *is* the correction; reconcile **all four sources** to the tiered
> reality (3 tuned extractors / 10 walker overrides / 509 registered / forest
> reachable) **before** any external 400+ claim ships. Note the guide leans on
> `00-OVERVIEW.md` as authority for the spine while that same doc carries the
> figure being corrected — the reconciliation must reach back to it explicitly.

---

## 2. Faster — the latency floor graphify cannot reach

"Faster" is not a vibe; it is a structural property of where the answer comes
from. There are three latency tiers, and aOa and graphify sit on different ones.

| | graphify | aOa |
|---|---|---|
| Answer source | `nx.DiGraph` loaded from a `graph.json` **build artifact**, mtime/size-polled on call (`serve.py:19 _load_graph`) | live `ports.Index` + edge store, fed by the always-on parse pass |
| Seed step | **un-indexed full node scan** (`serve.py:144 for nid,data in G.nodes()`; `get_node` linear scan `:717-718`) | O(1) token→location lookup (`storage.go:60 Tokens map`) |
| Transport | MCP stdio + JSON-RPC handshake/session | unix-socket flat method switch, no handshake, no envelope (`socket/server.go:206 handleRequest`, cases `:208+`) |
| Read latency | bounded below by JSON-RPC over stdio | daemon-socket **sub-ms**; process-exec ~ms |
| Freshness | stale between builds — answers can be silently wrong | fsnotify → `onFileChanged` reindex → revision bump (`watcher.go:20`) |

**The decisive asymmetry: freshness is the axis graphify structurally loses.**
graphify's core retrieval (`query_graph`/`get_node`/`get_neighbors`) is an
un-indexed full scan over a snapshot only as fresh as the last manual build —
literally a slower grep over a stale graph. The tell is that graphify's own
output has to *nudge agents off grep* (`serve.py:415-418`, `:670-675`). The
frontier CLI agents (Claude Code, Codex, Aider) rejected exactly this
standing-graph retrieval for agentic grep, citing freshness/staleness/cost —
which are aOa's strengths
(`2026-06-19-graphify-plus-mcp-research.md:96-103`).

**aOa is faster on both axes that matter:** the read floor (sub-ms socket vs
JSON-RPC-over-stdio that structurally cannot beat it), and the freshness floor
(live reindex vs a hand-run script over a stale artifact). On the grep-beating
query classes — reachability, affected-set/blast-radius, orientation — aOa
derives the same edges from the AST inside the live parse pass, so those classes
become **graph-capable AND never-stale**, capturing graphify's real (narrow)
value while eliminating its structural defect.

---

## 3. Better — granularity + provenance + a falsifiable quality gate

"Better" decomposes into three substantiated advantages graphify cannot match,
each tied to an artifact.

### 3.1 Per-method granularity — a different altitude graphify never reaches

graphify's coarsest *and finest* node is file/function (`repo/GAP.md:39` — only
file/function nodes). aOa's `SymbolMeta` carries per-method `[start-end]` byte
ranges (`storage.go:72-80`: `Name, Signature, Kind, StartLine, EndLine, Parent,
Tags`) — the ranges that power `peek` and hand an agent the actual method body,
not a node label. The KG's coarsest node is a module/package (DSM n=modules
≤~50); `peek` operates *below* that grain. **These are different altitudes, not
competitors** — the connectivity layer (graph) and the freshness-and-precision
layer (grep→peek) read one substrate and cite the same `file:line:commit`.

### 3.2 Provenance on every answer — but stated honestly by layer

Every rendered pixel and every agent answer traces to a fact with a
`source{file,line,commit}` pointer, surfaced via `aoa arch facts`. graphify has
**no evidence trail** — its edges are LLM-guessed and its grouping is
force-directed physics. But the provenance argument must be split by layer, or a
red-teamer correctly calls it overstated:

- **On layer-1 REAL edges (the keystone import edge), `source` is an
  audit/freshness anchor, not an inference leash.** The edge is deterministic
  AST output — there is no inference to discipline — so the stamp's job is to
  make the fact *auditable and re-derivable*: it powers `aoa arch facts`, records
  the commit the edge was derived at, and lets the substrate recompute on change.
  Here provenance is *closer to* freshness/audit metadata than to a safety
  mechanism, and that concession is honest.
- **On layer-2 MIXED content (agent grouping/naming/verb inference), the stamp
  is the load-bearing mechanism that makes inference safe to ship.** Here an
  agent *is* adding meaning on top of extracted facts; the stamp pins each named
  bucket or inferred verb back to the REAL facts it sits on, so the leash is
  checkable. This is the row where "provenance makes inference safe to ship" is
  literally true.

The scope law is the leash: the agent "may name/group/annotate extracted facts,
**NEVER add a node**" (`2026-06-11-core-competence-and-scope-line.md:26`,
verbatim). Edges are layer-1 REAL facts, so an agent *adding an edge* would be
inventing structure the code does not contain — out of bounds for the same
reason a node is. We carry the leash as "never add a node **or edge**," but that
"/edge" is our derived corollary; the binding ADR text says only "node." This is
exactly why graphify's LLM `semantically_similar_to` edges and force-directed
physics are explicitly **dropped**.

### 3.3 A falsifiable visual quality gate — the moat no competitor has

graphify's quality bar is one word: **"eyeball"** (`00-OVERVIEW.md:85`). aOa's
is a falsifiable, automatable gate: the **blind judge**
(`../standards/MODEL-STANDARD.md:43-53`) hands a judge agent *only* a screenshot,
the view's `question`, and the `pass` criterion — no JSON, no context (`:45-50`)
— and asks "can you answer the question from the image alone?" This is the third
of a three-step gate (lint → render → judge, `MODEL-STANDARD.md:18-53`).

The gate disciplines the *substrate*, not just the CSS: a view rendered at member
grain — e.g. a faulted event-platform component view at *66 components*
(`../mockups/archmodel/manifest.json:366`, an `8 buckets · 66 components · 10
edges` count) — risks failing readability, which is why the **deriver**
aggregates to bucket grain *before* it renders (`../integration/03-visualization.md:298-305`).
(That shard is itself `prov: simulated` (`manifest.json:369`), so it stands as
the illustrative member-grain risk, not a recorded judge failure.) The visual
bar reaches back and constrains how the graph is shaped — a quality discipline
graphify's "eyeball" cannot provide.

---

## 4. Our own thing — the current-vs-future diff renderer

This is the wedge: the one capability that is not "graphify but better" but
rather **a category graphify's architecture structurally cannot produce.**

graphify renders one import graph over a stale `graph.json`. A diff between two
points in time requires two consistent snapshots of *derived* edges, cheaply
recomputed, never drifting from the code — which a hand-run build artifact cannot
guarantee. aOa derives edges from the AST inside the live parse pass and can
snapshot them per commit (SHA-snapshot edges), so:

- **"What changed since `<ref>`" pack** — `delta` facts between two commits,
  scoped to a focus area: affected closure, view diffs, new findings
  (`../ENHANCEMENT-GUIDE.md §8`). **Derived from AST, free to compute, no doc to
  maintain.** It is market white space — only credible from code, and graphify's
  build-artifact model cannot match it (`2026-06-19-graphify-plus-mcp-research.md:174-176`).
- **Affected-set / PR blast-radius — graphify's single best idea, worth
  stealing.** "What breaks if I change X / which in-flight PRs collide?" is a
  reverse-transitive-dependency + set-intersection-across-changesets operation
  grep has no notion of (`serve.py:862-932 get_pr_impact/triage_prs`, blast-rank
  `:919-926`). It is **cheap, and never stale** because the PR file-list is a git
  feature. aOa should take this idea and run it on fresh edges instead of stale
  ones.

Both should be **elevated** from "Phase ③ someday" to named first-class targets
once the keystone lands. The diff renderer in particular is the strongest "our
own thing" claim in the whole positioning — it is not a faster competitor, it is
a capability the competitor's shape forbids.

---

## 5. The honest graphify frame (including the unverified YC-S26)

Calibration matters: overstating the threat is as damaging to credibility as
understating the moat. The disciplined frame:

- **The project is real.** ~69K stars, 36 tree-sitter grammars, Leiden community
  detection, a production-grade MCP transport (stdio + Streamable HTTP,
  constant-time API-key gate, DNS-rebinding guard, `sanitize_label` on
  LLM-derived fields). Treat the traction as real
  (`2026-06-19-graphify-plus-mcp-research.md:108-111`).
- **The "YC S26" badge is UNVERIFIED / self-applied.** The YC company profile
  404s; the S26 batch is not announced until Sept 2026; the founder bio omits
  YC. Treat the YC label as marketing, not fact
  (`2026-06-19-graphify-plus-mcp-research.md:108-111`). Do **not** repeat "YC-backed"
  as established.
- **graphify's genuine value is NARROW.** Only 3 of its ~7 MCP tools beat grep:
  reachability/`shortest_path` (`serve.py:822`), affected-set/PR-blast-radius
  (`serve.py:862-932`), and architecture orientation (`god_nodes`/`graph_stats`,
  `:775-792`). The other 7 — `query_graph`, `get_node`, `get_neighbors`,
  `get_community`, etc. — degrade into a worse, stale grep
  (`serve.py:144` full scan, `:717-718` linear scan).
- **Even graphify's 3 good tools carry staleness** its build-artifact
  architecture cannot shed. aOa takes the 3 good ideas, runs them on fresh
  AST-derived edges, and removes the ceiling — and graphify itself becomes "just
  another estate in the dropdown."

**The honest head-to-head:**

| | graphify | aOa arch views |
|---|---|---|
| Languages | Python (+partial JS/Java), 36 grammars, hand-grouped | 3 tuned extractors (Go/Python/JS-TS-TSX) / 509 registered / forest reachable, one walker |
| Views | 1 import graph (+tree, callflow) | 16 architect-trusted standard types on 5 shard kinds, one engine |
| Freshness | manual script run over a stale `graph.json` | rides the live index — regenerates as you type |
| Granularity | file/function nodes only (`GAP.md:39`) | per-method `[start-end]` byte ranges (`peek`) |
| Evidence | none (LLM-guessed edges, physics grouping) | every edge carries file:line:commit (`aoa arch facts`) |
| Quality bar | *"eyeball"* | lint + **blind-judge gate** (`MODEL-STANDARD.md:43-53`) |
| For AI agents | stale graph that must nudge agents off grep | CLI-first, sub-ms daemon reads, zero standing token cost |
| Diff / blast-radius | blast-radius on stale edges | current-vs-future diff + blast-radius on **fresh** edges (the wedge) |
| Cost to adopt | clone, pip, run a script | already inside the tool indexing your repo |
| YC status | "S26" badge — **unverified/self-applied** | n/a |

---

## 6. The landing — substantiated, not slogan

**Faster:** sub-ms unix-socket reads on a live fsnotify-reindexed index
(`socket/server.go:206`, `watcher.go:20`) — a latency-and-freshness floor MCP
over stdio and graphify's stale `graph.json` (`serve.py:19`) structurally cannot
reach.

**Better:** per-method `[start-end]` granularity below graphify's file/function
grain (`storage.go:72-80` vs `GAP.md:39`); file:line:commit provenance on every
answer (audit-anchor on REAL edges, inference-leash on MIXED, never conflated);
and a falsifiable blind-judge quality gate (`MODEL-STANDARD.md:43-53`) against
graphify's "eyeball."

**Our own thing:** the current-vs-future diff renderer (SHA-snapshot edges,
AST-derived, free to compute) — a capability graphify's build-artifact shape
forbids, not a feature it merely lacks — plus affected-set/PR-blast-radius run on
fresh edges, the best idea worth stealing.

**Kept honest throughout:** the language reach is a ladder (3 tuned / 10
walker-tuned / 509 registered / forest reachable), never a flat "400+"; the
graphify threat is real-but-narrow (3 of 10 tools) with an unverified YC badge;
and every grep-beating claim is gated on the keystone landing on the always-on
parse pass within the G0 budget. *Until that keystone ships, aOa is a proven face
(the judged viewer) waiting for a substrate (the import edges).*

---

## Appendix — falsifiable anchor index (red-team this first)

| Claim | Anchor |
|---|---|
| EXTRACTION tuning = 3 hand-written extractors + generic fallback | `parser.go:235 extractSymbols` → `:347 extractGo`, `:458 extractPython`, `:532 extractJavaScript`, `:250 extractGeneric` |
| Walker concept-resolution tuning = 10 (different subsystem) | `lang_map.go:48 langOverrides` (ten map keys, from the literal); `SupportedLanguages():172-174` returns those keys |
| 509 grammars registered | `languages_forest.go:5` (`// Languages: 509`) |
| Stale "28" carried by the guide's own source | `../integration/00-OVERVIEW.md:81` ("28, one walker"); also `README.md:544`, `CLAUDE.md`, scope law — §1 ladder is the correction |
| Sub-ms socket read; flat method switch (no JSON-RPC envelope) | `socket/server.go:206 handleRequest`, cases `:208+` |
| Live reindex (freshness floor) | `watcher.go:20 onFileChanged` (serialized `:43 a.mu.Lock`, `ParseFileToMeta` inside) |
| Per-method granularity vs graphify file/function nodes | `storage.go:72-80 SymbolMeta` ([start-end] ranges) vs `repo/GAP.md:39` |
| O(1) token lookup vs graphify un-indexed full scan | `storage.go:60 Tokens` vs `serve.py:144`/`:717-718` |
| graphify stale build artifact | `serve.py:19 _load_graph` (mtime/size poll) |
| graphify wins narrow (3 of 10 tools) | `serve.py:822` (shortest_path), `:862-932` (PR impact), `:775-792` (god/stats); anti-pattern `:144, :717-718, :415-418, :670-675` |
| Blind-judge gate (the moat) | `../standards/MODEL-STANDARD.md:43-53` (judge gets only screenshot/question/pass, `:45-50`); 3-step process `:18-53` |
| Member-grain readability risk (illustrative, simulated prov) | `../mockups/archmodel/manifest.json:366` (`8 buckets · 66 components · 10 edges`), `:369` (`prov: simulated`); deriver aggregates `../integration/03-visualization.md:298-305` |
| Scope law leash + provenance role-by-layer | `2026-06-11-core-competence-and-scope-line.md:25-26` (DERIVE/REAL row; "NEVER add a node" verbatim; "/edge" is our corollary) |
| Diff renderer / blast-radius as the wedge | `../ENHANCEMENT-GUIDE.md §8`; `serve.py:862-932`; `2026-06-19-graphify-plus-mcp-research.md:174-176` |
| Unverified YC-S26; project real (~69K stars) | `2026-06-19-graphify-plus-mcp-research.md:108-111` |
| Frontier CLI agents rejected stale graph for grep | `2026-06-19-graphify-plus-mcp-research.md:96-103` |
