# 03 — Visualization: Shipping the Playbook Viewer as aOa's Production Presentation Plane

**Status:** implementation-grade spec · aligned to ENHANCEMENT-GUIDE phases ①–④ and the
scope ADR (`.context/decisions/2026-06-11-core-competence-and-scope-line.md`).
**Companion specs:** 01 (facts substrate), 02 (derivation/extractors). This document covers
only the presentation plane: the viewer, its data feed, governance surfaces, and export.

The playbook proved the viewer: 16 standard views on one rendering engine, provenance
pills, the three-segment dock, journeys, the interaction grammar, and a quality gate
(lint + blind judge) that the views actually pass. This spec moves that viewer — currently
a ~1,100-line JS template inside `playbook/generators/build_c4_mockup.py:408-1422` —
into `internal/adapters/web/` as a versioned product asset, fed by the daemon instead of
static files, without forking the source.

---

## 1. Productization path: one viewer source, two consumers

### 1.1 What exists

| Piece | Where | Evidence |
|---|---|---|
| Viewer JS (React Flow + ELK + htm, ESM) | string constant `JS` in `build_c4_mockup.py:408-1422` | imports at `:409-414` |
| HTML shell | string constant `HTML` at `build_c4_mockup.py:1423-1439` (hover CSS, React Flow stylesheet link) | |
| Build-time injection #1 | `__VIEW_INTENT__` replaced with question/vital/hover/pass per view from `view-standards.json` (`build_c4_mockup.py:1440-1444`, consumed at `:502`) | |
| Build-time injection #2 | `PALETTES` hardcoded in JS (`:454-459`); shards reference palettes by name (`"palette":"gf"` etc.) | |
| Data contract | tiny manifest + one shard per view, each shard hashed (`build_c4_mockup.py:370-406`; `playbook/mockups/archmodel/manifest.json` — 152KB manifest, 284 shards) | |
| Lazy loading | `?model=` query param sets `MODEL_PATH`; `BASE` derived from it (`:423-424`); shards fetched on first view open with `?v=<hash>` (`:1224-1228`) | |
| Integration target | `internal/adapters/web/` — `go:embed static` (`embed.go:7-8`), localhost-only listener (`server.go:69`), static handler with `no-cache` (`server.go:82-90`), JSON API + revision-ETag middleware (`server.go:92-113`, `:159-173`), port file (`server.go:124-126`) | |

The viewer is already fully data-driven — "the contract file IS the data source —
anything that emits a valid archmodel gets every view" (`build_c4_mockup.py:416`). The
productization job is therefore extraction + a feed swap, not a rewrite.

### 1.2 Target layout

```
internal/adapters/web/static/arch/
  index.html          # thin shell: <div id=root> + <script type=module src=viewer.js>
  viewer.js           # THE canonical viewer source (extracted from the JS constant)
  viewer.css          # hover/edge CSS extracted from the HTML constant
  vendor/             # Phase ④v: vendored ESM bundle (see §5)
```

Served at `http://localhost:{port}/arch/` by the existing static handler — no new
serving code; `go:embed static` picks the directory up automatically. The existing
dashboard (`static/index.html`, 5 tabs) gains an "Architecture" link; the two UIs stay
separate pages sharing the same server.

### 1.3 The two build-time injections become runtime data

1. **`VIEW_INTENT`** → the viewer fetches standards at boot from `BASE + "standards.json"`
   (one extra fetch beside the manifest; the daemon serves `view-standards.json` content at
   `/api/arch/standards.json`, and the playbook generator copies the same file into
   `mockups/archmodel/standards.json`). The `__VIEW_INTENT__` string-replace in
   `build_c4_mockup.py:1444` is deleted. The standards file is embedded in the Go binary
   via `go:embed` (same pattern as `atlas/v1`), so the product and the gate lint against
   the identical document.
