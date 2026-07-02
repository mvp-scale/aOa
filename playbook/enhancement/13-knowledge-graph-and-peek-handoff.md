# 13 — The KG↔Peek Handoff & Granularity Model

> **The direct answer to:** *"Can a knowledge graph peek into the line of code?"*
>
> **No — and it doesn't need to.** A KG does not *contain* the line; it *addresses*
> it, reusing the same `TokenRef{FileID, Line}` identity that grep already emits and
> peek already resolves to a body. Graph navigation yields `TokenRef`s; those
> `TokenRef`s terminate in the **existing** peek backend. The KG is the relational
> layer *on top of* grep+peek — not a replacement for them.

**Status of this doc:** NOW / falsifiable. This is the architecture argument the
validator and red-teamer hit directly. It must stand alone, tightly cited, so a
wrong anchor voids only this doc and not the toolkit prose (doc 14) or the
interactive surface (doc 15).

**Legend:** every code claim cites `file:line` (verified against branch `playbook`
HEAD). **BUILT** = exists in source today. **PROPOSED** = net-new per
`12-onboarding-plan.md`. External claims cite URL.

---

## 1. The granularity model — three layers, one identity, one daemon

aOa already runs a **two-verb composition**: grep finds the location, peek reads the
body. The KG inserts a **third verb in the middle**, at a different altitude. All
three speak one identity and ride one daemon.

| Verb | Question | Input → Output | Altitude | Status |
|---|---|---|---|---|
| **grep** | *where is this token?* (lexical) | `token → []TokenRef` | textual occurrence | **BUILT** — O(1) `Tokens` map (`internal/ports/storage.go:60`); a hit emits a peek code (`internal/domain/index/searcher.go:43`) |
| **graph** | *what connects to this?* (relational) | `TokenRef → []TokenRef` (edge closure) | structural — in/out-degree, reachability, cycles | **PROPOSED** — no edge is representable today; `Index` is three node-maps (`storage.go:59-63`), and `SymbolMeta.Parent` (`storage.go:78`) is a display *string*, not a traversable reference |
| **peek** | *show me the body* (content) | `TokenRef → source lines` | the literal method body | **BUILT** — `internal/adapters/socket/server.go:458-519` |

### 1.1 The shared identity: `TokenRef`

The single 6-byte location key holds the whole model together:

```go
// internal/ports/storage.go:65-69
type TokenRef struct {
    FileID uint32
    Line   uint16
}
```

It is the key type of the persisted index — `Tokens map[string][]TokenRef`
(`storage.go:60`) and `Metadata map[TokenRef]*SymbolMeta` (`storage.go:61`) — and
the body coordinates live on `SymbolMeta.StartLine` / `EndLine`
(`storage.go:76-77`).

### 1.2 Peek is already a pure `TokenRef → body` function

Verified end-to-end in source:

1. **grep emits a base36 peek code.** `peek.Encode(fileID, startLine)` packs
   `fileID<<16 | startLine` (`internal/peek/codec.go:17-20`), called at
   `searcher.go:43` where `startLine` comes from `SymbolMeta.StartLine` via
   `PeekRef()`. The **gate** is `rangeSize > 0 && rangeSize <= peek.MaxRange`
   (`MaxRange = 500`, `codec.go:13`) at `searcher.go:41`; larger symbols emit `--`
   (the documented Read fallback).
2. **peek decodes the *same* code back.** `peek.Decode` (`server.go:474`; codec at
   `codec.go:23-31`) reconstructs `ref := ports.TokenRef{FileID: fileID, Line: startLine}`
   (`server.go:480`) — **the exact identity grep encoded** — looks up
   `sym := s.idx.Metadata[ref]` (`server.go:481`), and slices
   `allLines[sym.StartLine-1 : sym.EndLine]` from disk (`server.go:506-515`).

So the composition is mechanical:

```
grep:  token    → location   (Tokens map)
peek:  location → body        (Metadata + disk slice)
       — they compose through TokenRef.
```

A graph verb whose **output is a set of `TokenRef`s** plugs into the *same*
`server.go:458-519` backend with **zero new read machinery**. The proposal's own
edge endpoint type *is* `TokenRef` (`01-knowledge-graph-and-visualization.md:76`) —
the same identity `server.go:480` decodes to. This is the playbook's "different
altitudes, not competitors" landing (`03-access-surface.md:179`;
`01-knowledge-graph-and-visualization.md:238`), now proven at the code level.

### 1.3 One daemon, one freshness signal — at the honest cost basis

The edge is index data, and it *is* freshness-coupled to the grep answer. Both are
produced from one `parser.Parse` tree, and the incremental file-save path re-runs
`ParseFileToMeta` on `onFileChanged` (`internal/app/watcher.go:132`). So the grep
answer and the reachability answer cite the **same `file:line:commit`** and
invalidate together
(`01-knowledge-graph-and-visualization.md:243-249`; `03-access-surface.md:188-198`).

