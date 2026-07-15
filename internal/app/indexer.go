package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	idx, result, _, err := buildIndexCore(root, parser, false, nil)
	return idx, result, err
}

// BuildIndexWithFacts is identical to BuildIndex but additionally extracts
// import edges from P1 languages (Go/Python/JS/TS) when archEnabled=true and
// the parser implements ports.FactParser.
//
// When archEnabled=false, the returned edge slice is always nil (C4 kill switch).
// Used by App.WarmCaches and App.Reindex instead of BuildIndex.
func BuildIndexWithFacts(root string, parser ports.Parser, archEnabled bool) (*ports.Index, *IndexResult, []ports.ImportEdge, error) {
	return buildIndexCore(root, parser, archEnabled, nil)
}

// BuildIndexWithFactsAndSink is BuildIndexWithFacts plus dual-run Fact
// emission (FDN-2, board #28): every import edge additionally becomes a raw
// ports.Fact emitted through sink, ALONGSIDE (never instead of) the legacy
// []ports.ImportEdge return value. sink may be nil, in which case this is
// byte-identical to BuildIndexWithFacts (no caller is forced to adopt a
// sink dependency). Existing BuildIndexWithFacts callers are untouched —
// consumers switch to the Fact-based path in FDN-4 (D25).
//
// Facts emitted here are RAW (Object empty, Attrs["spec"] = the literal,
// unresolved import specifier) per 01-facts-substrate.md §1.2 "Rules": the
// parser only sees one file at a time, so resolving the import target to a
// unit ID or "ext:" is the compactor's job (§2.4, FDN-3), not this pass's.
// Subject IS computable here — it is pure path math on the importING file,
// no cross-file knowledge required — so raw facts still carry a real,
// derived Subject rather than a placeholder.
func BuildIndexWithFactsAndSink(root string, parser ports.Parser, archEnabled bool, sink ports.FactSink) (*ports.Index, *IndexResult, []ports.ImportEdge, error) {
	return buildIndexCore(root, parser, archEnabled, sink)
}

