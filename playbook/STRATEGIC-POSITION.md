# aOa — Integrated Strategic Position (DRAFT, round-2 revision)

*Lead strategist synthesis. Branch `playbook`. Date 2026-06-19. Revised to survive a four-lens adversarial review (growth-hacker / founder-VC / feasibility-engineer / leading-edge). Markdown only — no code change. Every load-bearing claim cites a scan (S#), a URL, or an aOa `file:line` / source-doc anchor verified against live source. Vendor/marketing/unverified figures are flagged inline. The falsifiable seams are stated in §F so the next red-team has a real surface to hit.*

> **Round-2 changelog (what the four lenses moved):**
> 1. **CURRENT-STATE BANNER added (§0)** — the load-bearing graph surface (`aoa arch`, overlay loader, arch-shard endpoint, file-save→ETag tick, AG-UI) **does not exist yet**. Every §B/§C demo is written in the *future* tense and gated on the keystone + arch surface. Verified against source. The body no longer contradicts the §F concession.
> 2. **Moats re-split on two axes (§B)** — *felt/shared by users* (Moat A freshness, Moat B the AST-derived diff) lead the **adoption** story; *defensible in diligence* (provenance-by-layer, recon switching-cost, thin-MCP economics) are the **retention/investor** spine. Five co-equal walls → two adoption headliners + three diligence deepeners.
> 3. **Freshness reclassified** from "structural Moat 1" to *structural for the local per-keystroke CLI/socket answer; strong-but-narrowing for the viewer* — Sourcegraph designed SCIP for incremental, and the 2026 watch-mode cluster (codegraph/codemap/codebase-memory) already ships tree-sitter + file-watch + incremental + MCP. File-watch is table-stakes now.
> 4. **Recon de-mooted as a "data network effect"** (technically wrong for single-team data, a16z 2019) → reframed as a **feature-gap + switching-cost / system-of-record** moat.
> 5. **Blast-radius conceded table-stakes** (Potpie ships it today, funded) — Moat moved onto delivery shape (deterministic + fresh + zero-standing-token + provenance), not the query.
> 6. **The interactive agent-canvas conceded a won, commoditizing 2026 pattern** (tldraw Agents-on-Canvas, Excalidraw+MCP, Mermaid-living-contract) — aOa's *only* defensible delta is the AST-derived, `file:line:commit`-grounded BEFORE.
> 7. **Adoption sequencing rebuilt (§D, §E)** — Phase 0 leads with the **already-shipping** grep→peek CLI + judged viewer (zero new code), not the unbuilt diagram. One pre-keystone loop and one real install-loop named, with the single instrumented metric.
> 8. **Host-coupling stated honestly** — the recon/live-status loops need a running daemon + Claude Code session logs; beachhead = Claude Code's CLI-agent base.
> 9. **New free rendition** — aOa can emit a deterministic **Mermaid/plain-text living-contract** of any shard (in-context, PR-diffable) — turns the dominant 2026 best-practice into a free aOa face.

---

## §0. CURRENT-STATE BANNER (read first — the rest of the doc is a roadmap, not a position)

**What ships today (verified against live source):**
- `grep`/`egrep`/`find`/`locate`/`tree` over the O(1) token index; per-method `[start-end]` ranges; `peek` — the sub-ms CLI/socket agent surface. **This is adoptable in one command today.**
- The 16-view ReactFlow/elkjs **architecture viewer** — but as a **build-time Python mockup generator** (`playbook/generators/build_blueprint_viewer.py`, `build_c4_mockup.py`), gated by the **blind-judge** acceptance test (`playbook/standards/MODEL-STANDARD.md`, `view-standards.json`). Not yet a daemon-served live endpoint.
- The localhost **dashboard** (`internal/adapters/web/server.go`) with `withETag`/304 transport (`server.go:156-170`) and the **click-fires-an-action precedent** `POST /api/recon-investigate` (`recon.go:555`) which mutates *annotation* via `SetFileInvestigated` (`recon.go:577`) — **never the substrate** (the leash precedent is real).
- `fsnotify → Reindex` hot-reload (`watcher.go:20`, `app.go:2816`) — the index updates in place on save.
- Atlas 134-domain enrichment; the live recon/session-signal layer feeding learner/autotune/status-line.

**What is NOT built yet (all of §B/§C is gated on these — verified absent in source):**
- **No `aoa arch` command** (no `cmd/aoa/cmd/arch.go`), **no `MethodArchFacts/Reach/Blast` socket methods.** Every Moat demo and the §C loop *invoke* these in the future tense.
- **No import-edge keystone** — the always-on `extractSymbols` pass visits but never *emits/persists* edges (prior research §Q1). This is the foundation everything else rides.
- **No overlay loader** — `grep -rin overlay internal/ cmd/` returns only an unrelated status-line comment. Mode A's "rejects invented ids" loader is **net-new** (small, in-bounds, leash-clean, but not present).
- **No arch-shard web endpoint** — the viewer is the Python build-time generator above, not a `withETag`-gated in-process shard producer.
- **The file-save→ETag tick DOES NOT EXIST.** `bumpRevision()` (`app.go:350`) is called only by `searchObserver` (`app.go:564`), `onSessionEvent` (`app.go:901`), `SetFileInvestigated` (`app.go:2896`), `ClearInvestigated` (`app.go:2905`). It is **NOT** called by `onFileChanged` (`watcher.go:20`) or `Reindex` (`app.go:2816`). **A code edit reindexes symbols but does not bump the ETag** — the live viewer serves a stale 304 after a save until an unrelated search or Claude-session event fires. The "real-time on save" viewer requires **one currently-absent line** (`onFileChanged`/`Reindex` must call `bumpRevision()`). The 304 *transport* ships; the file-change *trigger* does not.
- **No AG-UI** anywhere in the tree.

**Symmetric concession (the compounding-wall cuts both ways, §B.6):** the argument "to neutralize aOa a competitor must do all five" is also **aOa's build obligation** — until the keystone + arch surface land inside the G0 ≤+3% budget, the moats are a **roadmap**, not a position. The §E Phase-0 gate **is** the de-risking milestone a seed investor funds toward.

---

## The position in one paragraph

> Agents now write code faster than any human can keep the architecture coherent. Anthropic **named the failure mode in 2026 — "agentic technical debt" / "architectural drift"** ([Dev|Journal on Anthropic's framing](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/), S7). Crucially, the *same* sources say the remedy is **written constraints — ADRs / CLAUDE.md / specs — that give architectural *direction***, and that *"memory tools solve for data recall but fail to provide architectural direction"* (ibid.). aOa supplies fresh facts (recall) **and** the machine-checkable layer those written constraints need to be enforced: **conformance — declared-pattern-vs-derived-actual diff.** That is the bridge from the named demand to the product. aOa turns any repo into a deterministic, provenance-stamped fact substrate the instant you run `aoa init`, and serves **one substrate, two faces**: the agent (CLI/socket, sub-ms, `file:line:commit` on every answer — adoptable *today*) and the human (architecture views that pass a blind-readability gate, plus an AST-derived before/after diff the team argues over — *post-keystone*). The diagram and the agent's answer are the *same auditable fact*. **The grep→peek CLI drives developer adoption today; the diff renderer is the share-worthy artifact that arrives in Phase 2; governance + evidence packs convert the org** (the money path: CAST charges $10.29K/app/yr verified, with M&A-due-diligence engagements at a much higher services tier, vendor-estimate-grade). The durable walls competitors structurally lack: **provenance** (every answer cited — the grounding receipt the agentic industry is discovering it needs, KPMG/citation-hallucination, S6 §4), the **AST-derived diff** (an un-fakeable BEFORE no LLM-drawn canvas can produce), and a **deterministic, thin-MCP delivery shape** (4 grep-beater tools vs the 42-45 the 2026 cluster ships). Freshness and recon are real edges aOa gets for free — but they are *narrowing* (freshness) or *switching-cost, not data-network* (recon), and the doc no longer sells them as structural walls.

**The spine:** the **moats** (§B) and the **interactive before/after diagram loop** (§C). The landscape (§A) and growth/founder framing (§D) substantiate and sell them. **All of it is gated on §0's Phase-0 keystone.**

---

## §A. The competitive landscape, distilled

Four clusters, one shared bet: *a standing graph/index artifact serves the agent* — the bet the frontier CLI agents declined (Claude Code removed vector search May 2025; "Is Grep All You Need?" 2026 found inline lexical beat dense retrieval, S6 §1). The convergent winning design is **hybrid** (structural query + grep fallback), aOa's deliberate shape. **Inline language caveat:** aOa's structural depth is **3 tuned extractors (Go/Python/JS-TS-TSX)** behind a ladder (10 walker / 509 registered / dynamic forest); the "28 languages" figure in the scope-law ADR (`2026-06-11-...:12`) is stale and must be reconciled to the ladder before any external language claim (§E Phase-1 gate). The body never implies 28 first-class structural extractors.

| Competitor | Cluster / value prop | Traction (flagged) | The structural weakness aOa exploits |
|---|---|---|---|
| **graphify** (safishamsi) | Code-KG over MCP; one graph spanning code+DB+infra+multimodal | ~69.2K stars (real); "YC S26" **self-applied/unverified** (S1) | **Silently stale BUILD ARTIFACT** — [#653](https://github.com/safishamsi/graphify/issues/653) (rebuild refuses to overwrite, MCP serves old graph), [#341](https://github.com/safishamsi/graphify/issues/341) (`update` 114 min, unfinished on 50K-file repo). LLM-inferred edges → AMBIGUOUS tags. *("Fails silently" is fair HERE — see footnote.)* |
| **Potpie** | Enterprise code-KG + agent; **ships blast-radius + system-design TODAY** | $2.2M pre-seed Feb 2026, Emergent Ventures ([FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html), verified) | **Neo4j + Celery/Redis batch ingestion, no incremental path** ([TFN](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/)) — opposite of CLI-first/local/fresh. **Blast-radius is table-stakes, not aOa's edge** (see Moat C). |
| **2026 watch-mode cluster** — codegraph (colbymchenry), codemap (grahambrooks), **codebase-memory-mcp** (DeusData), CodeGraph (codegraph-ai), code-graph-mcp | Local-first, tree-sitter, **file-watch + incremental reindex + MCP** | codegraph ~51K stars; codebase-memory: 158 langs, sub-ms, single binary, XXH3 ~4× incremental ([repo](https://github.com/DeusData/codebase-memory-mcp)) | **The closest cluster — and it proves file-watch is TABLE-STAKES in 2026.** Native OS file events + 2s debounce (codegraph), Merkle/SHA/FNV incremental, **42-45 MCP tools** (codegraph-ai). **No provenance-on-every-answer, no recon signal, no blind-judge gate, no AST-derived diff** — that's aOa's surviving surface, *not* freshness. |
| **Blarify** | Tree-sitter + SCIP + Neo4j | 229 stars | **Highest edge precision, refresh is roadmap** (S1) |
| **Sourcegraph (SCIP/Amp)** | Enterprise code search + precise-nav + MCP | Cody→enterprise-only; Amp spun out Dec 2025 | **SCIP was DESIGNED for incremental-on-push** ([announcing-scip](https://sourcegraph.com/blog/announcing-scip)) — but incremental remains *"on the roadmap"* ([scip-clang](https://sourcegraph.com/blog/announcing-scip-clang)); auto-indexing runs in sandboxed executors per-policy, full reindex 1-2h on large repos. **Their data plane already wants incremental** — the gap is cadence (per-commit cloud vs per-keystroke local), a tunable parameter, not a structural wall. |
| **Glean** | Enterprise *work* graph (100+ sources); adjacent | $7.2B val / $200M ARR (verified, S2) | **~5-min webhook latency, cloud-crawled; no per-method, no code-structural edges, no commit provenance** (S2) |
| **GitHub (Blackbird/Copilot)** | Planetary lexical search + MCP registry | 200M-repo reach | **Best freshness still "within minutes" w/ 1.25PB infra; lexical, not structural.** Real threat = registry-as-gatekeeper, not parity (S2) |
| **CAST** | M&A due-diligence "MRI for software" + MCP (Aug 2025) | **$10.29K/app/yr (verified pricing page, S3)**; DD-engagement services tier *much higher, vendor-estimate-grade* | **Batch reverse-engineered map; per-app shelfware economics; heuristic "hidden links," no confidence labels** (S3). *Genuinely out-covers aOa on languages (150+, mainframe) — its one real advantage.* |
| **Sonar / Sonargraph / NDepend / Lattix** | Architecture conformance / DSM; some now MCP | per-seat→bundled (S3) | **Batch scan artifacts; language-capped (Sonar 5/1, NDepend .NET-only); no commit provenance, no REAL/INFERRED labels** (S3) |
| **Microsoft Architecture Review Agent** | Interactive Excalidraw diagram via MCP + risk/fan-in-out | open-source (S4, S6) | **Parses YAML/Markdown/brain-dumps with an LLM — no source grounding, no freshness, no provenance.** "Built the interaction layer aOa wants and grounded it on sand" (S6 §6) |
| **The agent-canvas SDK cluster** — tldraw "Agents on the Canvas" (Steve Ruiz, [JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw)), Excalidraw+MCP ([mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw)), Mermaid-living-contract / [claude-mermaid] | The interactive agent-on-canvas loop — *"the biggest shift of 2026"* | tldraw SDK + MIT starter kits, widely adopted | **The INTERACTION half is a won, commoditizing pattern — aOa is RIDING it, not inventing it.** But every one grounds on the **LLM's reading of a screenshot + shape data** (tldraw `BlurryShape`/`FocusedShape`) or prose/Mermaid-text. **None grounds on AST with `file:line:commit`.** *The grounded intersection is empty — the ungrounded one is crowded.* |
| **GitDiagram / DeepWiki / Swark / Eraser** | LLM diagram-from-code | viral (GitDiagram) | **All LLM-guessed, all stale** ([arxiv 2512.02170](https://arxiv.org/html/2512.02170v2)) |

**One correction carried forward (S3):** the prior "competitors have **no agent surface**" fact is **stale and false** — CAST (Aug 2025) and Sonargraph (June 2026) ship MCP. **Retire it.** Durable truth: *they bolted MCP onto a stale, per-app-licensed, unstamped batch artifact; aOa's MCP rides a live, provenance-stamped substrate.*

**The recurring weaknesses = the moat surface, NARROWED to what survives 2026 (S1 synthesis + this revision):** (1) ~~staleness fails silently everywhere~~ → **only the build-artifact tier fails silently** (graphify #653, CAST/Potpie/Sonargraph batch); the watch-mode cluster now *detects* drift via fingerprint comparison, so aOa's edge over THEM is *never building a standing artifact to drift against*, not "they fail silently"; (2) edges are untyped or LLM-noisy; (3) **no provenance on every answer** (still the cleanest gap); (4) **no live human+agent recon signal** (no equivalent anywhere); (5) graph-as-primary-retrieval is the bet the frontier declined.

---

## §B. The moats — split by axis (adoption headliners vs diligence spine)

*Two axes, deliberately. The **felt/shared** moats (A, B) are what a developer demos to a teammate and what spreads. The **defensible-in-diligence** moats (C, D, E) are what survives a VC's "why can't an incumbent copy this" — but no developer adopts for them. The body leads adoption with A+B and reserves C/D/E for §D.5/§B.6. Each moat names the **specific competitor inability**, why a feature-copy doesn't close it, the **graph integration**, one **falsifiable demo** (future tense — gated on §0), and a **conceded seam**.*

### §B.0 — What was CUT (so the red-team hits the right surface)

- **Per-method `[start-end]` granularity** — real asset, but codegraph ships typed nodes and SCIP has symbol ranges. Quality edge, not a wall. **Folded into Moat D** (it makes the provenance stamp method-precise, `storage.go`).
- **Project-scoped zero-cost-at-init** — differentiating vs Glean/CAST/Augment, but the watch-mode cluster is also local-first. A **frictionless WEDGE, not a structural moat** (and not, on its own, a growth LOOP — see §D.1). **Folded into the §D wedge.**
- **Freshness-as-standalone-Moat-1** — *demoted*. File-watch + incremental is table-stakes (2026 cluster). Freshness survives as **Moat A** but scoped: structural only for the local per-keystroke CLI answer; near-live for the viewer. It is "the property that keeps the OTHER moats in sync" (§B.6), not a standalone wall.

---

### ADOPTION HEADLINER — MOAT A — Freshness, scoped to where it's structural (the demoable "ours moved, theirs didn't")

**Wedge (what a user FEELS).** Edit a file; aOa's CLI answer is current sub-ms on the next keystroke; the build-artifact competitor still returns the pre-edit graph and **doesn't warn you**.

**Competitor inability — where it actually holds.** Against the **build-artifact tier** it's decisive and demoable: graphify #653/#341; CAST/Potpie/Sonargraph batch; Sourcegraph 1-2h full reindex (incremental "on roadmap," [scip-clang](https://sourcegraph.com/blog/announcing-scip-clang)); GitHub "within minutes." Against the **watch-mode cluster** (codegraph/codemap/codebase-memory) it's *narrower* — they ship file-watch + incremental too; their coverage even recommends "daily full rebuild with fingerprint comparison to catch desync."

**Why structural for the CLI answer, narrowing for the viewer.** The genuine structural floor is the **local per-keystroke CLI/socket answer**: no JSON-RPC round-trip, O(1) token lookup off the live in-memory index — a latency an incumbent's cloud-indexed pipeline *structurally cannot reach*. For the *graph/viewer* face, freshness is a **strong-but-narrowing edge**: an incumbent narrows the cadence gap (minutes→seconds, per-commit→per-push) with a tunable parameter, and the watch-mode cluster already chose local+watch. **aOa's durable distinction over the cluster is not "they fail silently" — it's that aOa never builds a standing artifact to drift against in the first place; the substrate IS the working tree.**

**Integration.** The keystone (REAL import edges riding the always-on `extractSymbols` pass, G0 ≤+3%) means edges inherit the `fsnotify→reindex` path. **REQUIRED, currently-absent change (§0):** `onFileChanged`/`Reindex` must call `bumpRevision()` so the viewer's ETag invalidates on the same file-change tick — **one line, not yet present.** Until it lands, "live viewer pixel current on save" is false.

**Demo (future tense — gated on keystone + the bumpRevision line).** `aoa init` a stranger's repo → REAL-stamped DSM/cycles → edit one package, watch only affected shards change live → same query against graphify shows the pre-edit graph (reproduce #653). *"Ours moved, theirs didn't, and theirs didn't warn you."*

**Conceded seam.** The viewer is content-hashed shards: "live" is recompute + ETag poll, **not streaming**, and **the file-change→revision wiring is absent** (§0). The moat is airtight for **CLI/socket answers**; **near-live, and currently un-wired,** for the viewer. Do not claim "refreshes on every keystroke" for the diagram.

---

### ADOPTION HEADLINER — MOAT B — The AST-derived before/after DIFF (the only natively viral artifact)

**Wedge (what a user SHARES).** aOa derives **both** sides of a before/after from the same substrate — BEFORE = live code-derived current state, AFTER = an agent's proposed plan re-run through the *same* deterministic detectors — and computes the delta from edge-set arithmetic, gated by a blind judge. **This is the one share-worthy decision artifact teammates argue over.** Every competitor fakes "before/after" as two hand-drawn pictures and "quality" as eyeballing.

**Competitor inability — the GROUNDED intersection is empty (S4, sharpened).** The *interaction* loop is a **won, commoditizing 2026 pattern** — tldraw "Agents on the Canvas" ([JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw)), Excalidraw+MCP ([mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw)), Mermaid-as-living-contract ([erdembircan](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development)). aOa does **not** invent the canvas. The single defensible delta: **every shipping canvas grounds on the LLM's reading of a screenshot/shape-data/prose/Mermaid-text** (tldraw's own `BlurryShape`/`FocusedShape` are *simplified* shape summaries for "model understanding"); **aOa's BEFORE is AST-derived and every node is `file:line:commit`.** Every plan surface (Claude Code, Cursor, Copilot, Aider, Amp) is Markdown prose; Copilot Workspace's current-vs-proposed spec was killed (sunset May 2025). Microsoft's agent does the analysis over an **LLM's reading of a brain-dump** (S6 §6).

**Why structural (the diff, not the canvas).** **The diff is un-fakeable only if the BEFORE is *derived*, not drawn.** A competitor without a deterministic substrate can only redraw two pictures; aOa diffs two SHA-snapshot edge-sets and the delta (new cycles, affected closure, new findings) falls out of *set arithmetic*. The **blind-judge gate** (`MODEL-STANDARD.md`, `view-standards.json`) is a falsifiable acceptance test no canvas-SDK ships; an LLM-drawn diagram that admits to hallucinating can't reliably pass it. The **leash rule** (`2026-06-11-...:27-30`: "the diagram cannot say anything the facts don't") makes the rendered view hallucination-immune.

**Integration.** Free to compute off the keystone edges — no doc to maintain, no LLM pass. New data = new edge-sets to diff (declare intended pattern → diff against derived actual → **conformance/drift**, the §D.4 paid tier — *this is the "direction" Anthropic's remedy calls for*).

**Demo (future tense — Phase 2, depends on keystone + overlay loader, §0).** Claude (Plan Mode) proposes a refactor → render its plan as an overlay on the REAL current import graph (AFTER=SIMULATED, BEFORE=REAL) → **click the proposed edge → `aoa arch blast` fires → "this creates a cycle through these 3 modules, blast radius 12 files."** Side-by-side with Microsoft's agent: theirs draws what you described; aOa diffs what your code *is* against what the plan *would make it*.

**Conceded seam (this is the most-contingent thing in the doc).** The diff renderer is a **Phase-2 target, not shipped**; it depends on the keystone (≤+3%) AND the net-new overlay loader (§0). The **strongest adoption artifact arrives in Phase 2, not at launch** — §D/§E sequence growth-spend accordingly and do **not** front-load a launch on it.

---

### DILIGENCE SPINE — MOAT C — Deterministic, thin-MCP delivery shape (the token receipt + the 4-vs-45 tool wall)

*Investor/retention moat. Lead with the token economics, not "deterministic edges are novel" — they aren't anymore.*

**Wedge.** aOa serves the *few* genuinely graph-shaped queries grep can't answer (reachability, blast-radius, cycles/DSM, god-nodes) as deterministic confidence-1.0 tools on a fresh grep/peek spine — the hybrid shape the frontier proved wins.

**What is conceded table-stakes (in-body, matching the seam).** **Blast-radius/reachability as a feature is table-stakes** — Potpie ships it today, funded ([FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html)); graphify and CAST expose it. **Deterministic AST extraction + EXTRACTED-vs-INFERRED tagging is now standard 2026 practice** (graphify itself ships confidence tags; the watch-mode cluster ships typed edges). So "no LLM in the path" is **not** a wall.

**What actually survives diligence — two sub-claims.** (1) **Thin-MCP enforced by the scope law, not willpower** — Tier-1 refuses call/inheritance/LLM-semantic edges and exposes **only 4 grep-beaters**, where the 2026 cluster ships **9-45 tools** (codegraph-ai: **45**; codebase-memory/codemap/SuperLocalMemory in the 9-22 band; graphify: 10). (2) **The token receipt** — Anthropic measures **~40% of context can go to MCP tool metadata**; a CLI-first agent that shells out pays **zero standing tool-definition tokens** (S6 §5). A competitor whose *business is the rich graph* structurally can't shrink to 4 tools without abandoning their value prop.

**Integration.** All four queries ride keystone edges: reachability = walk; blast-radius = git-changed-files ∩ edge closure (never stale); cycles/DSM = Tarjan/matrix; god-nodes = fan-in/out. Sub-ms, stamped, fresh. Tools the agent reaches for when grep can't answer in one shot — never a replacement retrieval index.

**Demo (future tense).** "What breaks if I change `X`?" — `aoa arch blast X` returns the affected closure in <1ms, stamped, fresh. Then the **token receipt**: CLI-first 0 standing tokens vs the cluster's 9-45-tool MCP load.

**Conceded seam.** A competitor *could* add grep fallback (Amp did). The surviving structural part is narrow: **deterministic + fresh + zero-standing-token + scope-law-enforced-thinness**. If a competitor ships deterministic edges + grep fallback + thin MCP, this compresses to Moats A+D. Moat C is **delivery shape, not load-bearing alone** — its durability rides A and D.

---

### DILIGENCE SPINE — MOAT D — Provenance on every answer/pixel, split by scope-law layer (the grounding receipt)

*Investor/trust moat. A developer doesn't adopt for "provenance is correctly scoped by layer" — but it's the spine both faces read, and it's the compliance moat VCs fund.*

**Wedge.** Every answer and every rendered element carries `file:line:commit` — the diagram and the agent's answer are the *same auditable fact*.

**Competitor inability (the least-contested gap).** S1/S2/S3/S4: *not one competitor stamps `file:line:commit` on every answer*, none labels REAL vs INFERRED. **Demand-side proof (S6 §4):** KPMG pulled an agentic report after **5 of 45 citations** checked out ([nerova.ai](https://nerova.ai/troubleshooting-fixes/kpmg-pulls-agentic-ai-report-hallucinations-june-13-2026)); deep-research agents show 11-57% citation-hallucination ([arxiv 2605.06635](https://arxiv.org/html/2605.06635v1)); "Tool Receipts, Not Zero-Knowledge Proofs" ([arxiv 2603.10060](https://arxiv.org/html/2603.10060v1)) names the need as *a verifiable receipt that a claim traces to a real source* — exactly `aoa arch facts`.

**Why structural (couples to determinism).** Provenance is only trustworthy if the thing it stamps is derived deterministically. **LLM-inferred edges (graphify's `semantically_similar_to`, GraphRAG, NL→Cypher, Microsoft's Excalidraw agent) structurally cannot stamp a true source** — there's no single line that produced an inferred edge, only a probability. They can fake a citation; a faked citation is the exact KPMG failure.

**Integration.** `aoa arch facts <unit>` is the audit trail behind any rendered element — one fact rendered two ways, not stored twice. New data (lockfile/SBOM, churn) attaches to an already-provenanced node — breadth without losing the receipt.

**Conceded seam (the honesty that wins, S7 §3).** Split by **scope-law layer** (`2026-06-11-...:24-26`): on **layer-1 REAL edges**, `source` is an **audit/freshness anchor** (re-derivable, commit-recorded) — closer to freshness metadata than a safety mechanism; on **layer-2 MIXED** (agent grouping/naming), the stamp is **the load-bearing leash** pinning inferred buckets to REAL facts. Don't let "provenance makes inference safe" overstate the layer-1 case.

---

### DILIGENCE SPINE — MOAT E — The live recon signal as a SWITCHING-COST / system-of-record moat (NOT a data network effect)

*Feature-gap + embedding moat. Reframed: the a16z 2019 framing the prior draft tripped on says single-team usage data is a **scale** effect, not a **network** effect — it does not compound across teams. Drop the "data network effect" language entirely.*

**Wedge.** aOa observes what humans and agents *actually touch over time* (searches, tool-invocations, file-hits feeding `observe()`/autotune) and folds that lived signal back into the substrate — a behavioral layer **no competitor in any scan has an equivalent of**.

**Competitor inability (cleanest "no equivalent").** S1: *"Every competitor models the static graph; none observes what humans and agents actually touch over time."* vFunction observes *runtime* traces (a different signal, aOa's explicit out-of-scope line).

**Why defensible — the correct framing.** Not a data *network* effect (single-team data doesn't compound for other teams — [a16z, "The Empty Promise of Data Moats," Casado/Lauten 2019](https://a16z.com/the-empty-promise-of-data-moats/)). It defends as **(a) a feature-differentiator no competitor currently has**, and **(b) a switching-cost / workflow-embedding moat** — the value is that aOa is embedded in the team's live grep/agent loop and *is the index the repo already runs through* (system-of-record status). That is the combination the same VC consensus says actually defends: proprietary-data-alone won't, but data **paired with deep workflow embedding** does.

**Integration (highest upside, least-built).** Overlay recon onto the import/DSM graph to rank what matters: a god-node that's also recon-hot is a different finding than one nobody touches; blast-radius weighted by "the modules your team actually edits" beats topology-only. Competitors get *structural* centrality (Microsoft, NDepend fan-in/out); aOa can fuse structural + behavioral.

**Host-coupling (stated honestly).** The recon signal source is `~/.claude/projects/*.jsonl` — the recon + live-status loops require a **running daemon + Claude Code session logs.** This narrows the *immediately*-addressable base to **Claude Code users** — a real bound, framed as a **beachhead** (the fastest-growing CLI-agent base), not hidden.

**Conceded seam.** This layer currently powers learner/autotune/status-line; its connection to the *architecture graph* is **not yet built**. Real as an asset, **under-integrated** — claim the signal exists and uniquely *can* drive graph overlays; do **not** claim the diagram already shows recon-weighted hot paths. **Because it's not yet built into the graph, it cannot drive adoption now — it's a retention/diligence moat, not a wedge.**

---

### §B.6 — Why the set compounds (the binding mechanism — and aOa's own obligation)

The cut sixth candidate — **two-faces-one-substrate** — makes the moats compound:

- **Moat D (provenance) is the spine** — the shared primitive both faces read; without it, two faces are two products.
- **Moat A (freshness) keeps both faces in sync on one tick** — once the absent `bumpRevision()` line lands (§0), agent answer and human diagram invalidate together.
- **Moat B (diff) only works because A+C+D produce a fresh, deterministic, provenanced BEFORE** to diff against.
- **Moat E (recon) is the only one that strengthens the others with use** — re-weighting C's queries and B's findings by lived attention.
- **Moat C (deterministic thin hybrid) is the delivery shape** that gets it to the agent in the form the frontier proved wins.

**Defensibility of the set — stated symmetrically.** To neutralize aOa a competitor must invert their data plane *for the local answer* (A), rip out LLM extraction (C, D), build a blind-judge gate + a substrate clean enough to pass it (B), and accumulate a recon dataset + workflow embedding they can't clone from code (E) — bound by one provenance contract. **But this cuts both ways: aOa must ALSO do all five, and has shipped zero of the load-bearing graph work (§0).** Until the keystone lands inside G0 ≤+3%, the compounding wall is a **fundable roadmap**, not a position — the §E Phase-0 gate is the milestone a seed investor funds toward.

---

## §C. The interactive real-time diagram + Claude before/after loop (the second spine)

**Current-state banner (repeat for this section):** *No `aoa arch` command, socket method, arch-shard endpoint, overlay loader, file-save→ETag tick, or AG-UI exists yet (§0). Everything below is the intended build, gated on Phase-0.*

**Thesis (re-framed, leading-edge lens).** The interactive agent-canvas is a **won, commoditizing 2026 pattern** — tldraw "Agents on the Canvas," Excalidraw+MCP, Mermaid-living-contract are the occupiers; industry coverage calls "the agent that lives with your diagram" the biggest shift of 2026. **aOa does not invent the loop.** aOa's **one defensible delta** is grounding: every shipping canvas grounds on the LLM's reading of prose/screenshot/shape-data/Mermaid-text; **aOa's BEFORE is AST-derived and every node is `file:line:commit`.** The interaction tech is mature and borrowable (S5: "a few hours of React"); the defensible position "is not the click — it's that clicking a node lands on a fresh, provenance-stamped fact" (S5 §0.4). The **grounded** intersection is empty; the ungrounded one is crowded.

### C.1 — REAL-TIME — what ships vs what's net-new (corrected)

**Ships today:** the `withETag`/304 *transport* (`server.go:156-170`) — returns 304 when `If-None-Match` matches `revisionFn()`, else 200 + new ETag. **Does NOT ship:** the file-change→revision wiring — `bumpRevision()` is **not** called by `onFileChanged`/`Reindex` (§0), so a save does not invalidate the viewer's ETag. **Required net-new (small, in-bounds):** (a) call `bumpRevision()` on reindex; (b) an in-process **arch-shard producer** served through the existing `withETag` middleware; (c) per-scope ETag so `pkg/bar` doesn't invalidate when `pkg/foo` churns. The viewer is today a **build-time Python mockup generator** (`playbook/generators/`), *not* a daemon-served live endpoint — that wiring is net-new but cheap. *(The "recompute-on-compact" cadence named in the prior draft describes a mechanism that doesn't exist — dropped; treat shard-recompute cadence as a design proposal.)*

### C.2 — BEFORE/AFTER (edge-sets, not prose) — three modes, ship in order

| Mode | What | Provenance | Ships when |
|---|---|---|---|
| **A — proposed-edge overlay** (MVP, leash-native) | Claude emits a JSON patch of edges, every endpoint an **id already in the facts**; the (**net-new**) overlay loader rejects invented ids; renderer re-runs the same deterministic detectors over the hypothetical set | `SIMULATED · proposed` | **First** — requires the **net-new overlay loader** (pure graph algebra, no network, leash-clean) + the detectors; G0 holds |
| **B — branch/worktree re-derive** (high fidelity) | Plan written on a branch; aOa re-derives REAL edges; BEFORE=`HEAD`, AFTER=branch, same extractor | `REAL · derived @ branch-sha` | After git-worktree wiring; the un-fakeable upgrade |
| **C — autonomous worktree** (later) | Claude creates worktree, applies plan, aOa re-derives — automated Mode B | REAL | Only after the leash boundary is battle-tested — the **gimmick frontier the user warned against** |

**Recommendation (Advisory Rule):** ship **Mode A first** (leash-native, no git plumbing); Mode B is the credibility upgrade; **do not lead with C.** Note Mode A's "overlay loader" is **net-new, not "reused"** (§0).

**What the diff computes:** new/removed cycles, blast radius of the delta, god-node shift, new findings — Microsoft's feature list **computed over derived facts, not an LLM's reading of a brain-dump.**

### C.3 — INTERACTIVE (click fires Claude) — built on an existing precedent

aOa **already has a click-fires-an-action endpoint:** `POST /api/recon-investigate` (`recon.go:555`) takes `{file, action}` from a UI click and mutates **annotation** (`SetFileInvestigated`, `recon.go:577`) — **never the substrate** (verified). Track B adds sibling POSTs in the same shape. The loop (future tense — the `MethodArch*` methods are net-new, §0):

```
ReactFlow onNodeClick(node)
  → POST /api/arch/suggest {subject, kind, scope}
      → socket method (MethodArchFacts/Reach/Blast — NET-NEW)
      → returns GROUNDED context: the cycle's edges, each file:line:commit
  → grounded fact-pack → Claude (CLI subprocess / MCP tool result)
      → Claude returns a SUGGESTION (proposed-edge overlay, Mode A)
  → diff renderer computes BEFORE/AFTER; viewer re-renders AFTER, stamped SIMULATED
```

**The difference from every competitor:** the click hands Claude **the cycle's actual edges with `file:line:commit`**, not "a node labeled `AuthService`."

**Surfaces (ride the standards — corrected current-state):** (1) **localhost dashboard** (`server.go`) — **ships today**; **AG-UI streaming (`STATE_DELTA`/`TOOL_CALL_START`) is a NET-NEW adapter, NOT present** (§0). AG-UI/A2UI/MCP-Apps are the correct, adopted 2026 standards to target (AWS Bedrock AgentCore Mar 2026; SEP-1865 ratified 2026-01-26 under the Linux Foundation, S5) — aOa's viewer **does not yet emit AG-UI events; this is the intended integration**, gated by the unverified rendering-fidelity risk (§F.6). (2) **MCP App inside Claude** (`ui://` ReactFlow viewer) — lower-regret long bet, per-host rendering fidelity *unverified* (S5 §5). Use A2UI's "trusted catalog" framing so the agent picks from aOa's **judged** view types — preserving the blind-judge gate.

### C.4 — The leash holds (the part the red-team pushes hardest)

The agent **never writes a node/edge into the REAL substrate** — it writes a **proposed-edge overlay file**; the (net-new) overlay loader rejects any id not in the facts; applying an overlay drops the view to MIXED (or AFTER to SIMULATED) — never REAL (`2026-06-11-...:27-30`). **No LLM call happens inside the service** — Claude runs *outside* (CLI/MCP host), produces a file, the deterministic renderer consumes it; **G0 (no network on any derive path) is intact.** Same guarantee `recon-investigate` already honors. **Falsifiable leash test:** if any Track-B path lets agent output appear in a REAL-stamped view, the design is broken — by construction it cannot. *(This is architecturally correct and verified against the recon precedent; the leash is the one part of §C with a live analog.)*

### C.5 — Leading-edge, not gimmick

*Why leading-edge:* the demand is named and dated — Anthropic named "agentic technical debt," and its **named remedy is written constraints (ADRs/CLAUDE.md/specs) for architectural *direction*** ([Dev|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/), S7); the before/after-fires-agents canvas is the interactive face of the **conformance** layer those constraints need. *The gimmick risk:* "click → Claude autonomously rewrites your code" (Mode C overreach). aOa stays on the **SUGGEST** side: the agent proposes, the human disposes. The shareable artifact is the **decision** (BEFORE vs AST-derived AFTER) — viral loop and leash-safe boundary at once. **The defensible novelty is the grounding, not the canvas — §C never claims aOa invented the loop.**

### C.6 — The living-contract rendition (turn a competitor best-practice into a free aOa face)

The dominant 2026 best-practice for *agent-diffable* architecture is the **living-contract**: plain-text (Mermaid) checked into the repo, diffable in a PR, readable inside the agent's context window without rendering ([erdembircan](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development), [Docsie 2026](https://www.docsie.io/blog/articles/technical-diagrams-docs-as-code-2026/)). aOa's content-hashed shard + ReactFlow viewer is richer and gated, but is **not in the agent's context window and not natively PR-diffable** — the very properties the cluster prizes. **Position:** aOa can emit a **deterministic Mermaid/plain-text rendition of any shard** — near-free given it already emits shard JSON — adding two properties the gated viewer lacks: in-context consumption and PR-diff. This **strengthens the "renditions of one substrate" thesis** (a third face, derived not drawn) and means aOa *covers* the living-contract pattern rather than being blindsided by it. Marked as a **proposed rendition**, not shipped.

---

## §D. Growth-hacker + founder positioning

**Sequencing law (top of §D, per growth lens):** the share-worthy loops light up only **after Phase 0**. The doc leads adoption with **what ships today**, not the unbuilt diagram.

### D.0 — The pre-keystone adoption loop (zero new code — ships TODAY)

The grep→peek CLI, atlas enrichment, and the judged viewer **already exist.** The Phase-0 wedge is the **sub-ms `grep → peek` CLI an agent adopts in one command** (`aoa init`, then the agent's CLAUDE.md routes code search through `aoa grep` → `aoa peek`). This is adoptable **next Tuesday**, needs zero new code, and is the *only* loop that doesn't wait on the keystone. One **pre-keystone shareable artifact** that needs no new code: a **provenance-stamped DSM/cycles view of a popular OSS repo, rendered by the existing build-time generator**, that a maintainer would re-share — *prove it's share-worthy on its own*, independent of the unbuilt diff. (Caveat: the build-time generator produces the view today; the *live* daemon-served version is post-keystone.)

### D.1 — The wedge vs the loop (downgraded honestly)

**`aoa init` is a frictionless WEDGE — lower-friction than every competitor — but not, by itself, a LOOP.** A wedge proves easier-to-start; a loop describes how user N's init recruits user N+1. The wedge is real: graphify needs a build script (114 min/50K files, #341); Potpie needs Neo4j+PostgreSQL+Celery+Redis; CAST/Sonargraph need per-app license + server; Glean/Sourcegraph need cloud crawl. **aOa: `aoa init`, then a one-command npm binary install + a daemon.** *Honest friction (growth lens):* the real binary is the tree-sitter build with 509 compiled grammars + a CGo daemon; "zero infra" means installing a binary and running a daemon that tails Claude session logs — the recon/live-status value materializes only if the daemon runs persistently and the user is on **Claude Code** (the recon source is `~/.claude` logs). **Beachhead, not unbounded TAM.**

**The actual LOOP (designed, not asserted).** The interactive before/after canvas **fires Claude locally** — a teammate who receives a plan-diff link must run `aoa init` to click-to-investigate (the suggestion is computed against *their* local fresh substrate; a non-user can view a static PNG but cannot interact). That recipient-must-init mechanic is the install loop the static-PNG artifacts lack. **The single instrumented metric that proves the loop closes:** `init → second-session → teammate-init` (not merely `init → second-session`). *This loop is Phase-2 (depends on the canvas/diff).* Until then, growth is wedge-driven (D.0), not loop-driven.

### D.2 — Shareable artifacts, each marked with its earliest-ship gate

| Artifact | Viral mechanic | Earliest ship | Strength |
|---|---|---|---|
| **Provenance-stamped DSM/cycles of an OSS repo** (existing generator) | impression / maintainer re-share | **Phase 0 (today)** | Weak-medium — needs its own share-worthiness proof (D.0) |
| **README GIF** (`aoa init` → REAL-stamped view passing the judge) | star spike | **Phase 1** (post-keystone for a *live* REAL stamp) | **Weak** — drives a vanity star spike, not retained adoption; do NOT front-load a launch on it |
| **Before/after plan diff** (the decision artifact teammates argue over) | install loop (recipient must init) | **Phase 2** | **Strongest — but most contingent (Moat B seam)** |
| **Evidence pack** ("what changed since last review," SHA-stamped) | shares *upward* into the buying center | **Phase 3 (paid)** | High value, latest |

*Drop the OpenClaw star-velocity comp:* a discredited metric (stars, which the doc itself calls vanity) can't prove a loop works. The loop proof is the **recipient-must-init mechanic** (D.1), not a star-spike analogy. *Tactics:* README = one-line category + GIF (not a badge wall); launch as a sequence (community → SEO per-language landing pages); track the **init→second-session→teammate-init** funnel, not raw stars.

### D.3 — The founder why-now (assembled, with the demand→product arrow earned)

Four de-risking facts: (1) **the interface bet is won** — MCP under the Linux Foundation AAIF, >10,000 servers (S6 §5); (2) **agents need fresh structural context and the build-artifact tier is structurally stale** (the §A table); (3) **agentic technical debt at machine speed is the named demand** ([Dev|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/)); (4) **"Architecture as Code"** (Ford & Richards, O'Reilly 2026) is the conceptual seal.

**The demand→product arrow (earned, per founder lens):** Anthropic's named remedy is **written constraints (ADRs/CLAUDE.md/specs) for *direction*; memory tools give *recall*, which already works.** Fresh facts = recall (necessary-not-sufficient). **The product the named demand pulls toward is CONFORMANCE — aOa's declared-pattern-vs-derived-actual diff (§D.4 paid tier) — the automated enforcement those written constraints need.** Anthropic's own "skeptical memory" design even tells the agent to **grep to verify current reality** before critical changes — which is exactly aOa-as-fresh-grep-substrate. Lead the why-now with **conformance/drift-detection as the product**; demote "truth source" to a supporting claim.

> **Founder pitch:** *Anthropic named the failure (agentic tech debt) and named the remedy (written constraints for direction). Constraints written in prose can't be machine-enforced. aOa is the automated enforcement layer: it derives the actual architecture deterministically and diffs it against the declared pattern — conformance, fresh, provenance-stamped. Competitors have pieces — graphify has a stale, LLM-noisy graph; CAST has batch governance at enterprise prices; Potpie ($2.2M, validated thesis) has blast-radius on enterprise-Neo4j; the canvas SDKs have the interaction grounded on prose. aOa is the first to make the constraint machine-checkable against a fresh, provenance-stamped substrate.*

### D.4 — Moat-to-money: free structural views → paid governance

Governance is the conversion lever (HashiCorp/GitLab gate team-scale RBAC/audit/policy/SSO; PLG adds sales at $10M-$50M ARR, S7). The free/paid line maps onto the scope law:

| | **FREE at `aoa init`** | **PAID: governance & evidence** |
|---|---|---|
| Substrate + 16 views + shareable diagram + **Mermaid living-contract rendition** | ✓ | multi-repo/org rollups, estate landscape |
| Agent: CLI/socket/thin-MCP (grep→peek + 4 grep-beaters) | ✓ | — |
| Conformance (declared pattern vs derived actual) | — | ✓ baseline/freeze — *the "direction" layer (D.3)* |
| Drift / before-after diff | the diff renderer as a decision artifact | "what changed since last review" packs, SHA-stamped |
| Audit | `aoa arch facts` (grounding receipt) | audit-ready export (CycloneDX/SPDX SBOM, in-toto/SLSA-shaped) |

**Money anchor (corrected per founder lens).** Anchor the defensible math on the **verified $10.29K/app/yr** CAST product price (G2/pricing page, S3) — enough to support a "self-service, faster, cheaper" wedge. The **$200K-$800K M&A-due-diligence figure is a SERVICES-engagement number, vendor/estimate-grade (S3)** — keep it only as an *illustrative ceiling*, clearly flagged, **not** the headline willingness-to-pay. **Tailwind:** compliance is shifting point-in-time → continuous ("auditors expect near-real-time evidence," S7 §4) — a continuously-fresh, provenance-stamped substrate *is* a near-real-time evidence engine. **Pricing (S7, directional, agency-blog-sourced):** gate by usage metric (repos/LOC), not by crippling the substrate; per-seat in decline (IDC: 70% off per-seat by 2028 — *directional*); 3-5% freemium conversion on a viral base is healthy — *directional*.

### D.5 — The fundable shape (priority-ordered, S7 §3)

The 2026 VC playbook: *"Founders who answer with compliance moats are closing rounds in weeks"*; "hallucinations aren't a real issue" founders die in diligence. Lead with: (1) **verifiability/compliance moat** (determinism + provenance — *in-toto/auditability property, NOT SLSA reproducible-builds*, S6 §3) → **Moat D**; (2) **the AST-derived conformance/diff** → **Moat B**; (3) **thin-MCP economics** → **Moat C**. **Reclassify freshness (A) and recon (E) as defensible-but-not-structural** — A is narrowing (Sourcegraph designed SCIP for incremental; the watch-mode cluster ships file-watch), E is a switching-cost/feature moat, **not a data network effect** ([a16z 2019](https://a16z.com/the-empty-promise-of-data-moats/)). Category: **"agent infrastructure, not a wrapper,"** no per-query inference cost (vs Cursor's burn).

**Potpie reframed (per founder lens).** Not "Potpie sharpens the same thesis" (that signals me-too). **Potpie ($2.2M pre-seed, verified) validates the budget exists AND proves the thesis is already contested at seed — aOa's wedge is the freshness/provenance/local-economics that Potpie's enterprise-Neo4j/Celery-batch shape structurally cannot match.** Blast-radius as a feature is table-stakes (Potpie ships it); aOa's differentiation is **delivery shape**, not the query. **Market honesty:** no clean standalone "architecture governance" TAM — lead with the *tailwind* (GRC/compliance proxies, $17.2B compliance software ~16% CAGR; SOC2 automation $850M→$2.7B — *directional*) + the verified CAST/Potpie willingness-to-pay anchors, not a fabricated TAM.

---

## §E. The 90-day play (sequenced, falsifiable)

| Phase | Growth (what's shippable) | Founder | Gate |
|---|---|---|---|
| **0 — keystone + ship-what-exists** | **Lead with the grep→peek CLI (zero new code) + a provenance-stamped OSS DSM/cycles view from the existing generator (D.0).** Instrument `init→second-session`. | Pre-seed deck: compliance/verifiability (D) → conformance/diff (B) → thin-MCP economics (C); freshness/recon reclassified non-structural; Potpie = budget-validated-and-contested | Import edges on the always-on parse pass, **G0 ≤+3%**, DSM/cycles GREEN on a live stranger repo. **Reconcile the stale "28-language" ADR figure to the ladder (3 tuned / 10 walker / 509 registered / forest) before any external language claim.** **Add the absent `bumpRevision()` call on reindex (§0).** |
| **1 — the live face** | README = one-line category + GIF (live REAL-stamped view). *Expect a star spike (vanity); do NOT mistake it for retained adoption.* | Land "conformance is the enforcement layer for Anthropic's named remedy" | Net-new: arch-shard producer served through `withETag`; `aoa arch` command + `MethodArch*` socket methods |
| **2 — the loop (the real share artifact)** | **Ship before/after diff (Mode A) — the recipient-must-init loop (D.1). Track `init→second-session→teammate-init`.** MCP App inside Claude (node-click → `aoa arch facts`) | The agentic-tech-debt why-now | Net-new **overlay loader** (rejects invented ids); thin MCP = 4 grep-beaters only, registry-ready/signable |
| **3 — the money** | Evidence pack as the upward-traveling artifact | Open paid governance/conformance tier; target CAST's compliance budget self-service (anchor $10.29K/app, not the unverified DD number) | CycloneDX/SPDX SBOM export; conformance = declared-vs-derived (layer-3) |

---

## §F. Red-team targets (swing here first)

1. **The whole wedge is keystone-gated (§0).** Every freshness/diff/PLG-loop claim is roadmap until import edges ship within G0 budget on the 3 tuned extractors (*not* "400+"). *Defense:* the grep→peek CLI + judged viewer ship **today** (D.0) and carry Phase-0 adoption with zero new code; the moats are an honestly-staged build.
2. **Provenance (Moat D) is softening.** codegraph (typed edges) and the bitemporal/SHA-incremental long tail creep toward it; none stamps `file:line:commit` on every *answer* yet — *attack the durability, not the existence.*
3. **Freshness (Moat A) is the fastest-commoditizing axis.** Sourcegraph designed SCIP for incremental; the watch-mode cluster ships file-watch + incremental + drift-fingerprinting. aOa's structural floor is the **local per-keystroke CLI answer**, not the viewer. *Attack: "an incumbent narrows the cadence gap in a quarter" — true for the viewer; false for the local latency floor.*
4. **Is aOa an agent tool with a governance upsell, or a governance tool with an agent funnel?** The honest answer may be the latter (the conformance/diff is the fundable core per D.3) — the deck must not pretend otherwise.
5. **Blast-radius is table-stakes** (Potpie ships it, funded). Moat C survives only on *delivery shape* (deterministic + fresh + zero-token + thin), not the query. If a competitor ships deterministic edges + grep fallback + 4-tool MCP, C compresses to A+D.
6. **The interactive canvas is a crowded, won pattern** (tldraw/Excalidraw/Mermaid). aOa's only delta is AST-grounding. *Attack: "the grounding edge is thin if a competitor wires their canvas to a real index."* Defense: they'd still lack the deterministic provenance-stamped BEFORE and the blind-judge gate.
7. **Distribution + host-coupling asymmetry.** GitHub/Glean/Sourcegraph can bolt freshness onto their pipelines faster than aOa builds distribution; the recon/live loops are bound to Claude Code session logs (beachhead, not unbounded). "Ride the registry" is partnership-dependent.
8. **Real-time over a big estate could thrash.** Bounded by ETag 304-empty-body + per-scope invalidation (net-new); whole-estate interactive views deferred (S5 §5). Unproven at scale.
9. **MCP App rendering fidelity unverified** — whether a full ReactFlow/elkjs bundle renders cleanly inside Claude's iframe at hundreds of nodes is untested (S5 §5).
10. **Mode C is the shiny-object trap** — "autonomous worktree rewrite" breaks the leash if rushed; keep it gated behind a battle-tested boundary.

---

### Honesty flags carried forward
graphify "YC-S26" unverified/self-applied; Potpie $1.1M revenue (Latka, directional); all conversion %s, ARR, and IDC/CAGR figures agency-blog-sourced (directional); the **a16z data-moat essay is 2019** (Casado/Lauten — the framing is evergreen VC consensus, not a 2026 publication); the **$200K-$800K CAST DD figure is a services-engagement estimate, vendor-grade — NOT a product price** (anchor on the verified $10.29K/app/yr); no standalone architecture-governance TAM; **the keystone, `aoa arch` surface, overlay loader, arch-shard endpoint, file-save→ETag tick, and AG-UI are NOT yet built (§0)**; "deterministic" ≠ SLSA-reproducible (in-toto/auditability only); the O'Reilly "feedback framework" → conformance mapping is *interpretation*; star counts drift daily; CAST genuinely out-covers aOa on language breadth (its one real advantage); the recon/live loops require a running daemon + Claude Code session logs (Claude Code beachhead).

### Footnote — "fails silently" is scoped, not universal (leading-edge lens)
The "silently stale, fails without warning" indictment is **fair against the build-artifact tier** (graphify #653, CAST/Potpie/Sonargraph batch) but **overstated against the 2026 watch-mode cluster** (codegraph/codemap/codebase-memory), whose coverage explicitly recommends "daily full rebuild with graph-fingerprint comparison to catch desynchronization." Against THEM the sharper, defensible distinction is: **aOa never builds a standing artifact to drift against in the first place — the substrate is the working tree.**

### Grounding
**Scans S1-S7** (this brief) + **2026 web verification** (this revision): [Sourcegraph SCIP](https://sourcegraph.com/blog/announcing-scip) / [scip-clang](https://sourcegraph.com/blog/announcing-scip-clang) (incremental "on roadmap"); [a16z "Empty Promise of Data Moats" 2019](https://a16z.com/the-empty-promise-of-data-moats/); [tldraw Agents on the Canvas, JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw) + [tldraw agent-template](https://github.com/tldraw/agent-template) + [mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw); [Potpie $2.2M, FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html) + [TFN](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/); 2026 watch-mode cluster ([codegraph](https://github.com/colbymchenry/codegraph), [codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp), [codemap](https://github.com/grahambrooks/codemap), [CodeGraph 45 tools](https://github.com/codegraph-ai/CodeGraph)); [Mermaid living-contract](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development) + [Docsie 2026](https://www.docsie.io/blog/articles/technical-diagrams-docs-as-code-2026/); [Anthropic agentic-tech-debt remedy = ADRs/CLAUDE.md/specs](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/). **aOa anchors (verified against live source):** `internal/adapters/web/server.go:34,49-52,156-170` (ETag/revision transport — note `revisionFn` exists, file-change wiring does NOT); `internal/app/app.go:350` (`bumpRevision`), `:564/:901/:2896/:2905` (its ONLY callers — NOT `onFileChanged`/`Reindex`); `internal/app/watcher.go:20` (`onFileChanged`, no bump); `internal/app/app.go:2816` (`Reindex`, no bump); `internal/adapters/web/recon.go:555,577` (click-fires-annotation precedent — never substrate); `cmd/aoa/cmd/` (NO `arch.go`); `internal/` (NO overlay loader, NO `MethodArch*`, NO AG-UI); `.context/decisions/2026-06-11-core-competence-and-scope-line.md:12` (stale "28 languages"), `:24-30` (three-layer ladder + leash); `playbook/generators/build_blueprint_viewer.py` (build-time mockup generator, NOT a live endpoint); `playbook/standards/MODEL-STANDARD.md` + `view-standards.json` (blind-judge gate); `.context/details/2026-06-19-graphify-plus-mcp-research.md` (prior verdict this extends).
