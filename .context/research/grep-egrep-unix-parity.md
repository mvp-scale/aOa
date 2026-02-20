# GNU grep/egrep vs aOa grep/egrep — Unix Parity Reference

> Generated: 2026-02-19 (Session 59)
> Source: `grep --help` (GNU grep 3.11) vs `aoa grep --help` / `aoa egrep --help`
> Test file: `test/migration/grep_parity_test.go` — 55 tests, all passing

---

## Alignment

GNU grep exposes 47 distinct flags. Not all are relevant to aOa's architecture (aOa is an indexed code search engine, not a stream processor). The alignment scores below reflect three tiers: **agent-critical** flags that AI agents actually pass, **useful** flags a power user might reach for, and **irrelevant** flags that don't apply.

### Current Alignment

| Tier | Total | Implemented | Alignment |
|------|------:|------------:|----------:|
| **Agent-Critical** (what Claude/agents pass) | 15 | 14 | **93%** |
| **Useful** (power user, scripting) | 9 | 0 | **0%** |
| **Irrelevant** (binary, stdin, Windows, etc.) | 23 | 0 | N/A |
| **Overall (agent-critical + useful)** | **24** | **14** | **58%** |

### Agent-Critical Flags (93% — 14/15)

These are the flags AI agents actually use in tool calls. One gap.

| Flag | aoa grep | aoa egrep | Status |
|------|:--------:|:---------:|--------|
| `-E` regex mode | DONE | N/A | |
| `-F` fixed strings | DONE (no-op) | — | |
| `-e` multi-pattern | DONE | DONE | |
| `-i` case-insensitive | DONE | **MISSING** | egrep gap |
| `-w` word boundary | DONE | — | egrep low-use |
| `-v` invert match | DONE | DONE | |
| `-c` count | DONE | DONE | |
| `-q` quiet / exit code | DONE | DONE | |
| `-m` max count | DONE | DONE | |
| `-r` recursive | DONE (no-op) | DONE (no-op) | |
| `-n` line number | DONE (no-op) | DONE (no-op) | |
| `-H` with filename | DONE (no-op) | DONE (no-op) | |
| `--include` glob | DONE | DONE | |
| `--exclude` glob | DONE | DONE | |
| `-l` files-with-matches | DONE (no-op) | — | |

**The one gap**: `-i` on egrep. Agents do pass `egrep -i` for case-insensitive regex search.

### Parity Board — What to Implement Next

All items added to GO-BOARD.md as L3.6–L3.14.

| ID | Board | Flag | Impact | Difficulty | Cf | St | Alignment After |
|:---|:------|:-----|:------:|:----------:|:--:|:--:|:----------------|
| P1 | L3.6 | egrep `-i` (case-insensitive regex) | **HIGH** | Easy | 🟢 | ⚪ | Agent: **100%**, Overall: 63% |
| P2 | L3.7 | `-A/-B/-C` context lines | **MEDIUM** | Medium | 🟡 | ⚪ | Agent: 100%, Overall: **67%** |
| P3 | L3.8 | `--exclude-dir` | LOW | Easy | 🟢 | ⚪ | Overall: 71% |
| P4 | L3.9 | `-o` only-matching | LOW | Easy | 🟢 | ⚪ | Overall: 75% |
| P5 | L3.10 | egrep `-w` (word boundary regex) | LOW | Easy | 🟢 | ⚪ | Overall: 79% |
| P6 | L3.11 | `-L` files-without-match | LOW | Easy | 🟢 | ⚪ | Overall: 83% |
| P7 | L3.12 | `-h` no-filename | LOW | Easy | 🟢 | ⚪ | Overall: 88% |
| P8 | L3.13 | `--color=never` | LOW | Easy | 🟢 | ⚪ | Overall: 92% |
| P9 | L3.14 | egrep `-a` (AND mode) | LOW | Easy | 🟢 | ⚪ | Overall: **96%** |

### Alignment Progression

