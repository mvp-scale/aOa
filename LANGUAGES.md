# aOa-go Language Support

## Compiled-In Grammars (28 languages)

These grammars are statically linked via CGo and ship in the 4.9MB binary.

### Core (14)
| Language | Extensions | Grammar Source | Binary Size Contribution |
|----------|-----------|----------------|--------------------------|
| Python | .py .pyw | tree-sitter/tree-sitter-python | 3.3MB C → compiled |
| JavaScript | .js .jsx .mjs .cjs | tree-sitter/tree-sitter-javascript | 2.8MB C → compiled |
| TypeScript | .ts .mts | tree-sitter/tree-sitter-typescript | 8.4MB C → compiled |
| TSX | .tsx | tree-sitter/tree-sitter-typescript | 8.4MB C → compiled |
| Go | .go | tree-sitter/tree-sitter-go | 1.5MB C → compiled |
| Rust | .rs | tree-sitter/tree-sitter-rust | 6.2MB C → compiled |
| Java | .java | tree-sitter/tree-sitter-java | 2.5MB C → compiled |
| C | .c .h | tree-sitter/tree-sitter-c | 3.7MB C → compiled |
| C++ | .cpp .hpp .cc .cxx .hxx | tree-sitter/tree-sitter-cpp | 17MB C → compiled |
| C# | .cs | tree-sitter/tree-sitter-c-sharp | 34MB C → compiled |
| Ruby | .rb | tree-sitter/tree-sitter-ruby | 15MB C → compiled |
| PHP | .php | tree-sitter/tree-sitter-php | 6.9MB C → compiled |
| Kotlin | .kt .kts | tree-sitter-grammars/tree-sitter-kotlin | 22MB C → compiled |
| Scala | .scala .sc | tree-sitter/tree-sitter-scala | 26MB C → compiled |

### Scripting & Shell (2)
| Language | Extensions | Grammar Source |
|----------|-----------|----------------|
| Bash | .sh .bash .zsh | tree-sitter/tree-sitter-bash |
| Lua | .lua | tree-sitter-grammars/tree-sitter-lua |

### Functional (2)
| Language | Extensions | Grammar Source |
|----------|-----------|----------------|
| Haskell | .hs .lhs | tree-sitter/tree-sitter-haskell |
| OCaml | .ml .mli | tree-sitter/tree-sitter-ocaml |

### Systems & Hardware (3)
| Language | Extensions | Grammar Source |
|----------|-----------|----------------|
| Zig | .zig | tree-sitter-grammars/tree-sitter-zig |
| CUDA | .cu .cuh | tree-sitter-grammars/tree-sitter-cuda |
| Verilog | .sv | tree-sitter/tree-sitter-verilog |

### Web & Frontend (3)
| Language | Extensions | Grammar Source |
|----------|-----------|----------------|
| HTML | .html .htm | tree-sitter/tree-sitter-html |
| CSS | .css .scss .less | tree-sitter/tree-sitter-css |
| Svelte | .svelte | tree-sitter-grammars/tree-sitter-svelte |

### Data & Config (4)
| Language | Extensions | Grammar Source |
|----------|-----------|----------------|
| JSON | .json .jsonc | tree-sitter/tree-sitter-json |
| YAML | .yaml .yml | tree-sitter-grammars/tree-sitter-yaml |
| TOML | .toml | tree-sitter-grammars/tree-sitter-toml |
| HCL | .tf .hcl | tree-sitter-grammars/tree-sitter-hcl |

---

## Extension-Mapped (29 languages)

These languages have extension mappings so files are **indexed via tokenization**, but tree-sitter structural parsing isn't available yet. When upstream repos fix their `bindings/go`, we add one import line to enable rich parsing.

**Awaiting upstream bindings/go fixes:**
- **R** (.r .R) — bindings exist but scanner.c not included (linker error)
- **Swift** (.swift) — bindings incomplete
- **Julia** (.jl) — no bindings/go directory
- **Markdown** (.md .mdx) — no bindings/go directory
- **Vue** (.vue) — no bindings/go directory
- **Elixir** (.ex .exs) — repo exists, no bindings/go
- **Erlang** (.erl .hrl) — repo exists, no bindings/go
- **Dart** (.dart) — repo exists, no bindings/go
- **Nim** (.nim) — repo exists, no bindings/go
- **Clojure** (.clj .cljs .cljc) — repo exists, no bindings/go
- **D** (.d) — repo exists, no bindings/go
- **Gleam** (.gleam) — repo exists, no bindings/go
- **Elm** (.elm) — repo exists, no bindings/go
- **PureScript** (.purs) — repo exists, no bindings/go
- **Odin** (.odin) — repo exists, no bindings/go
- **V** (.v) — repo exists, no bindings/go
- **Ada** (.ada .adb .ads) — repo exists, no bindings/go
- **Fortran** (.f90 .f95 .f03 .f) — repo exists, no bindings/go
- **Fennel** (.fnl) — repo exists, no bindings/go
- **Groovy** (.groovy .gradle) — repo exists, no bindings/go
- **GraphQL** (.graphql .gql) — repo exists, no bindings/go
- **CMake** (.cmake) — repo exists, no bindings/go
- **Make** (.mk) — repo exists, no bindings/go
- **Nix** (.nix) — repo exists, no bindings/go
- **Objective-C** (.m .mm) — repo exists, no bindings/go
- **VHDL** (.vhd .vhdl) — repo exists, no bindings/go
- **GLSL** (.glsl .vert .frag) — repo exists, no bindings/go
- **HLSL** (.hlsl) — repo exists, no bindings/go
- **SQL** (.sql) — repo exists, no bindings/go
- **Dockerfile** (Dockerfile .dockerfile) — repo exists, no bindings/go

---

## Runtime Loading (Future: S-02b)

Once the purego `.so` loader is implemented (Phase 6, S-02b), users can:

1. Download pre-compiled grammars from GitHub Releases (C-03)
2. Drop `.so` files in `.aoa/grammars/`
3. Restart daemon
4. All extensions above become fully parseable

This enables access to **all ~1,050 tree-sitter grammars** without recompiling the binary.

---

## Python Parity Status

| Metric | Python aOa | Go aOa | Status |
|--------|-----------|--------|--------|
| Languages with symbol extraction | 47 | 28 | 🟡 60% parity |
| File extensions mapped | 97 | 101 | ✅ 104% parity |
| Grammar access | 165+ via tree-sitter-language-pack | 28 compiled + unlimited via .so (S-02b) | 🟡 Needs S-02b |
| Performance | 50-200ms/file | 0.2ms/file | ✅ 250-1000x faster |
| Binary size | N/A (Python) | 4.9MB | ✅ |

---

## How to Add a Language

### If upstream has bindings/go:
```bash
go get github.com/{org}/tree-sitter-{lang}/bindings/go@latest
```
Edit `languages.go`, add:
```go
import ts_{lang} "github.com/{org}/tree-sitter-{lang}/bindings/go"
// In registerBuiltinLanguages():
p.addLang("{lang}", langPtr(ts_{lang}.Language()))
// In registerExtensions():
p.addExt("{lang}", ".ext")
```

### If upstream doesn't have bindings/go (S-02b required):
1. Compile grammar to `.so`: `tree-sitter build`
2. Drop in `.aoa/grammars/{lang}.so`
3. Daemon loads via purego at runtime

---

**Last updated:** 2026-02-16
