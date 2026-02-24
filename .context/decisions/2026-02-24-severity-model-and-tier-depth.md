# ADR: Severity Model and Tier Depth

**Date:** 2026-02-24
**Status:** Proposed
**Supersedes:** None (extends `2026-02-23-declarative-yaml-rules.md`)

## Problem

The recon dashboard produces noise, not signal. Quality shows 15,000 findings. Architecture shows 4,000. These numbers destroy confidence — users see a wall of yellow and stop trusting the tool.

Root causes:

1. **51 of 86 rules (59%) are text-only.** They grep for strings like `"var "`, `"func ("`, `"return err"`, `".Lock()"`. These fire on nearly every file in any codebase. A text match is not a finding — it's a candidate.

2. **Shallow tiers can't differentiate.** Security has 36 bits across 12 dimensions. A file can score anywhere from 1 to 36 on that tier, creating a meaningful gradient. Performance has 16 bits across 4 dimensions. Quality has 17 across 4. With so few bits, the score compresses — most files land at the same level, and the dashboard can't distinguish "mildly messy" from "structurally deficient."

3. **Severity levels are assigned without rigor.** Text-only rules at `warning` severity contribute 12% to the score bar per hit. Three text-only warnings = 36% score = borderline red. But a text match for `".Lock()"` is not a warning — it's an observation. The severity doesn't match the confidence level.

The result: the bitmask captures breadth (many bits set) but not signal (which bits actually matter). Recon should identify methods and files where **multiple structural concerns co-occur** — where the bitmask pattern suggests a real problem, not where a string appeared.

## Design Principles

### 1. Structural confirmation earns severity

A rule's severity must correspond to its detection confidence:

| Severity | Detection Required | Confidence | Dashboard Behavior |
|----------|-------------------|------------|-------------------|
| **critical** | Composite (text + structural) or structural with multiple constraints | High — AST confirms the pattern exists in the code structure | Always visible, red indicator |
| **warning** | Structural-only with at least one constraint, or composite | Medium — AST shape matches but may have benign explanations | Visible by default, yellow indicator |
| **info** | Text-only, or structural with minimal constraints | Low — pattern exists in text, not confirmed structurally | Hidden by default, available on expand |

**No text-only rule may be `warning` or `critical`.** If we can't confirm it in the AST, we can't be confident enough to alert on it. Text-only rules serve as Layer 1 candidates for composite rules, or as `info`-level observations.

### 2. The bitmask earns its score through co-occurrence

A single finding is not recon. Recon is when multiple dimensions light up on the same method or file — when the bitmask pattern tells a story:

- A method with `command_injection` + `tainted_path_join` + `exec_with_variable` = **injection cluster** → high confidence this method handles external input unsafely
- A method with `long_function` + `nesting_depth` + `too_many_params` = **complexity cluster** → high confidence this method needs refactoring
- A file with `global_state` + `god_object` + `excessive_imports` = **architecture cluster** → high confidence this module has design problems

The value is in the combination. Each individual bit is a question. The bitmask across a method is the answer. Scoring should reward co-occurrence across dimensions, not just count individual hits.

### 3. Minimum tier depth for meaningful gradient

Each tier needs enough bits to create a score gradient that distinguishes files. Target: **24-36 rules per tier** across **6-8 dimensions**. This gives:

- Enough bits that a file with 3 concerns scores differently from a file with 12
- Enough dimensions that co-occurrence across dimensions is meaningful
- Enough structural/composite rules that the warning/critical band has real content

### 4. Text patterns are candidates, not findings

Text-only rules exist for two purposes:
1. **Layer 1 pre-filter** for composite rules (text finds candidates, structural confirms)
2. **Info-level observations** for users who want maximum detail

They never drive the score. They never trigger yellow or red. They are the breadcrumb trail that the structural layer validates.

## Proposed Tier Structure

### Tier reduction: 6 → 5

