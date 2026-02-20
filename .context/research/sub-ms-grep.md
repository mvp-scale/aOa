# Sub-Millisecond Content Search (grep parity at speed)

> **Date**: 2026-02-19
> **Status**: Research complete, ready for implementation
> **Problem**: `aoa grep tree` takes 4-9ms on 173 files / 74K lines. Target: <=1ms.

---

## Problem Statement

`aoa grep` and `aoa egrep` must be 100% behavioral replacements for Unix grep/egrep.
The current content search path is a brute-force linear scan: for every query, it iterates
ALL 74K cached lines, calling `containsFold(line, query)` per line. Even when only 20 results
are needed, it touches every line.

The existing token-based inverted index (contentIndex in FileCache) cannot be used for
substring matching because it tokenizes on word boundaries and CamelCase splits. Searching
for "tree" via the token index finds lines containing the token "tree" but misses "btree",
"subtree", "TreeSitter" -- a parity violation (G1).

**The fundamental constraint**: grep semantics require arbitrary substring matching. Any
optimization must preserve this: `grep tree` finds "btree", "subtreeOf", "TreeSitter".

---

## Current Architecture

```
Query "tree"
  |
  v
Search() in search.go
  |-- searchLiteral("tree") -> O(1) symbol lookup (already fast, sub-ms)
  |-- scanFileContents("tree", ...) -> BOTTLENECK
        |
        v
      scanContentBruteForce()
        for each of 173 files:
          for each of ~427 lines/file (74K total):
            containsFold(line, "tree")  <- ~1 byte-compare per char
        sort results
        return
```

**Cost breakdown** (estimated for 74K lines, avg 80 chars/line):
- `containsFold`: ~80 byte comparisons per line x 74K lines = ~5.9M byte ops
- Overhead: map lookups for dedup, Hit struct allocation, sort
- Measured: 4-9ms total

