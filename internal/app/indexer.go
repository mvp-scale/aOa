package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
)

// IndexResult holds statistics from a BuildIndex operation.
type IndexResult struct {
	FileCount   int
	SymbolCount int
	TokenCount  int
	EdgeCount   int // import edges extracted (non-zero only when archEnabled=true)
}

// skipDirs lists directories to skip during indexing (matches fsnotify watcher).
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true,
	"__pycache__":  true,
	"vendor":       true,
	".idea":        true,
	".vscode":      true,
	"dist":         true,
	"build":        true,
	".aoa":         true,
	".next":        true,
	"target":       true,
	".claude":      true,
}

// defaultCodeExtensions is the set of file extensions indexed when no parser is
// available (tokenization-only mode). Mirrors the core set from treesitter/extensions.go
// so that file discovery works identically with or without CGo.
var defaultCodeExtensions = map[string]bool{
	// Core languages
	".go": true, ".py": true, ".pyw": true,
	".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".ts": true, ".mts": true, ".tsx": true,
	".rs": true, ".java": true,
	".c": true, ".h": true, ".cpp": true, ".hpp": true, ".cc": true, ".cxx": true, ".hxx": true,
	".cs": true, ".rb": true, ".php": true, ".swift": true,
	".kt": true, ".kts": true, ".scala": true, ".sc": true,
	// Scripting
	".sh": true, ".bash": true, ".zsh": true, ".lua": true,
	".pl": true, ".pm": true, ".r": true, ".R": true, ".jl": true,
	".ex": true, ".exs": true, ".erl": true, ".hrl": true,
	// Functional
	".hs": true, ".lhs": true, ".ml": true, ".mli": true,
	".gleam": true, ".elm": true,
	".clj": true, ".cljs": true, ".cljc": true,
	".purs": true, ".fnl": true,
	// Systems & Emerging
	".zig": true, ".d": true, ".cu": true, ".cuh": true,
	".odin": true, ".v": true, ".nim": true,
	".m": true, ".mm": true,
	".ada": true, ".adb": true, ".ads": true,
	".f90": true, ".f95": true, ".f03": true, ".f": true,
	".sv": true, ".vhd": true, ".vhdl": true,
	// Web & Frontend
	".html": true, ".htm": true, ".css": true, ".scss": true, ".less": true,
	".vue": true, ".svelte": true, ".dart": true,
	// Data & Config
	".json": true, ".jsonc": true, ".yaml": true, ".yml": true, ".toml": true,
	".sql": true, ".md": true, ".mdx": true,
	".graphql": true, ".gql": true,
	".tf": true, ".hcl": true, ".nix": true,
	// Build
	".cmake": true, ".mk": true, ".groovy": true, ".gradle": true,
	".glsl": true, ".vert": true, ".frag": true, ".hlsl": true,
}

// BuildIndex walks a project root, parses source files (when parser is non-nil),
// and builds a fresh search index. When parser is nil, it operates in
// tokenization-only mode: discovers files by extension, tokenizes content,
// but produces no symbol metadata.
//
// This is the backward-compatible entry point. For import-edge extraction,
// use BuildIndexWithFacts (called by App when ArchEnabled=true).
func BuildIndex(root string, parser ports.Parser) (*ports.Index, *IndexResult, error) {
	idx, result, _, err := buildIndexCore(root, parser, false)
	return idx, result, err
}

// BuildIndexWithFacts is identical to BuildIndex but additionally extracts
// import edges from P1 languages (Go/Python/JS/TS) when archEnabled=true and
// the parser implements ports.FactParser.
//
// When archEnabled=false, the returned edge slice is always nil (C4 kill switch).
// Used by App.WarmCaches and App.Reindex instead of BuildIndex.
func BuildIndexWithFacts(root string, parser ports.Parser, archEnabled bool) (*ports.Index, *IndexResult, []ports.ImportEdge, error) {
	return buildIndexCore(root, parser, archEnabled)
}

