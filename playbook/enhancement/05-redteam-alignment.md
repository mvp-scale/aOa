# 05 — The Red-Team & Alignment Record

**Status:** the trust appendix for the enhancement position. This document does
not propose anything new. It exists to answer one question — *why should we
believe the plan in `ENHANCEMENT-GUIDE.md` and the `integration/` specs?* — and
it answers it the only way this playbook accepts: by recording every adversarial
attack the position survived, and by walking the final direction past each
binding law (G0–G6, the scope ladder, hexagonal G4) line by line, plus the
explicit list of things we are choosing **not** to build and why.

**Read order:** this is the last door, not the first. The argument lives in
`ENHANCEMENT-GUIDE.md`; the implementation detail lives in
`integration/01-facts-substrate.md`, `02-arch-service.md`, `03-visualization.md`;
the binding laws are `.context/GOALS.md`,
`.context/decisions/2026-06-11-core-competence-and-scope-line.md`,
`playbook/standards/view-standards.json`, and
`playbook/standards/MODEL-STANDARD.md`. This file curates the three rounds of
red-team that hardened that argument and turns the verdicts into a standing
checklist.

**The document's own contract** (inherited from the guide): every load-bearing
claim cites a `file:line` or a source doc. *If a cited anchor is wrong, the claim
built on it is void.* That rule is exactly why the red-team rounds below kept
firing on anchor precision even after the strategy was accepted — in a falsifiable
document, a citation that points at the wrong subsystem is a defect of the same
class as a wrong strategy.

---

## 1. What survived, and what the attack actually killed

Across three rounds the **spine was attacked directly and held**; what failed was
never the strategy, it was specific cited anchors and a handful of seam-level
contradictions. That distinction is the whole point of this record: a falsifiable
document earns trust precisely by showing which of its own claims got cut.

**The spine that survived every round (verified against live code each time):**

- `ports.Index` is exactly three node-shaped maps with **zero** edge primitives
  (`internal/ports/storage.go:59-63`; `Parent` is a display string, `:78`). The
  "a KG adds exactly one new shape — a typed relation" claim is structurally
  sound.
- The keystone's two-site distinction is **correct and precisely pinned**:
  `ParseFileToMeta` calls only `extractSymbols` and never the walker
  (`parser.go:104,108`); `extractGo` switches on exactly three node kinds with no
  `import_declaration` (`parser.go:347`); `countImportSpecs` returns an `int` and
  discards names (`walker.go:568-583`), reached **only** via `walkContext.walk`
  (`walker.go:54`) — the dimensions engine, not the index pass.
- The socket is a flat method switch where an arch method `case` (e.g. `MethodArchDerive:`; reach/blast are CLI aliases per ADR 2026-07-02) is genuinely a
  one-liner (`socket/server.go:207`), with no JSON-RPC envelope — so the
  native-first / MCP-last latency argument is grounded, not asserted.
- The graphify anchors all verify (`serve.py:822` shortest_path, `:775-780`
  god_nodes, `:144`/`:717-718` un-indexed scan, `:19` build-artifact load) — so
  the "narrow value, structurally stale" read is fair.
- The blind-judge gate is real and falsifiable
  (`MODEL-STANDARD.md:43-53`) — the moat claim stands.

**What the attack actually killed (and the guide now carries the fix):** a
wrong-path/wrong-subsystem language anchor, a G0 over-claim that collapsed two
different tree-sitter passes into "one pass," an unsurfaced hot-path write cost,
a self-contradiction between the locking law and the keystone, a strengthened
quote of binding ADR text, a phase-scope contradiction, and several anchor
imprecisions. None of these sank the direction. All of them sank a *specific
cited claim* — which is the failure mode the document exists to catch.

---

## 2. The round-by-round record

Three rounds, all returning **NEEDS_WORK** until the final integration. Each
finding is recorded with its severity, the live-code verification that fired it,
and how the final position answers it. This is the curated audit trail; the raw
verdicts live in the guide's own revision summary.

### Round 1 — the anchors and the G0 keystone claim

> **Verdict:** strategically sound, *fails its own falsifiability contract in two
> load-bearing places, plus one architectural error weakening the central G0
> safety claim.* The architecture survives; several anchors and the §4/§5
> language story do not.

