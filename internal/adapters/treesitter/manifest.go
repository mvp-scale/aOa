package treesitter

import (
	"encoding/json"
	"fmt"
	"os"
)

// GrammarInfo describes a single grammar in the manifest.
type GrammarInfo struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Priority   string   `json:"priority"` // "P1", "P2", "P3", "P4"
	Extensions []string `json:"extensions"`
	RepoURL    string   `json:"repo_url"`
	Sizes      PlatSize `json:"sizes,omitempty"`
	SHA256     PlatHash `json:"sha256,omitempty"`
}

// PlatSize maps platform (e.g. "linux-amd64") to file size in bytes.
type PlatSize map[string]int64

// PlatHash maps platform to SHA256 hex digest.
type PlatHash map[string]string

// Manifest is the grammar registry listing all available grammars.
type Manifest struct {
	Version  int                    `json:"version"`
	BaseURL  string                 `json:"base_url"`
	Grammars map[string]GrammarInfo `json:"grammars"`
}

// LoadManifest reads a manifest from a JSON file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// GrammarsByPriority returns grammar names for the given priority tier.
func (m *Manifest) GrammarsByPriority(priority string) []string {
	var names []string
	for name, info := range m.Grammars {
		if info.Priority == priority {
			names = append(names, name)
		}
	}
	return names
}

// PackGrammars returns grammar names for a named pack.
func (m *Manifest) PackGrammars(pack string) []string {
	switch pack {
	case "core":
		return m.GrammarsByPriority("P1")
	case "common":
		return m.GrammarsByPriority("P2")
	case "extended":
		return m.GrammarsByPriority("P3")
	case "specialist":
		return m.GrammarsByPriority("P4")
	case "all":
		names := make([]string, 0, len(m.Grammars))
		for name := range m.Grammars {
			names = append(names, name)
		}
		return names
	default:
		return nil
	}
}

// AllPriorities defines the priority tiers in order.
var AllPriorities = []struct {
	Code string
	Name string
}{
	{"P1", "Core"},
	{"P2", "Common"},
	{"P3", "Extended"},
	{"P4", "Specialist"},
}

// AllPacks defines the named download packs.
var AllPacks = []string{"core", "common", "extended", "specialist", "all"}

