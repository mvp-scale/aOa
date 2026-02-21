// aoa-recon is the companion binary for aOa that provides tree-sitter parsing
// and security scanning. It enhances the aoa search index with symbol metadata
// and produces recon scan findings.
//
// Subcommands:
//   - enhance: full project scan — parse all files, write symbols + findings to bbolt
//   - enhance-file: single file incremental update
//   - version: print version
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/recon"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/analyzer"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/corey/aoa/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aoa-recon",
	Short: "aOa Recon — tree-sitter parsing and security scanning",
	Long:  "Companion binary for aOa: enhances the search index with symbols and scans code for issues.",
}

var enhanceDB string
var enhanceRoot string

var enhanceCmd = &cobra.Command{
	Use:   "enhance",
	Short: "Full project scan: parse files, write symbols to index",
	Long:  "Walks the project, parses all source files with tree-sitter, and writes symbol metadata to the bbolt index.",
	RunE:  runEnhance,
}

var enhanceFileCmd = &cobra.Command{
	Use:   "enhance-file",
	Short: "Incremental single-file update",
	Long:  "Parses one file and updates its symbols in the bbolt index.",
	RunE:  runEnhanceFile,
}

var enhanceFilePath string

func init() {
	enhanceCmd.Flags().StringVar(&enhanceDB, "db", "", "path to bbolt database (required)")
	enhanceCmd.Flags().StringVar(&enhanceRoot, "root", "", "project root directory (required)")
	enhanceCmd.MarkFlagRequired("db")
	enhanceCmd.MarkFlagRequired("root")

	enhanceFileCmd.Flags().StringVar(&enhanceDB, "db", "", "path to bbolt database (required)")
	enhanceFileCmd.Flags().StringVar(&enhanceFilePath, "file", "", "file to parse (required)")
	enhanceFileCmd.MarkFlagRequired("db")
	enhanceFileCmd.MarkFlagRequired("file")

	rootCmd.Version = version.String()
	rootCmd.AddCommand(enhanceCmd)
	rootCmd.AddCommand(enhanceFileCmd)
}

// skipDirs mirrors internal/app/indexer.go skipDirs.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, ".venv": true,
	"__pycache__": true, "vendor": true, ".idea": true,
	".vscode": true, "dist": true, "build": true,
	".aoa": true, ".next": true, "target": true, ".claude": true,
}

func runEnhance(cmd *cobra.Command, args []string) error {
	absRoot, err := filepath.Abs(enhanceRoot)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	projectID := filepath.Base(absRoot)

	store, err := bbolt.NewStore(enhanceDB)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	// Load existing index or create fresh
	idx, err := store.LoadIndex(projectID)
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}
	if idx == nil {
		idx = &ports.Index{
			Tokens:   make(map[string][]ports.TokenRef),
			Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
			Files:    make(map[uint32]*ports.FileMeta),
		}
	}

	parser := treesitter.NewParser()

	// Walk project and parse files
	var files []string
	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
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
		return fmt.Errorf("walk project: %w", err)
	}
	sort.Strings(files)

	// Build a reverse map of relPath → fileID from existing index
	pathToID := make(map[string]uint32)
	var maxID uint32
	for id, fm := range idx.Files {
		pathToID[fm.Path] = id
		if id > maxID {
			maxID = id
		}
	}

	var enhanced, skipped int

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || info.Size() > 1<<20 {
			skipped++
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			skipped++
			continue
		}

		relPath, _ := filepath.Rel(absRoot, path)

		metas, err := parser.ParseFileToMeta(path, source)
		if err != nil || len(metas) == 0 {
			skipped++
			continue
		}

		// Find or assign fileID
		fileID, exists := pathToID[relPath]
		if !exists {
			maxID++
			fileID = maxID
			ext := strings.TrimPrefix(filepath.Ext(path), ".")
			idx.Files[fileID] = &ports.FileMeta{
				Path:         relPath,
				LastModified: info.ModTime().Unix(),
				Language:     ext,
				Size:         info.Size(),
			}
			pathToID[relPath] = fileID
		}

		// Clear old metadata for this file
		for ref := range idx.Metadata {
			if ref.FileID == fileID {
				delete(idx.Metadata, ref)
			}
		}

		// Write new symbols
		for _, meta := range metas {
			ref := ports.TokenRef{FileID: fileID, Line: meta.StartLine}
			idx.Metadata[ref] = meta

			tokens := index.Tokenize(meta.Name)
			for _, tok := range tokens {
				idx.Tokens[tok] = append(idx.Tokens[tok], ref)
			}
			lower := strings.ToLower(meta.Name)
			if lower != "" {
				idx.Tokens[lower] = append(idx.Tokens[lower], ref)
			}
		}
		enhanced++
	}

	// Save enhanced index
	if err := store.SaveIndex(projectID, idx); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	// Dimensional analysis: run AC + AST + bitmask compose on all files
	rules := analyzer.AllRules()
	engine := recon.NewEngine(rules, parser)
	analyses := map[string]*analyzer.FileAnalysis{}
	dimFindings := 0

	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		relPath, _ := filepath.Rel(absRoot, path)
		base := filepath.Base(relPath)
		isTest := isTestFile(base)
		isMain := isMainFile(relPath, base)
		analysis := engine.AnalyzeFile(path, source, isTest, isMain)
		if analysis != nil {
			analysis.Path = relPath // store relative path
			analyses[relPath] = analysis
			dimFindings += len(analysis.Findings)
		}
	}

	if len(analyses) > 0 {
		if err := store.SaveAllDimensions(projectID, analyses); err != nil {
			return fmt.Errorf("save dimensions: %w", err)
		}
	}

	fmt.Printf("aoa-recon enhanced %d files (%d skipped), %d total files in index, %d dimensional findings\n",
		enhanced, skipped, len(idx.Files), dimFindings)
	return nil
}

