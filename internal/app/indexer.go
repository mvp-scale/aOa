package app

import (
	"os"
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
func BuildIndex(root string, parser ports.Parser) (*ports.Index, *IndexResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}

	var files []string
	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
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
		// When parser is available, use its extension support; otherwise fall back
		// to the built-in defaultCodeExtensions list for tokenization-only mode.
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
	if err != nil {
		return nil, nil, err
	}

	sort.Strings(files)

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	var totalSymbols int
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

		// When parser is available, extract symbols; otherwise tokenize file content.
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
	}

	return idx, result, nil
}
