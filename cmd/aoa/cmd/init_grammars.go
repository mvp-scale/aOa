package cmd

import (
	"bufio"
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

// fetchFile downloads a URL to a local path using curl. No net/http in the binary.
func fetchFile(url, dest string) error {
	if _, err := exec.LookPath("curl"); err != nil {
		return fmt.Errorf("curl not found — install curl to continue")
	}
	cmd := exec.Command("curl", "-sfL", "--retry", "2", url, "-o", dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(dest)
		return fmt.Errorf("download failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
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

// confirmDownload prompts the user and returns true if they accept.
func confirmDownload() bool {
	fmt.Printf("  Download and install? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
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

// --- init flow ---

// grammarSetupFlow handles the complete grammar setup:
//   - Fetches parsers.json if missing (via curl)
//   - Scans project languages
//   - Shows download plan
//   - Downloads pre-built .so/.dylib, verifies SHA-256
//
// Returns (handled, pending):
//   - handled=true, pending=false: grammars ready, proceed to indexing
//   - handled=true, pending=true:  user needs to take action, halt init
//   - handled=false: could not set up (no curl, etc.), caller may try fallback
func grammarSetupFlow(root string) (handled bool, pending bool) {
	if os.Getenv("AOA_NO_GRAMMAR_DOWNLOAD") == "1" || noGrammarsFlag {
		return true, false
	}

	grammarDir := filepath.Join(root, ".aoa", "grammars")
	pjPath := filepath.Join(grammarDir, "parsers.json")
	platform := detectPlatform()
	ext := grammarLibExt()

	// Handle --update: fetch fresh parsers.json, then continue to check/download.
	if updateFlag {
		os.MkdirAll(grammarDir, 0755)
		fmt.Println("")
		if info, err := os.Stat(pjPath); err == nil {
			fmt.Printf("  parsers.json last updated: %s\n", info.ModTime().Format("2006-01-02"))
		}
		fmt.Println("  Fetching latest parsers.json...")
		if err := fetchFile(parsersJSONURL, pjPath); err != nil {
			fmt.Printf("  %v\n", err)
			printManualCurlMessage()
			return true, true
		}
		fmt.Println("  Updated.")
		// Fall through to check/download grammars below.
	}

	// Fetch parsers.json if missing.
	if _, err := os.Stat(pjPath); err != nil {
		os.MkdirAll(grammarDir, 0755)
		fmt.Println("")
		fmt.Println("  Fetching grammar manifest...")
		if err := fetchFile(parsersJSONURL, pjPath); err != nil {
			fmt.Printf("  %v\n", err)
			fmt.Println("")
			printManualCurlMessage()
			return true, true
		}
	}

	entries, err := loadParsersJSON(pjPath)
	if err != nil {
		fmt.Printf("  parsers.json error: %v\n", err)
		return false, false
	}

	langs := scanProjectLanguages(root)
	if len(langs) == 0 {
		return true, false
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
		fmt.Printf("  %d grammars ready\n", len(installed))
		return true, false
	}

	// Show download plan.
	totalSize := int64(0)
	fmt.Printf("\n  Your project uses %d languages. Grammars needed:\n\n", len(matchedNames))
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
	fmt.Printf("\n  Total: %d KB from grammars/%s/\n", totalSize/1024, platform)
	if len(installed) > 0 {
		fmt.Printf("  Already installed: %s\n", strings.Join(installed, ", "))
	}
	fmt.Println("")

	// Prompt for confirmation.
	if !confirmDownload() {
		fmt.Println("  Skipped. Re-run aoa init when ready.")
		return true, true
	}

	// Download and verify each grammar.
	fmt.Println("")
	start := time.Now()
	downloaded := 0
	for _, name := range missing {
		entry := findEntry(entries, name)
		if entry == nil {
			continue
		}
		ps, ok := entry.Platforms[platform]
		if !ok || ps.Status != "ok" {
			continue
		}

		url := fmt.Sprintf("%s/%s/%s%s", grammarsBaseURL, platform, name, ext)
		dest := filepath.Join(grammarDir, name+ext)

		if err := fetchFile(url, dest); err != nil {
			fmt.Printf("    %-14s FAILED: %v\n", name, err)
			continue
		}

		// Verify SHA-256.
		actual, err := sha256File(dest)
		if err != nil {
			fmt.Printf("    %-14s SHA error: %v\n", name, err)
			os.Remove(dest)
			continue
		}
		if actual != ps.SHA256 {
			fmt.Printf("    %-14s SHA MISMATCH\n", name)
			fmt.Printf("                expected: %s\n", ps.SHA256)
			fmt.Printf("                     got: %s\n", actual)
			os.Remove(dest)
			continue
		}

		fmt.Printf("    %-14s %4d KB   verified\n", name, ps.SizeBytes/1024)
		downloaded++
	}

	elapsed := time.Since(start)
	fmt.Printf("\n  %d grammars downloaded in %v\n", downloaded, elapsed.Round(time.Millisecond))

	// Generate grammars.conf for reference.
	generateGrammarsConf(grammarDir, matchedNames)

	// Re-check — all installed now?
	installed2, missing2 := checkInstalledGrammars(grammarDir, matchedNames)
	if len(missing2) > 0 {
		fmt.Printf("  %d grammars still missing: %s\n", len(missing2), strings.Join(missing2, ", "))
		return true, true
	}

	fmt.Printf("  %d grammars ready\n", len(installed2))
	return true, false
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

func printManualCurlMessage() {
	fmt.Println("  Download manually:")
	fmt.Printf("    curl -sL %s \\\n", parsersJSONURL)
	fmt.Printf("      -o .aoa/grammars/parsers.json\n")
	fmt.Println("")
	fmt.Println("  Then re-run: aoa init")
	fmt.Println("")
}

func printParsersJSONMessage(root string) {
	fmt.Println("")
	fmt.Println("  Grammar manifest not found.")
	fmt.Println("")
	fmt.Println("  parsers.json is a weekly-audited registry of tree-sitter")
	fmt.Println("  grammars — SHA-verified, open source, traced to maintainers.")
	fmt.Println("")
	printManualCurlMessage()
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
