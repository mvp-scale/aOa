# Session 83 | 2026-02-28 | L4.4 Phase 3: Pre-built Grammar Distribution

> **Session**: 83

## Context

S82 built `aoa init` grammar setup flow (compile from source via download.sh). User identified that compiling locally is slow, fragile, and requires a C compiler — antithetical to zero-friction install. New approach: distribute pre-built .so/.dylib files from `grammars/` folder in the aOa repo. Weekly CI compiles all grammars, commits binaries. `aoa init` generates a download.sh that the user reviews and runs. No network calls from the binary — security promise intact.

## Done

- [x] Board updated: G2 goal, Mission, L4.4 terminology, end-to-end flow, Phase 3 restructured with 6 tasks visible in active board, key decisions, L10 descriptions
- [x] INDEX.md updated: active layer, unblocked tasks, layer status
- [x] L4.4-3.1: `grammar-validation.yml` updated — new "Organize grammars directory" step commits compiled .so/.dylib to `grammars/{platform}/`, moves parsers.json + GRAMMAR_REPORT.md into `grammars/`. Verified: CI run 22535020780 green, all 509 grammars × 4 platforms committed.
- [x] L4.4-3.2: `grammars/README.md` created — directory structure, parsers.json schema, SHA-256 verification, how aoa init uses it
- [x] Removed old `grammars/manifest.json` (superseded by parsers.json)
- [x] L4.4-3.3: Simplified `aoa init` — generates download.sh that loops over grammars.conf, fetches parsers.json + pre-built .so files. User runs it. `aoa init` on re-run verifies SHA-256 and indexes. Zero network calls from binary.
- [x] L4.4-3.4 (partial): Removed embedded curl (`exec.Command("curl")`), `confirmDownload()`, `bufio` stdin prompt — violated SECURITY.md "zero outbound network" promise. Restored download.sh generation model.
- [x] `deploy.sh` + `build.sh`: now copy binary to `~/bin/aoa` if `~/bin` exists
- [x] Integration tests: `AOA_NO_GRAMMAR_DOWNLOAD=1` env var added to `runAOA` helper

## Verified End-to-End Flow

```
1. aoa init              → scans languages, generates download.sh + grammars.conf
2. sh .aoa/grammars/download.sh  → downloads parsers.json + .so files from aOa repo
3. aoa init              → SHA-256 verified, grammars ready, indexes project
```

Two user actions. Zero network from binary. download.sh is a clean loop over grammars.conf (~35 lines).

## Decisions

- Pre-built .so distribution replaces compile-from-source (S83 direction change)
- `grammars/` folder in mvp-scale/aOa repo — 4 platform subdirs, parsers.json, GRAMMAR_REPORT.md, README.md
- **No embedded downloader** — binary generates download.sh, user runs it. `exec.Command("curl")` violated SECURITY.md and was reverted.
- download.sh loops over grammars.conf (not per-grammar code generation)
- Two variants: full (no parsers.json yet — downloads manifest + .so) and partial (has parsers.json — downloads .so with SHA verification via download.sha256)
- Grammar report is the validation gate — nothing lands without passing weekly CI
- S82 code reused: loadParsersJSON, scanProjectLanguages, matchParsersJSON, checkInstalledGrammars, grammarExtMap, verifyInstalledGrammars (new)

## Not Yet Committed

- Reverted embedded curl → download.sh loop model (init_grammars.go rewrite)
- build.sh / deploy.sh ~/bin copy
- These changes are local, need commit + push

## Next

- [ ] Commit + push latest changes
- [ ] L4.4-3.5: `aoa init --update` — fetch fresh parsers.json, compare SHAs, regenerate download.sh for changes
- [ ] L4.4-3.6: End-to-end verify on fresh machine
- [ ] Consider convenience flag (e.g. `aoa init --download`) for streamlined flow
- [ ] L4.4 Phase 4: npm platform packages (4.1), release workflow (4.3), e2e test (4.4)
- [ ] L5.Va: per-rule detection validation