| # | Sev | Finding | Verified by | How the final position answers it |
|---|-----|---------|-------------|-----------------------------------|
| 1.1 | **blocker** | **Keystone G0 claim partly false** — the two insertion sites are in two *different* passes, not "one pass." The draft said "either way the edge is born inside the existing single tree-sitter pass." | `ParseFileToMeta` (`parser.go:104`) calls `extractSymbols` only and never the walker; `countImportSpecs` is reached only via `walkContext.walk` → the dimensions engine (`dim_engine.go:200 SaveAllDimensions`), recon-gated, *not* the index build. | Guide §5.1 now states plainly: **site (a)** (`extractSymbols` on the always-on `ParseFileToMeta`/`indexer.go:140` path) is the **only** G0-free choice and the recommendation; **site (b)** (`countImportSpecs`, dimensions/recon pass) would either pull recon into the base binary (G2 violation) or force a second walk (G0 violation). The "no second walk" guarantee is attached to site (a) **alone**. |
| 1.2 | **blocker** | **Language-ladder anchor wrong path + subsystem conflation.** Draft cited the "10 tuned" to the wrong path and claimed the keystone inherits the walker's 10. | `langOverrides` (`internal/domain/analyzer/lang_map.go:48-143`, ten keys) tunes the *walker's* concept resolution; the keystone rides `parser.go`'s `extractSymbols`, whose switch has only **three** tuned branches (`extractGo`/`extractPython`/`extractJavaScript` shared by js/ts/tsx) + `extractGeneric`. | Guide §4 now splits the two subsystems explicitly: **(a) walker concept-resolution tuning = 10** vs **(b) symbol/import EXTRACTION tuning = 3**. REAL-stamped import-edge parity is initially **Go / Python / JS-TS-TSX**, not 10, not 509. Path corrected everywhere. |
| 1.3 | major | **Appendix anchors drift from the cited line** — `app.go:698` was cited as "fsnotify → reindex" but is `Watcher.Watch(...)` setup, not the reindex handler. | The per-file reindex is `onFileChanged` (`watcher.go:20`), serialized at `:43`. | Guide re-pins the freshness anchor to `onFileChanged` (`watcher.go:20`) and the `a.mu.Lock` site `:43-44`; `app.go:698` retained only as the wire-up. Storage normalized to `:59-63`. |
| 1.4 | minor | **G4: the off-interface shortcut is flagged but also leaned on.** | `SaveAllDimensions`/`LoadAllDimensions` are concrete-only (`bbolt/store.go:461`/`:488`), absent from the `Storage` interface (`storage.go:12-56`). | Guide §5.3 ties site (b) to this shortcut: choosing it is *both* a second/recon-gated pass **and** the G4-dirtier route. `define ports.EdgeStore first` becomes a hard **precondition** on the keystone task, not a caveat. |
| 1.5 | minor | **Freshness-"for-free" over-claim** — true only at site (a). | At site (b) the dimensions pass has its own recon-gated update path (`dim_engine.go:222 updateDimForFile`). | Guide §0.3 / §5.2 make "freshness for free" an explicit **consequence of the site-(a) choice**, never a property of either site. |

### Round 2 — the locking-law contradiction

> **Verdict:** survives a real attack on its spine — `ports.Index` is three
> node-maps, the keystone two-site distinction is correct and precisely pinned,
> graphify and blind-judge anchors verify. *What it gets wrong is at the seams:
> one genuine contradiction between the draft's own cited locking law and its
> App.mu claim, plus small provenance slips.*