2. **`PALETTES`** → moves out of JS into the standards file under
   `global.palette.named_palettes` (the canonical-layer pins are already there,
   `view-standards.json:28-37`). The JS color-resolution chain
   (view palette → canonical pin → stable name hash, `build_c4_mockup.py:793-808`)
   is unchanged — only the lookup table's source changes. Estate fixtures keep
   referencing palettes by name.

After this, `viewer.js` contains zero generated content. It is a plain static file.

### 1.4 Single-source strategy (the no-fork rule)

**The canonical source is `internal/adapters/web/static/arch/viewer.js`.** The playbook
generator stops carrying the JS string and instead:

```python
JS = open("internal/adapters/web/static/arch/viewer.js").read()   # single source
open("playbook/mockups/architecture-c4.html","w").write(HTML_SHELL.replace("__JS__", JS))
```

(Inlining keeps the mockup a self-contained file-server artifact; a `<script src>`
reference to the repo path works equally and is acceptable.) The generator's remaining
job — its permanent job — is the **fixture lab**: build manifest + shards from `go list`,
graphify imports, campaign estate fixtures (`build_c4_mockup.py:1-406`), so that
standards work, estate authoring, and gate development continue against fixtures without
a running daemon. SIMULATED estates live here and only here (§3.3).

**Fork guard:** a unit test in `internal/adapters/web` asserts
`playbook/mockups/architecture-c4.html` embeds a byte-identical copy of the embedded
`viewer.js` (hash compare). If the generator and the product ever diverge, CI fails.

### 1.5 Versioning

- `viewer.js` carries `const VIEWER_VERSION="arch-viewer/1"` and asserts
  `manifest.schema` starts with `aoa.archmodel/v1` — the mock suffix (`v1-mock`,
  `build_c4_mockup.py:286`) drops when the daemon emits it. Schema mismatch →
  the existing `showFatal` banner (`build_c4_mockup.py:418-422`), never a blank canvas.
- Asset cache behavior is already correct: the static handler sends `Cache-Control:
  no-cache` (`server.go:88`), so a daemon upgrade delivers the new viewer on reload.

---

## 2. Data feed: daemon HTTP API instead of static files

### 2.1 Endpoints

All under the existing mux (`server.go:77-113`), localhost-only, GET, JSON.

| Endpoint | Returns | Caching |
|---|---|---|
| `GET /api/arch/manifest` | the archmodel manifest (estates → scopes → view summaries + shard refs, exactly the shape `build_c4_mockup.py:374-405` writes) | `no-cache` + revision ETag via `withETag` (`server.go:159-173`) |
| `GET /api/arch/standards.json` | view-standards content + named palettes (§1.3) | revision ETag |
| `GET /api/arch/{estate}/{scope}/{view}.json` | one view shard | `Cache-Control: public, max-age=31536000, immutable` — safe because the viewer already appends `?v=<hash>` (`build_c4_mockup.py:1225`) and the manifest carries fresh hashes |
| `GET /api/arch/{estate}/journeys/{id}.json` | one journey shard (shape per the journeys contract, `view-standards.json:46`) | immutable, hash-keyed |
| `GET /api/arch/findings?new=1` | findings facts, baseline-aware (§4.2) | revision ETag |

**Routing note:** shard paths in the manifest are relative (`"local/aoa/context.json"`).
The viewer computes `BASE` from `MODEL_PATH` (`build_c4_mockup.py:424`), so serving the
product page as `/arch/?model=/api/arch/manifest` makes every shard fetch resolve to
`/api/arch/<relpath>?v=<hash>` **with zero viewer changes**. One Go handler
(`GET /api/arch/{path...}`) serves manifest, standards, shards, and journeys from the
substrate's rendition store; `index.html` defaults `MODEL_PATH` to `/api/arch/manifest`
when served from the daemon (detect: `location.pathname.startsWith("/arch")`).

