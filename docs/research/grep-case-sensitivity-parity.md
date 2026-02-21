# grep/egrep Case-Sensitivity Parity Analysis (Session 56)

> **Date**: 2026-02-19
> **Status**: G1 violation identified, fix planned in L2.5
> **Goal**: `aoa grep` aliasable as `grep` — transparent drop-in replacement

---

## The Violation

Unix `grep` is **case-sensitive by default**. `grep SessionID` matches only "SessionID", not "sessionid" or "SESSIONID". The `-i` flag enables case-insensitive matching.

`aoa grep` is **case-insensitive by default**. `aoa grep SessionID` matches "SessionID", "sessionid", "SESSIONID". The `-i` flag exists but is redundant — the default behavior is already case-insensitive.

This means `aoa grep` cannot be aliased as `grep` without changing search behavior. An AI agent expecting grep semantics would get different results. **G1 (parity) and G3 (agent-first) violation.**

### Where the violation lives

`internal/domain/index/content.go`, `buildContentMatcher()`, default case:

```go
default:
    lowerQuery := strings.ToLower(query)
    return func(line string) bool {
        return containsFold(line, lowerQuery)
    }
```

This always case-folds, regardless of whether `-i` was passed. The `Mode: "case_insensitive"` option set by the `-i` flag doesn't reach this code path — the default already does case-insensitive matching.

### Symbol search is different (by design)

Symbol search (the token inverted index) is inherently case-insensitive because `Tokenize()` lowercases all tokens at index time. This is acceptable and intentional — symbol names like `SessionID`, `sessionId`, `session_id` should all be found by searching "session". The enrichment layer (domains, terms, tags) depends on this normalization.

The case-sensitivity fix applies **only to content search** (the grep-like line matching).

---

## Fix Plan (L2.5)

### Content search modes after fix:

| Scenario | Mode | Verification |
|----------|------|-------------|
| `aoa grep tree` (default) | Case-sensitive | `strings.Contains(originalLine, "tree")` |
| `aoa grep -i tree` | Case-insensitive | `strings.Contains(lowerLine, "tree")` |
| `aoa egrep "Tree.*Sitter"` | Case-sensitive regex | `regexp.MatchString(originalLine, "Tree.*Sitter")` |
| `aoa egrep -i "tree.*sitter"` | Case-insensitive regex | `regexp.Compile("(?i)tree.*sitter")` |

### Changes required:

1. **`buildContentMatcher` default** — change from `containsFold` to `strings.Contains(line, query)` (exact case)
2. **`buildContentMatcher` case_insensitive mode** — use `containsFold` or `strings.Contains(lowerLine, lowerQuery)` (only when `-i` flag set)
3. **Trigram verification (L2.5)** — select verification function based on mode:
   - Default: `strings.Contains(originalLine, query)`
   - `-i`: `strings.Contains(lowerLine, lowerQuery)`
4. **Regex mode** — default already case-sensitive (`regexp.Compile`). For `-i`, prepend `(?i)` or use `regexp.Compile("(?i)" + pattern)`.

### Behavioral impact:

`aoa grep tree` will no longer find "Tree", "TREE", "TreeSitter" in content hits. It will only find lines containing lowercase "tree". This matches what `grep tree` does.

`aoa grep -i tree` continues to find all case variants. This matches `grep -i tree`.

Symbol hits are unaffected — the 4 symbol hits for "tree" (which come from the token index, inherently lowered) will still appear. Only the 16 content hits change behavior.

---

## grep/egrep Flag Compatibility Matrix

Full mapping of Unix grep flags to aoa implementation status. Target: 80-90% coverage for transparent aliasing.

### Pattern Matching

| Flag | grep | aoa grep | Status |
|------|------|----------|--------|
| (default) | Case-sensitive literal | Case-insensitive literal | **G1 VIOLATION — fix in L2.5** |
| `-i` / `--ignore-case` | Case-insensitive | Sets `Mode: "case_insensitive"` (redundant currently) | Exists, needs fix to be meaningful |
| `-w` / `--word-regexp` | Word boundary `\bword\b` | `WordBoundary: true` | **Supported** |
| `-x` / `--line-regexp` | Match whole line | Not implemented | Missing |
| `-F` / `--fixed-strings` | Literal (not regex) | No-op (default is literal) | **Supported** (hidden flag) |
| `-E` / `--extended-regexp` | Extended regex | Routes to egrep | **Supported** |
| `-P` / `--perl-regexp` | Perl regex | Not implemented | Missing (Go regex is RE2, not PCRE) |
| `-e PATTERN` / `--regexp` | Multiple patterns (OR) | `grepPatterns []string` joined with space | **Supported** |
| `-f FILE` / `--file` | Read patterns from file | Not implemented | Missing |

