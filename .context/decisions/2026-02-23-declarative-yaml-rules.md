# ADR: Declarative YAML-Driven Dimensional Rules

**Date:** 2026-02-24
**Status:** Accepted
**Relates to:** `docs/research/bitmask-dimensional-analysis.md`

## Context

The dimensional analysis engine answers 142 yes/no questions about every file in a project. Each question maps to a bit in a per-method bitmask across 6 tiers and 21 dimensions. Three detection layers run at index time:

1. **AC automaton** — single-pass text scan over raw source bytes, ~115 literal patterns
2. **Structural walker** — traverses tree-sitter AST, matches parameterized shapes via declarative YAML
3. **Regex confirmation** — targeted second-stage on AC candidate lines only

Rules live in YAML files embedded via `//go:embed`. One rule definition covers all 28 tree-sitter grammars through `lang_map` concept resolution.

Previous implementation attempts deviated by replacing the declarative structural schema with hardcoded Go function pointers (`structural_check: checkFuncName`). This ADR locks in the correct approach.

---

## Decision

### 1. YAML Schema — Common Fields

Every rule has these top-level fields:

```yaml
- id: sql_string_concat           # unique rule ID (snake_case)
  bit: 0                           # position in tier bitmask (0-63)
  severity: critical               # info | warning | high | critical
  dimension: injection             # dimension within the tier
  tier: security                   # security | performance | quality | architecture | observability | compliance
  label: SQL query built via string concatenation
  code_only: true                  # skip non-code files (optional, default false)
  skip_test: true                  # skip test files (optional, default false)
  skip_main: false                 # skip main packages (optional, default false)
  skip_langs: [rust]               # languages where this rule doesn't apply (optional)
```

### 2. YAML Schema — Detection Blocks

Detection is specified by **one or more** of these blocks on the same rule. No explicit `kind:` field — the loader infers it:

- **text** — has `text_patterns` only
- **structural** — has `structural` block only
- **composite** — has both `text_patterns` AND `structural` block

#### Text patterns (AC automaton — Layer 1)

Language-agnostic literal strings matched in a single pass:

```yaml
  text_patterns:
    - "exec.Command("
    - "os.system("
    - "subprocess.call("
```

#### Structural patterns (AST walker — Layer 2, declarative)

The `structural` block declares **what AST shape to match**, not a Go function to call:

```yaml
  structural:
    match: call                    # unified concept → lang_map resolves per language
    receiver_contains: [query, execute, exec, prepare, raw, cursor]
    has_arg:
      type: [string_concat, format_call, template_string]
      text_contains: ["SELECT", "INSERT", "UPDATE", "DELETE", "DROP"]
```

The walker resolves `match: call` to language-specific node kinds via `lang_map.go` (already exists, 10 languages). The structural block is **data** interpreted by a generic evaluator. There is NO switch-case per rule ID. There is NO `structural_check: funcName` field.

#### Per-rule lang_map overrides (optional)

```yaml
  lang_map:
    go: { call: call_expression, string_concat: binary_expression }
    python: { call: call, string_concat: binary_operator }
    rust: { call: call_expression, format_call: macro_invocation }
```

Override the global `lang_map.go` for this specific rule. ~80% of rules use the global map; ~20% need overrides.

#### Regex confirmation (optional — Layer 3, post-AC)

```yaml
  regex: "AKIA[0-9A-Z]{16}"       # second-stage on AC candidate lines
```

### 3. Three-Layer Validation Example

A single rule combining all three layers:

```yaml
- id: aws_credentials
  bit: 20
  severity: critical
  dimension: secrets
  tier: security
  label: AWS credentials in source code
  text_patterns:                   # Layer 1: AC finds candidate lines
    - "AKIA"
  regex: "AKIA[0-9A-Z]{16}"       # Layer 3: Regex confirms exact pattern
  structural:                      # Layer 2: AST confirms it's in an assignment
    match: assignment
    value_type: string_literal
```

AC scan → regex confirm → structural validate. All present layers must agree.

### 4. Structural Pattern Templates

The ~8-10 core reusable templates (from research doc §Layer 1):

