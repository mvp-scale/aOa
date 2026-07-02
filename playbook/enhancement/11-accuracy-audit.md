# 11 — The Accuracy Audit

> **The trust ledger for the enhancement pool.** The pool stakes its own credibility on
> falsifiability: *"every load-bearing claim cites a file:line or URL; if a cited anchor is
> wrong, the claim built on it is void."* This document holds the pool to exactly that rule.
> Every entry below was personally re-verified against live source on branch `playbook`
> (date 2026-06-19) or against a primary external source — not inherited from a prior red-team.
>
> Markdown only. No source-code change was made or proposed. This is an audit of the docs,
> not of the code.

---

## A. Overall verdict — MINOR_FIXES

The enhancement pool is **substantially accurate**. Every load-bearing Go code anchor that
carries an argument verifies against live source, and the high-stakes external facts are
confirmed against primary sources. The defects are a **contained set** of two kinds:

1. **Off-by-one / wrong-path code anchors** (a doc-comment line cited instead of the `func`
   line; a truncated interface range; a wrong package path). These do not void the underlying
   claims — the symbols and behaviors are real — but under the pool's own rule the *anchors*
   must be corrected.
2. **A handful of wrong external anchors** (one arXiv ID reused for an unrelated paper; a
   "defunct" framing for a company that was actually acquired; a mis-stated statistic).

The **single material integrity issue** is doc10's round-2 synthesis (§ below): it cites a
`STRATEGIC-POSITION.md` "§F.4" that does not exist and attributes named comps/labels to
specific SP line numbers whose text does not contain those strings. Under the pool's
falsifiability rule those anchors are **void** — even though the adjacent SP content is
thematically related. The fix is to relabel them as doc10's own synthesis, not to delete the
reasoning.

**A correction to the audit brief itself:** the brief's premise that `STRATEGIC-POSITION.md` is
a *pre-round-2 draft* is **FALSE**. SP line 1 reads `# aOa — Integrated Strategic Position
(DRAFT, round-2 revision)` and SP line 5 carries the `Round-2 changelog`. SP *is* the round-2
document and carries the round-2 spine fixes. The real problem is narrower and the opposite
shape: doc10 invents SP `§F.4` anchors and pins doc10-only labels to SP line numbers — see § E.

---

## B. Methodology

Six independent verification passes, then adjudication:

| Pass | Scope | Method |
|------|-------|--------|
| Code verifier ×3 | Every `file:line`, symbol name, and claim about aOa Go behavior | Semantic `grep`/`egrep` over branch `playbook` + exact-line `Read`. Three independent passes; a claim is CONFIRMED only on agreement. |
| External / library ×2 | Every URL, competitor fact, funding/market figure, library-capability claim | `WebSearch`/`WebFetch` to primary sources; library/framework capability claims cross-checked. Two passes; vendor/marketing figures flagged. |
| Consistency ×1 | Cross-doc anchors (esp. the many `STRATEGIC-POSITION.md:NN` references), internal contradictions, and conflicts with the binding law (`GOALS.md` G0–G6, the scope-line ADR, `MODEL-STANDARD.md`) | `grep` of SP for every attributed string; line-by-line check of cited SP bodies. |
| Adjudication ×1 | Reconcile the passes into one ledger | A claim is **CONFIRMED** only if personally verified; **WRONG** if the anchor/fact is false (with the corrected value); **UNVERIFIABLE** if it cannot be checked; **INCONSISTENT** if it conflicts with another claim or the binding law. |

