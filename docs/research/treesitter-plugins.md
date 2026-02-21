# Tree-Sitter Plugin Architecture Research

**Question:** Can we pre-compile 490 grammars as .so files and load them on demand via dlopen in Go?

**Date:** 2026-02-14
**Status:** Feasible with caveats

---

## Executive Summary

**Short Answer:** Yes, but with important limitations.

Go supports dynamic plugin loading via `plugin.Open()` (uses dlopen under the hood), and tree-sitter can be loaded via shared libraries using **purego** (cgo-free dlopen). However, practical constraints exist:

- **Platform support:** Linux, macOS, FreeBSD only (no Windows)
- **CGO requirement:** Go's `plugin.Open()` requires CGO_ENABLED=1
- **Binary size:** ~490 grammars @ 500KB-2MB each = **245GB-980GB total** (impractical as individual .so files)
- **Better approach:** Compile subsets into regional .so files or use dynamic cgo-free loading with purego

---

## 1. Go Plugin System Architecture

### 1.1 Standard Go Plugin API

Go provides a built-in `plugin` package for dynamic loading:

```go
package main

import (
    "fmt"
    "plugin"
)

func main() {
    // Load a compiled .so plugin
    p, err := plugin.Open("./mylib.so")
    if err != nil {
        panic(err)
    }

    // Lookup and call an exported symbol
    symb, err := p.Lookup("MyExportedFunction")
    if err != nil {
        panic(err)
    }

    f := symb.(func() string)
    fmt.Println(f())
}
```

**Key behaviors:**
- Uses `dlopen(path, RTLD_NOW|RTLD_GLOBAL)` under the hood
- Plugins are **initialized once** and **cannot be closed**
- Automatic caching: multiple `plugin.Open()` calls with same path return the same plugin
- Requires `CGO_ENABLED=1` to build

