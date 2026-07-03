package facts

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Go resolver tests ─────────────────────────────────────────────────────────

func TestResolveGo_Stdlib(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/foo/bar": ""}}
	fileSet := map[string]bool{"main.go": true, "internal/app/app.go": true}

	edges := []ports.ImportEdge{
		{FromFile: "main.go", ImportPath: "fmt", StartLine: 3},
		{FromFile: "main.go", ImportPath: "net/http", StartLine: 4},
		{FromFile: "main.go", ImportPath: "path/filepath", StartLine: 5},
		{FromFile: "main.go", ImportPath: "encoding/json", StartLine: 6},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 4, len(result.Resolved), "all stdlib imports resolve")
	assert.Empty(t, result.Unresolved)
	for _, e := range result.Resolved {
		assert.True(t, e.ImportPath == "ext:std/fmt" ||
			e.ImportPath == "ext:std/net/http" ||
			e.ImportPath == "ext:std/path/filepath" ||
			e.ImportPath == "ext:std/encoding/json",
			"stdlib import %q must be ext:std/...", e.ImportPath)
	}
}

func TestResolveGo_External(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/foo/bar": ""}}
	fileSet := map[string]bool{"main.go": true}

	edges := []ports.ImportEdge{
		{FromFile: "main.go", ImportPath: "github.com/spf13/cobra", StartLine: 3},
		{FromFile: "main.go", ImportPath: "go.etcd.io/bbolt", StartLine: 4},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 2, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	paths := make([]string, 0, len(result.Resolved))
	for _, e := range result.Resolved {
		paths = append(paths, e.ImportPath)
	}
	assert.Contains(t, paths, "ext:github.com/spf13/cobra")
	assert.Contains(t, paths, "ext:go.etcd.io/bbolt")
}

func TestResolveGo_IntraRepo(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/foo/bar": ""}}
	fileSet := map[string]bool{
		"main.go":                  true,
		"internal/app/app.go":      true,
		"internal/ports/ports.go":  true,
		"internal/domain/model.go": true,
	}

	edges := []ports.ImportEdge{
		{FromFile: "main.go", ImportPath: "github.com/foo/bar/internal/app", StartLine: 3},
		{FromFile: "main.go", ImportPath: "github.com/foo/bar/internal/ports", StartLine: 4},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 2, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	paths := make([]string, 0, len(result.Resolved))
	for _, e := range result.Resolved {
		paths = append(paths, e.ImportPath)
	}
	assert.Contains(t, paths, "internal/app")
	assert.Contains(t, paths, "internal/ports")
}

func TestResolveGo_MonorepoLongestPrefixWins(t *testing.T) {
	// Two nested modules: outer and inner. Inner should match its own module path.
	manifests := Manifests{GoModules: map[string]string{
		"github.com/foo/bar":         "",
		"github.com/foo/bar/backend": "backend",
	}}
	fileSet := map[string]bool{
		"main.go":              true,
		"backend/handler.go":   true,
		"backend/api/route.go": true,
	}

	edges := []ports.ImportEdge{
		// This should match the longer "github.com/foo/bar/backend" prefix.
		{FromFile: "main.go", ImportPath: "github.com/foo/bar/backend/api", StartLine: 3},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 1, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	assert.Equal(t, "backend/api", result.Resolved[0].ImportPath,
		"longest prefix wins: backend/api should resolve via github.com/foo/bar/backend")
}

func TestResolveGo_EmptyEdges(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	result := Resolve(nil, nil, manifests)
	assert.Empty(t, result.Resolved)
	assert.Empty(t, result.Unresolved)
}

// ── Python resolver tests ────────────────────────────────────────────────────

func TestResolvePython_Absolute(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{"src/app.py": true}

	edges := []ports.ImportEdge{
		{FromFile: "src/app.py", ImportPath: "numpy", StartLine: 1},
		{FromFile: "src/app.py", ImportPath: "os.path", StartLine: 2},
		{FromFile: "src/app.py", ImportPath: "requests", StartLine: 3},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 3, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	paths := make([]string, 0, len(result.Resolved))
	for _, e := range result.Resolved {
		paths = append(paths, e.ImportPath)
	}
	assert.Contains(t, paths, "ext:numpy")
	assert.Contains(t, paths, "ext:os")      // top-level of os.path
	assert.Contains(t, paths, "ext:requests")
}

func TestResolvePython_RelativeSingle(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{
		"pkg/app.py":    true,
		"pkg/models.py": true,
		"pkg/utils.py":  true,
	}

	edges := []ports.ImportEdge{
		// from .models import User — single dot
		{FromFile: "pkg/app.py", ImportPath: ".models", StartLine: 3},
		// from .utils import helper
		{FromFile: "pkg/app.py", ImportPath: ".utils", StartLine: 4},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 2, len(result.Resolved), "relative single-dot imports probe correctly")
	assert.Empty(t, result.Unresolved)
	paths := make([]string, 0, len(result.Resolved))
	for _, e := range result.Resolved {
		paths = append(paths, e.ImportPath)
	}
	assert.Contains(t, paths, "pkg/models.py")
	assert.Contains(t, paths, "pkg/utils.py")
}

func TestResolvePython_RelativeDouble(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{
		"pkg/sub/app.py": true,
		"pkg/models.py":  true,
	}

	edges := []ports.ImportEdge{
		// from ..models import User — double dot walks up to pkg/
		{FromFile: "pkg/sub/app.py", ImportPath: "..models", StartLine: 5},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 1, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	assert.Equal(t, "pkg/models.py", result.Resolved[0].ImportPath)
}

func TestResolvePython_RelativeUnresolved(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{"pkg/app.py": true}

	edges := []ports.ImportEdge{
		// from .missing import Something — no matching file
		{FromFile: "pkg/app.py", ImportPath: ".missing", StartLine: 3},
	}
	result := Resolve(edges, fileSet, manifests)

	assert.Empty(t, result.Resolved)
	require.Equal(t, 1, len(result.Unresolved), "unresolved relative import retained as finding")
	assert.Equal(t, ".missing", result.Unresolved[0].ImportPath)
}

// ── JS/TS resolver tests ─────────────────────────────────────────────────────

func TestResolveJS_BareSpecifier(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{"src/index.ts": true}

	edges := []ports.ImportEdge{
		{FromFile: "src/index.ts", ImportPath: "react", StartLine: 1},
		{FromFile: "src/index.ts", ImportPath: "@scope/package", StartLine: 2},
		{FromFile: "src/index.ts", ImportPath: "lodash/merge", StartLine: 3},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 3, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	paths := make([]string, 0, len(result.Resolved))
	for _, e := range result.Resolved {
		paths = append(paths, e.ImportPath)
	}
	assert.Contains(t, paths, "ext:react")
	assert.Contains(t, paths, "ext:@scope/package")
	assert.Contains(t, paths, "ext:lodash/merge")
}

func TestResolveJS_RelativeFound(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{
		"src/index.ts":      true,
		"src/utils.ts":      true,
		"src/components.tsx": true,
	}

	edges := []ports.ImportEdge{
		{FromFile: "src/index.ts", ImportPath: "./utils", StartLine: 2},
		{FromFile: "src/index.ts", ImportPath: "./components", StartLine: 3},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 2, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	paths := make([]string, 0, len(result.Resolved))
	for _, e := range result.Resolved {
		paths = append(paths, e.ImportPath)
	}
	assert.Contains(t, paths, "src/utils.ts")
	assert.Contains(t, paths, "src/components.tsx")
}

func TestResolveJS_RelativeIndex(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{
		"src/app.ts":            true,
		"src/lib/index.ts":      true,
	}

	edges := []ports.ImportEdge{
		// import from './lib' should probe src/lib/index.ts
		{FromFile: "src/app.ts", ImportPath: "./lib", StartLine: 1},
	}
	result := Resolve(edges, fileSet, manifests)

	require.Equal(t, 1, len(result.Resolved))
	assert.Empty(t, result.Unresolved)
	assert.Equal(t, "src/lib/index.ts", result.Resolved[0].ImportPath)
}

func TestResolveJS_RelativeUnresolved(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{}}
	fileSet := map[string]bool{"src/index.ts": true}

	edges := []ports.ImportEdge{
		{FromFile: "src/index.ts", ImportPath: "./missing", StartLine: 4},
	}
	result := Resolve(edges, fileSet, manifests)

	assert.Empty(t, result.Resolved)
	require.Equal(t, 1, len(result.Unresolved))
	assert.Equal(t, "./missing", result.Unresolved[0].ImportPath)
}

// ── Provenance preservation test ─────────────────────────────────────────────

func TestResolve_PreservesProvenance(t *testing.T) {
	// FromFile and StartLine must survive resolution unchanged (G7).
	manifests := Manifests{GoModules: map[string]string{"github.com/foo/bar": ""}}
	fileSet := map[string]bool{"main.go": true}

	edge := ports.ImportEdge{FromFile: "main.go", ImportPath: "fmt", StartLine: 42}
	result := Resolve([]ports.ImportEdge{edge}, fileSet, manifests)

	require.Equal(t, 1, len(result.Resolved))
	assert.Equal(t, "main.go", result.Resolved[0].FromFile, "FromFile preserved")
	assert.Equal(t, uint32(42), result.Resolved[0].StartLine, "StartLine preserved")
}

// ── ReadManifests smoke test ─────────────────────────────────────────────────

func TestReadManifests_GoMod(t *testing.T) {
	// Use the actual aOa-f1 repo which has a go.mod at the root.
	// This is an integration smoke test — not pinned to a specific corpus.
	m := ReadManifests("/home/corey/aOa-f1")
	if _, ok := m.GoModules["github.com/corey/aoa"]; !ok {
		// Skip if not running in the expected environment.
		t.Skip("aOa-f1 go.mod not found — skipping ReadManifests integration test")
	}
	dir := m.GoModules["github.com/corey/aoa"]
	assert.Equal(t, "", dir, "root module dir should be empty string (repo root)")
}