```
NOW        ██████████████░░░░░░░░░░  58%  (14/24)  Agent: 93%
After P1   ███████████████░░░░░░░░░  63%  (15/24)  Agent: 100%  ← one flag
After P2   ████████████████░░░░░░░░  67%  (16/24)
After P3   █████████████████░░░░░░░  71%  (17/24)
After P4   ██████████████████░░░░░░  75%  (18/24)
After P5   ███████████████████░░░░░  79%  (19/24)
After P6   ████████████████████░░░░  83%  (20/24)
After P7   █████████████████████░░░  88%  (21/24)
After P8   ██████████████████████░░  92%  (22/24)
After P9   ███████████████████████░  96%  (23/24)  ← all easy+medium
```

### Recommended Execution Order

1. **P1** (L3.6: egrep `-i`) — immediately gets agent-critical to **100%**. ~20 lines.
2. **P2** (L3.7: `-A/-B/-C`) — the single most impactful power-user feature. ~100 lines.
3. **P3-P9** (L3.8-L3.14) — all Easy, diminishing returns, implement in any order.

---

## Detailed Reference

### Alignment Tier Definitions

**Agent-Critical** (15 flags): Flags that appear in real Claude Code tool calls, grep one-liners, and search workflows. If an agent passes one of these and aoa rejects or mishandles it, the tool call fails.

**Useful** (9 flags): Flags a developer or script might use. Not typically in agent tool calls but would be expected by a grep power user. `-A/-B/-C` context, `-o` only-matching, `--exclude-dir`, `-L`, `-h`, `--color`, `-x`.

**Irrelevant** (23 flags): Flags for binary file handling (`-a/--text`, `-I`, `--binary-files`, `-U`), stdin processing (`-z`, `--label`, `--line-buffered`), device handling (`-d`, `-D`), alternative regex engines (`-G`, `-P`), pattern files (`-f`, `--exclude-from`), and formatting (`-T`, `-Z`, `--group-separator`, `--no-group-separator`, `-R`, `--no-ignore-case`, `-s`, `-V`, `-b`). These don't apply to aOa's architecture (indexed project search, not stream processing).

---

## Executive Summary

| Metric | Value |
|--------|-------|
| GNU grep flags (total) | 47 |
| aoa grep: active flags | 11 |
| aoa grep: no-op compat flags (hidden) | 5 |
| aoa grep: total accepted | 16 |
| aoa egrep: active flags | 7 |
| aoa egrep: no-op compat flags (hidden) | 3 |
| aoa egrep: total accepted | 10 |
| aoa-only flags (not in GNU grep) | 1 (`-a` / `--and`) |
| Not implemented (useful) | 9 |
| Not implemented (irrelevant) | 23 |
| Parity tests passing | 55/55 |
| Agent-critical alignment | **93%** (14/15) |
| Overall alignment (agent + useful) | **58%** (14/24) |

---

## Help Menu Comparison

### GNU `grep --help`

