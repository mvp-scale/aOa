package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/spf13/cobra"
)

// Directories to skip during indexing (matches fsnotify watcher).
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

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Index the current project",
	Long:  "Scans all code files, extracts symbols with tree-sitter, and builds the search index.",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	dbPath := filepath.Join(root, ".aoa", "aoa.db")
	projectID := filepath.Base(root)

	// Ensure .aoa directory exists
	if err := os.MkdirAll(filepath.Join(root, ".aoa"), 0755); err != nil {
		return fmt.Errorf("create .aoa dir: %w", err)
	}

	store, err := bbolt.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	parser := treesitter.NewParser()

	// Walk the project and collect code files
	var files []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		return fmt.Errorf("walk: %w", err)
	}

	sort.Strings(files)

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	var totalSymbols int
	var fileID uint32

	fmt.Printf("⚡ Scanning %d files...\n", len(files))

	for i, path := range files {
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
		relPath, _ := filepath.Rel(root, path)

		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		idx.Files[fileID] = &ports.FileMeta{
			Path:         relPath,
			LastModified: info.ModTime().Unix(),
			Language:     ext,
		}

		metas, err := parser.ParseFileToMeta(path, source)
		if err != nil || len(metas) == 0 {
			continue
		}

		if (i+1)%50 == 0 {
			fmt.Printf("  %d/%d files...\n", i+1, len(files))
		}

		for _, meta := range metas {
			ref := ports.TokenRef{FileID: fileID, Line: meta.StartLine}
			idx.Metadata[ref] = meta
			totalSymbols++

			// Tokenize symbol name and add to inverted index
			tokens := index.Tokenize(meta.Name)
			for _, tok := range tokens {
				idx.Tokens[tok] = append(idx.Tokens[tok], ref)
			}

			// Also index the full name as a token
			lower := strings.ToLower(meta.Name)
			if lower != "" {
				idx.Tokens[lower] = append(idx.Tokens[lower], ref)
			}
		}
	}

	if err := store.SaveIndex(projectID, idx); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	fmt.Printf("⚡ aOa indexed %d files, %d symbols, %d tokens\n", len(files), totalSymbols, len(idx.Tokens))
	return nil
}
