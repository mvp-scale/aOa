//go:build cgo

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/corey/aoa/internal/adapters/treesitter"
)

// scanAndCompileGrammars walks the project, detects needed languages,
// and compiles missing grammars from go-sitter-forest C source in the
// Go module cache. No network calls — source is already local.
func scanAndDownloadGrammars(root string) {
	if os.Getenv("AOA_NO_GRAMMAR_DOWNLOAD") == "1" {
		return
	}
	if noGrammarsFlag {
		return
	}

	manifest := treesitter.BuiltinManifest()

	// Quick-scan: walk the project collecting file extensions.
	extCount := make(map[string]int)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			switch name {
			case ".git", "node_modules", ".venv", "__pycache__", "vendor",
				".idea", ".vscode", "dist", "build", ".aoa", ".next",
				"target", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != "" {
			extCount[ext]++
		}
		base := filepath.Base(path)
		lang := treesitter.ExtensionToLanguage(base)
		if lang != "" {
			extCount[base]++
		}
		return nil
	})

	// Map extensions to grammar names.
	needed := make(map[string]bool)
	for ext := range extCount {
		lang := treesitter.ExtensionToLanguage(ext)
		if lang != "" {
			if _, ok := manifest.Grammars[lang]; ok {
				needed[lang] = true
			}
		}
	}

	if len(needed) == 0 {
		return
	}

	// Grammars are project-scoped: {root}/.aoa/grammars/
	grammarDir := filepath.Join(root, ".aoa", "grammars")
	paths := treesitter.DefaultGrammarPaths(root)
	loader := treesitter.NewDynamicLoader(paths)

	var installed []string
	var missing []string
	for lang := range needed {
		if loader.GrammarPath(lang) != "" {
			installed = append(installed, lang)
		} else {
			missing = append(missing, lang)
		}
	}

	sort.Strings(installed)
	sort.Strings(missing)

	if len(missing) == 0 {
		fmt.Printf("  %d grammars ready\n", len(needed))
		return
	}

	// Show what we found.
	fmt.Printf("\n  Detected %d languages in your project\n", len(needed))
	if len(installed) > 0 {
		fmt.Printf("  Ready:   %s\n", strings.Join(installed, ", "))
	}
	fmt.Printf("  Missing: %s\n", strings.Join(missing, ", "))

	// Compile from go-sitter-forest source.
	compileGrammars(missing, grammarDir)
}

// compileGrammars compiles missing grammars from go-sitter-forest C source
// in the Go module cache. Requires gcc.
func compileGrammars(langs []string, grammarDir string) {
	// Locate go-sitter-forest in the module cache.
	forestBase := findForestBase()
	if forestBase == "" {
		fmt.Println("\n  go-sitter-forest not found in module cache.")
		fmt.Println("  Run: go mod download")
		return
	}

	// Check for a C compiler.
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "gcc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		fmt.Printf("\n  %s not found — cannot compile grammars.\n", cc)
		fmt.Println("  Install gcc and re-run: aoa init")
		return
	}

	ext := treesitter.LibExtension()
	sharedFlag := "-shared"
	if runtime.GOOS == "darwin" {
		sharedFlag = "-dynamiclib"
	}

	os.MkdirAll(grammarDir, 0755)

	total := len(langs)
	fmt.Printf("\n  Compiling %d grammars...\n\n", total)

	start := time.Now()
	ok := 0
	failed := 0

	for i, lang := range langs {
		srcDir := findGrammarSource(forestBase, lang)
		if srcDir == "" {
			fmt.Printf("    [%d/%d] %-14s skipped (not in forest)\n", i+1, total, lang)
			failed++
			continue
		}

		parserC := filepath.Join(srcDir, "parser.c")
		if _, err := os.Stat(parserC); os.IsNotExist(err) {
			fmt.Printf("    [%d/%d] %-14s skipped (no parser.c)\n", i+1, total, lang)
			failed++
			continue
		}

		outFile := filepath.Join(grammarDir, lang+ext)
		sources := []string{parserC}
		if scannerC := filepath.Join(srcDir, "scanner.c"); fileExists(scannerC) {
			sources = append(sources, scannerC)
		}

		t0 := time.Now()
		args := []string{sharedFlag, "-fPIC", "-O2", "-I", srcDir, "-o", outFile}
		args = append(args, sources...)
		out, err := exec.Command(cc, args...).CombinedOutput()
		dt := time.Since(t0)

		if err != nil {
			fmt.Printf("    [%d/%d] %-14s FAILED (%v)\n", i+1, total, lang, dt.Round(time.Millisecond))
			if len(out) > 0 {
				first := strings.SplitN(string(out), "\n", 2)[0]
				fmt.Printf("             %s\n", first)
			}
			os.Remove(outFile)
			failed++
			continue
		}

		sizeKB := int64(0)
		if info, _ := os.Stat(outFile); info != nil {
			sizeKB = info.Size() / 1024
		}

		// ETA after a few data points.
		eta := ""
		if i >= 2 && i < total-1 {
			avg := time.Since(start) / time.Duration(i+1)
			remaining := avg * time.Duration(total-i-1)
			eta = fmt.Sprintf("  ~%s left", remaining.Round(time.Second))
		}

		fmt.Printf("    [%d/%d] %-14s ok  %3d KB  %v%s\n", i+1, total, lang, sizeKB, dt.Round(time.Millisecond), eta)
		ok++
	}

	elapsed := time.Since(start)
	fmt.Printf("\n  %d compiled, %d failed (%v)\n", ok, failed, elapsed.Round(time.Millisecond))
}

// findForestBase locates go-sitter-forest in the Go module cache.
func findForestBase() string {
	cache := os.Getenv("GOMODCACHE")
	if cache == "" {
		out, err := exec.Command("go", "env", "GOMODCACHE").Output()
		if err != nil {
			return ""
		}
		cache = strings.TrimSpace(string(out))
	}
	base := filepath.Join(cache, "github.com", "alexaandru", "go-sitter-forest")
	if _, err := os.Stat(base); err != nil {
		return ""
	}
	return base
}

// findGrammarSource finds the source directory for a grammar in go-sitter-forest.
func findGrammarSource(forestBase, lang string) string {
	matches, err := filepath.Glob(filepath.Join(forestBase, lang+"@*"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	dir := matches[len(matches)-1]
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