| # | Sev | Finding | Verified by | How the final position answers it |
|---|-----|---------|-------------|-----------------------------------|
| 2.1 | **major** | **App.mu locking-law contradiction.** The per-file edge upsert must run under `App.mu`, yet the draft's own source says "no arch/facts write ever holds App.mu." | `onFileChanged` holds `a.mu.Lock` (`watcher.go:43-44`); `ParseFileToMeta` runs inside that locked section (`watcher.go:132`); locking law at `00-OVERVIEW.md:101`. | Guide §5.1 draws the line the draft elided: the **import-edge FACT is index data** — it rides the existing `SaveIndex` write under `App.mu`, exactly as every symbol already does; that *is* what makes freshness free. The "arch/facts writes" the law governs are the **derived** renditions (shards, DSM, cycles, conformance), computed off the hot path and served daemon-first. Reconciliation noted for the board: `00-OVERVIEW.md:101` should read "no **derived** arch/facts write holds App.mu." |
| 2.2 | minor | **Scope-law quote strengthened beyond source** — draft quoted "never add a node/edge"; the ADR says only "never add a **node**." | ADR `2026-06-11-core-competence-and-scope-line.md:26` says "NEVER add a node" verbatim; `03-visualization.md:343` carries the "/edge" wording. | Guide §2.1 quotes the ADR verbatim and labels the edge-prohibition the **draft's own derived corollary** (edges are layer-1 REAL, so an agent adding one invents structure) — not the ADR's words. |
| 2.3 | minor | **"SupportedLanguages() test asserts exactly these 10" is unbacked.** | The 10 keys are real (`lang_map.go:48-143`); no test asserts the count. | Guide grounds the "10" to the **map literal**, not a test, and drops the false test attribution. |
| 2.4 | minor | **"66 unlabeled members fails the gate" over-reads the anchor.** | `manifest.json:366` shows `8 buckets · 66 components · 10 edges` for a *bucketed* view, `prov: simulated` (`:369`); nothing there records a judge failure. | Guide §0.2 softens to "a view at member grain (66 components) **risks** failing the readability gate, which is why the deriver aggregates to bucket grain first" — illustrative, not a recorded verdict. |
| 2.5 | minor | **The "stale 28" flag misses the guide's own primary source.** | `00-OVERVIEW.md:81` says "28, one walker" — a doc the guide cites repeatedly as authority. | Guide §4 adds `00-OVERVIEW.md:81` to the stale-doc reconciliation list alongside README.md:544 + CLAUDE.md; the §4 ladder is the correction that must reach back to all four sources. |

### Round 3 — the hot-path write cost and the phase contradiction

> **Verdict:** strong round-3 draft; **every load-bearing `file:line` checked is
> accurate.** G0/G2/G4/scope-law alignment is handled with real care. *Where it
> still strays: unbacked perf claims on the write path, an MCP-vs-faster framing
> gap, and a Phase-② scope contradiction it flags but doesn't resolve.*

| # | Sev | Finding | Verified by | How the final position answers it |
|---|-----|---------|-------------|-----------------------------------|
| 3.1 | **major** | **Unbacked hot-path write cost** — the incremental edge upsert inherits a linear scan the draft never surfaced; a flat `[]Edge` makes per-file delete O(all edges) on every keystroke. | `onFileChanged` already does **two** full-map scans of `a.Index.Files` per edit: find-ID (`watcher.go:65`) and alloc-ID (`watcher.go:110`). | Guide §1/§5.1/§5.2 now state: `[]Edge` is the **logical** shape; the **storage** layout must be keyed for per-file deletion (`map[FileID][]Edge` + inbound index) so `onFileChanged` drops-and-re-emits one file's edges in **O(edges-for-that-file)**, never O(all edges). Enforced at the port via `DeleteEdgesForFile(FileID)` (§5.3). This is the G0-relevant **write** number the prior draft omitted. |
| 3.2 | minor | **"MCP vs faster" dodged.** Draft proved MCP can't beat the socket, then recommended building MCP — without stating the consequence. | `socket/server.go:207` flat switch vs MCP stdio+JSON-RPC handshake/session overhead sits structurally above it. | Guide §3.2 adds the synthesis: MCP is **not the faster move and is not meant to be — it is the wider move**, late and thin. Justified ONLY as a reach/compatibility surface for agents that can't shell out; **never** fronts a latency-sensitive query. |
| 3.3 | minor | **Phase-② scope contradiction flagged but not resolved** — §10 said "~1–2 wk" while §6 assigned 5–7 views to ②. | A reviewer correctly flagged ~10 eng-wk if ② meant 5 views; the M-effort component view + 3 detectors + socket + self-test alone approaches that. | Guide §6/§10 resolve by **scoping, not re-estimating**: ② = exactly the three keystone-derived views that share one edge set (**component + dsm + cycles**, ~1–2 wk); `code`/`domains` → **②b** (cheap, GREEN, keystone-independent); `techportfolio`/`sbom` → **③** (need a new lockfile parser). Tables now match prose. |
| 3.4 | minor | **Provenance over-generalized** — the "makes inference safe to ship" framing applied uniformly, including to REAL edges where the stamp is closer to decoration. | A layer-1 DERIVE edge has no inference to leash; its `{file,line,commit}` is just its own source location. | Guide §2.1 splits the claim by layer: on **layer-1 REAL edges** the stamp is an **audit/freshness anchor** (powers `aoa arch facts` + re-derivation), *not* a leash; on **layer-2 MIXED content** it is the load-bearing mechanism that makes naming/grouping safe. Both kept, no longer conflated. |
| 3.5 | minor | **Blind-judge anchor imprecision** — cited `:44-54`/`:18-54`; the gate is `:43-53`. | `MODEL-STANDARD.md:43-53` is the "### 3. Blind judge" section; `:18-42` is lint+render, line 44 blank. | Guide tightens to `:43-53` for the gate; the broader `:18-54` is relabeled "the 3-step gate process (lint → render → judge)." |

