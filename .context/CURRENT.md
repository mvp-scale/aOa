# Session 85 | 2026-03-01 | L8 Recon Scoring + Noise Reduction

> **Session**: 85

## Done

- [x] L8: `serveDimensionalResults()` rewritten -- iterates `fa.Methods` with score gate instead of `fa.Findings`
- [x] L8: `SevHigh` collapsed to `"warning"` in dashboard output
- [x] L8: `Symbol` field set to `method.Name` on emitted findings
- [x] L5/L8: Formula P scoring algorithm designed and validated -- 36/36 scenarios
- [x] L5/L8: `SignalScore()` implemented in `score.go` -- replaces `Score()` for method-level scoring
- [x] L5/L8: `Amplifier float64` field added to Rule struct + YAML schema
- [x] L5/L8: Amplifier values added to 9 priority YAML rules
- [x] L5/L8: `Compose()` updated to use `SignalScore()` with per-method line spans
- [x] L8: Scoring documented in `recon/rules/README.md`
- [x] L8: Gitignore-aware indexing -- `git ls-files` replaces `filepath.Walk` hardcoded skipDirs (79% of findings from `_tmp_sitter_forest/`)
- [x] L8: Generated file detection -- `// Code generated ... DO NOT EDIT` in first 2KB skipped (`languages_forest.go` = 509 FPs)
- [x] L8: `error_not_checked` replaced by `empty_catch_body` -- structural rule, cross-language (4,406 FPs eliminated)
- [x] L8: Index membership filtering in `loadDimensionalFromStore` -- prunes stale bbolt entries
- [x] L8: `ConceptCatchClause` added to lang_map with Python/Ruby overrides
- [x] L8: Gate raised to 6 (MinMethodScore in `recon.go`, gate in `score.go`)
- [x] L8: `cmd/dim-dump/` tool -- exports method-level bbolt data as JSON fixture
- [x] L8: `TestRealWorldDistribution` -- fixture-driven scoring test (0.28s, no rebuild)
- [x] L8: 92% finding reduction: 49,838 -> 4,149

## Decisions

- Noise was in source data, not formula: gitignore, generated files, bad rules
- `error_not_checked` unfixable without type info -- replaced, not patched
- Gate 6 (raised from 3) based on real-world score distribution
- `dim-dump` tool for repeatable fixture generation from live data

## Next

- [ ] Commit all S85 changes
- [ ] Consider more rules for `amplifier` values based on real-world results
- [ ] L5.Va: per-rule detection accuracy tests (synthetic fixtures)
- [ ] L8.6: source line editor view (not started, needs design conversation)
