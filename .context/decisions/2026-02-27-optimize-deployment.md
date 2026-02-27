# ADR: Optimize Deployment — aOa + aOa-Recon Installation Path

**Date:** 2026-02-27
**Status:** Investigation
**Related:** `2026-02-24-severity-model-and-tier-depth.md`, Session 71 recon separation

## Problem

`aoa grep` and `aoa egrep` behave like basic grep — no symbol names, no line ranges, no structural context, no semantic mapping. TreeSitter enrichment that was previously working is not reaching search results.

Three root causes identified:

### 1. Binary Identity Crisis

The daemon log reveals the deployed binary keeps flipping between lean (0 rules) and recon (136 rules) builds. Both overwrite the same file path. During development, `./build.sh` (lean) and `./build.sh --recon` alternate, so the daemon's capabilities change unpredictably per restart.

Evidence from `.aoa/log/daemon.log`:
```
[2026-02-27T10:42:43] ready — 136 rules (recon build, dim scan ran)
[2026-02-27T11:16:24] daemon starting — 0 rules (lean build, no parser)
```

The current running daemon: **0 rules** — lean build, no treesitter.

### 2. Enrichment Gap: Dimensional Results Don't Reach grep

Even when running a recon build (136 rules), the dimensional analysis only flows to the **web dashboard**. The search pipeline in `internal/domain/index/search.go` never queries dimensional results:

```
Search Pipeline (search.go)
  Phase 7: enrichHits() → atlas domains only (keyword overlap)
  Phase 8: fillContentTags() → atlas terms only

  ❌ Never queries: dimEngine, dimCache, DimensionalResults()
  ❌ No structural context (findings, bitmasks, methods) in search hits
```

The `SearchHit` wire format has `Domain` and `Tags` fields, but no fields for dimensional tier, structural findings, or confidence levels.

### 3. Two-Binary Architecture Not Fully Wired

The separation design (Session 71) correctly positions aoa-recon as a companion binary that writes to shared bbolt. But the integration has a gap:

| What aoa-recon writes | Where it goes | Who reads it |
|----------------------|---------------|--------------|
| SymbolMeta (name, kind, line range) | `index` bucket in bbolt | Search engine ✅ |
| Dimensional findings (bitmask, methods) | `dimensions` bucket in bbolt | Web dashboard only ❌ |
| Rule violations per file | `dimensions` bucket in bbolt | Web dashboard only ❌ |

**Symbol extraction works** — when aoa-recon runs `enhance`, it writes SymbolMeta to the index bucket, and search.go reads it via `idx.Metadata[ref]`. This gives grep symbol names and line ranges.

**Dimensional enrichment doesn't flow to grep** — findings, confidence, structural context stay siloed in the dashboard.

## Current Installation Flow

### npm install -g @mvpscale/aoa (Lean)

```
User runs: npm install -g @mvpscale/aoa
  → Node.js shim wraps platform-specific Go binary
  → Binary built with: CGO_ENABLED=0, -tags lean
  → Parser = nil (parser_nocgo.go)
  → aoa init indexes tokens only (no SymbolMeta)
  → aoa grep = token search + content scanning + atlas domains
  → No symbol names, no line ranges, no structural context
```

### npm install -g @mvpscale/aoa-recon (Companion)

```
User runs: npm install -g @mvpscale/aoa-recon
  → Separate Node.js shim wraps aoa-recon Go binary
  → Binary built with: CGO_ENABLED=1, -tags recon (includes treesitter)
  → User must run: aoa recon init
    → Writes .aoa/recon/enabled marker
    → Runs aoa-recon enhance → writes SymbolMeta + dimensional data to bbolt
  → aoa daemon restarts → detects .aoa/recon/enabled
    → ReconBridge discovers aoa-recon in PATH
    → BUT: daemon is still lean build → 0 rules, no dim engine
```

### The Gap

After `npm install -g @mvpscale/aoa @mvpscale/aoa-recon` + `aoa recon init`:

| Capability | Expected | Actual |
|-----------|----------|--------|
| Symbol extraction in index | ✅ aoa-recon enhance writes SymbolMeta | ✅ Works |
| Symbol names in grep results | ✅ search.go reads idx.Metadata | ✅ Works (after enhance) |
| Line ranges in grep results | ✅ buildHit() uses sym.StartLine/EndLine | ✅ Works (after enhance) |
| Domain enrichment | ✅ enrichHits() via atlas | ✅ Works |
| Dimensional context in grep | ✅ Structural findings alongside hits | ❌ Not wired |
| Confidence levels in grep | ✅ Structural vs text-only distinction | ❌ Not wired |
| Auto-enhance on file change | ✅ watcher triggers EnhanceFile | ⚠️ Only if ReconBridge.Available() |

## What Works Today

1. **aoa (lean) + aoa-recon enhance**: After running `aoa recon init`, the enhance command writes SymbolMeta into the index bucket. The lean daemon loads this enriched index and search results DO include symbol names and line ranges. This is the primary enrichment path and it works.

