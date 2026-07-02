# 03 — The Access Surface (deep dive)

**Status:** integrated architectural position — the definitive treatment of *how
agents and humans reach the substrate*. Companion to the spine in
`ENHANCEMENT-GUIDE.md` (§3) and the prior research verdict
`.context/details/2026-06-19-graphify-plus-mcp-research.md`. **Falsifiable
document:** every load-bearing claim cites a `file:line` or a source doc; if a
cited anchor is wrong, the claim built on it is void.

**Binding law (do not relitigate here):**
- **Goals** — `.context/GOALS.md` (G0 Speed sub-ms, G2 Two-Binary split, G3
  Agent-First drop-in shim, G4 Hexagonal — non-negotiable)
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
  (derive REAL / infer-leashed MIXED / declare-and-diff)
- **Prior research verdict** — `.context/details/2026-06-19-graphify-plus-mcp-research.md`
  (treated as established; sharpened, not relitigated)

**Scope of this document.** This is the *access-surface* deep dive: the surfaces
through which the substrate is reached (daemon-socket method, `aoa arch` CLI, web
adapter, MCP), the **order** in which they are built, the **rationale** for that
order, and the single load-bearing question — *do the shimmed `grep`/`peek`
verbs stay useful once a knowledge graph exists?* — argued both ways and landed.
It also fixes which graphify tools are exposed and which are explicitly refused.
What this document does **not** re-derive: the keystone import-edge extraction
(see ENHANCEMENT-GUIDE §5 / `integration/01-facts-substrate.md`), the rendition
engine (`integration/02-arch-service.md`), or the viewer (`integration/03-visualization.md`).

---

## 0. The one-sentence recommendation

**Build the surfaces in this order: ① daemon-socket method → ② `aoa arch` CLI →
③ web/viewer feed → ④ MCP adapter, last and thin. Never replace fresh
`grep→peek` with a stale graph scan; never let MCP front a latency-sensitive
query.**

Everything below is the falsifiable justification for that ordering, and the
explicit list of which graph queries each surface may expose.

---

## 1. The four surfaces — what each one is, and what it costs

The substrate is reached through exactly four transports. Three already exist in
the codebase as patterns; the fourth (MCP) is a new adapter that rides the same
service the other three do. The ordering recommendation falls directly out of
the cost structure of these four.

| # | Surface | Today | Net-new for arch | Latency floor |
|---|---|---|---|---|
| 1 | **Daemon-socket method** | flat JSON method switch, `socket/server.go:224-249` | one `case MethodArchDerive:` arm + handler — a one-liner that inherits the existing sub-ms read path (reach/blast = CLI aliases, ADR 2026-07-02) | **sub-ms** (unix socket, no handshake) |
| 2 | **`aoa arch` CLI** | cobra parent/child precedent, `cmd/aoa/cmd/root.go`; `grammar` parent/child | `archCmd` + children, thin delegate to the daemon/App | sub-ms (warm daemon) / ~ms (cold process-exec) |
| 3 | **Web / viewer feed** | localhost dashboard + `withETag` + `//go:embed`, `web/server.go:77-113` (`/api/recon*` precedent) | `GET /api/arch/*` reusing `withETag`; embed the viewer at `GET /arch` | ETag-polled (recompute-on-compact, not streaming) |
| 4 | **MCP adapter** | none | one more adapter beside socket/web, wrapping the same `ArchQuerier` 1:1 | **JSON-RPC over stdio + handshake/session** — structurally *above* surface 1 |

The decisive fact in that table is the last column: **surface 1 is the latency
floor, and surface 4 sits structurally above it.** You cannot make an MCP server
faster than the socket it would wrap — MCP adds a JSON-RPC envelope, a stdio
transport, and a session/handshake the socket switch has none of (the socket
switch is a bare `switch req.Method` with no envelope, `server.go:207`). So you
build the floor first and the wrapper last.

---

## 2. Why this order — the falsifiable rationale

### 2.1 G3 is binding precedent for native-first

`GOALS.md:10` (G3) is literally *"Agent-First — Drop-in shim for grep/egrep/find…
Agents never know it's not GNU grep."* The agent-access contract is **already
won** through the grep/peek shim and the daemon behind it. That means the
primary, latency-critical agent surface is the socket, by existing law — not a
new protocol bolted on top. MCP is a *second, optional* way to reach the same
service, never the path G3 already paved.

