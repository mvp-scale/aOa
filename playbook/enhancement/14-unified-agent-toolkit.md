# 14 — The Unified Agent Toolkit: grep + graph + peek ("tips & tricks," extended)

**Status:** PROPOSED guidance, grounded in BUILT primitives. Lifecycle stamp: **ships with E6** (the `aoa arch` CLI + `MethodArch*` socket arms, `12-onboarding-plan.md:181`). Every code claim cites `file:line` against branch `playbook` HEAD; every proposal claim cites the pool doc; external claims cite URL.

**Companion docs:** read this alongside **13** (the handoff & granularity model — the falsifiable architecture argument) and **15** (inline-peek inside the interactive diagram — Phase-2+). This doc is the *runtime guidance* deliverable: the prose that actually extends the CLAUDE.md `grep → peek` block. Its audience is the agent at runtime, not the validator.

**The one-line thesis:** grep+peek STAY primary. The graph adds *only* the relational verbs grep structurally cannot form, and every relational verb **terminates in peek** by handing back the same `TokenRef{FileID, Line}` identity (`internal/ports/storage.go:65-69`) that grep already emits and peek already resolves to a body (`internal/adapters/socket/server.go:458-519`). The KG addresses the line; peek reads it. Symmetric to how grep+peek already compose.

---

## 1. The division of labor — three verbs, one identity, one daemon

aOa already runs a two-verb composition. The graph inserts a third verb *in the middle*, at a different altitude. All three speak one location identity and ride one daemon.

| Verb | Question | Input → Output | Altitude | Status |
|---|---|---|---|---|
| **grep** | *where is this token?* (lexical) | `token → []TokenRef` | textual occurrence | **BUILT** — O(1) `Tokens` map (`internal/ports/storage.go:60`); a hit emits a base36 peek code (`internal/domain/index/searcher.go:43`) |
| **graph** | *what connects to this?* (relational) | `TokenRef → []TokenRef` (edge closure) | structural — in/out-degree, reachability, blast-radius, cycles | **PROPOSED** — no edge representable today; `Index` is three node-maps (`storage.go:59-63`); `SymbolMeta.Parent` (`:78`) is a display *string*, not traversable |
| **peek** | *show me the body* (content) | `TokenRef → source lines` | the literal method body | **BUILT** — `internal/adapters/socket/server.go:458-519` (CLI is a thin client over the same backend, `cmd/aoa/cmd/peek.go:27`) |

**Why they compose.** The shared identity is `TokenRef{FileID uint32, Line uint16}` (`storage.go:65-69`) — the key type of the entire persisted model: `Tokens map[string][]TokenRef` (`:60`) and `Metadata map[TokenRef]*SymbolMeta` (`:61`), where `SymbolMeta` carries the body coordinates `StartLine`/`EndLine` (`storage.go:76-77`).

- **grep does token→location** via `Tokens`, emitting `peek.Encode(fileID, startLine)` (`searcher.go:43`; codec `internal/peek/codec.go:13-31`).
- **peek does location→body**: it decodes the *same* code back to `(fileID, startLine)` (`server.go:474`), rebuilds the exact ref grep encoded (`server.go:480`), looks up `s.idx.Metadata[ref]` (`server.go:481`), and slices `allLines[sym.StartLine-1 : sym.EndLine]` off disk (`server.go:506-515`).
- **A graph verb whose output IS a set of `TokenRef`s plugs into that same backend with zero new read machinery.** It is the relational layer *on top of* grep+peek.

**One daemon, one freshness signal.** Edges are index data, produced from the same `parser.Parse` tree as the grep answer (`internal/adapters/treesitter/parser.go:101`) and re-derived on save via `ParseFileToMeta` in `onFileChanged` (`internal/app/watcher.go:132`). So the grep answer and the reachability answer cite the **same** `file:line:commit` and invalidate together (`01-knowledge-graph-and-visualization.md:243-249`). Honest cost note: the import extractor is genuinely net-new — `extractGo` switches only on function/method/type declarations and **never visits imports today** (`parser.go:347-363`), so the keystone edge is added extraction folded into the existing traversal, budgeted at G0 ≤ +3% (`12-onboarding-plan.md:71`), not free.

---

## 2. The new relational verbs — what they add, and when each wins

The graph exposes *only* the query classes that genuinely beat grep and never degrade into a worse, stale grep (`03-access-surface.md:211-228`). Four capabilities — the **grep-beaters** (`12-onboarding-plan.md:203,205`):