// isTestFile returns true if the filename looks like a test file.
func isTestFile(base string) bool {
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, "_test.rs") ||
		strings.Contains(base, "test_")
}

// isMainFile returns true if the file is in a cmd/ directory or is named main.go/main.py.
func isMainFile(relPath, base string) bool {
	return strings.Contains(relPath, "cmd/") ||
		base == "main.go" || base == "main.py"
}

func runEnhanceFile(cmd *cobra.Command, args []string) error {
	absFile, err := filepath.Abs(enhanceFilePath)
	if err != nil {
		return fmt.Errorf("resolve file: %w", err)
	}

	store, err := bbolt.NewStore(enhanceDB)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	// We need the project root to compute relative paths and projectID.
	// Derive from the DB path: .aoa/aoa.db → project root is two levels up.
	dbDir := filepath.Dir(enhanceDB)
	projectRoot := filepath.Dir(dbDir)
	projectID := filepath.Base(projectRoot)

	idx, err := store.LoadIndex(projectID)
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}
	if idx == nil {
		return fmt.Errorf("no index found for project %s", projectID)
	}

	parser := treesitter.NewParser()
	ext := strings.ToLower(filepath.Ext(absFile))
	if !parser.SupportsExtension(ext) {
		fmt.Printf("aoa-recon: unsupported extension %s\n", ext)
		return nil
	}

	source, err := os.ReadFile(absFile)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	relPath, _ := filepath.Rel(projectRoot, absFile)

	// Find fileID
	var fileID uint32
	for id, fm := range idx.Files {
		if fm.Path == relPath {
			fileID = id
			break
		}
	}
	if fileID == 0 {
		fmt.Printf("aoa-recon: file %s not in index\n", relPath)
		return nil
	}

	metas, err := parser.ParseFileToMeta(absFile, source)
	if err != nil {
		return fmt.Errorf("parse file: %w", err)
	}

	// Clear old metadata
	for ref := range idx.Metadata {
		if ref.FileID == fileID {
			delete(idx.Metadata, ref)
		}
	}

	// Write new symbols
	for _, meta := range metas {
		ref := ports.TokenRef{FileID: fileID, Line: meta.StartLine}
		idx.Metadata[ref] = meta

		tokens := index.Tokenize(meta.Name)
		for _, tok := range tokens {
			idx.Tokens[tok] = append(idx.Tokens[tok], ref)
		}
		lower := strings.ToLower(meta.Name)
		if lower != "" {
			idx.Tokens[lower] = append(idx.Tokens[lower], ref)
		}
	}

	if err := store.SaveIndex(projectID, idx); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	fmt.Printf("aoa-recon enhanced %s: %d symbols\n", relPath, len(metas))
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
