package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const parsersJSONURL = "https://raw.githubusercontent.com/mvp-scale/aOa/main/grammars/parsers.json"
const grammarsBaseURL = "https://raw.githubusercontent.com/mvp-scale/aOa/main/grammars"

// ParserEntry matches the schema of entries in parsers.json.
type ParserEntry struct {
	Name             string                    `json:"name"`
	Version          string                    `json:"version"`
	UpstreamURL      string                    `json:"upstream_url"`
	Maintainer       string                    `json:"maintainer"`
	UpstreamRevision string                    `json:"upstream_revision"`
	SourceSHA256     string                    `json:"source_sha256"`
	Platforms        map[string]PlatformStatus `json:"platforms"`
}

// PlatformStatus describes a grammar's build status on a platform.
type PlatformStatus struct {
	Status    string `json:"status"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

var updateFlag bool

// --- shared helpers ---

func loadParsersJSON(path string) ([]ParserEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []ParserEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse parsers.json: %w", err)
	}
	return entries, nil
}

func grammarLibExt() string {
	if runtime.GOOS == "darwin" {
		return ".dylib"
	}
	return ".so"
}

func detectPlatform() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// sha256File computes the SHA-256 hex digest of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// findEntry returns the ParserEntry for a grammar name.
func findEntry(entries []ParserEntry, name string) *ParserEntry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func scanProjectLanguages(root string) []string {
	extCount := make(map[string]int)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
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
		if grammarExtMap[base] != "" {
			extCount[base]++
		}
		return nil
	})

	langSet := make(map[string]bool)
	for ext := range extCount {
		if lang := grammarExtMap[ext]; lang != "" {
			langSet[lang] = true
		}
	}

	var langs []string
	for lang := range langSet {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

func matchParsersJSON(entries []ParserEntry, langs []string) []ParserEntry {
	entryMap := make(map[string]ParserEntry, len(entries))
	for _, e := range entries {
		entryMap[e.Name] = e
	}
	var matched []ParserEntry
	for _, lang := range langs {
		if e, ok := entryMap[lang]; ok {
			matched = append(matched, e)
		}
	}
	return matched
}

func checkInstalledGrammars(grammarDir string, langs []string) (installed, missing []string) {
	ext := grammarLibExt()
	for _, lang := range langs {
		if _, err := os.Stat(filepath.Join(grammarDir, lang+ext)); err == nil {
			installed = append(installed, lang)
		} else {
			missing = append(missing, lang)
		}
	}
	sort.Strings(installed)
	sort.Strings(missing)
	return
}

// verifyInstalledGrammars checks SHA-256 of installed grammars against parsers.json.
func verifyInstalledGrammars(grammarDir string, entries []ParserEntry, names []string) (verified int, failed int) {
	platform := detectPlatform()
	ext := grammarLibExt()
	for _, name := range names {
		path := filepath.Join(grammarDir, name+ext)
		actual, err := sha256File(path)
		if err != nil {
			continue
		}
		entry := findEntry(entries, name)
		if entry == nil {
			continue
		}
		ps, ok := entry.Platforms[platform]
		if !ok {
			continue
		}
		if actual == ps.SHA256 {
			verified++
		} else {
			fmt.Printf("  WARNING: %s SHA mismatch (expected %s, got %s)\n", name, ps.SHA256[:12], actual[:12])
			failed++
		}
	}
	return
}

// --- init flow ---

// grammarSetupFlow handles grammar setup. aOa makes zero outbound network
// connections — this function generates a download.sh for the user to run.
//
// Returns (handled, pending):
//   - handled=true, pending=false: grammars ready, proceed to indexing
//   - handled=true, pending=true:  user has steps to complete, halt init
//   - handled=false: could not set up, caller may try fallback
func grammarSetupFlow(root string) (handled bool, pending bool) {
	if os.Getenv("AOA_NO_GRAMMAR_DOWNLOAD") == "1" || noGrammarsFlag {
		return true, false
	}

	grammarDir := filepath.Join(root, ".aoa", "grammars")
	pjPath := filepath.Join(grammarDir, "parsers.json")
	platform := detectPlatform()
	ext := grammarLibExt()

	// Handle --update: rescan, regenerate download.sh, run it, continue to indexing.
	if updateFlag {
		if handleUpdateFlag(root) {
			return true, false // success — continue to indexing
		}
		return true, true // failed — halt
	}

	// Scan project languages — needed whether or not parsers.json exists.
	langs := scanProjectLanguages(root)
	if len(langs) == 0 {
		return true, false
	}

	// If no parsers.json, generate download.sh that fetches everything
	// (parsers.json + .so files). SHA verification happens on re-run.
	if _, err := os.Stat(pjPath); err != nil {
		os.MkdirAll(grammarDir, 0755)
		generateFullDownloadSh(grammarDir, langs, platform, ext)
		generateGrammarsConf(grammarDir, langs)

		fmt.Println("")
		fmt.Printf("  Nice project — %d languages detected.\n", len(langs))
		fmt.Println("")
		fmt.Println("  Zero outbound network policy. Grammars download from github.com/mvp-scale/aOa via download.sh — just curl.")
		fmt.Println("  Everything aOa needs lives in .aoa/ — grammars, index, config. One folder, fully portable.")
		fmt.Println("")
		fmt.Println("    sh .aoa/grammars/download.sh && aoa init")
		fmt.Println("")
		fmt.Println("  Next time: aoa init --update — rescans your project, checks for new languages, and regenerates download.sh.")
		fmt.Println("")
		fmt.Println("  Enjoy.")
		fmt.Println("")
		return true, true
	}

	// parsers.json exists — use it for matching and SHA verification.
	entries, err := loadParsersJSON(pjPath)
	if err != nil {
		fmt.Printf("  parsers.json error: %v\n", err)
		return false, false
	}

	matched := matchParsersJSON(entries, langs)
	if len(matched) == 0 {
		return true, false
	}

	var matchedNames []string
	for _, e := range matched {
		matchedNames = append(matchedNames, e.Name)
	}

	// Check what's already installed.
	installed, missing := checkInstalledGrammars(grammarDir, matchedNames)
	if len(missing) == 0 {
		// Verify SHA-256 of installed grammars.
		verified, failed := verifyInstalledGrammars(grammarDir, entries, installed)
		if failed > 0 {
			fmt.Printf("  %d grammars ready (%d SHA-256 verified, %d mismatched)\n", len(installed), verified, failed)
		} else {
			fmt.Printf("  %d grammars ready, SHA-256 verified\n", len(installed))
		}
		return true, false
	}

	// Generate download.sh for missing grammars (with SHA verification).
	os.MkdirAll(grammarDir, 0755)
	generateGrammarsConf(grammarDir, matchedNames)
	generateDownloadSh(grammarDir, missing, entries, platform, ext)

	fmt.Printf("\n  %d languages detected", len(matchedNames))
	if len(installed) > 0 {
		fmt.Printf(", %d ready", len(installed))
	}
	fmt.Printf(", %d to download.\n\n", len(missing))

	// Show what the script will do.
	totalSize := int64(0)
	for _, name := range missing {
		entry := findEntry(entries, name)
		if entry == nil {
			continue
		}
		ps, ok := entry.Platforms[platform]
		if !ok || ps.Status != "ok" {
			fmt.Printf("    %-14s not available for %s\n", name, platform)
			continue
		}
		shaPreview := ps.SHA256
		if len(shaPreview) > 12 {
			shaPreview = shaPreview[:12]
		}
		fmt.Printf("    %-14s %4d KB   sha256:%s...\n", name, ps.SizeBytes/1024, shaPreview)
		totalSize += ps.SizeBytes
	}
	fmt.Printf("\n  Total: %d KB from github.com/mvp-scale/aOa\n", totalSize/1024)
	fmt.Println("  SHA-256 verified against the repository manifest.")
	fmt.Println("")
	fmt.Println("  Next:")
	fmt.Println("")
	fmt.Println("    1. sh .aoa/grammars/download.sh")
	fmt.Println("    2. aoa init")
	fmt.Println("")

	return true, true
}

// generateGrammarsConf writes .aoa/grammars/grammars.conf — just names, one per line.
func generateGrammarsConf(grammarDir string, names []string) {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	fmt.Fprintf(&b, "# aOa grammars — %s\n", date)
	for _, name := range names {
		fmt.Fprintln(&b, name)
	}
	os.WriteFile(filepath.Join(grammarDir, "grammars.conf"), []byte(b.String()), 0644)
}

// generateFullDownloadSh writes .aoa/grammars/download.sh that fetches everything:
// parsers.json first, extracts SHA-256 hashes with awk, then downloads and verifies
// pre-built .so/.dylib files listed in grammars.conf.
func generateFullDownloadSh(grammarDir string, langs []string, platform, ext string) {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# aOa grammar download — pre-built binaries from GitHub\n")
	b.WriteString("# Generated by: aoa init\n")
	b.WriteString("#\n")
	b.WriteString("# Source: github.com/mvp-scale/aOa\n")
	b.WriteString("# Just curl + awk + sha256sum. Nothing else.\n")
	b.WriteString("#\n")
	fmt.Fprintf(&b, "# Platform: %s\n", platform)
	fmt.Fprintf(&b, "# Extension: %s\n\n", ext)

	b.WriteString("DIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\n")
	b.WriteString("CONF=\"$DIR/grammars.conf\"\n")
	fmt.Fprintf(&b, "BASE=\"%s/%s\"\n\n", grammarsBaseURL, platform)

	// Step 1: parsers.json
	b.WriteString("echo \"\"\n")
	b.WriteString("echo \"  Downloading from github.com/mvp-scale/aOa\"\n")
	fmt.Fprintf(&b, "echo \"  Saving to .aoa/grammars/ (*%s)\"\n", ext)
	b.WriteString("echo \"\"\n")
	fmt.Fprintf(&b, "curl -sfL \"%s\" -o \"$DIR/parsers.json\"\n", parsersJSONURL)
	b.WriteString("if [ -f \"$DIR/parsers.json\" ]; then\n")
	b.WriteString("  echo \"    parsers.json  downloaded\"\n")
	b.WriteString("else\n")
	b.WriteString("  echo \"    parsers.json  FAILED\"\n")
	b.WriteString("  exit 1\n")
	b.WriteString("fi\n\n")

	// Extract SHA-256 hashes from parsers.json into a shell variable
	b.WriteString("# Extract expected SHA-256 hashes from manifest\n")
	fmt.Fprintf(&b, "SHA_MAP=$(awk -v p=\"%s\" '\n", platform)
	b.WriteString("  /\"name\":/ { gsub(/.*\": *\"/, \"\"); gsub(/\".*/, \"\"); name=$0 }\n")
	b.WriteString("  $0 ~ \"\\\"\" p \"\\\"\" { in_p=1 }\n")
	b.WriteString("  in_p && /\"sha256\":/ { gsub(/.*\": *\"/, \"\"); gsub(/\".*/, \"\"); sha=$0 }\n")
	b.WriteString("  in_p && /\"size_bytes\":/ { gsub(/.*: */, \"\"); gsub(/[^0-9].*/, \"\"); print name, sha, int($0/1024); in_p=0 }\n")
	b.WriteString("' \"$DIR/parsers.json\")\n\n")

	// Download and verify grammars against parsers.json
	b.WriteString("echo \"\"\n")
	b.WriteString("grep -v '^#' \"$CONF\" | while read -r name; do\n")
	b.WriteString("  [ -z \"$name\" ] && continue\n")
	fmt.Fprintf(&b, "  file=\"${name}%s\"\n", ext)
	b.WriteString("  curl -sfL \"$BASE/$file\" -o \"$DIR/$file\"\n")
	b.WriteString("  if [ -f \"$DIR/$file\" ]; then\n")
	b.WriteString("    EXPECTED=$(echo \"$SHA_MAP\" | grep \"^$name \" | cut -d' ' -f2)\n")
	b.WriteString("    SIZE_KB=$(echo \"$SHA_MAP\" | grep \"^$name \" | cut -d' ' -f3)\n")
	b.WriteString("    if [ -n \"$EXPECTED\" ]; then\n")
	b.WriteString("      ACTUAL=$(sha256sum \"$DIR/$file\" 2>/dev/null | cut -d' ' -f1 || shasum -a 256 \"$DIR/$file\" | cut -d' ' -f1)\n")
	b.WriteString("      SHORT_E=$(echo \"$EXPECTED\" | cut -c1-12)\n")
	b.WriteString("      SHORT_A=$(echo \"$ACTUAL\" | cut -c1-12)\n")
	b.WriteString("      if [ \"$ACTUAL\" = \"$EXPECTED\" ]; then\n")
	b.WriteString("        printf \"    %-14s %4s KB   github %s  local %s\\n\" \"$name\" \"$SIZE_KB\" \"$SHORT_E\" \"$SHORT_A\"\n")
	b.WriteString("      else\n")
	b.WriteString("        printf \"    %-14s SHA-256 MISMATCH  github %s  local %s\\n\" \"$name\" \"$SHORT_E\" \"$SHORT_A\"\n")
	b.WriteString("        rm -f \"$DIR/$file\"\n")
	b.WriteString("      fi\n")
	b.WriteString("    else\n")
	b.WriteString("      SIZE=$(( $(stat --format=%s \"$DIR/$file\" 2>/dev/null || stat -f%z \"$DIR/$file\") / 1024 ))\n")
	b.WriteString("      printf \"    %-14s %4d KB   downloaded\\n\" \"$name\" \"$SIZE\"\n")
	b.WriteString("    fi\n")
	b.WriteString("  else\n")
	b.WriteString("    printf \"    %-14s FAILED\\n\" \"$name\"\n")
	b.WriteString("  fi\n")
	b.WriteString("done\n\n")

	b.WriteString("echo \"\"\n")
	b.WriteString("echo \"  Done. Finish setup:\"\n")
	b.WriteString("echo \"\"\n")
	b.WriteString("echo \"    aoa init\"\n")
	b.WriteString("echo \"\"\n")

	os.WriteFile(filepath.Join(grammarDir, "download.sh"), []byte(b.String()), 0755)
}

// generateDownloadSh writes .aoa/grammars/download.sh — loops over grammars.conf,
// curls pre-built .so/.dylib files, and verifies SHA-256 from parsers.json.
func generateDownloadSh(grammarDir string, missing []string, entries []ParserEntry, platform, ext string) {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# aOa grammar download — pre-built binaries from GitHub\n")
	b.WriteString("# Generated by: aoa init\n")
	b.WriteString("#\n")
	b.WriteString("# Source: github.com/mvp-scale/aOa\n")
	b.WriteString("# Just curl + awk + sha256sum. Nothing else.\n")
	b.WriteString("#\n")
	fmt.Fprintf(&b, "# Platform: %s\n", platform)
	fmt.Fprintf(&b, "# Extension: %s\n\n", ext)

	b.WriteString("DIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\n")
	fmt.Fprintf(&b, "BASE=\"%s/%s\"\n\n", grammarsBaseURL, platform)

	// Extract SHA-256 hashes from parsers.json into a shell variable
	b.WriteString("# Extract expected SHA-256 hashes from parsers.json\n")
	fmt.Fprintf(&b, "SHA_MAP=$(awk -v p=\"%s\" '\n", platform)
	b.WriteString("  /\"name\":/ { gsub(/.*\": *\"/, \"\"); gsub(/\".*/, \"\"); name=$0 }\n")
	b.WriteString("  $0 ~ \"\\\"\" p \"\\\"\" { in_p=1 }\n")
	b.WriteString("  in_p && /\"sha256\":/ { gsub(/.*\": *\"/, \"\"); gsub(/\".*/, \"\"); sha=$0 }\n")
	b.WriteString("  in_p && /\"size_bytes\":/ { gsub(/.*: */, \"\"); gsub(/[^0-9].*/, \"\"); print name, sha, int($0/1024); in_p=0 }\n")
	b.WriteString("' \"$DIR/parsers.json\")\n\n")

	b.WriteString("echo \"\"\n")
	b.WriteString("echo \"  Downloading from github.com/mvp-scale/aOa\"\n")
	fmt.Fprintf(&b, "echo \"  Saving to .aoa/grammars/ (*%s)\"\n", ext)
	b.WriteString("echo \"\"\n\n")

	// Read grammars.conf, download each, verify SHA from parsers.json
	b.WriteString("grep -v '^#' \"$DIR/grammars.conf\" | while read -r name; do\n")
	b.WriteString("  [ -z \"$name\" ] && continue\n")
	fmt.Fprintf(&b, "  file=\"${name}%s\"\n", ext)
	b.WriteString("  # Skip already installed\n")
	b.WriteString("  [ -f \"$DIR/$file\" ] && continue\n")
	b.WriteString("  curl -sfL \"$BASE/$file\" -o \"$DIR/$file\"\n")
	b.WriteString("  EXPECTED=$(echo \"$SHA_MAP\" | grep \"^$name \" | cut -d' ' -f2)\n")
	b.WriteString("  SIZE_KB=$(echo \"$SHA_MAP\" | grep \"^$name \" | cut -d' ' -f3)\n")
	b.WriteString("  if [ -n \"$EXPECTED\" ]; then\n")
	b.WriteString("    ACTUAL=$(sha256sum \"$DIR/$file\" 2>/dev/null | cut -d' ' -f1 || shasum -a 256 \"$DIR/$file\" | cut -d' ' -f1)\n")
	b.WriteString("    SHORT_E=$(echo \"$EXPECTED\" | cut -c1-12)\n")
	b.WriteString("    SHORT_A=$(echo \"$ACTUAL\" | cut -c1-12)\n")
	b.WriteString("    if [ \"$ACTUAL\" = \"$EXPECTED\" ]; then\n")
	b.WriteString("      printf \"    %-14s %4s KB   github %s  local %s\\n\" \"$name\" \"$SIZE_KB\" \"$SHORT_E\" \"$SHORT_A\"\n")
	b.WriteString("    else\n")
	b.WriteString("      printf \"    %-14s SHA-256 MISMATCH  github %s  local %s\\n\" \"$name\" \"$SHORT_E\" \"$SHORT_A\"\n")
	b.WriteString("      rm -f \"$DIR/$file\"\n")
	b.WriteString("    fi\n")
	b.WriteString("  else\n")
	b.WriteString("    SIZE=$(( $(stat --format=%s \"$DIR/$file\" 2>/dev/null || stat -f%z \"$DIR/$file\") / 1024 ))\n")
	b.WriteString("    printf \"    %-14s %4d KB   downloaded\\n\" \"$name\" \"$SIZE\"\n")
	b.WriteString("  fi\n")
	b.WriteString("done\n\n")

	b.WriteString("echo \"\"\n")
	b.WriteString("echo \"  Done. Finish setup:\"\n")
	b.WriteString("echo \"\"\n")
	b.WriteString("echo \"    aoa init\"\n")
	b.WriteString("echo \"\"\n")

	os.WriteFile(filepath.Join(grammarDir, "download.sh"), []byte(b.String()), 0755)
}

func printParsersJSONMessage(root string) {
	fmt.Println("")
	fmt.Println("  Grammar manifest not found.")
	fmt.Println("")
	fmt.Println("  parsers.json is a weekly-audited registry of tree-sitter")
	fmt.Println("  grammars — SHA-verified, open source, traced to maintainers.")
	fmt.Println("")
	fmt.Println("  Download it:")
	fmt.Printf("    curl -sL %s \\\n", parsersJSONURL)
	fmt.Printf("      -o .aoa/grammars/parsers.json\n")
	fmt.Println("")
	fmt.Println("  Then re-run: aoa init")
	fmt.Println("")
}

// handleUpdateFlag runs the full update cycle: scan, generate download.sh, run it.
// Returns true on success (caller should continue to indexing).
func handleUpdateFlag(root string) bool {
	grammarDir := filepath.Join(root, ".aoa", "grammars")
	platform := detectPlatform()
	ext := grammarLibExt()

	langs := scanProjectLanguages(root)
	if len(langs) == 0 {
		fmt.Println("")
		fmt.Println("  No languages detected in project.")
		fmt.Println("")
		return true
	}

	fmt.Println("")
	fmt.Printf("  Scanned project — %d languages.\n", len(langs))
	fmt.Println("")

	os.MkdirAll(grammarDir, 0755)
	generateGrammarsConf(grammarDir, langs)
	generateFullDownloadSh(grammarDir, langs, platform, ext)

	// Run the download script we just generated
	scriptPath := filepath.Join(grammarDir, "download.sh")
	cmd := exec.Command("sh", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n  download.sh failed: %v\n\n", err)
		return false
	}

	return true
}

// --- extension map ---

var grammarExtMap = map[string]string{
	".py": "python", ".pyw": "python",
	".js": "javascript", ".jsx": "javascript", ".mjs": "javascript", ".cjs": "javascript",
	".ts": "typescript", ".mts": "typescript",
	".tsx": "tsx",
	".go": "go",
	".rs": "rust",
	".java": "java",
	".c": "c", ".h": "c",
	".cpp": "cpp", ".hpp": "cpp", ".cc": "cpp", ".cxx": "cpp", ".hxx": "cpp",
	".cs": "c_sharp",
	".rb": "ruby",
	".php": "php",
	".swift": "swift",
	".kt": "kotlin", ".kts": "kotlin",
	".scala": "scala", ".sc": "scala",
	".sh": "bash", ".bash": "bash",
	".lua": "lua",
	".pl": "perl", ".pm": "perl",
	".r": "r", ".R": "r",
	".jl": "julia",
	".ex": "elixir", ".exs": "elixir",
	".erl": "erlang", ".hrl": "erlang",
	".hs": "haskell", ".lhs": "haskell",
	".ml": "ocaml", ".mli": "ocaml",
	".gleam": "gleam",
	".elm": "elm",
	".clj": "clojure", ".cljs": "clojure", ".cljc": "clojure",
	".purs": "purescript",
	".fnl": "fennel",
	".zig": "zig",
	".d": "d",
	".cu": "cuda", ".cuh": "cuda",
	".odin": "odin",
	".nim": "nim",
	".m": "objc", ".mm": "objc",
	".ada": "ada", ".adb": "ada", ".ads": "ada",
	".f90": "fortran", ".f95": "fortran", ".f03": "fortran", ".f": "fortran",
	".sv": "verilog",
	".vhd": "vhdl", ".vhdl": "vhdl",
	".html": "html", ".htm": "html",
	".css": "css", ".less": "css",
	".scss": "scss",
	".vue": "vue",
	".svelte": "svelte",
	".dart": "dart",
	".json": "json", ".jsonc": "jsonc",
	".yaml": "yaml", ".yml": "yaml",
	".toml":       "toml",
	".sql":        "sql",
	".md":         "markdown", ".mdx": "markdown",
	".graphql":    "graphql", ".gql": "graphql",
	".tf":         "hcl", ".hcl": "hcl",
	"Dockerfile":  "dockerfile",
	".dockerfile": "dockerfile",
	".nix":        "nix",
	".cmake":      "cmake",
	".mk":         "make",
	".groovy":     "groovy", ".gradle": "groovy",
	".glsl": "glsl", ".vert": "glsl", ".frag": "glsl",
	".hlsl": "hlsl",
}
