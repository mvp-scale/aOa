# Model-Standard Pilot — Hartwell (retail-enterprise-faulted) · 2026-06-10

First run of the model-standard gate (`playbook/standards/MODEL-STANDARD.md`):
lint + render + blind judge over all 28 views. Judges received ONLY the
screenshot, the view's question, and the pass criterion.

## Result

| Check | Result |
|---|---|
| Lint (mechanical) | 22/56 Hartwell shards below standard · 85 findings (label budgets, unlabeled verbs) |
| Blind judge | **26/28 pass** · 2 fail |
| After renderer fixes | data/trust fail resolved · supply/domains remains (author-level) |

## The two fails

**data/trust — FIXED (renderer).** The violating edge's payload label
("intraday direct reads of OLTP payment feeds ×3") clipped at the canvas
edge — only "ls ×3" visible. The single most vital label in the view was
unreadable. Cause: fitView fits node bounds only; detour-routed edge labels
fell outside. Fix: invisible spacer nodes covering edge-label extents.

**supply/domains — OPEN (author fail-back).** Domains rendered as atomic
boxes with no members, so "what lives in each" is unanswerable; no edge
weights, so "heaviest dependency" is inference. Goes back to the supply
author with the intent block.

## Also fixed this round (renderer tier work)

- Metadata moved off every card face → hover cards (`sub`, tech, scale,
  overlay detail). `?hover=<id>` test hook for screenshot verification.
- Labels never ellipsize: node/entity width follows the label; long member
  labels wrap to two lines; member `sub` no longer competes with identity.
- Entity card titles (core/code COBOL programs) clipped mid-word → width
  now label-driven.
- Header view-title keeps its designed flex-truncation but carries a native
  tooltip with the full title.

## Author worklist (reported, not blocked — content stays authentic)

1. supply/domains: add domain members + edge counts (judge fail).
2. 85 lint findings — mostly member labels over the 26-char budget in
   merch/stores/supply (e.g. "Deal management (co-op / MDF / volume rebate)"
   at 45 chars); names belong on canvas, qualifiers in `sub` (hover).
3. stores/domains: header claims 33 subsystems, view shows 26 members —
   count mismatch.
4. Judges note native-resolution text is small on dense views (zoom needed);
   acceptable on screen, worth watching for print/export.

## Recurring chrome notes (known/by design)

- Sidebar bottom item clips behind scrollbar (scrollable panel — by design).
- "+N more…" collapsed tile on over-budget buckets is the intended Class B
  containment; the 46-member bucket is lint-flagged to the author.

---

# Round 2 — fully-baked re-author · 2026-06-10 (later)

All 7 scopes re-authored to the FULL 15-view catalog (105 views per variant,
210 total), with named hover stats, canonical layers, label budgets, and
F1-F7 preserved exactly (assembler-verified). Renderer round shipped first:
semantic color system + canvas legend, sidebar drill/detail panel + concerns,
hover z-index fix, named-stats hover cards.

| Check | Round 1 (28 views) | Round 2 (210 views) |
|---|---|---|
| Lint findings | 85 | **1 (the intentional F6 overflow)** |
| Blind judge | 26/28 | **14/15 sample pass** (one per view kind; full-set judging skipped — logged cap) |
| Open fail | supply/domains content gap | supply/domains layout density (content now passes) |

Round-2 fail detail: 8 buckets · 43 members force fitView to ~5px text;
×N counts on perimeter-routed violation edges illegible; ~28% canvas use.
Author content judged sound — moved to backlog as renderer layout-density
work (aspect-aware packing, endpoint-anchored edge labels).

Judge praise highlights: ecom/sequence retells BOPIS start-to-finish with
p95 figures; corp/techportfolio names riskiest platform; data/trust violation
identified "within seconds"; merch/datamodel schema spine fully sketchable.
