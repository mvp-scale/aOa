# 12 — The Element-Onboarding Plan

**Branch:** `playbook`. **Markdown only — no source changes in this doc.** This is the buildable "how we actually bring each element in" plan: engine substrate first, product/interactive surface second, in strict dependency order, MVP-first.

Every load-bearing anchor below was personally re-verified against live source on branch `playbook` (HEAD). Where the prior onboarding draft's feasibility story was wrong, the red-team's correction is folded in and the anchor re-grounded. Verdicts: **CONFIRMED** facts are stated inline; corrections to the draft are flagged **[CORRECTED]**; cross-doc defects the plan must reconcile are flagged **[FIX]**.

The binding law for every element:
- **G0** — Speed, sub-ms hot path, O(1) search, ≤+3% build budget (`GOALS.md:7`, CONFIRMED).
- **G2** — Two binaries: base `aoa` never depends on `aoa-recon` (`GOALS.md:9`, CONFIRMED).
- **G4** — Hexagonal: domain logic dependency-free, externals behind ports (`GOALS.md:11`, CONFIRMED).
- **Scope-law leash** — layer-1 facts DERIVE→REAL; layer-2 naming/grouping is INFER-leashed, "NEVER add a node" (`.context/decisions/2026-06-11-core-competence-and-scope-line.md:25-26`); the leash is absolute — anything the agent adds drops the view to MIXED (`:29`, CONFIRMED). The keystone import-edge extraction is explicitly sanctioned by the scope-law sequence (`:64-65` — "generalized import-edge extraction (tree-sitter queries) → component/DSM/cycles GREEN on any live repo", CONFIRMED).
- **Blind-judge gate** — every product UI step ports the existing blind judge: the judge receives ONLY screenshot + question + pass criterion (`playbook/standards/MODEL-STANDARD.md:43-53`, CONFIRMED).

---

## Dependency DAG (full pipeline, engine → product)

```
ENGINE                                           PRODUCT / INTERACTIVE
(E1) import-edge extraction ─┬─> (E2) EdgeStore ─┬─> (E4) arch renderers ─┐
   (net-new extractor)       │     bbolt bucket  ├─> (E5) detectors       ├─> shard JSON
                             │                   └─> (E6) aoa arch CLI ────┤
(E3) bumpRevision wiring ────┘     + socket MethodArch*                    │
   (one defer line)                            └─> (E7) thin MCP adapter   │
                                                                          │
        ┌─────────────────────────────────────────────────────────────────┘
        ▼
(P1) viewer extraction → static/arch/      (reuse //go:embed)
(P2) GET /api/arch/* handler               (reuse 304 stack)   ← needs E2 bucket filled
(P3) refresh-on-save                        (E3 + ETag poll)
(P4) before/after diff (Mode A overlay)     (E1 edges + overlay loader)
(P5) click→fact-pack→Claude loop            (E6 MethodArch* + leash precedent)
(P6) +2 AG-UI / MCP-App streaming           (last, thinnest)
[OUT] Mode C autonomous worktree            (off the ladder)
```

E1 and E3 can land in parallel — E3 is independent. Everything else has data only after E1 produces edges and E2 persists them.

---

# PART A — ENGINE SUBSTRATE

## E1 — Keystone: net-new import-edge extraction in the always-on parse pass

### [CORRECTED] The feasibility story the prior draft got wrong

The prior draft framed the keystone as cheap additive **reuse** of an existing walk: it claimed import nodes are "already being walked" by the always-on `extractSymbols`/`ParseFileToMeta` pass via `countImportSpecs`, calling that "the exact AST locus where the path string is in hand and thrown away." **That is FALSE, verified against live source:**

- `countImportSpecs` (`internal/adapters/treesitter/walker.go:568`, returns `int`) is called from **exactly one site**: `walker.go:483` (`return ctx.countImportSpecs(n) > threshold`), inside `WalkForDimensions` (`walker.go:21`). That walk is reached only via the recon adapter — `recon.Engine.AnalyzeFile` (`internal/adapters/recon/engine.go:95`) calls `WalkForDimensions` at `engine.go:186`. `engine.go` is `//go:build core` and is wired by `internal/app/dim_engine.go` (also `//go:build core`). It is a **separate full-tree walk**, not part of the symbol-extraction pass.
- The actual always-on tuned Go extractor `extractGo` (`parser.go:347`) loops `root.Child(i)` and `switch`-es on **only** `function_declaration`, `method_declaration`, `type_declaration` (verified `parser.go:351-360`). **It never visits import nodes.** No import path string is "thrown away" in the `extractSymbols` pass — there is nothing to recover.