```
Usage: grep [OPTION]... PATTERNS [FILE]...
Search for PATTERNS in each FILE.
Example: grep -i 'hello world' menu.h main.c
PATTERNS can contain multiple patterns separated by newlines.

Pattern selection and interpretation:
  -E, --extended-regexp     PATTERNS are extended regular expressions
  -F, --fixed-strings       PATTERNS are strings
  -G, --basic-regexp        PATTERNS are basic regular expressions
  -P, --perl-regexp         PATTERNS are Perl regular expressions
  -e, --regexp=PATTERNS     use PATTERNS for matching
  -f, --file=FILE           take PATTERNS from FILE
  -i, --ignore-case         ignore case distinctions in patterns and data
      --no-ignore-case      do not ignore case distinctions (default)
  -w, --word-regexp         match only whole words
  -x, --line-regexp         match only whole lines
  -z, --null-data           a data line ends in 0 byte, not newline

Miscellaneous:
  -s, --no-messages         suppress error messages
  -v, --invert-match        select non-matching lines
  -V, --version             display version information and exit
      --help                display this help text and exit

Output control:
  -m, --max-count=NUM       stop after NUM selected lines
  -b, --byte-offset         print the byte offset with output lines
  -n, --line-number         print line number with output lines
      --line-buffered       flush output on every line
  -H, --with-filename       print file name with output lines
  -h, --no-filename         suppress the file name prefix on output
      --label=LABEL         use LABEL as the standard input file name prefix
  -o, --only-matching       show only nonempty parts of lines that match
  -q, --quiet, --silent     suppress all normal output
      --binary-files=TYPE   assume that binary files are TYPE
  -a, --text                equivalent to --binary-files=text
  -I                        equivalent to --binary-files=without-match
  -d, --directories=ACTION  how to handle directories
  -D, --devices=ACTION      how to handle devices, FIFOs and sockets
  -r, --recursive           like --directories=recurse
  -R, --dereference-recursive  likewise, but follow all symlinks
      --include=GLOB        search only files that match GLOB
      --exclude=GLOB        skip files that match GLOB
      --exclude-from=FILE   skip files that match any file pattern from FILE
      --exclude-dir=GLOB    skip directories that match GLOB
  -L, --files-without-match  print only names of FILEs with no selected lines
  -l, --files-with-matches  print only names of FILEs with selected lines
  -c, --count               print only a count of selected lines per FILE
  -T, --initial-tab         make tabs line up (if needed)
  -Z, --null                print 0 byte after FILE name

Context control:
  -B, --before-context=NUM  print NUM lines of leading context
  -A, --after-context=NUM   print NUM lines of trailing context
  -C, --context=NUM         print NUM lines of output context
  -NUM                      same as --context=NUM
      --group-separator=SEP print SEP on line between matches with context
      --no-group-separator  do not print separator for matches with context
      --color[=WHEN]        use markers to highlight the matching strings
  -U, --binary              do not strip CR characters at EOL (MSDOS/Windows)
```

### `aoa grep --help`

```
Fast O(1) symbol lookup. Space-separated terms are OR search, ranked by density.

Usage:
  aoa grep [flags] <query>

Flags:
  -a, --and                  AND mode (comma-separated terms)
  -c, --count                Count only
      --exclude string       File glob filter (exclude)
  -E, --extended-regexp      Use regex (routes to egrep)
  -h, --help                 help for grep
  -i, --ignore-case          Case insensitive
      --include string       File glob filter (include)
  -v, --invert-match         Select non-matching
  -m, --max-count int        Max results (default 20)
  -q, --quiet                Quiet mode (exit code only)
  -e, --regexp stringArray   Multiple patterns (OR)
  -w, --word-regexp          Word boundary

Hidden (accepted, no-op):
  -r, --recursive            Always recursive (no-op)
  -n, --line-number          Always shows line numbers (no-op)
  -H, --with-filename        Always shows filenames (no-op)
  -F, --fixed-strings        Already literal (no-op)
  -l, --files-with-matches   Default behavior (no-op)
```

### `aoa egrep --help`

```
Extended regular expression search. Scans all symbols with full regex support.

Usage:
  aoa egrep [flags] <pattern>

Flags:
  -c, --count                Count only
      --exclude string       File glob filter (exclude)
  -h, --help                 help for egrep
      --include string       File glob filter (include)
  -v, --invert-match         Select non-matching
  -m, --max-count int        Max results (default 20)
  -q, --quiet                Quiet mode (exit code only)
  -e, --regexp stringArray   Multiple patterns (combined with |)

Hidden (accepted, no-op):
  -r, --recursive            Always recursive (no-op)
  -n, --line-number          Always shows line numbers (no-op)
  -H, --with-filename        Always shows filenames (no-op)
```

---

## Full Flag-by-Flag Parity Table

Every flag from `grep --help`, mapped to aoa status.

### Pattern Selection and Interpretation

