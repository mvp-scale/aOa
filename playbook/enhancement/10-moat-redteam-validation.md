# 10 — The Four-Lens Red-Team Validation Record (Moats + Interactive Diagram)

**Status:** the trust appendix for the *strategic* position. This document
proposes nothing new. It exists to answer one question — *why should we believe
the five moats and the interactive before/after diagram in
`STRATEGIC-POSITION.md` are real, fundable, leading-edge, and growth-driving,
and not shiny-object chasing?* — and it answers it the only way this playbook
accepts: by recording every adversarial attack the position survived across two
rounds and four lenses, and by turning the surviving claims into a standing,
falsifiable checklist of **what was cut and why**.

**This is the sibling of `05-redteam-alignment.md`, not a duplicate.** Doc 05 red-teams the
**engineering keystone** (the import-edge build inside the always-on parse pass, G0/G2/G4,
the `bumpRevision`/locking-law anchors). This doc red-teams the **market position** — the
five moats, the diff loop, the growth motion, the fundability — against the four commercial
lenses. 05 asks *"can it be built inside the laws?"* This asks *"is it worth building, can it
be defended, and will anyone adopt or fund it?"* They share one fact base and one keystone.

**Read order:** last door, not first. The argument lives in
`playbook/STRATEGIC-POSITION.md`; the moat detail lives in §A–§F there and in the
`enhancement/01`–`04` pool; the binding laws are `.context/GOALS.md`,
`.context/decisions/2026-06-11-core-competence-and-scope-line.md`,
`playbook/standards/view-standards.json`, and `playbook/standards/MODEL-STANDARD.md`;
the engineering red-team is `05-redteam-alignment.md`. This file curates the two
rounds × four lenses that hardened the position and turns the verdicts into a
standing checklist.

**The document's own contract** (inherited from the position): every load-bearing
claim cites a `file:line`, a URL, or a scan. *If a cited anchor is wrong, the
claim built on it is void.* That rule is why the lenses below kept firing on
attribution precision (the "Anthropic named it" blog) and on present-tense-vs-
future-tense (the unbuilt `aoa arch` surface) even after the strategy was
accepted — in a falsifiable position, a claim dressed as shipped-when-it-is-not is
a defect of the same class as a wrong strategy.

---

## 1. The headline: what survived, and what the four lenses actually killed

Across two rounds, **the strategic spine held and the marketing froth was burned off**.
The spine is small and it never moved:

- **Hybrid, not graph-as-retrieval.** Structural graph query *subordinate to* fresh
  `grep→peek`, never a replacement index — the bet the frontier CLI agents proved wins.
  No lens attacked this; the leading-edge lens *confirmed* it (`STRATEGIC-POSITION.md:48`,
  `:120`).
- **The AST-derived BEFORE is the un-fakeable core.** Every lens conceded the canvas is
  commoditized but agreed the *grounded* intersection (derived BEFORE, `file:line:commit` on
  every node, blind-judge gate) is empty (`:104`, `:186`).
- **Provenance is the cleanest gap.** Not one competitor in any scan stamps
  `file:line:commit` on every *answer* (`:140`). Contested only on *durability*, never on
  *existence*.
- **The leash holds by construction.** Claude runs *outside* the service; the agent writes a
  proposed-edge overlay file a deterministic renderer consumes; no LLM call on any derive
  path → G0 intact. The feasibility lens verified this against the live
  `recon-investigate` precedent and graded §C **SOLID** (`recon.go:555,577`; `:222-224`).

**What the four lenses actually killed (and the position now carries the fix):** five
co-equal "structural walls" → two adoption headliners + three diligence deepeners; a false
"real-time on save" claim → "one currently-absent line"; a "data network effect" → a
switching-cost effect; an invented-novelty canvas → a conceded, commoditized 2026 pattern;
a present-tense `aoa arch` demo surface → a future-tensed, §0-gated roadmap; and a
brand-attributed why-now → a demand-attributed one. **None of these sank the direction. All
of them sank a specific over-claim** — which is the failure mode this record exists to catch.

**The one finding that reframes everything (and is honestly carried):** every wall a VC
would underwrite is **gated on a keystone that ships zero load-bearing code today**
(`STRATEGIC-POSITION.md:18-35`). The position no longer hides this as a footnote — §0 leads
with it, and the §E Phase-0 gate *is* the de-risking milestone. The honest verdict is: this
is a **team/execution bet with a sequenced de-risking milestone**, fundable at pre-seed on
conviction (the Kosli/Heavybit continuous-compliance seed — $3.1M led by Heavybit, Nov 2022 — is a comp for VCs underwriting this *before*
the product exists), **not** an already-built moat. The position says so plainly.

---

## 2. The round-by-round, lens-by-lens record

Two rounds, four lenses. Round 1 returned **NEEDS_WORK** on all four. Round 2 returned
**NEEDS_WORK** on three and **SOLID** on feasibility. Each finding is recorded with its
lens, severity, the verification that fired it, and how the final position answers it.

### Round 1 — the four opening attacks

