package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var treeDepth int

var treeCmd = &cobra.Command{
	Use:   "tree [dir]",
	Short: "Show directory structure",
	Long:  "Walks the filesystem and prints a tree structure. No daemon required.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTree,
}

func init() {
	treeCmd.Flags().IntVarP(&treeDepth, "depth", "d", 0, "Max depth (0 = unlimited)")
}

var treeIgnore = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true,
	"__pycache__":  true,
	".DS_Store":    true,
	"vendor":       true,
	".idea":        true,
	".vscode":      true,
	".aoa":         true,
}

func runTree(cmd *cobra.Command, args []string) error {
	dir := projectRoot()
	if len(args) > 0 {
		if filepath.IsAbs(args[0]) {
			dir = args[0]
		} else {
			dir = filepath.Join(dir, args[0])
		}
	}

	fmt.Println(dir)
	var sb strings.Builder
	walkTree(&sb, dir, "", 1)
	fmt.Print(sb.String())
	return nil
}

func walkTree(sb *strings.Builder, dir, prefix string, depth int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	// Filter ignored entries and separate dirs/files
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if treeIgnore[e.Name()] {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	// Sort each group alphabetically
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	// Combine: directories first, then files
	all := make([]os.DirEntry, 0, len(dirs)+len(files))
	all = append(all, dirs...)
	all = append(all, files...)

	for i, entry := range all {
		isLast := i == len(all)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}

		sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, connector, name))

		if entry.IsDir() && (treeDepth == 0 || depth < treeDepth) {
			newPrefix := prefix + "│   "
			if isLast {
				newPrefix = prefix + "    "
			}
			walkTree(sb, filepath.Join(dir, entry.Name()), newPrefix, depth+1)
		}
	}
}
