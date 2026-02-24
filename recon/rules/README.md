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