| Capability | The question grep cannot form | Why grep structurally can't | When it wins |
|---|---|---|---|
| **Reachability** (shortest-path) | "how does A reach B?" | transitive-closure; answer length unbounded a priori; an agent fakes it only by recursively grepping each callee N hops deep — and **cannot prove "no path exists"** (`03:220`) | multi-hop "does this connect to that" / dead-path proofs |
| **Blast-radius** (reverse-deps / affected-set) | "what breaks if I change X?" | grep finds *forward* literal occurrences; reverse transitive dependency + set-intersection across a changeset are edge-closure ops grep has no notion of (`03:221`) | impact analysis before an edit; PR triage |
| **Cycles** (Tarjan SCC) | "are there dependency cycles?" | a cycle is a topological property with no string to match (`03:225-228`) | refactor planning; finding tangles |
| **God-nodes** (degree-centrality) | "what's most connected?" | degree-centrality is a global graph property; grep counts textual occurrences, not structural in/out-degree (`03:222`) | orientation in an unfamiliar codebase |

**Verb naming — OPEN SEAM, do not hardcode yet.** The owning engine spec defines **six** canonical methods: `MethodArchViews`, `MethodArchView`, `MethodArchFindings`, `MethodArchJourney`, `MethodArchDerive`, `MethodArchFacts` (`12-onboarding-plan.md:181`). The looser `Reach`/`Blast` naming used in docs 03/07/08 (e.g. `aoa arch reach` / `MethodArchReach`, `aoa arch blast` / `MethodArchBlast`, `03-access-surface.md:218-220`) is **illustrative, not in the engine spec.** Reconcile the CLI/socket verb names to the six canonical ones **before** the CLAUDE.md extension hardcodes anything. Below, `<reach>` / `<blast-radius>` are placeholders for whichever canonical verb ships.

**Where grep+peek STAY primary (the discriminator):**
- **Lexical / precision** — you have a name or a string → `grep` is the default verb (`03:190`). Never reach for the graph here.
- **1-hop "show X and its neighbors"** — fresh `grep → peek` wins; reaching for a graph verb risks the **stale-graph trap** (`01:211`; `03:240`). Only go relational when the question is genuinely *transitive or topological*.

---

## 3. The chaining pattern — every relational verb terminates in peek

The graph never replaces the read. It hands back `TokenRef`s; peek reads the bodies. This mirrors the existing CLAUDE.md `grep → peek` two-liner, extended by exactly one verb in the middle:

```
$ grep authHandler              # lexical: find the symbol        -> peek code 2dkfzw
$ aoa arch <blast-radius> 2dkfzw   # relational: who depends on it -> a set of peek codes
$ aoa peek 2dkg19 4a1b ...      # content: read each affected body
  # external-import refs in the set return "symbol not found"
  #   -> read at package grain if vendored, else the import path is all you get
```

**`facts` is the audit handoff.** `aoa arch facts <subject>` returns `{facts:[… source{file,line,commit}]}` (`12-onboarding-plan.md:305`); that `source` pointer *is* a peek-resolvable ref for intra-repo subjects. So: **the graph names a relation, `facts` hands you the addresses, peek shows the bodies.** The whole arch surface is a thin delegate riding the existing flat socket switch beside `MethodSearch`/`MethodPeek` (`server.go:206-230`, default keyword `:228`, return `:229`; constant block `internal/adapters/socket/protocol.go:38-49`) — *added beside, never instead of* (`03:200-205`).

### The honest caveat the agent must know — not every graph result peeks to a body

The edge shape is `Edge = {Src TokenRef, Dst TokenRef|DstPath, Kind, prov}` (`01-knowledge-graph-and-visualization.md:76`; `02-integration-touchpoints.md:73`). The `|DstPath` union IS the asymmetry, and it is the user's exact concern:

| Edge | Status | Src peekable? | Dst peekable? |
|---|---|---|---|
| **Import → external package** | **v1 — the only edge that ships** | **Yes** → import-site body (a real `TokenRef` with `StartLine`) | **No** → `DstPath` is a package *path* string, not in this repo's `Metadata`; `server.go:482-483` returns `"symbol not found"` |
| **Intra-repo resolved** (Dst ∈ `Metadata`) | PROPOSED — not in keystone | Yes → body | Yes → body (both ends decode via `server.go:481`) |
| **`contains`** (method → parent) | PROPOSED — today only `SymbolMeta.Parent`, a display string (`storage.go:78`) | n/a in v1 | n/a in v1 |

**In v1, every edge's Dst is the asymmetric, path-only kind** — so the user's concern is not an edge case, it is the entire v1 surface. The external-Dst floor:
- **(a) method body** — *never*. No `TokenRef`, no `Metadata` entry → `server.go:482-483` returns `"symbol not found"` unconditionally.
- **(b) file/package grain** — *only if* the package is vendored / on disk / indexed in this repo; then open it and grep within.
- **(c) import path string only** — the common case (non-vendored, off-disk, not indexed): no peek, no in-repo grep target.

