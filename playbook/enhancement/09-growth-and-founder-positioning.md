# 09 — Growth & Founder Positioning: the PLG wedge, the viral loop, and the moat-to-money path

**Status:** growth-hacker + founder/VC deep dive — DRAFT companion to
`../ENHANCEMENT-GUIDE.md` and the integrated position in
`../STRATEGIC-POSITION.md`. **No code changes prescribed — markdown only.**
**Falsifiable document:** every load-bearing claim cites a `file:line`, a source
doc, or a URL; vendor/marketing/unverified figures are flagged inline; if a cited
anchor is wrong, the claim built on it is void.

**Binding law (do not relitigate here):**
- **Final position (authoritative, survived a four-lens red-team)** —
  `../STRATEGIC-POSITION.md` (§0 current-state banner, §B moats-by-axis,
  §C the loop, §D growth/founder, §E 90-day play, §F red-team). This doc is the
  **growth/founder zoom** of that position's §D/§E — it sharpens, it does not
  contradict.
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
  (three-layer ladder; the free/paid line maps onto it, §5)
- **Goals** — `.context/GOALS.md` (esp. G3 Agent-First, G6 Value Proof)
- **Prior research verdict** — `.context/details/2026-06-19-graphify-plus-mcp-research.md`
- **Sibling deep-dives** — `04-scale-and-positioning.md` (the honest 400+ story
  + the diff wedge), `03-access-surface.md` (the thin-MCP shape), `05-redteam-alignment.md`

**Scope of this document.** Strategy docs 01–05 prove *what aOa is* and *why the
moats hold in diligence*. This doc answers the orthogonal question: **how does it
get adopted, spread, and convert to money — and how does the positioning survive
the two honest risks (grep-preference and graph-as-retrieval-is-contested)
without flinching?** It is two halves held together: the **growth-hacker play**
(wedge, loops, instrumented funnel) and the **founder/VC framing** (why-now,
moat-to-money, pricing posture, the fundable shape).

---

## §0. The current-state banner (read first — this is a go-to-market roadmap, not a launched GTM)

The position's §0 banner binds here too, because **growth claims are the easiest
place to overstate.** Restated for this doc:

- **Ships today (zero new code):** the sub-ms `grep → peek` CLI over the O(1)
  token index (`internal/ports/storage.go:60`); atlas enrichment; the
  build-time judged viewer (`playbook/generators/build_blueprint_viewer.py`,
  gated by `playbook/standards/MODEL-STANDARD.md`). **This is the only thing the
  Phase-0 GTM may lead with.**
- **NOT built (every loop below that depends on it is Phase-2+):** the import-edge
  keystone, `aoa arch`, the overlay loader, the live arch-shard endpoint, the
  file-save→ETag tick (`bumpRevision()` has four callers, none on the file-change
  path — `../STRATEGIC-POSITION.md` §0, verified `internal/app/app.go:350` /
  `:564` / `:901` / `:2896` / `:2905`), and AG-UI.

**The growth consequence, stated up front:** the **strongest viral artifact (the
before/after plan diff) arrives in Phase 2, not at launch.** A launch front-loaded
on the diff is a launch on vapor. Phase 0 growth is **wedge-driven** (the
shipping CLI), not **loop-driven**. The §6 sequencing law makes this binding so a
growth-hacker reading only this doc cannot accidentally promise the unbuilt thing.

---

## §1. The position in one growth paragraph