#### Lens: GROWTH-HACKER — *"defensible, but why does anyone adopt it next Tuesday?"*

> **Verdict:** strong on defensibility, weak on adoption. Every viral loop is gated on
> unbuilt code; `aoa init` is a wedge sold as a loop; three of five moats are
> engineer's-pride no user feels or shares.

| # | Sev | Finding | How the final position answers it |
|---|-----|---------|-----------------------------------|
| G1.1 | **blocker** | The whole growth engine is gated on unbuilt code, but the doc sold the loops as live. | **§0 CURRENT-STATE BANNER** added; **D.0 pre-keystone loop** (grep→peek CLI, zero new code, ships today) leads §D; each artifact in the **D.2 table** marked with its earliest-ship gate (`:250-257`). |
| G1.2 | major | "Free at `aoa init`" asserted as a *loop* but only substantiated as a *wedge* — nothing recruits user N+1. | **D.1 downgrades `aoa init` to a wedge** and designs the real **recipient-must-init loop** (the canvas fires Claude *locally*, so a teammate must init to interact) with one instrumented metric `init→second-session→teammate-init` (`:244-248`). |
| G1.3 | major | Three of five moats (provenance-by-layer, thin-MCP-tokens, recon) are not felt or shared by users, yet ranked as the spine. | **Moats re-split on two axes** (`:72-74`): adoption headliners A+B lead the growth story; C/D/E reserved for diligence — never sold as adoption drivers. |
| G1.4 | major | The genuinely viral artifact (the diff) is the *most contingent* thing and the plan doesn't price that into timing. | **D.2 + Moat-B seam** state the diff is Phase-2 and most-contingent; **§E sequences growth-spend** so no launch front-loads on it (`:112`, `:256`). |
| G1.5 | minor | "Zero infra" overclaims against the real tree-sitter/CGo/509-grammar daemon build, coupled to Claude Code logs. | **D.1 states host-coupling honestly** — daemon + `~/.claude` logs = Claude Code **beachhead, not unbounded TAM** (`:246`). |
| G1.6 | minor | The OpenClaw star-velocity comp validates a loop while the doc elsewhere discredits stars as vanity — internally inconsistent. | **OpenClaw comp dropped**; loop proof is the recipient-must-init mechanic, not a star spike (`:259`). |

#### Lens: FOUNDER-VC — *"this loses a diligence call on the exact claim it leads with."*

> **Verdict:** investable as a verifiability/freshness/governance play with honest comps,
> but two of the five "structural" moats are copyable features dressed as walls.

| # | Sev | Finding | How the final position answers it |
|---|-----|---------|-----------------------------------|
| F1.1 | **blocker** | Moat 1 (freshness) overstates the wall — Sourcegraph SCIP already ships incremental-on-push; the gap is cadence (a tunable parameter), not a data-plane inversion. | **Freshness reclassified** (`:80`, `:84-96`): structural only for the **local per-keystroke CLI/socket answer**; strong-but-narrowing for the viewer. Cites SCIP "on roadmap." |
| F1.2 | **blocker** | Moat 3 (recon) uses the exact "data network effect" framing 2026 VCs discount on sight (a16z 2019: single-team data is a *scale*, not *network*, effect). | **Recon de-mooted** (`:150-152`): "data network effect" language **dropped entirely**; reframed as a **switching-cost / system-of-record** moat, the combination a16z says *does* defend. |
| F1.3 | major | The why-now (agentic tech debt) is real but the demand→product *arrow* is asserted — Anthropic's named remedy is *written constraints*, and fresh facts = recall, the part that already works. | **D.3 earns the arrow** (`:261-267`): the product the demand pulls toward is **conformance** (declared-pattern-vs-derived-actual diff) — the *machine-checkable enforcement* those prose constraints need. |
| F1.4 | major | Potpie (closest-funded, $2.2M) already ships blast-radius + system-design — cuts against Moat 5's novelty and the "Potpie sharpens our thesis" framing. | **Blast-radius conceded table-stakes**; Moat moved onto **delivery shape** (deterministic + fresh + zero-token + provenance). Potpie reframed as **budget-validated-and-contested**, not me-too (`:54`, `:122`, `:287`). |
| F1.5 | minor | The $200K–$800K M&A-DD figure is load-bearing but uncorroborated (services-grade, not a product price). | **Money anchored on the verified $10.29K/app/yr** CAST product price; the DD figure flagged **vendor/estimate-grade, illustrative ceiling only** (`:281`, `:318`). |
| F1.6 | minor | The "to neutralize aOa a competitor must do all five" wall is also aOa's own un-built obligation, stated one-sided. | **§B.6 + §0 add the symmetric concession** (`:35`, `:178`): the compounding wall is aOa's build obligation too; the Phase-0 gate is the funded milestone. |

#### Lens: FEASIBILITY-ENGINEER — *"the vision outran the architecture in one decisive place."*

> **Verdict:** strategy sound and self-honest (leash holds, Mode A/B/C correctly ranked, G0
> preserved), **but one false load-bearing fact is stated twice as a decisive build fact.**

