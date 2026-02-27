//go:build cgo

package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

var grammarCmd = &cobra.Command{
	Use:   "grammar",
	Short: "Manage tree-sitter grammar packs",
	Long:  "List, install, and inspect dynamically-loaded tree-sitter grammars.",
}

var grammarListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available and installed grammars",
	RunE:  runGrammarList,
}

var grammarInfoCmd = &cobra.Command{
	Use:   "info <language>",
	Short: "Show details about a grammar",
	Args:  cobra.ExactArgs(1),
	RunE:  runGrammarInfo,
}

var grammarInstallCmd = &cobra.Command{
	Use:   "install <language|pack>",
	Short: "Install grammar packs or individual grammars",
	Long: `Install grammars by name or pack:

  aoa grammar install core        Install P1 core grammars
  aoa grammar install common      Install P2 common grammars
  aoa grammar install extended    Install P3 extended grammars
  aoa grammar install specialist  Install P4 specialist grammars
  aoa grammar install all         Install all grammars
  aoa grammar install python      Install a single grammar`,
	Args: cobra.MinimumNArgs(1),
	RunE: runGrammarInstall,
}

var grammarPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show grammar search paths",
	RunE:  runGrammarPath,
}

func init() {
	grammarCmd.AddCommand(grammarListCmd)
	grammarCmd.AddCommand(grammarInfoCmd)
	grammarCmd.AddCommand(grammarInstallCmd)
	grammarCmd.AddCommand(grammarPathCmd)
}

func runGrammarList(cmd *cobra.Command, args []string) error {
	manifest := treesitter.BuiltinManifest()
	root := projectRoot()
	paths := treesitter.DefaultGrammarPaths(root)
	loader := treesitter.NewDynamicLoader(paths)
	installed := loader.InstalledGrammars()
	installedSet := make(map[string]bool)
	for _, g := range installed {
		installedSet[g] = true
	}

	// Build parser to check compiled-in grammars
	parser := treesitter.NewParser()

	// Group by priority
	for _, tier := range treesitter.AllPriorities {
		grammars := manifest.GrammarsByPriority(tier.Code)
		if len(grammars) == 0 {
			continue
		}
		sort.Strings(grammars)

		fmt.Fprintf(cmd.OutOrStdout(), "\n%s %s (%d languages)\n", tier.Code, tier.Name, len(grammars))
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 50))

		for _, name := range grammars {
			info := manifest.Grammars[name]
			status := "  "
			if parser.HasLanguage(name) {
				status = "B " // built-in
			} else if installedSet[name] || installedSet[treesitter.SOBaseName(name)] {
				status = "D " // dynamic
			}

			exts := strings.Join(info.Extensions, " ")
			fmt.Fprintf(cmd.OutOrStdout(), "  %s%-14s %s\n", status, name, exts)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "B = built-in (compiled)  D = dynamic (.so installed)")
	fmt.Fprintf(cmd.OutOrStdout(), "Search paths: %s\n", strings.Join(paths, ", "))
	return nil
}

func runGrammarInfo(cmd *cobra.Command, args []string) error {
	lang := args[0]
	manifest := treesitter.BuiltinManifest()

	info, ok := manifest.Grammars[lang]
	if !ok {
		return fmt.Errorf("unknown grammar: %s", lang)
	}

	root := projectRoot()
	paths := treesitter.DefaultGrammarPaths(root)
	loader := treesitter.NewDynamicLoader(paths)
	parser := treesitter.NewParser()

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Grammar:    %s\n", info.Name)
	fmt.Fprintf(out, "Version:    %s\n", info.Version)
	fmt.Fprintf(out, "Priority:   %s\n", info.Priority)
	fmt.Fprintf(out, "Extensions: %s\n", strings.Join(info.Extensions, ", "))
	fmt.Fprintf(out, "Repository: %s\n", info.RepoURL)

	if parser.HasLanguage(lang) {
		fmt.Fprintln(out, "Status:     built-in (compiled into binary)")
	} else if p := loader.GrammarPath(lang); p != "" {
		fmt.Fprintf(out, "Status:     installed (%s)\n", p)
	} else {
		fmt.Fprintln(out, "Status:     not installed")
	}

	fmt.Fprintf(out, "C symbol:   %s\n", treesitter.CSymbolName(lang))
	fmt.Fprintf(out, "SO file:    %s%s\n", treesitter.SOBaseName(lang), treesitter.LibExtension())
	return nil
}

func runGrammarInstall(cmd *cobra.Command, args []string) error {
	manifest := treesitter.BuiltinManifest()
	root := projectRoot()

	// Use global grammar dir (~/.aoa/grammars/) for installs.
	grammarDir := treesitter.GlobalGrammarDir()
	if grammarDir == "" {
		grammarDir = app.NewPaths(root).GrammarsDir
	}

	// Resolve targets — could be pack names or individual grammars.
	var targets []string
	for _, arg := range args {
		if pack := manifest.PackGrammars(arg); len(pack) > 0 {
			targets = append(targets, pack...)
		} else if _, ok := manifest.Grammars[arg]; ok {
			targets = append(targets, arg)
		} else {
			return fmt.Errorf("unknown grammar or pack: %s\nAvailable packs: %s",
				arg, strings.Join(treesitter.AllPacks, ", "))
		}
	}

	// Deduplicate.
	seen := make(map[string]bool)
	var unique []string
	for _, t := range targets {
		if !seen[t] {
			seen[t] = true
			unique = append(unique, t)
		}
	}
	sort.Strings(unique)

	// Check which are already installed.
	paths := treesitter.DefaultGrammarPaths(root)
	loader := treesitter.NewDynamicLoader(paths)
	ext := treesitter.LibExtension()
	platform := treesitter.PlatformString()

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Grammar directory: %s\n", grammarDir)
	fmt.Fprintf(out, "Platform:          %s\n\n", platform)

	var missing []string
	for _, name := range unique {
		if loader.GrammarPath(name) != "" {
			fmt.Fprintf(out, "  installed  %s\n", name)
		} else {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		fmt.Fprintln(out, "\nAll grammars already installed.")
		return nil
	}

	// Print curl commands for the user to run.
	fmt.Fprintf(out, "\nTo install %d missing grammars, run:\n\n", len(missing))
	fmt.Fprintf(out, "  mkdir -p %s\n", grammarDir)
	for _, name := range missing {
		url := fmt.Sprintf("%s/grammars-v1/%s-%s%s", manifest.BaseURL, name, platform, ext)
		dest := fmt.Sprintf("%s/%s%s", grammarDir, name, ext)
		fmt.Fprintf(out, "  curl -fSL -o %s \\\n    %s\n", dest, url)
	}
	fmt.Fprintln(out, "\nThen restart the daemon to pick up new grammars.")
	return nil
}

func runGrammarPath(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	paths := treesitter.DefaultGrammarPaths(root)
	out := cmd.OutOrStdout()

	for i, p := range paths {
		exists := "  "
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			exists = "* "
		}
		priority := "global"
		if i == 0 {
			priority = "project"
		}
		fmt.Fprintf(out, "%s%s (%s)\n", exists, p, priority)
	}
	fmt.Fprintln(out, "\n* = directory exists")
	return nil
}
