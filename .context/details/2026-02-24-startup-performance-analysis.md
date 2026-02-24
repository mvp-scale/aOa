# Storage & Startup Performance Refactor

**Date**: 2026-02-24 | **Status**: Research complete, pending board approval

---

## Requirements

| Req | Statement | Measured by |
|-----|-----------|-------------|
| **R0** | **<1s API response** — All search, recon, and dashboard APIs respond in <1s, including during startup (partial results acceptable) | Benchmark: every API endpoint <1s p99 |
| **R1** | **<5s warm restart** — Daemon restart with unchanged codebase resumes full operation in <5s at any codebase size up to 100K files | Measured: daemon log "all caches warm" timestamp |
| **R2** | **<10s cold start** — First-ever scan of a 100K file codebase completes initial indexing in <10s | Measured: daemon log timestamp |
| **R3** | **<100MB database at 4K files** — Storage footprint proportional to codebase size, not to write history | Measured: `ls -lh .aoa/aoa.db` |
| **R4** | **O(1) write per file change** — Editing a file writes only that file's data to storage, not the full index | Measured: bytes written per watcher event |
| **R5** | **No unbounded growth** — Database does not grow without bound over time for a stable codebase | Measured: db size stable after 1000+ file change cycles |
| **R6** | **Core fast without recon** — aOa without recon starts in <5s. Recon adds <1s on warm restart | Measured: startup with/without dim engine |
| **R7** | **No new external dependencies** — Use stdlib `encoding/gob`, `encoding/binary`, `compress/zlib`. No msgpack, protobuf, etc. | Checked: `go.mod` unchanged |

## Drivers

| Current | Problem | Impact |
|---------|---------|--------|
| 50s startup | 3 sequential JSON unmarshals + sequential file I/O + sequential dim scan | Users wait ~1 minute after every daemon restart |
| 964MB database | 306MB live JSON data + 658MB bbolt freelist bloat from rewriting 306MB on every file change | Disk waste; slow reads; I/O pressure |
| 306MB tokens blob | 12.9M TokenRefs encoded as `{"FileID":1234,"Line":56}` (25 bytes) instead of binary (6 bytes) | 4.2x size inflation on the dominant data structure |
| 160,000x write amplification | `SaveIndex()` rewrites all 3 blobs (~306MB) on every single file change event | bbolt file bloats to 3.2x live data; disk thrashing |
| Dim cache ignores persistence | `warmDimCache()` re-reads + re-parses all files from disk, ignoring `LoadAllDimensions()` in bbolt | Redundant 5s scan on every restart |
| Dim engine ignores file cache | `warmDimCache()` calls `os.ReadFile()` per file instead of using the file cache just built | Double disk reads for every file |
| Zero parallelism | All startup phases run sequentially in one goroutine | Can't use multiple cores |
| Sessions accumulate | No TTL or retention limit on session summaries | Unbounded (but small per entry — ~400B) |

## Observed Data (aOa-go project, 3959 files)

```
=== Index Blob Sizes ===
tokens   blob: 48,896 unique tokens → 305.2 MB   (99.8% of total)
metadata blob: 402 entries          →   0.1 MB
files    blob: 3,961 entries        →   0.5 MB
TOTAL:                                305.8 MB

=== Token Posting List Distribution ===
Total refs: 12,911,607 (avg 264 refs/token)
         1 ref:   4,348 tokens
       2-5 refs: 21,622 tokens
      6-20 refs: 14,420 tokens
    21-100 refs:  4,495 tokens
   101-500 refs:  2,278 tokens
      500+ refs:  1,733 tokens

=== JSON Overhead ===
Single TokenRef JSON: {"FileID":1234,"Line":56} = 25 bytes
Single TokenRef binary: uint32+uint16            =  6 bytes  (4.2x smaller)
12.9M refs × 25 bytes = 307.8 MB (JSON)
12.9M refs × 6 bytes  =  73.9 MB (binary, no compression)

=== bbolt ===
File on disk:   963.7 MB
Live JSON data: 305.8 MB
Overhead ratio: 3.2x
```

---

## Task Board

