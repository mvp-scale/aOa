# Session 83 | 2026-02-28 | L4.4 Phase 3: Pre-built Grammar Distribution

> **Session**: 83

## Context

S82 built `aoa init` grammar setup flow (compile from source via download.sh). User identified that compiling locally is slow, fragile, and requires a C compiler — antithetical to zero-friction install. New approach: distribute pre-built .so/.dylib files from `grammars/` folder in the aOa repo. Weekly CI compiles all grammars, commits binaries. `aoa init` downloads what the project needs, verifies SHA-256, done. No compilation, no C compiler, seconds instead of minutes.

## Done

- [x] Board updated: G2 goal, Mission, L4.4 terminology, end-to-end flow, Phase 3 restructured, key decisions, L10 descriptions
- [x] INDEX.md updated: active layer, unblocked tasks, layer status
- [x] CURRENT.md: new session

## In Progress

- [x] L4.4-3.1: Update `grammar-validation.yml` — new "Organize grammars directory" step copies binaries to `grammars/{platform}/`, commit step moves parsers.json + GRAMMAR_REPORT.md into `grammars/`, removes old root-level copies
- [x] L4.4-3.2: Create `grammars/README.md` — directory structure, parsers.json schema, SHA verification, how aoa init uses it
- [x] Removed old `grammars/manifest.json` (superseded by parsers.json)
- [ ] L4.4-3.3: Simplify `aoa init` — fetch parsers.json, show download plan, user confirms, download + verify + copy
- [ ] L4.4-3.4: Remove obsolete compile-from-source code (download.sh, grammar compile, GitHub API source listing)
- [ ] L4.4-3.5: `aoa init --update` — compare installed SHAs vs latest, download only changes
- [ ] L4.4-3.6: End-to-end verify

## Decisions

- Pre-built .so distribution replaces compile-from-source (S83 direction change)
- `grammars/` folder in mvp-scale/aOa repo — not a separate repo, not GitHub Releases
- 4 platform subdirs: linux-amd64/, linux-arm64/, darwin-arm64/, darwin-amd64/
- parsers.json fetched fresh every `aoa init` — not embedded in binary (would drift)
- Git LFS for binary .so/.dylib files in the grammars/ folder
- Grammar report is the validation gate — nothing lands without passing weekly CI
- S82 code reused: loadParsersJSON, scanProjectLanguages, matchParsersJSON, checkInstalledGrammars, grammarExtMap
- S82 code superseded: generateDownloadSh, runGrammarCompile, checkSourceDownloaded, forestRawURL

## Next

- L4.4 Phase 4: npm platform packages (4.1), release workflow (4.3), e2e test (4.4)
- L5.Va: per-rule detection validation
