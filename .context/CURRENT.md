# Session 79 | 2026-02-27 | Dynamic Grammar Download System

> **Session**: 79

## Now

- [ ] L10.8: Build all 509 grammars with `scripts/build-grammars.sh --all` and verify
- [ ] L10.8: Create GitHub release `grammars-v1` with .so for all 4 platforms
- [ ] L10.8: Wire CI workflow for cross-platform grammar builds
- [ ] L10.9: End-to-end test on fresh project

## Done

- [x] L10.1: Core build tier -- `./build.sh --core` with tree-sitter C runtime, zero compiled-in grammars
- [x] L10.2: Grammar paths wired into parser -- `newParser(root)` configures DefaultGrammarPaths, all callers updated
- [x] L10.3: No outbound network -- removed Go HTTP downloader, `aoa init` prints curl commands instead
- [x] L10.4: Grammar build script -- `scripts/build-grammars.sh` compiles from go-sitter-forest C source (11 grammars, 11 MB)
- [x] L10.5: `aoa init` as single command -- scans project, detects languages, shows curl for missing grammars, removed aoa-recon dependency
- [x] L10.6: Command rename -- `wipe` -> `reset` (clear data) + `remove` (full uninstall), `wipe` kept as hidden alias
- [x] L10.7: deploy.sh updated to use `./build.sh --core`
- [x] TSX fix: removed soFileOverrides["tsx"] = "typescript" -- each grammar gets own .so
- [x] L8.5 updated: dashboard empty state now shows "Run aoa init" instead of "npm install aoa-recon"
- [x] Repo renamed: remote origin updated from mvp-scale/aOa-go to mvp-scale/aOa

## Decisions

- Single binary with dynamic grammars over two-binary model (G2 goal updated)
- No outbound network from binary -- curl commands for full user transparency
- Core build tier = C runtime + zero grammars (between pure Go and full recon)
- `aoa init` replaces `aoa recon init` as sole entry point

## Next

- L10.8: Build all 509 grammars + GitHub release
- L10.9: End-to-end test on fresh project
- L4.4: Installation docs (now needs L10 context)
- L5.Va: per-rule detection validation across all 5 tiers
- L5.10/L5.11: dimension scores + query support
- L8.2-5: remaining Va gaps (browser-only, unit tests)
- L8.6: recon source line editor view
