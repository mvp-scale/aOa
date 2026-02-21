# 2026-02-20 — Decision: Two-Binary Distribution (aoa + aoa-recon)

**Date**: 2026-02-20 (Session 62–63)
**Status**: Implemented
**Author**: Corey

---

## The Problem

aOa ships as a single binary containing everything: search engine, learning system, dashboard, tree-sitter grammars for 28+ languages, and security scanning. This creates three tensions:

1. **Size** — The binary is 76–80 MB. Most of that weight is tree-sitter grammars compiled via CGo. Users who want fast code search are downloading a security scanner they may never use. Users evaluating aOa bounce at 80 MB.

2. **CGo dependency** — Tree-sitter requires CGo. This means cross-compilation is painful (need per-platform C toolchains), CI builds are slow, and the binary can't be statically linked. Pure Go binaries are portable, fast to build, and trivially cross-compiled.

3. **Scope coupling** — The search engine, which needs to be rock-solid and fast, shares a binary with experimental scanning features. A bug in the tree-sitter grammar for Gleam shouldn't be able to crash your search daemon.

## The Decision

Split into two binaries:

| Binary | What it does | Size | CGo |
|--------|-------------|------|-----|
| **aoa** | Search, learning, dashboard, daemon | ~8 MB | No |
| **aoa-recon** | Tree-sitter parsing, security scanning | ~73 MB | Yes |

`aoa` works standalone. `aoa-recon` enhances `aoa` when present.

## Why Two, Not Three or More

We considered splitting further (aoa-dashboard, aoa-learn, etc.) but the search engine, learner, and dashboard are tightly coupled through the in-memory index and bbolt database. Splitting them would mean IPC overhead for every search query. The natural fault line is CGo: everything that needs C goes in one binary, everything that doesn't goes in the other.

## How They Communicate

**Shared bbolt database.** Both binaries read/write the same `.aoa/aoa.db` file. `aoa` builds the token index (file-level). `aoa-recon` enhances it with symbol metadata (function names, line ranges, types). No socket protocol, no RPC, no shared memory. Just a file.

This is the simplest possible integration. bbolt handles concurrent readers. Write contention is managed by the caller (aoa holds the lock while running; aoa-recon runs as a one-shot subprocess that opens/closes the DB).

## Discovery Model

`aoa` finds `aoa-recon` the same way `git` finds `git-lfs`:

1. `exec.LookPath("aoa-recon")` — on PATH (npm global install)
2. `.aoa/bin/aoa-recon` — project-local (vendored or per-project install)
3. Same directory as `aoa` binary — sibling (manual download)

Zero configuration. If `aoa-recon` is present, `aoa` invokes it after indexing. If not, everything still works — just without symbols.

## What Users Lose Without aoa-recon

| Feature | aoa only | aoa + aoa-recon |
|---------|----------|-----------------|
| File-level search | Works | Works |
| Symbol-level search | No — file hits only | Full: function names, line ranges, types |
| Learning | Works | Works |
| Dashboard (all tabs) | Works | Works |
| Recon tab | Shows install prompt | Full scanner results |
| Binary size | 8 MB | 8 MB + 73 MB |

The only regression is symbol-level search. File-level search (tokenization-only) still returns relevant files ranked by token frequency. For most agent workflows (grep for a string, find files), this is sufficient. Symbol search is an enhancement.

## Distribution via npm

npm's `optionalDependencies` with `os`/`cpu` constraints (the pattern used by esbuild, turbo, and swc):

```
npm install -g aoa           # 8 MB, works immediately
npm install -g aoa-recon     # 73 MB, aoa auto-discovers it
```

Per-project install also works (`npm install --save-dev`). Platform-specific packages mean npm only downloads the binary for your OS/arch.

## Future Plan

### Near-term (next 2–3 sessions)

- **Validate integration** — Build both binaries, test the full flow: init → grep (file-level) → recon enhance → grep (symbol-level) → daemon with auto-discovery → dashboard
- **Wire scanner into aoa-recon enhance** — Currently aoa-recon only writes symbol metadata. It should also run the pattern scanner and write findings to bbolt, so the Recon tab works even when the dashboard can't scan files itself
- **`aoa-recon scan` command** — Standalone scan output (JSON or human-readable) without needing the daemon

### Medium-term

- **Full bitmask engine in aoa-recon** — The L5.1–L5.5 dimensional analysis engine (AST walker, AC text scanner, bitmask composer) belongs in aoa-recon. The lightweight 10-pattern scanner stays in aoa as a fallback
- **Incremental enhancement via watcher** — When `aoa` detects a file change and aoa-recon is available, call `aoa-recon enhance-file` to update symbols for that file. Already wired but needs testing under load
- **Grammar management moves to aoa-recon** — `aoa-recon grammar list/install` instead of `aoa grammar`. The pure aoa binary has no use for grammar management

### Long-term

- **npm publish pipeline** — Tag push triggers GitHub Actions: build 8 binaries (2 x 4 platforms), publish to npm registry. Already defined in `.github/workflows/release.yml`, needs first real tag to validate
- **Homebrew tap** — `brew install aoa` and `brew install aoa-recon` as an alternative to npm
- **Version coordination** — Both binaries share the same version tag. `aoa --version` and `aoa-recon --version` should always match in a release. Goreleaser handles this via the shared git tag

## Key Insight

The split isn't about reducing binary size (though that's nice). It's about **decoupling the stable core from the experimental edge**. Search and learning are mature — 470+ tests, behavioral parity proven. Scanning is nascent — 10 patterns, no AST engine yet. Shipping them separately means we can iterate on scanning without risking search stability, and users can adopt search immediately without waiting for scanning to mature.