**Pattern across all three rounds:** zero findings against the *strategy*
(KG-subordinate-to-grep, MCP-thin-and-late, scope-law fences, keystone-at-the-
parse-pass). Every finding was an anchor, a seam, or a quantitative claim — the
class of defect a falsifiable document is built to surface and the reason it took
three rounds to go green.

---

## 3. Alignment checklist — G0–G6, scope law, hexagonal

The standing check. Each row states the goal/law, the proposed direction's
position, the verification, and the verdict. A red-teamer should attack this
table first.

### 3.1 Goals (G0–G6, `.context/GOALS.md`)

| Goal | Constraint | The position's answer | Verified by | Verdict |
|------|-----------|----------------------|-------------|---------|
| **G0** | Speed — sub-ms reads, no avoidable O(n) on hot paths, **≤+3% index build** | Keystone rides the **always-on** `extractSymbols` pass — no second walk, no second file read; reads are O(edges) with a cached laid-out shard; **the write/invalidation path is bounded to O(edges-for-that-file)** by a keyed-by-file store, so per-edit reindex adds no full edge-set scan. Detectors run at compact-time, off the hot path. | always-on path `parser.go:104,108`→`indexer.go:140`; ≤+3% budget `00-OVERVIEW.md:99`; two existing full-map scans the upsert must not add a third to (`watcher.go:65,110`) | **ALIGNED** — site (a) only; the G0-relevant write number is now explicit. |
| **G1** | Parity — zero behavioral divergence from Python; fixtures are truth | Net-new arch surface only; no change to the search/learner paths fixtures cover. Edge facts are deterministic AST output, themselves fixture-able. | scope is additive (§5.2 seams); no fixture path touched | **ALIGNED** — orthogonal to parity. |
| **G2** | Two binaries, clean split — `aoa` must **never** depend on `aoa-recon` | Keystone is rejected at site (b) **precisely because** it would pull the recon-gated dimensions pass into the base binary. Site (a) lives entirely in the standalone index build. | site (b) recon-gating `dim_engine.go:200`; site (a) base-path `parser.go`/`indexer.go` | **ALIGNED** — G2 is the named reason site (b) is refused. |
| **G3** | Agent-First — drop-in grep/egrep/find; agents never know it's not GNU grep | grep→peek stays the **default verb** (freshness + per-method `[start-end]` granularity); the KG adds only the connectivity verbs grep structurally can't form. Native socket/CLI is the agent contract, already won via the shim. | shim precedent `GOALS.md:10`; granularity below the graph's coarsest node (peek vs module-grain DSM) | **ALIGNED** — KG subordinate to grep, not a replacement. |
| **G4** | Clean Architecture — hexagonal, domain dependency-free, behind interfaces | **Precondition:** define `ports.EdgeStore` (with `DeleteEdgesForFile`) **first**; the bbolt adapter implements it — never the off-interface `SaveAllDimensions` shortcut. Socket/web/MCP all delegate to one app service, never reach into the domain. | off-interface caveat `bbolt/store.go:461`/`storage.go:12-56`; port-first gate §5.3 | **ALIGNED — conditional on the port precondition holding.** |
| **G5** | Self-Learning | Untouched; the learner/autotune path is orthogonal to the arch substrate. | no learner path in any seam | **N/A — no interaction.** |
| **G6** | Value Proof — surface measurable savings | The diff/blast-radius pack and the "answer = diagram, one truth" framing are direct value surfaces; provenance makes savings auditable. | evidence packs §8; diff wedge `research:174-176` | **ALIGNED** — strengthens value proof. |

### 3.2 Scope law (`2026-06-11-core-competence-and-scope-line.md`)

