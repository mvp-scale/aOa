# BUGFIX.md

Active bug tracker. Report bugs here, fix in batches.

## Open

| # | Bug | Severity | Status | Notes |
|---|-----|----------|--------|-------|
| 1 | Lean build broken — `dim_engine.go` missing `//go:build !lean` | critical | fixed | Added tag + lean stub |
| 2 | Binary bloat 366MB — built without lean flags | critical | fixed | Use `make build-pure` |
| 3 | Recon scanner runs unconditionally in `WarmCaches` | high | fixed | Gated behind `reconBridge.Available()` |
| 4 | Recon API inline fallback scans all files on every request | high | partial | Returns empty JSON but recon tab still not showing install prompt |
| 5 | Legacy "Learn aOa +N keywords" in Live tab activity feed | medium | fixed | Removed `pushActivity` block in observer.go |
| 6 | `App.dimEngine` typed as `*recon.Engine` breaks lean | critical | fixed | Changed to `any` |
| 7 | Daemon log says "recon cached" when no recon exists | medium | open | Pure aOa should not mention recon at all in logs |
| 8 | Learner state destroyed by db wipe during troubleshooting | critical | open | 1702 prompts of training data lost; not recoverable |
| 9 | Recon tab does not show install prompt in pure mode | high | open | Should show "npm install @mbp-scale/aoa-recon" when recon not installed |
| 10 | `Reindex()` calls `warmReconCache()` unconditionally | high | open | Line 2024 in app.go — same bug as #3 but in the reindex path |
| 11 | `updateReconOrDimForFile` calls old scanner without checking recon availability | medium | open | Line 2103 in app.go — file watcher path |
| 12 | `./aoa` command not in permissions — requires manual approval every time | low | open | Settings has `aoa:*` but not `./aoa:*` |
| 13 | `head` command keeps prompting despite being in allow list | low | open | Unclear why permission keeps re-prompting |
| 14 | Debrief: assistant text truncated to 500 chars, thinking to 2000 | medium | open | `turnFromBuilder` at app.go:1637-1638 — user expects full text in debrief |
| 15 | File change triggers full symbol rebuild for all files | high | open | `updateReconForFile` calls `BuildFileSymbols(a.Index)` for entire index on every file change, holds reconMu lock, blocks dashboard |
| 16 | Daemon log has no operational events — only startup/shutdown | high | open | File changes, reindexes, searches, hangs, errors — all invisible. No way to diagnose runtime issues from the log. |
| 17 | No debug mode exists | critical | open | Need `AOA_DEBUG=1` or `--debug` flag that logs all internal events: file changes, reindex, searches, lock contention, recon scans, timings. Immediate need. |

## Pending Verification

- [ ] Rebuild lean binary with ALL fixes
- [ ] `go test ./internal/app/ -count=1`
- [ ] Recon tab shows install prompt (not blank, not data) in pure mode
- [ ] Live tab clean — no "Learn aOa" entries
- [ ] Daemon log has zero recon references in pure mode
- [ ] Stale docstring in observer.go references "Pushes a Learn activity"

## Pattern: Recon Entanglement

Recon code is deeply wired into aOa core. Every path that touches files has a recon fallback:
- `WarmCaches` → `warmReconCache()` (bug #3, fixed)
- `handleRecon` API → inline `recon.Scan()` (bug #4, partial)
- `Reindex()` → `warmReconCache()` (bug #10, open)
- `onFileChanged` → `updateReconOrDimForFile` → `updateReconForFile` (bug #11, open)

**Root cause**: recon was built as a mandatory subsystem, not an optional addon. There is no single "recon enabled" flag — each call site independently decides whether to run recon. The fix needs to be architectural: a single `reconEnabled()` check that all paths respect, or better, the recon scanner code should not exist in the lean binary at all.

## Pattern: Session Destruction

Claude repeatedly killed the daemon, wiped the database, and restarted during troubleshooting without preserving user data. The learner state (1702 prompts) was destroyed. Backups should be taken before any destructive operation.

## Pattern: Build Discipline

Claude used `go build` directly instead of `make build-pure` three times, producing 366MB binaries. The Makefile exists specifically to prevent this. Always use `make` targets.