| # | Sev | Finding | Verified by | How the final position answers it |
|---|-----|---------|-------------|-----------------------------------|
| E1.1 | **blocker** | "The viewer's ETag invalidates on the same tick the substrate changes" is **false**. | `bumpRevision()` (`app.go:350`) has exactly four callers — `searchObserver` (`:564`), `onSessionEvent` (`:901`), `SetFileInvestigated` (`:2896`), `ClearInvestigated` (`:2905`) — **none is `onFileChanged` (`watcher.go:20`) or `Reindex` (`app.go:2816`)**. A code edit reindexes symbols but does **not** bump the ETag. | **Corrected to "one currently-absent line"** (`:32`, `:92`, `:190`): the 304 *transport* ships; the file-change *trigger* does not. "Real-time on save" requires `onFileChanged`/`Reindex` to call `bumpRevision()`. |
| E1.2 | major | The "overlay loader (rejects invented ids)" that Mode A depends on **does not exist** — `grep -rin overlay` returns one unrelated comment. | `internal/`, `cmd/` — no overlay loader. | Reclassified **net-new, not "reused"** (`:30`, `:196`, `:200`); still small/in-bounds/leash-clean, but future-tensed. |
| E1.3 | major | Every Moat demo and the §C loop invoke an `aoa arch` surface (`MethodArchFacts/Reach/Blast`) that **does not exist**. | No `cmd/aoa/cmd/arch.go`; no `MethodArch*` socket methods. | **§0 banner + per-section banner** (`:28`, `:184`); all §B/§C demos future-tensed. |
| E1.4 | major | The "serve the arch shard through the existing `withETag` handler" implies a live endpoint; the viewer is a **build-time Python mockup generator**, and "recompute-on-compact" names a mechanism that doesn't exist. | `playbook/generators/build_blueprint_viewer.py`. | **C.1 distinguishes** the build-time generator from the net-new daemon-served endpoint; **"recompute-on-compact" dropped** as a non-existent mechanism (`:190`). |
| E1.5 | minor | "AG-UI streaming ships today" is false — AG-UI appears nowhere in the tree. | grep: AG-UI absent. | **C.3 corrected** (`:220`): dashboard ships; AG-UI emission is a **net-new adapter**, the intended-not-present next step. |
| E1.6 | minor | The "28 languages" figure in the scope-law ADR collides with the "3 tuned extractors" reality. | ADR `2026-06-11-...:12`; `parser.go:235` (3 extractors). | **§A inline caveat + §E Phase-1 gate** reconcile to the ladder (3 tuned / 10 walker / 509 registered / forest) before any external language claim (`:49`, `:295`). |

#### Lens: LEADING-EDGE — *"frontier-aware, but locally trailing in three places."*

> **Verdict:** mostly current and honest, but under-reads how fast 2026 moved — and sells two
> table-stakes as moats.

| # | Sev | Finding | How the final position answers it |
|---|-----|---------|-----------------------------------|
| L1.1 | major | §C sold the interactive canvas as a near-empty intersection; the interaction *loop* is the most-discussed 2026 devtools pattern (tldraw Agents-on-Canvas, Excalidraw+MCP, Mermaid-living-contract). | **Canvas conceded a won, commoditizing pattern** (`:11`, `:104`, `:186`): aOa's *only* defensible delta is the AST-derived, `file:line:commit`-grounded BEFORE. SDK cluster added as a §A row; "intersection is empty" → "the **grounded** intersection is empty." |
| L1.2 | major | Moat 1 (freshness) ranked first/hardest-to-copy, but a same-week GitHub-Trending watch-mode cluster ships tree-sitter + file-watch + incremental + MCP — freshness is the *fastest-commoditizing* axis. | **Freshness demoted** to Moat A scoped + "the property that keeps the OTHER moats in sync" (`:80`, `:88`); watch-mode cluster cited as the 2026 floor (`:55`). |
| L1.3 | minor | Moat 5's "deterministic edges / EXTRACTED-vs-INFERRED" is now a standard 2026 design pattern (graphify ships confidence tags); the surviving claim is thin-MCP-by-scope-law + the token receipt. | **Moat C re-led with the 4-vs-45 tool wall + the token receipt** (`:116-124`); deterministic-edges-as-novel cut in-body to match the seam. |
| L1.4 | minor | "AG-UI ships today" overstates; the surface (dashboard) exists, emission does not. | **C.3 scoped** to "the correct, adopted 2026 standards to target" — emission is the named-not-present next step (`:220`). |
| L1.5 | minor | Best-practice gap: the dominant 2026 *agent-diffable* pattern is the plain-text **living-contract** (Mermaid in git, PR-diffable, in-context) — the bespoke viewer is neither. | **New C.6 living-contract rendition** (`:230-232`): aOa emits a deterministic Mermaid/plain-text rendition of any shard — turns a competitor best-practice into a free third face. |
| L1.6 | minor | "Fails silently everywhere" overstates against watch-mode competitors who now ship drift-fingerprinting. | **Scoped to the build-artifact tier**; the footnote concedes the cluster detects drift, and re-grounds aOa's edge as "never builds a standing artifact to drift against" (`:68`, `:320-321`). |