| Short | Long | GNU grep | aoa grep | aoa egrep | Status | Test | Notes |
|:-----:|------|----------|----------|-----------|--------|------|-------|
| `-E` | `--extended-regexp` | Regex mode | Active: routes to egrep | N/A (egrep IS regex) | MATCH | `TestGrep_Flag_E_RoutesToRegex` | |
| `-F` | `--fixed-strings` | Literal mode | Hidden no-op (already literal) | — | MATCH | `TestGrep_Noop_r_AlwaysRecursive` | aoa default is literal |
| `-G` | `--basic-regexp` | BRE mode (default) | — | — | NOT IMPL | — | aoa uses literal default, not BRE |
| `-P` | `--perl-regexp` | PCRE mode | — | — | NOT IMPL | — | Go regex is RE2, not PCRE |
| `-e` | `--regexp=PATTERN` | Multi-pattern | Active: OR'd patterns | Active: joined with `\|` | MATCH | `TestGrep_Flag_e_MultiplePatterns`, `TestEgrep_Flag_e_MultiplePatterns` | |
| `-f` | `--file=FILE` | Patterns from file | — | — | NOT IMPL | — | Low priority |
| `-i` | `--ignore-case` | Case insensitive | Active | — | PARTIAL | `TestGrep_Flag_i_CaseInsensitive` | **Missing on egrep** |
|  | `--no-ignore-case` | Undo `-i` | — | — | NOT IMPL | — | aoa default is case-sensitive |
| `-w` | `--word-regexp` | Whole word match | Active | — | PARTIAL | `TestGrep_Flag_w_WordBoundary` | **Missing on egrep** |
| `-x` | `--line-regexp` | Whole line match | — | — | NOT IMPL | — | |
| `-z` | `--null-data` | Null-delimited | — | — | NOT IMPL | — | |

### Miscellaneous

| Short | Long | GNU grep | aoa grep | aoa egrep | Status | Test | Notes |
|:-----:|------|----------|----------|-----------|--------|------|-------|
| `-s` | `--no-messages` | Suppress errors | — | — | NOT IMPL | — | |
| `-v` | `--invert-match` | Non-matching | Active | Active | MATCH | `TestGrep_Flag_v_InvertMatch`, `TestEgrep_Flag_v` | |
| `-V` | `--version` | Version | Via `aoa --version` | Via `aoa --version` | MATCH | — | Top-level flag, not per-command |
|  | `--help` | Help | Active (`-h`) | Active (`-h`) | MATCH | — | |

### Output Control

| Short | Long | GNU grep | aoa grep | aoa egrep | Status | Test | Notes |
|:-----:|------|----------|----------|-----------|--------|------|-------|
| `-m` | `--max-count=NUM` | Limit results | Active (default 20) | Active (default 20) | MATCH | `TestGrep_Flag_m_MaxCount`, `TestGrep_Flag_m1_MaxCountOne`, `TestEgrep_Flag_m` | GNU default unlimited; aoa default 20 |
| `-b` | `--byte-offset` | Byte offset | — | — | NOT IMPL | — | |
| `-n` | `--line-number` | Line numbers | Hidden no-op (always on) | Hidden no-op (always on) | MATCH | `TestGrep_Noop_n_AlwaysLineNumbers` | aoa always shows line numbers |
|  | `--line-buffered` | Flush per line | — | — | NOT IMPL | — | |
| `-H` | `--with-filename` | Show filename | Hidden no-op (always on) | Hidden no-op (always on) | MATCH | `TestGrep_Noop_H_AlwaysFilename` | aoa always shows filenames |
| `-h` | `--no-filename` | Hide filename | — | — | NOT IMPL | — | |
|  | `--label=LABEL` | Stdin label | — | — | NOT IMPL | — | aoa doesn't read stdin |
| `-o` | `--only-matching` | Matching part only | — | — | NOT IMPL | — | |
| `-q` | `--quiet` | Exit code only | Active | Active | MATCH | `TestGrep_Flag_q_Quiet_Found`, `TestGrep_Flag_q_Quiet_NotFound`, `TestEgrep_Flag_q_Found`, `TestEgrep_Flag_q_NotFound` | |
|  | `--binary-files` | Binary handling | — | — | NOT IMPL | — | aoa skips binary automatically |
| `-a` | `--text` | Binary as text | **DIVERGENT**: `-a` = AND mode | — | **DIVERGENT** | `TestGrep_Flag_a_ANDMode`, `TestGrep_Flag_a_ANDMode_NoIntersection` | See divergence section below |
| `-I` | | Skip binary | — | — | NOT IMPL | — | aoa skips binary automatically |
| `-d` | `--directories` | Dir handling | — | — | NOT IMPL | — | |
| `-D` | `--devices` | Device handling | — | — | NOT IMPL | — | |
| `-r` | `--recursive` | Recurse dirs | Hidden no-op (always on) | Hidden no-op (always on) | MATCH | `TestGrep_Noop_r_AlwaysRecursive` | aoa always recursive |
| `-R` | `--dereference-recursive` | Recurse + follow | — | — | NOT IMPL | — | |
|  | `--include=GLOB` | File filter (in) | Active | Active | MATCH | `TestGrep_Flag_include`, `TestEgrep_Flag_include` | |
|  | `--exclude=GLOB` | File filter (out) | Active | Active | MATCH | `TestGrep_Flag_exclude`, `TestEgrep_Flag_exclude` | |
|  | `--exclude-from=FILE` | Patterns from file | — | — | NOT IMPL | — | |
|  | `--exclude-dir=GLOB` | Dir filter | — | — | NOT IMPL | — | aoa uses `--exclude` for file globs; dirs skipped via built-in list |
| `-L` | `--files-without-match` | Files w/o match | — | — | NOT IMPL | — | |
| `-l` | `--files-with-matches` | Files w/ match | Hidden no-op | — | MATCH | — | aoa default behavior shows files |
| `-c` | `--count` | Count only | Active | Active | MATCH | `TestGrep_Flag_c_CountOnly`, `TestEgrep_Flag_c` | |
| `-T` | `--initial-tab` | Tab alignment | — | — | NOT IMPL | — | |
| `-Z` | `--null` | Null after name | — | — | NOT IMPL | — | |

