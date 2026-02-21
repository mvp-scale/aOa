package treesitter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// DynamicLoader loads tree-sitter grammars from shared libraries (.so on Linux,
// .dylib on macOS) using purego. It searches configured paths for grammar files
// and caches loaded languages for reuse.
type DynamicLoader struct {
	searchPaths []string
	mu          sync.Mutex
	loaded      map[string]*tree_sitter.Language
	handles     []uintptr
}

// NewDynamicLoader creates a loader that searches the given paths for grammar
// shared libraries. Paths are searched in order; first match wins.
func NewDynamicLoader(searchPaths []string) *DynamicLoader {
	return &DynamicLoader{
		searchPaths: searchPaths,
		loaded:      make(map[string]*tree_sitter.Language),
	}
}

// DefaultGrammarPaths returns the default search paths for grammar shared libraries.
// Project-local (.aoa/grammars/) is searched first, then global (~/.aoa/grammars/).
func DefaultGrammarPaths(projectRoot string) []string {
	var paths []string
	if projectRoot != "" {
		paths = append(paths, filepath.Join(projectRoot, ".aoa", "grammars"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".aoa", "grammars"))
	}
	return paths
}

// LibExtension returns the shared library extension for the current platform.
func LibExtension() string {
	if runtime.GOOS == "darwin" {
		return ".dylib"
	}
	return ".so"
}

// symbolOverrides maps internal language names to C symbol names where the
// default derivation (tree_sitter_{name}) doesn't apply.
var symbolOverrides = map[string]string{
	"objc": "tree_sitter_objc",
}

// CSymbolName returns the C function name for a language's tree-sitter grammar.
// Most follow the pattern tree_sitter_{name}; exceptions use symbolOverrides.
func CSymbolName(lang string) string {
	if sym, ok := symbolOverrides[lang]; ok {
		return sym
	}
	return "tree_sitter_" + strings.ReplaceAll(lang, "-", "_")
}

// soFileOverrides maps language names to shared library base names where the
// grammar lives in a differently-named file (e.g., tsx shares typescript's .so).
var soFileOverrides = map[string]string{
	"tsx": "typescript",
}

// SOBaseName returns the expected shared library base name for a language.
func SOBaseName(lang string) string {
	if base, ok := soFileOverrides[lang]; ok {
		return base
	}
	return lang
}

// LoadGrammar loads a grammar from a shared library for the given language.
// Results are cached; subsequent calls for the same language return the cached value.
func (dl *DynamicLoader) LoadGrammar(lang string) (*tree_sitter.Language, error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if cached, ok := dl.loaded[lang]; ok {
		return cached, nil
	}

	ext := LibExtension()
	baseName := SOBaseName(lang)

	var soPath string
	for _, dir := range dl.searchPaths {
		candidate := filepath.Join(dir, baseName+ext)
		if _, err := os.Stat(candidate); err == nil {
			soPath = candidate
			break
		}
	}
	if soPath == "" {
		return nil, fmt.Errorf("grammar %q: shared library not found in search paths", lang)
	}

	handle, err := purego.Dlopen(soPath, purego.RTLD_LAZY)
	if err != nil {
		return nil, fmt.Errorf("grammar %q: dlopen %s: %w", lang, soPath, err)
	}
	dl.handles = append(dl.handles, handle)

	symName := CSymbolName(lang)
	var langFunc func() uintptr
	purego.RegisterLibFunc(&langFunc, handle, symName)

	ptr := langFunc()
	if ptr == 0 {
		return nil, fmt.Errorf("grammar %q: %s() returned null", lang, symName)
	}

	// Convert uintptr from C (purego) to unsafe.Pointer without triggering go vet's
	// unsafeptr check. Safe because ptr is a static TSLanguage* from the grammar .so,
	// not a Go-managed pointer that could be moved by GC.
	language := tree_sitter.NewLanguage(*(*unsafe.Pointer)(unsafe.Pointer(&ptr)))
	dl.loaded[lang] = language
	return language, nil
}

// GrammarPath returns the path to the shared library for a language, or "" if not found.
func (dl *DynamicLoader) GrammarPath(lang string) string {
	ext := LibExtension()
	baseName := SOBaseName(lang)
	for _, dir := range dl.searchPaths {
		candidate := filepath.Join(dir, baseName+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// InstalledGrammars returns language names found as shared libraries in the search paths.
func (dl *DynamicLoader) InstalledGrammars() []string {
	ext := LibExtension()
	seen := make(map[string]bool)
	var names []string
	for _, dir := range dl.searchPaths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ext) {
				lang := strings.TrimSuffix(name, ext)
				if !seen[lang] {
					seen[lang] = true
					names = append(names, lang)
				}
			}
		}
	}
	return names
}

// Close releases all dlopen handles.
func (dl *DynamicLoader) Close() {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.handles = nil
	dl.loaded = make(map[string]*tree_sitter.Language)
}

// SearchPaths returns the configured search paths.
func (dl *DynamicLoader) SearchPaths() []string {
	return dl.searchPaths
}