### Round 2 — the second pass after the revision

Three lenses returned **NEEDS_WORK** again (sharper, deeper attacks on the *resolved* draft);
feasibility returned **SOLID**. The remaining findings are the open seams §F now names.

#### Lens: GROWTH-HACKER (R2) — *"no real loop exists until your riskiest milestone ships."*

| # | Sev | Finding | How the final position answers it |
|---|-----|---------|-----------------------------------|
| G2.1 | **blocker** | No viral/PLG loop exists until Phase 2; the one designed loop is triple-gated (Claude Code host + persistent daemon + unbuilt canvas). The doc treats this as a sequencing footnote, not the central gap. | **D.0/D.1 name the pre-keystone loop** (the CLAUDE.md routes-the-team mechanic this very repo uses — one dev commits `aoa grep`→`peek` guidance, teammates inherit it on pull) and state plainly that until Phase 2 **growth is wedge-driven (push), not loop-driven (pull)** (`:240-248`). *Carried as §F.1.* |
| G2.2 | major | The Phase-0 shareable (static DSM PNG of an OSS repo) enters the GitDiagram-flooded viral lane stripped of every differentiator a thumbnail can't show. | **D.2 marks it Weak-medium** and demands its own share-worthiness proof; the recommendation is to carry a **falsifiable finding** (a named cycle / god-node / "this PR adds a cycle") — a claim that invites argument, not a picture that invites a scroll-past (`:242`, `:254`). |
| G2.3 | major | The one loop is triple-gated and only one gate is counted; conversion is the *product* of all three. | **D.1 states all three gates**; host-coupling + daemon friction + Phase-2 contingency named; addressable fraction flagged unproven (`:246-248`). *Carried as §F.7.* |
| G2.4 | major | The felt-at-first-touch value (generic grep CLI) is commoditized; the shareable moat (recon-weighted, live) sits behind daemon + host + Phase 2 — wedge and moat are disjoint in the early funnel. | **Acknowledged**: provenance promoted from "diligence spine" to **the felt per-answer retention reason** — the one defensible thing a user touches every query (D.2 minor fix; Moat D `:134-138`). |
| G2.5 | major | Conformance-as-the-product (the fundable core) is a seller-push motion; the free-CLI-user → governance-buyer arrow is asserted, not earned. | **SP §F red-team item 4 names the tension explicitly** (`:307`): the company is a **PLG-dev-tool-into-compliance-expansion** play, distribution ≠ revenue; the bridge is land-and-expand. (Note: 'Snyk shape'/'land-and-expand' are this doc's synthesis labels, not strings in SP.) |

#### Lens: FOUNDER-VC (R2) — *"the moat IS the execution — say so plainly."*

| # | Sev | Finding | How the final position answers it |
|---|-----|---------|-----------------------------------|
| F2.1 | **blocker** | The five-fold compounding wall is circular: if the moat only exists once all five are built and none are built, there is no moat today — there is a build plan. | **§0 + §B.6 reframe the defensibility as a TEAM/EXECUTION bet** with a sequenced de-risking milestone (the keystone is the one irreversible bet; the others are renditions). The investable unit is *keystone-lands-in-budget*, not the assembled wall (`:35`, `:178`). (The Kosli/Heavybit continuous-compliance comp is this doc's addition — it does not appear in SP.) |
| F2.2 | **blocker** | The two moats still labeled "structural" (provenance, thin-MCP) are the two an SCIP/Blackbird incumbent neutralizes fastest. | **The data-plane-shape asymmetry is the spine** (SP Moat-A freshness, `:84-90`): an incumbent (Sourcegraph/GitHub/CAST) is cloud-and-pipeline-shaped and **cannot go local-per-keystroke-fresh without cannibalizing their per-seat/per-app business**. Provenance/thin-MCP are necessary-but-copyable (SP red-team item 2, `:305`). (The 'Christensen'/'data-plane-shape' labels are this doc's framing, not SP terms.) |
| F2.3 | major | No concrete path from the free CLI wedge to the $10K+ governance check; "governance converts the org" is asserted. | **D.4 + §F.4 label the bridge a HYPOTHESIS** anchored to the continuous-compliance tailwind ("auditors expect near-real-time evidence"); the upward artifact is the evidence pack (Phase 3); a commercial leading indicator named alongside the adoption metric (`:269-281`, `:307`). |
| F2.4 | major | Why-now conflates two clocks: "the demand is named" is a tailwind, not a *closing window* for aOa specifically. | **D.3 frames the founder why-now** — the deterministic-fresh-local shape is still unoccupied; Potpie funded on the wrong-for-CLI Neo4j shape, the watch-mode cluster lacking provenance/recon/judge-gate (`:261-265`). (The 'window closes/contested NOW' phrasing is this doc's, not SP's.) |
| F2.5 | minor | The "agent tool vs governance tool" identity is answered inconsistently across sections. | **SP §F red-team item 4 (`:307`) frames the sequence**: agent-tool wedge (PLG, Claude Code beachhead) = distribution; conformance/governance = revenue. (The 'Snyk comp set' label is this doc's addition, not in SP.) |
| F2.6 | minor | Even the softened recon (switching-cost) overstates — it is unbuilt into the graph and host-coupled. | **Moat E conceded** under-integrated; it "cannot drive adoption now"; demoted to a retention/diligence moat ("not a wedge," "cannot drive adoption now") until fused into overlays *and* the host base broadens (`:160-164`). |

#### Lens: FEASIBILITY-ENGINEER (R2) — **SOLID**

> **Verdict:** under a hard feasibility attack the draft holds. Every load-bearing source claim
> independently verified: `bumpRevision()` has exactly four callers and neither `onFileChanged`
> nor `Reindex` is among them; `aoa arch`/`MethodArch*`/overlay-loader/AG-UI confirmed absent;
> the recon-investigate leash precedent is real; the `withETag` 304 transport exists; the
> keystone claim is precisely right (the parse pass only *counts* imports — `walker.go:568`
> `countImportSpecs` returns an `int`; the `Symbol` struct `parser.go:20` has no edge field, so
> edges are net-new). **NEEDS_WORK is unwarranted; the findings are sharpening, not blockers.**

| # | Sev | Finding | How the final position answers it |
|---|-----|---------|-----------------------------------|
| E2.1 | minor | The G0 ≤+3% keystone budget is asserted everywhere but never measured against the real parse path (emitting edges is more than `countImportSpecs`' int increment). | **Carried as an open measurement** — the first Phase-0 exit criterion is to benchmark edge emission on a large repo *before* committing the moat sequencing; fallback = edges move off the always-on pass to opt-in/async derive. *(Reconcile into §E + §F.)* |
| E2.2 | minor | The "click fires Claude" loop is the one hand-wavy step — the service must **never** spawn an agent; the recon precedent is a self-contained annotation write, not an outbound invocation. | **C.3/C.4 split the halves**: the grounded fact-pack endpoint is leash-clean and recon-shaped; the *fact-pack → Claude* hop is done by the **host** (MCP tool result / paste), never the service. The service never initiates an LLM call — that is what preserves G0 (`:213-224`). |
| E2.3 | minor | Mode B (branch re-derive) is sold as the un-fakeable upgrade but its cost (a second full `BuildIndex` pass, two live indexes) is unexamined. | Mode B re-derive is an explicit **async/offline** operation, never on the interactive click path; two-index memory flagged as an open question (`:197`). |
| E2.4 | minor | Blast-radius = git-changed ∩ closure smuggles a git dependency into a derive path; the ports/adapters placement is unstated. | Graph algorithms (reachability/Tarjan/DSM/fan-in-out) are **pure domain** (`internal/domain/arch`, no imports); git-changed comes through a **port**; the overlay loader is an adapter producing a domain-validated struct (aligns with 05 §3.3). |
| E2.5 | minor | Moat E's recon-weighted overlays risk making the most-differentiated graph feature depend on the recon/host layer, violating the G2 firewall. | **G2 firewall stated**: the arch graph + all four queries produce complete REAL-stamped results with **zero recon data**; recon weighting is a strictly-additive optional overlay that drops the view to MIXED, never required for a base answer (`:160-164`). |

#### Lens: LEADING-EDGE (R2) — *"frontier-aware, but a prominent map gap and a fragile attribution."*

| # | Sev | Finding | How the final position answers it |
|---|-----|---------|-----------------------------------|
| L2.1 | major | **Material omission** — Google Code Wiki (Nov 2025) absent; DeepWiki/Cognition demoted to a throwaway row. These are the best-funded occupants of aOa's *exact* lane (interactive, source-linked, "continuously-updated" code-grounded diagrams + MCP). | **Conceded as a map gap, not a strategy hole** (both are LLM-generated, so the deterministic-`file:line:commit` + blind-judge differentiation survives). The precise correction: a **source hyperlink is not a re-derivable `file:line:commit` stamp**; "continuously updated" = LLM regeneration on a schedule, not deterministic re-derivation off the working tree; neither passes a falsifiable blind-judge gate. Their adoption **proves the demand** (a tailwind for D.3), reframing the threat as validation. *(Add as a named §A row.)* |
| L2.2 | major | **Fragile attribution** — "Anthropic named the failure mode — agentic technical debt" rests on a single secondary blog (earezki.com); no official Anthropic publication using the term could be verified. | **Downgraded** to "a 2026 practitioner write-up attributes the framing to Claude Code practice"; lean on the **demand** (drift at machine speed; memory=recall vs constraints=direction), which is independently corroborated and survives without the brand attribution. *(Added to §F honesty flags.)* |
| L2.3 | minor | Stale frontier claim — "every plan surface is Markdown prose" is false as of Cursor 2.2 (inline Mermaid plan diagrams with file refs). | Corrected to "plan surfaces are **converging on LLM-DRAWN diagrams** (Cursor 2.2) — but they render the agent's *guess*, not an AST-derived BEFORE." This **sharpens** Moat B: the ungrounded surface is no longer empty; the **grounded** one still is. |
| L2.4 | minor | Unverified specific-fact — tldraw's `BlurryShape`/`FocusedShape` primitive names asserted with confidence; the talk is unpublished. | Soften to "every shipping agent-canvas grounds on the LLM's reading of shape geometry + labels, not source AST," without asserting the primitive names. *(Flagged unverified in §F until a primary tldraw source.)* |
| L2.5 | minor | Best-practice framing gap — "deterministic + provenance" should connect to the emerging **tool-receipts** standard, not generic "compliance moat." | **Elevate `aoa arch facts` as a deterministic implementation of the verifiable-receipt pattern** the agentic-trust literature is converging on (arxiv 2603.10060), with KPMG citation-failure as the demand-side proof — puts Moat D *on* the named frontier (`:140`). |
| L2.6 | minor | "No standing artifact" over-applied to the watch-mode cluster, which also maintains live state via incremental reindex. | Tighten to the part that is actually structural: aOa's answer comes from the **same in-memory O(1) index the live grep rides** (one substrate, zero JSON-RPC round-trip), where the cluster maintains a **separate** incremental graph store queried over MCP (`:90`, `:321`). |

---

## 3. The "we are not chasing shiny objects" evidence

The clearest proof a position is disciplined is the list of attractive things it **refused**.
Each row is a shiny object the four lenses tempted the position toward — and the binding reason
it was declined.

| Shiny object | Why it's tempting | Why it was declined | Source |
|---|---|---|---|
| **Mode C — autonomous worktree rewrite** ("click → Claude rewrites your code") | The viral demo; the "agent that lives with your diagram" taken to its limit | Breaks the leash if rushed; the user explicitly warned against it. aOa stays on the **SUGGEST** side — the agent proposes, the human disposes. Gated behind a battle-tested boundary. | `STRATEGIC-POSITION.md:198`, `:228`, §F.10 (`:313`) |
| **The interactive canvas as an invented moat** | "the biggest shift of 2026"; demos beautifully | Conceded a **won, commoditizing pattern** (tldraw/Excalidraw/Mermaid). The only delta is AST-grounding — claiming invention would have lost the leading-edge call. | `:104`, `:186`, §F.6 (`:309`) |
| **Heavyweight semantic graph as primary retrieval** | Potpie raised $2.2M on it; graphify has ~69K stars | The bet the **frontier CLI agents declined** (Claude Code removed vector search May 2025). Tier-1 graph stays narrow (imports + derived reachability/DSM/cycles/blast), subordinate to grep. | `:48`, 05 OUT-list |
| **"Data network effect" framing for recon** | The slide VCs *used* to fund | a16z 2019: single-team data is a *scale* effect, not a *network* effect. Kept the **switching-cost** framing that actually survives diligence. | `:150-152` |
| **Freshness as the lead structural moat** | The most demoable ("ours moved, theirs didn't") | File-watch is **table-stakes** in 2026 (the watch-mode cluster). Demoted to scoped Moat A; structural only for the local per-keystroke answer. | `:80`, §F.3 (`:306`) |
| **AG-UI streaming "ships today"** | Aligns with the adopted 2026 standard | It is **absent from the tree**. Scoped to "the correct standard to target," net-new, gated by unverified rendering fidelity. | `:220`, §F.9 (`:312`) |
| **The $200K–$800K M&A-DD headline price** | Makes the money path look large | Services-engagement estimate, vendor-grade — **not** a product price. Anchored on the verified **$10.29K/app/yr** instead. | `:281`, `:318` |
| **"Competitors have no agent surface"** | A clean prior fact | **Stale and false** — CAST (Aug 2025) and Sonargraph (June 2026) ship MCP. Retired; durable truth is *they bolted MCP onto a stale unstamped batch artifact*. | `:66` |
| **"509 languages of architect-grade analysis"** | Impressive breadth number | 509 is the **registration** count; only **3 tuned extractors** (Go/Python/JS-TS-TSX) have REAL import-edge parity. Reconciled to the ladder. | `:49`, 05 OUT-list |
| **MCP fronting a latency-sensitive query** | "MCP has won the interface" | MCP structurally cannot beat the socket it wraps — buys **reach, never speed**. Thin (4 tools), late, never fronts a hot path. | 05 §3.2 |

---

## 4. The standing checklist — each moat × five tests

The red-team's demand, distilled. Each of the five moats is scored against five tests:
**defensible** (survives "why can't an incumbent copy this?"), **feasible on the real stack**
(buildable inside G0/G2/G4 against verified source), **leading-edge** (near the frontier, not
trailing), **growth-driving** (felt/shared by users or it pulls adoption), and
**founder-fundable** (a VC underwrites it). The honest scores are mixed by design — a row of
all-greens would itself be a red flag.

Legend: ✅ holds · ◑ holds with the stated scope/caveat · ⏳ gated on the keystone (roadmap) · ➖ explicitly not this moat's job.

| Moat | Defensible? | Feasible on real stack? | Leading-edge? | Growth-driving? | Founder-fundable? |
|---|---|---|---|---|---|
| **A — Freshness** (scoped) | ◑ structural only for the **local per-keystroke CLI** answer; narrowing for the viewer (SCIP incremental, watch-mode cluster) | ⏳ rides the always-on index; **requires the absent `bumpRevision()` line** for the live viewer (`watcher.go`/`app.go`) | ◑ file-watch is now table-stakes — the edge is "never builds a standing artifact to drift against" | ✅ the demoable "ours moved, theirs didn't" — but felt only as the *absence* of a rare failure (must be **shown**, not relied on to spread) | ◑ defensible-but-narrowing; the fundable part is the data-plane-shape asymmetry (incumbents can't go local without cannibalizing) |
| **B — AST-derived diff** | ✅ un-fakeable: the BEFORE is *derived*, not drawn; blind-judge gate is falsifiable; the **grounded** intersection is empty | ⏳ Phase-2; depends on keystone (≤+3%) **and** the net-new overlay loader — **the most-contingent thing in the position** | ✅ on the frontier (Cursor 2.2 renders LLM-*guessed* plans; aOa renders the derived BEFORE) | ✅ **the only natively viral artifact** — the decision teammates argue over; powers the recipient-must-init loop | ✅ the AST-derived conformance/diff is the fundable core (D.3) |
| **C — Deterministic thin-MCP** | ◑ deterministic edges + blast-radius are **table-stakes** (Potpie, the cluster); survives only on **thin-MCP-by-scope-law (4 vs 45 tools) + the token receipt** | ✅ 4 grep-beaters ride keystone edges; sub-ms socket switch (verified `socket/server.go`) | ✅ the token receipt (~40% context → tool metadata) is current and sharp; `aoa arch facts` is a tool-receipt implementation | ➖ no developer adopts for thin MCP — diligence/retention, not adoption | ✅ "agent infrastructure, no per-query inference cost" — but compresses to A+D if a competitor copies the shape |
| **D — Provenance on every answer** | ◑ the **cleanest existence gap** (no competitor stamps `file:line:commit` on every *answer*); an SCIP/Blackbird incumbent **can copy it in a quarter** — attack durability, not existence | ✅ one fact rendered two ways; layer-split is verified against the scope-law ladder | ✅ the verifiable-receipt / tool-receipts frontier; KPMG citation-failure is the dated demand proof | ◑ promoted to **the felt per-answer retention reason** — the one defensible thing touched every query | ✅ the compliance/verifiability moat "closing rounds in weeks" (the lead pitch) |
| **E — Recon switching-cost** | ◑ **not** a data network effect (a16z 2019) — a switching-cost/system-of-record moat; today **option value** until fused into overlays | ⏳ exists in learner/autotune/status-line; **not yet wired to the arch graph**; G2 firewall = base graph works with recon absent | ✅ no competitor observes what humans+agents actually touch (vFunction observes *runtime*, a different signal) | ➖ under-integrated — **cannot drive adoption now**; retention/diligence only; host-coupled to Claude Code logs | ◑ differentiated future capability; fundable as embedding, not as a proprietary-data slide |

**How to read this table:** the only moat that scores well across **all five** is **B (the
AST-derived diff)** — and it is also the most contingent (⏳ Phase 2). The moats that are
**feasible/defensible today** (D provenance, the grep CLI behind A) are the ones users **don't
share**; the moat that is **shareable** (B) is the one not yet built. That disjunction — *nothing
felt-and-shareable is also shipped-and-uncontested* — is the position's true growth risk, and it
is stated openly (§F.1, the growth R2 verdict) rather than papered over. The honest synthesis: a
**team/execution bet** whose investable unit is the Phase-0 keystone landing inside G0 ≤+3%.

---

## 5. The open seams the next red-team should swing at first

The position is not claimed closed. These are the live attack surfaces, each already named in
`STRATEGIC-POSITION.md:302-313` (§F) — listed here so the next reviewer starts at the weakest wall.

1. **The whole wedge is keystone-gated (§0).** Every freshness/diff/PLG-loop claim is roadmap
   until import edges ship within G0 ≤+3% on the **3 tuned extractors** — and that ≤+3% budget is
   **asserted, not yet measured** (feasibility R2). *First Phase-0 exit criterion: benchmark it.*
2. **No growth loop until Phase 2, and that loop is triple-gated** (Claude Code host + persistent
   daemon + the unbuilt canvas). The addressable fraction is the *product* of three gates and is
   unproven. Pre-Phase-2 is **push (wedge), not pull (loop)**.
3. **Provenance (Moat D) and thin-MCP (Moat C) are copyable in a quarter** by an SCIP/Blackbird
   incumbent. The durable spine is the **data-plane-shape asymmetry**, not these two.
4. **Identity tension:** PLG dev-tool (distribution) vs governance/compliance (revenue). The
   free-CLI-user → governance-buyer bridge is a **hypothesis to validate in Phase 3**, not a proven
   path. Commit: the Snyk shape.
5. **The "Anthropic named it" attribution is single-blog-sourced** — lean on the corroborated
   demand, not the brand.
6. **Map gap:** Google Code Wiki + DeepWiki/Cognition are the closest-adjacency funded threats;
   add them as named §A rows. (They survive aOa's differentiation — both are LLM-generated — but
   omitting them reads as not-knowing-the-frontier.)
7. **The interactive canvas is a won pattern; the only delta is AST-grounding.** If a competitor
   wires their canvas to a real index, the edge thins to "deterministic provenance-stamped BEFORE +
   blind-judge gate."

---

## 6. Why we trust this — the one-paragraph landing

The position was attacked twice, by four lenses each, and the **spine never moved**: the graph
stays subordinate to fresh `grep→peek`; the un-fakeable core is the **AST-derived BEFORE** no
LLM-drawn canvas can produce; provenance is the cleanest existence gap and the felt per-answer
retention reason; the leash holds by construction (verified **SOLID** by the feasibility lens
against the live `recon-investigate` precedent). What the lenses **did** cut was over-claim, every
time — five co-equal walls collapsed to two adoption headliners plus three diligence deepeners; a
false "real-time on save" became "one currently-absent line" (`bumpRevision` verified to have four
callers, none on the file-change path); a "data network effect" became a switching-cost effect; an
invented canvas became a conceded 2026 pattern; a present-tense `aoa arch` surface became a
future-tensed, §0-gated roadmap; and the five-fold compounding wall was reframed, symmetrically, as
**aOa's own build obligation** — a team/execution bet whose investable unit is the Phase-0 keystone
landing inside G0 ≤+3%. The position is trustworthy not because no one attacked it, but because the
attacks are recorded here, the surviving claims each cite a `file:line` or URL you can check, the
shiny objects it refused are listed by name, and its single largest risk — *nothing
felt-and-shareable is also shipped-and-uncontested until Phase 2* — is stated in its own words
rather than discovered in a diligence call.

---

### Appendix — the anchor index for this record (red-team this list first)

| Claim in this record | Anchor |
|---|---|
| The five-moat split + the two-axis reframe | `STRATEGIC-POSITION.md:72-178` |
| `bumpRevision()` has four callers, none on the file-change path → "one absent line" | `app.go:350` (def), `:564/:901/:2896/:2905` (callers); `watcher.go:20` (`onFileChanged`, no bump); `app.go:2816` (`Reindex`, no bump) |
| The `withETag` 304 transport ships; the trigger does not | `internal/adapters/web/server.go:156-170` |
| The leash precedent is real (click → annotation, never substrate) | `internal/adapters/web/recon.go:555,577` |
| Keystone edges are net-new (parse pass only *counts* imports) | `walker.go:568 countImportSpecs` (returns `int`); `parser.go:20` (`Symbol`, no edge field); `parser.go:235` (3 extractors) |
| `aoa arch` / `MethodArch*` / overlay loader / AG-UI absent | `cmd/aoa/cmd/` (no `arch.go`); `internal/` (no overlay loader, no `MethodArch*`, no AG-UI) |
| Build-time generator ≠ live endpoint | `playbook/generators/build_blueprint_viewer.py` |
| Blind-judge gate (falsifiable) | `playbook/standards/MODEL-STANDARD.md:43-53`; `view-standards.json` |
| Scope-law ladder + leash ("NEVER add a node") + stale "28" | `2026-06-11-core-competence-and-scope-line.md:12` (stale 28), `:24-30` (ladder + leash) |
| Verified money anchor / vendor-grade DD figure | CAST `$10.29K/app/yr` (pricing page, S3); `$200K–$800K` flagged vendor-estimate |
| a16z data-moat framing (recon ≠ network effect) | [a16z, "The Empty Promise of Data Moats," 2019](https://a16z.com/the-empty-promise-of-data-moats/) |
| Sourcegraph SCIP incremental "on roadmap" | [announcing-scip](https://sourcegraph.com/blog/announcing-scip) / [scip-clang](https://sourcegraph.com/blog/announcing-scip-clang) |
| Potpie funded ($2.2M, blast-radius table-stakes) | [FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html) |
| Agent-canvas won pattern | [tldraw Agents on the Canvas, JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw); [mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw) |
| Living-contract best practice | [Mermaid living-contract](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development) |
| Tool-receipts / provenance frontier | [arxiv 2603.10060 "Tool Receipts"](https://arxiv.org/html/2603.10060v1); KPMG [nerova.ai](https://nerova.ai/troubleshooting-fixes/kpmg-pulls-agentic-ai-report-hallucinations-june-13-2026) |
| Anthropic "agentic technical debt" attribution (single-blog, flagged) | [Dev\|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/) |
| Engineering red-team (sibling record) | `playbook/enhancement/05-redteam-alignment.md` |
| The full position this record validates | `playbook/STRATEGIC-POSITION.md` |
