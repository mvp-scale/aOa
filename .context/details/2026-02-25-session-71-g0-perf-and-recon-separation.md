# Session 71: G0 Performance Fixes and Clean Recon Separation

> **Date**: 2026-02-25 | **Session**: 71 | **Status**: Complete
> **Related Tasks**: G0 (speed goal), P0 (critical bugs), L5 (walker fix)

## Context

Session 71 had two major themes: completing the clean separation of aOa-pure from recon (finishing the P0 bug sweep), and discovering and fixing two critical G0 speed violations during live gauntlet testing. The G0 fixes are the key artifact -- they represent fundamental performance improvements to the search engine.

---

## G0 Performance Fixes

### Problem 1: Regex Search Bypassed Trigram Index (5 seconds to 8ms)

**Discovery**: During gauntlet testing, `egrep "WarmCaches|Reindex|WipeProject"` took 5,009ms. This was a G0 violation -- the goal states all operations must complete in under 50ms.

**Root Cause**: In `internal/domain/index/content.go`, the `canUseTrigram()` function at line 47 explicitly excluded regex mode from the trigram path. This forced every regex search into a brute-force O(all-lines) scan across the entire file index (6,734 files in a real project). The trigram index -- which provides O(1) posting list lookups for 3-character sequences -- was completely bypassed for regex queries.

The exclusion was originally defensive: regex patterns like `.*` or `[abc]` don't have extractable literal substrings, so the code conservatively disabled trigrams for all regex. But most real regex patterns DO contain literal substrings (e.g., `func.*Observer` contains "func" and "Observer").

**Fix**: New function `scanContentRegexTrigram()` in `content.go` that extracts literal substrings from regex patterns using Go's `regexp/syntax` AST parser. The approach:

1. Parse the regex into an AST via `regexp/syntax.Parse()`
2. Walk the AST to extract literal sequences of 3+ characters
3. For **concatenation** patterns (e.g., `func.*Observer`): extract "func" AND "Observer", INTERSECT their trigram posting lists (both must appear on the same line)
4. For **alternation** patterns (e.g., `A|B|C`): UNION trigram candidates across branches
5. Fall back to brute-force only when no literals of 3+ chars can be extracted (e.g., pure wildcard `.*`)

New types introduced:
- `regexLiteralGroup` -- holds `intersect` (literals that must co-occur) and `branches` (alternative literal sets)
- `extractRegexLiterals()` -- recursive AST walker that returns a `regexLiteralGroup`

**Results**:
| Query | Before | After | Speedup |
|-------|--------|-------|---------|
| `egrep "func.*Observer"` | 405ms | 10ms | 40x |
| `egrep "WarmCaches\|Reindex\|WipeProject"` | 5,009ms | 8ms | 625x |

**Files Modified**: `internal/domain/index/content.go`, `internal/domain/index/content_test.go`

---

### Problem 2: Symbol Search Iterated Empty Posting Lists in Lean Mode (186ms to 4 microseconds)

**Discovery**: During gauntlet testing, `grep sessionID` took 249ms. Phase timing breakdown (added via AOA_DEBUG=1 from B17) revealed: symbol search phase = 186ms, content search phase = 1.1ms. The content phase was already fast (trigrams working). The symbol phase was the bottleneck.

**Root Cause**: In `internal/domain/index/search.go`, the `searchOR()` function iterated through all token posting list entries for each query token, checking `e.idx.Metadata[ref]` for every file reference. In lean mode (no tree-sitter), `e.idx.Metadata` is empty -- there are zero symbol entries. But the posting lists still contain content-derived token references (tens of thousands of entries from the inverted content index). The code was iterating through ALL of them, performing a map lookup on an empty map for each one, and getting nil every time.

The posting lists are shared between content tokens and symbol tokens -- a token like "sessionID" might appear in 500+ files via content indexing. The symbol search phase was checking every one of those 500+ posting list entries against an empty metadata map.

**Fix**: Guard the entire symbol search phase with `if len(e.idx.Metadata) > 0`. In lean mode (no tree-sitter symbols loaded), this skips the symbol iteration entirely. The check is O(1) -- just a map length check.

