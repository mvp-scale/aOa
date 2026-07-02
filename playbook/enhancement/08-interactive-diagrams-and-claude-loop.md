# 08 — The Interactive Real-Time Diagram + the Claude Before/After Loop

**Status:** headline-feature architectural position — DRAFT for green-light. No
code changes prescribed. **This is a falsifiable document:** every load-bearing
claim cites a `file:line` or a source doc; if a cited anchor is wrong, the claim
built on it is void. Vendor/marketing/unverified figures are flagged inline. The
falsifiable seams are stated in §8 so the next red-team has a real surface to hit.

**What this doc answers (the brief):** the *vision* — the real-time live diagram
off the substrate; the plan **BEFORE/AFTER diff** and how the "after" is derived
(overlay / branch-worktree); the **interactive** diagram whose nodes/edges/findings
*fire off Claude* (ReactFlow click → socket/MCP → Claude → suggestion → re-render);
the MVP loop that ships first vs later; and the G0 / scope-law (suggest-never-mutate
leash) / feasibility alignment the red-team validated. This is the *why this is the
headline feature and how it is buildable on the real stack* — the planes themselves
are specified in the integration specs.

**Binding law (read before relitigating anything here):**
- **Scope law** — `.context/decisions/2026-06-11-core-competence-and-scope-line.md`
  (the three-layer ladder: derive REAL / infer-leashed MIXED / declare-and-diff;
  the leash, `:24-30`)
- **Goals** — `.context/GOALS.md` (G0 Speed, G2 Two-Binary split, G3 Agent-First,
  G4 Hexagonal — non-negotiable)
- **Rendering law** — `playbook/standards/view-standards.json` + quality gate
  `playbook/standards/MODEL-STANDARD.md` (the blind judge)
- **Prior research verdict** — `.context/details/2026-06-19-graphify-plus-mcp-research.md`
  (treated as established; sharpened here, not relitigated)