### Context Control

| Short | Long | GNU grep | aoa grep | aoa egrep | Status | Test | Notes |
|:-----:|------|----------|----------|-----------|--------|------|-------|
| `-B` | `--before-context=NUM` | Lines before | — | — | NOT IMPL | — | |
| `-A` | `--after-context=NUM` | Lines after | — | — | NOT IMPL | — | |
| `-C` | `--context=NUM` | Lines around | — | — | NOT IMPL | — | |
|  | `--group-separator` | Separator | — | — | NOT IMPL | — | |
|  | `--no-group-separator` | No separator | — | — | NOT IMPL | — | |
|  | `--color` | Colorize | — | — | NOT IMPL | — | aoa always colorizes output |
| `-U` | `--binary` | No CR strip | — | — | NOT IMPL | — | |

### aoa-Only Flags (not in GNU grep)

| Short | Long | aoa grep | aoa egrep | Test | Notes |
|:-----:|------|----------|-----------|------|-------|
| `-a` | `--and` | Active: AND mode (comma-separated) | — | `TestGrep_Flag_a_ANDMode` | **Overloads GNU `-a` / `--text`** |

---

## Exit Code Parity

| Condition | GNU grep | aoa grep | aoa egrep | Test |
|-----------|----------|----------|-----------|------|
| Match found | 0 | 0 | 0 | `TestGrep_Flag_q_Quiet_Found`, `TestEgrep_Flag_q_Found` |
| No match | 1 | 1 | 1 | `TestGrep_Flag_q_Quiet_NotFound`, `TestEgrep_Flag_q_NotFound` |
| Error | 2 | (returns error) | (returns error) | CLI integration tests |

---

## Flag Combination Tests

These verify flags work correctly when combined (the real-world usage patterns).

