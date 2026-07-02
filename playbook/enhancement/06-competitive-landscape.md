# 06 — The Competitive Landscape

*Enhancement-pool companion to 01–05. Branch `playbook`. Date 2026-06-19. Markdown only — no code change.*

> **What this doc is.** One navigable map of the whole field aOa competes in, distilled from seven landscape scans. For every notable player: **what it is**, its **value prop**, **traction** (cited; vendor/marketing figures flagged), and the **structural weakness aOa exploits** — the thing it cannot fix without rebuilding. It ends with the **white-space map**: the four-way intersection nobody occupies.
>
> **Read this against the position, not instead of it.** The strategic argument lives in `playbook/STRATEGIC-POSITION.md`; the assets and keystone live in `playbook/ENHANCEMENT-GUIDE.md` and `playbook/enhancement/01-knowledge-graph-and-visualization.md`. This doc is the *terrain*, not the *plan*.
>
> **Honesty banner (inherited from the position's §0).** aOa's load-bearing graph surface — `aoa arch`, the overlay loader, the arch-shard web endpoint, the file-save→ETag tick, an MCP/AG-UI face — **is not built yet** (verified absent in live source). What ships *today* is the grep/egrep/find/locate/tree CLI over the O(1) token index, per-method `[start-end]` ranges + peek, `fsnotify → Reindex` hot-reload (`internal/app/watcher.go:20`, `internal/app/app.go:2816`), atlas enrichment, the recon/session-signal layer, the localhost dashboard with `withETag`/304 transport (`internal/adapters/web/server.go:156`), and the blind-judge-gated viewer as a **build-time generator** (`playbook/generators/build_blueprint_viewer.py`). Every "aOa wins on X" below is therefore split into **ships-today** vs **roadmap-gated-on-the-keystone**. The moats are real *and* they are a build obligation.

---

## How to read the map — six clusters, one seam

The field divides into six clusters. They look like different markets but they share **one structural ceiling**: every one of them serves the agent (or the architect) a **build artifact** — a graph, an index, a scan, or an LLM-drawn diagram that is only as fresh as its last build, and that carries no `file:line:commit` audit trail on the answer it returns.

| Cluster | Who | The bet they make | The seam aOa sits in |
|---|---|---|---|
| **1. Code-KG OSS** | graphify, Potpie, codegraph, CGC, Blarify, FalkorDB code-graph… | A standing graph artifact is the agent's primary retrieval substrate | Staleness is silent; edges are LLM-noisy or freshness-traded; no provenance, no recon |
| **2. Code-intel incumbents** | Sourcegraph, Glean, GitHub (Blackbird/Copilot), Augment | A hosted, org-scoped index/graph over MCP, per-seat + usage billing | Freshness bottoms out at *minutes*; cloud infra; no per-answer commit stamp; not project-local |
| **3. Arch/governance $ market** | CAST, vFunction, Sonar, Sonargraph, NDepend, Lattix/Structure101, Moderne | Reverse-engineer a map; sell it to DD/ARB/compliance at $10K–$800K/app | Batch, not live; no provenance honesty (REAL vs INFERRED unlabeled); per-app shelfware economics |
| **4. AI agents + diagram tools** | Claude Code, Cursor, Copilot, Devin, GitDiagram, DeepWiki, Swimm… | Plan-before-execute (prose) + diagram-from-code (LLM-guessed) | Plan is never a diagram; diagram is never code-derived, never fresh, never provenanced |
| **5. Interactive-canvas tech** | tldraw, MCP Apps, AG-UI, A2UI, ReactFlow, Excalidraw/Figma/Miro | The canvas as an agent control surface | The interaction loop is *won and commoditizing* — the only delta is AST-grounding underneath it |
| **6. Funding/market picture** | Potpie ($2.2M), Glean ($7.2B), Moderne ($30M), Cursor ($29.3B)… | Agent-infra + governance/compliance moat is what 2026 capital pays a premium for | The fundable wedge is determinism + provenance (verifiability moat) — structurally aOa's |

**The one seam, stated once:** *fresh-by-construction + provenance-on-every-answer + agent-queryable + visually-gated + diff-able.* No player in any cluster holds more than two of those five. The white-space map (§7) is what falls out when you intersect all five.

---

## Cluster 1 — Code-Knowledge-Graph / MCP OSS (the direct lane)

*Star counts are live-fetched this session and drift daily. This is **THIS** graphify — `safishamsi/graphify`, tree-sitter code knowledge graph over MCP — not the unrelated design/data-viz tools of the same name.*

Every project here makes the **same contested bet**: a standing graph artifact is the agent's primary retrieval substrate. They differ only in graph store (Neo4j / FalkorDB / Kuzu / SQLite / in-memory JSON), edge source (tree-sitter AST vs LSP/SCIP vs LLM inference), and whether they ship file-watching. **None** combines provenance, a live human+agent recon layer, per-method byte ranges, and a blind-judge-gated visual face.

### graphify (`safishamsi/graphify`) — the headline competitor
- **What it is.** An "AI coding assistant skill" that turns a folder of code/docs/SQL/scripts/images/video into a queryable knowledge graph, exposed to agents via MCP.
- **Value prop.** One graph spanning app code + DB schema + infra + multimodal docs; query it instead of grepping. Confidence tags (EXTRACTED/INFERRED/AMBIGUOUS), Leiden community detection, MCP (stdio + Streamable HTTP).
- **Traction.** **~69.2K stars, 7K forks, MIT**, branch `v8`, `v0.8.42` (2026-06-18) ([repo](https://github.com/safishamsi/graphify)). **Displays a "YC S26" badge** — but the batch is unannounced and the founder bio omits YC; **treat the stars as real, the YC label as unverified marketing** (per prior research). The most feature-complete project in the cluster: 36 grammars, multimodal ingestion, production-grade MCP transport.
- **Structural weakness aOa exploits — silent staleness, now documented in graphify's *own* issue tracker** (the strongest external confirmation in the whole landscape):
  - [#653](https://github.com/safishamsi/graphify/issues/653) — post-commit rebuild's safety check refuses to overwrite a shrunken graph, so `query`/MCP **keep serving the old graph with no warning** (~1,400 semantic nodes silently dropped).
  - [#857](https://github.com/safishamsi/graphify/issues/857) — a shared `manifest.json` makes `extract` skip semantically-stale files ("already processed").
  - [#341](https://github.com/safishamsi/graphify/issues/341) — `update` ran **114 min without completing** on a 50K-file monorepo (96-core Xeon).
  - The graph is a periodically-rebuilt snapshot whose freshness depends on git hooks that demonstrably drift and fail silently; its LLM-inferred edges are the noisy layer the AMBIGUOUS tag exists to manage. **aOa rides the live fsnotify index (fresh by construction) and emits only deterministic AST edges (confidence-1.0, no LLM in the path).**

### Potpie (`potpie-ai/potpie`) — the funded enterprise play
- **What it is.** Open-source platform giving AI agents context in large codebases; now "spec-driven development."
- **Value prop.** Fuse fragmented engineering context (code + Jira + Sentry + Notion + reviews) into one Neo4j graph; layer Debug / Q&A / Code-gen agents on top. **It ships blast-radius** — conceded table-stakes, not a moat.
- **Traction.** **~5.5K stars, Apache-2.0** ([repo](https://github.com/potpie-ai/potpie)); **$2.2M pre-seed (Feb 2026, led by Emergent Ventures)** ([TechFundingNews](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/), [Sovereign Magazine](https://www.sovereignmagazine.com/startups/potpie-ai-raises-2-2-million-to-give-ai-agents-codebase-context/)). *Vendor/Latka, unverifiable:* $1.1M revenue mid-2025; a 40M-LOC customer cut RCA "week → 30 min."
- **Structural weakness aOa exploits.** **Heavyweight, build-time, server-bound.** Neo4j is mandatory; parsing is async batch via Celery; **no documented incremental/real-time path**. Its enterprise breadth (logs/tickets) is also its determinism problem — LLM-fused multi-source edges are inherently un-provenanced. It is the *opposite* of CLI-first / sub-ms / local.

### colbymchenry/codegraph — the freshness-aware outlier (closest on features)
- **What it is.** Local-first code-intel library + CLI + MCP server, pre-indexed and auto-syncing.
- **Value prop.** "Fewer tokens, fewer tool calls, 100% local"; "index never stale." TypeScript, **SQLite/FTS5** (not a graph DB), tree-sitter, 20+ languages, 22 NodeKinds / 12 EdgeKinds, auto-configures 8 agents.
- **Traction.** **~51.3K stars, MIT** ([repo](https://github.com/colbymchenry/codegraph)) — high count; surge recency unverified, likely a 2026 viral spike (treat as real but volatile).
- **Structural weakness aOa exploits.** **No provenance, no live human/agent recon signal, no per-method ranges, no visual face.** Edges are typed but un-stamped (no `file:line:commit` on answers); SQLite-relational, so true graph queries (reachability/shortest-path) aren't first-class. It is aOa-shaped *minus the substrate's differentiators* — the competitor whose feature list overlaps ours most, with the gap living entirely *below the index*.

### The rest of Cluster 1, at a glance
| Project | Stack | One-liner / value | Structural weakness aOa exploits | Source |
|---|---|---|---|---|
| **CodeGraphContext (CGC)** | Python, FalkorDB-Lite/Kuzu, tree-sitter+SCIP, 23 langs | Lowest-friction local setup; `cgc watch`; pre-indexed `.cgc` bundles | No commit stamping, no recon; single-maintainer (187 open issues); callers/callees overlap fresh grep→peek | [repo](https://github.com/CodeGraphContext/CodeGraphContext) |
| **Blarify** (Blar) | Python, tree-sitter + **LSP/SCIP** + Neo4j | Highest-fidelity reference resolution (SCIP "330× faster than LSP," compiler-grade) | **No graph refresh** — incremental update is roadmap, not built; buys edge precision at the cost of freshness | [repo](https://github.com/blarApp/blarify) |
| **FalkorDB code-graph** | FastAPI+React, FalkorDB, MCP (7 tools) | 3D force-graph + GraphRAG NL→Cypher; `impact_analysis`/`find_path` | Exists to sell FalkorDB; **SSPLv1** (commercially hostile); build-time, LLM-mediated chat | [repo](https://github.com/FalkorDB/code-graph) |
| **code-graph-rag** (+`@er77` fork) | Memgraph, tree-sitter (12 langs), NL→Cypher | In-graph editing + NL query, clean unified schema | Memgraph-bound; NL→Cypher is an LLM translation-error surface; build-time | [npm](https://www.npmjs.com/package/@er77/code-graph-rag-mcp) |
| **GitNexus** | tree-sitter WASM + LadybugDB WASM, in-browser | Zero-server, no upload — privacy story | Browser perf ceiling; same standing-graph staleness | [MarkTechPost](https://www.marktechpost.com/2026/04/24/meet-gitnexus-an-open-source-mcp-native-knowledge-graph-engine-that-gives-claude-code-and-cursor-full-codebase-structural-awareness/) |
| **Phoenixrr2113/codebase-graph** | FalkorDB-Lite + HNSW | **Bitemporal** + SHA-256 incremental change detection | Genuinely differentiated idea; still a standing artifact, no provenance/recon | [repo](https://github.com/Phoenixrr2113/codebase-graph) |
| **tirth8205/code-review-graph** | local graph, MCP+CLI, **GitHub Action** | CI-runner builds the graph; no code leaves CI | CI-gated freshness (not continuous); no provenance/recon | [repo](https://github.com/tirth8205/code-review-graph) |

Plus the academic confirmation that **the lane is filling**: *Codebase-Memory* ([arXiv 2603.27277](https://arxiv.org/html/2603.27277)) and *Prometheus* are tree-sitter-KG-over-MCP systems in exactly this lane. Differentiation must be the assets these tools structurally lack — not the lane itself.

**Cluster-1 synthesis — five recurring weaknesses (the moat surface):** (1) staleness is structural and *fails silently* (graphify's own tracker proves it); (2) untyped or LLM-noisy edges (only Blarify matches deterministic precision, and pays in freshness); (3) **no provenance** — not one stamps `file:line:commit`; (4) **no live human+agent recon signal** — none observes what humans/agents actually touch; (5) graph-as-primary-retrieval is the bet the frontier CLI agents already declined (graphify and codegraph ship install hooks that *nudge agents off grep*).

---

## Cluster 2 — Code-Intel / Code-Search Incumbents (distribution + capital)

The whole cluster is **"index/graph as a hosted build artifact."** Every freshness story bottoms out at *minutes* (Glean, GitHub) or *opt-in/periodic* (Sourcegraph), on cloud infra, per-seat + usage billing. aOa doesn't out-scale them — it out-*freshes* and out-*proves* them at the project boundary the agent actually works in.

### Sourcegraph — the incumbent that pivoted out from under itself
- **What it is.** Cross-repo code search + precise navigation over **SCIP**, repositioned as enterprise "code understanding/oversight" + an MCP server.
- **Value prop / traction.** SCIP go-to-def / find-references; **MCP Server v6.8 (Sept 2025), experimental, Enterprise-only** ([changelog](https://sourcegraph.com/changelog/sourcegraph-mcp-server)). **2025 self-disruption (verified):** Cody new Free/Pro signups stopped June 25, 2025, Free/Pro terminated July 23, 2025; **Amp** spun out into a separate company Dec 2025 ([Sourcegraph blog](https://sourcegraph.com/blog/changes-to-cody-free-pro-and-enterprise-starter-plans), [Wikipedia](https://en.wikipedia.org/wiki/Sourcegraph)). Pricing third-party/flagged (~$59/user/mo, platform ~$16K+).
- **Structural weakness aOa exploits.** Precise nav is **opt-in and upload-driven** (admins upload SCIP indexes per repo); **staleness is admitted and structural** — Sourcegraph's own scip-clang team calls incrementality "the bigger elephant in the room," full reindex of large repos is **1–2h**, and per-commit reindex is **on the roadmap, not shipped** ([scip-clang blog](https://sourcegraph.com/blog/announcing-scip-clang)). No per-method byte ranges as an agent primitive, no commit-stamped provenance, server-hosted/org-scoped. **SCIP precise-nav is the most direct functional overlap — and it's a heavyweight, opt-in, periodically-stale artifact.**

### Glean — the $7.2B enterprise knowledge graph (adjacent, not a code tool)
- **What it is.** Enterprise *work* search over an "Enterprise Graph" of 100+ SaaS sources; code repos are *one connector*. Not a head-on competitor — the canonical "KG-over-MCP, permission-gated" enterprise pattern aOa is compared against.
- **Traction (verified).** **$150M Series F at $7.2B (June 10, 2025)**; **ARR doubled to $200M by Dec 2025** ([Businesswire](https://www.businesswire.com/news/home/20250610090029/en/Glean-Raises-$150M-Series-F-at-$7.2B-Valuation-to-Accelerate-Enterprise-AI-Agent-Innovation-Globally), [Futurum](https://futurumgroup.com/insights/glean-doubles-arr-to-200m-can-its-knowledge-graph-beat-copilot/)). MCP server + host; differentiator is **permission enforcement at the data layer**.
- **Structural weakness aOa exploits.** Cloud-crawled, org-wide, **~5-min webhook latency** ([Glean docs](https://docs.glean.com/connectors/crawling-frequency)); no per-method granularity, no AST-deterministic code edges, no commit-stamped code provenance. Wins "everything in the company"; structurally cannot win "this repo, right now, with proof." *Borrow its permission-at-the-data-layer framing for any future enterprise story; don't position head-on.*

### GitHub — the distribution giant (Blackbird + Copilot + MCP registry)
- **What it is.** Planetary-scale code search (**Blackbird**, Rust) + Copilot + an **MCP registry** that makes GitHub the discovery/governance chokepoint for MCP servers.
- **Traction (verified, instructive).** Blackbird: incremental indexing, `<1s` for 95% of queries, **commit-level consistency**, new code searchable **"within minutes"** on 5,184 vCPUs / 53B+ files ([GitHub engineering](https://github.blog/engineering/architecture-optimization/the-technology-behind-githubs-new-code-search/)). **The best-engineered freshness story among incumbents — and it still lands at "within minutes" with a petabyte of infra.** MCP Registry + enterprise allowlist controls shipped through 2025 ([Meet the MCP Registry](https://github.blog/ai-and-ml/github-copilot/meet-the-github-mcp-registry-the-fastest-way-to-discover-mcp-servers/)). Copilot Business $19/Enterprise $39/user-mo, usage-based from June 1, 2026 ([GitHub blog](https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/)).
- **Structural weakness aOa exploits.** Blackbird is **lexical/regex, not a structural graph** — no reachability, DSM, affected-set, or per-method semantics; the agent gets text matches, not architecture. No `file:line:commit` provenance contract; minutes-latent. **The real threat is distribution + the registry as gatekeeper — so the counter is to *be in the registry* as the always-fresh, provenance-stamped, project-scoped server that complements (not competes with) the lexical index.** Ride the wave; don't fight the ocean.

### Augment — the "Context Engine" scale play (closest philosophical rival)
- **What it is.** A Context Engine indexing **~500K files across dozens of repos** so multi-agent sessions share one architectural picture; supports Claude Code/Codex/OpenCode as providers.
- **Traction.** Credit-based pricing (Oct 2025): Standard $60/dev, Max $200/dev; reception "mixed" ("credits burn fast") ([Augment blog](https://www.augmentcode.com/blog/augment-codes-pricing-is-changing)).
- **Structural weakness aOa exploits.** Its moat *is* a maintained cloud semantic index — **exactly the staleness-and-cost surface the frontier CLI agents rejected** (Claude Code removed vector search May 2025). Cloud-hosted, credit-metered, index-grounded, non-deterministic, no commit stamp. **It validates the demand for codebase-wide context while being most exposed to the freshness/provenance critique** — where Augment says "we index 500K files in the cloud," aOa says "we parse this project deterministically, on save, with `file:line:commit`, no credits, no upload."

**The CLI-won signal (cluster-2 meta-finding).** The frontier agentic-coding camp converged on **grep-in-a-loop over maintained indexes**, and even index-heritage vendors run **hybrid** — Sourcegraph's Amp layers `Grep`/`glob`/`Read` + an agentic `codebase_search_agent` *on top of* the graph ([DeepWiki: Amp](https://deepwiki.com/x1xhlol/system-prompts-and-models-of-ai-tools/5.3-amp-by-sourcegraph)). This confirms the prior verdict: **hybrid (structural query + grep fallback) is the convergent design**, and aOa's CLI-first + thin-structural-graph shape is on the winning side.

---

## Cluster 3 — Architecture & Code-Governance Incumbents (the $ market)

This is the **money cluster** — sold to M&A diligence teams, ARBs, compliance officers, modernization programs at **$10K–$800K+ per app/yr** plus 7-figure consulting. **One prior "fact" must be retired here:** *"competitors have no agent surface"* is **now false** — CAST shipped an MCP server (GA from Aug 2025), Sonargraph shipped one (June 2026), Sonar shipped intended-vs-actual "Architecture" conformance (GA on Cloud March 2026). The durable gap is **what their MCP serves: a stale batch artifact** — not the existence of an agent interface.

| Vendor | What it sells | Pricing (flagged) | Structural weakness aOa exploits |
|---|---|---|---|
| **CAST** (Imaging + Highlight) | "MRI for software" — reverse-engineered 3D blueprints, ISO 5055 scoring, M&A tech-DD exhibits | **~$10.29K/yr floor, six-figure-per-app ceiling**, per application ([pricing](https://www.castsoftware.com/imaging/pricing), [Capterra](https://www.capterra.com/p/275956/CAST-Imaging/)) | **Batch reverse-engineered map**, not a live index; inferred "hidden links" presented without confidence labels; per-app shelfware economics. **Caveat: CAST genuinely out-covers aOa** (150+ techs incl. mainframe) — their one real coverage advantage. Has an MCP server with a *near-identical* "LLMs need deterministic context" pitch ([CAST news](https://www.castsoftware.com/news/cast-announces-early-access-to-cast-imaging-mcp-server)) |
| **vFunction** | Runtime **architectural observability** + drift events + refactoring engine | Usage-based, per-app t-shirt; no public figures | The one true *runtime* play (aOa's explicit out-of-scope line); Java/.NET only; **MCP server status unverified** as of June 2026 |
| **Sonar** (SonarQube + Architecture) | Intended-vs-actual conformance, cycle detection, CI quality gates | Bundled; Cloud Architecture **GA for all incl. Free March 2026** | Architecture feature: **5 langs (Cloud) / 1 (Server)**; CI-scan artifact, not live; agent-forward but conformance MCP exposure only partially verified |
| **Sonargraph** (hello2morrow) | DSL to declare+enforce arch rules, DSM, cycle break-up | Rental/perpetual, consulting-shaped; no public figures | MCP server (June 2026) is **Java + file-based models only** — real but very narrow ([blog](https://blog.hello2morrow.com/2026/06/sonargraph-mcp/)) |
| **NDepend** | .NET static analysis, DSM, SQALE tech-debt | **~$399–492/dev perpetual** (the one real public number) ([Wikipedia](https://en.wikipedia.org/wiki/NDepend)) | **.NET/C# only**, Windows-only UI; desktop/CI scan, not live |
| **Lattix / Structure101** | DSM dependency governance; ISO 26262 / IEC 62304 compliance evidence | Quote-by-sales; no public figures | Batch DSM artifact; their regulated-industry framing *validates* aOa's evidence-pack white space |
| **Moderne / OpenRewrite** | Automated code remediation at scale; LST + deterministic recipes | $30M Series B (Feb 2025); OSS core | **Closest to aOa's determinism thesis** — but LST is JVM-centric and built for *transformation*, not governance/diagram/freshness. *Stay out of transformation (they own it); own the truthful before/after* ([financialcontent](https://markets.financialcontent.com/stocks/article/gnwcq-2025-2-11-moderne-secures-30m-to-drive-billions-in-enterprise-code-modernization-savings-based-on-its-innovative-tech-used-by-aws-microsoft-and-broadcom-ai-assistants)) |

**Cluster-3 durable gaps (corrected and honest):** (1) **batch, not live** — the strongest gap; every map is a scan artifact fresh only as of the last run; (2) **no provenance honesty** — none stamps `file:line:commit` or labels REAL vs INFERRED; (3) **per-app shelfware economics** ($10K–$800K/app → orgs image 3 flagship apps, leave 300 dark); (4) **language-capped *at the architecture surface*** (Sonar 5/1, NDepend .NET-only, vFunction Java/.NET) — **except CAST, which out-covers aOa, be honest about it**. **Retired:** "no agent surface." The reframe is *"they bolted MCP onto a build artifact; aOa serves a live, provenance-stamped substrate."* The market has *also validated* that conformance / intended-vs-actual / drift is **worth paying for** — which is aOa's in-scope diff view.

---

## Cluster 4 — AI Coding Agents + Plan / Diagram / Before-After Surfaces

Two surfaces have converged, and both leave the seam open. **Plan-before-execute is table-stakes — and universally *prose*. Diagram-from-code is universal — and universally *LLM-guessed*.** No tool combines them into a real-time, code-derived, provenance-backed diagram you can act on.

**The plan surface (all prose):** Claude Code Plan Mode (Markdown in `~/.claude/plans/`), Cursor Plan Mode (editable Markdown with file refs), **GitHub Copilot Workspace** (the closest anyone got — explicit *current-state / proposed-state* bullet lists — and it was **killed**, sunset May 30 2025), Windsurf/Cascade (inline diff staging; **Cascade EOL July 1 2026 → "Devin Local"**), Devin 2.0 (confidence-gated planning), Aider (architect mode), Amp (index-backed plan) ([Claude Code plan mode](https://blink.new/blog/claude-code-plan-mode), [Cursor docs](https://cursor.com/docs/agent/plan-mode), [Copilot Workspace](https://github.blog/news-insights/product-news/github-copilot-workspace/)). **The finding: no agent renders its plan as an architecture diagram, and none diffs the plan against a code-derived current-state model.**

**The diagram-from-code surface (all LLM-guessed, all stale):**
| Tool | Mechanism | Determinism / provenance | Freshness | Source |
|---|---|---|---|---|
| **GitDiagram** | Two-pass LLM → Mermaid from file tree + README | None (LLM-inferred edges) | One-shot, manual regen | [repo](https://github.com/ahmedkhaleel2004/gitdiagram) |
| **DeepWiki** (Cognition) | LLM + code analysis → wiki + diagrams | LLM-extracted "patterns" | **Re-index every few hours** (freshest LLM tool, still batch) | [Miraheze](https://ai.miraheze.org/wiki/DeepWiki) |
| **Swimm** | Mermaid "Smart Tokens" — backtick-bind labels | Partial: token-coupling, **not topology** | Best-in-class for *labels*, not graph structure | [Swimm blog](https://swimm.io/blog/create-up-to-date-diagrams-with-swimm-s-mermaid-integration) |
| **Swark** | LLM-only "natively supports all languages" | **Explicitly none** | Static | [repo](https://github.com/swark-io/swark) |
| **CodeSee** | Deterministic file/dep maps (the rare non-LLM one) | Closest to deterministic | **Acquired** by GitKraken (May 2024), folded into its DevEx platform | [GitKraken](https://www.gitkraken.com/blog/gitkraken-launches-devex-platform-acquires-codesee) |
| **Eraser DiagramGPT** | GPT-4 → diagram-as-code | None | Static | [eraser.io](https://www.eraser.io/diagramgpt) |

**The documented Achilles' heel — independently confirmed:** LLM-generated Mermaid suffers **syntax failures** (patched by parse-repair) and, more damning, **semantic drift/hallucination** — "renders fine but misrepresents the actual logic or has gone stale." Verification is hard (Mermaid admits many equivalent forms), so the best anyone has is **LLM-as-a-judge against a gold reference** ([GenAIScript](https://microsoft.github.io/genaiscript/blog/mermaids/)). **This is precisely the failure mode aOa's deterministic AST edges + blind-judge gate eliminate — a verified market pain, not a strawman.**

**The interactive "click-node → fire-agent" surface — emerging, unbacked:** **Microsoft's open-source Architecture Review Agent** renders a fully interactive Excalidraw diagram via MCP + risk analysis (SPOFs, fan-in/fan-out, orphans) — *but LLM-generated, no AST derivation, no provenance* ([MS Community Hub](https://techcommunity.microsoft.com/blog/educatordeveloperblog/stop-drawing-architecture-diagrams-manually-meet-the-open-source-ai-architecture/4496271)). **AG-UI** standardizes "frontend interaction → backend agent action" as a named 2026 pattern. *Microsoft built the interaction layer aOa wants and grounded it on sand; aOa has the ground and is building the interaction layer.*

---

## Cluster 5 — Interactive-Canvas / "Agentic Canvas" Tech (the borrowable layer)

**The "click-an-element-dispatches-an-agent" loop is a solved, productized pattern as of 2025–2026** — three independent stacks ship it. aOa is **not inventing the mechanism**; it is applying a known pattern to a *deterministic code-graph substrate none of them have.* This cluster is **conceded**: the interaction loop is won and commoditizing — the only delta is the AST-grounding underneath.

| Stack | What it is | Relevance to aOa | Source |
|---|---|---|---|
| **MCP Apps (SEP-1865)** | Co-authored **Anthropic + OpenAI**, announced 2025-11-21, ratified **2026-01-26** under the Linux Foundation. An MCP server returns a `ui://` sandboxed-iframe HTML/React UI that calls back into MCP tools bidirectionally | **The headline.** aOa's ReactFlow viewer can ship *inside Claude* as an MCP App; a node-click calls `aoa arch facts`/reachability/affected-set over the same MCP transport, returning `file:line:commit`. "One substrate, two faces" collapsed into one surface. Standards-track, provider-neutral, low-regret | [MCP Apps overview](https://modelcontextprotocol.io/extensions/apps/overview), [blog](https://blog.modelcontextprotocol.io/posts/2025-11-21-mcp-apps/) |
| **AG-UI** (CopilotKit) | Generic agent↔UI typed-event bus over HTTP/SSE (`TOOL_CALL_START`, `STATE_DELTA`, human-in-the-loop) | The mature wire format **if aOa serves its own localhost dashboard** (`web/server.go`) — node-click → STATE_DELTA → streamed agent suggestion. Complementary to MCP Apps, not competing | [docs.ag-ui.com](https://docs.ag-ui.com/introduction) |
| **A2UI** (Google) | Declarative JSON component-tree the agent emits; client renders with *its own* trusted components | The **safe** version for aOa's determinism thesis: the agent assembles views from aOa's *judged* view catalog, never invents pixels — preserves the blind-judge gate | [generative-ui](https://github.com/CopilotKit/generative-ui) |
| **tldraw** | "Canvas as agent control surface" across 4 generations (Make Real Nov 2023 → Agent Starter Kit late 2025); React/DOM-based, $10M Series A | **Make Real proved the exact "render output as canvas element → annotate → re-prompt" loop** the north star describes — viral since Nov 2023. aOa's AST-derived diff is the same loop grounded in real code, not a screenshot | [tldraw Series A](https://tldraw.dev/blog/announcing-tldraw-series-a), [Make Real](https://tldraw.dev/blog/make-real-the-story-so-far) |
| **ReactFlow** (`@xyflow/react`) | aOa's actual viewer stack; native `onNodeClick`/context-menu/button-edge | Click→dispatch is **native and trivial** — a few hours of React. The CopilotKit MCP demo already wires ReactFlow-node-click → MCP-tool end-to-end | [reactflow.dev](https://reactflow.dev/learn/concepts/adding-interactivity), [CopilotKit MCP demo](https://www.copilotkit.ai/blog/add-an-mcp-client-to-any-react-app-in-under-30-minutes) |
| **Figma / Miro / Excalidraw** | Text→diagram AI canvases (drawings the model invents) | Context, not threat. **None is a code-KG tool; all generate drawings with no AST derivation, freshness, or provenance.** Match their interaction *feel*; the *trust* is the wedge they structurally lack. (Excalidraw's `mermaid-to-excalidraw` bridge is reusable for static export) | [Excalidraw mermaid](https://github.com/excalidraw/mermaid-to-excalidraw) |

**Cluster-5 verdict:** the interaction technology is mature and borrowable; aOa's defensible position is **not the click — it's that clicking a node lands on a fresh, provenance-stamped fact**, which is exactly what tldraw/Figma/Miro/Excalidraw and even graphify cannot offer.

---

## Cluster 6 — Funding / Market Picture (the founder lens)

**The why-now maps exactly onto aOa's moat.** Anthropic formally named **"agentic technical debt" / "architectural drift"** in 2026 — agents "re-derive foundational choices each session," accumulating entropy at machine speed; the industry's named answer is **"harness engineering"** (architectural constraints that hold the design true) ([New Stack](https://thenewstack.io/hidden-agentic-technical-debt/), [Dev Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/)). **aOa's diff-renderer + provenance is a harness primitive.** Crucially, the same sources say *"memory tools solve for data recall but fail to provide architectural direction"* — and aOa supplies both: fresh facts (recall) *and* the machine-checkable conformance layer (direction).

**Funding climate — a barbell favorable to an early raise.** AI took 61% of global VC in 2025; **seed-stage AI startups carry a ~42% valuation premium** ([iExchange](https://iexchange.substack.com/p/the-2026-vc-playbook-how-investment-criteria-are-evolving-in-ai-first-startups)). Comparables: Potpie **$2.2M pre-seed**, Glean **$7.2B**, Moderne **$30M Series B**, Cursor **$29.3B**. **The fundable thesis:** *"Founders who answer with compliance moats — auditability, domain-specific verification — close rounds in weeks,"* while "hallucinations aren't a real issue" founders die in diligence. **aOa's determinism + provenance is a verifiability moat by construction** — and only **11–14% of enterprise agent pilots reach production**, so aOa's counter is that it is *not* a probabilistic pilot but a deterministic tool with a verifiable output.

**The growth motion — open-core PLG with a shareable artifact.** Free at `aoa init`; viral via the diagram (the "build-a-tool-whose-output-is-shareable" lever — every paste into a PR/doc/tweet is an impression); convert on **governance** (the universal upgrade trigger across HashiCorp/GitLab/Sourcegraph: free for the developer, paid for RBAC/audit/policy/conformance). The market is shifting **point-in-time → continuous compliance** ("auditors now expect near-real-time evidence") — and *a continuously-fresh, provenance-stamped architecture substrate is a near-real-time evidence engine* spreadsheets can't deliver. Note Cursor reportedly spends ~100% of revenue on AI inference; **a deterministic, no-LLM-inference substrate has structurally better unit economics** — itself a VC talking point.

**Could-not-verify (cluster 6):** no clean standalone "architecture governance" TAM (folded into GRC: compliance software ~$17.2B, SOC2-automation ~$850M, AI-governance ~$308M — all analyst/vendor-cited); Potpie's $1.1M revenue and graphify's traction beyond stars; all free-to-paid conversion percentages trace to agency blogs citing primary reports — directional, not investor-grade.

---

## 7. The White-Space Map — what NOBODY does

Five capabilities. Each axis is occupied **separately**. The four-way intersection is **empty** — confirmed by an explicit search for the union of (AST-level diagram) × (strict determinism) × (line-level provenance), which returned *"None of the results describe a single tool that fully packages all three as a polished product."*

| Capability | Who has it (separately) | Who has it **fresh + deterministic + provenanced** |
|---|---|---|
| Plan before execute | Everyone (Cluster 4) | **No one renders it as a diagram** |
| Current-vs-proposed structure | Copilot Workspace (**killed**) | **No one, code-derived** |
| Diagram from code | GitDiagram / DeepWiki / Swark / Eraser | **No one — all LLM-guessed, stale** |
| Click-node → fire agent | MS Excalidraw agent, AG-UI / MCP Apps | **No one, over real code** |
| Falsifiable visual quality gate | **No one** ("eyeball" is the universal bar) | — |
| `file:line:commit` on every element | **No one** | — |

**aOa is the only design positioned to occupy every row at once, because it already has the substrate the others fake with an LLM:**
- **Freshness** — rides the live fsnotify index, never a build artifact (beats GitDiagram's one-shot, DeepWiki's hourly batch, Glean/GitHub's "within minutes," Sourcegraph's 1–2h reindex). *Scoped honestly: structural for the local per-keystroke CLI; narrowing for the viewer — but ahead.*
- **Determinism** — AST-derived edges, confidence-1.0, no LLM in the path (beats every Mermaid-LLM tool's documented hallucination).
- **Provenance** — `file:line:commit` on every answer and every pixel. **Literally no competitor in any cluster surfaces this** — and it is exactly the **grounding receipt** the whole agentic-AI industry is discovering it needs (KPMG pulled an agentic report after only 5 of 45 citations checked out — [case](https://nerova.ai/troubleshooting-fixes/kpmg-pulls-agentic-ai-report-hallucinations-june-13-2026); only 28% of orgs can trace agent actions).
- **Visual quality gate** — the blind-judge readability test, a *falsifiable* acceptance bar against a market where "eyeball" is universal.
- **Diff-able architecture** — the BEFORE derived from code (not prose), the AFTER an agent's plan rendered in the same engine, the delta computed from two SHA-snapshot edge sets. **The un-fakeable artifact no LLM-drawn canvas can produce** — and the entire diagram market (Miro/Eraser/MS) does before/after as *two manually-prompted drawings*.

**The compounding wall (stated symmetrically, per the position's §0).** To neutralize aOa a competitor must ship *all five at once* — but that is **also aOa's own build obligation**: until the import-edge keystone + `aoa arch` surface land inside the G0 ≤+3% budget, the moats are a **roadmap**, not a position. The Phase-0 keystone *is* the de-risking milestone a seed investor funds toward. What ships **today** — the grep→peek CLI over the live index, with per-method ranges and provenance-anchored results — is the adoptable wedge; the diff-able, visually-gated, agent-firing canvas is what arrives once the keystone lands.

> **The one-line map:** every competitor serves the agent a *stale build artifact with no audit trail*; aOa serves *one fresh, provenance-stamped substrate to two faces* — the agent and the human — with a diff-able, visually-gated architecture no one else can draw, because for everyone else the diagram is a drawing of the code, and for aOa the diagram *is* the code.

---

*Grounding files: `playbook/ENHANCEMENT-GUIDE.md` · `playbook/enhancement/01-knowledge-graph-and-visualization.md` (blind-judge moat, the one demo) · `playbook/STRATEGIC-POSITION.md` (the red-teamed position this terrain supports) · `.context/details/2026-06-19-graphify-plus-mcp-research.md` (prior verdict) · `.context/decisions/2026-06-11-core-competence-and-scope-line.md` (binding scope law).*
