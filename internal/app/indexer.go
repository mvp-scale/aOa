package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/adapters/treesitter"
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

// BuildIndex walks a project root, parses source files with tree-sitter,
// and builds a fresh search index. This is the shared logic used by both
// `aoa init` (no daemon) and daemon-side reindex.
func BuildIndex(root string, parser *treesitter.Parser) (*ports.Index, *IndexResult, error) {
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
		if ext != "" && parser.SupportsExtension(ext) {
			files = append(files, path)
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

		metas, err := parser.ParseFileToMeta(path, source)
		if err != nil || len(metas) == 0 {
			continue
		}

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
	}

	result := &IndexResult{
		FileCount:   len(idx.Files),
		SymbolCount: totalSymbols,
		TokenCount:  len(idx.Tokens),
	}

	return idx, result, nil
}
