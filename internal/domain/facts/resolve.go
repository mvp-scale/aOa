// Package facts implements the §2.4 import-spec resolver: given raw ImportEdge
// slices (emitted by extractors), it classifies each spec as intra-repo (file/
// package target), external ("ext:" prefix), or unresolved (looks internal,
// probe failed).
//
// Resolver design (01-facts-substrate.md §2.4):
//
//	Phase A (extractors): emit raw ImportEdge with attrs.spec — done by treesitter adapters.
//	Phase B (this package): with the complete file table + manifests, resolve every
//	raw spec to a unit ID or "ext:". Pure function; deterministic on same inputs.
//
// Per-language rules implemented here: Go, Python, JavaScript/TypeScript.
// Languages without an extractor emit nothing; the resolver never sees their edges.
package facts

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/ports"
)

// ResolveResult is the output of Resolve: every input edge appears in exactly
// one of Resolved or Unresolved.
type ResolveResult struct {
	// Resolved contains all edges with ImportPath classified:
	//   - intra-repo: ImportPath = relative directory path (e.g. "internal/app")
	//   - external:   ImportPath = "ext:<spec>" or "ext:std/<spec>"
	Resolved []ports.ImportEdge

	// Unresolved contains edges whose spec appeared intra-repo (relative path or
	// module-prefixed) but did not probe to any file in the current index.
	// These are findings fuel (broken-import candidates, §2.4) and are persisted
	// in the facts_unresolved bucket by the caller — never silently dropped.
	Unresolved []ports.ImportEdge
}

// Manifests holds build-manifest data collected from the project root.
// All fields are optional; a zero-value Manifests is valid (everything resolves
// to "ext:" or stays unresolved for relative imports).
type Manifests struct {
	// GoModules maps module path → relative directory of the module root.
	// For a single-module repo: {"github.com/foo/bar": ""}.
	// For a monorepo: {"github.com/foo/bar": "backend", "github.com/foo/ui": "ui"}.
	GoModules map[string]string
}

// ReadManifests finds and reads build manifests rooted at dir.
// Currently collects go.mod files (Go module paths). Safe to call with an
// arbitrary directory; returns zero-value Manifests if nothing is found.
func ReadManifests(dir string) Manifests {
	m := Manifests{GoModules: make(map[string]string)}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip directories that never contain user code.
			name := info.Name()
			switch name {
			case ".git", "node_modules", ".venv", "__pycache__", "vendor",
				".idea", ".vscode", "dist", "build", ".aoa", ".next", "target", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() != "go.mod" {
			return nil
		}
		modPath := readGoModPath(path)
		if modPath == "" {
			return nil
		}
		rel, err := filepath.Rel(dir, filepath.Dir(path))
		if err != nil {
			rel = ""
		}
		// Normalise to forward-slash relative paths.
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		m.GoModules[modPath] = rel
		return nil
	})
	return m
}