Compliance is removed as a tier. Static analysis cannot deliver compliance confidence — real compliance is process, policy, and audit, not code shape. CVE detection requires a continuously updated vulnerability database; a static YAML file is a snapshot that goes stale immediately. Rather than ship misleading signals, we redistribute compliance's viable rules into tiers that can back them up structurally:

| Former compliance rule | New home | Rationale |
|----------------------|----------|-----------|
| PII in logs (`pii_in_log`) | Security → data | It's a data leak, structurally confirmable |
| Sensitive data in errors (`sensitive_in_error`) | Security → exposure | Information exposure |
| Deprecated stdlib APIs (`deprecated_stdlib`) | Quality → errors | Stable signal — deprecated APIs don't un-deprecate |
| Unsafe defaults (`unsafe_defaults`) | Security → config | Misconfiguration |
| DB operations without transactions | Performance → query | Data integrity |
| Known vuln patterns (`known_vuln_pattern`) | Drop | Stale without live CVE feed |
| License headers (`missing_license_header`) | Drop | Grep, not recon |
| GPL contamination (`gpl_contamination`) | Drop | Grep, not recon |

The bitmask remains `[6]uint64` — slot 5 stays reserved for future use. The 5 active tiers are: Security, Performance, Quality, Architecture, Observability.

### Security (current: 36 rules, 12 dimensions) — Baseline

Already meets the target. 12 composite rules, 8 text+regex, strong structural coverage. Minor cleanup: tighten text-only rules to info, absorb compliance's `pii_in_log`, `sensitive_in_error`, and `unsafe_defaults`.

### Performance (current: 11 rules, 4 dimensions → target: ~30 rules, 7 dimensions)

| Dimension | Current | Gap |
|-----------|---------|-----|
| resources | 4 rules (3 text-only) | Promote to composite — text finds `os.Open(`, structural confirms no `defer` or `Close()` sibling |
| concurrency | 3 rules (all text-only) | Promote to composite — text finds `.Lock()`, structural confirms no `.Unlock()` in same block |
| query | 2 rules (all text-only) | Promote to composite — text finds `.Query(`, structural confirms inside `for_loop` |
| memory | 2 rules (all text-only) | Promote to composite — text finds `make([]byte,`, structural confirms inside `for_loop` |
| **io** | *new* | Structural — buffered I/O patterns, sync file operations in hot paths |
| **scheduling** | *new* | Structural — timer misuse, ticker without stop, sleep in handlers |
| **caching** | *new* | Composite — cache patterns without TTL, unbounded map growth |

### Quality (current: 16 rules, 4 dimensions → target: ~30 rules, 7 dimensions)

Quality detects structural deficiency — not individual style violations, but patterns that indicate code is poorly organized, hard to maintain, or likely generated without design intent. The structural signals compound: a file with 280 methods averaging 80 lines each, 40 imports, and deep nesting is not 4 separate problems — it's one structural diagnosis. The bitmask co-occurrence across quality dimensions is the detection mechanism for low-cohesion code.

| Dimension | Current | Gap |
|-----------|---------|-----|
| errors | 6 rules (well-structured) | Mostly structural already — keep, add error wrapping checks |
| complexity | 4 rules (all structural) | Strong — add cyclomatic proxy, parameter object pattern |
| dead_code | 3 rules (all text-only) | Promote — `unreachable_code` is structural, `commented_out_code` stays info. Add: empty function bodies (structural — function with 0 statements) |
| conventions | 3 rules (1 structural, 2 text) | Promote `init_side_effects` to composite |
| **cohesion** | *new* | Structural — file-level method count threshold, average method length, method-to-import ratio. Detects monolithic files where everything is in one place with no separation of concerns |
| **testing** | *new* | Structural — test functions without assertions, test helpers without `t.Helper()` |
| **consistency** | *new* | Structural — mixed error handling patterns in same file (some wrapped, some bare), inconsistent receiver names |

All thresholds (line counts, method counts, parameter counts) are set to industry defaults in the YAML and are user-adjustable. A team that accepts 200-line functions changes `line_threshold: 200` in the rule file.