**Results**:
| Query | Before | After | Speedup |
|-------|--------|-------|---------|
| `grep sessionID` | 249ms | 6ms | 41x |
| Symbol phase specifically | 186ms | 4 microseconds | 46,500x |

The content phase (1.1ms) was unchanged -- it was already fast via trigram index.

**Files Modified**: `internal/domain/index/search.go`

---

### Combined Gauntlet Results

After both fixes, full gauntlet -- all operations sub-25ms:

| Command | Before | After |
|---------|--------|-------|
| grep sessionID | 249ms | 6ms |
| egrep "func.*Observer" | 405ms | 10ms |
| egrep "WarmCaches\|Reindex\|WipeProject" | 5,009ms | 8ms |
| grep error | 14ms | 13ms |
| locate bridge | 8ms | 10ms |
| find "*.go" | 14ms | 24ms |
| tree internal/app | 6ms | 4ms |

Every operation is now well within the G0 target of 50ms. The two fixes removed entire categories of performance pathology rather than optimizing constants.

---

## Clean Recon Separation (P0 Completion)

### What Was Done

The old embedded recon scanner was fully deleted from the lean path. This was an 8-step plan executed across Sessions 70-71:

1. **Deleted from app.go**: `warmReconCache()`, `updateReconForFile()`, `CachedReconResult()`, `appFileCacheAdapter`, `reconCache`/`reconScannedAt` fields
2. **Local types in web/recon.go**: Replaced imports of `internal/adapters/recon` with local `reconResult`/`reconFinding`/`reconFileInfo` structs
3. **Interface cleanup**: Removed `CachedReconResult` from `AppQueries` interface
4. **Gated ReconBridge**: `initReconBridge()` only runs when `.aoa/recon.enabled` marker file exists
5. **New command**: `aoa recon init` creates the marker and bootstraps recon
6. **Zero recon imports**: Verified no `internal/adapters/recon` imports in lean-path files

### P0 Bug Resolution

All 7 P0 bugs resolved:
- **B7**: Old scanner deleted -- no recon code to produce log messages
- **B9**: Install prompt is the only path when recon not enabled
- **B10**: Superseded -- warmReconCache() deleted entirely
- **B11**: Superseded -- updateReconForFile() deleted entirely
- **B14**: Removed 500-char truncation on user input text in conversation feed
- **B15**: Superseded -- BuildFileSymbols calls deleted with old scanner
- **B17**: AOA_DEBUG=1 debug logging at all key event points

---

## Walker Bug Fix (L5)

Fixed `TestWalker_IgnoredError` -- the `checkNameContains` and `checkHasArg` functions in `internal/adapters/treesitter/walker.go` now look through `expression_list` wrappers in Go AST. Go's tree-sitter grammar wraps function call arguments in expression_list nodes that the old code didn't traverse.

---

## E2E Verification

- Pure AOA lean build: zero recon, zero dim engine, 0 rules
- Dashboard shows install prompt, no recon data
- All 18 test packages green, go vet clean
- Both `go build ./cmd/aoa/` and `go build -tags lean ./cmd/aoa/` compile
- Full gauntlet timing validation (all sub-25ms)

## Key Files Modified

| File | Change |
|------|--------|
| `internal/domain/index/content.go` | Regex trigram extraction, `scanContentRegexTrigram()`, `extractRegexLiterals()` |
| `internal/domain/index/search.go` | Symbol search metadata guard, phase debug timing |
| `internal/domain/index/content_test.go` | Tests for `extractRegexLiterals()` |
| `internal/adapters/web/recon.go` | Local types, removed recon import |
| `internal/app/app.go` | Deleted old scanner, debug mode, removed truncation |
| `internal/adapters/socket/server.go` | Removed CachedReconResult from interface |
| `internal/app/recon_bridge.go` | Gated on .aoa/recon.enabled |
| `internal/app/watcher.go` | Added debug call |
| `internal/adapters/treesitter/walker.go` | expression_list fix |
| `cmd/aoa/cmd/recon.go` | NEW: aoa recon init command |
| `cmd/aoa/cmd/root.go` | Registered reconCmd |
| `cmd/aoa/cmd/init.go` | Gated recon behind enabled check |