| Combination | Test | Status |
|-------------|------|--------|
| `-i -w` (case-insensitive + word boundary) | `TestGrep_Combo_i_w` | PASS |
| `-v -c` (invert + count) | `TestGrep_Combo_v_c` | PASS |
| `-i -c` (case-insensitive + count) | `TestGrep_Combo_i_c` | PASS |
| `-a --include` (AND + file filter) | `TestGrep_Combo_a_include` | PASS |
| `-a --exclude` (AND + file filter) | `TestGrep_Combo_a_exclude` | PASS |
| `-v --include` (invert + file filter) | `TestGrep_Combo_v_include` | PASS |
| `-v --exclude` (invert + file filter) | `TestGrep_Combo_v_exclude` | PASS |
| `-q -v` (quiet + invert) | `TestGrep_Combo_q_v` | PASS |
| `-i -a` (case-insensitive + AND) | `TestGrep_Combo_i_a` | PASS |
| `-m -v` (max-count + invert) | `TestGrep_Combo_m_v` | PASS |
| `-c --include` (count + file filter) | `TestGrep_Combo_c_include` | PASS |
| egrep `-v -c` (invert + count) | `TestEgrep_Combo_v_c` | PASS |
| egrep `-v --include` (invert + filter) | `TestEgrep_Combo_v_include` | PASS |
| egrep `-m -v` (max-count + invert) | `TestEgrep_Combo_m_v` | PASS |

---

## Edge Case Tests

| Case | Test | Status |
|------|------|--------|
| Single-char token (below min length) | `TestGrep_Edge_ShortToken` | PASS |
| Two-char token | `TestGrep_Edge_TwoCharToken` | PASS |
| Unicode query | `TestGrep_Edge_Unicode` | PASS |
| CamelCase tokenization | `TestGrep_Edge_CamelCaseTokenization` | PASS |
| Dotted tokenization (`app.post`) | `TestGrep_Edge_DottedTokenization` | PASS |
| Hyphenated tokenization (`tree-sitter`) | `TestGrep_Edge_HyphenatedTokenization` | PASS |
| MaxCount=0 | `TestGrep_Edge_MaxCount_Zero` | PASS |
| Invalid regex | `TestEgrep_Edge_InvalidRegex` | PASS |

---

## Divergence: `-a` Flag

**GNU grep**: `-a` / `--text` = treat binary files as text.
**aoa grep**: `-a` / `--and` = AND mode (comma-separated term intersection).

This is the single intentional divergence. GNU grep's `-a` is irrelevant for aoa (binary files are always skipped). aoa's AND mode is a domain-specific feature for intersecting search terms.

An AI agent passing `-a` to aoa expecting binary-as-text will instead get AND-mode search. This could produce unexpected results if the query contains commas.

### Risk Assessment

Low risk. AI agents (Claude) do not typically use `grep -a` for binary handling. The common agent pattern is `grep -r pattern .` or `grep -i pattern file`, neither of which uses `-a`. The AND mode is documented in aoa's help and is a useful search feature.

---

## What Remains (Not Implemented)

### Potentially Useful for Agent Workflows

| Flag | Why | Priority |
|------|-----|----------|
| `-A/-B/-C` context lines | Agents use context lines frequently | Medium |
| `--exclude-dir` | More precise than `--exclude` for dir patterns | Low |
| `-o` only matching | Useful for extraction pipelines | Low |
| `-i` on egrep | Parity gap — grep has it, egrep doesn't | Medium |
| `-w` on egrep | Parity gap — grep has it, egrep doesn't | Low |

### Irrelevant for aoa's Architecture

| Flag | Why Not Needed |
|------|----------------|
| `-G` / `-P` | aoa uses Go RE2 regex; BRE/PCRE not applicable |
| `-f` patterns from file | aoa gets patterns from CLI args |
| `-z` null-data | aoa processes source files, not streams |
| `-s` suppress errors | aoa returns errors to caller |
| `-h` no-filename | aoa always shows context (file + symbol + line) |
| `--label` | No stdin support |
| `--binary-files` / `-I` | aoa auto-skips binary |
| `-d` / `-D` | aoa always walks directories |
| `-R` dereference | aoa doesn't follow symlinks by design |
| `--exclude-from` | aoa uses single glob patterns |
| `-L` files-without-match | Opposite of default; low demand |
| `-T` initial-tab | Output format is fixed |
| `-Z` null byte | Not a pipeline tool |
| `--line-buffered` | aoa returns complete results |
| `--color` | aoa always colorizes; no `--no-color` yet |
| `-U` binary (Windows) | Linux/macOS only |
| `--group-separator` | aoa output format doesn't use separators |

---

## Test Inventory

### test/migration/grep_parity_test.go (55 tests)