| Template | YAML fields | What it matches |
|----------|-------------|-----------------|
| call_with_arg | `match: call` + `receiver_contains` + `has_arg` | Function call where receiver matches and arg has specific shape |
| call_inside_loop | `match: call` + `inside: loop` | Call nested inside for/while/loop body |
| assignment_with_literal | `match: assignment` + `name_contains` + `value_type` | Assignment where LHS matches pattern, RHS is literal type |
| call_without_sibling | `match: call` + `without_sibling` | Call missing an expected companion call (open without close) |
| nesting_depth | `match: block` + `nesting_threshold` | Nesting exceeds threshold (>4 levels) |
| branch_count | `match: function` + `child_count_threshold` | Too many branches/cases/params in function body |
| literal_in_context | `match: literal` + `parent_kinds` + `text_contains` | Literal in specific structural context |
| return_without_wrap | `match: return` + `without_wrap` | Return with bare error variable (no wrapping) |

### 5. Generic Walker Evaluator

The walker interprets `structuralBlock` at runtime:

```
For each AST node:
  For each structural rule:
    1. Resolve rule.Structural.Match → node kind(s) via lang_map
       - Check per-rule lang_map override first, then global lang_map.go
    2. If current node kind ∉ resolved kinds → skip
    3. Check all constraints present on the rule:
       - receiver_contains → callee/receiver text substring match
       - inside → verify ancestor matches concept (e.g., loop)
       - has_arg → walk argument children for type + text_contains
       - name_contains → check identifier children for substring
       - value_type → check RHS node type
       - without_sibling → verify no matching sibling exists
       - nesting_threshold → count nesting depth, fire if exceeded
       - child_count_threshold → count matching children, fire if exceeded
       - text_contains → node text substring match
    4. All constraints pass → emit finding with (tier, bit, severity, line)
```

**New rule = new YAML entry. No Go code changes.**

### 6. Go Types

```go
type yamlRule struct {
    ID           string                       `yaml:"id"`
    Label        string                       `yaml:"label"`
    Dimension    string                       `yaml:"dimension"`
    Tier         string                       `yaml:"tier"`
    Bit          int                          `yaml:"bit"`
    Severity     string                       `yaml:"severity"`
    CodeOnly     bool                         `yaml:"code_only,omitempty"`
    SkipTest     bool                         `yaml:"skip_test,omitempty"`
    SkipMain     bool                         `yaml:"skip_main,omitempty"`
    SkipLangs    []string                     `yaml:"skip_langs,omitempty"`
    TextPatterns []string                     `yaml:"text_patterns,omitempty"`
    Regex        string                       `yaml:"regex,omitempty"`
    Structural   *structuralBlock             `yaml:"structural,omitempty"`
    LangMap      map[string]map[string]string `yaml:"lang_map,omitempty"`
}

type structuralBlock struct {
    Match               string   `yaml:"match"`
    ReceiverContains    []string `yaml:"receiver_contains,omitempty"`
    Inside              string   `yaml:"inside,omitempty"`
    HasArg              *argSpec `yaml:"has_arg,omitempty"`
    NameContains        []string `yaml:"name_contains,omitempty"`
    ValueType           string   `yaml:"value_type,omitempty"`
    WithoutSibling      string   `yaml:"without_sibling,omitempty"`
    NestingThreshold    int      `yaml:"nesting_threshold,omitempty"`
    ChildCountThreshold int      `yaml:"child_count_threshold,omitempty"`
    ParentKinds         []string `yaml:"parent_kinds,omitempty"`
    TextContains        []string `yaml:"text_contains,omitempty"`
}

type argSpec struct {
    Type         []string `yaml:"type"`
    TextContains []string `yaml:"text_contains,omitempty"`
}
```

These types live in `internal/domain/analyzer/`. The `Rule` struct gains a `Structural *StructuralBlock` field (exported) replacing the old `StructuralCheck string` field.

### 7. File Layout

