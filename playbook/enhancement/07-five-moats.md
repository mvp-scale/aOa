# 07 — The Five Moats (the strategic centerpiece)

**Status:** the defensible position, distilled. Companion to the full synthesis in
`playbook/STRATEGIC-POSITION.md` (§B) and the prior research verdict
`.context/details/2026-06-19-graphify-plus-mcp-research.md`. **Falsifiable
document:** every load-bearing claim cites a `file:line`, a source doc, or an
external URL; if a cited anchor is wrong, the claim built on it is void.
Vendor/marketing/unverifiable figures are flagged inline.

**Binding law (do not relitigate here):**
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
  (three-layer ladder: derive REAL / infer-leashed MIXED / declare-and-diff;
  leash text at `:26` is "**NEVER add a node**")
- **Goals** — `.context/GOALS.md` (G0 Speed ≤+3% build, G2 Two-Binary, G3
  Agent-First, G4 Hexagonal — non-negotiable)
- **Prior research verdict** — `.context/details/2026-06-19-graphify-plus-mcp-research.md`
  (established; sharpened here, not relitigated)
- **Sibling deep-dives** — `01-knowledge-graph-and-visualization.md`,
  `02-integration-touchpoints.md`, `03-access-surface.md`,
  `04-scale-and-positioning.md`, `05-redteam-alignment.md`

**What this doc is.** The four sibling docs answer *what to build* and *where it
attaches*. This one answers the only question a founder or a VC actually asks:
**what can aOa do that a funded competitor structurally CANNOT, and why won't a
feature-copy close it?** Five walls, each with a one-sentence wedge, the named
competitor inability it exploits (cited), the structural — not copyable — reason,
how it integrates the knowledge-graph data *better/faster* than the field, and the
single falsifiable demo that shows the value. Plus the cut candidates and why the
red-team cut them.

---

## §0. Read this first — the moats are a roadmap until the keystone ships

This is binding and non-negotiable: **the load-bearing graph surface does not exist
yet.** Verified absent against live source:

- **No `aoa arch` command** — `ls cmd/aoa/cmd/arch.go` → *No such file* (verified).
- **No import-edge keystone** — the recon dimension walker (`countImportSpecs`,
  `internal/adapters/treesitter/walker.go:568`) already traverses import nodes for
  the import-bloat rule, but that walk runs only in the parked recon path
  (`WalkForDimensions`, `walker.go:21`, invoked at `:483`); the always-on `extractGo`
  pass (`internal/adapters/treesitter/parser.go:347`) does not visit import nodes at
  all (prior research §Q1). This is the foundation every moat below rides.
- **No overlay loader, no `MethodArch*` socket methods, no arch-shard web
  endpoint, no AG-UI** (verified absent).
- **The file-save→ETag tick does NOT exist.** `bumpRevision()`
  (`internal/app/app.go:350`) is called only by the search-observer, session-event,
  and investigated-file paths — **never** by `onFileChanged` (`watcher.go`) or
  `Reindex` (`app.go`). A code edit reindexes symbols but does **not** bump the
  ETag, so the live viewer serves a stale `304` after a save until an unrelated
  event fires. The "real-time on save" viewer needs **one currently-absent line**.
  The `withETag`/`304` *transport* ships (`internal/adapters/web/server.go:34,52`);
  the file-change *trigger* does not.

**The symmetric concession.** The strongest sentence in this doc — *"to neutralize
aOa a competitor must do all five"* (§B.6) — is also **aOa's own build
obligation.** Until the keystone lands inside the G0 ≤+3% budget on the three tuned
extractors, these are a **fundable roadmap, not a position.** Every demo below is
written in the **future tense** and gated on that keystone. The §E Phase-0 gate in
`STRATEGIC-POSITION.md` *is* the de-risking milestone a seed investor funds toward.

**One shippable-today exception:** the sub-ms `grep → peek` CLI over the O(1) token
index, atlas enrichment, and the blind-judge-gated build-time viewer all exist now
— that is the Phase-0 wedge (`03-access-surface.md`), and it carries adoption with
zero new code while the keystone is built. The moats are what that adoption
*retains* and what diligence *funds*.