2. **Web dashboard**: Dimensional results (bitmasks, findings, methods) display correctly on the dashboard when loaded from bbolt.

3. **File watching**: When `.aoa/recon/enabled` exists and aoa-recon is in PATH, file changes trigger incremental `enhance-file` calls.

## What Doesn't Work

1. **Dim engine in lean daemon**: The lean build stubs out `initDimEngine()` as a no-op. Even with dimensional data in bbolt (written by aoa-recon), the lean daemon's `warmDimCache()` returns `(0, 0, false)`. The dashboard shows nothing. Only a recon-tagged build loads dimensional data.

2. **Dimensional data in grep**: Even in recon builds where dimCache is populated, search.go never queries it. No search hit includes tier, findings, or structural confidence.

3. **Daemon build stability**: The deployed binary identity isn't locked. Development builds overwrite the installed binary, flipping capabilities.

## Deployment Model: What Needs to Be True

### For npm Users (Production)

```
npm install -g @mvpscale/aoa          # Lean binary — fast, small, pure Go
npm install -g @mvpscale/aoa-recon    # Companion — treesitter + analysis

aoa init                               # Token-only index (fast)
aoa recon init                         # Enhance with symbols + dimensional
aoa grep "pattern"                     # Results: symbol + domain + structural context
```

### What the Lean Daemon Needs to Do

The lean daemon (no CGo, no treesitter compiled in) needs to:
1. ✅ Load SymbolMeta from index bucket (already works)
2. ❓ Load dimensional results from bbolt `dimensions` bucket (currently stubbed out in lean builds)
3. ❓ Include dimensional context in search hit enrichment (not implemented)

### Key Design Question

**Should the lean daemon read dimensional data from bbolt?**

The dimensional data was written by aoa-recon (which has treesitter). The lean daemon doesn't need treesitter to READ the results — it just needs to load JSON from bbolt and attach it to search hits.

Currently, `dim_engine_lean.go` stubs everything:
```go
//go:build !recon
func (a *App) initDimEngine() {}
func (a *App) warmDimCache(logFn func(string)) (int, int, bool) { return 0, 0, false }
func (a *App) updateDimForFile(fileID uint32, relPath string) {}
```

This was correct for the "lean never does recon work" rule. But there's a difference between:
- **Computing** dimensional analysis (requires treesitter) → recon only
- **Reading** pre-computed dimensional results from bbolt → any build

## Options

### Option A: Lean reads dimensional data from bbolt (Recommended)

Add a `loadDimFromStore()` path in the lean build that reads pre-computed dimensional results from bbolt without needing the dim engine or treesitter. The lean daemon becomes a consumer of recon's output.

**Changes needed:**
- `dim_engine_lean.go`: `warmDimCache()` loads from bbolt (no treesitter needed)
- `search.go`: `enrichHits()` checks dimCache for structural context
- `socket/protocol.go`: Add dimensional fields to `SearchHit` (optional, only present with recon data)

**Pros:** Single lean binary serves enriched results. Users get the upgrade path: install aoa-recon → grep gets richer.
**Cons:** Adds bbolt read path to lean build. Must validate that dimensional data format is stable.

### Option B: Recon build for full grep enrichment

Keep lean builds token-only. Users who want enriched grep must use the recon-tagged build (`./build.sh --recon`). This is the current model but requires distributing a larger CGo binary.

**Pros:** Clean separation — lean is truly lean.
**Cons:** Defeats the purpose of the two-binary architecture. Users must choose one binary, not compose capabilities.

### Option C: Proxy dimensional queries through aoa-recon

The lean daemon calls aoa-recon to fetch dimensional data on demand (via subprocess or socket). This keeps lean builds free of dimensional knowledge.

**Pros:** True separation of concerns.
**Cons:** Performance cost per query. Complexity of a second IPC channel. aoa-recon would need a query mode (not just enhance).

## Recommended Path: Option A

The two-binary architecture's value proposition is: **aoa-recon computes, aoa consumes**. The shared bbolt database is already the interface. The lean daemon should read dimensional results the same way it reads SymbolMeta — load from bbolt, cache in memory, attach to search hits.

This aligns with the stated goal: "you can update the recon component as a composite compendium versus rebuilding the entire solution."

## Next Steps

- [ ] Validate: confirm aoa-recon enhance writes SymbolMeta correctly to index bucket
- [ ] Validate: confirm lean daemon loads enriched index after enhance
- [ ] Design: what dimensional fields belong in grep output (tier? confidence? finding count?)
- [ ] Implement: lean build reads dimensions from bbolt (consumer-only, no treesitter)
- [ ] Implement: search pipeline attaches dimensional context to hits
- [ ] Deploy: lock binary paths so lean and recon don't overwrite each other
- [ ] Test: npm install flow end-to-end with both packages
