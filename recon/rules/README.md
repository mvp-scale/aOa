# Rules Engine

## You're Covered

aOa ships with 509 compiled tree-sitter grammars. Every rule in this directory runs against all of them — automatically, universally, with no per-language configuration. If your language is in the list below, your code is analyzed.

| Category | Languages |
|----------|-----------|
| **Core** | Go, Python, JavaScript, TypeScript, TSX, Rust, Java, C, C++, C#, Ruby, PHP, Swift, Kotlin, Scala |
| **Scripting** | Bash, Lua, Perl, R, Julia, Elixir, Erlang, AWK, Fish, Nu, PowerShell, TCL |
| **Functional** | Haskell, OCaml, Gleam, Elm, Clojure, PureScript, Fennel, F#, Scheme, Racket, Common Lisp, SML, ReScript |
| **Systems** | Zig, D, CUDA, Odin, V, Nim, Objective-C, Ada, Fortran, Verilog, SystemVerilog, VHDL, Pascal, Crystal, Hare, Cairo |
| **Web** | HTML, CSS, SCSS, LESS, Vue, Svelte, Dart, Astro, Pug, Slim, HAML, ERB, HEEX |
| **Data & Config** | JSON, JSONC, JSON5, YAML, TOML, SQL, Markdown, MDX, GraphQL, HCL/Terraform, Dockerfile, Nix, XML, CSV, INI, Protocol Buffers, LaTeX, KDL |
| **Build** | CMake, Make, Groovy/Gradle, GLSL, HLSL, WGSL, Just, Ninja, Meson |
| **Blockchain** | Solidity, Cairo, Prisma |
| **Other** | GDScript, Godot, Vim, Typst, Pkl, Assembly, NASM, Starlark, COBOL, and 400+ more |