`"symbol not found"` is the **correct, designed signal** for an unresolved external Dst — identical in spirit to the fallback the existing CLAUDE.md block already documents ("If peek returns 'symbol not found', fall back to Read," `CLAUDE.md:26`). The scope law forbids synthesizing the missing target (`12:72`; `2026-06-11-core-competence-and-scope-line.md:26`), so the path-grain Dst is the *boundary* of a deterministic REAL-derived edge, not a bug. (A second, milder limit: peek is gated at `MaxRange = 500` lines, `internal/peek/codec.go:13`; any endpoint on a larger symbol falls back to Read-at-`[start-end]`, exactly as grep does today.)

---

## 4. The CLAUDE.md guidance-block extension

The existing block is a two-verb composition with the three ingredients the user's own notes confirm *work*: a concrete output example, the `grep → peek` 2-liner, and the peek-failure → Read fallback. The extension preserves that shape and adds the *when-to-reach-for-graph* discriminator plus the external-Dst caveat. **Do not paste the placeholder verb names; substitute the canonical `MethodArch*` CLI surface once §2's naming seam is resolved.**

> ### Relational queries — `grep → graph → peek`
>
> grep stays your default verb. Reach for a **graph verb only when the question is transitive or topological** — something grep cannot form by lexical search:
>
> - **reachability** — "how does A reach B?" / "is there *any* path?" (grep can't prove no-path)
> - **blast-radius** — "what breaks if I change X?" (reverse transitive deps)
> - **cycles** — "are there dependency cycles?"
> - **god-nodes** — "what's the most-connected symbol?" (orientation)
>
> Do **NOT** reach for a graph verb for 1-hop "show X and its neighbors" — fresh `grep → peek` wins there.
>
> Every graph verb returns peek codes. Chain straight into peek:
>
> ```
> $ grep processTaintBaseEviction       # lexical -> 2dkfzw [979-1068]
> $ aoa arch <blast-radius> 2dkfzw      # relational -> set of refs that depend on it
> $ aoa peek 2dkg19 4a1b                # content -> bodies of the affected methods
>   # external-import refs in the set return "symbol not found"
>   #   -> read at package grain (if vendored) else the import path is all you get
> ```
>
> peek resolves **intra-repo** refs to method bodies. An **external import-edge Dst** is a package path, so peek returns `"symbol not found"` — that is correct, not an error: fall back to package grain if vendored, else the import path is the finest grain available.

Add to the existing **Commands** table (verb names pending §2 reconciliation):

| Task | Command | Example |
|------|---------|---------|
| Reachability | `aoa arch <reach> A B` | does auth reach the DB layer? |
| Blast-radius | `aoa arch <blast-radius> code` | what depends on this symbol? |
| Cycles | `aoa arch <cycles>` | find dependency tangles |
| Audit facts | `aoa arch facts subject` | relation + `file:line:commit` to peek |

---

## 5. Built-vs-proposed ledger

- **BUILT (the handoff backend is ready):** `TokenRef`/`SymbolMeta` identity (`storage.go:65-80`); grep emits peek codes (gate `searcher.go:41`, Encode `:43`, codec `peek/codec.go:13-31`); peek resolves ref→body over socket (`server.go:458-519`: Decode `:474`, ref-build `:480`, `Metadata` lookup `:481`, slice `:506-515`) and CLI (`cmd/aoa/cmd/peek.go:27`); the flat socket switch the arch methods join (`server.go:206-230`; constants `protocol.go:38-49`).
- **PROPOSED (net-new, per `12-onboarding-plan.md`):** every edge (the keystone import edge is itself net-new extraction — `extractGo` does not visit imports, `parser.go:347-363`); the `EdgeStore` port + bbolt bucket; the `arch` detectors/renderers; the `aoa arch` CLI + six `MethodArch*` socket arms; the MCP adapter. No edge exists today — `Index` is three node-maps with zero relations (`storage.go:59-63`). **The handoff backend (peek) is ready; the graph that would feed it is not yet built.**

## 6. Open seams for the red-teamer

1. **Verb naming.** `Reach`/`Blast` (docs 03/07/08) are not the engine spec's six `MethodArch*` methods (`12-onboarding-plan.md:181`; `03:218-220`). Resolve before the CLAUDE.md extension hardcodes a verb that won't exist — every placeholder above (`<reach>`, `<blast-radius>`, `<cycles>`) is gated on this.
2. **`contains` peekability.** `contains` is currently only `SymbolMeta.Parent`, a display string (`storage.go:78`); promoting it (or `calls`) to a real `TokenRef` endpoint is net-new and unspecified beyond the keystone. v1 ships **no** both-ends-peekable edge.
3. **External-Dst grain.** The §3 floor: method-body *never*, file/package grain *only if vendored*, else path-string only with no peek and no in-repo grep target.
