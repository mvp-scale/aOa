# Hybrid Retrieval + Inline-Peek inside the Interactive Diagram

**Status:** architectural position — DRAFT, Phase-2+ scope. No code changes
prescribed. **Falsifiable:** every `file:line` is cited against branch `playbook`
HEAD; every proposal claim cites a pool doc; every external claim cites a URL. If a
cited anchor is wrong, the claim built on it is void.

**Lifecycle stamp:** Phase-2+ / gated on BOTH the keystone (doc 12 E1) AND the
Phase-2 web/MCP surface (doc 08 MVP ladder `08:296-298`; doc 12 puts MCP last,
`12:200-218`). **Read docs 13 (handoff & granularity) and 14 (unified agent
toolkit) as near-term; this doc is *not* immediate scope.**

**BUILT** = exists in source today. **PROPOSED** = net-new per `12-onboarding-plan.md`.

---

## 0. Where this doc sits (and where it doesn't)

This doc is the **third** of the hybrid-retrieval split. It is the deliberately
later, surface-specific delta — not a restatement of the architecture argument.

| Doc | Owns | Lifecycle |
|---|---|---|
| **13** | The handoff & granularity model — the three-layer toolkit (grep → graph → peek) and the honest import-edge asymmetry, grounded in `TokenRef`/peek codes | NOW / falsifiable |
| **14** | The unified agent "tips & tricks" — verbs, chaining, the CLAUDE.md guidance extension (the G3 agent-first deliverable) | E6 / ships with the CLI |
| **15 (this)** | The hybrid graph+lexical evidence (cited) **and** the inline-peek-on-click extension of the interactive diagram (doc 08), making the §13 asymmetry *visible in the UI* | Phase-2+ / gated on keystone + web/MCP surface |

**This doc is justified as a distinct third area on two grounds the synthesis
accepted:** (1) a different *audience* — the dashboard/MCP human surface, not the
agent CLI; (2) a different *lifecycle* — gated on the Phase-2 web/MCP surface, the
last thing to ship. It adds exactly one new mechanism not covered by 13/14: the
**read** side of the diagram click. Doc 08 already specifies the click that hands
Claude the cycle's edges with `file:line:commit` (the *write/dispatch* side,
`08:104-105,205-225`); this doc closes the loop on the *read* side — the same click
can **inline-peek** the endpoint bodies.

Everything here rests on the §13 result and does not relitigate it: the KG
**addresses** lines via the shared `TokenRef{FileID, Line}` identity
(`internal/ports/storage.go:65-69`) and **hands off** to peek
(`internal/adapters/socket/server.go:458-519`) to read bodies. This doc asks one
new question: *what does that handoff look like when the trigger is a mouse click on
a node, edge, or finding in the live diagram?*

---

## 1. The hybrid-retrieval evidence — graph + lexical, cited

The leading-edge 2025–26 consensus is that **structural (graph) retrieval and
lexical (grep/index) retrieval are complements, and a hybrid beats either alone** —
and that the graph *navigates to* a location while a separate step *reads* the
body. This is exactly aOa's grep+peek+graph composition, and it directly
corroborates the user's intuition that a KG is great for relationships but "may not
peek directly into the line of code."

*External sources are cited by URL and paraphrased; verify each section against the
live source before publishing — the thesis does not depend on any verbatim quote.*