**509 grammars total.** aOa stays current with the [go-sitter-forest](https://github.com/alexaandru/go-sitter-forest) grammar pack. If a language isn't here, it likely doesn't have a tree-sitter grammar yet.

## How It Works

All 509 languages produce different AST node types, but the differences are small. A function call is `call_expression` in most languages, `call` in Python, `method_invocation` in Java — but it's still a function call. aOa's universal translation layer collapses all these variants into 15 abstract concepts:

```
509 languages  →  ~78 unique AST node types  →  15 concepts
```

| Concept | What it matches |
|---------|----------------|
| `call` | Function/method calls |
| `assignment` | Variable assignments and declarations |
| `function` | Function and method definitions |
| `class` | Classes, structs, types, modules |
| `for_loop` | For loops, while loops, iteration |
| `return` | Return statements |
| `import` | Imports, includes, use declarations |
| `block` | Code blocks and compound statements |
| `switch` | Switch, match, case statements |
| `string_literal` | String values |
| `string_concat` | String concatenation via binary operators |
| `format_call` | Format/macro invocations |
| `defer` | Deferred execution |
| `type_assertion` | Type assertions and casts |
| `interface` | Interface declarations |

Rules are written against these concepts. The translation layer handles the rest. A rule that matches `call` will find `call_expression` in Go, `call` in Python, `method_invocation` in Java, and the default `call_expression` in all 499 other languages — without the rule author knowing or caring about any of that.

**The hard work is done.** You write rules in simple YAML using concept names. The engine handles 509 languages for you.

## Writing Rules

Rules live in 5 YAML files, one per active tier. YAML is the source of truth — adding a rule is a new entry, no Go changes needed. Rules are embedded at compile time.

### Minimal Example

```yaml
- id: todo_fixme
  label: TODO/FIXME/HACK/XXX marker in source
  dimension: debug
  tier: observability
  bit: 1
  severity: info
  text_patterns: ["TODO", "FIXME", "HACK", "XXX"]
```

Six lines. Runs across all 509 languages. Finds TODO markers in any file.

### Detection Layers

Each rule can use one, two, or all three detection layers:

**Layer 1 — Text patterns.** Fast literal string matching. Scans the full file in one pass.

**Layer 2 — Structural.** AST-aware matching using concepts. Finds code patterns that text matching can't express — like "a defer statement inside a loop" or "a function longer than 100 lines."

**Layer 3 — Regex.** Confirmation pass that reduces false positives from broad text matches.

The rule's **kind** is inferred from which layers are present:
- `text_patterns` only → **text** rule
- `structural` only → **structural** rule
- both → **composite** rule (all layers must agree within ±3 lines)

### Rule Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Unique identifier, e.g. `command_injection` |
| `label` | string | yes | Human-readable description |
| `dimension` | string | yes | Category within the tier, e.g. `injection`, `secrets` |
| `tier` | string | yes | `security`, `performance`, `quality`, `observability`, `architecture` |
| `bit` | int | yes | Bit position 0-63 within the tier (must be unique per tier) |
| `severity` | string | yes | `info`, `warning`, `high`, `critical` |
| `text_patterns` | []string | | Literal strings for Layer 1 |
| `structural` | object | | AST matching block for Layer 2 (see below) |
| `regex` | string | | Regex pattern for Layer 3 |
| `skip_test` | bool | | Skip test files |
| `skip_main` | bool | | Skip main/cmd packages |
| `code_only` | bool | | Only scan code files |
| `skip_langs` | []string | | Languages to skip entirely |
| `amplifier` | float64 | | Signal amplifier — bypasses density requirement (see scoring) |

### Structural Block

The `structural:` block matches AST nodes. All fields present must be satisfied.

| Field | Type | Description |
|-------|------|-------------|
| `match` | string | Concept to match: `call`, `assignment`, `function`, `defer`, `block`, `switch`, etc. |
| `receiver_contains` | []string | Node text must contain one of these (case-insensitive) |
| `inside` | string | Ancestor concept required, e.g. `for_loop` |
| `has_arg` | object | Argument constraint: `type` (concept list) and `text_contains` (substring list) |
| `name_contains` | []string | Identifier child must contain one of these |
| `without_sibling` | string | Semantic template: `comma_ok`, `doc_comment`, `after_return`, `error_check` |
| `nesting_threshold` | int | Nesting depth threshold |
| `child_count_threshold` | int | Child count threshold (context-sensitive: params for functions, cases for switches, fields for classes) |
| `text_contains` | []string | Node text must contain one of these (case-insensitive) |
| `line_threshold` | int | Line span threshold |

## Examples

### Text + Regex — Secret Detection

```yaml
- id: aws_credentials
  label: AWS credentials in source code
  dimension: secrets
  tier: security
  bit: 9
  severity: critical
  skip_test: true
  code_only: true
  regex: 'AKIA[0-9A-Z]{16}'
  text_patterns: ["AKIA", "aws_secret_access_key", "aws_access_key_id"]
```

Text finds candidates. Regex confirms the 20-character AWS key format.

### Structural — Code Pattern

```yaml
- id: defer_in_loop
  label: defer inside loop body
  dimension: resources
  tier: performance
  bit: 0
  severity: warning
  code_only: true
  structural:
    match: defer
    inside: for_loop
```

Finds `defer` inside a loop — pure AST matching. In languages without `defer`, no nodes match, zero false positives.

### Structural — Threshold

```yaml
- id: long_function
  label: Function exceeds 100 lines
  dimension: complexity
  tier: quality
  bit: 6
  severity: info
  code_only: true
  structural:
    match: function
    line_threshold: 100
```

Works across all 509 languages. `function` resolves to the right node type per grammar.

### Composite — Text + Structural

```yaml
- id: command_injection
  label: Potential command injection via exec/system call
  dimension: injection
  tier: security
  bit: 0
  severity: critical
  skip_test: true
  code_only: true
  text_patterns:
    - "exec.Command("
    - "os.system("
    - "subprocess.call("
  structural:
    match: call
    receiver_contains: ["exec.Command", "os.system", "subprocess.call"]
```

Text finds candidate lines. Structural confirms a `call` node with matching receiver exists within ±3 lines. Both must agree.

### Composite — Argument Constraints

```yaml
- id: sql_string_concat
  label: SQL query built via string concatenation
  dimension: injection
  tier: security
  bit: 20
  severity: critical
  skip_test: true
  code_only: true
  structural:
    match: call
    receiver_contains: [query, execute, exec, prepare, raw, cursor]
    has_arg:
      type: [string_concat, format_call, template_string]
      text_contains: ["SELECT", "INSERT", "UPDATE", "DELETE", "DROP"]
```

Matches a database call where an argument is a concatenated/formatted string containing SQL keywords. All concept types (`string_concat`, `format_call`) resolve universally across all 509 languages.

### All 3 Layers — Maximum Precision

```yaml
- id: hardcoded_secret
  label: Potential hardcoded secret or credential
  dimension: secrets
  tier: security
  bit: 8
  severity: critical
  skip_test: true
  code_only: true
  regex: '(?:password|passwd|secret|api_?key|private_key|access_token)\s*[:=]\s*[''"]?\S{4,}'
  text_patterns:
    - "password="
    - "secret="
    - "api_key="
  structural:
    match: assignment
    name_contains: ["password", "secret", "api_key", "token", "passwd"]
    value_type: string_literal
```

Text catches broad patterns. Structural confirms it's an assignment with a sensitive name. Regex validates the format. All three must agree.

## Adding a New Rule

1. **Pick tier and dimension.** See the reference below.
2. **Find an unused bit** (0-63) within that tier. Check the YAML file.
3. **Write the YAML entry.** Use concept names (`call`, `assignment`, `function`, etc.) in structural blocks — the concept layer handles all 509 languages automatically.
4. **Use `skip_langs` only if the rule doesn't make semantic sense** for certain languages.
5. **Test:**
   ```bash
   go test ./internal/domain/analyzer/ -v    # YAML loading, unique IDs/bits
   go test ./internal/adapters/recon/ -v     # engine pipeline
   ```

## Tier and Dimension Reference

### Security (tier 0) — 9 dimensions
| Dimension | Bits | Description |
|-----------|------|-------------|
| injection | 0-7, 20-25 | Command injection, SQL injection, XSS, path traversal, deserialization |
| secrets | 8-14 | Hardcoded passwords, API keys, private keys, connection strings |
| crypto | 15-18 | Weak hashes, insecure random, deprecated TLS, ECB mode |
| denial | 25 | Regex DoS |
| transport | 26-28 | Insecure TLS, disabled cert verify, HTTP URLs |
| exposure | 29-30 | Debug endpoints, CORS wildcards |
| config | 31-32, 37 | Hardcoded IPs, world-readable permissions, unsafe defaults |
| data | 33 | Sensitive data in logs |
| auth | 34-36 | Missing CSRF, insecure password comparison, missing headers |

### Performance (tier 1) — 5 dimensions
| Dimension | Bits | Description |
|-----------|------|-------------|
| resources | 0-4 | Defer in loop, unclosed handles, leaked contexts, resource alloc in loop |
| concurrency | 5-9 | Lock/goroutine/channel in loop, goroutine leaks, sync primitives |
| query | 10-14 | N+1 queries, exec in loop, unbounded selects, raw SQL |
| memory | 15-19 | Allocation in loop, append, string concat, regex compile in loop |
| hot_path | 20-25 | Reflection, JSON marshal in loop, fmt.Sprint in loop, sort in loop |

### Quality (tier 2) — 4 dimensions
| Dimension | Bits | Description |
|-----------|------|-------------|
| errors | 0-5, 20 | Ignored errors, panic in lib, unchecked assertions, deprecated stdlib |
| complexity | 6-9, 21-22 | Long functions, deep nesting, params, switch, god function, wide function |
| dead_code | 10-14 | Unreachable code, commented-out code, unused imports, empty body, disabled tests |
| conventions | 15-19, 23 | Doc comments, init side effects, magic numbers, boolean params, nested callbacks |

### Observability (tier 3) — 5 dimensions
| Dimension | Bits | Description |
|-----------|------|-------------|
| debug | 0-4 | Print statements, TODO markers, debug endpoints, verbose logging, sleep |
| silent_failures | 5-9 | Recovered panics, fire-and-forget goroutines, discarded errors |
| logging | 10-14 | Unstructured logs, sensitive log data, PII in logs |
| resilience | 15-19 | Panic in handler, os.Exit/log.Fatal in lib, signal/health hints |
| error_visibility | 20-24 | Bare error return, errors.New, error wrapping, sensitive in error |

### Architecture (tier 4) — 4 dimensions
| Dimension | Bits | Description |
|-----------|------|-------------|
| antipattern | 0-4 | Global state, god objects, singletons, hardcoded config, massive structs |
| imports | 5-9 | Excessive imports, banned imports, hexagonal violations, extreme imports |
| api_surface | 10-14 | Fat interfaces, exported surface area, method count, deep inheritance |
| coupling | 15-23 | Parameter explosion, deep nesting, method chains, handler length, init weight |

## Pipeline

```
Source file (any of 509 languages)
    │
    ├─ Layer 1: Text Scan ─────────────────────────────────────────────┐
    │  Aho-Corasick automaton, O(n) in file size.                     │
    │  Finds literal pattern matches with byte offsets.               │
    │                                                                  │
    ├─ Layer 2: Structural Walk ───────────────────────────────────┐  │
    │  tree-sitter parse → AST node walk.                          │  │
    │  Concepts resolve to concrete node types universally.        │  │
    │  Evaluates structural constraints per node.                  │  │
    │                                                               │  │
    └─ Layer 3: Regex Confirmation ────────────────────────────┐   │  │
       Per-line regex on candidates. Cuts false positives.     │   │  │
                                                                │   │  │
    ┌───────────────────────────────────────────────────────────┘   │  │
    ▼                                                               │  │
    Composite Resolution ◄──────────────────────────────────────────┘  │
    │  All present layers must agree within ±3 lines.                  │
    ▼                                                                  │
    Bitmask Composition ◄──────────────────────────────────────────────┘
    │  Findings → (tier, bit) → file bitmask.
    │  Attributed to function/method spans.
    ▼
    FileAnalysis { bitmask, methods[], findings[] }
```

## Method-Level Scoring (Signal Score)

After the pipeline produces per-file findings, they're attributed to enclosing methods via tree-sitter symbol spans. Each method accumulates a **per-line bitmask matrix** — a binary matrix of (lines × bits) showing which rules fired on which lines.

The unit of detection is the **method**, not the line. A method surfaces as a finding only when its **signal score** meets the gate threshold (≥ 3). The score combines six independent signals from the bitmask matrix:

### The Formula

```
score = base + co_occurrence + clustering + breadth
```

Where `base` is the sum of per-bit contributions, each modulated by severity and density.

### Component 1: Severity-Anchored Base

Each unique (tier, bit) pair that fires in the method contributes to the base score. The contribution depends on **severity** and **density** (fraction of method lines where this bit fires):

| Severity | Weight | Density Requirement |
|----------|--------|---------------------|
| `critical` | 10 | None — always full weight. One secret = one finding. |
| `high` | 7 | Floored at 50%. Even sparse high findings contribute ≥ 3.5. |
| `warning` | 3 | Scaled by density × 10, capped at 1.0. Needs ~10% density for full weight. |
| `info` | 1 | Scaled by density × 5, capped at 1.0. Needs ~20% density for full weight. |

**Why density modulation?** A single `info`-level finding in a 500-line method is noise. The same finding on 40% of lines is a pattern. Density distinguishes signal from noise for low-severity rules. Critical/high bypass density because their presence alone is significant.

### Component 2: Rule Amplifier

Some rules are categorically meaningful — their presence alone is actionable regardless of density. These carry an `amplifier` field in the YAML definition:

```yaml
- id: domain_imports_adapter
  severity: warning
  amplifier: 3.0    # presence alone = weight × 3.0 = 9.0, clears gate
```

Amplified rules contribute:

```
base_contribution = weight × amplifier + density × weight × 10
```

The amplifier provides a floor (always at least `weight × amplifier`), and density adds on top without capping. This means an amplified info rule at 30% density scores higher than a non-amplified warning at 1% density — which is the correct behavior for rules like `print_statement` (left-in debugging).

**Rules with amplifier:**

| Rule | Severity | Amplifier | Rationale |
|------|----------|-----------|-----------|
| `domain_imports_adapter` | warning | 3.0 | Hexagonal violation is always wrong |
| `banned_import` | warning | 2.0 | Deprecated import is always actionable |
| `panic_in_lib` | warning | 2.0 | Panic in library code is always wrong |
| `panic_in_handler` | warning | 2.0 | Panic in handler crashes the server |
| `os_exit_in_lib` | warning | 2.0 | os.Exit in library kills the caller |
| `fatal_log_in_lib` | warning | 2.0 | log.Fatal in library kills the caller |
| `recovered_panic_no_log` | warning | 2.0 | Silent panic recovery hides failures |
| `test_import_in_prod` | info | 2.0 | Test code in production is always wrong |
| `print_statement` | info | 1.5 | Left-in debugging, density makes it actionable |

### Component 3: Co-occurrence (+2.0 per pair)

When two or more distinct bits fire on the **same line**, they describe the same code statement. This is a compound finding — stronger than two independent hits.

Example: SQL concatenation (bit 0) + raw SQL without params (bit 2) on the same line = definite SQL injection, not two separate observations.

```
co_occurrence = Σ_lines[ pairs × 2.0 ]
  where pairs = n × (n-1) / 2 for lines with n ≥ 2 distinct bits
```

### Component 4: Clustering (+1.0 per unique bit in cluster)

Signal-bearing lines within ±2 lines of each other form a **cluster** — a single code block with concentrated issues. The bonus scales with the number of unique bits in the largest cluster:

```
clustering = unique_bits_in_largest_cluster × 1.0   (if cluster ≥ 2 lines)
```

Example: 3 different warning bits on lines 50, 51, 52 (adjacent) = cluster with 3 unique bits = +3.0 bonus. The same 3 bits scattered across lines 10, 150, 290 = no cluster = no bonus.

### Component 5: Breadth (+1.0 per warning bit beyond 2)

Many distinct findings in one method signal systematic debt, even if each individual finding has low density:

```
breadth = max(
    (warning_bits - 2) × 1.0,     if warning_bits ≥ 3
    (total_unique_bits - 3) × 0.5, if total_bits ≥ 4
)
```

Example: 4 distinct warning bits each at 1% density individually score ~0.3 each (base ≈ 1.2). But breadth adds 2.0, pushing total above the gate. The method has 4 different quality problems — that's worth reporting.

### Gate Threshold

Methods with signal score **≥ 3** surface as findings. Below that is suppressed as noise.

What clears the gate:
- Any single critical-severity rule (score = 10)
- Any single high-severity rule (score ≥ 3.5)
- Any amplified warning rule (e.g., hexagonal violation: 3 × 3.0 = 9)
- Dense warning pattern at ≥10% density (3 × 1.0 = 3)
- 3+ distinct warning bits with breadth bonus
- Co-occurring bits on the same line
- Clustered compound findings

What stays suppressed:
- 1 info finding in a 500-line method (score ≈ 0.01)
- 2 scattered warnings in a 400-line method (score ≈ 0.15)
- 3 sparse info hits across 200 lines (score ≈ 0.08)

### `SevHigh` → `warning` Collapse

The `high` severity level (weight 7) exists internally for scoring — it distinguishes rules that need less evidence than `warning` but aren't as categorical as `critical`. In dashboard output, `high` maps to `"warning"` to match the existing filter buttons (critical / warning / info). No UI changes needed.

### Severity vs Amplifier — Two Axes

| | Low Amplifier (0) | High Amplifier (2-3) |
|---|---|---|
| **Critical** | Always surfaces (weight 10 > gate 3) | N/A — critical doesn't need amplifier |
| **Warning** | Needs density/breadth/co-occurrence | Surfaces on presence alone |
| **Info** | Needs high density (≥20%) | Surfaces when dense enough (amplifier + density compound) |

Severity answers "**how bad** is this type of issue?" Amplifier answers "**how much evidence** do I need before reporting it?"
