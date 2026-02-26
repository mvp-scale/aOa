# Session 75 | 2026-02-26 | Telemetry + QOL

> **Session**: 75

## Now

- [x] Tool result size tracking -- 7 files, full pipeline tailer->parser->reader->app->socket->dashboard
- [x] Throughput formula fix -- throughput >= conversation speed guaranteed
- [x] Telemetry model document -- 8 parts, captures full data hierarchy and proposed phases

## Done

- Implemented tool result size tracking (tailer/parser, ports/session, claude/reader, app/app, socket/protocol, web app.js, web index.html)
- Fixed throughput formula: `max(outputTokens, textBasedTokens) + resultTokens`
- Conversation Speed = text only (user + assistant + thinking), Throughput = max(API, text) + tool results
- Created `details/2026-02-26-throughput-telemetry-model.md` (8-part telemetry reference)
- Raw character count as universal measurement unit -- never convert to tokens at capture time
- All tests pass, build clean, daemon verified live with real result_chars data

## Decisions

- Tool result text extraction measures ONLY content string, not JSON metadata
- Phased telemetry roadmap: Phase 0 (done) -> Phase 1 (ContentMeter) -> Phase 2 (persisted) -> Phase 3 (subagent) -> Phase 4 (burst)
- Shadow pattern for counterfactual extends existing readSavings/burnRateCounterfact system

## Next

- Phase 1: ContentMeter (unified char+timestamp capture)
- Continue QOL dashboard walk-through (Intel, Debrief, Arsenal tabs)
- Daemon restart reliability (stops aren't always clean)
- L5.7/8/16-19: per-rule detection validation (Va gaps)
- L5.10/L5.11: dimension scores in search results + query support
- L8.1-4: remaining Va gaps (bitmask, browser-only, unit tests)
- L8.6: recon source line editor view
- L4.4: installation docs
- L7.1: startup progress timing test