**Verdict rule (the pool's own):** a wrong anchor voids the claim built on it. Where the
*underlying claim* survives (the symbol/behavior is real, only the line/path is off), the ledger
says so explicitly and supplies the corrected anchor.

---

## C. What verified clean (the load-bearing spine)

These were re-verified against live source on branch `playbook` and are **CONFIRMED**. They are
the anchors the pool's arguments hang on, so their correctness is the reason the overall verdict
is MINOR_FIXES rather than worse.

| Claim | Anchor | Verified |
|-------|--------|----------|
| `bumpRevision` definition | `internal/app/app.go:350` (`func (a *App) bumpRevision() { a.revision.Add(1) }`) | ✅ |
| `bumpRevision` four callers | `app.go:564`, `:901`, `:2896`, `:2905` | ✅ all four |
| `Symbol` struct has **no edge field** | `treesitter/parser.go:20` — fields are `Name, Signature, Kind, StartLine, EndLine, Parent` | ✅ |
| `extractSymbols` + 3 tuned extractors | `treesitter/parser.go:235` (dispatch); extractors are Go/Python/JavaScript + generic | ✅ |
| socket `handleRequest` switch | `socket/server.go:206` (`func`), `:207` (`switch req.Method`) | ✅ |
| `withETag` / 304 transport | `web/server.go:156-170` (304 on `If-None-Match` match at `:166-168`) | ✅ |
| recon POST handler precedent | `web/recon.go:555` (`handleReconInvestigate`), `:577` (`SetFileInvestigated`) | ✅ |
| `Storage` index three maps | `ports/storage.go` `Index` struct (`Tokens`/… maps) | ✅ |
| ten `langOverrides` | `internal/domain/analyzer/lang_map.go:48` map literal — **ten** keys (go, python, js, ts, tsx, rust, java, c, cpp, ruby) | ✅ |
| forest = 509 languages | `treesitter/languages_forest.go:5` (`// Languages: 509`) | ✅ |
| scope-law leash precedent | recon POST/leash pattern at `web/recon.go` | ✅ |
| MODEL-STANDARD blind-judge gate | `playbook/standards/MODEL-STANDARD.md:43-53` (judge receives image + question + pass criterion only) | ✅ |
| `onFileChanged` is **not** the hot-reload anchor doc06 cited | lives at `internal/app/watcher.go:20`, **not** `fsnotify/watcher.go:20` (see § D) | ✅ (anchor was wrong) |
| `countImportSpecs` returns `int`, parse pass only counts imports | `treesitter/walker.go:568` (func), called at `:483` | ✅ (claim true; line off-by-one, see § E) |

**High-stakes external facts — CONFIRMED against primary sources** (re-verified this pass where
load-bearing; the remainder confirmed in the source material and not contradicted here): Potpie
$2.2M; Glean $7.2B valuation / $200M ARR; Blackbird; Copilot billing; **MCP Apps SEP-1865**;
**CVE-2025-6514**; Moderne $30M; "Architecture as Code" (Ford & Richards, O'Reilly 2026);
arXiv 2603.27277 and 2603.10060. The **Kosli $3.1M seed led by Heavybit** is real (see § E for
its date problem). The **CodeSee → GitKraken** event is real (see § D for its framing problem).

---

## D. Corrected-claims ledger — wrong external / wrong-path anchors

### 06-competitive-landscape.md

| Line | Status | Claim as written | Correction | Evidence |
|------|--------|------------------|------------|----------|
| 9 | **WRONG** | hot-reload anchor `internal/adapters/fsnotify/watcher.go:20` | `internal/app/watcher.go:20` | `internal/app/watcher.go:20` = `func (a *App) onFileChanged`; `fsnotify/watcher.go:20` = `".venv": true` ignore-dirs entry — an unrelated construct |
| 134 | **WRONG** | CodeSee **"Defunct — wound down Feb 2024, → GitKraken"** | **Acquired** by GitKraken, announced **May 14 2024**, folded into its DevEx platform | gitkraken.com / prnewswire — "GitKraken Acquires CodeSee; Launches DevEx Platform," May 14 2024. The cited GitKraken URL is an *acquisition* announcement, not a shutdown. Both the date and the "wound down" framing are wrong; the acquisition is real. |
| 137 | **WRONG** | LLM-Mermaid "Achilles heel" cites `arXiv 2512.02170` | drop the 2512.02170 citation; keep the [GenAIScript](https://microsoft.github.io/genaiscript/blog/mermaids/) URL | arXiv 2512.02170 = **"Flowchart2Mermaid: A VLM-Powered System for Converting Flowcharts into Editable Diagram Code"** (Deka & Devereux, QUB) — an *image-to-Mermaid via VLM* system. It does **not** study LLM-Mermaid syntax-failure / semantic-drift / LLM-as-judge. Anchor void. |

### 07-five-moats.md

| Line | Status | Claim as written | Correction | Evidence |
|------|--------|------------------|------------|----------|
| 40 | **WRONG** | no-import-edge keystone cites `internal/domain/index/parser.go` | `internal/adapters/treesitter/parser.go` | `internal/domain/index/parser.go` **does not exist** (`ls` → no such file); `extractSymbols` lives at `treesitter/parser.go:235` |
| 484 | **WRONG** | absence-checklist cites `internal/domain/index/parser.go` | `internal/adapters/treesitter/parser.go` | same — no such file; treesitter parser is authoritative |
| 303 | **WRONG** | "deep-research agents show **11-57% citation-hallucination**" (arXiv 2605.06635) | the paper reports **39-77% factual accuracy** across systems and a **~42% average accuracy drop** as tool calls scale 2→150 — **not "11-57% citation-hallucination"** | arXiv 2605.06635 "Cited but Not Verified" (PwC). The 11-57% figure does not appear in the paper. (Its related-work cites a separate 3-13% hallucinated-URL / 5-18% non-resolving range — also not 11-57%.) |

### 09-growth-and-founder-positioning.md

| Lines | Status | Claim as written | Correction | Evidence |
|-------|--------|------------------|------------|----------|
| 162, 231, 595 | **WRONG** | "LLM-diagram-from-code cohort … did not convert to retained use" cites `arXiv 2512.02170` (3×) | remove the 2512.02170 citation — it points to the unrelated **Flowchart2Mermaid** (image-to-Mermaid via VLM) paper; cite the GitDiagram/DeepWiki *viral-but-unretained* evidence instead | arXiv 2512.02170 = Flowchart2Mermaid (Deka & Devereux); unrelated to LLM-diagram retention. Same wrong-paper anchor as 06:137. |

### 04-scale-and-positioning.md

| Line | Status | Claim as written | Correction | Evidence |
|------|--------|------------------|------------|----------|
| 230 | **WRONG** | "Only 3 of its **10** MCP tools beat grep" | "Only 3 of its **~7** MCP tools beat grep" | `github.com/safishamsi/graphify` MCP server enumerates ~7 tools (v8 main: `query_graph, get_node, get_neighbors, shortest_path, list_prs, get_pr_impact, triage_prs`; the skill.md variant lists a different ~7: `…get_community, god_nodes, graph_stats…`). The "10" count is not corroborated by the current repo on either pass. |

---

## E. Corrected-claims ledger — wrong / unverifiable code anchors and the SP synthesis

### 02-integration-touchpoints.md — truncated `Storage` interface range

| Line | Status | Claim as written | Correction | Evidence |
|------|--------|------------------|------------|----------|
| 270 | **WRONG** | `storage.go:12-45`, "which ends at the Telemetry methods" | `storage.go:12-56` | `ports/storage.go`: interface **closes at `:56`**; `LoadTelemetry` runs `:51-55`. Line 45 is mid-signature of `SaveSessionWithTelemetry` — so `:12-45` truncates mid-interface and contradicts the prose "ends at the Telemetry methods." |
| 368 | **WRONG** | off-interface caveat row cites `storage.go:12-45` | `storage.go:12-56` | same — interface ends at `:56`, not `:45` |

### 03-access-surface.md — G0 off-by-one

| Line | Status | Claim as written | Correction | Evidence |
|------|--------|------------------|------------|----------|
| 83 | **WRONG** | "the sub-ms read path G0 (`GOALS.md:8`)" | `GOALS.md:7` | `.context/GOALS.md:7` = **G0 (Speed)**; `:8` = **G1 (Parity)**. The anchor pointed at the wrong goal. |
| 310 | **WRONG** | anchor-table row "G0 … `.context/GOALS.md:8`" | `.context/GOALS.md:7` | same off-by-one |

### 08-interactive-diagrams-and-claude-loop.md — short recon-route range

| Line | Status | Claim as written | Correction | Evidence |
|------|--------|------------------|------------|----------|
| 49 | **WRONG** | `/api/recon*`, `server.go:107-109` | `server.go:107-110` | recon GET family spans **four** routes `:107-110` (`/api/recon`, `/recon/summary`, `/recon/tree`, `/recon/findings`); `:107-109` drops `/api/recon/findings` at `:110` |
| 128 | **WRONG** | "ride it exactly as `/api/recon*` … `server.go:107-109`" | `server.go:107-110` | same — four routes `:107-110` |
| 384 | **WRONG** | 304-table row "recon endpoints ride it `:107-109`" | `:107-110` | same — four routes `:107-110` (note: the same row's `withETag` anchor `:157-167` and `revisionFn :34` were checked and are within range — only the recon sub-anchor is short) |

### 10-moat-redteam-validation.md — countImportSpecs off-by-one + the SP synthesis problem

**Code anchors (both off-by-one — claim true, line wrong):**

| Line | Status | Claim as written | Correction | Evidence |
|------|--------|------------------|------------|----------|
| 304 | **WRONG** | anchor-table row `walker.go:567 countImportSpecs` | `walker.go:568` | `treesitter/walker.go:567` = the doc-comment; `:568` = `func (ctx *walkContext) countImportSpecs(…) int`. (The companion `parser.go:20` no-edge `Symbol` and `parser.go:235` 3-extractors anchors in the same row are **correct**.) |
| 173 | **WRONG** | narrative "the parse pass only *counts* imports — `walker.go:567`" | `walker.go:568` | same off-by-one |

**The SP synthesis problem — the single material integrity issue.** Doc10's round-2 lens (rows
G2.5, F2.1–F2.6, and the § narrative at line 71) cites `STRATEGIC-POSITION.md` for a set of
*named labels and comps that are doc10's own synthesis and do not appear as strings in SP*. A
`grep` of `STRATEGIC-POSITION.md` returns **zero** hits for: `Snyk`, `§F.4`, `land-and-expand`,
`Kosli`, `Heavybit`, `Christensen`, `data-plane-shape`, `window closes`, `contested NOW`,
`option value`. The cited SP line bodies are *thematically adjacent* — but under the pool's own
falsifiability rule, an anchor whose target text does not contain the claimed string is **void**.
The fix is to relabel each as doc10 synthesis riding adjacent SP content, not to delete the
reasoning.

| Line / row | Status | What it claims SP says | What SP actually says at that anchor | Evidence |
|------------|--------|------------------------|--------------------------------------|----------|
| 154 (G2.5) | **WRONG** | "**§F.4** names the tension … the Snyk shape … land-and-expand (`:307`)" | SP has **no §F.4** (§F is a flat list 1-10, header `:302`). `SP:307` = red-team **item 4** (the "agent tool with governance upsell vs governance tool with agent funnel" question) — contains no "Snyk"/"land-and-expand" | grep: no `Snyk`/`§F.4`/`land-and-expand`; `SP:302` header; `SP:307` = item 4 |
| 160 (F2.1) | **WRONG** | "The **Kosli/Heavybit $3.1M** continuous-compliance seed … (`:35`, `:178`)" | `SP:35` = the symmetric-concession paragraph; `SP:178` = defensibility-of-the-set. Neither names Kosli/Heavybit | grep: no `Kosli`/`Heavybit` anywhere in SP |
| 161 (F2.2) | **WRONG** | "data-plane-shape asymmetry … the **Christensen** asymmetry (`:84-90`) … demoted (`:305`)" | `SP:84-90` = Moat-A freshness body; `SP:305` = red-team item 2 ("Provenance (Moat D) is softening"). Neither contains "Christensen"/"data-plane-shape" | grep: no `Christensen`/`data-plane-shape`; SP bodies confirmed |
| 163 (F2.4) | **WRONG** | "D.3 sharpens to '…contested NOW…'; '**the window closes** …' (`:261-265`)" | `SP:261-265` = section **D.3** (founder why-now: four de-risking facts + the demand→product arrow). Contains no "window closes"/"contested NOW" | grep: no `window closes`/`contested NOW`/`being contested` |
| 164 (F2.5) | **WRONG** | "**§F.4 commits to the sequence** … the Snyk comp set (`:307`)" | no §F.4; `SP:307` = red-team item 4, no "Snyk" string | grep: no `§F.4`/`Snyk`; `SP:307` = item 4 |
| 165 (F2.6) | **WRONG** | recon "demoted toward **option value** (`:160-164`)" | `SP:160-164` = Moat-E body; `SP:164` actually reads recon is "a **retention/diligence moat, not a wedge**" that "**cannot drive adoption now**" — not "option value" | grep: no `option value`; `SP:164` quoted verbatim |
| 71 (narrative) | **UNVERIFIABLE** | "the Kosli/Heavybit continuous-compliance comp proves VCs underwrite this *before* product" | The Kosli/Heavybit **$3.1M seed led by Heavybit is real but dated Nov 2 2022**, not 2026, and **no URL is cited in the pool** for it. Soften or source it. | prnewswire/devopsdigest confirm $3.1M Heavybit-led seed, announced **Nov 2 2022** |

**Recommended doc10 fix pattern** (preserves the argument, satisfies the rule): keep each
SP line anchor for the *adjacent* content it really supports, then add an explicit
"(this doc's synthesis label, not an SP string)" qualifier for `Snyk`, `Christensen`,
`land-and-expand`, `option value`, `window closes`, and the `Kosli/Heavybit` comp; and replace
every `§F.4` with `§F red-team item 4 (:307)`.

### ENHANCEMENT-GUIDE.md — SupportedLanguages off-by-one

| Anchor | Status | Claim as written | Correction | Evidence |
|--------|--------|------------------|------------|----------|
| `lang_map.go:173` (lines 345, 778) | **INCONSISTENT** | `SupportedLanguages()` returns those keys (`:173`) | `:172` (def) — align to `:172` or `:172-174` | `internal/domain/analyzer/lang_map.go:172` = `func SupportedLanguages() []string`; `:173` is the body line ranging over `langOverrides`. (The GUIDE's *package path* `internal/domain/analyzer/` is correct — note the audit brief's draft path `internal/adapters/analyzer/` was itself wrong.) |

---

## F. Per-doc accuracy summary

| Doc | Verdict | Defects found | Notes |
|-----|---------|---------------|-------|
| 01-knowledge-graph-and-visualization | **Clean** (no flagged anchors) | 0 | No load-bearing anchor failed this pass. |
| 02-integration-touchpoints | Minor fix | 2 (both `storage.go:12-45` → `:12-56`) | Same truncated interface range cited twice; claim survives. |
| 03-access-surface | Minor fix | 2 (G0 `:8` → `:7`, twice) | Off-by-one into G1; claim survives. |
| 04-scale-and-positioning | Minor fix | 1 (graphify "10" → "~7" MCP tools) | External count not corroborated; argument ("only 3 beat grep") survives. |
| 05-redteam-alignment | **Clean** (no flagged anchors) | 0 | — |
| 06-competitive-landscape | Fix | 3 (fsnotify path; CodeSee framing+date; arXiv 2512.02170) | Two are external-fact errors (CodeSee, wrong-paper); the honesty-banner code anchors otherwise verify (`web/server.go:156`, `app.go:2816`). |
| 07-five-moats | Fix | 3 (parser path ×2; 2605.06635 statistic) | The non-existent `internal/domain/index/parser.go` path appears twice; the citation-hallucination figure is mis-stated. Keystone *claim* (no-edge `Symbol`, count-only parse) is correct. |
| 08-interactive-diagrams-and-claude-loop | Minor fix | 3 (recon range `:107-109` → `:107-110`, three places) | Same short range; `withETag`/`revisionFn` anchors in the same rows verify. |
| 09-growth-and-founder-positioning | Fix | 3 (arXiv 2512.02170, three places) | Wrong-paper anchor reused; replace with viral-but-unretained evidence. |
| 10-moat-redteam-validation | **Fix — material** | 9 (countImportSpecs `:567`→`:568` ×2; the 7 SP-synthesis rows incl. nonexistent §F.4 and the unverifiable Kosli/Heavybit comp) | The pool's only integrity issue. SP anchors are *thematically* close but the named labels are not SP strings → void under the rule. Relabel as doc10 synthesis. |
| ENHANCEMENT-GUIDE | Minor fix | 1 (`lang_map.go:173` → `:172`) | Off-by-one; the package path it uses is correct. |
| STRATEGIC-POSITION | **Source, not target** | 0 self-anchors failed | SP is the **round-2** document (line 1) and its cited bodies are exactly as quoted. The failures are in *doc10's references to SP*, not in SP itself. |

**Tally:** 0 docs failed; 6 need fixes (06, 07, 09, 10 substantive; 02, 03, 04, 08, GUIDE
cosmetic-anchor); 1 (doc10) carries the one material integrity issue. **0 load-bearing code
behaviors were found to be misrepresented** — every defect is an anchor/label/external-fact
correction, not a false claim about what aOa does.

---

## G. The honest state

The pool earns its falsifiability claim. The spine — keystone-edges-are-net-new, the
no-edge `Symbol` struct, the count-only parse pass, the `withETag`/304 transport, the recon
POST precedent, the socket `handleRequest` switch, the ten walker overrides vs three tuned
extractors vs 509 forest languages, the blind-judge gate, the scope law — all hold against
live source. The high-stakes external facts hold against primary sources.

What needs correcting is real but bounded: a cluster of off-by-one line anchors (doc-comment
cited instead of `func`; G0/G1 boundary; truncated ranges), one non-existent package path used
twice (`internal/domain/index/parser.go` — the parser is `internal/adapters/treesitter/parser.go`),
three wrong external anchors (CodeSee was *acquired by GitKraken, May 2024*, not "defunct
Feb 2024"; arXiv 2512.02170 is *Flowchart2Mermaid*, not an LLM-Mermaid/diagram-retention study;
the 2605.06635 figure is *39-77% factual accuracy / ~42% drop*, not "11-57%"), one un-corroborated
tool count (graphify ~7, not 10), and — the only thing that rises to *material* — doc10's habit of
pinning its own synthesis labels (Snyk shape, Christensen asymmetry, land-and-expand, option
value, window-closes, the Kosli/Heavybit comp) to `STRATEGIC-POSITION.md` line numbers and a
nonexistent `§F.4`. Those labels are good analysis; they are just **doc10's**, not SP's, and the
rule requires saying so.

Correct the anchors as listed and the pool is internally consistent and externally defensible.