### Architecture (current: 9 rules, 3 dimensions → target: ~24 rules, 6 dimensions)

| Dimension | Current | Gap |
|-----------|---------|-----|
| antipattern | 4 rules (3 text-only) | `global_state` with `"var "` fires on everything — must be structural (match: assignment at package scope) |
| imports | 3 rules (2 text-only) | Keep, add circular dependency detection |
| api_surface | 2 rules (1 structural, 1 text) | `leaking_internal_type` with `"func ("` is pure noise — needs structural |
| **layering** | *new* | Structural — handler calling repo directly, business logic importing HTTP |
| **coupling** | *new* | Structural — excessive parameters from other packages, shared mutable state |
| **size** | *new* | Structural — file line count, package fan-out, method count per type |

### Observability (current: 7 rules, 2 dimensions → target: ~24 rules, 5 dimensions)

| Dimension | Current | Gap |
|-----------|---------|-----|
| debug | 4 rules (all text-only) | Demote all to info. `todo_fixme` without `code_only` matches changelogs |
| silent_failures | 3 rules (all text-only) | Promote to composite — text finds `recover()`, structural confirms no log call in same block |
| **logging** | *new* | Structural — no structured logging, missing context fields, inconsistent log levels |
| **metrics** | *new* | Composite — handler without duration measurement, retry without counter |
| **tracing** | *new* | Composite — outbound call without span, missing trace context propagation |

## Scoring Adjustment

The current formula:

```
score_pct = min(100, critical × 30 + warning × 12 + info × 4)
```

With the severity model above, `info` rules (text-only) should contribute minimally or not at all to the visual score. Proposed:

```
score_pct = min(100, critical × 25 + warning × 10 + info × 0)
```

Info findings are still stored, still queryable, still visible on expand. They just don't color the dashboard. The score represents **structural confidence**, not text occurrence count.

## Implementation Phases

**Phase 1: Tighten existing rules.** Demote all text-only `warning`/`critical` rules to `info`. Promote rules that can be made composite (add structural block to existing text patterns). This alone cuts the noise — no new rules needed.

**Phase 2: Expand dimensions.** Add new structural and composite rules to reach 24-30 per tier. Each new rule must have structural confirmation to be `warning` or above.

**Phase 3: Score recalibration.** Adjust the dashboard formula. Info at weight 0. Test against real codebases to validate that the gradient is meaningful — files with known problems should score higher than clean files.

## Constraints

1. **No text-only rule at warning or critical.** The AST must confirm the pattern.
2. **Every new rule must use structural or composite detection.** Text-only rules are info-level only.
3. **Universal concept layer handles all languages.** No per-rule language mapping.
4. **Bitmask depth per tier: minimum 24 bits.** Fewer bits = compressed gradient = noise.
5. **Co-occurrence across dimensions is the signal.** A single dimension lighting up is observation. Multiple dimensions = recon finding.
6. **Thresholds are industry defaults, user-adjustable.** All numeric thresholds (line counts, method counts, parameter counts, nesting depth) are set to industry best practices in the YAML. Users modify the rule file to adjust — no Go changes, no config system, the YAML is the config.
7. **Info is available, not visible.** Info findings are stored, queryable, and expandable. They do not drive the dashboard score or color. Users opt in to see info-level detail. The default view shows warning and critical only — actionable signal.

## Consequences

- Dashboard noise drops dramatically (info hidden by default, text-only can't trigger yellow/red)
- Score becomes a confidence indicator, not a count
- Rule authors must think structurally — "what AST shape proves this problem exists?"
- Each tier has enough depth that the bitmask gradient is meaningful
- Users trust the tool because yellow means "look at this" and red means "fix this"
- Quality tier detects structural deficiency through co-occurrence — monolithic files, low cohesion, poor separation of concerns light up multiple dimensions simultaneously
- Compliance removed — static analysis can't deliver compliance confidence. Viable rules redistributed to tiers that can structurally verify them. No misleading signals.