Hash-based invalidation already exists end-to-end: shards are content-hashed at
generation (`build_c4_mockup.py:382-392`), the manifest carries the hash, the viewer
fetches by hash. The Go renderer reproduces this contract verbatim.

### 2.2 Where shards come from in-product

Per ENHANCEMENT-GUIDE §2, renditions are computed from the facts substrate at
substrate-compact time (not per-request) and cached in bbolt
(`arch_shards` bucket: key = `estate/scope/view`, value = shard bytes + hash).
The HTTP handler is a bucket read — O(1), consistent with G0. Re-derivation triggers:
index rebuild completes, or declarations file (`{root}/.aoa/arch.yaml`) changes.

### 2.3 Live-update story — keeping "always current" truthful

The footer stamps `generated <timestamp> · always current` (`build_c4_mockup.py:1058`).
In the static mockup that is aspirational; in-product it must be literal.

**Recommendation: ETag polling first, SSE later.**

| Option | Mechanics | Verdict |
|---|---|---|
| Poll manifest with `If-None-Match` | viewer polls `/api/arch/manifest` every 15s; `withETag` returns 304 until the global revision bumps (`server.go:165-171`, revision source already wired, L17.6); on 200, viewer diffs shard hashes and refetches only the open view | **Phase ② — ship this.** ~10 lines of viewer code, zero server code, free when idle |
| SSE `/api/arch/events` | fsnotify → rebuild → revision bump → `data: {rev}` push | Phase ④, only if polling latency (≤15s) proves annoying; one more handler, no protocol risk |

On refresh the viewer preserves selection/dock state (view change already preserves dock
expansion, `build_c4_mockup.py:1215`) and flashes the footer timestamp. The pipeline that
makes this honest already exists: fsnotify (debounced, filtered) → reindex → revision++.

---

## 3. Real-estate mapping: how a live repo fills the hierarchy

### 3.1 Single scope first

The hierarchy is estate → capability/scope → view (`vocabulary`,
`view-standards.json:47-53`). For one daemon on one project root:

| Level | Phase ② source | Later |
|---|---|---|
| Estate | exactly one: `local` — "Local · {root basename}", `sim:false` (mirrors `MODEL["estates"]["local"]`, `build_c4_mockup.py:290`) | estate dropdown lists registered projects (multi-root substrate union, ENHANCEMENT-GUIDE §3 "estate landscape") |
| Scope | one scope = the project. If the repo is a monorepo with clear top-level modules (go.work, workspaces, multiple manifests), the deriver MAY emit one scope per module — facts decide, never config guesses | cross-repo capabilities once a project registry exists (`/tmp/aoa-{hash}.sock` per project already gives discovery primitives) |
| Views | the GREEN set from facts: component / dsm / cycles / code / techportfolio (ENHANCEMENT-GUIDE §3 matrix), plus domains (enricher exists) | AMBER extractor views (routes, schemas, deploy, ownership) per phase ③ |

The sidebar's full standard catalog behavior carries over unchanged: every system shows
the complete catalog; views without facts render as `○ planned · extractor gated`
(`STD_CATALOG` + `dynamicCatalog`, `build_c4_mockup.py:470-522`, status legend `:1104-1106`).
This is a feature — the catalog is the roadmap, rendered in-product.

### 3.2 Provenance pills from real facts

The header pill (REAL/MIXED/SIMULATED, `build_c4_mockup.py:1346-1349`) stops being
authored metadata and becomes **computed**: every fact carries `prov` and a source
pointer (ENHANCEMENT-GUIDE §2 fact shape); a view's `prov.kind` is the floor of its
inputs per the three-layer ladder (ADR: derive=REAL, leashed-inference=MIXED,
declaration=DECLARED/SIMULATED). The renderer computes the label, e.g.
`REAL · 312 import facts · commit a1b2c3d`. The `⧉` copy-prompt for planned views
(`genPrompt`, `build_c4_mockup.py:503-515`) stays in the playbook lab only — in-product,
planned views point at the extractor that would light them up, not at an authoring prompt.

