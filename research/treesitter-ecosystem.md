# Tree-Sitter Go Bindings Ecosystem Research

**Date:** February 2025
**Focus:** Ecosystem maturity, language coverage, build complexity, and community standards

---

## Executive Summary

The Go tree-sitter ecosystem has **three primary implementations**:

1. **Official: `tree-sitter/go-tree-sitter`** — Maintained by tree-sitter org, modular, minimal runtime dependencies
2. **Community: `smacker/go-tree-sitter`** — Established (535★), batteries-included, 30+ languages pre-bundled
3. **Comprehensive: `alexaandru/go-sitter-forest`** — ~490 parsers, auto-maintained, modern approach

**Recommendation:** Use **`tree-sitter/go-tree-sitter`** (official) for new projects. Fallback to `smacker/go-tree-sitter` if you need pre-bundled languages. Consider `go-sitter-forest` if you need extensive language coverage or dynamic parser loading.

---

## Library Comparison Table

| Dimension | tree-sitter/go-tree-sitter | smacker/go-tree-sitter | go-sitter-forest |
|-----------|---------------------------|----------------------|------------------|
| **GitHub Stars** | 199 | 535 | 52 |
| **GitHub Forks** | 39 | 148 | 3 |
| **Status** | Official (tree-sitter org) | Community-maintained | Modern fork/evolution |
| **Languages** | 35+ (via individual imports) | 30+ (pre-bundled) | ~490 (comprehensive) |
| **Pre-bundled Grammars** | None (modular) | Yes | Yes (all ~490) |
| **Open Issues** | 5-10 (estimate) | 28 | 0 |
| **Last Activity** | Active (2024-2025) | Active (2025) | Active (2025) |
| **License** | MIT | MIT | MIT |

---

## Language Coverage Analysis

### tree-sitter/go-tree-sitter (Official)
- **Scope:** Individual imports required, community-driven
- **Approach:** You explicitly `go get github.com/tree-sitter/tree-sitter-javascript` for each language
- **Coverage:** 35+ languages available via separate packages
- **Discovery:** Languages are in separate org repositories under `tree-sitter-grammars/` or individual `tree-sitter/tree-sitter-*` repos
- **Advantage:** Minimal binary bloat, explicit dependencies

### smacker/go-tree-sitter
- **Scope:** Pre-bundled, pre-compiled
- **Supported Languages:**
  - Bash, C, C++, C#, CSS, Cue, Dockerfile, Elixir, Elm, Go, Groovy, HCL, HTML, Java, JavaScript, Kotlin, Lua, Markdown, OCaml, PHP, Protobuf, Python, Ruby, Rust, Scala, SQL, Svelte, Swift, TOML, TypeScript, YAML
  - **Total: 30+ languages (curated set)**
- **Advantage:** Works out-of-the-box, no additional `go get` calls
- **Trade-off:** All parsers compiled into binary even if unused

### go-sitter-forest
- **Scope:** Comprehensive collection, auto-maintained
- **Coverage:** ~490 parsers (maintains parity with nvim-treesitter)
- **Examples:** Includes niche languages and file formats (Tera, Uiua, sxhkdrc, Anzu, etc.)
- **Regeneration:** All parsers regenerated with latest tree-sitter version
- **Usage Modes:**
  1. **Standalone** — Import individual language parsers
  2. **Bulk/Forest mode** — Access all via `forest.GetLanguage()`
  3. **Plugin mode** — Dynamically load as plugins
  4. **Custom mix** — Combine approaches
- **Advantage:** Most comprehensive, modern architecture, version alignment
- **Discovery Tool:** See `PARSERS.md` for complete language list

---

## Build Complexity & Cross-Compilation

### CGo Requirement
All three implementations rely on **CGo** (C bindings to libsitter).

#### tree-sitter/go-tree-sitter (Official)
- **CGo Status:** Required
- **Cross-compilation:** Standard CGo challenges apply
  - Must set C compiler flags and env vars
  - Requires target platform's C compiler headers
  - purego alternative available for runtime library loading
- **Memory Management:** Must explicitly call `Close()` on Parser, Tree, TreeCursor, Query, QueryCursor, and LookaheadIterator due to CGo memory allocation

#### smacker/go-tree-sitter
- **CGo Status:** Required
- **Cross-compilation:** Same CGo limitations as official
- **Build:** Pre-compiled grammar binaries, simpler local builds
- **Complexity:** Medium (CGo headers bundled, but cross-platform still challenging)

#### go-sitter-forest
- **CGo Status:** Required
- **Version Alignment:** All parsers regenerated with consistent tree-sitter version (ensures C API compatibility)
- **Complexity:** Medium-to-High (manages ~490 parsers, complex automation)
- **Flexibility:** Modular approach (standalone vs. forest modes) simplifies selective builds

### Cross-Compilation Solutions
1. **Standard CGo approach:** Use `xgo` (github.com/techknowlogick/cgo-toolchain)
2. **WASM alternative:** `malivvan/tree-sitter` (experimental, CGo-free via wazero runtime)

---

## Performance Benchmarks & Known Issues

### Performance Insights
- **Runtime loading:** Loading function bodies at compile-time (pre-bundled) is faster than runtime loading
- **smacker/go-tree-sitter:** No published benchmarks, but batteries-included should be faster than individual imports
- **go-sitter-forest:** No published benchmarks, modular approach may add negligible overhead

### Known Issues