// BuiltinManifest returns a manifest with all known grammar metadata.
// This is embedded in the binary so `aoa grammar list` works without network.
func BuiltinManifest() *Manifest {
	return &Manifest{
		Version: 1,
		BaseURL: "https://github.com/corey/aoa/releases/download",
		Grammars: map[string]GrammarInfo{
			// P1 Core
			"python":     {Name: "python", Version: "0.25.0", Priority: "P1", Extensions: []string{".py", ".pyw"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-python"},
			"javascript": {Name: "javascript", Version: "0.25.0", Priority: "P1", Extensions: []string{".js", ".jsx", ".mjs", ".cjs"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-javascript"},
			"typescript": {Name: "typescript", Version: "0.23.2", Priority: "P1", Extensions: []string{".ts", ".mts"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-typescript"},
			"tsx":        {Name: "tsx", Version: "0.23.2", Priority: "P1", Extensions: []string{".tsx"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-typescript"},
			"go":         {Name: "go", Version: "0.25.0", Priority: "P1", Extensions: []string{".go"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-go"},
			"rust":       {Name: "rust", Version: "0.24.0", Priority: "P1", Extensions: []string{".rs"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-rust"},
			"java":       {Name: "java", Version: "0.23.5", Priority: "P1", Extensions: []string{".java"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-java"},
			"c":          {Name: "c", Version: "0.24.1", Priority: "P1", Extensions: []string{".c", ".h"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-c"},
			"cpp":        {Name: "cpp", Version: "0.23.4", Priority: "P1", Extensions: []string{".cpp", ".hpp", ".cc", ".cxx", ".hxx"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-cpp"},
			"bash":       {Name: "bash", Version: "0.25.1", Priority: "P1", Extensions: []string{".sh", ".bash", ".zsh"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-bash"},
			"json":       {Name: "json", Version: "0.24.8", Priority: "P1", Extensions: []string{".json", ".jsonc"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-json"},

			// P2 Common
			"csharp":     {Name: "csharp", Version: "0.23.1", Priority: "P2", Extensions: []string{".cs"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-c-sharp"},
			"ruby":       {Name: "ruby", Version: "0.23.1", Priority: "P2", Extensions: []string{".rb"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-ruby"},
			"php":        {Name: "php", Version: "0.24.2", Priority: "P2", Extensions: []string{".php"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-php"},
			"kotlin":     {Name: "kotlin", Version: "1.1.0", Priority: "P2", Extensions: []string{".kt", ".kts"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-kotlin"},
			"yaml":       {Name: "yaml", Version: "0.7.2", Priority: "P2", Extensions: []string{".yaml", ".yml"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-yaml"},
			"html":       {Name: "html", Version: "0.23.2", Priority: "P2", Extensions: []string{".html", ".htm"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-html"},
			"css":        {Name: "css", Version: "0.25.0", Priority: "P2", Extensions: []string{".css", ".scss", ".less"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-css"},
			"toml":       {Name: "toml", Version: "0.7.0", Priority: "P2", Extensions: []string{".toml"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-toml"},
			"markdown":   {Name: "markdown", Version: "0.5.2", Priority: "P2", Extensions: []string{".md", ".mdx"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-markdown"},
			"dockerfile": {Name: "dockerfile", Version: "0.2.0", Priority: "P2", Extensions: []string{"Dockerfile", ".dockerfile"}, RepoURL: "https://github.com/camdencheek/tree-sitter-dockerfile"},
			"sql":        {Name: "sql", Version: "0.3.11", Priority: "P2", Extensions: []string{".sql"}, RepoURL: "https://github.com/DerekStride/tree-sitter-sql"},

			// P3 Extended
			"scala":   {Name: "scala", Version: "0.24.0", Priority: "P3", Extensions: []string{".scala", ".sc"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-scala"},
			"lua":     {Name: "lua", Version: "0.4.1", Priority: "P3", Extensions: []string{".lua"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-lua"},
			"svelte":  {Name: "svelte", Version: "1.0.2", Priority: "P3", Extensions: []string{".svelte"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-svelte"},
			"hcl":     {Name: "hcl", Version: "1.2.0", Priority: "P3", Extensions: []string{".tf", ".hcl"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-hcl"},
			"swift":   {Name: "swift", Version: "0.0.0", Priority: "P3", Extensions: []string{".swift"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-swift"},
			"r":       {Name: "r", Version: "1.2.0", Priority: "P3", Extensions: []string{".r", ".R"}, RepoURL: "https://github.com/r-lib/tree-sitter-r"},
			"vue":     {Name: "vue", Version: "0.0.0", Priority: "P3", Extensions: []string{".vue"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-vue"},
			"dart":    {Name: "dart", Version: "0.0.0", Priority: "P3", Extensions: []string{".dart"}, RepoURL: "https://github.com/UserNobody14/tree-sitter-dart"},
			"elixir":  {Name: "elixir", Version: "0.0.0", Priority: "P3", Extensions: []string{".ex", ".exs"}, RepoURL: "https://github.com/ananthakumaran/tree-sitter-elixir"},
			"erlang":  {Name: "erlang", Version: "0.0.0", Priority: "P3", Extensions: []string{".erl", ".hrl"}, RepoURL: "https://github.com/abstractmachineslab/tree-sitter-erlang"},
			"groovy":  {Name: "groovy", Version: "0.0.0", Priority: "P3", Extensions: []string{".groovy", ".gradle"}, RepoURL: "https://github.com/murtaza64/tree-sitter-groovy"},
			"graphql": {Name: "graphql", Version: "0.0.0", Priority: "P3", Extensions: []string{".graphql", ".gql"}, RepoURL: "https://github.com/bkegley/tree-sitter-graphql"},
			"clojure": {Name: "clojure", Version: "0.0.13", Priority: "P3", Extensions: []string{".clj", ".cljs", ".cljc"}, RepoURL: "https://github.com/sogaiu/tree-sitter-clojure"},
			"gleam":   {Name: "gleam", Version: "1.1.0", Priority: "P3", Extensions: []string{".gleam"}, RepoURL: "https://github.com/gleam-lang/tree-sitter-gleam"},
			"cmake":   {Name: "cmake", Version: "0.7.2", Priority: "P3", Extensions: []string{".cmake"}, RepoURL: "https://github.com/uyha/tree-sitter-cmake"},
			"make":    {Name: "make", Version: "0.0.0", Priority: "P3", Extensions: []string{".mk"}, RepoURL: "https://github.com/alemuller/tree-sitter-make"},
			"nix":     {Name: "nix", Version: "0.3.0", Priority: "P3", Extensions: []string{".nix"}, RepoURL: "https://github.com/nix-community/tree-sitter-nix"},

			// P4 Specialist
			"ocaml":     {Name: "ocaml", Version: "0.24.2", Priority: "P4", Extensions: []string{".ml", ".mli"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-ocaml"},
			"verilog":   {Name: "verilog", Version: "1.0.3", Priority: "P4", Extensions: []string{".sv"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-verilog"},
			"haskell":   {Name: "haskell", Version: "0.23.1", Priority: "P4", Extensions: []string{".hs", ".lhs"}, RepoURL: "https://github.com/tree-sitter/tree-sitter-haskell"},
			"cuda":      {Name: "cuda", Version: "0.21.1", Priority: "P4", Extensions: []string{".cu", ".cuh"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-cuda"},
			"zig":       {Name: "zig", Version: "1.1.2", Priority: "P4", Extensions: []string{".zig"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-zig"},
			"julia":     {Name: "julia", Version: "0.0.0", Priority: "P4", Extensions: []string{".jl"}, RepoURL: "https://github.com/tree-sitter-grammars/tree-sitter-julia"},
			"d":         {Name: "d", Version: "0.8.2", Priority: "P4", Extensions: []string{".d"}, RepoURL: "https://github.com/gdamore/tree-sitter-d"},
			"fortran":   {Name: "fortran", Version: "0.5.1", Priority: "P4", Extensions: []string{".f90", ".f95", ".f03", ".f"}, RepoURL: "https://github.com/stadelmanma/tree-sitter-fortran"},
			"nim":       {Name: "nim", Version: "0.0.0", Priority: "P4", Extensions: []string{".nim"}, RepoURL: "https://github.com/alaviss/tree-sitter-nim"},
			"objc":      {Name: "objc", Version: "2.1.0", Priority: "P4", Extensions: []string{".m", ".mm"}, RepoURL: "https://github.com/amaanq/tree-sitter-objc"},
			"vhdl":      {Name: "vhdl", Version: "0.0.0", Priority: "P4", Extensions: []string{".vhd", ".vhdl"}, RepoURL: "https://github.com/alemuller/tree-sitter-vhdl"},
			"purescript": {Name: "purescript", Version: "0.3.0", Priority: "P4", Extensions: []string{".purs"}, RepoURL: "https://github.com/postsolar/tree-sitter-purescript"},
			"odin":      {Name: "odin", Version: "0.0.0", Priority: "P4", Extensions: []string{".odin"}, RepoURL: "https://github.com/ap29600/tree-sitter-odin"},
			"ada":       {Name: "ada", Version: "0.0.0", Priority: "P4", Extensions: []string{".ada", ".adb", ".ads"}, RepoURL: "https://github.com/briot/tree-sitter-ada"},
			"elm":       {Name: "elm", Version: "5.7.0", Priority: "P4", Extensions: []string{".elm"}, RepoURL: "https://github.com/elm-tooling/tree-sitter-elm"},
			"fennel":    {Name: "fennel", Version: "0.0.0", Priority: "P4", Extensions: []string{".fnl"}, RepoURL: "https://github.com/alexmozaidze/tree-sitter-fennel"},
			"glsl":      {Name: "glsl", Version: "0.2.0", Priority: "P4", Extensions: []string{".glsl", ".vert", ".frag"}, RepoURL: "https://github.com/theHamsta/tree-sitter-glsl"},
			"hlsl":      {Name: "hlsl", Version: "0.2.0", Priority: "P4", Extensions: []string{".hlsl"}, RepoURL: "https://github.com/theHamsta/tree-sitter-hlsl"},
		},
	}
}