### 3.3 What happens to the SIMULATED estates

They stay in the playbook lab, permanently, as: (a) capability demos for the standard
catalog, (b) lint/judge fixtures for gate development, (c) the faulted variants as
regression tests for findings rendering. **None ship in the binary.** The product
dropdown shows real estates only; the `◌ / ●` sim marker (`build_c4_mockup.py:1340`)
remains in the viewer for the lab's sake and as the honesty marker if a user loads a
declared-only estate.

---

## 4. Governance surfaces

### 4.1 Findings dock fed by findings facts

Today the dock's FINDINGS segment (`BottomDock`, `build_c4_mockup.py:927-1002`) shows
problems computed **client-side at layout time**: band violations, orphans, god
components, cycle DFS (`layoutBuckets`, `build_c4_mockup.py:813-830`), plus table/matrix
self-derived concerns (`:1229-1241`).

In-product, detectors run at substrate-compact time and findings are facts
(`kind:finding`, ENHANCEMENT-GUIDE §4). Contract change: a view shard MAY carry
`"findings":[{id, severity, text, anchors:[ids], rule, source}]`; the viewer renders
shard findings **first** and keeps its render-time checks as a safety net (anything the
client finds that the server didn't is itself a finding worth seeing). Selection
highlighting (`hl()`, `build_c4_mockup.py:930-931`) keys on `anchors` instead of label
substring — a small robustness upgrade. The recon findings plumbing
(`recon.go:395`, `handleReconFindings`) is the precedent and partial code-reuse target.

### 4.2 Baseline / `--new` toggle

Per ENHANCEMENT-GUIDE §4, baseline/freeze (ArchUnit pattern) lives in bbolt: a stored
snapshot of finding fingerprints at a chosen commit.

- API: `GET /api/arch/findings` returns all; `?new=1` filters to post-baseline.
  `aoa arch findings [--new]` (CLI family, ENHANCEMENT-GUIDE §5) reads the same service.
- UI: the header `⚠ N` chip (`build_c4_mockup.py:1350-1353`) splits into `⚠ N · 3 new`;
  a `NEW only` toggle sits in the dock's FINDINGS segment header. Baseline set/reset is
  CLI-only (`aoa arch baseline set`) — a governance act that belongs in version-controlled
  workflow, not a browser click.

### 4.3 Conformance view rendering

Conformance = declared template (`.aoa/arch.yaml`: pattern + role→path map) diffed
against derived edges, yielding **convergent / divergent / absent** edge classes
(ENHANCEMENT-GUIDE §4). This extends the existing edge tag machinery — `e.tag` already
flips an edge to red-dashed with a `⚠` label and feeds the problems list
(`build_c4_mockup.py:779-787` simple, `:817-818` + `:874-878` buckets):

| Class | Meaning | Rendering |
|---|---|---|
| `convergent` | edge exists in code AND declaration | normal edge (optionally a subtle ✓ in hover) |
| `divergent` | edge exists in code, forbidden/undeclared | existing violation styling: red dashed + `⚠` label + finding — zero new machinery, set `tag:"divergent"` |
| `absent` | declared, not found in code | **new**: ghost edge — grey dashed at 0.4 opacity, label `∅ declared`, included in ELK layout so it occupies space honestly; clicking yields a SELECTION record citing the declaration line |

Contract: edge gains optional `conf` field; the conformance view is the 17th catalog
entry under "Classical structure". The caption derives the verdict line
("38 edges · 31 convergent · 5 divergent · 2 absent") via the existing `caption()`
mechanism (`build_c4_mockup.py:901-926`). Effort: S on the viewer (one edge style + a
caption branch); the diff engine itself is spec 02 territory.

### 4.4 Evidence-pack export (DD / PCI)

Three packs (ENHANCEMENT-GUIDE §6): DD exhibit set, PCI/SOC2 bundle, "what changed
since ref". The question is render mechanics.

| Approach | Pros | Cons | Effort |
|---|---|---|---|
| Print stylesheet on the live viewer | zero deps, S | React Flow canvas + fixed-viewport chrome paginate badly; ELK fit-to-viewport ≠ fit-to-page; non-deterministic across browsers | S, low quality ceiling |
| Server-side render: headless chromium screenshots per view, composed into one document | deterministic; **identical machinery to the model-standard gate** (MODEL-STANDARD.md §2 — same URL params `?estate=&scope=&auto=&sel=&hover=`), so every exported figure is gate-verifiable; figures carry provenance + commit stamp baked in | requires chromium on the host | M |

**Recommendation: server-side render, M.** `aoa arch pack <dd|pci|delta>` drives headless
chromium (discovered on PATH; clear error + fallback below if absent) against the
daemon's own `/arch/` URL, captures PNG/SVG per exhibit view, and composes a single
**self-contained HTML file** (images inlined base64, print CSS included → user prints to
PDF; no PDF library dependency). Each figure block: image + view question + caption +
provenance label + commit + generation timestamp — the anti-"seller diagram" contract.
Fallback when chromium is absent: the pack is generated with live iframes instead of
images and a banner stating figures are not frozen. Tables (SBOM, techportfolio,
findings scorecard) render server-side as plain HTML — no browser needed for those.

---

## 5. Dependencies & risks

### 5.1 CDN imports → vendored bundle

The viewer imports from esm.sh at runtime (`build_c4_mockup.py:409-414`, stylesheet at
`:1424`): react 18.3.1, react-dom, @xyflow/react 12.3.5, elkjs 0.11.1, htm 3.1.1. For a
localhost product this is wrong twice over: offline/airgapped enterprises get a blank
page, and a code-intelligence tool phoning a CDN is a procurement red flag.

**Recommendation: vendored single-file ESM bundle, mandatory for the product (not
optional).** Build with esbuild in CI (`make viewer-bundle`): one `vendor/deps.js`
exporting `{React, createRoot, ReactFlow, …, ELK, htm}` + `vendor/xyflow.css`; viewer.js
imports flip from URLs to `"./vendor/deps.js"`. The bundle output is committed (or
release-built) and ships inside `go:embed static`. The playbook lab may keep esm.sh —
but since the import lines are the only difference, prefer pointing the lab at the same
vendored file to preserve the no-fork rule (§1.4); the generator already serves the
whole playbook tree over `http.server`.

**Size budget** (minified, pre-gzip — to be measured at bundling, treat as the gate):
react+react-dom ~140KB, @xyflow/react ~400KB + 8KB css, elkjs ~1.5MB (the layout engine
dominates), htm ~1KB → **budget: ≤2.2MB raw, ≤650KB gzipped, hard cap 3MB raw.** Against
a binary that already embeds 134 atlas domains and tree-sitter, this is acceptable; if
the cap is threatened, elkjs is the lever (lazy-load it as a second chunk on first
canvas view — tables/matrix views don't need it).

### 5.2 Browser support

`viewer.js` uses native ESM **with top-level await** (`build_c4_mockup.py:426`) →
Chrome/Edge ≥89, Firefox ≥89, Safari ≥15. Acceptable for a developer-tool localhost UI;
document it, don't transpile. The `showFatal` banner is the failure mode for anything
older.

### 5.3 Layout density (existing backlog item)

Member budgets already auto-collapse over-40 buckets with a visible `COLLAPSED` chip
(`build_c4_mockup.py:831-832`, `:632`), and the standard caps views at 30 nodes
(`view-standards.json:17-23`). Real enterprise scopes will hit these caps constantly.
Mitigations, in order: (1) the deriver aggregates to module grain before rendering
(facts → buckets, n ≤ ~50, per ENHANCEMENT-GUIDE perf notes); (2) cache the laid-out
shard server-side so ELK runs once per revision, not per page load; (3) the dense-view
density work (semantic zoom / level-of-detail) stays a backlog item — tracked, not
blocking, because budgets + collapse keep every shipped view inside the standard.

### 5.4 Other risks

| Risk | Mitigation |
|---|---|
| ELK in main thread janks on big views | elkjs supports web-worker mode; adopt when layout >200ms is observed (measure first) |
| Manifest growth with many estates/views (already 152KB for 21 lab estates) | product manifest covers real estates only (one, initially — KBs); keep summary-only manifest discipline (`build_c4_mockup.py:389-392`) |
| Viewer/daemon contract drift | schema version assert (§1.5) + the fork-guard test (§1.4) + parity fixture: one golden manifest+shard set checked by both the Python lint and a Go contract test |
| `?v=` immutable caching serving stale shard after hash collision | 12-hex content hash (`build_c4_mockup.py:383`) — collision risk negligible; revision-ETag manifest is the source of truth for freshness |

---

## 6. Graphify visualization: inventory and keep/improve/drop

What graphify actually renders (`repo/ARCHITECTURE.md:23-24`, `repo/graphify/export.py`,
`tree_html.py`, `callflow_html.py`): a vis-network 9.1.6 force-directed `graph.html`
(`export.py:783`) with node-size-by-degree, click-to-inspect panel, search box,
community filter, confidence-styled edges, hyperedge overlays, and an aggregated
community meta-graph fallback above a node limit (`export.py:625-700`); a D3 v7
collapsible file tree (`tree_html.py:1-4`); a Mermaid 11 architecture/call-flow page
with a pan/zoom toolbar (`callflow_html.py:52-61`, `:1677`); plus `graph.svg`, an
Obsidian vault, Cypher export, and GRAPH_REPORT.md. Its strongest idea is the
EXTRACTED/INFERRED/AMBIGUOUS confidence label on every edge with a numeric rubric
(`docs/how-it-works.md:34-50`). What users like: zero-setup single-file HTML output,
search, and the inspect panel.

| Graphify feature | Verdict | Disposition in aOa |
|---|---|---|
| Confidence labels per edge (EXTRACTED/INFERRED/AMBIGUOUS) | **keep — surpassed** | our provenance ladder does this per-fact AND per-view with source pointers; graphify stops at the edge |
| Click-to-inspect panel | **keep — surpassed** | the SELECTION dock segment: stat table + relations table, violations first (`view-standards.json:44`) |
| Search box over nodes | **keep — adopt (gap!)** | the viewer has NO search today. Add: header search → select + center the matching element (reuses pending-selection machinery, `build_c4_mockup.py:1296-1300`). Phase ③, S |
| Aggregated meta-graph when too big | **keep — equivalent** | bucket grain + member budgets + auto-collapse already enforce this by standard, not by fallback |
| Single self-contained HTML artifact | **keep — relocated** | the product is a served app, but evidence packs (§4.4) are self-contained HTML — the property survives where it matters (sharing) |
| Legend, community filter | **keep — improved** | derived canvas legend shows only what's on screen (`build_c4_mockup.py:1005-1024`); color-is-meaning rule replaces per-run community colors |
| Force-directed physics layout | **drop** | non-deterministic (every load differs — screenshots can't gate it), unreadable past ~50 nodes; ELK layered layout is reproducible and judged |
| Mermaid call-flow renderer | **drop** | second render engine, second grammar; sequence view ships on the one engine when call-edge facts exist (catalog: "needs call-edge resolution") |
| D3 collapsible tree | **drop (viz), keep (capability)** | `aoa tree` already serves the CLI need; an arch-view tree adds nothing a component view doesn't |
| LLM semantic edges (`semantically_similar_to`), hyperedges | **drop** | violates the leash rule (ADR layer 2: agent may name/group, never add a node/edge); no fact source → doesn't ship |
| Obsidian vault / Cypher export | **drop** | foreign data planes, out of scope; evidence packs + `aoa arch facts` cover the export need |

Net: graphify's good ideas (confidence, inspect, search, self-contained sharing) all
land in the viewer — three already surpassed, one (search) adopted as a concrete task.
ENHANCEMENT-GUIDE §7 stands: graphify remains an estate in the dropdown, not a renderer.

---

## 7. Phased task list and the in-product quality gate

Phases align with ENHANCEMENT-GUIDE §8 (① substrate mock ② keystone+GREEN ③ live estate
④ governance). Sizes: S <1d · M 2–4d · L 1–2wk.

| # | Task | Size | Phase |
|---|---|---|---|
| V1 | Extract `viewer.js`/`index.html`/`viewer.css` from the JS/HTML constants into `internal/adapters/web/static/arch/`; generator reads the file (no-fork, §1.4) + fork-guard test | M | ② |
| V2 | Runtime standards: `__VIEW_INTENT__` → fetched `standards.json`; palettes into standards; embed standards via go:embed | S | ② |
| V3 | `/api/arch/` handler: manifest + shard + journey + standards serving from bbolt rendition bucket; immutable caching on hashed shards; manifest under `withETag` | M | ② |
| V4 | Local-estate manifest emission from substrate (single scope; GREEN views; computed provenance pills) | M (renderer side; derivation is spec 02) | ② |
| V5 | ETag-poll live refresh + truthful footer timestamp | S | ② |
| V6 | Findings in shard contract + dock reads server findings, anchor-based highlight | S | ③ |
| V7 | Element search in header (graphify adoption, §6) | S | ③ |
| V8 | Vendored dependency bundle + size-budget CI check; drop esm.sh from product | M | ③ (earlier if any enterprise demo needs offline) |
| V9 | Baseline `--new` toggle: API filter + chip split + dock toggle | S | ④ |
| V10 | Conformance edge classes (`conf` field, ghost `absent` edges, caption verdict) | S viewer / (diff engine in spec 02) | ④ |
| V11 | Evidence packs: `aoa arch pack` headless-chromium compose, self-contained HTML + print CSS | M | ④ |
| V12 | SSE push channel (only if polling proves insufficient) | S | ④ |
| V13 | Multi-project estate dropdown (project registry) | L | ④+ |

### 7.1 Running the model-standard gate against the in-product viewer in CI

The gate (MODEL-STANDARD.md) has three checks; all three port because every hook is a
URL parameter or a JSON file, and both stay identical in-product:

1. **Lint** — `lint_views.py` globs built shards. Add a `--dir` argument so it points at
   either `playbook/mockups/archmodel/` (lab) or a CI dump of the daemon's renditions
   (`aoa arch view --all --out dir/`, or `curl` the API). Same standards file (§1.3) by
   construction.
2. **Render + screenshot** — CI job: build via `./build.sh`, `aoa init` + index a fixture
   repo, start the daemon, read `.aoa/http.port` (`server.go:124-126`), then the existing
   chromium command verbatim against
   `http://localhost:{port}/arch/?estate=local&scope=…&auto=<view>:1200&hover=<id>` —
   `auto`/`sel`/`hover`/`journey` test hooks all live in the viewer
   (`build_c4_mockup.py:1249-1252`, `:1296`, `:1316-1318`), not in the generator, so they
   ship in-product for free.
3. **Blind judge** — unchanged: judges receive PNG + question + pass criterion from
   `standards.json`, nothing else. Lab and product screenshots are interchangeable
   inputs; judge verdicts beat lint, per the standard.

CI wiring: the gate runs on the lab fixtures on every viewer/standards change (fast,
deterministic), and on the live local estate in the nightly/release lane (proves the
daemon-served path end-to-end). A failing judge on shipped content reports visibly and
never blocks — same enforcement posture as MODEL-STANDARD.md.