### 2.2 The latency gap is real, measured, and one-directional

The daemon answers over a unix socket through a flat method dispatch with **no
handshake, no session, no JSON-RPC envelope**: `handleRequest` is a bare
`switch req.Method` (`socket/server.go:207`) whose arms (`MethodSearch`,
`MethodPeek`, … `socket/server.go:208-227`, switch closes `:230`) each delegate
to a handler and return. Adding a `case MethodArchDerive:` (the spec method behind the `reach` CLI alias) is a **one-line
addition** that inherits the sub-ms read path G0 (`GOALS.md:7`) mandates.

MCP's stdio + JSON-RPC handshake/session overhead sits **above** that floor by
construction. This is not a tuning question; it is a layering fact: MCP would
*wrap* the socket (or the same App service the socket calls), so it can only ever
add latency, never subtract it. The socket is the floor → you build the floor
first.

### 2.3 The market split confirms native-first for a CLI tool

The prior research (`research:96-103`) establishes a clean split: **CLI/agent
tools chose grep; IDE tools chose indexes.** Claude Code / Codex / Aider
deliberately use *no* index — agentic grep "outperformed RAG by a lot" on
freshness, security, and simplicity (`research:99-100`). The counter-signal is
real but lives in a different segment: IDE-embedded tools (Cursor) *do* maintain
semantic indexes (`research:100-102`). And the cost asymmetry is measured: the
`gh` CLI was **7–32× cheaper than GitHub's MCP for bulk ops** (`research:96`).

aOa is CLI-first → it is in the camp that won the CLI-agent segment. Building the
native socket/CLI surface first is alignment with that result, not preference.

---

## 3. So why build MCP at all? — and the direct answer to "MCP vs faster"

The user's sharp question is: *is MCP a distraction, or is building it faster the
move?* The honest answer has two halves that must be held together.

**Half one — build MCP, because the interface bet is low-regret.** MCP *the
protocol* has won: OpenAI (Mar 2025), Google (Apr 2025), Microsoft VS Code GA
(Jul 2025), donated to the Linux Foundation Agentic AI Foundation (Dec 2025)
(`research:93-95`). Exposing aOa over MCP rides a real wave, and hexagonal
architecture (G4) makes the adapter nearly free *later*: it is one more adapter
beside `socket/`/`web/`, wrapping the same `ArchQuerier` 1:1 with zero
duplicated logic. The caveat is noted, not fatal: MCP security is immature
(CVE-2025-6514, NSA guidance, `research:95`) — a reason to scope it tightly and
ship it late, not a reason to skip it.

**Half two — the load-bearing synthesis the casual reader misses.** Because MCP
structurally cannot beat the socket (§2.2), *building MCP is not the faster move,
and is not meant to be — it is the **wider** move, late and thin.* MCP buys
**reach** (MCP-only agents and IDEs that cannot shell out), never **speed**.
"Faster" is the socket's job: the socket is the floor and already exists in
pattern. So the single condition under which MCP is correctly scoped — and not a
distraction — is precisely this: **MCP must never front a latency-sensitive
query.** If an agent can shell out, it should use the CLI/socket; MCP is the
fallback for those that can't.

| | CLI / socket (`aoa arch …`) | MCP server |
|---|---|---|
| **Latency** | daemon-socket sub-ms; process-exec ~ms | JSON-RPC over stdio + handshake/session — structurally above the socket |
| **Buys you** | speed + freshness (the hot path) | reach (MCP-only agents/IDEs), never speed |
| **Precedent** | G3 "agent-first CLI" already proven via grep shim | new protocol surface to maintain |
| **Reach** | any agent that can shell out (all of them) | agents/IDEs with MCP support only |
| **Architecture** | new cobra commands + socket cases over the same App service | one more adapter beside socket/web — cheap *later* |
| **When it's the right surface** | always, for latency-sensitive retrieval | only as a fallback for agents that cannot shell out |

**One-line verdict:** MCP is not a distraction *iff* it is built last, scoped to
the grep-beating queries only (§5), and never placed on a latency-sensitive
agent's hot path. Under any other framing — MCP-as-primary, or MCP-as-speed-win —
it is a distraction.

---

## 4. The load-bearing question: do `grep`/`peek` stay useful once a KG exists?