---

## §1. The two axes — why "five co-equal walls" is the wrong frame

The honest split (it survived the four-lens red-team in `STRATEGIC-POSITION.md`):
moats divide on **what a user feels** vs **what survives diligence.** A developer
adopts for the first pair and never for the second; a VC funds the second and
discounts the first. A doc that sells all five as equal walls fails both audiences.

| | Axis | Moat | Role |
|---|---|---|---|
| **A** | Adoption headliner (felt/shared) | **Freshness, scoped to the local CLI answer** | the demoable "ours moved, theirs didn't" |
| **B** | Adoption headliner (felt/shared) | **The AST-derived before/after DIFF** | the one natively viral decision artifact |
| **C** | Diligence spine (defensible) | **Deterministic, thin-MCP delivery shape** | the token receipt + the 4-vs-45-tool wall |
| **D** | Diligence spine (defensible) | **Provenance on every answer/pixel** | the grounding receipt the industry is discovering it needs |
| **E** | Diligence spine (defensible) | **The live recon signal** | switching-cost / system-of-record — **NOT** a data network effect |

The body leads adoption with **A + B** and reserves **C / D / E** for the
diligence/retention story. §B.6 is the binding mechanism that makes the set
compound — and the one place the obligation cuts back on aOa.

---

## §2. The competitor inabilities these exploit (the table the moats sit on)

Distilled from `04-scale-and-positioning.md` and the §A landscape in
`STRATEGIC-POSITION.md`. The recurring weaknesses **are** the moat surface —
narrowed to what survives 2026:

- **graphify** (safishamsi, ~69.2K stars *real*; "YC S26" **self-applied /
  unverified** — `ycombinator.com/companies/graphify` 404s, S1): a **silently stale
  build artifact** — issue [#653](https://github.com/safishamsi/graphify/issues/653)
  (rebuild refuses to overwrite, MCP serves the old graph),
  [#341](https://github.com/safishamsi/graphify/issues/341) (`update` 114 min,
  unfinished on a 50K-file repo). LLM-inferred edges → ambiguous tags.
- **Potpie** ($2.2M pre-seed Feb 2026, verified —
  [FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html)):
  Neo4j + Celery/Redis **batch ingestion, no incremental path**
  ([TFN](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/)).
  **Ships blast-radius today** — so blast-radius-as-a-feature is table-stakes.
- **2026 watch-mode cluster** — codegraph, codemap, codebase-memory-mcp, CodeGraph
  (45 MCP tools — [repo](https://github.com/codegraph-ai/CodeGraph)): local-first,
  tree-sitter, **file-watch + incremental + MCP**. The closest cluster — it proves
  **file-watch is table-stakes**, so freshness is *narrowing*, not a standalone
  wall. **No provenance-on-every-answer, no recon signal, no blind-judge gate, no
  AST-derived diff** — that is aOa's surviving surface.
- **Sourcegraph (SCIP/Amp):** SCIP was **designed for incremental**
  ([announcing-scip](https://sourcegraph.com/blog/announcing-scip)) but incremental
  remains *"on the roadmap"* ([scip-clang](https://sourcegraph.com/blog/announcing-scip-clang));
  full reindex 1-2 h on large repos. The cadence gap is a tunable parameter, not a
  wall.
- **CAST** (M&A "MRI for software" + MCP Aug 2025): **$10.29K/app/yr verified**
  pricing; batch reverse-engineered map; heuristic "hidden links," no confidence
  labels. *Genuinely out-covers aOa on language breadth (150+, mainframe) — its one
  real advantage.*
- **Microsoft Architecture Review Agent / the agent-canvas SDK cluster** (tldraw
  "Agents on the Canvas" — [JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw);
  Excalidraw+MCP — [mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw);
  Mermaid-living-contract): the **interaction loop is a won, commoditizing 2026
  pattern.** But every one grounds on the **LLM's reading of a screenshot / shape
  data / prose / Mermaid text** — *none grounds on AST with `file:line:commit`.* The
  grounded intersection is empty; the ungrounded one is crowded.

**One correction carried forward (retire it):** the old "competitors have **no agent
surface**" claim is **false** — CAST (Aug 2025) and Sonargraph (June 2026) ship
MCP. The durable truth: *they bolted MCP onto a stale, per-app-licensed, unstamped
batch artifact; aOa's MCP rides a live, provenance-stamped substrate.*

---

## §B. The Five Moats

> Each moat: **wedge** (one sentence) · the **named competitor inability** it
> exploits (cited) · **why structural, not copyable** · **how it integrates the KG
> data better/faster** · the **falsifiable demo** (future tense — gated on §0) · the
> **conceded seam** (where a red-teamer is right).

---

### ADOPTION HEADLINER — MOAT A — Freshness, scoped to where it's structural

**Wedge.** Edit a file; aOa's CLI answer is current sub-ms on the next keystroke,
while the build-artifact competitor still returns the pre-edit graph **and doesn't
warn you.**

**Competitor inability — where it actually holds.** Decisive and demoable against
the **build-artifact tier**: graphify [#653](https://github.com/safishamsi/graphify/issues/653)/[#341](https://github.com/safishamsi/graphify/issues/341);
CAST/Potpie/Sonargraph batch; Sourcegraph 1-2 h full reindex (incremental "on
roadmap"); GitHub "within minutes." **Narrower against the watch-mode cluster** —
they ship file-watch + incremental too, and their own coverage recommends "daily
full rebuild with fingerprint comparison to catch desync."

**Why structural for the CLI answer, narrowing for the viewer.** The genuine
structural floor is the **local per-keystroke CLI/socket answer**: no JSON-RPC
round-trip, O(1) token lookup off the live in-memory index — a latency an
incumbent's cloud-indexed pipeline *structurally cannot reach.* For the
graph/viewer face, freshness is **strong-but-narrowing**: an incumbent narrows the
cadence gap (minutes→seconds, per-commit→per-push) with a tunable parameter.
**aOa's durable distinction over the cluster is not "they fail silently" — it is
that aOa never builds a standing artifact to drift against; the substrate IS the
working tree.**

**Integration (better/faster).** Once the keystone lands (REAL import edges riding
the always-on `extractSymbols` pass, G0 ≤+3%), edges inherit the `fsnotify →
reindex` path for free — the same invalidation tick feeds the agent's socket read
and the human's ETag-polled canvas (`02-integration-touchpoints.md` seam 6).
**Required, currently-absent change (§0):** `onFileChanged`/`Reindex` must call
`bumpRevision()` (`app.go:350`) so the viewer's ETag invalidates on the file-change
tick — **one line, not yet present.**

**Demo (future tense — gated on keystone + the `bumpRevision` line).** `aoa init` a
stranger's repo → REAL-stamped DSM/cycles render → edit one package, watch only the
affected shards change live → the same query against graphify shows the pre-edit
graph (reproduces #653). *"Ours moved, theirs didn't, and theirs didn't warn you."*

**Conceded seam.** The viewer is content-hashed shards: "live" is recompute + ETag
poll, **not streaming**, and the file-change→revision wiring is **absent** (§0). The
moat is airtight for **CLI/socket answers**; **near-live and currently un-wired** for
the viewer. Do not claim "refreshes on every keystroke" for the diagram.

---

### ADOPTION HEADLINER — MOAT B — The AST-derived before/after DIFF

**Wedge.** aOa derives **both** sides of a before/after from the same substrate —
BEFORE = live code-derived current state, AFTER = an agent's proposed plan re-run
through the **same** deterministic detectors — and computes the delta from edge-set
arithmetic, gated by a blind judge. **This is the one share-worthy decision
artifact teammates argue over.** Every competitor fakes "before/after" as two
hand-drawn pictures and "quality" as eyeballing.

**Competitor inability — the GROUNDED intersection is empty.** The *interaction*
loop is a **won, commoditizing 2026 pattern** — tldraw "Agents on the Canvas"
([JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw)),
Excalidraw+MCP ([mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw)),
Mermaid-as-living-contract ([erdembircan](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development)).
**aOa does not invent the canvas.** The single defensible delta: every shipping
canvas grounds on the LLM's reading of a screenshot / shape data / prose / Mermaid
text (tldraw's own `BlurryShape`/`FocusedShape` are *simplified* shape summaries for
model understanding); **aOa's BEFORE is AST-derived and every node is
`file:line:commit`.** Every plan surface (Claude Code, Cursor, Copilot, Aider, Amp)
is Markdown prose; Copilot Workspace's current-vs-proposed spec was **sunset May
2025**. Microsoft's agent does its analysis over an **LLM's reading of a brain-dump.**

**Why structural (the diff, not the canvas).** The diff is **un-fakeable only if the
BEFORE is *derived*, not drawn.** A competitor without a deterministic substrate can
only redraw two pictures; aOa diffs two SHA-snapshot edge-sets and the delta (new
cycles, affected closure, new findings) **falls out of set arithmetic.** The
**blind-judge gate** (`playbook/standards/MODEL-STANDARD.md`,
`playbook/standards/view-standards.json`) is a falsifiable acceptance test no
canvas-SDK ships — an LLM-drawn diagram that admits to hallucinating can't reliably
pass it. The **leash rule** (scope law `:27-30`: the diagram cannot say anything the
facts don't) makes the rendered view hallucination-immune.

**Integration (better/faster).** Free to compute off the keystone edges — no doc to
maintain, no LLM pass. New data = new edge-sets to diff: declare the intended
pattern → diff against derived actual → **conformance/drift** (the §D.4 paid tier in
`STRATEGIC-POSITION.md`) — *this is precisely the "architectural direction" Anthropic's
named remedy calls for* ([Dev|Journal on "agentic technical debt"](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/)).

**Demo (future tense — Phase 2; depends on keystone + the net-new overlay loader,
§0).** Claude (Plan Mode) proposes a refactor → render its plan as an overlay on the
REAL current import graph (AFTER = SIMULATED, BEFORE = REAL) → **click the proposed
edge → `aoa arch blast` fires → "this creates a cycle through these 3 modules, blast
radius 12 files."** Side-by-side with Microsoft's agent: theirs draws what you
*described*; aOa diffs what your code *is* against what the plan *would make it.*

**Conceded seam (the most-contingent thing in the position).** The diff renderer is a
**Phase-2 target, not shipped**; it depends on the keystone (≤+3%) **and** the
net-new overlay loader (§0). The strongest adoption artifact arrives in **Phase 2,
not at launch** — do not front-load a launch on it.

---

### DILIGENCE SPINE — MOAT C — Deterministic, thin-MCP delivery shape

*Investor/retention moat. Lead with the token economics — "deterministic edges are
novel" is no longer true.*

**Wedge.** aOa serves the *few* genuinely graph-shaped queries grep can't answer
(reachability, blast-radius, cycles/DSM, god-nodes) as deterministic
confidence-1.0 tools on a fresh grep/peek spine — the **hybrid** shape the frontier
proved wins (`03-access-surface.md`).

**What is conceded table-stakes (in-body).** **Blast-radius/reachability as a feature
is table-stakes** — Potpie ships it today, funded ([FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html));
graphify and CAST expose it. **Deterministic AST extraction + EXTRACTED-vs-INFERRED
tagging is now standard 2026 practice** (graphify itself ships confidence tags; the
watch-mode cluster ships typed edges). "No LLM in the path" is **not** a wall.

**What actually survives diligence — two sub-claims.**
1. **Thin-MCP enforced by the scope law, not willpower** — Tier-1 refuses
   call/inheritance/LLM-semantic edges and exposes **only 4 grep-beaters**, where the
   2026 cluster ships **9-45 tools** (CodeGraph: **45**; codebase-memory / codemap in
   the 9-22 band; graphify: 10). A competitor whose *business is the rich graph*
   structurally can't shrink to 4 tools without abandoning its value prop.
2. **The token receipt** — Anthropic measures **~40% of context can go to MCP tool
   metadata**; a CLI-first agent that shells out pays **zero standing
   tool-definition tokens.** Source: MCP/context-engineering cluster (S6 §5).

**Why structural (not copyable).** The thinness is *forced by the scope-law ladder*,
not chosen by discipline a competitor could also choose: aOa's value prop is the
deterministic substrate, so refusing rich edges costs it nothing — refusing them
costs graphify/Potpie *their product.* The constraint that is free for aOa is
expensive for them.

**Integration (better/faster).** All four queries ride keystone edges: reachability =
walk; blast-radius = git-changed-files ∩ edge closure (**never stale** — the PR
file-list is a git feature); cycles/DSM = Tarjan/matrix; god-nodes = fan-in/out.
Sub-ms, stamped, fresh — tools the agent reaches for *when grep can't answer in one
shot*, never a replacement retrieval index (`01-knowledge-graph-and-visualization.md`
§2).

**Demo (future tense).** "What breaks if I change `X`?" → `aoa arch blast X` returns
the affected closure in <1 ms, stamped, fresh. Then the **token receipt**: CLI-first
0 standing tokens vs the cluster's 9-45-tool MCP load.

**Conceded seam.** A competitor *could* add grep fallback (Amp did). The surviving
structural part is narrow: **deterministic + fresh + zero-standing-token +
scope-law-enforced-thinness.** If a competitor ships all four, Moat C compresses to
A + D — so **C is delivery shape, not load-bearing alone; its durability rides A and
D.**

---

### DILIGENCE SPINE — MOAT D — Provenance on every answer/pixel

*Investor/trust moat. A developer doesn't adopt for "provenance is correctly scoped
by layer" — but it is the spine both faces read, and the compliance moat VCs fund.*

**Wedge.** Every answer and every rendered element carries `file:line:commit` — the
diagram and the agent's answer are the **same auditable fact.**

**Competitor inability (the least-contested gap).** Across every scan: **not one
competitor stamps `file:line:commit` on every answer**, none labels REAL vs INFERRED.
**Demand-side proof:** KPMG pulled an agentic report after **5 of 45 citations**
checked out ([nerova.ai](https://nerova.ai/troubleshooting-fixes/kpmg-pulls-agentic-ai-report-hallucinations-june-13-2026));
deep-research agents show a large factual-accuracy drop (the paper reports ~39-77% accuracy across systems, ~42% average drop)
([arxiv 2605.06635](https://arxiv.org/html/2605.06635v1)); "Tool Receipts, Not
Zero-Knowledge Proofs" ([arxiv 2603.10060](https://arxiv.org/html/2603.10060v1))
names the need as *a verifiable receipt that a claim traces to a real source* —
exactly `aoa arch facts`.

**Why structural (couples to determinism — not copyable).** Provenance is only
trustworthy if the thing it stamps is derived deterministically. **LLM-inferred
edges** (graphify's `semantically_similar_to`, GraphRAG, NL→Cypher, Microsoft's
Excalidraw agent) **structurally cannot stamp a true source** — there's no single
line that produced an inferred edge, only a probability. They can *fake* a citation;
a faked citation is the exact KPMG failure. A competitor whose graph is part-LLM
cannot bolt on honest provenance without ripping out the LLM extraction that is
their value prop.

**Integration (better/faster — one fact, two faces).** `aoa arch facts <unit>` is the
single audit trail behind any rendered element — one fact rendered two ways, not
stored twice (`01-knowledge-graph-and-visualization.md` §1). New data
(lockfile/SBOM, churn) attaches to an already-provenanced node — breadth without
losing the receipt. Per-method `[start-end]` ranges (the cut "granularity" candidate,
§C) make the stamp **method-precise**, below the grain any module-graph competitor
models.

**Demo (future tense).** Click any node → `aoa arch facts` returns the exact
`file:line:commit` set behind it; the same fact backs the agent's CLI answer.
Side-by-side with an LLM-drawn diagram that can only gesture at "AuthService."

**Conceded seam (the honesty that wins).** Split by **scope-law layer** (`:24-26`): on
**layer-1 REAL edges**, `source` is an **audit/freshness anchor** (re-derivable,
commit-recorded) — closer to freshness metadata than a safety mechanism; on
**layer-2 MIXED** content (agent grouping/naming), the stamp is **the load-bearing
leash** pinning inferred buckets to REAL facts. Do not let "provenance makes
inference safe" overstate the layer-1 case.

---

### DILIGENCE SPINE — MOAT E — The live recon signal (switching-cost, NOT a data network effect)

*Feature-gap + embedding moat. The a16z framing the prior draft tripped on is
respected here: single-team usage data is a **scale** effect, not a **network**
effect — drop the "data network effect" language entirely.*

**Wedge.** aOa observes what humans and agents *actually touch over time* (searches,
tool-invocations, file-hits feeding `observe()`/autotune) and folds that lived signal
back into the substrate — a behavioral layer **no competitor in any scan has an
equivalent of.**

**Competitor inability (the cleanest "no equivalent").** Every competitor models the
**static** graph; none observes what humans and agents actually touch over time.
vFunction observes *runtime* traces — a different signal, and aOa's explicit
out-of-scope line.

**Why defensible — the correct framing.** Not a data *network* effect (single-team
data doesn't compound for other teams — [a16z, "The Empty Promise of Data Moats,"
Casado/Lauten 2019](https://a16z.com/the-empty-promise-of-data-moats/) — framing is
evergreen VC consensus, *not a 2026 publication*). It defends as **(a) a
feature-differentiator no competitor has**, and **(b) a switching-cost /
workflow-embedding moat** — aOa is embedded in the team's live grep/agent loop and
*is the index the repo already runs through* (system-of-record status). That is the
combination the same VC consensus says actually defends: proprietary data **paired
with deep workflow embedding.**

**Integration (highest upside, least-built).** Overlay recon onto the import/DSM graph
to rank what matters: a god-node that's *also* recon-hot is a different finding than
one nobody touches; blast-radius weighted by "the modules your team actually edits"
beats topology-only. Competitors get *structural* centrality (Microsoft, NDepend
fan-in/out); **aOa can fuse structural + behavioral** — the one moat that strengthens
the others *with use.*

**Host-coupling (stated honestly).** The recon signal source is
`~/.claude/projects/*.jsonl` — the recon + live-status loops require a **running
daemon + Claude Code session logs.** This narrows the *immediately*-addressable base
to **Claude Code users** — a real bound, framed as a **beachhead** (the
fastest-growing CLI-agent base), not hidden.

**Demo (future tense).** Two `god_nodes` views side by side: topology-only (every
competitor) vs recon-weighted (aOa) — the second surfaces the abstraction the team
*actually* churns, which the first buries.

**Conceded seam.** This layer today powers learner/autotune/status-line; its
connection to the *architecture graph* is **not yet built.** Real as an asset,
**under-integrated.** Claim the signal exists and uniquely *can* drive graph
overlays; do **not** claim the diagram already shows recon-weighted hot paths.
**Because it's not yet built into the graph, it cannot drive adoption now — it's a
retention/diligence moat, not a wedge.**

---

## §B.6 — Why the set compounds (the binding mechanism — and aOa's own obligation)

The cut sixth candidate — **two-faces-one-substrate** — is not a moat; it is the
*mechanism* that makes the five compound:

- **Moat D (provenance) is the spine** — the shared primitive both faces read.
  Without it, two faces are two products.
- **Moat A (freshness) keeps both faces in sync on one tick** — once the absent
  `bumpRevision()` line lands (§0), agent answer and human diagram invalidate
  together.
- **Moat B (diff) only works because A + C + D produce a fresh, deterministic,
  provenanced BEFORE** to diff against.
- **Moat E (recon) is the only one that strengthens the others with use** —
  re-weighting C's queries and B's findings by lived attention.
- **Moat C (deterministic thin hybrid) is the delivery shape** that gets it to the
  agent in the form the frontier proved wins.

**Defensibility of the set — stated symmetrically.** To neutralize aOa a competitor
must invert their data plane *for the local answer* (A), rip out LLM extraction
(C, D), build a blind-judge gate + a substrate clean enough to pass it (B), and
accumulate a recon dataset + workflow embedding they can't clone from code (E) —
all bound by one provenance contract. **But this cuts both ways: aOa must ALSO do
all five, and has shipped zero of the load-bearing graph work (§0).** Until the
keystone lands inside G0 ≤+3%, the compounding wall is a **fundable roadmap.** The
Phase-0 gate is the milestone a seed investor funds toward.

---

## §3. Why THESE five — and what the red-team cut

The five are not the five most impressive-sounding capabilities; they are the five
that **survived** an adversarial cut. Naming the rejects is what keeps the position
falsifiable — and tells the next red-team where *not* to waste its swing.

| Candidate | Verdict | Why cut (and where it went) |
|---|---|---|
| **Per-method `[start-end]` granularity** | **CUT → folded into Moat D** | Real asset, but codegraph ships typed nodes and SCIP has symbol ranges. A quality edge, not a wall. It survives as the property that makes the provenance stamp method-precise. |
| **Project-scoped zero-cost-at-`init`** | **CUT → §D wedge, not a moat** | Differentiating vs Glean/CAST/Augment, but the watch-mode cluster is also local-first. A frictionless **wedge**, not a structural moat — and not, on its own, a growth **loop** (`STRATEGIC-POSITION.md` §D.1). |
| **Freshness as a standalone Moat-1** | **DEMOTED → scoped Moat A** | File-watch + incremental is table-stakes (2026 cluster); Sourcegraph designed SCIP for incremental. Freshness survives **only** scoped to the local per-keystroke CLI answer; near-live (not structural) for the viewer. |
| **Blast-radius as the graph-query edge** | **CONCEDED table-stakes → Moat C delivery shape** | Potpie ships it today, funded; graphify/CAST expose it. The query is not the edge; the *delivery shape* (deterministic + fresh + zero-token + thin) is. |
| **Recon as a "data network effect"** | **REFRAMED → switching-cost (Moat E)** | a16z 2019: single-team usage data is a scale effect, not a network effect — it does not compound across teams. Sold as a network effect, it dies in diligence; sold as switching-cost + system-of-record, it holds. |
| **The interactive agent-canvas as a moat** | **CONCEDED won/commoditizing** | tldraw/Excalidraw/Mermaid occupy it; "the agent that lives with your diagram" is the biggest shift of 2026. aOa **rides** the loop — its *only* defensible delta is the AST-grounded `file:line:commit` BEFORE (which is Moat B, not a sixth moat). |
| **Two-faces-one-substrate** | **CUT as a moat → §B.6 mechanism** | It is how the five compound, not itself a wall a competitor must independently breach. |
| **Heavyweight standing knowledge graph / `query_graph` retrieval** | **OUT (scope law + lost bet)** | The frontier CLI agents (Claude Code, Codex, Aider) rejected the standing graph for agentic grep; graphify's `query_graph` is a stale full-scan — a worse grep. Building it chases a bet the market already lost (`01-knowledge-graph-and-visualization.md` §2.3). |
| **Cross-modal / LLM-inferred edges, automatic pattern detection** | **OUT (scope law)** | Conflicts with the determinism thesis (which underwrites Moats C and D); 30 years of research caps architecture detection at unusable precision. aOa does declare-and-diff (conformance), never detect. |

**The shape of the cut.** The red-team removed everything a funded competitor can
**copy in a quarter** (granularity, local-first init, the query itself, the canvas)
and everything that **chases a lost or out-of-scope bet** (standing graph, LLM
edges, detection). What remained are the four properties a competitor cannot adopt
without *abandoning their own value prop* — determinism (C/D), the derived BEFORE
(B), the local-latency floor (A), and a behavioral dataset that can't be cloned from
source (E) — plus the mechanism that makes them reinforce (§B.6). **That is the
difference between a feature list and a moat.**

---

## §4. The one-paragraph statement (for the deck)

> Agents now write code faster than any human can keep the architecture coherent —
> Anthropic named the failure "agentic technical debt" and named the remedy:
> **written constraints (ADRs/CLAUDE.md/specs) for architectural direction**
> ([Dev|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/)).
> Constraints written in prose can't be machine-enforced. aOa is the enforcement
> layer: it derives the actual architecture deterministically and diffs it against
> the declared pattern — **conformance, fresh, provenance-stamped.** The durable
> walls competitors structurally lack are **provenance** (every answer cited — the
> grounding receipt the KPMG failure proved the industry needs), the **AST-derived
> diff** (an un-fakeable BEFORE no LLM-drawn canvas can produce), and a
> **deterministic, thin-MCP delivery shape** (4 grep-beaters vs the 45 the 2026
> cluster ships) — backstopped by a **local-latency freshness floor** and a **live
> recon signal** no competitor has an equivalent of. Each wall is a property a
> competitor cannot adopt without abandoning their own value prop; to neutralize
> aOa they must breach all five at once. **All of it is gated on one keystone (§0)
> — until it ships inside G0 ≤+3%, these are a fundable roadmap, and the Phase-0
> gate is the milestone that de-risks it.**

---

### Grounding

**Position synthesis:** `playbook/STRATEGIC-POSITION.md` (§B, four-lens-survived).
**Sibling deep-dives:** `01-knowledge-graph-and-visualization.md` (could-vs-should,
blind-judge), `02-integration-touchpoints.md` (the keystone + six seams),
`03-access-surface.md` (native-first, thin MCP), `04-scale-and-positioning.md` (the
language ladder + freshness/provenance frame), `05-redteam-alignment.md` (the
attack record). **Prior verdict:** `.context/details/2026-06-19-graphify-plus-mcp-research.md`.
**Scope law:** `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
(`:24-30` ladder + leash). **aOa anchors (verified against live source):**
`internal/app/app.go:350` (`bumpRevision`, NOT called by `onFileChanged`/`Reindex`);
`internal/adapters/web/server.go:34,52` (ETag/revision transport — file-change
wiring absent); `internal/adapters/web/recon.go:555,577` (click-fires-annotation
precedent — never the substrate); `cmd/aoa/cmd/` (NO `arch.go`); `internal/` (NO
overlay loader, NO `MethodArch*`, NO AG-UI); `internal/adapters/treesitter/parser.go`
(the always-on `extractSymbols` pass never visits import nodes; only the parked
recon walker's `countImportSpecs`, `walker.go:568`, traverses them).
**External (all cited inline):** graphify [#653](https://github.com/safishamsi/graphify/issues/653)/[#341](https://github.com/safishamsi/graphify/issues/341);
[Potpie $2.2M](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html)/[TFN](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/);
[CodeGraph 45 tools](https://github.com/codegraph-ai/CodeGraph);
[Sourcegraph SCIP](https://sourcegraph.com/blog/announcing-scip)/[scip-clang](https://sourcegraph.com/blog/announcing-scip-clang);
[tldraw JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw)/[mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw)/[Mermaid living-contract](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development);
[KPMG pull](https://nerova.ai/troubleshooting-fixes/kpmg-pulls-agentic-ai-report-hallucinations-june-13-2026)/[citation-hallucination](https://arxiv.org/html/2605.06635v1)/[tool receipts](https://arxiv.org/html/2603.10060v1);
[a16z data-moats 2019](https://a16z.com/the-empty-promise-of-data-moats/);
[Anthropic agentic-tech-debt remedy](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/).

**Honesty flags:** graphify "YC-S26" self-applied/unverified; Potpie revenue figures
directional; conversion/ARR/CAGR figures agency-blog-sourced (directional); a16z
data-moat essay is 2019 (framing evergreen, not 2026); CAST $10.29K/app/yr verified,
the $200K-$800K DD figure is a services estimate, NOT a product price; "deterministic"
≠ SLSA-reproducible (in-toto/auditability only); the keystone, `aoa arch` surface,
overlay loader, arch-shard endpoint, file-save→ETag tick, and AG-UI are **NOT yet
built (§0)**; the recon→graph integration is real-as-asset but **under-integrated**;
the recon/live loops require a running daemon + Claude Code session logs (Claude Code
beachhead).