| System | What it confirms | Source |
|---|---|---|
| **Codebase-Memory-style tree-sitter KG over MCP** | The graph intentionally stores **relationships, not source lines** — and that becomes a limitation; the conclusion is a **hybrid** (graph for structural queries + fallback file exploration), shipping a **separate code-snippet tool** — peek by another name. | https://arxiv.org/abs/2603.27277 |
| **CodeCompass-style architectural-context MCP** | Frames the move as *navigation, not retrieval*: `get_architectural_context` returns **file paths with edge-type labels, not code**; the agent reads those files independently. Identical handoff — **but at FILE grain.** | https://arxiv.org/abs/2602.20048 |
| **SCIP / Glean / Glass** | The graph encodes definition/reference line+character ranges in `Occurrence`; a *separate* layer reads the snippet. The graph addresses; a higher layer reads. | https://sourcegraph.com/blog/announcing-scip |
| **LocAgent (ACL 2025) / RepoGraph (ICLR 2025) / CodexGraph (NAACL 2025)** | Heterogeneous graphs (e.g. LocAgent's `{contain, import, invoke, inherit}`) output *which entity*; the agent then reads it. RepoGraph is a **plug-in**, not a replacement — confirming the "in addition to" framing. | (per the in-repo research verdict below) |

**The magnitude is small in absolute terms — stated honestly.** The prior in-repo
research records the SWE-bench-style gains from graph augmentation as recall-side
and small: **"~+2 pts on SWE-bench despite +32.8% relative"**
(`.context/details/2026-06-19-graphify-plus-mcp-research.md:97-99`). The hybrid wins,
but as a complement that lifts recall on the structural minority of queries — not as
a replacement for the lexical hot path. **grep+peek stay primary**
(`03-access-surface.md:162,190`).

### 1.1 Where aOa is ahead — and the one shared limit

aOa's edge proposal is *ahead* of these systems on the **mechanism** of the handoff,
because of one move the field does not make:

1. **Shared identity (the unique move).** The systems above use a *separate*
   addressing scheme for the graph vs. the read tool (node IDs / file paths, then a
   distinct re-resolve). aOa reuses **one `TokenRef`** across grep, the edge
   endpoints, and peek — the edge's Dst type is `TokenRef` (`01:76`), the same key
   `server.go:480` decodes a peek code into. The grep answer and the reachability
   answer cite the same `file:line:commit` *by construction* (`01:246`), not by a
   join.
2. **Method grain, not file grain.** CodeCompass hands back *files*; aOa's peek
   hands back a *method body* via `allLines[sym.StartLine-1 : sym.EndLine]`
   (`server.go:506-515`) — one altitude finer than the leading-edge MCP graph tools.
   *(Realized on intra-repo refs; v1's import edge still hands the external Dst at
   path grain — §2.)*
3. **Freshness.** Build-artifact graphs carry staleness as the #1 failure; aOa's
   edges ride the always-on fsnotify-reindexed parse pass
   (`internal/app/watcher.go:132`, `onFileChanged` → `ParseFileToMeta`), so the
   navigate-step and the read-step share **one** invalidation signal.

**The one shared, honest limit (the user's concern, universal not aOa-specific):**
the import/external-edge asymmetry. The "relationships but not source lines"
boundary and the path-returning architectural tools above hit the *same* wall.
**Naming it honestly — and rendering it in the UI (§3.2) — is the contribution.**

---

## 2. The asymmetry this doc must render (recap of §13, not relitigation)

The full grounding lives in doc 13. The one fact this doc needs, verified:

The edge shape is `Edge = {Src TokenRef, Dst TokenRef|DstPath, Kind, prov}`
(`01-knowledge-graph-and-visualization.md:76`; `02-integration-touchpoints.md:73`).
**In v1 there is exactly one edge kind, and it is the asymmetric one:** the import
edge `(importerFileID → importPath)` (`02:120`; keystone E1, `12:43-75`).

| Endpoint | Peekable? | Why (verified) |
|---|---|---|
| **Src** (the import-statement site) | **Yes → body** | A real `TokenRef` carrying `StartLine` → `Metadata[ref]` resolves → `server.go:481` → slice from disk. |
| **Dst** (external package) | **No → path-only** | A `DstPath` string, no `(FileID,Line)` in this repo → `s.idx.Metadata[ref]` has nothing → peek returns `"symbol not found"` (`server.go:482-483`). |

Both-ends-peekable edges (intra-repo `calls`/`contains` resolving to bodies) are
**PROPOSED / not in the keystone** — `contains` exists today only as
`SymbolMeta.Parent`, an un-indexed display *string* (`storage.go:78`). So in v1,
**every edge's Dst is the asymmetric, path-only kind.** That is precisely what the
interactive diagram must surface honestly — not paper over.

---

## 3. The inline-peek extension of the interactive diagram (the new delta)

This is the one mechanism this doc adds beyond 13/14. Doc 08's loop is a **write**
loop (click → hand Claude grounded edges → suggestion overlay → re-render). This doc
adds the **read** half: the same click can **inline-peek the endpoint body** — the
audit trail made visible *in place*, no context switch to a terminal.

### 3.1 What is BUILT vs PROPOSED

**BUILT (the transport this rides):**
- The `withETag`/304 in-process read transport — `withETag` returns 304 on
  `If-None-Match` match (`internal/adapters/web/server.go:34` `revisionFn`,
  `:49-52` `SetRevisionSource`, `:92-113` all `GET /api/*` routes ride it).
- The **click-fires-an-action precedent** — `POST /api/recon-investigate`
  (`internal/adapters/web/server.go:111`; handler `recon.go:555`) already takes a
  `{file, action}` from a UI click and acts on it, mutating **annotation** via
  `SetFileInvestigated` (`recon.go:577`) — **never the substrate**. This is the
  exact precedent shape doc 08 §6 rides (`08:205-225`).
- The peek backend itself — `TokenRef`→body over the socket
  (`server.go:458-519`) and the thin CLI client over the same backend
  (`cmd/aoa/cmd/peek.go:27` delegates to `client.Peek`).

**PROPOSED (genuinely net-new — confirmed):**
- **There is no peek endpoint on the web surface today.** A `grep` of
  `internal/adapters/web/*.go` for any peek route returns nothing — the dashboard
  never calls peek. An inline-peek-in-diagram needs a new
  `GET /api/arch/peek?ref=…` (or equivalent) that wraps the *existing*
  `server.go:458-519` backend behind `withETag`, plus the ReactFlow click handlers
  (`onNodeClick`/`onEdgeClick`/`onFindingClick`, `08:212`).
- Everything is gated on the keystone edges (E1) being present to click on, and on
  the Phase-2 arch web feed (`GET /api/arch/*`, `03:293`; doc 08 MVP `+1` cut,
  `08:297`).

### 3.2 The interaction — read and write, one click, leash intact

```
ReactFlow onNodeClick(node) | onEdgeClick(edge) | onFindingClick(finding)
  │
  ├─(READ — this doc's delta)──> GET /api/arch/peek?ref=<TokenRef of the endpoint>
  │     └─> wraps server.go:458-519 backend (Decode → Metadata[ref] → slice)
  │         ├─ intra-repo ref      -> inline panel shows the METHOD BODY
  │         └─ external import Dst  -> "symbol not found" -> panel stamps
  │                                    "package grain only" (path string; §2)
  │
  └─(WRITE — doc 08's loop, unchanged)──> POST /api/arch/suggest {subject,kind,scope}
        └─> MethodArch* -> grounded fact-pack (edges w/ file:line:commit, 08:215)
            └─> Claude (OUTSIDE the derive path: CLI subprocess / MCP host, 08:311)
                └─> proposed-edge overlay -> overlay loader rejects invented ids
                    └─> viewer re-renders AFTER, stamped SIMULATED · proposed (08:221)
```

**The two halves compose, they do not compete.** Read answers *"what is this thing
I clicked?"* (peek the body). Write answers *"what if I changed it?"* (dispatch
Claude, get a simulated overlay). The user can do the first with zero LLM
involvement — it is pure deterministic peek — and *optionally* escalate to the
second.

**The leash is intact on both halves:**
- **Read** is read-only: it slices bytes from disk through the existing peek
  backend; it writes nothing.
- **Write** keeps Claude *outside* the deterministic derive path (`08:311`;
  `12:311`); the model produces a *proposed-edge overlay file* the deterministic
  renderer consumes; the overlay loader **rejects any id not present in the facts**
  and drops the view to MIXED / the AFTER to SIMULATED (`08:264-269`; scope law
  "NEVER add a node," `2026-06-11-core-competence-and-scope-line.md:26`). The agent
  **never writes a node or edge into the REAL substrate** — exactly as
  `recon-investigate` mutates only annotation, never the substrate
  (`recon.go:577`; `08:274`).

### 3.3 The asymmetry, made visible (the honest UI contract)

This is why inline-peek-in-diagram is worth a doc: it is the **first surface where
the §2 asymmetry becomes a user-visible behavior**, and the design must stamp it
honestly rather than fake a body.

| Clicked endpoint | Inline panel shows | Stamp |
|---|---|---|
| Intra-repo node / Src import site | the **method body** (peek of the `TokenRef`) | REAL · `file:line:commit` |
| External import **Dst** (the v1 common case) | the **import path string** — no body | **"package grain only"** (vendored → open package; else path-only) |
| Symbol larger than `MaxRange = 500` lines (`internal/peek/codec.go:13`) | `[start-end]` range, no inline body | "Read at range" (same as grep today) |

The scope law **forbids synthesizing the missing external target** ("never
synthesize a target," `12:72`). So the "package grain only" stamp is *not a bug to
fix* — it is the boundary of what a deterministic, REAL-derived edge can know, made
honest in the UI. The peek-failure path (`"symbol not found"`,
`server.go:482-483`) **is the expected, designed signal** for an unresolved
external Dst — the UI renders it as a grain stamp, exactly as the agent CLI falls
back per CLAUDE.md (`CLAUDE.md:26`).

---

## 4. Built-vs-proposed ledger (honest)

**BUILT:**
- Peek backend `TokenRef`→body (`server.go:458-519`: Decode `:474`, ref-build
  `:480`, `Metadata` lookup `:481`, `"symbol not found"` `:482-483`, slice
  `:506-515`); CLI client over it (`cmd/aoa/cmd/peek.go:27`).
- Shared `TokenRef`/`SymbolMeta` identity (`internal/ports/storage.go:65-80`).
- `withETag`/304 transport (`web/server.go:34,49-52,92-113`).
- `POST /api/recon-investigate` click-fires-action precedent
  (`web/server.go:111`; `recon.go:555`, mutate `:577`).

**PROPOSED (net-new):**
- **The web peek endpoint** — no peek route exists on the web surface today (grep of
  `internal/adapters/web/*.go` is empty); inline-peek needs a new
  `GET /api/arch/peek` wrapping the existing backend behind `withETag`.
- The ReactFlow click handlers and inline panel (`08:212`).
- Everything upstream it depends on: the keystone import edge (E1), the `arch`
  web feed `GET /api/arch/*` (`03:293`), and — for the write half — `aoa arch` +
  `MethodArch*` (E6) and the overlay loader (`02-arch-service.md:168`).

**The handoff *backend* (peek) is ready; the graph that feeds it and the web route
that exposes it are not yet built.**

---

## 5. Open seams for the red-teamer

1. **Verb naming (inherited from 13/14).** `Reach`/`Blast` (docs 03 `:218-221`, 07,
   08) are *not* among the six canonical `MethodArch*` methods (`12:181`). The web
   peek route and any dispatch must reconcile to the canonical verbs before
   hardcoding surface.
2. **External-Dst grain in the UI.** The "package grain only" stamp is method-body
   **never**, file/package grain **only if vendored/on-disk/indexed**, else
   path-string only — the UI must render all three honestly (§3.3), not collapse
   them.
3. **Pool-doc drift (note for the next pass).** `02-integration-touchpoints.md:120-122`
   still carries the struck "born in the existing pass / no extra cost" framing that
   doc 12 E1 `[CORRECTED]` repudiates (`12:43-75`); and `countImportSpecs` is
   `//go:build !lean` (`walker.go` header), not `core`. Neither changes this doc's
   substance — flagged for consistency.