This is *the* question for the whole access surface. If a knowledge graph
supersedes the shimmed `grep→peek` verbs, then the access surface reorganizes
around the graph and the shim becomes legacy. It does not. Here is the honest
case both ways, then the landing.

### 4.1 The case that they're superseded

Once typed edges exist, *"show me X and what it references"* **could** be one
graph hop instead of a `grep→peek` round-trip. If the graph were always fresh and
always complete, an agent would rationally prefer the structured one-hop answer
over a textual search followed by a body read. Taken at face value, a complete
fresh graph makes the neighbor-lookup verbs redundant.

### 4.2 The case that they stay primary (the stronger case)

Three independent reasons, each grounded:

1. **Freshness.** `grep` reads the live, fsnotify-reindexed index; *any* KG is a
   build artifact, and between builds its answers can be silently wrong. This is
   exactly the property the frontier CLI agents rejected the graph over
   (`research:86-87, 99-100`). graphify's own server proves the failure mode: it
   serves from a `graph.json` build artifact loaded by mtime-poll
   (`serve.py:19 _load_graph`), and its install hooks must actively *nudge*
   agents off grep (`research:87`) — the tell that the stale graph loses to fresh
   grep in practice.

2. **Granularity.** `peek`'s per-method `[start-end]` byte ranges
   (`SymbolMeta.StartLine/EndLine`, `internal/ports/storage.go:72-80`) operate
   *below* the grain a `unit`/`dep` graph models. The graph's coarsest useful
   node is a module/package (DSM `n=modules`, small); `peek` delivers the actual
   method body. These are **different altitudes, not competitors** — the graph
   answers connectivity *between* modules, `peek` answers *what is in* a method.

3. **Scope law forbids the rich edges that would threaten grep.** Tier-1 refuses
   call/inheritance/LLM-semantic edges (ENHANCEMENT-GUIDE §2.3; scope law). The
   KG is deliberately narrow — imports plus derived DSM/cycles. It was never
   going to answer the questions `grep` answers; it answers a *different,
   smaller, graph-shaped* set. There is no overlap to contest.

### 4.3 The landing — both stay, at different altitudes

**`grep→peek` is the freshness-and-precision layer and stays the default verb;
the KG is the connectivity-and-orientation layer that adds the three verbs `grep`
structurally cannot form (reachability, blast-radius, orientation).** aOa's edge
is that *both layers read one substrate* — so the agent's `grep` answer and its
reachability answer cite the **same** `file:line:commit`. The strongest market
conclusion is **hybrid** — structural query via graph, fallback to grep/file,
beats either alone (`research:106-107`) — and aOa is that hybrid *by
construction*, because the connectivity layer and the precision layer share one
index and one freshness signal.

**Practical consequence for the access surface:** the socket keeps
`MethodSearch`/`MethodPeek` as first-class, always-on methods. The arch methods
(the six spec `MethodArch*` — reach/blast ride `arch.derive`/`arch.findings` as CLI aliases, ADR 2026-07-02) are *added beside* them in the same
switch (`server.go:225-248`) — never *instead of* them. An agent's default verb
remains the shim; the graph verbs are reached only when the question is
transitive/topological.

---

## 5. Which graphify tools to expose — and which to refuse

The access surface exposes exactly the queries that genuinely beat `grep`, and
**nothing** that degrades into a worse, stale grep. graphify's own 10-tool
surface (`serve.py:564-684`) is the falsifiable reference for "what this would
actually be."

### 5.1 EXPOSE — the three classes that beat grep (all REAL-derivable, never stale)

| Surfaced as | Query class | Why grep structurally can't do it | graphify evidence |
|---|---|---|---|
| `aoa arch reach A B` (CLI alias → `arch.derive`) | **Reachability / shortest_path** | Transitive-closure question; answer length unbounded a priori. An agent fakes it only by recursively grepping each callee N hops deep — and **cannot prove "no path exists."** | `nx.shortest_path serve.py:822`, hop chain `:828-847`, "No path found" `:823-824` |
| `aoa arch blast <ref\|PR>` (CLI alias → `arch.findings`) | **Affected-set / reverse-deps / PR blast-radius** — *graphify's single best idea* | grep finds *forward* literal occurrences; reverse transitive dependency and set-intersection across changesets are edge-closure ops grep has no notion of. **Cheap and never stale** — the PR file-list is a git feature. | `get_pr_impact`/`triage_prs serve.py:862-932`, blast-radius rank `:919-926` |
| `aoa arch views` (orientation) | **Architecture orientation — god_nodes** | Degree-centrality is a global graph property; grep counts textual occurrences, not structural in/out-degree. | `god_nodes serve.py:775-780`, `graph_stats :782-792` |

