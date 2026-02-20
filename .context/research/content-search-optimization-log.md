# Content Search Optimization Log (Session 56)

> **Date**: 2026-02-19
> **Starting point**: `aoa grep tree` = 23ms
> **Ending point**: `aoa grep tree` = 4-9ms (brute-force with deferred tags + containsFold)
> **Next target**: <1ms via trigram index (L2.4-L2.7)

---

## What Was Tried (in order)

### Attempt 1: Content Token Inverted Index

**Idea**: Build a token inverted index during cache warm. Same pattern as the symbol index — token → list of (fileID, lineNum) refs. Content lookup becomes one map read instead of a full scan.

**Implementation**:
- Added `contentRef` type, `contentIndex map[string][]contentRef` to FileCache
- Built index during `WarmFromIndex` via `TokenizeContentLine` (normalize non-alnum → tokenize → camelCase split → lowercase)
- Added `scanContentIndexOR` and `scanContentIndexAND` dispatch paths
- Token lookup was O(1), verified working

**Result**: Search dropped to ~2ms. But then the critical flaw was identified.

**Why it failed (G1 violation)**: Token index does exact **token** matching, not **substring** matching. `Tokenize("btree")` → `["btree"]` (single token, no camelCase boundary). So searching for "tree" via the token index finds lines with the token "tree" but misses lines containing "btree", "subtree", "treetop". Unix `grep tree` finds all of these.

**Decision**: Reverted the content index dispatch. Content search always uses brute-force to preserve full grep substring semantics. The token inverted index infrastructure remains in FileCache (it was already built; no harm keeping it for potential future use like autocomplete), but it is not used for content search dispatch.

**Lesson**: Speed without behavioral correctness is a regression, not an optimization. This became the canonical example in the GH agent's Goal Evaluation Protocol.

---

### Attempt 2: Deferred Tag Generation

**Idea**: `generateContentTags()` was the most expensive per-hit operation (regex normalize + tokenize + atlas resolveTerms). It ran on every matching line, including the 80%+ that get discarded by MaxCount truncation.

**Implementation**:
- `buildContentHit()` now produces hits with `Tags: nil`
- `fillContentTags()` called in `Search()` after `buildResult()` truncation
- Tags only generated for the final ~20 hits that survive

**Result**: 23ms → 2-3ms for the indexed path, ~12ms for brute-force.

**Why it helped**: For query "tree", ~555 content lines matched but only 16 survived truncation (MaxCount=20 minus 4 symbol hits). Previously generating tags for all 555 lines; now only for 16. Eliminated ~97% of `generateContentTags` calls.

**Status**: Kept. This optimization applies to both brute-force and any future indexed path.

---

### Attempt 3: `containsFold` — Allocation-Free Case-Insensitive Matching

**Idea**: The brute-force matcher was calling `strings.Contains(strings.ToLower(line), lowerQuery)` on every line. `strings.ToLower` allocates a new string per call. With 74K lines, that's 74K heap allocations per search.

**Implementation**:
- Hand-written `containsFold(s, lowerSubstr string) bool` in content.go
- Byte-at-a-time ASCII case-fold: `if c >= 'A' && c <= 'Z' { c += 'a' - 'A' }`
- Zero allocations — compares in-place against the original string

**Result**: ~12ms → 4-9ms.

**Why it helped**: Eliminated 74K `strings.ToLower` allocations. The GC pressure from those allocations was a significant fraction of the total cost.

**Status**: Kept. Even with trigram index, `containsFold` is used for:
- Verification of trigram candidates in case-insensitive mode
- Brute-force fallback for queries < 3 chars, InvertMatch, regex without literals

---

### Attempt 4: `normalizeNonAlnum` — Regex-Free Content Line Normalization

**Idea**: `TokenizeContentLine` called `nonAlnumRe.ReplaceAllString(line, " ")` which is regex-based. Replace with a hand-written byte loop.

**Implementation**:
- `normalizeNonAlnum(s string) string` in tokenizer.go
- Iterates bytes, replaces non-`[a-zA-Z0-9]` with space, collapses consecutive non-alnum to single space

**Result**: Benchmark improved from ~80µs to ~55µs per op. Small improvement to tag generation and content index build.

**Status**: Kept. Used by `TokenizeContentLine` which is called during cache warm (content index build) and deferred tag generation.

---

## Profiling Discovery: 74K Lines, Not 17K

Early estimates assumed ~17K lines across 173 files (~100 lines/file average). Diagnostic logging revealed the actual number: **73,691 lines**. The `.context/` markdown files (GO-BOARD, COMPLETED, archived boards) are large — some with 400+ char lines. This 4x underestimate explained why the brute-force scan was slower than expected.

Breakdown:
- 173 files indexed
- 172 served from cache (1 disk read — likely a file just over 512KB cache-per-file limit)
- 73,691 total lines scanned per query
- ~555 lines matched "tree" (before MaxCount truncation)

---

## Current State (end of session)

### What's in place:
- **Brute-force content scan** with `containsFold` — 4-9ms, full grep substring semantics
- **Deferred tag generation** — tags only for final truncated hit set
- **Content token inverted index** — built in FileCache but not used for content dispatch (kept for potential future use)
- **Pre-computed fileSpans** — enclosing symbol lookup without per-search rebuild

### What's next (L2.4-L2.7):
- **Trigram index** — one lowered index, dual-mode verification (case-sensitive default, `-i` for insensitive)
- **Case-sensitivity G1 fix** — `aoa grep` must be case-sensitive by default like Unix grep
- **Target**: <1ms for queries ≥ 3 chars
- **Research**: [sub-ms-grep.md](sub-ms-grep.md)

### Performance progression this session:
```
23ms    Starting point (brute-force + inline tags + strings.ToLower)
12ms    After deferred tag generation
 8ms    After containsFold (allocation-free matching)
 4-9ms  After normalizeNonAlnum + cleanup (current)
<1ms    Target (trigram index, L2.4-L2.7)
```

---

## Files Modified This Session

| File | Changes |
|------|---------|
| `internal/domain/index/tokenizer.go` | Added `TokenizeContentLine`, `normalizeNonAlnum` |
| `internal/domain/index/filecache.go` | Added `contentRef`, `contentIndex`, `buildContentIndex`, `ContentLookup`, `HasContentIndex` |
| `internal/domain/index/content.go` | Refactored `scanFileContents` (brute-force only), deferred tags, `buildContentHit`, `fillContentTags`, `containsFold`, `contentFileLine` type |
| `internal/domain/index/search.go` | Added `fillContentTags` call after `buildResult`, extracted `sortByFileIDLine` |
| `internal/domain/index/filecache_test.go` | 5 new content index tests, fixed map-order flaky test in `TestFileCache_WarmAndGet` |
| `internal/domain/index/content_test.go` | 6 new tests: cached single-token, substring semantics, AND, regex, invert-match, `TestContentSearch_SubstringMatch` (G1 parity proof) |
| `test/benchmark_test.go` | Implemented `BenchmarkSearch_E2E` (500 files, ~55µs/op) |
| `.claude/agents/gh.md` | Added G0-G6 goals, Goal Evaluation Protocol, goal alignment in output format |
