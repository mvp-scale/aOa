# Session 79 | 2026-02-27 | Installation Guide + Grammar Pipeline

> **Session**: 79

## Now

- [ ] L4.4: Build all 510 grammars locally, verify every one compiles and loads
- [ ] L4.4: Update manifest with all 510 grammars (maintainer + upstream repo from go-sitter-forest)
- [ ] L4.4: GitHub Actions workflow to compile grammars per platform with SLSA provenance
- [ ] L4.4: `aoa init` flow — detect → install → index with progress + attribution
- [ ] L4.4: `aoa grammar list` shows maintainer and source for each grammar
- [ ] L4.4: Installation guide document (docs/INSTALL.md)

## Done

- [x] L10.1-L10.7: Core build tier, grammar paths, no-network, build script, aoa init, command renames, deploy.sh
- [x] build.sh simplified: default = tree-sitter + dynamic grammars, --light = pure Go, --recon deprecated
- [x] All data project-scoped under {root}/.aoa/ — nothing at ~/.aoa/
- [x] Advisory Rule added to CLAUDE.md
- [x] `aoa init` compiles missing grammars from go-sitter-forest source with gcc + progress/ETA
- [x] Grammar source validated: alexaandru/go-sitter-forest aggregates 510 languages from upstream tree-sitter repos

## Decisions

- Single install path: `npm install -g @mvpscale/aoa` (lightweight, binary only)
- Grammar .so files pre-compiled via GitHub Actions with SLSA provenance
- Grammar source: go-sitter-forest (aggregates official tree-sitter grammars)
- Maintainer attribution shown per grammar — we give credit to the people who built them
- Everything project-scoped: grammars, shims, data under {root}/.aoa/
- No outbound network from aoa binary itself
- Advisory Rule: surface better approaches early, reference goals, let user choose

## Next

- L4.4: npm package update (binary only, lightweight)
- L4.4: User config file (.aoa/languages) for manual grammar selection
- L10.8/L10.9: absorbed into L4.4 grammar pipeline
- L5.Va: per-rule detection validation
- L5.10/L5.11: dimension scores + query support