// buildIndexCore is the shared implementation for BuildIndex, BuildIndexWithFacts,
// and BuildIndexWithFactsAndSink. sink is nil-safe: nil skips Fact emission
// entirely (zero-cost dual-run opt-out).
func buildIndexCore(root string, parser ports.Parser, archEnabled bool, sink ports.FactSink) (*ports.Index, *IndexResult, []ports.ImportEdge, error) {
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
		// parsedByFacts is set when ParseFileToMetaAndFacts succeeds, preventing
		// a redundant ParseFileToMeta call for symbol-less files.
		var parsedByFacts bool
		if fp != nil {
			metas, edges, parseErr := fp.ParseFileToMetaAndFacts(path, source)
			if parseErr == nil {
				parsedByFacts = true
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
				var factSubject string
				if sink != nil && len(edges) > 0 {
					factSubject = factSubjectForFile(relPath, ext)
				}
				for _, e := range edges {
					e.FromFile = relPath
					allEdges = append(allEdges, e)
					if sink != nil {
						sink.Emit(importEdgeToFact(e, factSubject))
					}
				}
				if len(metas) > 0 {
					continue // symbols found; skip content tokenization fallback
				}
				// len(metas)==0: fall through to content tokenization (C4 invariant T30:
				// flag-on must produce identical index content to flag-off for symbol-less
				// parseable files — only edge emission differs between the two modes).
			}
		}

		// When parser is available (but not FactParser or edges not needed),
		// extract symbols only. Skipped when parsedByFacts=true to avoid re-parsing.
		if !parsedByFacts && parser != nil {
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

// factSubjectForFile computes the canonical unit ID (D7, §1.3 <ns>:<path>)
// for the file that OWNS an import edge. This is pure path math on the
// importing file's own relative path + detected extension — no file-set or
// manifest lookup required, unlike resolving the import TARGET (§2.4, the
// compactor's job in FDN-3/4). Namespaces mirror the doc's worked examples:
// go (package dir), py (module file sans extension), ts (JS/TS/TSX module
// file sans extension, shared family). Falls back to "file:" for any
// language reaching here that isn't one of the three P1 import extractors
// (should not currently happen — importExtractors only registers go/python/
// javascript/typescript/tsx).
func factSubjectForFile(relPath, ext string) string {
	relPath = filepath.ToSlash(relPath)
	switch strings.ToLower(ext) {
	case "go":
		dir := filepath.ToSlash(filepath.Dir(relPath))
		if dir == "." {
			dir = ""
		}
		return "go:" + dir
	case "py", "pyw":
		return "py:" + strings.TrimSuffix(relPath, filepath.Ext(relPath))
	case "js", "jsx", "mjs", "cjs", "ts", "mts", "tsx":
		return "ts:" + strings.TrimSuffix(relPath, filepath.Ext(relPath))
	default:
		return "file:" + relPath
	}
}

// importEdgeToFact converts one raw ports.ImportEdge into a raw ports.Fact
// (FactDep, ProvDerived — tree-sitter output is REAL, D2). Object is left
// empty and the literal specifier is preserved in Attrs["spec"]: resolution
// to a unit ID or "ext:" happens later, off the parse pass (§1.2 "Rules").
func importEdgeToFact(e ports.ImportEdge, subject string) ports.Fact {
	return ports.Fact{
		Kind:    ports.FactDep,
		Subject: subject,
		Attrs:   map[string]string{"spec": e.ImportPath},
		Source:  ports.FactSource{File: e.FromFile, Line: e.StartLine},
		Prov:    ports.ProvDerived,
		TS:      time.Now().Unix(),
	}
}

// groupEdgesByFile maps a flat []ports.ImportEdge slice into a map from fileID
// to the edges belonging to that file. It derives fileID from idx.Files by
// matching the edge's FromFile (relative path) against FileMeta.Path.
//
// Only edges whose FromFile resolves to a known fileID are included — phantom
// edges (files absent from the index) are silently dropped (no phantom nodes).
// Returns nil when edges is empty or idx has no files.
// GroupEdgesByFile returns a fileID→edges map built from the index's path→fileID
// mapping. Used by init (cmd layer) and WarmCaches/Reindex (app layer) to
// produce the input for ReplaceAllEdges.
func GroupEdgesByFile(idx *ports.Index, edges []ports.ImportEdge) map[uint32][]ports.ImportEdge {
	return groupEdgesByFile(idx, edges)
}

func groupEdgesByFile(idx *ports.Index, edges []ports.ImportEdge) map[uint32][]ports.ImportEdge {
	if len(edges) == 0 || idx == nil || len(idx.Files) == 0 {
		return nil
	}
	// Build path→fileID lookup from the index.
	pathToID := make(map[string]uint32, len(idx.Files))
	for id, fm := range idx.Files {
		pathToID[fm.Path] = id
	}
	byFile := make(map[uint32][]ports.ImportEdge, len(idx.Files))
	for _, e := range edges {
		if id, ok := pathToID[e.FromFile]; ok {
			byFile[id] = append(byFile[id], e)
		}
	}
	if len(byFile) == 0 {
		return nil
	}
	return byFile
}

// BuildFileSet extracts the set of relative file paths from an index for O(1)
// resolver lookup. Exported for use by the init command (cmd layer).
func BuildFileSet(idx *ports.Index) map[string]bool {
	return buildFileSet(idx)
}

// buildFileSet extracts the set of all relative file paths from an index.
// Returns a map[relPath]bool for O(1) lookup by the §2.4 resolver.
func buildFileSet(idx *ports.Index) map[string]bool {
	if idx == nil || len(idx.Files) == 0 {
		return nil
	}
	s := make(map[string]bool, len(idx.Files))
	for _, fm := range idx.Files {
		s[fm.Path] = true
	}
	return s
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