// buildIndexCore is the shared implementation for BuildIndex and BuildIndexWithFacts.
func buildIndexCore(root string, parser ports.Parser, archEnabled bool) (*ports.Index, *IndexResult, []ports.ImportEdge, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, nil, err
	}

	// Prefer git ls-files to respect .gitignore. Falls back to filepath.Walk
	// if not in a git repo or git is unavailable.
	files, err := gitTrackedFiles(absRoot, parser)
	if err != nil {
		// Fallback: walk with hardcoded skipDirs (non-git projects)
		files, err = walkFiles(absRoot, parser)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	sort.Strings(files)

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	// Type-assert once; fp is nil when parser does not implement FactParser
	// or when arch extraction is disabled (C4). Zero cost on hot path.
	var fp ports.FactParser
	if archEnabled && parser != nil {
		fp, _ = parser.(ports.FactParser)
	}

	var totalSymbols int
	var allEdges []ports.ImportEdge
	var fileID uint32

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		// Skip files > 1MB
		if info.Size() > 1<<20 {
			continue
		}

		source, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		fileID++
		relPath, _ := filepath.Rel(absRoot, path)

		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		idx.Files[fileID] = &ports.FileMeta{
			Path:         relPath,
			LastModified: info.ModTime().Unix(),
			Language:     ext,
			Size:         info.Size(),
		}

		// When FactParser is available (C4 on + parser implements FactParser),
		// extract symbols AND edges in a single parse pass (G0: one traversal).
		if fp != nil {
			metas, edges, parseErr := fp.ParseFileToMetaAndFacts(path, source)
			if parseErr == nil {
				for _, meta := range metas {
					ref := ports.TokenRef{FileID: fileID, Line: meta.StartLine}
					idx.Metadata[ref] = meta
					totalSymbols++

					tokens := index.Tokenize(meta.Name)
					for _, tok := range tokens {
						idx.Tokens[tok] = append(idx.Tokens[tok], ref)
					}

					lower := strings.ToLower(meta.Name)
					if lower != "" {
						idx.Tokens[lower] = append(idx.Tokens[lower], ref)
					}
				}
				// Emit edges with relative FromFile path (G7: provenance stamps).
				// Decoupled from the metas gate: var/const-only files produce 0 metas
				// but may still have import edges (e.g. Go files with only declarations).
				for _, e := range edges {
					e.FromFile = relPath
					allEdges = append(allEdges, e)
				}
				continue
			}
		}

		// When parser is available (but not FactParser or edges not needed),
		// extract symbols only.
		if parser != nil {
			metas, parseErr := parser.ParseFileToMeta(path, source)
			if parseErr == nil && len(metas) > 0 {
				for _, meta := range metas {
					ref := ports.TokenRef{FileID: fileID, Line: meta.StartLine}
					idx.Metadata[ref] = meta
					totalSymbols++

					tokens := index.Tokenize(meta.Name)
					for _, tok := range tokens {
						idx.Tokens[tok] = append(idx.Tokens[tok], ref)
					}

					lower := strings.ToLower(meta.Name)
					if lower != "" {
						idx.Tokens[lower] = append(idx.Tokens[lower], ref)
					}
				}
				continue
			}
		}

		// Tokenization-only fallback: tokenize file content line-by-line for file-level search.
		lines := strings.Split(string(source), "\n")
		for _, line := range lines {
			tokens := index.TokenizeContentLine(line)
			for _, tok := range tokens {
				ref := ports.TokenRef{FileID: fileID, Line: 0}
				idx.Tokens[tok] = append(idx.Tokens[tok], ref)
			}
		}
	}

	result := &IndexResult{
		FileCount:   len(idx.Files),
		SymbolCount: totalSymbols,
		TokenCount:  len(idx.Tokens),
		EdgeCount:   len(allEdges),
	}

	return idx, result, allEdges, nil
}

// gitTrackedFiles uses "git ls-files" to enumerate files that are tracked
// (or untracked but not ignored), respecting .gitignore, .git/info/exclude,
// and nested gitignore files. Returns absolute paths filtered by parser support.
func gitTrackedFiles(absRoot string, parser ports.Parser) ([]string, error) {
	// --cached: tracked files. --others --exclude-standard: untracked but not ignored.
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = absRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	for _, rel := range strings.Split(string(out), "\x00") {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == "" {
			continue
		}
		if parser != nil {
			if !parser.SupportsExtension(ext) {
				continue
			}
		} else {
			if !defaultCodeExtensions[ext] {
				continue
			}
		}
		files = append(files, filepath.Join(absRoot, rel))
	}
	sort.Strings(files)
	return files, nil
}

// GitIgnoredDirs returns absolute paths of directories ignored by .gitignore.
// Uses "git ls-files --others --ignored --exclude-standard --directory" which
// respects all .gitignore files (root, nested, .git/info/exclude).
// Returns nil if not in a git repo or git is unavailable.
func GitIgnoredDirs(absRoot string) []string {
	cmd := exec.Command("git", "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
	cmd.Dir = absRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var dirs []string
	for _, rel := range strings.Split(string(out), "\x00") {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		// Only collect directories (trailing /)
		if strings.HasSuffix(rel, "/") {
			dirs = append(dirs, filepath.Join(absRoot, rel))
		}
	}
	return dirs
}

// walkFiles is the fallback file discovery for non-git projects.
// Uses the hardcoded skipDirs list.
func walkFiles(absRoot string, parser ports.Parser) ([]string, error) {
	var files []string
	err := filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			return nil
		}
		if parser != nil {
			if parser.SupportsExtension(ext) {
				files = append(files, path)
			}
		} else {
			if defaultCodeExtensions[ext] {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}