> Format matches `.context/BOARD.md`. Layer P = Performance refactor. Dependencies are task IDs.

| Layer | ID | G0 | G2 | G4 | Dep | Cf | St | Va | Task | Value | Va Detail |
|-------|-----|:--:|:--:|:--:|-----|:--:|:--:|:--:|------|-------|-----------|
| **P** | P.1 | x | | | - | 🟢 | ⚪ | ⚪ | Parallel JSON unmarshal in `LoadIndex` | 26.9s → ~10s index load | Benchmark: LoadIndex before/after |
| **P** | P.2 | x | | | - | 🟢 | ⚪ | ⚪ | Parallel file reads in `WarmFromIndex` (16-worker pool) | 17.7s → ~3s file cache warm | Benchmark: WarmCache before/after |
| **P** | P.3 | x | | x | - | 🟢 | ⚪ | ⚪ | Load dim results from bbolt on startup, diff timestamps, rescan only changed files | ~5s → ~0.5s dim warm on restart | Benchmark: warmDimCache with persistence |
| **P** | P.4 | x | | x | P.3 | 🟢 | ⚪ | ⚪ | Dim engine reads file cache instead of `os.ReadFile` | Eliminate redundant disk reads | Unit test: dim engine accepts `[]byte` |
| **P** | P.5 | x | | | P.1,P.2,P.3 | 🟢 | ⚪ | ⚪ | Concurrent startup phases (index ‖ dim load; then file cache) | Overlap independent work | Integration: startup <12s at 4K files |
| **P** | P.6 | x | | | - | 🟢 | ⚪ | ⚪ | Debounced index saves (dirty timer, 1s batch) | 200 saves/session → ~50; reduces write amplification 4x | Unit test: multiple changes → one save |
| **P** | P.7 | x | x | | - | 🟢 | ⚪ | ⚪ | Binary posting lists (`encoding/binary` packed uint32+uint16) | 306MB → ~74MB tokens blob; 4.2x smaller | Roundtrip test: encode/decode posting list |
| **P** | P.8 | x | x | | P.7 | 🟢 | ⚪ | ⚪ | Binary structs (`encoding/gob`) for FileMeta, SymbolMeta, FileAnalysis | ~2-3x smaller for metadata/dim data | Roundtrip test: gob encode/decode each struct |
| **P** | P.9 | x | x | | P.7,P.8 | 🟡 | ⚪ | ⚪ | Format version header + migration (detect v0 JSON / v1 binary) | Backward compat on upgrade; no data loss | Test: load v0 JSON db, verify migration to v1 |
| **P** | P.10 | x | | | P.7,P.8 | 🟢 | ⚪ | ⚪ | Per-file index storage in bbolt (tokens/files/symbols as separate keys) | O(1) write per file change (R4); eliminates 160,000x write amplification | Test: change 1 file, measure bytes written |
| **P** | P.11 | x | | | P.10 | 🟢 | ⚪ | ⚪ | Incremental `SaveIndex` — write only changed file's tokens/symbols | Single file change writes ~1KB not ~306MB | Benchmark: SaveIndex after 1-file change |
| **P** | P.12 | x | x | | P.7 | 🟡 | ⚪ | ⚪ | Compress posting lists >1KB with `compress/zlib` | ~74MB → ~20-30MB; posting lists compress 3-4x | Benchmark: compressed vs uncompressed decode speed |
| **P** | P.13 | | | | - | 🟢 | ⚪ | ⚪ | Session retention limit (keep last 200, prune oldest on save) | Bound the only unbounded accumulator (R5) | Test: 300 sessions → 200 after prune |
| **P** | P.14 | x | | | P.10 | 🟡 | ⚪ | ⚪ | Lazy trigram index — build on first search that needs it | Defer ~1.5s from startup path | Test: first trigram search builds index |
| **P** | P.15 | x | | | P.5 | 🟡 | ⚪ | ⚪ | Progressive API responses during startup warm | APIs return partial results while caches load (R0) | Integration: search returns results before full warm |
| **P** | P.16 | x | | | P.10,P.11 | 🟡 | ⚪ | ⚪ | Investigate 131MB search alloc — reduce per-query garbage | 78ms/131MB → <50ms/<20MB per search | Benchmark: search alloc before/after |

