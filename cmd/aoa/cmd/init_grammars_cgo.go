//go:build cgo

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/adapters/treesitter"
)

// scanAndDownloadGrammars walks the project, detects needed languages,
// and prints curl commands for the user to fetch missing grammar .so files.
// aoa never makes outbound network connections — the user runs the commands.
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

	// Check which grammars are already available.
	grammarDir := treesitter.GlobalGrammarDir()
	if grammarDir == "" {
		return
	}
	paths := treesitter.DefaultGrammarPaths(root)
	loader := treesitter.NewDynamicLoader(paths)

	var missing []string
	for lang := range needed {
		if loader.GrammarPath(lang) == "" {
			missing = append(missing, lang)
		}
	}

	if len(missing) == 0 {
		fmt.Printf("  %d grammars available for %d detected languages\n", len(needed), len(needed))
		return
	}

	sort.Strings(missing)
	printGrammarScript(missing, grammarDir, manifest)
}

// printGrammarScript outputs curl commands for the user to download grammars.
func printGrammarScript(langs []string, grammarDir string, manifest *treesitter.Manifest) {
	ext := treesitter.LibExtension()
	platform := treesitter.PlatformString()

	fmt.Printf("\n  %d languages detected, %d grammars needed\n", len(langs), len(langs))
	fmt.Printf("  To download, run the following commands:\n\n")
	fmt.Printf("  mkdir -p %s\n", grammarDir)
	for _, lang := range langs {
		url := fmt.Sprintf("%s/grammars-v1/%s-%s%s", manifest.BaseURL, lang, platform, ext)
		dest := filepath.Join(grammarDir, lang+ext)
		fmt.Printf("  curl -fSL -o %s \\\n    %s\n", dest, url)
	}
	fmt.Printf("\n  Then re-run: aoa init\n\n")
}