**But the import extractor is genuinely NET-NEW — not a free rider on an existing
import visit.** The always-on Go extractor `extractGo` switches *only* on
`function_declaration` / `method_declaration` / `type_declaration`
(`internal/adapters/treesitter/parser.go:351-360`) and **never visits import
nodes**. No import path is currently "in hand and thrown away" in the parse pass.
(`countImportSpecs` — `internal/adapters/treesitter/walker.go:568` — *does* count
imports, but it lives in a **separate `//go:build !lean` recon walk**
(`WalkForDimensions`), reached only via the recon adapter, never via the always-on
`extractGo`.)

The honest cost basis: an added import-extraction concern folded into `extractGo`'s
existing `root.Child(i)` loop so it rides ONE traversal, budgeted at **G0 ≤ +3%**
against a correctly-characterized baseline (`12-onboarding-plan.md:71`) — *not* "no
extra cost / born in the existing walk." Doc 12 E1 carries this exact correction
with a `[CORRECTED]` flag (`12-onboarding-plan.md:43-75`). Hybrid-by-construction,
but earned, not free.

> **Pool-doc note:** `02-integration-touchpoints.md:80-81` and `:120-122` still
> carry the struck framing ("the import AST nodes are already visited during
> parse" / "no second walk, no extra cost"). Doc 12 E1 `[CORRECTED]` and this §1.3
> repudiate it. Cite `02` for the edge *shape* (correct), not for the cost basis.

---

## 2. The honest gap — which graph results peek to a body, and which don't

**This is the user's exact concern, grounded.** The edge shape is:

```
Edge = { Src TokenRef, Dst TokenRef | DstPath, Kind, prov }
       (01-knowledge-graph-and-visualization.md:76; 02-integration-touchpoints.md:73)
```

The `| DstPath` union is **load-bearing — it *is* the asymmetry**, and it is honest
in the proposal, not hidden.

### 2.1 In v1 there is exactly one edge kind, and it is the asymmetric one

The keystone (`12-onboarding-plan.md` E1) ships **only** the import edge
`(importerFileID → importPath)` (`02-integration-touchpoints.md:120`; typed
`{FromFile, ImportPath, StartLine}`). So the user's concern is **not an edge case —
it is the entire v1 surface.** The both-ends-peekable rows below are
PROPOSED/future, **not** in the keystone.

| Edge kind | Status | Src peekable? | Dst peekable? | Why |
|---|---|---|---|---|
| **Import → EXTERNAL package** | **v1 — SHIPS (keystone, the only edge)** | **Yes → import-site body** | **No → path-only** (see §2.3 floor) | Src = the import-statement site, a real `TokenRef` carrying `StartLine` → peekable. **Dst = a `DstPath` string** (a package path); no `(FileID,Line)` exists in this repo → `Metadata[ref]` has nothing → peek returns `"symbol not found"` (`server.go:482-483`). |
| **Intra-repo resolved** (Dst ∈ `Metadata`) | **PROPOSED — not in keystone** | Yes → body | Yes → body | If/when shipped: both endpoints are `TokenRef`s present in `Index.Metadata` → both decode → peek reads both bodies (`server.go:481`). |
| **`contains`** (method → receiver/parent) | **PROPOSED — not in keystone; today only a display string** | (n/a in v1) | (n/a in v1) | `contains` exists today **only** as `SymbolMeta.Parent`, a plain un-indexed *string* (`storage.go:78`). Promoting it to a real `TokenRef` endpoint is itself net-new. **It does NOT ship in v1.** |

### 2.2 The smoking gun is in the proposal's own keystone spec

The Dst is `importPath` — a string the recon walker *counts and discards*
(`countImportSpecs` returns an `int`, `walker.go:568`). There is no external symbol
table; the target package is **not in this repo's `Index.Metadata` at all.** So for
the keystone edge:

- **Src (import site):** real `TokenRef` → `Metadata` → peek shows the import block. ✅
- **Dst (external package):** a path string, no `TokenRef`, no `Metadata` entry,
  nothing to slice from disk → peek's per-method path cannot apply →
  `"symbol not found"` (`server.go:482-483`). ❌

**The KG addresses what grep+peek already read** — but in v1, every edge's Dst is
the asymmetric, path-only kind.

### 2.3 The true external-Dst floor (no papering)

For an external import Dst, peek-grain is conditional, and the common case has **no
peek at all**:

| External Dst grain | When |
|---|---|
| **(a) method body** | **Never.** No `TokenRef`, no `Metadata` entry — `server.go:482-483` returns `"symbol not found"` unconditionally. |
| **(b) file/package grain** | **Only if** the package is vendored / on disk / indexed in *this* repo — then you can open the package and grep within it. |
| **(c) import-path string only** | The common case: a non-vendored external module, not on disk and not indexed → **no peek and no in-repo grep target.** The fact returns the import path and nothing finer. |

### 2.4 The mitigation (already the design's stance, not a patch)

1. **Keep edges keyed by `TokenRef` at symbol grain** wherever the endpoint is
   intra-repo — that endpoint *is already a valid peek code*, no translation layer.
   (Not exercised in v1, since v1 ships only the import edge.)
2. **For external targets, stamp the grain honestly** per §2.3. The scope law
   *forbids* synthesizing the missing target ("never synthesize a target,"
   `12-onboarding-plan.md:72`; leash "NEVER add a node,"
   `.context/decisions/2026-06-11-core-competence-and-scope-line.md:26`) — so the
   path-grain Dst is **not a bug to fix; it is the boundary** of what a
   deterministic, REAL-derived edge can know.
3. **The peek-failure path IS the expected signal.** `"symbol not found"`
   (`server.go:482-483`) is the correct, designed response for an unresolved
   external Dst — at which point the agent falls back to file/package grain *if
   vendored*, else to the path string, exactly as CLAUDE.md already documents the
   fallback ("If peek returns 'symbol not found', fall back to Read at the
   `[start-end]` lines," `CLAUDE.md`).

### 2.5 A second, milder asymmetry (existing behavior, not new)

peek is gated at `MaxRange = 500` lines (`codec.go:13`). Any edge endpoint landing
on a symbol larger than that gets no peek code and falls back to Read-at-`[start-end]`
— identical to grep today. Worth stating, but it is the existing gate, not new.

---

## 3. The handoff in one picture

```
  grep authHandler              # lexical:    name        → TokenRef → peek code 2dkfzw
        │
        ▼
  aoa arch <blast-radius>       # relational: TokenRef    → set of TokenRefs (edge closure)   [PROPOSED]
        │
        ▼
  aoa peek 2dkg19 4a1b …        # content:    TokenRef    → method bodies                      [BUILT backend]
        │
        └─ external-import Dst → "symbol not found"  → package grain if vendored, else path-only
```

Every relational result **terminates in the existing peek backend** because its
output type is `TokenRef`. The KG never reads source; it hands addresses to the verb
that does. (The unified agent "tips & tricks" — verb names, chaining, and the
CLAUDE.md extension — are the subject of doc 14; the interactive inline-peek-on-click
surface is doc 15.)

---

## Built-vs-proposed ledger

**BUILT (exists in source today):**
- `TokenRef` / `SymbolMeta` shared identity — `storage.go:65-80`
- grep emits peek codes — gate `searcher.go:41`, Encode `searcher.go:43`,
  codec `internal/peek/codec.go:13-31`
- peek resolves `TokenRef` → body over the socket — `server.go:458-519`
  (Decode `:474`, ref-build `:480`, `Metadata` lookup `:481`,
  `"symbol not found"` `:482-483`, disk slice `:506-515`)
- the CLI `aoa peek` is a thin client over that same backend —
  `cmd/aoa/cmd/peek.go`
- the flat socket constant block / switch the arch methods would join —
  `internal/adapters/socket/protocol.go:38-49`

**PROPOSED (net-new, per `12-onboarding-plan.md`):**
- every edge — the keystone import edge is itself net-new extraction folded into
  `extractGo`, which does **not** currently visit imports
  (`internal/adapters/treesitter/parser.go:351-360`)
- the `EdgeStore` port + bbolt bucket (E2); the `arch` renderers/detectors (E4/E5);
  the `aoa arch` CLI + the six `MethodArch*` socket arms (E6); the MCP adapter
- **no edge exists today** — `Index` is three node-maps with zero relations
  (`storage.go:59-63`). The handoff *backend* (peek) is ready; the graph that would
  *feed* it is not yet built.

---

## Open seams (for the red-teamer)

1. **External-Dst grain is method-body *never*** — file/package grain only if
   vendored/on-disk/indexed, else path-string only with no peek and no in-repo grep
   (§2.3). This is the universal "relationships but not source lines" limit, not an
   aOa defect — naming it honestly is the point.
2. **`contains` is not in v1.** It exists today only as `SymbolMeta.Parent`, a
   display *string* (`storage.go:78`); v1 ships **no** both-ends-peekable edge —
   only the asymmetric import edge. Promoting `contains` / `calls` to real
   `TokenRef` endpoints is net-new and unspecified beyond the keystone.
3. **Cost-basis pool-doc drift.** `02-integration-touchpoints.md:80-81,120-122`
   still claims the import edge is born free in the existing parse pass; corrected by
   doc 12 E1 and §1.3 here. Fix on the next pool-doc pass.
4. **Source-vs-doc note.** `countImportSpecs` / `WalkForDimensions` is
   `//go:build !lean` (verified `walker.go` header), not `//go:build core` as doc 12
   states. Substance unchanged (still a separate recon-only walk, never the
   always-on `extractGo`); correct the build tag in doc 12 E1/E2.

---

## Where this sits among the docs

- **13 (this doc) — NOW / falsifiable.** The handoff & granularity model: the
  three-layer toolkit and the honest import-edge asymmetry, grounded in `TokenRef` /
  peek. The architecture argument.
- **14 — E6 / ships with the CLI.** The unified agent "tips & tricks": verb names,
  chaining, the CLAUDE.md guidance extension. The runtime-agent deliverable.
- **15 — Phase-2+ / gated on keystone + web/MCP surface.** Hybrid retrieval +
  inline-peek-on-click inside the interactive diagram, tying to doc 08's click-loop.
  The dashboard/MCP surface. **Read 13 and 14 as near-term; 15 is not immediate scope.**