| Ladder layer | The position's discipline | Verdict |
|---|---|---|
| **Layer 1 — DERIVE / REAL** | Import edges are deterministic AST output, stamped REAL with `{file,line,commit}` as an audit/freshness anchor. Cycles/DSM/god_nodes are pure derivations of the same edge set. | **IN — the core build.** |
| **Layer 2 — INFER, leashed / MIXED** | Agent may **name/group/annotate** extracted facts, **never add a node** (ADR `:26` verbatim). Every named bucket pins back to REAL facts; the provenance stamp is what makes the leash checkable. Grouping/domain/verb rows are stamped MIXED. | **IN — leash enforced by provenance.** |
| **Layer 3 — DECLARE/INGEST, diff vs layer 1** | Conformance is **declare-and-diff** (`.aoa/arch.yaml` template diffed against derived edges), never pattern detection. Runtime/cost/business data ingested with OBSERVED provenance only. | **IN — diff only; detection is OUT (§4).** |

The scope-law **test** ("rendition of facts we derived, or diff against a
declaration?") is the gate every view passes; anything else gets a provenance
stamp saying what it is, or it doesn't ship.

### 3.3 Hexagonal (G4, restated as a seam discipline)

- **Domain stays dependency-free.** Edge facts are a `ports`-level type; graph
  algorithms (Tarjan SCC, reachability) are domain logic with no adapter import.
- **Adapters implement ports, never the reverse.** `ports.EdgeStore` defined
  first; bbolt, socket, web, and the eventual MCP adapter each wrap the one app
  service 1:1. MCP is "one more adapter beside socket/web" — cheap *because*
  hexagonal, and only *later*.
- **The one flagged debt** (`SaveAllDimensions` off-interface,
  `bbolt/store.go:461`) is explicitly **not** to be repeated; it is the
  compounding reason site (b) is rejected and the reason the port-first
  precondition is a hard gate.

---

## 4. The out-of-scope guardrail list

What we are deliberately **not** building, each with its binding rationale. This
is the fence; crossing any line below requires reopening the scope-law ADR, not
just a design review.

| Out-of-scope | Why — the binding rationale | Source |
|---|---|---|
| **`query_graph` / `get_node` / `get_neighbors`** (1-hop neighbor lookup as the agent's retrieval) | It is a **slower grep over a stale graph** — an un-indexed full node scan over a build artifact. graphify's own output must *nudge agents off grep* (the tell). Fresh `grep→peek` wins on freshness + precision + per-method `[start-end]` grain. | `serve.py:144`, `:717-718`, `:415-418`; guide §2.3 |
| **Cross-modal / LLM-inferred edges** (`semantically_similar_to`, pdf/image/audio) | Directly conflicts with the **determinism thesis** that enables parity fixtures; LLM `calls` edges needed guards to drop phantoms from shared names (`render`/`parse`). Scope-law layer-3 at best, build only with explicit inferred-provenance if ever. | `research:47,88`; scope law `:50-53` |
| **Heavyweight standing/stale graph as primary retrieval** | The frontier CLI agents (Claude Code, Codex, Aider) **deliberately rejected** indexes/graphs for agentic grep, citing exactly aOa's strengths. Don't chase the bet they lost. | `research:99-103` |
| **Automatic pattern DETECTION** ("this is 73% hexagonal") | 30 years of research caps architecture-level detection at unusable precision (F1 0.09–0.70). aOa does **declare-and-diff** instead — conformance, never detection. | scope law `:48-49`; guide §2.3, §7 |
| **Runtime truth** (traces, flows under load) | vFunction/APM territory; **ingest with OBSERVED provenance only, never derive.** | scope law `:50-51` |
| **Capability maps, cost, incident history, business value** | HUMAN/EXT data plane we don't own; **render-if-provided, never claim derivation.** | scope law `:52` |
| **GoF class-pattern checking, rules DSLs, multi-repo CI governance platforms** | Owning a foreign data plane / requires pretending; out of the core-competence sentence. | scope law `:54` |
| **Full community detection (Leiden) as a Tier-1 build** | Lowest leverage; atlas 134-domain `@domain` enrichment already covers much of the orientation need as classification (a *fair substitute*, **not** topology-based community detection — a different thing). Optional tail, resist until Tier-1 lenses are solid. | `research:49`; guide §2.3 |
| **REAL import-edge parity claimed beyond the three tuned extractors** | Only **Go / Python / JS-TS-TSX** have tuned `parser.go` extractors; everything else is best-effort `extractGeneric`, stamped as the gap. Do **not** claim "509 languages of architect-grade analysis" — 509 is the *registration* count. | `parser.go:235` (3 extractors); `languages_forest.go:5`; guide §4 |
| **MCP fronting a latency-sensitive query** | MCP structurally cannot beat the socket it wraps; it buys **reach, never speed.** Correctly scoped ONLY when it never fronts a hot retrieval path — the single condition under which it is not a distraction. | `socket/server.go:207`; guide §3.2 |

---

## 5. Why we trust this plan — the one-paragraph landing

The strategy was attacked three times at its spine and never moved: the knowledge
graph stays subordinate to fresh `grep→peek` (G3), MCP is a thin late reach-
surface that never fronts a hot query (G3/G0), the scope-law ladder is the cut
rather than a label (every edge is layer-1 REAL or stamped for what it is), and
the whole feature pivots on one G0-gated, G2-clean, G4-clean keystone — emit
import edges inside the always-on parse pass, stored keyed-by-file so freshness
stays free and the write path stays O(edges-for-that-file). What the red-team
*did* repeatedly cut was anchor precision, two seam-level contradictions (the G0
"single pass" and the App.mu locking law), and a phase-scope overreach — and the
final position carries a verified fix for each, with the binding-law boundaries
reconciled back into the source docs. The plan is trustworthy not because no one
attacked it, but because the attacks are recorded here and the surviving claims
each cite a location you can check. Until the single keystone demo exists —
stranger's repo → `aoa init` → component/DSM/cycles render REAL-stamped → edit
one package → only affected shards change — aOa remains *a proven face waiting for
a substrate*, and this record is the standing proof that the substrate, when
built, is built inside every line it must respect.

---

## Appendix — the anchor index for this record (red-team this list first)

| Claim in this record | Anchor |
|---|---|
| Three node-maps, zero relations | `internal/ports/storage.go:59-63`; `Parent` proto-edge `:78` |
| Keystone site (a) = only G0-free choice (always-on index pass, never visits imports) | `parser.go:104,108`→`extractSymbols:235`→`extractGo:347` (3 node kinds, no `import_declaration`) |
| Keystone site (b) = recon-gated dimensions walk, counts+discards (NOT chosen) | `walker.go:54 walkContext.walk` → `:568 countImportSpecs` → `dim_engine.go:200 SaveAllDimensions` |
| Hot-path write cost → store must be keyed-by-file, not flat `[]Edge` | `onFileChanged` two full-map scans: find-ID `watcher.go:65`, alloc-ID `:110`; `ParseFileToMeta` inside the lock `:132` |
| App.mu reconciliation (import edge is index data, not a derived arch write) | `watcher.go:43-44 a.mu.Lock`; locking law `00-OVERVIEW.md:101` |
| Flat socket switch (MCP rides alongside) | `socket/server.go:207` |
| Off-interface `SaveAllDimensions` (the G4 debt, site-(b) compounder) | `bbolt/store.go:461`/`:488`; interface `storage.go:12-56` (neither declared) |
| Language ladder: walker tuning (10) ≠ extraction tuning (3) ≠ registered (509) | `lang_map.go:48-143` (ten keys); `parser.go:235` (3 extractors); `languages_forest.go:5` ("Languages: 509") |
| Blind-judge gate (the moat) | `MODEL-STANDARD.md:43-53` ("### 3. Blind judge") |
| Scope-law ladder + leash text + OUT list | `2026-06-11-core-competence-and-scope-line.md:23-31` (leash "NEVER add a node" `:26`), `:47-54` (OUT) |
| Goals G0–G6 | `.context/GOALS.md:7-13` |
| Stale "28" carried by the guide's own source | `00-OVERVIEW.md:81` ("28, one walker") + README.md:544 + CLAUDE.md |
| graphify wins narrow (3 of 10 tools) | `serve.py:822`, `:862-932`, `:775-792`; anti-pattern `:144`, `:717-718`, `:415-418` |
| Prior research verdict (sharpened, not relitigated) | `2026-06-19-graphify-plus-mcp-research.md` (esp. `:74-115`, `:160-184`) |
| ≤+3% G0 budget; locking law | `00-OVERVIEW.md:99`, `:101` |
