package test

// T11 — arch self-test CI regression (L19.15 §7).
//
// Runs aOa's own internal/ package dep graph through the arch detectors.
// Two invariants are asserted:
//  1. No internal/domain/* package imports internal/adapters/* (hexagonal law).
//  2. Cycle count == 0 (clean dependency graph).
//
// Implementation: parses Go source files with go/parser (stdlib, no CGo, no daemon).
// Only intra-module deps are included; stdlib + third-party are ignored.
// Test files (ending in _test.go) are excluded to avoid test-only import noise.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const modulePath = "github.com/corey/aoa"

// pkgEntry collects metadata for one Go package found during the walk.
type pkgEntry struct {
	importPath string   // e.g. "github.com/corey/aoa/internal/domain/arch"
	file       string   // representative .go file (for G7 source pointer)
	imports    []string // intra-module imports only
}

// buildSelfDepGraph walks the internal/ directory of the module, parses all
// non-test .go files with go/parser (ImportsOnly mode), and returns units +
// deps suitable for the arch detectors.
func buildSelfDepGraph(t *testing.T) ([]arch.UnitFact, []arch.DepFact) {
	t.Helper()

	// Test cwd is test/; module root is one level up.
	root, err := filepath.Abs("../")
	require.NoError(t, err, "cannot locate module root")

	internalDir := filepath.Join(root, "internal")
	_, err = os.Stat(internalDir)
	require.NoError(t, err, "internal/ directory must exist at module root")

	byPath := make(map[string]*pkgEntry)

	err = filepath.WalkDir(internalDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Derive the import path from the file's directory.
		dir := filepath.Dir(path)
		rel, relErr := filepath.Rel(root, dir)
		if relErr != nil {
			return nil
		}
		importPath := modulePath + "/" + filepath.ToSlash(rel)

		if _, exists := byPath[importPath]; !exists {
			byPath[importPath] = &pkgEntry{
				importPath: importPath,
				file:       path,
			}
		}

		// Parse imports only (fast — no type info needed).
		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return nil // skip unparseable file; don't abort the walk
		}
		ast.Inspect(f, func(n ast.Node) bool { return n != nil }) // no-op; keeps import

		for _, imp := range f.Imports {
			spec := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(spec, modulePath+"/") {
				byPath[importPath].imports = append(byPath[importPath].imports, spec)
			}
		}
		return nil
	})
	require.NoError(t, err, "WalkDir over internal/ must succeed")
	require.NotEmpty(t, byPath, "must find at least one Go package under internal/")

	// Convert to arch types. Use import path as unit ID (unique, stable).
	units := make([]arch.UnitFact, 0, len(byPath))
	for _, entry := range byPath {
		label := strings.TrimPrefix(entry.importPath, modulePath+"/")
		units = append(units, arch.UnitFact{
			ID:    entry.importPath,
			Label: label,
			Path:  strings.TrimPrefix(entry.importPath, modulePath+"/"),
			File:  entry.file,
			Line:  1,
		})
	}

	var deps []arch.DepFact
	for _, entry := range byPath {
		for _, imp := range entry.imports {
			// Only include deps where the target package was also walked (i.e. is in internal/).
			if _, ok := byPath[imp]; ok {
				deps = append(deps, arch.DepFact{
					FromUnit: entry.importPath,
					ToUnit:   imp,
					Count:    1,
					File:     entry.file,
					Line:     1,
				})
			}
		}
	}

	return units, deps
}

// TestArchSelftest_HexagonalConstraint asserts that no domain package imports
// an adapters package (T11: aOa's own hexagonal architecture as CI regression).
func TestArchSelftest_HexagonalConstraint(t *testing.T) {
	units, deps := buildSelfDepGraph(t)

	const domainInfix = "/internal/domain/"
	const adaptersInfix = "/internal/adapters/"

	for _, d := range deps {
		fromIsDomain := strings.Contains(d.FromUnit, domainInfix)
		toIsAdapters := strings.Contains(d.ToUnit, adaptersInfix)

		assert.False(t, fromIsDomain && toIsAdapters,
			"T11 hexagonal law violation: domain package %q must not import adapters package %q",
			d.FromUnit, d.ToUnit)
	}

	t.Logf("T11: checked %d units, %d dep edges — no domain→adapters violations", len(units), len(deps))
}

// TestArchSelftest_NoCycles asserts that the cycle count over aOa's own
// internal package dependency graph is zero (T11: clean dependency graph).
func TestArchSelftest_NoCycles(t *testing.T) {
	units, deps := buildSelfDepGraph(t)

	cycleFindings, sccs := arch.DetectCycles("selftest", units, deps)

	assert.Empty(t, sccs,
		"T11: aOa's own internal/ dep graph must have 0 dependency cycles; found %d SCC(s)", len(sccs))
	assert.Empty(t, cycleFindings,
		"T11: DetectCycles must return 0 cycle findings on aOa's own codebase")

	t.Logf("T11: %d packages, %d edges, %d SCCs (must be 0)", len(units), len(deps), len(sccs))
}