So the keystone is **genuinely net-new extraction**, exactly as the owning spec states (`01-facts-substrate.md:240-256`): a new import extractor added alongside `extractSymbols`, both fed by the single existing `parser.Parse` tree (`parser.go:101`). `countImportSpecs` is cited only as **proof** the `import_spec` nodes are tree-reachable and cheap (the spec's own "proof, not hook", `01-facts-substrate.md:245-249`) — never as the hook.

### Integration point (verified)

The single choke point both index paths funnel through:
- Build-time: `internal/app/indexer.go:140` → `parser.ParseFileToMeta(path, source)`.
- Incremental (file-save): `internal/app/watcher.go:132` → `a.Parser.ParseFileToMeta(absPath, source)`.
- Both land in `ParseFileToMeta` (`internal/adapters/treesitter/parser.go`) → `extractSymbols` (`parser.go:235`), the dispatch over the three tuned extractors (`go`/`python`/`javascript,typescript,tsx`) + `extractGeneric` default (`parser.go:236-245`).
- All extractors already iterate the root node's top-level children — `extractGo` at `parser.go:349-360`, `extractGeneric` at `parser.go:250`. The `import_declaration`/`import_statement`/`import_from_statement` nodes sit as **siblings** of the symbol declarations in that same child list. This is where the new `case` goes (`01-facts-substrate.md:240-244`).

### Net-new vs additive

- **Net-new (the real work):** a `ports`-side edge type (`ImportEdge`/`ports.Fact` — `{FromFile, ImportPath, StartLine}`), a new `imports.go` with `extractImportsGo/Python/JS(root, source, path)`, and a `ParseFileToMetaAndFacts` that parses once (existing `parser.Parse` at `parser.go:101`) and calls **both** `extractSymbols` AND the import extractor (`01-facts-substrate.md:253-256`). `ParseFileToMeta` keeps its exact current behavior (zero regression for existing callers).
- **Additive (the cheap part):** the cleanest implementation adds an `import_declaration` arm to `extractGo`'s **existing** `root.Child(i)` loop (`parser.go:349`) so the emission rides one traversal — "one more case in loops that already run" (`01-facts-substrate.md:242`). The `Symbol` struct (`parser.go:20-27`, 6 fields, **no edge field** — CONFIRMED) is untouched; edges return on a parallel channel.

### Dependencies / sequence
First. Everything downstream is empty without it. Can land in parallel with E3.

### Constraints
- **G0 (`GOALS.md:7`):** [CORRECTED] The honest cost basis is an **added import-extraction concern on the existing `parser.Parse` tree** — NOT a free rider on an existing visit (because `extractGo` does not currently visit imports). Keep it one traversal by folding the import `case` into `extractGo`'s child loop; if instead it is a sibling walk, its cost must be measured explicitly. Drop any "rides the existing walk, no extra cost" language. The §0 obligation in doc 07 (`07-five-moats.md:55`) gates the position on landing this "inside the G0 ≤+3% budget on the three tuned extractors" — that budget still applies, now against a correctly-characterized baseline.
- **Scope law (`:25`, `:64-65`):** import edges are layer-1 "What the code IS — deps" → DERIVE → REAL, sanctioned by the scope-law keystone sequence. The leash (`:26`, "NEVER add a node") means emit edges only between files/symbols that already exist as nodes; never synthesize a target.
- **G4 (`GOALS.md:11`):** the edge/fact type lives in `internal/ports`; extraction stays in the `treesitter` adapter.
- **G2 (`GOALS.md:9`):** the keystone rides the always-on parse, NOT the recon walker. (`WalkForDimensions`/`countImportSpecs` is `//go:build core` recon-adapter code — fine to keep as proof, but the keystone must not depend on it being compiled in.)

### Exit check
`BuildIndex` on a known Go repo emits one REAL import edge per top-level import, each stamped `file:line`. **Benchmark honestly:** measure `BuildIndex` before/after the added import-extraction concern (not against a "no-op" baseline); delta ≤+3%.

---

## E2 — `ports.EdgeStore` + bbolt bucket (keyed-by-file)

### Integration point (verified)
`EdgeStore` does not exist (no hits in `internal/`). The bbolt adapter is `internal/adapters/bbolt/store.go`:
- Bucket constants: [CORRECTED] the five bucket names are `store.go:28-32` (`bucketIndex`/`bucketLearner`/`bucketSessions`/`bucketDimensions`/`bucketTelemetry`); `:33-36` are the **key** constants (`keyTokens`/`keyMetadata`/`keyFiles`/`keyState`) in the same `var` block. A new `bucketEdges = []byte("edges")` joins the `:28-32` bucket list.
- Per-project top-level bucket pattern: `proj.CreateBucketIfNotExists(bucketIndex)`; read via `proj.Bucket(bucketIndex)` (the nested-bucket + cursor API the adapter already uses).

### Net-new vs additive
- **Net-new:** a `ports.EdgeStore` interface (or methods on `ports.Storage` — the `Storage` interface closes at `storage.go:56`, `LoadTelemetry` at `:51-55`; do NOT cite `:45`) + the `bucketEdges` constant.
- **Keyed-by-file layout:** inside the per-project bucket, an `edges` sub-bucket keyed `fileID(uint32, big-endian) → serialized edges for that file`. This gives **O(edges-for-file) per-file delete**: on a save, `bucket.Delete(fileIDKey)` drops exactly that file's edges, mirroring `removeFileFromIndex`'s per-file invalidation in the watcher.

### Dependencies / sequence
After E1 (needs edges to persist). The keyed-by-file design must mirror the incremental delete path so a file edit invalidates only that file's edges.

### Constraints
- **G4:** interface in `internal/ports`; bbolt impl behind it; JSON serialization follows the existing `bbolt/encoding.go` pattern.
- **G2:** edges are core-`aoa` data (derived from the always-on parse), never gated on `aoa-recon`.
- **Crash safety:** writes transactional, matching the `SaveIndex`/`SaveLearnerState` contract.

### Exit check
Save edges for a 3-file repo; delete one file's key; `LoadEdges` returns the other two intact and the deleted file's gone in O(edges-for-that-file). Round-trip JSON parity test alongside `store_test.go`.

> **Build-time note:** the per-file key encoding (big-endian `uint32` + bbolt cursor `Seek`/prefix-scan) rests on the nested-bucket API the live adapter already uses; confirm bbolt cursor `Seek`/prefix specifics against bbolt docs before finalizing the encoding. (Context7 was not reachable in the audit environment; the design is grounded in observed in-repo usage.)

---

## E3 — `bumpRevision` wiring (the currently-absent file-save line)

### Integration point (verified — this is the single absent line doc 07 §0 names)
- `bumpRevision` def: `app.go:350` (`func (a *App) bumpRevision() { a.revision.Add(1) }`).
- **Four existing callers, CONFIRMED:** `app.go:564` and `:901` (both `defer a.bumpRevision() // L17.6: invalidate ETag cache`), `:2896` (`SetFileInvestigated`), `:2905` (`ClearInvestigated`).
- **NOT called by** `onFileChanged` (`watcher.go:20`) or `Reindex` (`app.go:2816`) — verified by reading both. So a code edit reindexes symbols but does **not** bump the ETag — the live viewer serves a stale 304 until an unrelated search/session event fires. (Note: `internal/adapters/fsnotify/watcher.go:20` is the unrelated `.venv` ignore-dirs entry, per the ledger — the file-change handler is `internal/app/watcher.go:20`.)
- The transport it feeds: `withETag` (`web/server.go:159`) returns `304` on a matching `If-None-Match` (`StatusNotModified` at `:167`, `Set("ETag", rev)` at `:170`, closes `:172`); revision source wired via `SetRevisionSource` (`web/server.go:51`), backing field `revisionFn` (`web/server.go:34`).

### [CORRECTED] Net-new vs additive — insertion site
The prior draft named only two of `onFileChanged`'s mutation paths (`watcher.go:174` symbols branch and `:78` delete branch) and omitted the **no-symbols/tokenize branch** (`Engine.Rebuild()` at `watcher.go:148`). `onFileChanged` has **three** index-mutating return paths: delete (:78), tokenize (:148), symbols (:174). A bump on only two leaves a save that produces zero symbols (or a non-parser file) serving a stale 304.

**Correct fix — one line, one site:** a single `defer a.bumpRevision()` placed right after `a.mu.Lock(); defer a.mu.Unlock()` at `watcher.go:43-44`. It covers all three paths uniformly, the mutex is already held (no new locking, mutex-safe), and it fires once per change. Apply the same single-`defer` treatment to `Reindex` (`app.go:2816`).

### Dependencies / sequence
Independent of E1/E2 — can land first or in parallel. But it is **required** before the viewer (P2/P3) is honest.

### Constraints
- **G0:** atomic `.Add(1)` — sub-ms, zero allocation, no hot-path cost.
- **G4:** stays within `internal/app` wiring; touches no adapter contract.

### Exit check
Edit a file (including one that yields zero symbols) → daemon's `revision` increments on the same tick the index updates → next `GET /api/*` with the old `If-None-Match` returns **200 with a new ETag**, not a stale 304. Regression test patterned on the existing ETag test.

---

## E4 — `internal/domain/arch` renderers (component / dsm / cycles) → shard JSON

### Integration point (verified)
`internal/domain/arch` does not exist — new domain package. Consumes edges from E2, emits content-hashed shard JSON.

### Net-new vs additive
Entirely net-new package. Three renderers: **component** (module/package grouping over the edge-set), **dsm** (design-structure matrix — `fileID × fileID` adjacency), **cycles** (SCC over the edge-set — shares the Tarjan walk with E5). Output is content-hashed shard JSON, served behind ETag.

### Dependencies / sequence
After E2 (reads edges). Build the SCC/graph-walk primitive once; E5 consumes it too.

### Constraints
- **G4 (the hard one):** `internal/domain/arch` is dependency-free domain logic — takes an in-memory edge-set (plain `ports` types), returns shard structs. No bbolt/tree-sitter/web imports. The store (E2) and surfaces (E6) wire it.
- **Scope law (`:25-26`):** the component renderer groups/names (layer-2 MIXED, leashed) over REAL edges (layer-1). Grouping may annotate but NEVER add a node — every shard node maps back to a REAL file/symbol with a `file:line` stamp.
- **G0:** renderers run at request/compact time, never on the keystroke hot path.

### Exit check
Given a fixed edge-set fixture, each renderer emits deterministic shard JSON with a stable content hash; every node carries `file:line`; re-running on unchanged input yields a byte-identical hash (so the ETag/304 path short-circuits).

---

## E5 — Detectors at compact-time (Tarjan / god / orphan)

### Integration point (verified)
No detectors exist (part of the absent `internal/domain/arch`). Compact/autotune-time hooks live in `internal/app` (autotune cadence referenced near `app.go:615`/`app.go:938`). Detectors attach to a post-index/compact pass, never the per-keystroke path.

### Net-new vs additive
Net-new detectors in `internal/domain/arch`: **Tarjan** (SCC → import cycles; shares E4's graph walk), **god** (fan-in/out degree — high-coupling nodes), **orphan** (zero in + zero out edges). Per the spec, detectors live in `detect.go` (`02-arch-service.md:169`).

### Dependencies / sequence
After E2 (edges), alongside E4 (shares SCC/graph primitives). Wired to a compact-time trigger in `internal/app`, parallel to autotune.

### Constraints
- **G0:** detectors are O(V+E) (Tarjan is linear), run at compact-time, never inside `onFileChanged`'s critical section.
- **G4:** pure domain logic in `internal/domain/arch`; the `internal/app` trigger is the only adapter-side wiring.
- **Scope law (`:25`):** cycles/god/orphan are DERIVED facts over REAL edges → REAL findings — structural derivation, not the prohibited layer-3 intent-inference.

### Exit check
On a fixture with a known 3-module cycle, one god-node, one orphan: Tarjan returns exactly that SCC, the degree detector flags the god-node, the orphan detector flags the isolated file — all `file:line`-stamped, all deterministic.

---

## E6 — `aoa arch` CLI family + socket `MethodArch*`

### Integration point (verified)
- CLI: `cmd/aoa/cmd/arch.go` does not exist — new Cobra file alongside the existing family.
- Socket: `MethodArch*` does not exist. Method constants live in the `const (...)` block at `protocol.go:38-49` (`MethodSearch` :39 … `MethodPeek` :48). The dispatch switch is `handleRequest` at `socket/server.go:206` (`switch req.Method` at `:207`); [CORRECTED] the `default` "unknown method" arm is at **`server.go:228`** (not :227).

### [FIX — align to owning spec] Method names
The owning arch-service spec defines six method names (`02-arch-service.md:127-132`): `MethodArchViews` (`arch.views`), `MethodArchView` (`arch.view`), `MethodArchFindings` (`arch.findings`), `MethodArchJourney` (`arch.journey`), `MethodArchDerive` (`arch.derive`), `MethodArchFacts` (`arch.facts`). Doc 07/08's looser `MethodArchFacts/Reach/Blast` naming should align to these six (or be marked illustrative). `Reach`/`Blast` are not in the engine spec.

### Net-new vs additive
- **Additive:** new `MethodArch*` constants appended to the `protocol.go:38-49` block; new `case MethodArch*:` arms appended to the `server.go:207` switch (mirroring `case MethodReindex: return s.handleReindex(req)`).
- **Net-new:** `cmd/aoa/cmd/arch.go` with subcommands (the grep-beaters + provenance). Each shells to the daemon via the socket method, falling back to a direct in-process call (G3 three-mode parity).

### Dependencies / sequence
After E4 & E5. Socket methods call `internal/app` → domain renderers, preserving the hexagon.

### Constraints
- **G3 (Agent-First):** `aoa arch *` must work in all three modes (direct / pipe / daemon) with consistent output/exit codes, like `grep`/`peek`.
- **G2:** `aoa arch` ships in base `aoa`, never gated on `aoa-recon`.
- **G4:** CLI and socket are adapters; no domain logic in the command file.

### Exit check
`aoa arch cycles` / `aoa arch derive` return deterministic `file:line`-stamped results in <1 ms over the live in-memory edge-set; the socket `MethodArch*` round-trips JSON; the unknown-method fallback (`server.go:228`) stays untouched for non-arch methods.

---

## E7 — Thin MCP adapter (exactly 4 grep-beating tools)

### Integration point
New `internal/adapters/mcp` package (no MCP adapter exists today). Wraps E6's socket `MethodArch*`. The 4 grep-beaters (doc 07 `07-five-moats.md:259-261`, `272-274`): **reachability** (edge walk), **blast-radius** (git-changed-files ∩ edge closure), **cycles/DSM** (Tarjan/matrix from E4/E5), **god-nodes** (fan-in/out from E5).

### Net-new vs additive
Net-new package exposing **exactly 4 tools** — a thin translation layer: MCP tool call → existing socket `MethodArch*` → response. No new compute.

### Dependencies / sequence
Last. Depends on E6.

### Constraints
- **Scope law — thinness is forced, not chosen (`:26`, "NEVER add a node"):** Tier-1 refuses call/inheritance/LLM-semantic edges and exposes only 4 tools vs the 2026 cluster's 9–45. Do not add a 5th tool that crosses into inferred edges.
- **G3:** deterministic confidence-1.0 surfaces over the same substrate the CLI uses.
- **G4:** pure adapter; imports the socket protocol, not the domain.
- **G2:** ships in base `aoa`.

### Exit check
The MCP server enumerates exactly 4 tools; each returns a `file:line:commit`-stamped result identical to the corresponding `aoa arch` CLI call; blast-radius reflects a freshly git-changed file with no stale graph.

---

# PART B — PRODUCT / INTERACTIVE SURFACE

The four human-facing elements ride the engine substrate above. Anchors for the 304 transport, the recon-investigate leash precedent, and the build-time-generator gap re-verified against live source and the integration specs.

## P-summary

| # | Element | Net-new vs reuse | Ships when | Hard gate |
|---|---|---|---|---|
| P1 | Viewer-in-product (`/api/arch/*`) | reuse `//go:embed` + 304 stack; **net-new** handler + viewer extraction | Phase ② | E1 facts in bucket |
| P2/P3 | Real-time live diagram (refresh-on-save) | reuse `withETag` 304; **net-new** = E3's one line + ETag-poll | Phase ② | the one absent `bumpRevision()` line |
| P4 | Before/after diff (overlay; Mode A/B; **C OUT**) | **net-new** overlay loader + diff renderer | Phase ② (Mode A) → ③/④ (Mode B) | E1 edges + overlay loader |
| P5 | Click→fact-pack→Claude loop | reuse `recon-investigate` POST shape; **net-new** `/api/arch/suggest` + `MethodArch*` | Phase ③ | P1+P4, E6 |

## P1 — Viewer-in-product: the daemon-served arch endpoint

**Honest current state (CONFIRMED).** The 16-view ReactFlow/elkjs viewer exists today only as a build-time Python generator — viewer JS is a string constant `JS=r"""` at `build_c4_mockup.py:408`, the HTML shell at `:1423`, the `__VIEW_INTENT__` build-time string-replace at `:1444`. It is **not** a live endpoint.

**Net-new endpoint:** `GET /api/arch/{path...}` — one Go handler serving manifest + standards + shards + journeys from the bbolt rendition bucket (`02-arch-service.md:145`; viz spec `03-visualization.md:104-120`, task V3). Viewer served at `http://localhost:{port}/arch/?model=/api/arch/manifest`; every shard fetch resolves with zero viewer changes because the viewer computes `BASE` from `MODEL_PATH` (`build_c4_mockup.py:424`).

**Reuse (CONFIRMED):** `//go:embed static` (`embed.go:7-8`) picks up `static/arch/` automatically; localhost-only listener `127.0.0.1:%d` (`server.go:69`); static handler sets `Cache-Control: no-cache` (`server.go:88`); port file at `.aoa/http.port`.

**Net-new (small):** (a) extract `viewer.js`/`index.html`/`viewer.css` out of the Python string constants into `static/arch/` with a fork-guard hash-compare test (viz V1); (b) the `/api/arch/{path...}` bbolt-reading handler; (c) vendored ESM bundle (esbuild) to drop the esm.sh CDN imports at `build_c4_mockup.py:409-414` (react 18.3.1, @xyflow/react 12.3.5, elkjs 0.11.1, htm 3.1.1) — mandatory for offline/airgapped, budget ≤2.2MB raw / ≤650KB gzipped, elkjs dominates (~1.5MB) (viz V8). elkjs as heavy layout engine + web-worker mode (`elk-worker.js` via `workerUrl`) are CONFIRMED real (kieler/elkjs).

**MVP first:** static viewer extracted + `/api/arch/manifest` + `/api/arch/{estate}/{scope}/{view}.json` serving the GREEN view set (component/dsm/cycles/code/techportfolio) for the single `local` estate, REAL-stamped. No live refresh, no click yet.

**Engine dependency:** the bbolt rendition bucket filled by the arch-service at compact time (E4/E5). Viewer is a bucket read = O(1), G0-consistent.

**[FIX — cross-spec drift, material]** The bbolt bucket is named **`arch_renditions`** at `03-visualization.md:130` but **`arch_shards`** at `02-arch-service.md:266`, `:459`, and `04-review-findings.md:80`. Pin **`arch_shards`** as canonical (3:1, owning arch-service spec uses it twice + the sibling review spec agrees). `03-visualization.md:130`'s `arch_renditions` is the outlier and should be corrected.

**Constraint:** G0 holds — rendering at compact time, not per-request (`02-arch-service.md:266`); the HTTP path is a `[]byte` bucket get.

## P2/P3 — Real-time live diagram (refresh-on-save)

**Keystone fact (CONFIRMED — re-verified, see E3).** "Real-time on save" is the one currently-absent `bumpRevision()` line. Without it a save reindexes but does not bump the ETag, so the live viewer serves a stale 304.

**Reuse (CONFIRMED):** `withETag` (`server.go:159`, 304 at `:166-167`, ETag set `:170`, closes `:172`); revision wired via `SetRevisionSource` (`server.go:51`), field `revisionFn` (`server.go:34`). The recon/dashboard GET endpoints already ride it.

**[FIX — anchors in doc 08]**
- recon GET family is **`server.go:107-110`** (recon, /summary, /tree, /findings); `:111` is the POST recon-investigate (no ETag). Doc 08 cites `:107-109` — apply the fix at doc 08 lines 49, 128, 384.
- `withETag` is **`server.go:159-172`** (func at :159, 304 at :166-167, closes :172). Doc 08's `:157-167` truncates the function before its closing brace; the viz spec's `:159-173` is already correct.

**Net-new (in order — doc 08 §2 sequence CONFIRMED):**
1. **The one line:** the single `defer a.bumpRevision()` at `watcher.go:43-44` (covers all three mutation paths — see E3's correction) + the same in `Reindex` (`app.go:2816`).
2. **ETag-poll in the viewer** (~10 lines): poll `/api/arch/manifest` every 15s with `If-None-Match`; on 200, diff shard hashes, refetch only the open view (viz V5). The footer `generated <ts> · always current` (`build_c4_mockup.py:1058`) becomes literal.
3. **Per-scope ETag** so editing `pkg/foo` doesn't invalidate `pkg/bar`'s shard (the thrash bound).

**MVP first:** items 1+2 (refresh-on-save via ETag poll, single global revision). Per-scope ETag (3) is the scale upgrade.

**Constraint:** G0 intact — "live" is honestly recompute + ETag poll, not streaming. SSE (`/api/arch/events`) is Phase ④, only if 15s latency proves annoying (viz V12).

## P4 — Before/after diff loop (overlay loader, Mode A/B; Mode C OUT)

**Load-bearing idea (CONFIRMED).** The diff is un-fakeable only if BEFORE is **derived, not drawn**: two SHA-snapshot edge-sets diffed; delta (new cycles / blast / new findings) falls out of set arithmetic over the keystone edges — no LLM pass.

**Three modes, ship in order (doc 08 §3):**
- **Mode A — proposed-edge overlay (MVP, leash-native):** Claude emits a JSON edge-patch; every endpoint is an id already in the facts; the net-new overlay loader **rejects invented ids**; the renderer re-runs the same deterministic detectors over the hypothetical edge-set. Provenance `SIMULATED · proposed`. **Ships first.**
- **Mode B — branch/worktree re-derive (high fidelity):** plan written to a branch; aOa re-derives REAL edges from that tree's AST; BEFORE=`HEAD`, AFTER=branch, same extractor both sides. Provenance `REAL · derived @ branch-sha`. After git-worktree wiring.
- **Mode C — autonomous worktree (OUT):** Claude creates the worktree and applies the plan itself. **Explicitly off the MVP ladder** — gated behind a battle-tested leash boundary ("gimmick frontier" / "shiny-object trap", doc 08 §3/§6/§8.10).

**Overlay loader (the keystone net-new piece, CONFIRMED placement):** `grouping.go` owns "path-prefix + atlas-domain grouping; overlay loader" (`02-arch-service.md:168`); §2.3 (`:240-250`) specifies schema `aoa.arch-overlay/v1` with the leash check: "any id not present in facts is rejected with a warning fact; an applied overlay drops the view to MIXED" — the exact rejects-invented-ids loader. Detectors (Tarjan SCC / god / orphan / …) live in `detect.go` (`02-arch-service.md:169`).

**MVP first:** Mode A static diff — keystone edges → live component/DSM/cycles shard through `withETag` + the one `bumpRevision()` line; Mode A overlay (Claude in Plan Mode emits the patch) → overlay loader → diff renderer → side-by-side BEFORE/AFTER. No click-fires-Claude, no AG-UI. The conformance edge-class rendering (convergent/divergent/absent ghost edge) reuses the existing `e.tag` red-dashed machinery (`build_c4_mockup.py:779-787`; viz V10).

**Engine dependency:** the keystone import edges within G0 ≤+3%, PLUS the net-new overlay loader. (CONFIRMED no edge today: `Index{Tokens, Metadata, Files}` has no edge field, `storage.go:59-63`; `Symbol` `parser.go:20-27` and `extractSymbols` `parser.go:235` emit none.)

**Constraint (leash, CONFIRMED):** the overlay never touches the substrate; applying it drops the view to MIXED (or AFTER to SIMULATED), never REAL — scope-law leash verbatim (`:26`). G0 intact: no LLM call inside the service — the loader is "a file exists or it doesn't" (`02-arch-service.md:250`).

## P5 — Interactive click→fact-pack→Claude loop (leash intact)

**Live leash precedent (CONFIRMED — re-verified).** `POST /api/recon-investigate` is registered at `server.go:111` (no ETag — POST); handler `handleReconInvestigate` defined at `recon.go:556` ([CORRECTED] :555 is the doc-comment, :556 is the `func`), which takes `{file, action}` from a UI click and mutates **annotation** via `SetFileInvestigated` (`recon.go:577`) — never the substrate. The App-side `SetFileInvestigated`/`ClearInvestigated` both call `bumpRevision()` (`app.go:2896`, `:2905`). This is the exact shape the Claude loop is built in.

**The loop (future — `MethodArch*` and `/api/arch/*` are net-new):**
```
ReactFlow onNodeClick(node) | onEdgeClick(edge) | onFindingClick(f)
  → POST /api/arch/suggest {subject, kind, scope}
      → socket MethodArchFacts/View/Findings (NET-NEW, rides server.go:207 switch)
      → returns GROUNDED fact-pack: the cycle's edges, each with file:line:commit
  → grounded fact-pack → Claude (OUTSIDE the service: CLI subprocess or MCP tool)
      → Claude returns a SUGGESTION as a proposed-edge overlay (Mode A)
  → overlay loader rejects invented ids; diff renderer computes BEFORE/AFTER
  → viewer re-renders AFTER, stamped SIMULATED · proposed
```

**Integration points (CONFIRMED):** the socket `MethodArch*` cases ride `handleRequest` (`server.go:207`, default `:228`); constants join the `protocol.go:38-49` block (use the spec's six names — see E6's FIX). `onNodeClick`/`onEdgeClick` are standard `@xyflow/react` props receiving `(event, node/edge)` (ReactFlow docs); `onFindingClick` is a custom handler (findings aren't a ReactFlow primitive) — net-new, in-bounds. The grounded fact-pack is `aoa arch facts <subject>` returning `{facts:[…source{file,line,commit}]}` — the audit trail, not "a node labeled AuthService."

**MVP first:** the +1 cut (doc 08 §6) — click → `POST /api/arch/suggest` → `MethodArch*` → grounded fact-pack → Claude → overlay → re-render. Gated on P1's MVP + the `aoa arch` engine + `MethodArch*` (E6). The +2 cut (AG-UI `STATE_DELTA`/`TOOL_CALL_START` streaming + the `ui://` MCP App inside Claude) is last and thin (P6).

**Library facts for the +2 cut (CONFIRMED):** AG-UI's `STATE_DELTA` (JSON Patch / RFC 6902) and `TOOL_CALL_START` are real AG-UI event types. MCP Apps SEP-1865 is real (MCP maintainers at OpenAI + Anthropic with the MCP-UI WG; repo `modelcontextprotocol/ext-apps`; `ui://` scheme references embedded UI resources). The doc's rendering-fidelity-unverified caveat (§8.9) is appropriate — whether a full ReactFlow/elkjs bundle renders cleanly inside Claude's iframe at hundreds of nodes is untested; keep +2 gated on it.

**Constraint (where Claude runs — restate every time):** Claude is invoked **outside** the deterministic derive path — a CLI subprocess the daemon shells out to, or an MCP host — so G0 (no network on any derive path) is intact (doc 08 §4/§5). The model produces a file (the overlay); the deterministic renderer consumes it. The model call is in the *interaction* layer, never the *fact* layer.

---

## Sequenced build order (product side, mapped to verified engine tasks)

| Step | Element | Product task | Engine dependency | Gate |
|---|---|---|---|---|
| **P1a** | Viewer | Extract viewer.js/html/css → `static/arch/` + fork-guard test | viz V1, V2 | none (pure refactor) |
| **P1b** | Viewer | `GET /api/arch/{path...}` reading bbolt **`arch_shards`** | E4/E5 fill bucket (A6); viz V3 | E1 facts in bucket |
| **P3** | Live | The one `defer a.bumpRevision()` line (`watcher.go:43-44` + `Reindex`); viewer ETag-poll | reuse `withETag` `server.go:159` | P1b |
| **P4** | Diff | Mode A overlay loader (`grouping.go`) + diff renderer + side-by-side + conformance edge classes | E4 renderers, E5 detectors; viz V10 | E1 edges + P1b |
| **P5** | Loop | `POST /api/arch/suggest` + `MethodArch*` (`server.go:207`) + grounded fact-pack → Claude (CLI/MCP, outside service) | E6; `aoa arch facts` | P4 + `aoa arch` |
| **P6** | +2 | AG-UI `STATE_DELTA`/`TOOL_CALL_START` adapter; `ui://` MCP App | E7 (MCP adapter) | P5 + AG-UI fidelity unblocked (doc 08 §8.9) |
| **OUT** | Mode C | Autonomous worktree — **not on the ladder** | — | leash boundary battle-tested first |

**Quality gate at every UI step (CONFIRMED):** the blind judge (`MODEL-STANDARD.md:43-53`) ports unchanged — the in-product viewer carries the same `auto`/`sel`/`hover` URL test hooks (`build_c4_mockup.py:1249-1252`), so CI screenshots the daemon-served `/arch/` URL and runs the judge (viz V V7.1). Label budgets renderer-enforced/lint-checked: node ≤30 / member ≤26 / edge ≤48 / ≤40 members / ≤30 nodes (`view-standards.json:17-23` CONFIRMED).

---

## Defects this plan folds in / reconciles

1. **[CORRECTED — engine blocker] E1 keystone is net-new, not reuse.** `countImportSpecs` (`walker.go:568`) is reachable ONLY via `WalkForDimensions` (`walker.go:21`, called at `walker.go:483`) → recon `engine.go:186` under `//go:build core`. The always-on `extractGo` (`parser.go:347`) switches only on function/method/type declarations and **never visits import nodes**. No import path is "thrown away" in the parse pass. The keystone is a net-new import extractor on the existing `parser.Parse` tree (one parse, two extractor calls — `01-facts-substrate.md:253-256`); `countImportSpecs` is cited only as reachability proof. The G0 ≤+3% budget is measured against an added import-extraction concern, NOT a free rider.
2. **[CORRECTED — engine/product] `bumpRevision` insertion site.** `onFileChanged` has THREE mutation paths (`watcher.go:78` delete, `:148` tokenize, `:174` symbols). A single `defer a.bumpRevision()` at `watcher.go:43-44` (mutex already held) covers all three; same for `Reindex` (`app.go:2816`). The two-site enumeration in the prior draft leaves zero-symbol saves serving a stale 304.
3. **[FIX — cross-spec drift] bbolt bucket name.** Pin **`arch_shards`** (`02-arch-service.md:266`, `:459`; `04-review-findings.md:80`); correct `03-visualization.md:130`'s `arch_renditions` (outlier, 1 of 4).
4. **[FIX — doc 08 anchors]** recon GET range `:107-109` → **`:107-110`** (lines 49, 128, 384); `withETag` `:157-167` → **`:159-172`**.
5. **[FIX — align doc 07/08 to owning spec]** socket method names: the spec defines six (`MethodArchViews/View/Findings/Journey/Derive/Facts`, `02-arch-service.md:127-132`); `Reach`/`Blast` are illustrative-only.
6. **[FIX — doc 07 §0 void anchor]** doc 07 §0 (`07-five-moats.md:44`) cites the parser as `internal/domain/index/parser.go` (does not exist — parser is `internal/adapters/treesitter/parser.go`) and claims that parser "visits but never emits edges." The visit it means (`countImportSpecs`) is in the **recon walker**, not the parser; the always-on parser does not visit imports at all. Correct to: "the recon dimension walker (`walker.go:568`, `WalkForDimensions`) already traverses `import_spec` nodes for the import-bloat rule; the always-on `extractSymbols` pass does not" (matching `01-facts-substrate.md:245-248`).
7. **[minor — engine anchors]** socket `default` arm at **`server.go:228`** (not :227); bbolt bucket constants at **`store.go:28-32`** (`:33-36` are key constants, `bucketEdges` joins the :28-32 list); `handleReconInvestigate` def at **`recon.go:556`** (:555 is the doc-comment).

**Scope-law / leash — CONFIRMED clean (no violations).** E1 import edges are layer-1 DERIVE→REAL, sanctioned by the scope-law keystone sequence (`:64-65`); the leash "NEVER add a node" (`:26`) is correctly applied to emit edges only between existing nodes; E7's MCP thinness (exactly 4 tools, no inferred edges) is leash-forced; P5's Claude loop keeps the LLM call OUTSIDE the derive path (CLI subprocess / MCP host), with `recon-investigate` (`recon.go:556` → App `SetFileInvestigated`, mutates annotation never the substrate) as the verified live analog. Detectors run at compact-time, never in `onFileChanged`'s critical section.

**Premise correction (matches the ledger):** the brief's claim that `STRATEGIC-POSITION.md` is "the PRE-round-2 draft" is FALSE — its line 1 declares the round-2 revision and §C exists; docs that cite it for §C content do so correctly.

Sources (external, CONFIRMED): [kieler/elkjs](https://github.com/kieler/elkjs); [React Flow layouting](https://reactflow.dev/learn/layouting/layouting); AG-UI protocol + MCP Apps / SEP-1865 (`modelcontextprotocol/ext-apps`).