### Dependencies (visual)

```
P.1 (parallel unmarshal) ──┐
P.2 (parallel file reads) ─┤
P.3 (dim from bbolt) ──────┼── P.5 (concurrent startup) ── P.15 (progressive APIs)
       │                    │
       └── P.4 (dim uses    │
            file cache)     │
                            │
P.6 (debounced saves) ─────┘

P.7 (binary posting) ──┬── P.10 (per-file storage) ── P.11 (incremental save) ── P.16 (search alloc)
P.8 (gob structs) ─────┤
                        └── P.9 (format migration)
                        └── P.12 (compression)

P.13 (session prune) ── standalone
P.14 (lazy trigram) ── after P.10
```

### Execution phases

**Phase 1: Parallelism (P.1, P.2, P.3, P.4, P.5, P.6)** — No format changes. Concurrency only. Safe to merge incrementally. Expected: 50s → ~10s.

**Phase 2: Binary format (P.7, P.8, P.9)** — New encoding, format migration. Requires version header. Expected: db 964MB → ~100MB, load 10s → ~3s.

**Phase 3: Incremental storage (P.10, P.11, P.12)** — Per-file bbolt keys, incremental writes. The architectural win. Expected: write amplification 306MB → ~1KB per change. Warm restart <2s.

**Phase 4: Polish (P.13, P.14, P.15, P.16)** — Session prune, lazy trigram, progressive APIs, search alloc. Expected: all APIs <1s including during startup.

### Target outcomes by phase

| Scenario | Current | Phase 1 | Phase 2 | Phase 3 | Phase 4 |
|----------|---------|---------|---------|---------|---------|
| 4K files, warm restart | 50s | ~10s | ~3s | <2s | <2s |
| 100K files, warm restart | ~10min | ~3min | ~1min | <10s | <5s |
| Database size (4K files) | 964MB | 964MB | ~100MB | ~40MB | ~40MB |
| Database size (100K files) | ~25GB | ~25GB | ~1.2GB | ~500MB | ~500MB |
| Write per file change | 306MB | ~75MB | ~20MB | ~1KB | ~1KB |
| API during startup | blocked | blocked | blocked | blocked | <1s partial |
| Search alloc | 131MB | 131MB | 131MB | 131MB | <20MB |

---

## Research Detail

### Current Startup Timeline (3,959 files)

```
T+0.0s   daemon fork + socket listen
T+0.2s   socket ready, dashboard up       <-- daemon reachable, APIs return empty
T+0.2s   WarmCaches() begins (background, 100% sequential)
         ├─ LoadIndex from bbolt           26.9s  ██████████████████████████▉
         ├─ Engine.Rebuild()                0.3s  ▎
         ├─ LoadLearnerState                0.0s  ▏
         ├─ WarmCache (file cache)         17.7s  █████████████████▋
         └─ warmDimCache (recon scan)       ~5s   █████
T+50s    all caches warm — APIs return real data
```

### Benchmark Baselines (from `go test -bench`)

| Operation | Current | Notes |
|-----------|---------|-------|
| Search (E2E, 500 files) | 78ms | 131MB alloc per search |
| Observe (E2E) | 84us | Fast — learner hot path is good |
| Autotune (E2E) | 21us | Fast |
| Index single file (tree-sitter) | 10ms | Acceptable per-file |
| Tree-sitter parse | 238us | Fast per file |
| Aho-Corasick match | 12us | Fast |

### Phase 1: Index Load from bbolt (26.9s)

**Code path**: `bbolt/store.go:120-197` (LoadIndex)

Three sequential steps:
1. bbolt tx reads three blobs, copies bytes out of transaction
2. `json.Unmarshal` tokens (305MB), metadata (0.1MB), files (0.5MB) — sequential
3. Metadata/files keys parsed back from strings to numeric IDs via `fmt.Sscanf`

**Why it's slow**: Single-threaded unmarshal of 306MB JSON. The three blobs are independent but decoded sequentially. `fmt.Sscanf` per metadata entry adds overhead.