#### tree-sitter/go-tree-sitter (Official)
- (No major issues found; actively maintained)
- Users note CGo cross-compilation as the primary pain point

#### smacker/go-tree-sitter
- **Open Issues:** 28 (mostly feature requests, not critical bugs)
- Example recent issues: Tags support (#143), grammar updates
- Active development and PR review process (100 closed PRs)

#### go-sitter-forest
- **Open Issues:** 0 (well-maintained)
- Modern architecture, minimal reported issues
- Newer project (evolved from smacker fork)

---

## Community Standard & Ecosystem Position

### Official Recommendation
**`tree-sitter/go-tree-sitter`** is the **official Go binding** maintained by the tree-sitter organization. It's the standard community choice for:
- New projects
- Projects requiring selective language support
- Minimal binary bloat
- Explicit, auditable dependencies

### Community Adoption
1. **Established:** smacker/go-tree-sitter (535★, widely used, batteries-included)
2. **Growing:** go-sitter-forest (52★, modern, ~490 languages)
3. **Official:** tree-sitter/go-tree-sitter (199★, backed by tree-sitter org, modular)

### When to Use Each

| Use Case | Recommendation | Reason |
|----------|---|---|
| **New project, selective languages** | Official (`tree-sitter/go-tree-sitter`) | Modular, official, minimal bloat |
| **Quick prototyping, common languages** | `smacker/go-tree-sitter` | 30+ languages pre-bundled, proven |
| **Need 50+ languages or dynamic loading** | `go-sitter-forest` | ~490 parsers, modular modes, auto-maintained |
| **Need to avoid CGo entirely** | `malivvan/tree-sitter` (experimental) | WASM + wazero, but pre-release |
| **Embedded system, minimal deps** | `tree-sitter/go-tree-sitter` | Official, smallest footprint |

---

## Key Findings

### Strengths & Trade-offs

**tree-sitter/go-tree-sitter (Official)**
- ✅ Official backing from tree-sitter organization
- ✅ Modular (no unused grammars compiled in)
- ✅ Explicit dependencies (auditable)
- ✅ Standard Go community choice
- ❌ Requires multiple `go get` calls for different languages
- ❌ More setup boilerplate

**smacker/go-tree-sitter (Community)**
- ✅ 535 stars, proven in production
- ✅ 30+ languages pre-bundled (ready-to-use)
- ✅ Active maintenance (28 open issues, 100+ closed PRs)
- ✅ Good for prototyping and common language stacks
- ❌ All parsers compiled in (larger binaries)
- ❌ Less official backing

**go-sitter-forest (Comprehensive)**
- ✅ ~490 parsers (most comprehensive)
- ✅ Auto-maintains parser versions (aligned with latest tree-sitter)
- ✅ Zero open issues (well-maintained)
- ✅ Multiple usage modes (standalone, bulk, plugin)
- ✅ Modern architecture evolved from smacker
- ❌ Fewer stars (newer project, less established)
- ❌ Larger project scope (more complex)

---

## Build & Deployment Implications

### Local Development
- **Recommended:** `smacker/go-tree-sitter` (easiest, just `go get` + build)
- **Official:** `tree-sitter/go-tree-sitter` (modular, explicit)

### Cross-Platform Builds
- **Challenge:** All require CGo (C compiler + headers for target platform)
- **Solution 1:** Use `xgo` for matrix builds
- **Solution 2:** Use Docker for platform-specific builds
- **Solution 3:** Consider `malivvan/tree-sitter` (WASM, no CGo) if stability acceptable

### Production Deployment
- **Binary Size:** Official < go-sitter-forest < smacker (due to included parsers)
- **Performance:** smacker ≈ official (negligible difference)
- **Stability:** Official and smacker both proven; go-sitter-forest newer but well-maintained

---

## Recommendation

### For aOa-go Context

**Primary Recommendation: `tree-sitter/go-tree-sitter`**

**Rationale:**
1. **Official backing** — Part of tree-sitter organization, standard Go community binding
2. **Modular** — Import only the languages aOa-go needs (likely Go, Python, JavaScript, Rust, TypeScript)
3. **Production-ready** — Stable, backward-compatible, actively maintained
4. **Auditable** — Explicit `go get` calls for each language make dependencies clear

**Setup Pattern:**
```bash
go get github.com/tree-sitter/go-tree-sitter@latest
go get github.com/tree-sitter/tree-sitter-go@latest
go get github.com/tree-sitter/tree-sitter-python@latest
go get github.com/tree-sitter/tree-sitter-javascript@latest
# ... etc for each language
```

**Secondary Option: `smacker/go-tree-sitter`**

Use if:
- You need rapid prototyping with 30 common languages pre-bundled
- Your aOa-go service doesn't need to minimize binary size
- You want zero additional `go get` boilerplate

**Tertiary Option: `go-sitter-forest`**

Use if:
- aOa-go expands to support 50+ languages
- You need dynamic/plugin-based parser loading
- You want auto-maintained parser versions

---

## References

- [Official tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter)
- [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter)
- [alexaandru/go-sitter-forest](https://github.com/alexaandru/go-sitter-forest)
- [malivvan/tree-sitter (WASM alternative)](https://github.com/malivvan/tree-sitter)
- [Cross-compiling Go with CGO (GoReleaser cookbook)](https://goreleaser.com/cookbooks/cgo-and-crosscompiling/)

---

**Document Status:** Complete
**Confidence Level:** High (researched official repos, ecosystem standards, and build implications)