Plus two **aOa-native** topological classes graphify does *not* even expose
(`serve.py:564-684` lists 10 tools, no cycles/DSM): **cycles** (Tarjan SCC over
the dep edge set — a cycle is a topological property with no string to match) and
**DSM** (the adjacency matrix itself). These ride the same `dep` edge set as
`component`, so they are pure derivations, not new extraction.

**The decisive asymmetry on every exposed row is freshness.** In graphify these
ride a stale `graph.json` artifact; aOa derives the same edges from the AST
inside the live, fsnotify-reindexed parse pass — so the grep-can't classes become
**graph-capable AND never-stale**, capturing graphify's real (narrow) value while
eliminating its structural defect.

### 5.2 REFUSE — the stale-grep trap and the determinism violations

| Refused | Verdict | Why |
|---|---|---|
| **`query_graph` / `get_node` / `get_neighbors`** (1-hop "show X and its neighbors") | **DO NOT EXPOSE — the stale-grep trap** | graphify's seed step is an **un-indexed full node scan** (`_score_nodes serve.py:144`, `get_node` linear scan `:717-718`) over a snapshot only as fresh as the last build — literally a slower grep over a stale graph. The tell: graphify's own output must *nudge agents off grep* (`serve.py:415-418, :670-675`). For "show this function and what it calls," fresh `grep→peek` wins on freshness + precision + per-method `[start-end]` granularity (graphify has only file/function nodes — `GAP.md:39`). |
| **Cross-modal / LLM-inferred edges** (`semantically_similar_to`, pdf/image/audio) | **OUT** | Conflicts with the determinism thesis; LLM `calls` edges needed guards to drop phantoms from shared names (`render`/`parse`, `research:88-89`). Scope-law layer-3 at best. |
| **Heavyweight standing/stale graph as the agent's primary retrieval** | **OUT** | The frontier CLI agents rejected exactly this for agentic grep (`research:113-115`). Don't front the agent's hot path with the bet they lost. |

**The one-paragraph rule:** the access surface exposes exactly the
transitive/topological classes grep can't — reachability, affected-set/blast-
radius, cycles/DSM, god_nodes — all REAL-derivable and never-stale, and it ships
**nothing** for the 1-hop neighbor class where a graph degrades into a worse,
stale grep. This holds identically across **all four** surfaces: the socket
method, the CLI command, the web route, and the MCP tool all wrap the same
`ArchQuerier` and therefore expose the same vetted query set. **MCP does not get
its own, looser tool list** — it exposes a *subset* of the native surface
(reachability / blast-radius / orientation), never a superset, and never
`query_graph`.

---

## 6. The proposed `aoa arch` command family

JSON to stdout, mirroring the viewer's manifest→shard contract exactly, so the
agent's query result and the browser's render payload are one truth (the spine).

```
aoa arch views                      # catalog + status per view (live/mixed/declared/planned)
aoa arch view <id> [--scope p]      # one view's rendition JSON (= a shard)
aoa arch reach A B                  # reachability / shortest-path between two anchors
aoa arch blast <ref|PR>             # affected-set / PR blast-radius (graphify's best idea)
aoa arch findings [--new]           # findings, baseline-aware
aoa arch journey <id> | derive A B  # stored journey / focus-flow derivation
aoa arch facts <subject>            # raw facts + source pointers (the audit trail)
aoa arch pack <dd|pci|delta>        # evidence pack export
```

Each command is a **thin delegate to the daemon/App** (the cobra parent/child
pattern of `grammar`, `cmd/aoa/cmd/root.go`), and each maps 1:1 to a socket
method. The MCP adapter, when built, wraps the same handlers — so there is one
service, four transports, and one vetted query set.

`aoa arch facts <subject>` is the audit trail behind every other surface: a human
clicks a rendered node, an agent calls `facts`, and both land on the same
`file:line:commit`. That shared audit pointer is what makes the access surface
*provenance-uniform* across CLI, socket, web, and MCP.

---

## 7. The build order, restated as the access-surface phase plan