### Phase 2: File Cache Warm (17.7s)

**Code path**: `index/filecache.go:101-250` (WarmFromIndex)

Sequential steps:
1. Sort files by LastModified descending
2. Read each file: `os.Open` + 512-byte header check + `bufio.Scanner` — **one at a time**
3. Build content token inverted index (tokenize every line)
4. Build trigram index (extract every 3-byte substring from every line)

**Why it's slow**: 3500 files read sequentially × ~4ms each = ~14s I/O. Trigram index adds ~1.5s.

### Phase 3: Dimensional Cache Warm (~5s)

**Code path**: `app/dim_engine.go:54-90` (warmDimCache)

- Iterates all 3961 files in `Index.Files`
- Calls `os.ReadFile()` per file (ignoring file cache)
- Calls `dimEngine.AnalyzeFile()` per file (tree-sitter parse + AC match)
- `LoadAllDimensions()` exists in bbolt but is never called at startup

**Why it's slow**: Re-reads all files from disk (already cached). Sequential. No persistence reuse.

### Write Amplification

**Code path**: `watcher.go:80, 149, 174` — calls `SaveIndex()` on every file change

`SaveIndex()` (`bbolt/store.go:67-116`):
1. Converts all metadata keys to strings: `fmt.Sprintf("%d:%d", ref.FileID, ref.Line)`
2. Converts all file IDs to strings: `fmt.Sprintf("%d", fid)`
3. `json.Marshal` tokens (305MB) + metadata (0.1MB) + files (0.5MB)
4. Writes all three blobs in one bbolt update transaction

For a 100-file coding session: 100 × 306MB = **30.6GB** written through bbolt. bbolt reuses freed pages, so the file stabilizes at ~3.2x live data.

### Database Growth Bounds

| Data | Bounded? | Mechanism |
|------|----------|-----------|
| Index (tokens, metadata, files) | Yes | Mirrors codebase — `removeFileFromIndex()` cleans up on delete |
| Learner state | Yes | Autotune decay (×0.90 every 50 prompts) + prune (hits < 0.3 deleted) + blocklist (>1000 removed) |
| Dimensional results | Yes | Keyed by file path — deleted files removed |
| Sessions | **No** | Accumulates forever (~400B each, low priority) |

### Sharding Analysis

**Verdict: not useful.** The bottleneck is serialization, not storage layout. Splitting data across multiple databases adds distributed coordination without reducing the bytes that need deserializing. bbolt is already a B-tree. The inverted index must be complete to answer queries. The right fix is compact encoding + incremental writes in one database.

### Decoupling: Core aOa vs Recon

```
Core aOa (standalone, must be fast on its own)
├─ Index (tokens, metadata, files)
├─ Search engine (inverted index, file cache, trigram)
├─ Learner (observe, autotune, bigrams)
└─ File watcher → incremental index updates

Recon (additive, zero cost when absent)
├─ Dimensional engine (YAML rules, AC + tree-sitter)
├─ Dimensional cache (per-file bitmask + findings)
└─ Dashboard recon tab
```

**Coupling rules**:
1. Core never imports recon. Recon reads `ports.Index` and file cache (read-only).
2. File cache is the shared contract. Core builds it, recon consumes it.
3. One bbolt database, separate buckets. Recon uses `dimensions/`.
4. Startup phases overlap where data dependencies allow.
5. Incremental updates are independent — same watcher event, separate handlers.

### Encoding Recommendation

| Data | Encoding | Why |
|------|----------|-----|
| Posting lists ([]TokenRef) | `encoding/binary` packed arrays | Dominant data (99.8%). 6 bytes/ref vs 25. Trivial encode/decode. |
| FileMeta, SymbolMeta | `encoding/gob` | Struct-aware, stdlib, ~2-3x smaller than JSON |
| LearnerState | `encoding/gob` | Single blob, already small — gob is fine |
| FileAnalysis (dim results) | `encoding/gob` | Per-file, moderate size, struct-aware |
| SessionSummary | JSON (keep) | Tiny, human-readable for debugging, not worth changing |

All stdlib. No new dependencies. Satisfies R7.