> The wedge is `aoa init` + a one-command binary install: lower-friction than
> every competitor (graphify's build is 114 min on 50K files,
> [graphify #341](https://github.com/safishamsi/graphify/issues/341); Potpie needs
> Neo4j + PostgreSQL + Celery + Redis,
> [TechFundingNews](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/);
> CAST/Sonargraph need a per-app license + server). But a low-friction *wedge* is
> not a *loop* — it proves easier-to-start, it does not explain how user N
> recruits user N+1. aOa's one designed loop is the **recipient-must-`init`**
> mechanic: a teammate who receives an interactive plan-diff link must run
> `aoa init` to *click-to-investigate*, because the suggestion is computed against
> *their own* fresh local substrate. That loop is **Phase 2** (it depends on the
> diff renderer). Until then, growth is the shipping CLI an agent adopts in one
> command, plus one provenance-stamped OSS view a maintainer would re-share. The
> money path runs **free structural views → paid governance/evidence packs**,
> anchored on the **verified $10.29K/app/yr** CAST product price
> ([CAST pricing, S3, `../STRATEGIC-POSITION.md` §A]) — and the positioning
> answers the two honest risks head-on: aOa **is** the fresh-grep substrate the
> frontier preferred (it does not fight grep, it *is* grep + 4 grep-beaters), and
> it never bets the company on graph-as-retrieval (the contested bet) — it bets on
> **provenance + the AST-derived diff**, which no grep and no LLM-canvas can fake.

---

## §2. The PLG wedge — `aoa init`, frictionless, in-tool, and honestly bounded

### 2.1 The wedge is real and it is a category-low

Adoption friction is the first thing a growth motion optimizes, and here aOa wins
structurally — but the right verb is **wedge**, not **moat** (`../STRATEGIC-POSITION.md`
§D.1 downgrades this honestly):

| Competitor | What a developer must do to get a first answer | Friction class |
|---|---|---|
| **graphify** | clone, `pip`, run a build script (114 min / 50K files, [#341](https://github.com/safishamsi/graphify/issues/341)); rebuild can refuse to overwrite ([#653](https://github.com/safishamsi/graphify/issues/653)) | minutes-to-hours batch |
| **Potpie** | stand up Neo4j + PostgreSQL + Celery + Redis ([TFN](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/)) | infra-grade |
| **CAST / Sonargraph** | per-app license + server install | procurement-grade |
| **Glean / Sourcegraph** | cloud crawl + connector config | org-grade |
| **aOa** | `aoa init`, then a one-command npm binary install + a daemon | **single-command** |

The wedge converts to the agent in **one line of CLAUDE.md**: route code search
through `aoa grep → aoa peek` (the G3 drop-in-shim contract, `.context/GOALS.md`;
the agent "never knows it's not GNU grep"). This is the lowest-friction surface in
the category and it **ships today** (§0).

### 2.2 The honest friction (the growth lens must not hide it)

"Zero infra" is a near-truth, not a truth, and a founder who oversells it loses
the room:

- The **real binary is the tree-sitter build with 509 compiled grammars + a CGo
  daemon** (`internal/adapters/treesitter/languages_forest.go:5`) — not a
  pure-Go single static file. "Install a binary" is accurate; "no native
  dependencies" is not.
- The **recon / live-status value materializes only if the daemon runs
  persistently**, and only for users **on Claude Code** — the recon signal source
  is `~/.claude/projects/*.jsonl` (`../STRATEGIC-POSITION.md` §B-Moat-E). The
  *immediately*-addressable base for the live-signal loops is therefore **Claude
  Code's CLI-agent base — a beachhead, not unbounded TAM.** Name it as a
  beachhead and it reads as focus; hide it and it reads as a gap a red-teamer
  finds (§F.7 of the position).

The grep→peek CLI itself, however, has **none** of this coupling — it is a plain
binary an agent shells out to, host-agnostic. **The host-coupling is a property of
the recon loops, not of the wedge.** Lead adoption with the host-agnostic wedge;
introduce the recon loops as the deepening that follows once the user is already
on Claude Code.

---

## §3. The viral / shareable loops — sequenced by what actually ships

The growth law (top of `../STRATEGIC-POSITION.md` §D): **the share-worthy loops
light up only after Phase 0.** Lead with what ships; sequence the rest honestly.

### 3.1 The pre-keystone loop (zero new code, ships Tuesday)

The only loop that does not wait on the keystone:

- **The drop-in CLI an agent adopts in one command.** `aoa init` → CLAUDE.md
  routes search through `aoa grep`/`aoa peek` → the agent gets sub-ms,
  per-method, provenance-anchored answers it would otherwise grep for. The
  "spread" vector is **agent-config sharing** — a developer who adds the routing
  line to a repo's CLAUDE.md ships it to every teammate and every agent run on
  that repo. (Caveat: this is *config propagation*, weaker than the recipient-init
  loop of §3.3 — but it is real, it is today, and it needs proof of its own
  share-worthiness, not assumption.)
- **One shareable artifact that needs no new code:** a **provenance-stamped
  DSM/cycles view of a popular OSS repo**, rendered by the existing build-time
  generator (`playbook/generators/`), that a maintainer would re-share. The
  growth task in Phase 0 is to **prove that single artifact is share-worthy on
  its own** — independent of the unbuilt diff. (Caveat: the build-time generator
  produces the view today; the *live* daemon-served version is post-keystone, so
  the artifact carries a "rendered from a live `aoa init`" claim only after
  Phase 1.)

### 3.2 The wedge ≠ the loop (the distinction that keeps the deck honest)

A **wedge** proves easier-to-start. A **loop** describes how user N's `init`
recruits user N+1. Conflating them is the single most common growth-deck error,
and `../STRATEGIC-POSITION.md` §D.1 cuts it explicitly: `aoa init` is the
**lowest-friction wedge in the category** and **not, by itself, a loop.** The
static-PNG artifacts of §3.1 spread by *impression* (someone sees it, may
install) — real but weak, the same mechanic GitDiagram rode to a viral spike that
did not convert to retained use (the GitDiagram / DeepWiki viral-but-unretained
cohort of LLM-diagram-from-code tools).

### 3.3 The designed install loop (Phase 2 — the real one)

The interactive before/after canvas **fires Claude locally**. A teammate who
receives a plan-diff link must run `aoa init` to **click-to-investigate** —
because the suggestion is computed against *their* local fresh substrate (a
non-user can view a static PNG but **cannot interact**: clicking a node calls
`aoa arch facts` against a substrate they don't have). **That recipient-must-init
mechanic is the install loop the static artifacts lack** — the interaction is the
gate, and the gate is the install.

**The single instrumented metric that proves the loop closes:**
`init → second-session → teammate-init` (not merely `init → second-session`). The
third hop is the loop; the first two are activation. Instrument all three from
Phase 0 so the loop's arrival in Phase 2 has a baseline to lift.

> **Drop the OpenClaw / raw-star-velocity comp.** A discredited vanity metric
> (stars — which this doc itself calls vanity, §4.2) cannot prove a loop works.
> The loop proof is the **recipient-must-init mechanic** instrumented as the
> three-hop funnel, not a star-spike analogy.

### 3.4 Shareable-artifact ladder, each marked with its earliest-ship gate

| Artifact | Viral mechanic | Earliest ship | Strength (honest) |
|---|---|---|---|
| **Provenance-stamped DSM/cycles of an OSS repo** (existing generator) | impression / maintainer re-share | **Phase 0 (today)** | Weak–medium — needs its own share-worthiness proof (§3.1) |
| **README GIF** (`aoa init` → live REAL-stamped view passing the blind judge) | star spike | **Phase 1** (live stamp post-keystone) | **Weak** — a vanity star spike, not retained adoption; do **not** front-load a launch on it |
| **Before/after plan diff** (the decision artifact teammates argue over) | install loop (recipient must `init`) | **Phase 2** | **Strongest — and most contingent** (Moat B seam, `../STRATEGIC-POSITION.md` §B) |
| **Evidence pack** ("what changed since last review," SHA-stamped) | shares **upward** into the buying center | **Phase 3 (paid)** | High value, latest, converts the org (§5) |

**Why the diff is the only *natively* viral artifact:** every competitor fakes
"before/after" as two hand-drawn pictures and "quality" as eyeballing. aOa derives
**both** sides from one substrate (BEFORE = live code-derived; AFTER = an agent's
plan re-run through the *same* deterministic detectors) and computes the delta
from edge-set arithmetic, gated by the blind judge
(`playbook/standards/MODEL-STANDARD.md`). **The diff is un-fakeable only because
the BEFORE is *derived*, not *drawn*** — which is exactly why it is the share-worthy
decision artifact a team argues over, and exactly why a launch must wait for it
rather than fake it (`04-scale-and-positioning.md` §4).

---

## §4. Growth-hacker tactics (the boring, compounding ones — not shiny objects)

The user's north star is **growth-hacker oriented AND leading-edge — NOT
shiny-object chasing.** So this section is deliberately unglamorous: the plays that
compound.

### 4.1 Launch as a sequence, not a spike

- **README = one-line category claim + one GIF,** not a badge wall. The category
  line: *"the deterministic, provenance-stamped code-fact substrate your agent
  greps and your team sees."* The GIF: `aoa init` → a REAL-stamped view passing
  the judge (Phase 1).
- **Community-first, then SEO.** Land in the CLI-agent communities (the segment
  that *chose grep* — aOa is its native ally, not its challenger, §6.1), then
  build **per-language landing pages** (Go, Python, TS first — the three tuned
  extractors, `04-scale-and-positioning.md` §1) as durable SEO surfaces. Resist a
  single big-bang launch; the diff (the real artifact) is not ready for it until
  Phase 2.

### 4.2 Instrument the funnel that matters, ignore the vanity one

- **Track `init → second-session → teammate-init`** (§3.3). Second-session is
  activation; teammate-init is the loop.
- **Do not optimize raw GitHub stars.** Stars are an impression metric that drift
  daily and that the LLM-diagram cohort proved does not convert
  (the GitDiagram / DeepWiki viral-but-unretained cohort). A star spike off the
  Phase-1 README GIF is *expected and fine* — but it is a top-of-funnel signal, not
  retained adoption, and the deck must not mistake one for the other (`../STRATEGIC-POSITION.md`
  §F.1).

### 4.3 Ride the workflow-fit tailwind

The 2026 adoption evidence is unambiguous: the fastest-growing dev tools win on
**workflow fit, not benchmark scores** — in Q1 2026 the fastest-growing primary AI
coding tool "was the tool that slotted cleanly into existing IDE, terminal, and
review habits," not the benchmark leader
([AI Coding Tool Adoption 2026, digitalapplied](https://www.digitalapplied.com/blog/ai-coding-tool-adoption-2026-developer-survey)).
aOa's grep-shim *is* workflow fit — it slots into the terminal/agent habit an agent
already has (G3). **Lead the growth story on "it disappears into the grep you
already run," not on a feature list.**

### 4.4 The leash IS a growth feature, not just a safety one

The user's explicit warning is against gimmicks (autonomous-rewrite Mode C). The
growth framing of the leash: the shareable artifact is the **decision** (BEFORE vs
AST-derived AFTER) — the agent **suggests**, the human **disposes**
(`../STRATEGIC-POSITION.md` §C.5). A decision is more shareable than a silent
auto-rewrite, *and* it is leash-safe — viral loop and safety boundary are the same
boundary. That alignment is rare and worth naming in the pitch: the safe thing and
the viral thing are the same thing.

---

## §5. The moat-to-money path — free views → paid governance/evidence packs

### 5.1 Why governance is the conversion lever

The PLG-to-sales literature is convergent: a product-led developer tool monetizes
the org by **gating governance** (RBAC, SSO/SAML, SCIM, audit logs, data
residency) — these are "frequently what triggers the move upmarket and the need
for sales-assisted procurement"
([Lenny's / Hila Qu, GitLab](https://www.lennysnewsletter.com/p/the-ultimate-guide-to-adding-a-plg)).
GitLab and HashiCorp are the canonical templates: free/community substrate drives
bottom-up developer adoption; team-scale governance + audit converts the buyer
(ibid.). **aOa's free/paid line maps directly onto its own scope law** — so the
monetization is not bolted on, it is the layer ladder read as a price ladder:

| | **FREE at `aoa init`** | **PAID: governance & evidence** |
|---|---|---|
| Substrate + 16 views + shareable diagram + Mermaid living-contract rendition (layer 1, REAL) | ✓ | multi-repo / org rollups, estate landscape |
| Agent surface: CLI/socket/thin-MCP (grep→peek + 4 grep-beaters) | ✓ | — |
| Conformance — declared pattern vs derived actual (layer 3, declare-and-diff) | — | ✓ baseline/freeze — *the "direction" layer Anthropic's remedy calls for (§7)* |
| Drift / before-after diff | the diff renderer as a free decision artifact | "what changed since last review" packs, SHA-stamped, audit-ready |
| Audit | `aoa arch facts` (the grounding receipt) | audit-ready export (CycloneDX/SPDX SBOM, in-toto/SLSA-shaped) |

The gate is by **usage metric (repos / LOC) and governance surface**, never by
crippling the substrate — crippling the free substrate kills the wedge that drives
the whole motion.

### 5.2 The money anchor (verified, and the flagged ceiling)

- **Anchor on the verified product price: CAST charges $10.29K/app/yr**
  (`../STRATEGIC-POSITION.md` §A, S3, CAST pricing page). This is enough to
  support a "self-service, faster, cheaper" wedge under the incumbent's per-app
  shelfware economics — the classic land-from-below move.
- **Flag, do not headline, the M&A-due-diligence ceiling.** The $200K–$800K DD
  figure is a **services-engagement number, vendor/estimate-grade** — keep it as
  an *illustrative ceiling*, clearly marked, **not** the willingness-to-pay
  headline (`../STRATEGIC-POSITION.md` §D.4). Headlining an unverifiable services
  number is exactly the kind of figure a diligence lens cuts.

### 5.3 The timing — when to add the sales motion

The practitioner consensus is tight and worth pinning so the founder model is not
fantastical: **layer sales onto a PLG dev tool at roughly $10M self-serve ARR**
(Elena Verna's specific trigger; broader range $5M–$15M), and **only when intent
exists** — inbound $10K+ hand-raises, not ARR alone
([digitalapplied PLG vs sales-led 2026](https://www.digitalapplied.com/blog/plg-vs-sales-led-gtm-motion-2026-saas-decision-framework)).
The economic floor under the gate: **PLG hits a ceiling around ~$10K ARR/account**
(credit-card / expense-the-department-head zone); past it, legal + security
questionnaires + multi-stakeholder sign-off kick in — which is precisely why the
**audit/evidence pack travels *upward* into the buying center** (§3.4). aOa's
$10.29K CAST-anchored governance tier sits right at that PLG ceiling — the natural
hand-off point from self-serve to sales-assisted.

> **Correction carried in:** the loose "$10M–$50M ARR" band in some internal
> notes is not supported by the practitioner sources — the **add-sales** trigger
> clusters at **~$10M** ([ibid.](https://www.lennysnewsletter.com/p/the-ultimate-guide-to-adding-a-plg));
> $50M is a later *scale-enterprise-sales* stage, not the *add-sales* threshold.
> Use ~$10M as the add-sales number.

### 5.4 The tailwind that makes the paid tier inevitable, not optional

Compliance is shifting **point-in-time → continuous**: 2026 frameworks (SOC 2,
ISO 27001, NIS2, DORA) "require organizations to monitor controls *continuously*,
maintain real-time visibility into risks, and provide structured evidence during
audits"
([Quantarra compliance buyer's guide 2026](https://quantarra.io/blog/the-buyers-guide-to-compliance-automation-software-in-2026)).
A **continuously-fresh, provenance-stamped substrate is a near-real-time evidence
engine** — the diff renderer plus `aoa arch facts` *is* the structured,
continuously-current evidence those frameworks now demand. This is the rare case
where the moat (freshness + provenance) and the buyer's regulatory obligation are
the same property. (Pricing direction, agency-blog-sourced / **directional**:
per-seat is in decline — IDC projects 70% of vendors off pure per-seat by 2028 —
so gate by usage/governance, not seats; 3–5% freemium conversion on a viral base
is healthy.)

---

## §6. Pricing & positioning posture vs CAST / Sonar / Sourcegraph

| Incumbent | Their shape | aOa's posture against it |
|---|---|---|
| **CAST** | M&A "MRI for software," batch reverse-engineered map, **$10.29K/app/yr** + services DD tier; genuinely out-covers aOa on language breadth (150+, mainframe — its one real advantage) | **Self-service, faster, cheaper, continuously-fresh.** Land below the per-app shelfware line; concede language breadth, win on freshness + provenance + zero-procurement adoption. Do not claim to out-cover CAST on languages. |
| **Sonar / Sonargraph / NDepend** | Architecture-conformance / DSM as a **batch scan artifact**, language-capped (Sonar 5/1, NDepend .NET-only), per-seat → bundled; some now ship MCP | **Conformance on a live substrate, not a batch scan**; gate by usage not seats (riding the per-seat decline, §5.4). aOa's conformance is declare-and-diff against *fresh derived edges*, not a periodic report. |
| **Sourcegraph (SCIP/Amp)** | Enterprise code search + precise-nav + MCP; **SCIP was *designed* for incremental** but incremental remains "on the roadmap" ([scip-clang](https://sourcegraph.com/blog/announcing-scip-clang)) | **Cadence, not capability:** their data plane already *wants* incremental — the gap is per-commit-cloud vs per-keystroke-local, a tunable parameter for the viewer but a structural floor for the **local CLI answer**. Position on the local latency floor + provenance-on-every-answer, not on "we have a graph and they don't." |

**The posture in one line:** aOa is **not** the cheap clone of the governance
incumbents — it is the **bottom-up developer wedge that grows into their buyer**,
landing free in the terminal and expanding into the compliance/evidence budget the
2026 hybrid motion is built for
([getmonetizely bottom-up enterprise](https://www.getmonetizely.com/articles/can-you-win-enterprise-deals-with-bottom-up-developer-adoption)).
Snowflake's dual motion (developer adoption + data-leader engagement) is the model:
land with the dev, expand into the budget-holder (ibid.).

---

## §7. The founder why-now (the demand→product arrow, earned)

Four de-risking facts assemble the why-now (`../STRATEGIC-POSITION.md` §D.3):

1. **The interface bet is won** — MCP under the Linux Foundation Agentic AI
   Foundation, >10,000 servers (S6 §5). Exposing aOa over a *thin* MCP rides a
   won wave (and the thinness — 4 grep-beaters vs the cluster's 42–45 tools — is
   itself the diligence moat, `03-access-surface.md`).
2. **Agents need fresh structural context and the build-artifact tier is
   structurally stale** (the §A landscape table of `../STRATEGIC-POSITION.md`).
3. **Agentic technical debt at machine speed is the *named* demand** — Anthropic
   named the failure mode in 2026
   ([Dev|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/)).
4. **"Architecture as Code"** (Ford & Richards, O'Reilly 2026) is the conceptual
   seal — the category language now exists.

**The demand→product arrow (the part a founder lens insists be *earned*, not
asserted):** the *same* sources that name the failure name the **remedy —
*written constraints* (ADRs / CLAUDE.md / specs) that give architectural
*direction*** — and say plainly that *"memory tools solve for data recall but fail
to provide architectural direction"* (ibid.). Fresh facts = recall
(necessary-not-sufficient; the memory tools already do recall). **The product the
named demand actually pulls toward is CONFORMANCE — aOa's declared-pattern-vs-
derived-actual diff (§5, the paid tier) — the automated *enforcement* those
written constraints need.** Constraints written in prose cannot be machine-checked;
aOa is the layer that machine-checks them against a fresh, provenance-stamped
substrate. Anthropic's own "skeptical memory" guidance even tells the agent to
**grep to verify current reality** before critical changes — which is exactly
aOa-as-fresh-grep-substrate. **Lead the why-now with conformance/drift-detection as
the product; demote "truth source" to a supporting claim.**

> **Founder pitch (the spoken version):** *Anthropic named the failure — agentic
> tech debt — and named the remedy — written constraints for direction.
> Constraints written in prose can't be machine-enforced. aOa is the automated
> enforcement layer: it derives the actual architecture deterministically and
> diffs it against the declared pattern — conformance, fresh, provenance-stamped.
> Competitors have pieces — graphify has a stale, LLM-noisy graph; CAST has batch
> governance at enterprise prices; Potpie ($2.2M pre-seed, thesis validated) has
> blast-radius on enterprise-Neo4j; the canvas SDKs have the interaction grounded
> on prose. aOa is the first to make the constraint machine-checkable against a
> fresh, provenance-stamped substrate.*

---

## §8. The fundable shape — priority-ordered for diligence

The 2026 VC playbook is blunt: *"founders who answer with compliance moats are
closing rounds in weeks"*; founders who wave off hallucination risk die in
diligence (`../STRATEGIC-POSITION.md` §D.5, S7). So the deck **leads with what
survives diligence, not what demos best:**

1. **Verifiability / compliance moat** (determinism + provenance — an
   in-toto/auditability property, **NOT** an SLSA reproducible-builds claim, S6
   §3) → **Moat D.** This is the first thing a 2026 investor wants.
2. **The AST-derived conformance / diff** → **Moat B** (the un-fakeable BEFORE no
   LLM-canvas can produce).
3. **Thin-MCP token economics** → **Moat C** (4 grep-beaters vs 42–45; ~40% of
   context lost to MCP metadata is the receipt, S6 §5).
4. **Reclassify freshness (A) and recon (E) as defensible-but-not-structural** —
   A is narrowing (Sourcegraph designed SCIP for incremental; the watch-mode
   cluster ships file-watch), E is a switching-cost / system-of-record moat,
   **not** a data network effect
   ([a16z, "The Empty Promise of Data Moats," 2019](https://a16z.com/the-empty-promise-of-data-moats/)).
   Selling A or E as a structural wall is the fastest way to get cut in diligence.

**Category:** *"agent infrastructure, not a wrapper"* — no per-query inference cost
(vs Cursor's burn), because the substrate is deterministic AST output, not an LLM
in the answer path.

**Potpie reframed (the founder-lens correction):** **not** "Potpie sharpens the
same thesis" (that signals me-too). **Potpie ($2.2M pre-seed, verified
[FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html))
validates the budget exists AND proves the thesis is already contested at seed —
aOa's wedge is the freshness/provenance/local-economics Potpie's
enterprise-Neo4j/Celery-batch shape structurally cannot match.** Blast-radius as a
*feature* is table-stakes (Potpie ships it); aOa's differentiation is **delivery
shape**, not the query.

**Market honesty:** there is **no clean standalone "architecture governance" TAM.**
Lead with the *tailwind* (GRC / compliance-automation proxies — directional figures
only) plus the verified CAST/Potpie willingness-to-pay anchors, **not** a fabricated
TAM. A made-up TAM is a diligence landmine; verified per-app/per-round anchors are
defensible math.

---

## §9. How the positioning handles the two honest risks (do not flinch)

The whole position rests on two conceded risks. A growth/founder doc that hides
them is worthless; here is how the *positioning itself* absorbs each.

### 9.1 Risk: developers (and frontier agents) *prefer grep*

**The concession (real):** Claude Code removed vector search (May 2025); "Is Grep
All You Need?" (2026) found inline lexical beat dense retrieval; the frontier CLI
agents chose agentic grep over indexes, citing freshness/staleness/cost
(`../STRATEGIC-POSITION.md` §A, S6 §1; `.context/details/2026-06-19-graphify-plus-mcp-research.md`
§Q2).

**Why it does not threaten aOa — it *is* the strategy.** aOa does **not** ask a
developer or an agent to stop grepping. **aOa *is* the grep** — the drop-in shim
*is* `grep`/`peek`, only faster (sub-ms socket), fresher (live index), and
provenance-stamped (G3, `03-access-surface.md` §4.3). On top of that it adds
**exactly 4 grep-beaters** (reachability, blast-radius, cycles/DSM, god-nodes) —
the *only* queries grep structurally cannot answer in one shot — and **refuses**
the 1-hop neighbor lookup that degrades into a worse, stale grep
(`03-access-surface.md` §5.2). The convergent research conclusion is **hybrid**
(structural query + grep fallback) beats either alone
(`.context/details/2026-06-19-graphify-plus-mcp-research.md` §Q2), and aOa is that
hybrid *by construction.* **The grep-preference finding is aOa's tailwind, not its
headwind** — it validates the exact shape aOa chose, against every competitor that
bet on the standing graph the frontier declined.

### 9.2 Risk: graph-as-the-agent's-retrieval is *contested*

**The concession (real):** academic graph-retrieval gains are recall-side and
small in absolute terms (RepoGraph ~+2 pts on SWE-bench); the CLI-agent segment
declined the standing graph; graphify's own core `query_graph` is a stale
full-scan — a worse grep
(`.context/details/2026-06-19-graphify-plus-mcp-research.md` §Q2).

**Why it does not threaten aOa — aOa never bets the company on it.** The position
does **not** sell graph-as-primary-retrieval. The Tier-1 graph is **deliberately
narrow** — imports + derived DSM/cycles/reachability/affected-set — and it is
**never** the agent's default retrieval path (the default verb stays grep→peek,
`03-access-surface.md` §4.3; the heavyweight standing graph is on the explicit
OUT list, `05-redteam-alignment.md` §4). **What aOa bets on is provenance + the
AST-derived diff** — neither of which is a retrieval claim, and neither of which a
grep or an LLM-canvas can fake. The graph is a *substrate for the diff and the
4 grep-beaters*, not a retrieval index the agent is asked to trust over grep.
**So the contested bet is one aOa structurally declined to make** — the company
is funded on the moats the §8 ordering leads with (D, B, C), not on graph
retrieval (A/E are explicitly demoted).

**The meta-point for the founder:** both honest risks, fully conceded, leave the
fundable spine *intact* — because the spine was never the contested thing. That is
the strongest possible answer to a diligence lens: *"we already cut the part you'd
attack, and here is what's left."*

---

## §10. The 90-day GTM play (growth lane, mirroring `../STRATEGIC-POSITION.md` §E)

| Phase | Growth (what's shippable) | Founder | Gate |
|---|---|---|---|
| **0 — ship what exists** | Lead with the grep→peek CLI (zero new code) + a provenance-stamped OSS DSM/cycles view from the existing generator (§3.1). **Instrument `init → second-session → teammate-init` from day one.** | Pre-seed deck ordered D → B → C; freshness/recon reclassified non-structural; Potpie = budget-validated-and-contested; **anchor money on $10.29K, not the unverified DD ceiling** | Keystone (import edges on the always-on parse pass, **G0 ≤+3%**) GREEN on a live stranger repo; reconcile the stale "28-language" figure to the ladder (`04-scale-and-positioning.md` §1) |
| **1 — the live face** | README = one-line category + GIF (live REAL-stamped view). *Expect a star spike (vanity); do not mistake it for retained adoption.* Community-first, then per-language SEO pages. | Land "conformance is the enforcement layer for Anthropic's named remedy" (§7) | `aoa arch` + `MethodArch*` socket methods; arch-shard producer served through `withETag`; **the absent `bumpRevision()` on-reindex line** |
| **2 — the loop** | **Ship the before/after diff (Mode A) — the recipient-must-init loop (§3.3). Track the third funnel hop.** | The agentic-tech-debt why-now, live | Net-new overlay loader (rejects invented ids); thin MCP = 4 grep-beaters only |
| **3 — the money** | Evidence pack as the upward-traveling artifact (§3.4). | Open paid governance/conformance tier; target CAST's compliance budget self-service; add sales motion only at ~$10M self-serve ARR **with** $10K+ inbound intent (§5.3) | CycloneDX/SPDX SBOM export; conformance = declared-vs-derived (layer 3) |

---

## §11. Red-team targets (swing here first)

1. **The strongest viral artifact is Phase-2 (§0, §3.4).** Every loop claim past
   §3.1 is roadmap. *Defense:* the grep→peek CLI + judged OSS view carry Phase-0
   adoption with **zero new code**; the loop is honestly staged, and the funnel is
   instrumented from day one so the loop's arrival is measurable, not asserted.
2. **The wedge is not a loop (§3.2).** Config-propagation and impression-share are
   weak mechanics. *Defense:* the recipient-must-init mechanic (§3.3) is a *real*
   loop with a *single instrumented metric* — the position does not pretend the
   wedge spreads itself.
3. **Host-coupling bounds the recon-loop TAM (§2.2).** Beachhead = Claude Code.
   *Defense:* the grep→peek **wedge is host-agnostic**; only the recon *loops* are
   coupled — and Claude Code is the fastest-growing CLI-agent base, a focus not a
   ceiling.
4. **"Agent tool with a governance upsell, or governance tool with an agent
   funnel?"** The honest answer may be the latter (the conformance/diff is the
   fundable core, §7) — the deck must not pretend otherwise. *Defense:* either
   framing converges on the same product; §8 leads with the governance/compliance
   moat precisely because that is what a 2026 investor funds.
5. **The money anchor could be inflated.** *Defense:* this doc headlines the
   **verified $10.29K** and explicitly flags the $200K–$800K DD figure as a
   services-grade ceiling, not WTP (§5.2). The add-sales threshold is corrected to
   the sourced ~$10M, not the unsupported $50M (§5.3).
6. **Grep-preference and contested-graph-retrieval could sink the thesis.**
   *Defense:* §9 — both are conceded in full and both leave the spine intact,
   because the spine (provenance + diff) was never the contested thing.
7. **Pricing/conversion figures are directional.** *Defense:* every %, CAGR, and
   IDC/conversion number is flagged directional/agency-sourced; only the CAST
   product price, Potpie raise, Glean ARR, and the ~$10M add-sales trigger are
   treated as load-bearing, and each carries a URL.

---

## §12. Honesty flags (carried forward + new to this doc)

- The keystone, `aoa arch` surface, overlay loader, arch-shard endpoint,
  file-save→ETag tick, and AG-UI are **NOT yet built** (§0); the strongest viral
  artifact is Phase-2.
- `aoa init` is a **wedge, not a loop**; the only designed loop (recipient-must-init)
  is Phase-2.
- The recon/live loops require a running daemon + Claude Code session logs
  (**Claude Code beachhead**); the grep→peek wedge is host-agnostic.
- **Money anchor = the verified $10.29K/app/yr CAST product price**; the $200K–$800K
  M&A-DD figure is a **services-engagement estimate, vendor-grade — NOT a product
  price** and not the WTP headline.
- The **add-sales trigger is ~$10M self-serve ARR** (sourced); the looser
  "$10M–$50M" band is corrected — $50M is a later scale-enterprise stage, not the
  add-sales threshold.
- All conversion %s, CAGR, freemium-conversion, and IDC per-seat figures are
  **directional / agency-blog-sourced.**
- The **a16z data-moat essay is 2019** (Casado/Lauten — evergreen VC consensus,
  not a 2026 publication); recon defends as switching-cost/system-of-record, **not**
  a data network effect.
- graphify "YC-S26" is **unverified/self-applied** (the ~69K-star *project* is
  real); Potpie's revenue (Latka) is directional, the **$2.2M raise is verified**.
- "Deterministic" ≠ SLSA-reproducible (in-toto/auditability framing only); there
  is **no clean standalone architecture-governance TAM** (lead with tailwind +
  verified WTP anchors).
- CAST genuinely **out-covers aOa on language breadth** (150+, mainframe) — its one
  real advantage; do not claim parity there.

---

## §13. Grounding

**Internal (authoritative, verified against live source by the position):**
`../STRATEGIC-POSITION.md` (the four-lens-survived final position this doc zooms
into — §0 banner, §B moats, §C loop, §D growth/founder, §E play, §F red-team);
`.context/details/2026-06-19-graphify-plus-mcp-research.md` (prior verdict);
`04-scale-and-positioning.md` (the diff wedge + the honest language ladder);
`03-access-surface.md` (the thin-MCP / grep-stays-primary shape);
`05-redteam-alignment.md` (the OUT list + alignment checklist);
`.context/decisions/2026-06-11-core-competence-and-scope-line.md` (scope law →
free/paid ladder); `.context/GOALS.md` (G3, G6).

**aOa source anchors (verified):** `internal/ports/storage.go:60` (O(1) token
index); `internal/adapters/treesitter/languages_forest.go:5` (509 grammars / CGo
build reality); `internal/app/app.go:350` + `:564/:901/:2896/:2905`
(`bumpRevision` and its four callers — none on the file-change path);
`playbook/generators/build_blueprint_viewer.py` (build-time generator, not a live
endpoint); `playbook/standards/MODEL-STANDARD.md` (blind-judge gate).

**External (URL-cited; vendor/marketing/directional figures flagged inline):**
[Anthropic agentic-tech-debt + ADR/CLAUDE.md remedy (Dev|Journal)](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/);
[a16z "Empty Promise of Data Moats," 2019](https://a16z.com/the-empty-promise-of-data-moats/);
[Potpie $2.2M, FinSMEs](https://www.finsmes.com/2026/02/potpie-ai-raises-2-2m-in-pre-seed-funding.html)
+ [TechFundingNews (Neo4j/Celery infra)](https://techfundingnews.com/the-startup-building-a-knowledge-graph-for-code-raises-2-2m-to-make-ai-agents-actually-useful/);
[Sourcegraph SCIP incremental "on roadmap"](https://sourcegraph.com/blog/announcing-scip-clang);
graphify [#341 (114-min build)](https://github.com/safishamsi/graphify/issues/341)
+ [#653 (rebuild refuses overwrite)](https://github.com/safishamsi/graphify/issues/653);
LLM-diagram-from-code cohort (GitDiagram / DeepWiki viral-but-unretained evidence);
PLG→sales motion + GitLab/HashiCorp + governance-gating + ~$10K/account ceiling
([Lenny's / Hila Qu](https://www.lennysnewsletter.com/p/the-ultimate-guide-to-adding-a-plg),
[digitalapplied PLG vs sales-led 2026](https://www.digitalapplied.com/blog/plg-vs-sales-led-gtm-motion-2026-saas-decision-framework));
[bottom-up dev adoption → enterprise (getmonetizely)](https://www.getmonetizely.com/articles/can-you-win-enterprise-deals-with-bottom-up-developer-adoption);
[workflow-fit beats benchmarks, AI coding adoption 2026 (digitalapplied)](https://www.digitalapplied.com/blog/ai-coding-tool-adoption-2026-developer-survey);
[continuous-compliance / structured-evidence demand (Quantarra 2026)](https://quantarra.io/blog/the-buyers-guide-to-compliance-automation-software-in-2026).

---

**Provenance note.** This document is the growth/founder zoom of the round-2
integrated position in `../STRATEGIC-POSITION.md`; it reproduces that document's
verified anchors and adds the GTM/funnel/pricing/why-now detail with fresh
external grounding for the PLG-motion and bottom-up-adoption claims. No code or
source files were edited — markdown only.