```
recon/                             # repo root, standalone package
  embed.go                         # //go:embed rules/*.yaml — var FS embed.FS
  rules/
    security.yaml                  # 9 dimensions, ~67 questions
    performance.yaml               # 4 dimensions, ~19 questions
    quality.yaml                   # 4 dimensions, ~22 questions
    architecture.yaml              # 3 dimensions, ~14 questions
    observability.yaml             # 2 dimensions, ~9 questions
    compliance.yaml                # 3 dimensions, ~11 questions

internal/domain/analyzer/
    types.go                       # Tier/Bitmask/Rule types, StructuralBlock
    rules.go                       # LoadRulesFromFS — YAML loader
    rules_security.go              # Hardcoded fallback (existing, not primary)
    lang_map.go                    # Global concept → node kind resolution (existing)
    score.go                       # Scoring (existing, unchanged)

internal/adapters/recon/
    engine.go                      # AC + structural walker orchestration
    scanner.go                     # Old scanner (kept, not wired)

internal/adapters/treesitter/
    walker.go                      # Generic structural evaluator
```

### 8. Dimensions (21 total across 6 tiers)

| Tier | Dimensions |
|------|-----------|
| Security (9) | injection, secrets, auth, crypto, transport, exposure, config, data, denial |
| Performance (4) | resources, concurrency, query, memory |
| Quality (4) | errors, complexity, dead_code, conventions |
| Architecture (3) | antipattern, imports, api_surface |
| Observability (2) | debug, silent_failures |
| Compliance (3) | cve_patterns, licensing, data_handling |

---

## What's Already Built (Keep As-Is)

| File | What | Status |
|------|------|--------|
| `analyzer/types.go` | TierCount=6, TierArchitecture, TierCompliance, TierFromName, SeverityFromName | Done |
| `analyzer/rules_security.go` | Corrected tier assignments, FilterTextRules/FilterStructuralRules | Done |
| `socket/protocol.go` | `[6]uint64` bitmask DTOs | Done |
| `app/app.go` | dimEngine/dimRules fields, init lifecycle, dispatch | Done |
| `app/dim_engine.go` | warmDimCache, updateDimForFile, convertFileAnalysis | Done |
| `app/watcher.go` | updateReconOrDimForFile dispatch | Done |
| `web/recon.go` | Data-driven inferTierDim, SetRuleIndex, inferLabel | Done |
| `web/static/app.js` | 21 dimensions, 7 tiers (6 + investigated), compliance | Done |
| `web/static/style.css` | Compliance tier CSS (`--tier-comp`) | Done |
| `treesitter/walker.go` | Hardcoded check functions (fallback during transition) | Done |

## What Needs Rework

| File | What changes |
|------|-------------|
| `recon/embed.go` | **Move** from `internal/adapters/reconrules/` to `recon/` at repo root |
| `recon/rules/*.yaml` (all 6) | **Rewrite** — replace `structural_check: funcName` + `kind:` with declarative `structural:` block. Move to `recon/rules/` |
| `analyzer/rules.go` | **Update** `yamlRule` struct to match §6. Infer kind from blocks present. Convert `Structural` to exported `StructuralBlock` on `Rule`. |
| `analyzer/types.go` | **Update** `Rule` struct — replace `StructuralCheck string` with `Structural *StructuralBlock`. Add `Regex`, `SkipLangs`, `LangMap` fields. |
| `treesitter/walker.go` | **Add** generic structural evaluator per §5. Keep hardcoded checks as fallback. |
| `recon/engine.go` | **Update** — pass structural block data + per-rule lang_map to walker. Add regex confirmation layer. |
| `analyzer/rules_test.go` | **Update** imports — `reconrules` → `recon` |

---

## Constraints

1. **No `structural_check: funcName`** — structural rules use declarative `structural:` blocks, never Go function name strings
2. **No explicit `kind:` field in YAML** — inferred from which detection blocks are present
3. **Embed at `recon/`** — standalone package at repo root, like `atlas/v1/`, no dependency on `internal/`
4. **Generic walker** — interprets structural blocks at runtime, no switch-case per rule ID
5. **lang_map resolution** — per-rule `lang_map:` overrides global `lang_map.go` when present
6. **Three layers** — AC text, structural AST, regex confirmation. Present layers must all agree.

## Consequences

- New detection pattern = new YAML entry, zero Go changes
- Cross-language support automatic through lang_map concept resolution
- Three-layer validation eliminates false positives
- ~8-10 core structural templates cover ~80% of questions; ~20% need skip_langs or lang_map overrides
- Existing hardcoded walker checks remain as fallback during transition, then retire