The ordering recommendation maps onto the project phases as follows (consistent
with ENHANCEMENT-GUIDE §10):

| Phase | Access-surface deliverable | Why here |
|---|---|---|
| **②** (keystone) | the six spec `MethodArch*` handlers on the existing socket switch (reach/blast = CLI aliases, ADR 2026-07-02); `aoa arch view component/dsm/cycles`, `aoa arch facts` | The socket is the latency floor and a one-line addition (`server.go:225-248`); it must land *with* the keystone edges so the three edge-derived views are reachable sub-ms. |
| **③** (live estate) | web/viewer feed via `GET /api/arch/*` (reuse `withETag`, `web/server.go:77-113`); `aoa arch blast` elevated | The human face rides the same ETag-polled read path as `/api/recon*`; blast-radius elevated once the dep set is live. |
| **④** (governance) | **MCP adapter — thin, reachability/affected-set/orientation only, reach not speed** | Built last by design: it cannot beat the socket, so it ships only after the native surface is proven, scoped to the reach role and never a `query_graph` replacement. |

**The single invariant across all phases:** the default agent verb stays
`grep→peek` on the live index; the arch verbs are *added beside* it, never
*instead of* it; and no surface ever fronts a latency-sensitive query with a
stale graph scan. That invariant is the whole access-surface position in one
sentence.

---

## Appendix — falsifiable anchor index (red-team this list first)

| Claim | Anchor |
|---|---|
| Flat socket method switch (MCP/arch rides alongside as a one-line `case`) | `internal/adapters/socket/server.go:224-249` (switch `:225`, cases `:226-245`, default `:246`); method consts in `socket/protocol.go` |
| G3 agent-first / drop-in grep shim is binding precedent | `.context/GOALS.md:10` (G3) |
| G0 sub-ms read path the socket arm inherits | `.context/GOALS.md:7` (G0) |
| G2 two-binary split (why MCP must not pull recon onto the hot path) | `.context/GOALS.md:11` (G2) |
| `peek` per-method `[start-end]` grain is below the graph's altitude | `internal/ports/storage.go:72-80` (`SymbolMeta.StartLine/EndLine`) |
| Web ETag + embed precedent (`/api/recon*`) | `internal/adapters/web/server.go:77-113` |
| cobra parent/child precedent for `aoa arch` | `cmd/aoa/cmd/root.go` (`grammar` parent/child) |
| MCP-protocol-won (low-regret interface bet) | `research:93-95` |
| MCP security immature; `gh` 7–32× cheaper than GitHub MCP | `research:95-96` |
| Market split: CLI/agent → grep; IDE → indexes; aOa is CLI-first | `research:96-103` |
| Hybrid (graph + grep fallback) beats either alone | `research:106-107` |
| Frontier agents rejected the standing/stale graph as primary retrieval | `research:99-100, 113-115` |
| graphify serves from a stale build artifact (mtime-poll) | `serve.py:19 _load_graph` |
| EXPOSE reachability | `nx.shortest_path serve.py:822`, `:823-824, :828-847` |
| EXPOSE affected-set / PR blast-radius (graphify's best idea) | `get_pr_impact`/`triage_prs serve.py:862-932`, rank `:919-926` |
| EXPOSE orientation (god_nodes / stats) | `god_nodes serve.py:775-780`, `graph_stats :782-792` |
| graphify exposes no cycles/DSM (aOa-native opportunity) | `serve.py:564-684` (10-tool list) |
| REFUSE `query_graph`/`get_node`/`get_neighbors` (un-indexed full scan = stale grep) | `_score_nodes serve.py:144`, `get_node :717-718`, nudge-off-grep `:415-418, :670-675`; file/function-only nodes `GAP.md:39` |
| Locking law governs derived arch writes, not the in-pass index fact | `00-OVERVIEW.md:101` (reconciled in ENHANCEMENT-GUIDE §5.1) |
| Spine: one substrate, two faces, same `file:line:commit` | `2026-06-19-graphify-plus-mcp-research.md:121-126, 180-184`; `00-OVERVIEW.md:90-94` |

---

**Provenance note.** This document is the access-surface curation of the
round-3 integrated position in `ENHANCEMENT-GUIDE.md`; it reproduces that
document's verified anchors for the access surface and adds the socket/CLI/web/MCP
ordering detail. No code or source files were edited — markdown only.