- **Strategic position** — `playbook/STRATEGIC-POSITION.md` §C (this doc is the
  expansion of that section's loop; it must not contradict it)

**Sibling enhancement docs:** `01-knowledge-graph-and-visualization.md` (the two
faces, the blind-judge moat), `03-access-surface.md` (the socket/CLI/web/MCP
ordering this loop rides), `04-scale-and-positioning.md`,
`05-redteam-alignment.md`.

---

## 0. CURRENT-STATE BANNER (read first — the rest is a buildable vision, not a shipped feature)

This is the most contingent feature in the playbook. State the gap honestly before
the vision, because every later section is written in the future tense and gated on
the keystone + arch surface.

**What ships today (verified against live source):**
- The `withETag`/304 **transport** — `withETag` returns 304 when `If-None-Match`
  matches the current revision string (`internal/adapters/web/server.go:157-167`),
  fed by a `revisionFn` revision source (`server.go:34`, set via
  `SetRevisionSource` `:49-52`). The recon/dashboard endpoints already ride it
  (`/api/recon*`, `server.go:107-110`).
- The **click-fires-an-action precedent** — `POST /api/recon-investigate`
  (`recon.go:555`) takes a UI click and mutates **annotation** via
  `SetFileInvestigated` (`recon.go:577`) — **never the substrate.** The
  suggest-never-mutate leash already has a live analog in the codebase.
- The 16-view ReactFlow/elkjs viewer — but as a **build-time generator**
  (`playbook/generators/build_blueprint_viewer.py`, `build_c4_mockup.py`), gated by
  the blind judge (`playbook/standards/MODEL-STANDARD.md`, `view-standards.json`).
  Not yet a daemon-served live endpoint.

**What is NOT built (all of §1–§5 is gated on these — verified absent):**
- **No `aoa arch` command, no `MethodArchFacts/Reach/Blast` socket methods.** Every
  loop step below *invokes* these in the future tense.
- **No import-edge keystone** — the always-on parse pass visits but never emits the
  edges the BEFORE diff is computed against (`integration/01-facts-substrate.md`).
- **No overlay loader** — Mode A's "rejects invented ids" loader is **net-new**
  (small, in-bounds, leash-clean, but not present).
- **No arch-shard web endpoint** — the viewer is the build-time generator, not a
  `withETag`-gated in-process shard producer.
- **The file-save→ETag tick DOES NOT EXIST.** `bumpRevision()` (`app.go:350`) has
  exactly four callers — `searchObserver` (`app.go:564`), `onSessionEvent`
  (`app.go:901`), `SetFileInvestigated` (`app.go:2896`), `ClearInvestigated`
  (`app.go:2905`). **None is `onFileChanged` (`watcher.go:20`) or `Reindex`
  (`app.go:2816`).** A code edit reindexes symbols but **does not bump the ETag** —
  the live viewer serves a stale 304 after a save until an unrelated search or
  Claude-session event fires. "Real-time on save" requires **one currently-absent
  line** (`onFileChanged`/`Reindex` must call `bumpRevision()`). The 304 *transport*
  ships; the file-change *trigger* does not.
- **No AG-UI** anywhere in the tree.

**The honest framing.** The interaction half of this feature is, in 2026, a *won,
commoditizing pattern* — tldraw "Agents on the Canvas" ([JSNation
2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw)),
Excalidraw+MCP ([mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw)),
Mermaid-as-living-contract ([erdembircan](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development)).
aOa does **not** invent the canvas, and this doc never claims it does. aOa's single
defensible delta is **grounding**: every shipping canvas grounds on the LLM's
reading of a screenshot / shape-data / prose / Mermaid-text; **aOa's BEFORE is
AST-derived and every node carries `file:line:commit`.** The grounded intersection
is empty; the ungrounded one is crowded. That delta is the whole feature.

---

## 1. The vision in one paragraph

> Claude now writes code faster than any human can keep the architecture coherent —
> Anthropic named the failure mode in 2026, "agentic technical debt" / "architectural
> drift" ([Dev|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/)).
> The remedy the same sources name is *written constraints for architectural
> direction* — and the gap they admit is that those constraints can't be
> machine-checked. The interactive before/after diagram is the **human face of that
> check**: you look at the live architecture of your repo, you ask Claude (in Plan
> Mode) for a refactor, and the diagram shows you the *AST-derived BEFORE* beside the
> *simulated AFTER your plan would produce* — new cycles, blast radius, new findings
> falling out of edge-set arithmetic, not an LLM's drawing. Then you click a node, an
> edge, or a finding, and that click hands Claude **the cycle's actual edges with
> `file:line:commit`** — Claude proposes, you dispose, the canvas re-renders the
> suggestion stamped `SIMULATED`. The agent never writes a node or edge into the REAL
> substrate; it writes a *proposed-edge overlay file* the deterministic renderer
> consumes. **You see the plan before and after, and the diagram fires Claude — on a
> fresh, provenance-stamped substrate no LLM-drawn canvas can produce.**

This is the headline feature because it is the one natively viral artifact in the
whole product (§7) and the one face a competitor structurally can't copy without
the deterministic substrate underneath it. It is also the most contingent — it
arrives in Phase 2, gated on the keystone, the overlay loader, and the one absent
`bumpRevision()` line (§0).

---

## 2. REAL-TIME — what ships vs what's net-new (the corrected mechanism)

The "live diagram" is the easiest claim to overstate. Here is exactly what's there
and what isn't.

**Ships today: the 304 transport.** `withETag` (`server.go:157-167`) computes an
ETag from `revisionFn()` and returns `304 Not Modified` when the client's
`If-None-Match` matches — the auto-refreshing canvas polls and gets an empty-body
304 until the revision changes. This is real and reusable; the arch endpoint would
ride it exactly as `/api/recon*` already does (`server.go:107-110`).

**Does NOT ship: the file-change → revision trigger.** The viewer can only
"refresh on save" if a save *bumps the revision*. It does not. `bumpRevision()`
(`app.go:350`) is called by four paths — none of them the file-watch path
(`watcher.go:20` `onFileChanged`, `app.go:2816` `Reindex`) (§0). So today a code
edit reindexes symbols but leaves the ETag unchanged; the live viewer serves a
stale 304 after a save until an unrelated search or Claude-session event happens to
fire. **The "real-time on save" viewer is one currently-absent line.** Do not claim
"refreshes on every keystroke" for the diagram until that line lands.

**Net-new, small, in-bounds (the real-time work, in order):**
1. **One line:** `onFileChanged`/`Reindex` calls `bumpRevision()` so a save
   invalidates the viewer's ETag on the same tick the index updates (§0).
2. **An in-process arch-shard producer** served through the existing `withETag`
   middleware — the viewer becomes a daemon-served endpoint, not the build-time
   Python generator (`playbook/generators/build_blueprint_viewer.py`).
3. **Per-scope ETag** so editing `pkg/foo` doesn't invalidate `pkg/bar`'s shard —
   bounded recompute on a large estate (the thrash bound, §8.7).

**The shared-freshness payoff (post-keystone).** Once the keystone edges ride the
`fsnotify → reindex` path and that path bumps the revision, the agent's sub-ms
socket answer and the human's ETag-polled canvas invalidate **on one tick** — the
two faces (`01-knowledge-graph-and-visualization.md` §1.2) cannot drift, because
there is one freshness signal. **Concede the seam honestly:** "live" here is
*recompute + ETag poll*, not streaming, and the file-change trigger is absent until
item 1 lands. The moat is airtight for the **CLI/socket answer** (sub-ms off the
live in-memory index); **near-live, and currently un-wired,** for the viewer.

---

## 3. BEFORE/AFTER — the plan diff, derived not drawn (three modes, ship in order)

The before/after is the load-bearing idea. **The diff is un-fakeable only if the
BEFORE is *derived*, not drawn.** A competitor without a deterministic substrate
can only redraw two pictures; aOa diffs two SHA-snapshot edge-sets and the delta —
new cycles, affected closure, new findings — falls out of *set arithmetic* over the
keystone edges. No LLM pass, no doc to maintain.

How the "AFTER" is produced has three modes, shipped in order:

| Mode | What | How "AFTER" is derived | Provenance | Ships when |
|---|---|---|---|---|
| **A — proposed-edge overlay** (MVP, leash-native) | Claude emits a JSON patch of edges; every endpoint is an **id already in the facts**; the (net-new) overlay loader rejects invented ids; the renderer re-runs the same deterministic detectors (cycles/blast/DSM) over the hypothetical edge-set | overlay = REAL facts + agent-proposed edges between them, run through the same detectors | `SIMULATED · proposed` | **First** — needs the **net-new overlay loader** (pure graph algebra, no network, leash-clean) + the detectors; G0 holds |
| **B — branch/worktree re-derive** (high fidelity) | The plan is written to a branch/worktree; aOa re-derives REAL edges from that tree's AST; BEFORE = `HEAD`, AFTER = branch, same extractor on both | both sides DERIVED from real source at two SHAs | `REAL · derived @ branch-sha` | After git-worktree wiring — the un-fakeable upgrade |
| **C — autonomous worktree** (later) | Claude creates the worktree, applies the plan itself, aOa re-derives — an automated Mode B | as B, but the apply step is the agent's | REAL | Only after the leash boundary is battle-tested — the **gimmick frontier the user warned against** |

**Recommendation (Advisory Rule, G3-aligned):** ship **Mode A first.** It is
leash-native (the overlay never touches the substrate), needs no git plumbing, and
proves the diff with the cheapest possible mechanism. Mode B is the credibility
upgrade — when the AFTER is *also* derived from real source, the diff is
fully un-fakeable. **Do not lead with Mode C.** "Click → Claude autonomously
rewrites your code" is the shiny-object trap (§8.10); it breaks the leash if rushed.

**What the diff computes** (Microsoft's Architecture Review Agent feature list,
computed over derived facts instead of an LLM's reading of a brain-dump): new/removed
cycles, blast radius of the delta, god-node shift, new findings. The **blind-judge
gate** (`MODEL-STANDARD.md`, `view-standards.json`) is a falsifiable acceptance
test no canvas-SDK ships — an LLM-drawn diagram that admits to hallucinating can't
reliably pass it, which is exactly why aOa's AFTER (which is *derived*, even in
Mode A) can.

**Conceded seam (the most-contingent thing in the doc).** The diff renderer is a
**Phase-2 target, not shipped.** It depends on the keystone (≤+3%, G0) *and* the
net-new overlay loader (§0). The strongest adoption artifact arrives in Phase 2,
not at launch — §7 sequences growth-spend accordingly and does **not** front-load a
launch on it.

---

## 4. INTERACTIVE — the click that fires Claude (built on an existing precedent)

The interaction model is: a node, edge, or finding in the ReactFlow canvas is
clickable; the click gathers *grounded context* from the substrate and hands it to
Claude; Claude returns a *suggestion*; the diff renderer computes BEFORE/AFTER and
the canvas re-renders the AFTER stamped `SIMULATED`.

**This rides a precedent that already exists.** `POST /api/recon-investigate`
(`recon.go:555`) already takes a `{file, action}` from a UI click and acts on it —
it mutates **annotation** via `SetFileInvestigated` (`recon.go:577`), **never the
substrate.** Track B adds sibling POSTs in the same shape. The loop (future tense —
the `MethodArch*` methods and the `/api/arch/*` routes are net-new, §0):

```
ReactFlow onNodeClick(node)  |  onEdgeClick(edge)  |  onFindingClick(f)
  → POST /api/arch/suggest {subject, kind, scope}
      → socket method (MethodArchFacts / Reach / Blast — NET-NEW, rides server.go:207 switch)
      → returns GROUNDED context: the cycle's edges, each with file:line:commit
  → grounded fact-pack → Claude
      (Claude runs OUTSIDE the service: a CLI subprocess, or an MCP tool result;
       Opus 4.8, thinking:{type:"adaptive"}, the thin-MCP shape of 03-access-surface §5)
      → Claude returns a SUGGESTION as a proposed-edge overlay (Mode A)
  → overlay loader rejects invented ids; diff renderer computes BEFORE/AFTER
  → viewer re-renders AFTER, stamped SIMULATED · proposed
```

**The difference from every competitor, stated precisely:** the click hands Claude
**the cycle's actual edges with `file:line:commit`** — not "a node labeled
`AuthService`." tldraw's own `BlurryShape`/`FocusedShape` are *simplified shape
summaries* fed to the model for "understanding"; aOa's payload is the audit trail
itself (`aoa arch facts <subject>`, `03-access-surface.md` §6). The interaction
tech is mature and borrowable ("a few hours of React"); the defensible position
"is not the click — it's that clicking a node lands on a fresh, provenance-stamped
fact."

**Where Claude runs (and why it never runs inside the service).** Claude is invoked
*outside* the deterministic derive path — as a CLI subprocess the daemon shells out
to, or by an MCP host (`03-access-surface.md` §3, MCP last and thin) — so **G0 (no
network on any derive path) is intact.** Claude produces a file (the overlay); the
deterministic renderer consumes it. The model call is in the *interaction* layer,
never in the *fact* layer. (Provider note, per the Claude-loop nature of this
feature: the suggestion call targets Claude Opus 4.8 with adaptive thinking —
`thinking:{type:"adaptive"}` — and the thin-MCP tool surface of `03-access-surface`
§5, never a 40-tool retrieval index; this is a vision doc, so no code is prescribed.)

**Surfaces to ride (corrected current-state):**
1. **The localhost dashboard** (`server.go`) — **ships today** as the host for the
   net-new `/api/arch/*` routes. **AG-UI streaming
   (`STATE_DELTA`/`TOOL_CALL_START`) is a NET-NEW adapter, NOT present** (§0).
   AG-UI / A2UI / MCP-Apps are the correct, adopted 2026 standards to target (AWS
   Bedrock AgentCore Mar 2026; SEP-1865 ratified 2026-01-26 under the Linux
   Foundation) — aOa's viewer **does not yet emit AG-UI events; this is the intended
   integration**, gated by the unverified rendering-fidelity risk (§8.9). Use A2UI's
   "trusted catalog" framing so the agent picks from aOa's **judged** view types —
   preserving the blind-judge gate.
2. **MCP App inside Claude** (a `ui://` ReactFlow viewer) — lower-regret long bet;
   per-host rendering fidelity *unverified* (§8.9). Built last and thin, exactly as
   `03-access-surface.md` §3 scopes MCP — reach, not speed.

---

## 5. The leash holds — suggest-never-mutate (the part the red-team pushes hardest)

This is the one part of the loop with a live analog, so it is the most defensible.

The agent **never writes a node or edge into the REAL substrate.** It writes a
**proposed-edge overlay file**; the (net-new) overlay loader **rejects any id not in
the facts**; applying an overlay drops the view to MIXED (or the AFTER to SIMULATED)
— never REAL. This is the scope-law leash verbatim
(`2026-06-11-core-competence-and-scope-line.md:24-30`: *"the agent may
name/group/annotate extracted facts, NEVER add a node… anything the agent adds
drops the view to MIXED; an agent-only artifact is SIMULATED-with-source"*). The
"/edge" corollary is the project's own (an agent adding an edge invents structure
the code doesn't contain — out of bounds for the same reason a node is;
`01-knowledge-graph-and-visualization.md` §2.1).

**The same guarantee `recon-investigate` already honors.** That endpoint mutates
*annotation* (`SetFileInvestigated`, `recon.go:577`), never the index — the leash
precedent is real, verified against live source, and Track B is built in its exact
shape.

**No LLM call happens inside the service.** Claude runs in the interaction layer
(§4); the deterministic renderer consumes the file it produces; **G0 is intact.**

**Falsifiable leash test:** if any Track-B path lets agent output appear in a
REAL-stamped view, the design is broken — by construction it cannot. The shareable
artifact is the **decision** (BEFORE vs AST-derived AFTER) — viral loop and
leash-safe boundary at once. The agent proposes; the human disposes.

---

## 6. The MVP loop — what ships first vs later

The whole loop is Phase 2+, but it decomposes into a minimal first cut and an
upgrade path. Ship the smallest thing that proves the spine.

| Cut | Components | Gated on | Why this order |
|---|---|---|---|
| **MVP — the diff, static** | keystone edges → live component/DSM/cycles shard served through `withETag` + the one `bumpRevision()` line; **Mode A overlay** (Claude in Plan Mode emits a proposed-edge patch → overlay loader → diff renderer); side-by-side BEFORE/AFTER in the viewer | keystone (§0); overlay loader (§0); the absent `bumpRevision()` line (§2) | Proves "derived BEFORE vs simulated AFTER" with **no click-fires-Claude wiring** and **no AG-UI** — the cheapest demonstration of the un-fakeable diff |
| **+1 — the click fires Claude** | `onNodeClick/onEdgeClick/onFindingClick` → `POST /api/arch/suggest` → `MethodArch*` → grounded fact-pack → Claude → overlay → re-render | the MVP, plus `aoa arch` + `MethodArch*` (`03-access-surface.md` §6) | Adds the interaction precedent (`recon.go:555` shape); turns a static diff into a live loop |
| **+2 — streaming + MCP App** | AG-UI `STATE_DELTA`/`TOOL_CALL_START` adapter; the `ui://` MCP App inside Claude | the +1 loop; AG-UI rendering-fidelity unblocked (§8.9) | Reach (MCP-only hosts) and live-streaming UX — last and thin, never the hot path |
| **+3 — Mode B branch re-derive** | git-worktree wiring; BEFORE=`HEAD`, AFTER=branch, same extractor | worktree plumbing | The credibility upgrade: AFTER is now *also* derived, fully un-fakeable |

**Mode C (autonomous worktree apply) is explicitly out of the MVP ladder** — it is
gated behind a battle-tested leash boundary and named as the gimmick frontier
(§8.10). Do not let it onto the roadmap as anything but a guarded later option.

---

## 7. Why this is the headline feature — the growth + founder case

**Growth (the one natively viral artifact).** Every competitor fakes "before/after"
as two hand-drawn pictures and "quality" as eyeballing. aOa's before/after plan diff
is the **one share-worthy decision artifact teammates argue over** — and it closes a
real install loop: a teammate who receives a plan-diff link must run `aoa init` to
*click-to-investigate*, because the suggestion is computed against *their* local
fresh substrate (a non-user sees a static PNG but cannot interact). That
recipient-must-init mechanic is the install loop the static-PNG diagram tools lack;
the single instrumented metric that proves it closes is
`init → second-session → teammate-init`. **Sequencing law:** this loop is Phase 2
(it depends on the diff renderer); until then growth is wedge-driven by the
already-shipping grep→peek CLI, **not** loop-driven by the unbuilt diagram. Do not
front-load a launch on the diff.

**Founder (the demand→product arrow).** Anthropic named the failure (agentic tech
debt) and named the remedy (written constraints for *direction*; memory tools give
*recall*, which already works) ([Dev|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/)).
Constraints written in prose can't be machine-enforced. The interactive before/after
canvas is the **human face of conformance** — declared-pattern-vs-derived-actual
diff — the automated enforcement layer those constraints need. The canvas is a won
2026 pattern aOa *rides*; the defensible novelty is the **grounding**, not the click.

**The leading-edge / not-gimmick line.** The demand is named and dated; the
interaction tech is mature and borrowable; the gimmick risk is Mode C overreach
(§8.10). aOa stays on the **SUGGEST** side of the leash (§5) — the agent proposes,
the human disposes — which is simultaneously the viral artifact and the safe
boundary. **§C never claims aOa invented the loop** (`STRATEGIC-POSITION.md` §C).

---

## 8. Red-team targets (swing here first)

1. **The whole feature is keystone-gated (§0).** Every real-time/diff/click-fires
   claim is roadmap until import edges ship within G0 ≤+3% *and* the overlay loader
   *and* the absent `bumpRevision()` line land. *Defense:* the grep→peek CLI +
   judged viewer carry Phase-0 adoption with zero new code; this loop is the Phase-2
   payoff, honestly staged.
2. **"Real-time on save" was false and is now corrected.** `bumpRevision()` has four
   callers, none on the file-change path (`app.go:564/901/2896/2905`); the 304
   transport ships, the file-change trigger does not (§2). The claim is "one absent
   line," not "live today."
3. **The interactive canvas is a crowded, won pattern** (tldraw/Excalidraw/Mermaid).
   aOa's only delta is AST-grounding. *Attack:* "the grounding edge is thin if a
   competitor wires their canvas to a real index." *Defense:* they'd still lack the
   deterministic provenance-stamped BEFORE and the blind-judge gate — the grounded
   intersection is empty.
4. **The diff is the most-contingent artifact** — Phase-2, depends on keystone +
   overlay loader. The strongest adoption artifact arrives latest; growth spend must
   not front-load on it (§7).
5. **Mode A's overlay loader is net-new, not "reused."** It is small and leash-clean,
   but it is not present (§0). Don't describe it as a reuse of an existing component.
6. **Where does Claude run, exactly?** If any reader thinks the model runs *inside*
   the derive path, G0 is violated. It does not — Claude is in the interaction layer
   (CLI subprocess / MCP host), the renderer is deterministic (§4, §5). State it
   every time.
7. **Real-time over a big estate could thrash.** Bounded by ETag 304-empty-body +
   per-scope invalidation (net-new, §2 item 3); whole-estate interactive views
   deferred. Unproven at scale.
8. **Host-coupling.** The recon/live-status signal that enriches the canvas is bound
   to Claude Code session logs (`~/.claude/projects/*.jsonl`) — a Claude Code
   beachhead, not unbounded TAM. The diff/overlay loop itself is host-agnostic
   (any agent that can emit a JSON overlay), but the *recon-weighted* findings are
   Claude-Code-bound.
9. **MCP App / AG-UI rendering fidelity unverified** — whether a full ReactFlow/elkjs
   bundle renders cleanly inside Claude's iframe at hundreds of nodes is untested.
   The +2 cut is gated on this.
10. **Mode C is the shiny-object trap** — "autonomous worktree rewrite" breaks the
    leash if rushed; keep it gated behind a battle-tested boundary, never in the MVP
    ladder (§3, §6).

---

## Appendix — falsifiable anchor index (red-team this list first)

| Claim | Anchor |
|---|---|
| 304 transport ships (`withETag` returns 304 on `If-None-Match` match) | `internal/adapters/web/server.go:157-167`; `revisionFn` `:34`, `SetRevisionSource` `:49-52`; recon endpoints ride it `:107-110` |
| Click-fires-an-action precedent mutates annotation, never substrate | `internal/adapters/web/recon.go:555` (`POST /api/recon-investigate`), `:577` (`SetFileInvestigated`) |
| File-save→ETag tick is ABSENT — `bumpRevision` not on the file-change path | `internal/app/app.go:350` (`bumpRevision`); callers `:564/:901/:2896/:2905`; NOT `watcher.go:20` (`onFileChanged`) or `app.go:2816` (`Reindex`) |
| Viewer is a build-time generator, not a live endpoint | `playbook/generators/build_blueprint_viewer.py`, `build_c4_mockup.py` |
| Blind-judge gate (the falsifiable visual acceptance test) | `playbook/standards/MODEL-STANDARD.md`; `playbook/standards/view-standards.json` |
| Leash: agent may name/group, NEVER add a node (→ "/edge" corollary) | ADR `2026-06-11-core-competence-and-scope-line.md:24-30` ("never add a node"); "/edge" is the §2.1 corollary, carried by `integration/03-visualization.md:343` |
| Manifest → shard → viewer contract = agent query result | `01-knowledge-graph-and-visualization.md` §1; `integration/00-OVERVIEW.md:90-94` |
| Socket method switch (`MethodArch*` rides alongside as a one-line `case`) | `internal/adapters/socket/server.go:207` (`switch req.Method`) — per `03-access-surface.md` §2.2 |
| `aoa arch facts <subject>` is the grounded fact-pack the click hands Claude | `03-access-surface.md` §6 |
| MCP last and thin — reach not speed | `03-access-surface.md` §3, §7 |
| Keystone import edges on the always-on parse pass, G0 ≤+3% | `integration/01-facts-substrate.md`; `integration/00-OVERVIEW.md` (≤+3%) |
| `aoa arch` / `MethodArch*` / overlay loader / arch-shard endpoint / AG-UI all ABSENT | verified absent in tree (§0); `STRATEGIC-POSITION.md` §0 |
| Interactive agent-canvas is a won 2026 pattern (aOa rides, doesn't invent) | [tldraw JSNation 2026](https://gitnation.com/contents/agents-on-the-canvas-with-tldraw); [mcp_excalidraw](https://github.com/yctimlin/mcp_excalidraw); [Mermaid living-contract](https://erdembircan.github.io/blog/mermaid-flowcharts-agentic-development) |
| Anthropic named "agentic technical debt"; remedy = written constraints for direction | [Dev\|Journal](https://earezki.com/ai-news/2026-06-02-anthropic-gave-the-failure-mode-i-kept-hitting-with-claude-code-a-name-agentic-technical-debt/) |
| Loop must not contradict the strategic position | `playbook/STRATEGIC-POSITION.md` §C |

---

**Provenance note.** This document is the enhancement-pool expansion of
`STRATEGIC-POSITION.md` §C (the interactive before/after loop), curated to the real
stack: it reproduces that document's verified anchors and adds the MVP-ladder,
three-mode AFTER-derivation, and click-fires-Claude wiring detail. Every aOa
`file:line` anchor was checked against live source before writing — most
consequentially, the feasibility blocker is confirmed true: `bumpRevision()` has
exactly four callers, none on the file-change path, so "real-time on save" is one
absent line, not a shipped property. No code or source files were edited — markdown
only.