```
GREP INDIVIDUAL FLAGS (17 tests):
  TestGrep_Literal_SingleToken           PASS
  TestGrep_Literal_MultiTokenOR          PASS
  TestGrep_Literal_ZeroResults           PASS
  TestGrep_Flag_i_CaseInsensitive        PASS
  TestGrep_Flag_w_WordBoundary           PASS
  TestGrep_Flag_v_InvertMatch            PASS
  TestGrep_Flag_c_CountOnly              PASS
  TestGrep_Flag_q_Quiet_Found            PASS
  TestGrep_Flag_q_Quiet_NotFound         PASS
  TestGrep_Flag_m_MaxCount               PASS
  TestGrep_Flag_m1_MaxCountOne           PASS
  TestGrep_Flag_a_ANDMode                PASS
  TestGrep_Flag_a_ANDMode_NoIntersection PASS
  TestGrep_Flag_E_RoutesToRegex          PASS
  TestGrep_Flag_e_MultiplePatterns       PASS
  TestGrep_Flag_include                  PASS
  TestGrep_Flag_exclude                  PASS

GREP FLAG COMBINATIONS (11 tests):
  TestGrep_Combo_i_w                     PASS
  TestGrep_Combo_v_c                     PASS
  TestGrep_Combo_i_c                     PASS
  TestGrep_Combo_a_include               PASS
  TestGrep_Combo_a_exclude               PASS
  TestGrep_Combo_v_include               PASS
  TestGrep_Combo_v_exclude               PASS
  TestGrep_Combo_q_v                     PASS
  TestGrep_Combo_i_a                     PASS
  TestGrep_Combo_m_v                     PASS
  TestGrep_Combo_c_include               PASS

EGREP INDIVIDUAL FLAGS (12 tests):
  TestEgrep_Regex_SimplePattern          PASS
  TestEgrep_Regex_Alternation            PASS
  TestEgrep_Regex_Anchored               PASS
  TestEgrep_Regex_NoMatch                PASS
  TestEgrep_Flag_c                       PASS
  TestEgrep_Flag_q_Found                 PASS
  TestEgrep_Flag_q_NotFound              PASS
  TestEgrep_Flag_v                       PASS
  TestEgrep_Flag_m                       PASS
  TestEgrep_Flag_include                 PASS
  TestEgrep_Flag_exclude                 PASS
  TestEgrep_Flag_e_MultiplePatterns      PASS

EGREP FLAG COMBINATIONS (3 tests):
  TestEgrep_Combo_v_c                    PASS
  TestEgrep_Combo_v_include              PASS
  TestEgrep_Combo_m_v                    PASS

EDGE CASES (8 tests):
  TestGrep_Edge_ShortToken               PASS
  TestGrep_Edge_TwoCharToken             PASS
  TestGrep_Edge_Unicode                  PASS
  TestGrep_Edge_CamelCaseTokenization    PASS
  TestGrep_Edge_DottedTokenization       PASS
  TestGrep_Edge_HyphenatedTokenization   PASS
  TestGrep_Edge_MaxCount_Zero            PASS
  TestEgrep_Edge_InvalidRegex            PASS

NO-OP FLAGS (3 tests):
  TestGrep_Noop_r_AlwaysRecursive        PASS
  TestGrep_Noop_n_AlwaysLineNumbers      PASS
  TestGrep_Noop_H_AlwaysFilename         PASS

COVERAGE MATRIX (1 test):
  TestGrepParity_CoverageMatrix          PASS
```

### Additional Search Tests (existing, not in migration/)

```
test/parity_test.go:
  TestSearchParity (26 fixture queries)  PASS

test/integration/cli_test.go:
  TestGrep_Basic                         PASS
  TestGrep_NoDaemon                      PASS
  TestGrep_NoQuery                       PASS
  TestEgrep_Basic                        PASS
  TestEgrep_NoDaemon                     PASS
  TestGrep_InvertMatch                   PASS
  TestGrep_InvertMatch_CountOnly         PASS
  TestEgrep_InvertMatch                  PASS

internal/domain/index/content_test.go:
  30 content/body search tests           PASS
```