// readGoModPath returns the module path from a go.mod file, or "" on error.
func readGoModPath(gomodPath string) string {
	f, err := os.Open(gomodPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// Resolve classifies raw import edges extracted during the parse pass.
// Every input edge appears in exactly one output slice (Resolved or Unresolved).
//
// fileSet is the set of all project-relative file paths from Index.Files.
// manifests provides module-path and workspace context for intra-repo detection.
//
// Resolve is a pure function: same inputs → byte-identical result (§2.4 determinism).
func Resolve(edges []ports.ImportEdge, fileSet map[string]bool, manifests Manifests) ResolveResult {
	if len(edges) == 0 {
		return ResolveResult{}
	}

	// Build a directory set for O(1) intra-repo package directory checks.
	// A directory is "known" if at least one file in fileSet lives under it.
	dirSet := buildDirSet(fileSet)

	var result ResolveResult
	for _, e := range edges {
		lang := langFromPath(e.FromFile)
		resolved, ok := resolveOne(e, lang, fileSet, dirSet, manifests)
		if ok {
			result.Resolved = append(result.Resolved, resolved)
		} else {
			result.Unresolved = append(result.Unresolved, e)
		}
	}
	return result
}

// buildDirSet extracts all parent directory paths from the file set.
// Returns a map where every directory that contains at least one known file is true.
func buildDirSet(fileSet map[string]bool) map[string]bool {
	dirs := make(map[string]bool, len(fileSet))
	for f := range fileSet {
		dir := filepath.ToSlash(filepath.Dir(f))
		for dir != "." && dir != "" && !dirs[dir] {
			dirs[dir] = true
			dir = filepath.ToSlash(filepath.Dir(dir))
		}
	}
	return dirs
}

// langFromPath returns a normalised language tag for a relative file path.
func langFromPath(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "go":
		return "go"
	case "py", "pyw":
		return "python"
	case "js", "jsx", "mjs", "cjs":
		return "javascript"
	case "ts", "tsx", "mts":
		return "typescript"
	default:
		return ""
	}
}

// resolveOne resolves a single edge. Returns the resolved edge (true) or
// signals that it is unresolved (false) so the caller can route it.
func resolveOne(e ports.ImportEdge, lang string, fileSet, dirSet map[string]bool, manifests Manifests) (ports.ImportEdge, bool) {
	spec := e.ImportPath
	switch lang {
	case "go":
		return resolveGo(e, spec, dirSet, manifests)
	case "python":
		return resolvePython(e, spec, fileSet)
	case "javascript", "typescript":
		return resolveJS(e, spec, fileSet)
	default:
		// Unknown language — classify as external (no phantom nodes).
		out := e
		out.ImportPath = "ext:" + spec
		return out, true
	}
}

// resolveGo implements §2.4 Go resolution rules:
//
//  1. Std lib (no dot in first path segment) → "ext:std/<spec>"
//  2. Module prefix match → relative package directory (intra-repo)
//  3. All others → "ext:<spec>"
//
// Go imports are ALWAYS resolved to one of the above — no unresolved case for Go.
func resolveGo(e ports.ImportEdge, spec string, dirSet map[string]bool, manifests Manifests) (ports.ImportEdge, bool) {
	out := e

	// Rule 1: std lib (first segment has no dot).
	firstSeg := spec
	if i := strings.Index(spec, "/"); i >= 0 {
		firstSeg = spec[:i]
	}
	if !strings.Contains(firstSeg, ".") {
		out.ImportPath = "ext:std/" + spec
		return out, true
	}

	// Rule 2: intra-repo — check if any go.mod module path is a prefix of spec.
	// Longest prefix wins (for monorepos where one module is nested inside another).
	bestLen := 0
	bestMod := ""
	bestDir := ""
	for modPath, modDir := range manifests.GoModules {
		if spec == modPath || strings.HasPrefix(spec, modPath+"/") {
			if len(modPath) > bestLen {
				bestLen = len(modPath)
				bestMod = modPath
				bestDir = modDir
			}
		}
	}
	if bestLen > 0 {
		// Strip the module path prefix to get the package sub-path.
		remainder := strings.TrimPrefix(spec, bestMod)
		remainder = strings.TrimPrefix(remainder, "/")
		// Reconstruct the relative package directory.
		var pkgDir string
		switch {
		case bestDir == "":
			pkgDir = remainder
		case remainder == "":
			pkgDir = bestDir
		default:
			pkgDir = bestDir + "/" + remainder
		}
		// Validate: the directory must be known in the project.
		// If not present (e.g. generated, deleted, or workspace-external sub-module),
		// treat as external rather than unresolved — Go module paths are canonical.
		if pkgDir == "" || dirSet[pkgDir] {
			out.ImportPath = pkgDir
			return out, true
		}
		// Sub-path not in tree — e.g. importing a generated package not yet built.
		// Fall through to ext: (not unresolved — it is a valid module-path import).
	}

	// Rule 3: external.
	out.ImportPath = "ext:" + spec
	return out, true
}

// resolvePython implements §2.4 Python resolution rules:
//
//   - Relative (`.`/`..` prefix): walk up (dots-1) dirs from the importing file's
//     dir, append `module.replace(".","/")`; probe `<p>.py` then `<p>/__init__.py`.
//     No match → unresolved (broken import candidate).
//   - Absolute: classify as "ext:<top-level>" (simplified; no pyproject.toml walk
//     at F1 — Python root detection is an F2 enhancement).
func resolvePython(e ports.ImportEdge, spec string, fileSet map[string]bool) (ports.ImportEdge, bool) {
	out := e

	// Count leading dots for relative imports.
	dots := 0
	for _, ch := range spec {
		if ch == '.' {
			dots++
		} else {
			break
		}
	}

	if dots == 0 {
		// Absolute import: probe the repo FIRST — a Python repo importing its
		// own top-level package by absolute name (`from scrapy.http import …`)
		// is intra-repo, and stamping it "ext:" erases every cross-subpackage
		// dependency (the scrapy §10 empty-DSM root cause). Same probe ladder
		// as the relative branch: <p>.py then <p>/__init__.py, rooted at the
		// repo root. src-layout roots (src/<pkg>/…) are NOT probed — those
		// repos keep the ext: classification (honest v1 line, no guessing).
		p := strings.ReplaceAll(spec, ".", "/")
		if fileSet[p+".py"] {
			out.ImportPath = p + ".py"
			return out, true
		}
		if fileSet[p+"/__init__.py"] {
			out.ImportPath = p + "/__init__.py"
			return out, true
		}

		// No repo match: ext:<top-level-package>
		top := spec
		if i := strings.Index(spec, "."); i >= 0 {
			top = spec[:i]
		}
		out.ImportPath = "ext:" + top
		return out, true
	}

	// Relative import: walk up (dots-1) directories from the importer's dir.
	importerDir := filepath.ToSlash(filepath.Dir(e.FromFile))
	dir := importerDir
	for i := 1; i < dots; i++ { // dots-1 steps up
		dir = filepath.ToSlash(filepath.Dir(dir))
		if dir == "." || dir == "" {
			dir = ""
			break
		}
	}

	// The module path after the leading dots.
	modPart := strings.TrimLeft(spec, ".")
	modPath := strings.ReplaceAll(modPart, ".", "/")

	var base string
	switch {
	case dir == "" || dir == ".":
		base = modPath
	case modPath == "":
		base = dir
	default:
		base = dir + "/" + modPath
	}

	// Probe <base>.py then <base>/__init__.py.
	if base != "" {
		if fileSet[base+".py"] {
			out.ImportPath = base + ".py"
			return out, true
		}
		if fileSet[base+"/__init__.py"] {
			out.ImportPath = base + "/__init__.py"
			return out, true
		}
	}

	// Looks internal but no match — unresolved (broken import candidate).
	return e, false
}

// jsExtensions is the probe ladder for JS/TS imports that lack an extension.
var jsExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

// resolveJS implements §2.4 TS/JS resolution rules:
//
//  1. Relative (starts with `.` or `/`): join with importer dir, probe extension ladder.
//     No match → unresolved.
//  2. Scoped packages (@scope/pkg): "ext:@scope/pkg"
//  3. Bare specifier: "ext:<name>"
func resolveJS(e ports.ImportEdge, spec string, fileSet map[string]bool) (ports.ImportEdge, bool) {
	out := e

	if !strings.HasPrefix(spec, ".") && !strings.HasPrefix(spec, "/") {
		// External bare specifier or scoped package.
		out.ImportPath = "ext:" + spec
		return out, true
	}

	// Relative: join with importer directory.
	importerDir := filepath.ToSlash(filepath.Dir(e.FromFile))
	joined := filepath.ToSlash(filepath.Join(importerDir, spec))

	// If the path already has a recognised extension, check directly.
	ext := strings.ToLower(filepath.Ext(joined))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		if fileSet[joined] {
			out.ImportPath = joined
			return out, true
		}
		// Exact path not found — try without extension then re-probe.
		joined = strings.TrimSuffix(joined, ext)
	}

	// Probe extension ladder.
	for _, tryExt := range jsExtensions {
		candidate := joined + tryExt
		if fileSet[candidate] {
			out.ImportPath = candidate
			return out, true
		}
	}
	// Probe index files.
	for _, tryExt := range jsExtensions {
		candidate := joined + "/index" + tryExt
		if fileSet[candidate] {
			out.ImportPath = candidate
			return out, true
		}
	}

	// Relative path that probed to nothing — unresolved.
	return e, false
}
