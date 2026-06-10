# Model-Standard — the view validation skill

One design, one perspective. Every standard view answers exactly one question
(`view-standards.json` is the single source of truth for which). This skill is
the gate that checks each generated view actually does its job — run it after
authoring or regenerating any estate's views.

## Enforcement posture

- **Generation loop**: a failing view goes back to its author with the findings
  for another pass. Authors receive the intent block (question / canvas-vital /
  hover-tier) in their brief — it is also embedded in the viewer's ⧉ copy-prompt.
- **Shipped content**: findings are reported, never silently fixed and never
  blocking. Visible "below standard" reporting, same philosophy as ⚠/◌ chips.

## The three checks

### 1. Lint (mechanical)

```
python3 playbook/generators/lint_views.py [estate-prefix] [--strict]
```

Label budgets (nodes ≤30 · members ≤26 · edges ≤48 chars), vital-field
presence per view kind (verbs on flow edges, parts on trust zones, fields on
entities), bucket/node count budgets. Deterministic, runs on the built shards
in `playbook/mockups/archmodel/`.

### 2. Render + look

Rebuild and screenshot the views under test (server: `cd playbook &&
python3 -m http.server 8777`):

```
chromium-browser --headless --disable-gpu --window-size=1680,1000 \
  --virtual-time-budget=30000 --screenshot=<out.png> \
  "http://127.0.0.1:8777/mockups/architecture-c4.html?estate=<e>&scope=<s>&auto=<view>:1200"
```

`&hover=<nodeId>` forces one hover card open so the hover tier is verifiable
by screenshot. Read every PNG by eye before claiming anything.

### 3. Blind judge (the core validation)

For each view, spawn a judge agent that receives ONLY:
- the screenshot path,
- the view's `question` from `view-standards.json`,
- the `pass` criterion.

No JSON, no context. The judge must (a) answer the question from the image
alone, (b) report any label it cannot read (truncation/overlap/bleed), and
(c) verdict pass/fail against the criterion. If the judge can't answer, the
view is not doing its job — that is the finding, regardless of how the JSON
looks. Judge verdicts beat lint: a lint-clean view that fails its judge fails.

Fan the judges out in parallel (one per view); collect a scorecard:
`estate/scope/view · lint findings · judge verdict · unreadable labels · notes`.

## Tier rules the renderer enforces

- Canvas = identity + the view's one signal. Labels never ellipsize — node
  width follows the label; very long member labels wrap to two lines.
- Hover = supporting metadata (`sub`, tech, scale, volumes). The hover test:
  supporting means it *enhances the view's answer* — anything that doesn't
  belongs in drill or nowhere.
- Drill = the full record (node click / scope drill).
