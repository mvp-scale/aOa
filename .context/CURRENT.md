# Session 79 | 2026-02-27 | Installation Guide + Grammar Pipeline

> **Session**: 79

## Now

- [ ] Check CI and grammar validation workflow results
- [ ] Write installation guide (docs/INSTALL.md)
- [ ] Expand manifest from ~59 to all 510 grammars with maintainer data
- [ ] Wire `aoa init` to fetch pre-built .so from GitHub releases
- [ ] Implement `aoa grammar list` with maintainer/upstream attribution

## Done

- [x] build.sh simplified: default = tree-sitter + dynamic grammars, --light = pure Go, --recon/--core deprecated
- [x] Everything project-scoped under {root}/.aoa/ -- fixed loader.go, init.go, grammar_cgo.go
- [x] Advisory Rule added to CLAUDE.md
- [x] `aoa init` compiles grammars locally from go-sitter-forest C source (gcc, progress+ETA, 11 grammars in 10.6s)
- [x] All 509 grammars validated: scripts/validate-grammars.sh, 509/509 passed, dist/parsers.json produced
- [x] Weekly grammar validation workflow: .github/workflows/grammar-validation.yml (Sun 6am UTC, 3 platforms, cppcheck)
- [x] CI aligned with build.sh: standard (core tags) + light (lean tags), cmd/aoa-recon excluded
- [x] Grammar source strategy decided: go-sitter-forest = aggregated source, parsers.json = aOa-approved list
- [x] dist/ added to .gitignore

## Decisions

- npm install path: `npm install -g @mvpscale/aoa` (lightweight binary only)
- Grammar .so pre-compiled in GitHub Actions with SLSA provenance
- parsers.json = aOa-approved list (maintainer, upstream, sha256, security scan, cross-platform)
- build.sh is guardrails for Claude; ci.yml is the real build pipeline
- Advisory Rule: surface better approaches early, reference goals, let user choose

## Next

- L4.4: npm package update (binary only)
- L4.4: User config file (.aoa/languages) for manual grammar selection
- L10.8/L10.9: absorbed into L4.4 grammar pipeline
- L5.Va: per-rule detection validation
- L5.10/L5.11: dimension scores + query support
