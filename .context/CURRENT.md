# Session 78 | 2026-02-27 | Arsenal Dashboard + Recon Fixes

> **Session**: 78

## Now

- [ ] Port split-bar chart from mockup to live app.js
- [ ] Commit Session 78 work

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
- [x] Board updated with Session 78 work

## Decisions

- Split-bar chart design over log scale or side-by-side bars (savings gap visible at scale)
- Per-model tokens: accumulate by ev.Model in onSessionEvent, backward compatible with old sessions

## Next

- Port split-bar chart from mockup to live app.js
- Recon tab QOL pass (last dashboard tab)
- L5.Va: per-rule detection validation across all 5 tiers
- L5.10/L5.11: dimension scores + query support
- L8.2-5: remaining Va gaps (browser-only, unit tests)
- L8.6: recon source line editor view
- L4.4: installation docs
- L7.1: startup progress timing test