### Output Control

| Flag | grep | aoa grep | Status |
|------|------|----------|--------|
| `-c` / `--count` | Count matches only | `CountOnly: true` | **Supported** |
| `-l` / `--files-with-matches` | List matching files | No-op (hidden) — always shows files | Partial (always shows filenames) |
| `-L` / `--files-without-match` | List non-matching files | Not implemented | Missing |
| `-q` / `--quiet` | Exit code only | `Quiet: true` | **Supported** |
| `-m NUM` / `--max-count` | Stop after NUM matches | `MaxCount` (default 20) | **Supported** |
| `-n` / `--line-number` | Show line numbers | Always shows (no-op hidden) | **Supported** (always on) |
| `-H` / `--with-filename` | Show filename | Always shows (no-op hidden) | **Supported** (always on) |
| `-h` / `--no-filename` | Suppress filename | Not implemented | Missing |
| `-o` / `--only-matching` | Show only matching part | Not implemented | Missing |
| `-b` / `--byte-offset` | Show byte offset | Not implemented | Missing |
| `-Z` / `--null` | Null byte after filename | Not implemented | Missing |

### Context Lines

| Flag | grep | aoa grep | Status |
|------|------|----------|--------|
| `-A NUM` / `--after-context` | Lines after match | Not implemented | Missing |
| `-B NUM` / `--before-context` | Lines before match | Not implemented | Missing |
| `-C NUM` / `--context` | Lines around match | Not implemented | Missing |

### File Selection

| Flag | grep | aoa grep | Status |
|------|------|----------|--------|
| `-r` / `-R` / `--recursive` | Recursive search | Always recursive (no-op hidden) | **Supported** (always on) |
| `--include=GLOB` | Include files matching glob | `IncludeGlob` | **Supported** |
| `--exclude=GLOB` | Exclude files matching glob | `ExcludeGlob` | **Supported** |
| `--exclude-dir=DIR` | Exclude directories | Not implemented (handled by index skip list) | Partial |
| `-v` / `--invert-match` | Select non-matching | `InvertMatch: true` | **Supported** |

### aoa-Only Enhancements (not in grep)

| Flag | Purpose |
|------|---------|
| `-a` / `--and` | AND mode (comma-separated terms) |
| `--since` / `--before` | Time-based file filtering |
| (implicit) | Symbol hits with function signatures, line ranges |
| (implicit) | `@domain` and `#term` tags on every hit |
| (implicit) | Enclosing symbol context on content hits |

---

## Coverage Summary

| Category | Supported | Missing | Coverage |
|----------|-----------|---------|----------|
| Pattern matching | 5 | 3 (`-x`, `-P`, `-f`) | 63% |
| Output control | 5 | 5 (`-L`, `-h`, `-o`, `-b`, `-Z`) | 50% |
| Context lines | 0 | 3 (`-A`, `-B`, `-C`) | 0% |
| File selection | 4 | 1 (`--exclude-dir` partial) | 80% |
| **Total** | **14** | **12** | **54%** |

### Priority for reaching 80-90% coverage:

1. **Fix case-sensitivity default** (G1 critical — L2.5)
2. **Context lines `-A`/`-B`/`-C`** (most commonly used missing flags, agents use these)
3. **`-l` proper implementation** (list files only, don't show line content)
4. **`-L`** (files without match — complement of `-l`)
5. **`-h`** (suppress filename — useful in pipelines)
6. **`-o`** (only matching — useful for extraction)

Flags like `-P` (Perl regex), `-b` (byte offset), `-Z` (null separator) are low priority — rarely used by agents or in typical developer workflows.

---

## Alias Strategy

Target alias setup:
```bash
alias grep='aoa grep'
alias egrep='aoa egrep'
```

For this to work transparently:
1. Case-sensitive by default (L2.5) — **critical**
2. Unknown flags must not error — graceful degradation or pass-through
3. Exit codes must match grep convention (0 = match found, 1 = no match, 2 = error)
4. Output format differences (symbols, domains, tags) are additive — agents see richer output but can still parse filenames and line numbers from the same positions
