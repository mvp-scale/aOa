# Session Bug Log — 2026-02-24

Bugs introduced or discovered during the daemon log lifecycle overhaul.

## BUG-1: Lean build broken (FIXED)
- `dim_engine.go` created without `//go:build !lean` tag
- Imported `treesitter` and `recon` (CGo packages) unconditionally
- Lean/pure build (`CGO_ENABLED=0 -tags lean`) failed to compile
- **Fix**: Added `//go:build !lean` to `dim_engine.go`, created `dim_engine_lean.go` stub
- **Guard**: Added 15MB size gate to CI and `make check`

## BUG-2: Binary bloat — 366MB instead of ~8MB (FIXED)
- Built with `go build -o aoa` instead of `make build-pure`
- Missing `-tags lean -ldflags "-s -w"` and `CGO_ENABLED=0`
- Compiled in all 510 tree-sitter grammars from go-sitter-forest
- **Fix**: Use `make build-pure` for lean, `make build` for full

## BUG-3: Recon pattern scanner runs unconditionally (FIXED)
- `WarmCaches` ran `warmReconCache()` (text pattern scanner on all files) when dimEngine was nil
- Even lean builds with no recon binary scanned every file with regex patterns
- Added 43s to startup and populated recon tab with data that shouldn't exist
- **Fix**: Gated `warmReconCache()` behind `a.reconBridge.Available()`

## BUG-4: Recon API inline fallback scan (FIXED)
- `handleRecon` in web/recon.go had a "safety net" fallback that ran `recon.Scan()` inline
- When no cached recon result existed, every API request triggered a full scan
- Recon tab showed data even when recon was supposed to be offline
- **Fix**: Return empty `{"recon_available": false}` when no recon data exists

## BUG-5: Legacy "Learn aOa +N keywords" activity entry (FIXED in observer.go, test update in progress)
- `observer.go:100-107` pushed a "Learn" activity entry with keywords/terms/domains counts
- This was superseded by the Learned column in the activity feed
- Showed up in the Live tab as stale legacy data
- **Fix**: Removed the `pushActivity` block and unused imports, updated tests

## BUG-6: `dimEngine` field type breaks lean compilation (FIXED)
- `App.dimEngine` was typed as `*recon.Engine` — a type gated behind `//go:build !lean`
- Changed to `any` so the struct compiles in lean mode
- `dim_engine.go` (!lean) casts to `*recon.Engine` internally

## STILL TODO
- [ ] Rebuild lean binary with all fixes and verify
- [ ] Run tests: `go test ./internal/app/ -count=1`
- [ ] Verify recon tab shows "not available" with lean build
- [ ] Verify Live tab no longer shows "Learn aOa +keywords" entries
- [ ] Also check: stale comment in observer.go docstring references "Pushes a Learn activity"