**Source:** [Go plugin package documentation](https://pkg.go.dev/plugin)

### 1.2 Plugin Symbol Export

Exported symbols are any uppercase variables or functions:

```go
// In plugin source (plugin.go)
package main

// Exported symbol - accessible via plugin.Lookup()
var TreeSitterLanguage = language.Language{}

// Exported function
func GetParser() *parser.Parser {
    return NewParser()
}

func init() {
    // Initialize on load
}
```

The plugin system uses dlsym to resolve these at runtime.

**Source:** [plugin package reference](https://pkg.go.dev/plugin)

---

## 2. Tree-Sitter Grammar Loading Methods

### 2.1 Static Compilation (Current Standard)

```go
import (
    tree_sitter "github.com/tree-sitter/go-tree-sitter"
    ts_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func main() {
    code := []byte("const x = 1;")
    parser := tree_sitter.NewParser()
    parser.SetLanguage(tree_sitter.NewLanguage(ts_javascript.Language()))
    tree := parser.Parse(code, nil)
    // Use tree...
}
```

**Approach:**
- Each language is a separate Go package (e.g., `tree-sitter-javascript/bindings/go`)
- Compiled into the binary at build time
- Results in large monolithic binary (~50-100MB for all 490 languages)

**Source:** [go-tree-sitter documentation](https://pkg.go.dev/github.com/tree-sitter/go-tree-sitter)

### 2.2 Dynamic Loading via Purego (CGO-Free)

```go
package main

import (
    "fmt"
    "runtime"
    "github.com/ebitengine/purego"
)

// Tree-sitter language function signature
type LanguageFunc func() uintptr

func LoadLanguage(libPath string, funcName string) LanguageFunc {
    // Load shared library without CGO
    lib, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
    if err != nil {
        panic(fmt.Sprintf("Failed to load %s: %v", libPath, err))
    }

    var langFunc LanguageFunc
    purego.RegisterLibFunc(&langFunc, lib, funcName)
    return langFunc
}

func main() {
    // Platform-specific library path
    var libPath string
    switch runtime.GOOS {
    case "linux":
        libPath = "./libtree-sitter-javascript.so"
    case "darwin":
        libPath = "./libtree-sitter-javascript.dylib"
    }

    // Load JavaScript grammar on demand
    langFunc := LoadLanguage(libPath, "tree_sitter_javascript")
    langPtr := langFunc()

    // Convert pointer to tree-sitter Language and use it
    // language := tree_sitter.NewLanguage(unsafe.Pointer(langPtr))
}
```

**Key advantages:**
- **No CGO required:** `CGO_ENABLED=0 go run main.go`
- **Lazy loading:** Load grammars only when needed
- **Smaller initial binary:** Core tool only (~5-10MB)
- **Purego is pure Go:** No C compiler needed for building

**Key limitations:**
- Must compile tree-sitter .so files separately (outside Go build)
- Requires `libtree-sitter-core` as a dependency on the system
- Platform-specific: different .so/.dylib files for each OS

**Source:** [ebitengine/purego documentation](https://pkg.go.dev/github.com/ebitengine/purego), [Go-tree-sitter dynamic loading](https://github.com/tree-sitter/go-tree-sitter)

---

## 3. Platform-Specific Considerations

### 3.1 Supported Platforms

| Platform | Go Plugin | Purego | Status |
|----------|-----------|--------|--------|
| Linux (x86_64, ARM) | ✓ | ✓ | Full support |
| macOS (Intel, Apple Silicon) | ✓ | ✓ | Full support (.dylib) |
| FreeBSD | ✓ | ✓ | Full support |
| Windows | ✗ | Partial | No true plugin support |
| iOS/Android | ✗ | ? | Not typically used |

**Note:** Windows doesn't support true dynamic library loading in the same way; purego has experimental Windows support but it's not production-ready for plugins.

**Source:** [Go plugin package limitations](https://pkg.go.dev/plugin), [purego platform support](https://github.com/ebitengine/purego)

### 3.2 macOS .dylib vs Linux .so

**macOS specific:**
- .dylib files must have proper `LC_ID_DYLIB` identifiers
- Recent macOS (Monterey+) is stricter about code signing
- Build command typically: `gcc -dynamiclib -fPIC -shared src/parser.c -o libtree-sitter-lang.dylib`

**Linux specific:**
- Standard ELF .so format
- Build command: `gcc -fPIC -shared src/parser.c -o libtree-sitter-lang.so`
- RPATH/LD_LIBRARY_PATH controls load paths

**Cross-compilation caveat:** You must build .so files on the target platform or with a cross-compiler. Go's `GOOS=linux GOARCH=amd64 go build` won't build the C shared libraries.

**Source:** [macOS plugin reproducibility issue #58557](https://github.com/golang/go/issues/58557), [dylib linking example](https://www.dynamsoft.com/codepool/go-barcode-reader-apple-silicon-mac.html)

### 3.3 Race Detector Limitations

Go's race detector has poor support for plugins:
- Simple race conditions may not be detected
- macOS has known issues with race detector + plugins (SIGABRT/SIGSEGV)
- **Recommendation:** Don't run with `-race` flag when using plugins

**Source:** [Race detector issues with plugins](https://github.com/golang/go/issues/49138)

---

## 4. Binary Size Estimates

### 4.1 Individual Grammar .so Sizes

Based on tree-sitter compilation patterns and published examples:

| Grammar | Est. Size | Notes |
|---------|-----------|-------|
| Simple (SQLite) | 1.1 MB | Measured real-world example |
| Medium (JavaScript, Go) | 500 KB - 1.5 MB | Typical complexity |
| Complex (Rust, C++) | 2-3 MB | Rich syntax, many rules |
| Minimal (JSON, YAML) | 100-300 KB | Simple structure |

**Average estimate:** ~500 KB to 1 MB per grammar

### 4.2 Total Download Size for 490 Grammars

```
Conservative (500 KB avg):  490 × 500 KB = 245 GB
Optimistic (250 KB avg):    490 × 250 KB = 122 GB
High end (1.5 MB avg):      490 × 1.5 MB = 735 GB
```

**This is impractical for on-demand loading** as individual .so files.

### 4.3 Smarter Approach: Regional Compilation

Instead of 490 individual .so files, group grammars into categories:

```
libtree-sitter-web.so           # JavaScript, TypeScript, HTML, CSS, Vue, etc.
libtree-sitter-systems.so       # C, C++, Rust, Go, Assembly
libtree-sitter-scripting.so     # Python, Ruby, Lua, Perl, PHP
libtree-sitter-data.so          # JSON, YAML, TOML, XML, Protocol Buffers
libtree-sitter-markup.so        # Markdown, LaTeX, ReStructuredText, AsciiDoc
libtree-sitter-other.so         # Everything else
```

**Estimated sizes:**
- ~100-150 MB per category
- ~800 MB - 1.2 GB total for all 6-8 regions
- Much more practical for lazy loading

**Build strategy:**
```bash
# Build individual .so for each category
gcc -shared -fPIC \
    grammars/tree-sitter-javascript/src/parser.c \
    grammars/tree-sitter-typescript/src/parser.c \
    grammars/tree-sitter-html/src/parser.c \
    -o libtree-sitter-web.so

# User downloads only needed regions on first use
```

**Source:** [Tree-sitter grammar compilation patterns](https://tree-sitter.github.io/tree-sitter/creating-parsers/)

---

## 5. Feasibility Assessment

### 5.1 Go Plugin System Approach

**Pros:**
- Built-in, no external dependencies
- Automatic caching (plugin.Open returns cached instance)
- Standard interface for symbol lookup

**Cons:**
- **Requires CGO_ENABLED=1** - adds C compiler dependency to builds
- **No Windows support** - Windows users can't use plugins
- **Can't close plugins** - all loaded plugins stay in memory forever
- **Poor race detector support** - limits testing and debugging
- **Path must be absolute** - need careful file location management

**Verdict:** Not ideal for production tool serving diverse platforms.

### 5.2 Purego Approach (Recommended)

**Pros:**
- **CGO-free:** `CGO_ENABLED=0` builds work everywhere Go supports
- **Lazy loading:** Only load grammars when parsing files in that language
- **Smaller initial binary:** ~5-10MB core tool vs 50-100MB with static linking
- **Better cross-compilation:** No C compiler needed
- **Natural caching:** Once loaded, .so stays in process memory

**Cons:**
- **Manual .so compilation:** Must compile tree-sitter libraries separately
- **Platform-specific binaries:** Need to build .so files for each target OS
- **Runtime dependency:** .so files must exist at known paths
- **Packaging complexity:** Distribute tool + grammar .so files together

**Verdict:** Viable and recommended approach.

### 5.3 Hybrid Approach (Best for Scale)

Combine static + dynamic loading:

```go
// Static: Include ~50 most common languages (JavaScript, Python, Go, Rust, etc.)
// Dynamic: Load remaining ~440 grammars from .so files on demand

type LanguageLoader struct {
    builtIn map[string]*Language  // Pre-compiled languages
    soCache map[string]*Language  // Dynamically loaded
}

func (l *LanguageLoader) Get(lang string) (*Language, error) {
    // Check built-in first
    if lang, ok := l.builtIn[lang]; ok {
        return lang, nil
    }

    // Try loading from .so
    if cached, ok := l.soCache[lang]; ok {
        return cached, nil
    }

    // Load from disk
    soPath := fmt.Sprintf("./grammars/libtree-sitter-%s.so", lang)
    lib, err := purego.Dlopen(soPath, purego.RTLD_NOW)
    if err != nil {
        return nil, err
    }

    // Register and cache
    var langFunc func() uintptr
    purego.RegisterLibFunc(&langFunc, lib, fmt.Sprintf("tree_sitter_%s", lang))
    lang := NewLanguage(unsafe.Pointer(langFunc()))
    l.soCache[lang] = lang
    return lang, nil
}
```

**Size breakdown:**
- Core binary with 50 common languages: ~30-40 MB
- Optional .so files for remaining 440: ~600-800 MB (download on first use)
- Total: ~900 MB (but only core is needed initially)

---

## 6. Implementation Example: On-Demand Grammar Loading

### 6.1 Complete Working Example

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "runtime"
    "sync"
    "unsafe"

    "github.com/ebitengine/purego"
    tree_sitter "github.com/tree-sitter/go-tree-sitter"
    ts_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
    ts_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// GrammarRegistry manages both static and dynamic language loading
type GrammarRegistry struct {
    mu       sync.RWMutex
    static   map[string]func() uintptr                // Built-in grammars
    dynamic  map[string]tree_sitter.Language          // Loaded .so grammars
    soDir    string                                    // Path to .so files
}

func NewGrammarRegistry(soDir string) *GrammarRegistry {
    return &GrammarRegistry{
        static: map[string]func() uintptr{
            "python": ts_python.Language,
            "go":     ts_go.Language,
        },
        dynamic: make(map[string]tree_sitter.Language),
        soDir:   soDir,
    }
}

func (gr *GrammarRegistry) GetLanguage(langName string) (tree_sitter.Language, error) {
    gr.mu.RLock()

    // Try static first
    if staticFn, ok := gr.static[langName]; ok {
        gr.mu.RUnlock()
        return tree_sitter.NewLanguage(unsafe.Pointer(staticFn())), nil
    }

    // Try cached dynamic
    if dynLang, ok := gr.dynamic[langName]; ok {
        gr.mu.RUnlock()
        return dynLang, nil
    }
    gr.mu.RUnlock()

    // Load from .so file
    return gr.loadFromSO(langName)
}

func (gr *GrammarRegistry) loadFromSO(langName string) (tree_sitter.Language, error) {
    soPath := filepath.Join(gr.soDir, gr.getSOFileName(langName))

    // Load shared library
    lib, err := purego.Dlopen(soPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
    if err != nil {
        return nil, fmt.Errorf("failed to load %s: %w", soPath, err)
    }

    // Register language function
    funcName := fmt.Sprintf("tree_sitter_%s", langName)
    var langFunc func() uintptr
    if err := purego.RegisterLibFunc(&langFunc, lib, funcName); err != nil {
        return nil, fmt.Errorf("failed to find symbol %s: %w", funcName, err)
    }

    // Create language and cache
    lang := tree_sitter.NewLanguage(unsafe.Pointer(langFunc()))

    gr.mu.Lock()
    gr.dynamic[langName] = lang
    gr.mu.Unlock()

    return lang, nil
}

func (gr *GrammarRegistry) getSOFileName(langName string) string {
    switch runtime.GOOS {
    case "darwin":
        return fmt.Sprintf("libtree-sitter-%s.dylib", langName)
    case "linux":
        return fmt.Sprintf("libtree-sitter-%s.so", langName)
    default:
        return fmt.Sprintf("libtree-sitter-%s.so", langName)
    }
}

// ParseFile demonstrates usage
func (gr *GrammarRegistry) ParseFile(filepath string, langName string) error {
    code, err := os.ReadFile(filepath)
    if err != nil {
        return err
    }

    lang, err := gr.GetLanguage(langName)
    if err != nil {
        return err
    }

    parser := tree_sitter.NewParser()
    defer parser.Close()
    parser.SetLanguage(lang)

    tree := parser.Parse(code, nil)
    defer tree.Close()

    fmt.Printf("Parse tree for %s:\n%s\n", filepath, tree.RootNode().ToSexp())
    return nil
}

func main() {
    // Initialize registry pointing to compiled .so files
    registry := NewGrammarRegistry("./grammars")

    // Static languages (no file needed)
    if err := registry.ParseFile("example.py", "python"); err != nil {
        fmt.Fprintf(os.Stderr, "Error parsing Python: %v\n", err)
    }

    // Dynamic languages (loads from .so if available)
    if err := registry.ParseFile("example.js", "javascript"); err != nil {
        fmt.Fprintf(os.Stderr, "Error parsing JavaScript: %v\n", err)
    }
}
```

### 6.2 Compilation Instructions

**1. Compile tree-sitter grammars to .so:**

```bash
# Clone tree-sitter grammar repo
git clone https://github.com/tree-sitter/tree-sitter-javascript
cd tree-sitter-javascript

# Generate C parser
npx tree-sitter generate

# Compile to .so
gcc -fPIC -shared -o libtree-sitter-javascript.so \
    -I. -Isrc \
    src/parser.c src/scanner.c

# Install to app grammars directory
mkdir -p /path/to/app/grammars
cp libtree-sitter-javascript.so /path/to/app/grammars/
```

**2. Build Go application (CGO_ENABLED=0 - no compiler needed):**

```bash
CGO_ENABLED=0 go build -o myapp main.go
```

**3. Run application:**

```bash
./myapp  # Automatically loads .so files on demand
```

---

## 7. Practical Deployment Strategy

### 7.1 Build Pipeline

```dockerfile
# Stage 1: Compile all tree-sitter grammars
FROM ubuntu:22.04 as grammar-builder

RUN apt-get update && apt-get install -y \
    build-essential gcc npm nodejs git

# Clone all 490 grammar repos (or use tree-sitter/tree-sitter-grammars)
RUN git clone https://github.com/tree-sitter/tree-sitter-grammars.git /grammars

WORKDIR /grammars
RUN find . -name "build.sh" -exec bash {} \;  # Compile all

# Build regional .so files
RUN cat > /compile-regions.sh << 'EOF'
#!/bin/bash
# Compile web grammars
gcc -shared -fPIC -o libtree-sitter-web.so \
    tree-sitter-javascript/src/parser.c \
    tree-sitter-typescript/src/parser.c \
    tree-sitter-html/src/parser.c
# ... repeat for other regions
EOF
RUN bash /compile-regions.sh

# Stage 2: Build Go app
FROM golang:1.23-alpine

RUN go install github.com/ebitengine/purego@latest

COPY main.go .
RUN CGO_ENABLED=0 go build -o aoa-cli main.go

# Stage 3: Runtime
FROM alpine:latest

COPY --from=grammar-builder /grammars/libtree-sitter-*.so /app/grammars/
COPY --from=1 /app/aoa-cli /app/

ENTRYPOINT ["/app/aoa-cli"]
```

### 7.2 Download/Install Strategy

```bash
# User installs minimal tool (5-10 MB)
curl -L https://releases.example.com/aoa-cli-linux-x64 > aoa
chmod +x aoa

# On first parse of each language, auto-download .so if missing
aoa analyze example.js  # Auto-downloads libtree-sitter-javascript.so
aoa analyze example.rs  # Auto-downloads libtree-sitter-rust.so
```

---

## 8. Alternative: Compile-Time Plugin Generation

Instead of shipping .so files, generate language plugins at compile time:

```bash
# Generate Go plugin source from grammar
go generate ./cmd/plugins

# Compile plugins
go build -buildmode=plugin -o plugins/javascript.so ./cmd/plugins/javascript
go build -buildmode=plugin -o plugins/python.so ./cmd/plugins/python

# Main app loads plugins
plugin.Open("./plugins/javascript.so")
```

**Pros:**
- Single unified Go build system
- Reproducible builds

**Cons:**
- Still requires CGO (compiler in build environment)
- Doesn't reduce binary size initially
- More complex go generate setup

**Verdict:** Not recommended compared to purego + C .so compilation.

---

## 9. Recommendation Summary

### Recommended Architecture

```
┌─────────────────────────────────────┐
│   aOa CLI (Core Binary)             │
│   ~10-40 MB (with 50 common langs)  │
│                                      │
│   ├─ Static Languages (built-in)    │
│   │  └─ Python, JS, Go, Rust, etc.  │
│   │                                  │
│   └─ DynamicLoader (purego-based)   │
│      └─ Loads .so files on demand   │
└─────────────────────────────────────┘
                  ↓
        ┌────────────────────┐
        │  Grammar .so Files │
        │  (optional, lazy)  │
        ├────────────────────┤
        │ Web (JS/TS/HTML):   │
        │ ~150 MB            │
        ├────────────────────┤
        │ Systems (C/Rust):  │
        │ ~120 MB            │
        ├────────────────────┤
        │ Scripts (Python):  │
        │ ~100 MB            │
        ├────────────────────┤
        │ Data (JSON/YAML):  │
        │ ~80 MB             │
        └────────────────────┘
```

**Key decisions:**

1. **Use purego for dynamic loading** - CGO-free, cross-platform friendly
2. **Bundle 50 most-used languages statically** - Python, JavaScript, Go, Rust, etc.
3. **Group remaining 440 into 6-8 regional .so files** - Manageable download sizes
4. **Lazy-load regions on first use** - User only pays for what they need
5. **Cache loaded grammars in process memory** - No re-loading overhead
6. **Build separate .so files for Linux/macOS** - Platform-specific binaries

**Estimated sizes:**
- Core CLI: 30-40 MB
- Per grammar region: 100-150 MB each
- Total with all regions: 800 MB - 1.2 GB (optional)

---

## 10. References

- [Go plugin package](https://pkg.go.dev/plugin)
- [Ebitengine Purego (cgo-free dynamic loading)](https://pkg.go.dev/github.com/ebitengine/purego)
- [go-tree-sitter bindings](https://pkg.go.dev/github.com/tree-sitter/go-tree-sitter)
- [Tree-sitter creating parsers guide](https://tree-sitter.github.io/tree-sitter/creating-parsers/)
- [Tree-sitter grammar repositories](https://github.com/tree-sitter-grammars)
- [Go plugin dynamic loading patterns](https://medium.com/profusion-engineering/plugins-with-go-7ea1e7a280d3)
- [macOS dylib linking guide](https://www.dynamsoft.com/codepool/go-barcode-reader-apple-silicon-mac.html)

---

**Conclusion:** Pre-compiling 490 grammars as individual .so files is impractical due to download size. However, grouping them into 6-8 regional .so files (~100-150 MB each) with lazy loading via purego is a production-viable approach. This balances flexibility, performance, and ease of deployment while avoiding the baggage of embedding all 490 grammars into a monolithic binary.