**What's already optimized**:
- `containsFold` is allocation-free, ASCII byte loop (no `strings.ToLower` copy)
- FileCache holds all lines in memory (no disk I/O)
- fileSpans pre-computed (no per-search symbol span rebuild)
- Tags deferred until after MaxCount truncation
- Existing content token index (but it's word-boundary, not substring)

---

## Approaches Evaluated

### 1. Trigram Index

**How it works**: At cache-warm time, extract all overlapping 3-character substrings
from every line (lowercased). Build an inverted index: trigram -> [(fileID, lineNum)].
At query time, extract trigrams from the query, intersect their posting lists to get
candidate lines, then verify candidates with the full `containsFold` check.

**Example**: Query "tree" -> trigrams ["tre", "ree"] -> intersect posting lists ->
candidates (lines containing both "tre" and "ree") -> verify with containsFold.

| Aspect | Assessment |
|--------|-----------|
| G0 Speed | Strong. Reduces candidate set from 74K to typically <100 lines. Intersection is fast (sorted posting lists). Verification on ~100 lines is microseconds. |
| G1 Parity | Safe. Trigram intersection is a conservative filter -- it may include false positives but never misses true matches. Final `containsFold` verification ensures exact grep semantics. "btree" contains trigrams "btr", "tre", "ree" -- query "tree" needs "tre"+"ree", both present. |
| G3 Agent-First | Safe. No behavioral change -- same inputs, same outputs, just faster. |
| G2 Single Binary | Safe. Pure Go, no external dependencies. |
| G4 Clean Arch | Good. Index lives in FileCache, search logic stays in content.go. |
| Memory | Moderate. ~17K unique trigrams for English code. Each posting list entry is 6 bytes (4 fileID + 2 lineNum). For 74K lines with ~25 trigrams/line avg = ~1.85M postings x 6 bytes = ~11MB. Acceptable within 250MB budget. |
| Build time | One-time at cache warm. ~74K lines x ~80 chars = ~5.9M trigrams to extract. Fast in Go. |

**Pros**:
- Proven at scale (Google Code Search, Zoekt/Sourcegraph)
- Handles regex queries too (extract trigrams from regex)
- Conservative filter guarantees no false negatives (G1 safe)
- Works with case-insensitive search (lowercase trigrams at build time)

**Cons**:
- Memory overhead (~11MB for this project size)
- Short queries (<3 chars) can't use trigram index, fall back to brute force
- Build step adds to cache warm time (acceptable -- one-time cost)

**Verdict: RECOMMENDED -- best balance of speed, correctness, and simplicity.**

---

### 2. Suffix Array (Go stdlib `index/suffixarray`)

**How it works**: Concatenate all cached lines into one big byte buffer with line
separators. Build a suffix array over the entire buffer. Query with `Lookup()` which
does O(log(N) * len(query)) binary search.

| Aspect | Assessment |
|--------|-----------|
| G0 Speed | Excellent. ~500ns per lookup on 1MB data (Bendersky benchmarks). Our 74K lines at ~80 chars = ~5.9MB, so maybe ~1-2us per lookup. |
| G1 Parity | Safe with care. Suffix array finds exact substrings. Case-insensitive requires building the array on lowercased data and lowering the query. |
| G3 Agent-First | Safe. |
| Memory | High. Suffix array needs 4 bytes per byte of input = ~24MB for 5.9MB of text. Plus the original text. Total ~30MB. |
| Build time | O(N) via SA-IS. For 5.9MB, estimated ~50-150ms (Bendersky measured 250ms for 1MB on 2016 hardware; modern hardware + O(N) algorithm is faster). Acceptable one-time cost. |

**Pros**:
- O(log N) lookup -- faster than trigram for individual queries
- Go stdlib, zero dependencies
- Handles any substring length (no 3-char minimum)

**Cons**:
- High memory: ~30MB for suffix array alone (on top of line cache)
- Must maintain mapping from byte offset back to (fileID, lineNum) -- complex
- Case-insensitive requires separate lowercased buffer (doubles text memory)
- Rebuilding on file change is expensive (must rebuild entire array)
- Overkill for this scale -- 74K lines doesn't need O(log N)

**Verdict: REJECTED -- memory cost too high, offset-to-line mapping adds complexity,
and trigram is sufficient for this scale.**

---

### 3. Parallel Scanning (Goroutines per File)

**How it works**: Instead of scanning files sequentially, fan out to N goroutines
(one per file or one per chunk of files). Each goroutine scans its portion and sends
results to a collector channel.

| Aspect | Assessment |
|--------|-----------|
| G0 Speed | Moderate. On 8 cores, theoretical 8x speedup -> 4-9ms / 8 = 0.5-1.1ms. Marginal. Goroutine overhead (spawn, channel send, sync) eats into gains at this scale. |
| G1 Parity | Safe. Same matching logic, just parallelized. |
| G3 Agent-First | Safe. |
| Memory | Low. Just goroutine stacks (~8KB each x 173 = ~1.4MB transient). |
| Complexity | Moderate. Need to handle result ordering (deterministic sort after collect), dedup across goroutines, early termination when MaxCount reached. |

**Pros**:
- Simple conceptually
- No index build cost
- Works for all query types (substring, regex, AND)

**Cons**:
- Only ~4-8x speedup at best -- may not reliably hit <1ms
- Goroutine spawn + channel overhead significant for 173 files (each ~427 lines)
- Does not reduce total CPU work -- just spreads it across cores
- Deterministic ordering requires post-collect sort
- On single-core (CI environments), no speedup at all

**Verdict: SUPPLEMENT ONLY -- useful as a secondary optimization on top of trigram
index for the verification step, but insufficient alone to hit <1ms.**

---

### 4. Pre-Lowercased Line Cache

**How it works**: During cache warm, store a second array of lowercased lines alongside
the original lines. At search time, search the lowercased lines with a simple
`strings.Contains` (no case-folding needed per line).

| Aspect | Assessment |
|--------|-----------|
| G0 Speed | Small improvement. Eliminates per-byte case conversion in containsFold, but still scanning all 74K lines. Estimated savings: 20-30% of containsFold time (the branch/convert per byte). Might bring 4-9ms down to 3-7ms. Not enough alone. |
| G1 Parity | Safe. |
| Memory | Moderate. Doubles the line cache memory (~5.9MB -> ~11.8MB extra). |

**Pros**:
- Trivial to implement
- Eliminates case-fold overhead in hot loop
- `strings.Contains` may benefit from Go runtime's SIMD-optimized `strings.Index`

**Cons**:
- Still O(N) scan -- doesn't change the fundamental scaling
- Doubles line memory
- Insufficient alone to hit <1ms

**Verdict: USEFUL COMPLEMENT -- cheap to add alongside trigram index. The lowercased
buffer can serve double duty: trigram extraction uses it, and fallback brute-force
(queries <3 chars) searches it faster.**

---

### 5. Existing Token Index (contentIndex) Enhancement

**How it works**: The FileCache already has a token-based inverted index. Could we
enhance it to support substring matching by indexing all substrings of each token?

| Aspect | Assessment |
|--------|-----------|
| Feasibility | Poor. Indexing all substrings of all tokens creates O(L^2) entries per token where L is token length. For "authentication" (14 chars), that's 91 substring entries. Across all tokens, this explodes. |
| G1 Parity | Dangerous. Token boundaries still exist -- "btree" as a single token would need to be found by "tree", but if the line has "use btree index" the tokenizer might split differently than grep would. |

**Verdict: REJECTED -- explosion in index size, and token boundary issues risk G1 parity.**

---

## Goal Alignment Summary

| Goal | Trigram | Suffix Array | Parallel | Pre-Lower | Token Enhance |
|------|---------|-------------|----------|-----------|---------------|
| G0 Speed | ++ | +++ | + | + | + |
| G1 Parity | = (safe) | = (safe) | = (safe) | = (safe) | - (risky) |
| G2 Single Binary | = | = | = | = | = |
| G3 Agent-First | = | = | = | = | - (risky) |
| G4 Clean Arch | + | = | - (complexity) | + | = |
| G5 Self-Learning | = | = | = | = | = |
| G6 Value Proof | + | + | = | = | = |
| Memory | - (~11MB) | -- (~30MB) | = | - (~6MB) | --- (explodes) |

**No conflicts identified for the recommended approach (Trigram Index).**

---

## Recommended Approach: Trigram Index + Pre-Lowercased Lines

### Architecture

```
Cache Warm (one-time, on WarmFromIndex):
  for each file:
    for each line:
      lowerLine = strings.ToLower(line)
      store lowerLine in parallel array
      for i := 0; i <= len(lowerLine)-3; i++:
        trigram = lowerLine[i:i+3]
        trigramIndex[trigram] = append(..., {fileID, lineNum})

Query "tree" (default case-insensitive substring):
  1. lowerQuery = strings.ToLower("tree")
  2. Extract trigrams: ["tre", "ree"]
  3. Intersect posting lists for "tre" and "ree" -> candidate set
  4. For each candidate (fileID, lineNum):
       verify containsFold(originalLine, lowerQuery)
       if match: build Hit
  5. Return verified hits

Query "tr" (< 3 chars, fallback):
  Fall back to brute-force scan with pre-lowercased lines
  (still faster than current: strings.Contains on pre-lowered vs containsFold)

Query in regex mode:
  Extract literal trigrams from regex if possible (optimization)
  Otherwise fall back to brute-force scan
```

### Why Trigram Intersection Hits <1ms

- 74K lines, ~25 trigrams per line -> ~1.85M total posting entries
- Average posting list length per trigram: 1.85M / 17K unique trigrams = ~109 entries
- Query "tree" has 2 trigrams. Intersecting two lists of ~109 entries each: ~200 comparisons
- Candidate set after intersection: typically 5-50 lines
- Verification of 5-50 lines with containsFold: <10us
- Total: posting list lookup + intersection + verification = <100us estimated

For rare trigrams (e.g., "xyz"), posting lists are even shorter. For common trigrams
(e.g., "the"), the intersection with a second trigram dramatically reduces candidates.

### Fallback Cases

| Case | Approach | Expected Time |
|------|----------|---------------|
| Query >= 3 chars | Trigram index | <100us |
| Query < 3 chars | Brute-force on pre-lowered lines | 2-5ms (acceptable, rare in practice) |
| Regex mode | Try trigram extraction, else brute-force | Varies |
| InvertMatch (-v) | Must scan all lines (negation) | 4-9ms (same as now, inherent) |
| AND mode | Trigram per term, intersect all | <200us |
| Word boundary (-w) | Trigram + regex verify | <200us |

---

## Implementation Plan

### Phase 1: Trigram Index in FileCache (MVP)

| Step | File | Description | Status |
|------|------|-------------|--------|
| 1.1 | `filecache.go` | Add `lowerLines` field to `cacheEntry` -- pre-lowered copy of each line | Known |
| 1.2 | `filecache.go` | Add `trigramIndex map[trigramKey][]contentRef` to FileCache | Known |
| 1.3 | `filecache.go` | Define `trigramKey` as `[3]byte` (fixed-size, no string alloc for map key) | Known |
| 1.4 | `filecache.go` | In `WarmFromIndex`, after reading lines: build lowerLines + extract trigrams | Known |
| 1.5 | `filecache.go` | Add `TrigramLookup(trigrams [][3]byte) []contentRef` that intersects posting lists | Known |
| 1.6 | `filecache.go` | Add `GetLowerLines(fileID) []string` accessor | Known |
| 1.7 | `filecache_test.go` | Tests: trigram build, lookup, intersection, short-query fallback, case folding | Known |

### Phase 2: Wire Trigram into Content Search

| Step | File | Description | Status |
|------|------|-------------|--------|
| 2.1 | `content.go` | Add `extractTrigrams(query string) [][3]byte` helper | Known |
| 2.2 | `content.go` | Add `scanContentTrigram()` method: trigram lookup -> candidate lines -> verify | Known |
| 2.3 | `content.go` | In `scanFileContents()`: if cache has trigram index AND query >= 3 chars AND not InvertMatch -> use `scanContentTrigram()`, else fall back to brute force | Known |
| 2.4 | `content.go` | In brute-force fallback: use `GetLowerLines` + `strings.Contains` instead of `containsFold` for case-insensitive default mode | Known |
| 2.5 | `content_test.go` | Parity tests: "tree" finds "btree", "subtree" via trigram path | Known |
| 2.6 | `content_test.go` | Benchmark: BenchmarkContentSearch_74K comparing brute-force vs trigram | Known |

### Phase 3: Verification & Edge Cases

| Step | File | Description | Status |
|------|------|-------------|--------|
| 3.1 | `content_test.go` | Test: query < 3 chars falls back to brute force correctly | Known |
| 3.2 | `content_test.go` | Test: regex mode still works (brute force fallback) | Known |
| 3.3 | `content_test.go` | Test: InvertMatch still works (brute force) | Known |
| 3.4 | `content_test.go` | Test: AND mode uses trigram per term | Known |
| 3.5 | `benchmark_test.go` | Update E2E benchmark to measure trigram path | Known |
| 3.6 | All existing tests | Verify all 388+ tests still pass | Known |

### Phase 4: Optional -- Parallel Verification (if needed)

Only if Phase 2 doesn't hit <1ms (unlikely given the math above):

| Step | File | Description | Status |
|------|------|-------------|--------|
| 4.1 | `content.go` | Parallelize verification of trigram candidates across goroutines | Unknown |

---

## Key Design Decisions

### 1. Trigram key type: `[3]byte` vs `string`

Use `[3]byte` as map key. Fixed-size array keys in Go maps avoid string allocation
and hashing overhead. A `[3]byte` is 3 bytes, compared to a `string` which is 16 bytes
(pointer + length) plus the backing array.

### 2. Posting list storage: sorted slices

Store posting lists as `[]contentRef` sorted by (FileID, LineNum). This enables
efficient intersection via merge-join (linear scan of two sorted lists). No need for
bitmap compression at this scale.

### 3. Trigram extraction: lowercased bytes only

Extract trigrams from the lowercased version of each line. This means the trigram
index is inherently case-insensitive. For case-sensitive search (rare in practice),
fall back to brute force.

### 4. Dedup within the same line

A line like "tree tree tree" should produce only one posting entry for trigram "tre".
Use the same dedup logic as the existing content token index.

### 5. Preserve existing brute-force as fallback

The brute-force path must remain for:
- Queries shorter than 3 characters
- InvertMatch mode (need all non-matching lines)
- Regex mode (unless we add trigram extraction from regex, which is Phase 5)
- Any case where the trigram index isn't populated

---

## Memory Budget

| Component | Current | After Change |
|-----------|---------|-------------|
| Line cache (original) | ~5.9MB | ~5.9MB (unchanged) |
| Line cache (lowered) | 0 | ~5.9MB (new) |
| Content token index | ~2MB est. | ~2MB (unchanged) |
| Trigram index | 0 | ~11MB (new) |
| **Total cache** | **~8MB** | **~25MB** |

25MB is well within the 250MB budget. The trigram index is proportional to corpus size
and will scale linearly. For a 500-file project (the benchmark target), expect ~35MB
total -- still well under budget.

---

## Risks

| Risk | Mitigation | Fallback |
|------|-----------|----------|
| Trigram index build slows cache warm | Benchmark build time. Expect <50ms for 74K lines. | Lazy-build on first query instead |
| Very common trigrams ("the") have huge posting lists | Intersection with second/third trigram shrinks candidate set. Query "the" (3 chars) has 1 trigram = no intersection possible, falls back to brute force | Accept brute-force for 3-char queries with very common trigrams |
| Memory overhead > 250MB on large projects | Cap trigram index size. If > 100MB, disable and fall back to brute force | Config knob for trigram index enable/disable |
| Regex queries can't use trigram index | Regex path already works via brute force. Future: extract literal prefixes from regex for trigram optimization | Brute force fallback (existing behavior) |

---

## First Move

Implement step 1.1-1.4: add `lowerLines` to `cacheEntry` and build `trigramIndex`
during `WarmFromIndex`. Write a test that verifies `TrigramLookup(["tre","ree"])`
returns lines containing "tree", "btree", "subtree".

---

## References

- [Russ Cox: Regular Expression Matching with a Trigram Index](https://swtch.com/~rsc/regexp/regexp4.html)
- [google/codesearch -- trigram index package](https://pkg.go.dev/github.com/google/codesearch/index)
- [Sourcegraph Zoekt -- production trigram search](https://github.com/sourcegraph/zoekt)
- [Eli Bendersky: Suffix arrays in the Go standard library](https://eli.thegreenplace.net/2016/suffix-arrays-in-the-go-standard-library/)
- [Go stdlib index/suffixarray](https://pkg.go.dev/index/suffixarray)
- [Andrew Healey: Beating grep with Go](https://healeycodes.com/beating-grep-with-go)
- [dgryski/go-trigram -- reduced query times from 20ms to <1ms](https://blog.gopheracademy.com/advent-2014/string-matching/)
