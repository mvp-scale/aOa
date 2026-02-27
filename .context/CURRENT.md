# Session 78 | 2026-02-27 | Arsenal Dashboard + Test Strategy

> **Session**: 78

## Now

- [ ] Formal deploy for aOa and Recon
- [ ] Figure out installation guide (L4.4)
- [ ] Understand integration between aOa and Recon for test strategy

## Done

- [x] Arsenal: Daily Token Usage -- split-bar design (green actual bottom, red saved top), "now" marker, removed ghost bars, CSS bar height fix
- [x] Arsenal: Sessions Extended metric -- switched to totalSaved / burn_rate_per_min
- [x] Arsenal: Learning Curve -- session number ticks (S1, S2...), reversed chronological order, axis label
- [x] Arsenal: Guided column -- color-coded ratio-wrap (green >= 60%, yellow >= 30%, red below)
- [x] Arsenal: Session History table -- added # column, Opus/Sonnet/Haiku % columns, removed Prompts/Reads/Cost/Focus, added Total Tokens
- [x] Backend: per-model token tracking (ModelTokens map in SessionSummary, accumulated in app.go onSessionEvent)
- [x] Recon: fixed investigated_files missing from /api/recon response (both paths)
- [x] Recon: fixed build.sh --recon-bin missing -tags "recon"
- [x] Recon: built and connected aoa-recon binary, ran aoa recon init
- [x] Ported split-bar Daily Token Usage chart from mockup to live dashboard
- [x] Fixed chart card layout/sizing (340px row height, padding, Learning Curve canvas height, footer symmetry)
- [x] Fixed Learning Curve cost-per-prompt (smoothed 3-session window averages instead of single-point comparison)
- [x] Full regression run: 535 tests pass, 0 fail, 0 skip across 7 phases
- [x] Deleted 26 stale test stubs (index_test.go, format_test.go, parity_test.go -- all empty t.Skip placeholders)
- [x] Fixed CLI integration tests: added `testing` build tag to build guard, all 50 CLI tests pass
- [x] Updated test/README.md with full 8-phase test pipeline docs
- [x] Documented regression baseline: test/testdata/regression-2026-02-27.md
- [x] Board updated with test counts (535 pass, 0 fail, 0 skip)

## Decisions

- Split-bar chart design over log scale or side-by-side bars (savings gap visible at scale)
- Per-model tokens: accumulate by ev.Model in onSessionEvent, backward compatible with old sessions
- Learning Curve cost-per-prompt: use smoothed 3-session window averages, not first-vs-last
- Build guard tag: `!lean && !recon && !testing` -- integration tests pass `-tags testing` to compile test binary

## Next

- Formal deploy (aOa + Recon)
- L4.4: Installation docs
- Understand aOa/Recon integration for test strategy
- Recon tab QOL pass (last dashboard tab)
- L5.Va: per-rule detection validation across all 5 tiers
- L5.10/L5.11: dimension scores + query support
- L8.2-5: remaining Va gaps (browser-only, unit tests)
- L8.6: recon source line editor view
- L7.1: startup progress timing test
