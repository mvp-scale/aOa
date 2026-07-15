package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// isGitCommitHash / parseGitChurnLog — pure parsing, no subprocess.
// ---------------------------------------------------------------------------

func TestIsGitCommitHash_ValidHash(t *testing.T) {
	assert.True(t, isGitCommitHash("0123456789abcdef0123456789abcdef01234567"))
}

func TestIsGitCommitHash_RejectsFilePaths(t *testing.T) {
	assert.False(t, isGitCommitHash("internal/app/vl2.go"))
	assert.False(t, isGitCommitHash(""))
	assert.False(t, isGitCommitHash("ABCDEF0123456789abcdef0123456789abcdef01")) // uppercase not %H's shape
}

func TestParseGitChurnLog_GroupsByUnitDirectory(t *testing.T) {
	raw := []byte(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n" +
			"internal/domain/arch/model.go\n" +
			"internal/domain/arch/service.go\n" +
			"\n" +
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n" +
			"internal/domain/arch/model.go\n" +
			"internal/app/arch.go\n")

	files, commits := parseGitChurnLog(raw)

	require.Contains(t, files, "u_internal_domain_arch")
	assert.Len(t, files["u_internal_domain_arch"], 2, "model.go + service.go, deduped across commits")
	assert.Len(t, commits["u_internal_domain_arch"], 2, "both commits touched this unit")

	require.Contains(t, files, "u_internal_app")
	assert.Len(t, files["u_internal_app"], 1)
	assert.Len(t, commits["u_internal_app"], 1)
}

func TestParseGitChurnLog_RootFilesUseRootUnit(t *testing.T) {
	raw := []byte("cccccccccccccccccccccccccccccccccccccccc\nREADME.md\n")
	files, _ := parseGitChurnLog(raw)
	assert.Contains(t, files, "u_root")
}

func TestParseGitChurnLog_EmptyInput_EmptyMaps(t *testing.T) {
	files, commits := parseGitChurnLog(nil)
	assert.Empty(t, files)
	assert.Empty(t, commits)
}

// ---------------------------------------------------------------------------
// buildComplexity — indexed symbol count per unit.
// ---------------------------------------------------------------------------

func TestBuildComplexity_NilIndex_EmptyNotNil(t *testing.T) {
	complexity := buildComplexity(nil)
	assert.NotNil(t, complexity)
	assert.Empty(t, complexity)
}

func TestBuildComplexity_CountsSymbolsPerUnit(t *testing.T) {
	idx := &ports.Index{
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "internal/domain/arch/model.go"},
			2: {Path: "internal/domain/arch/service.go"},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 1}: {Name: "A"},
			{FileID: 1, Line: 2}: {Name: "B"},
			{FileID: 2, Line: 1}: {Name: "C"},
		},
	}
	complexity := buildComplexity(idx)
	assert.Equal(t, 3, complexity["u_internal_domain_arch"])
}

// ---------------------------------------------------------------------------
// buildChurnEntries — integration with a real (bounded) git repo.
// ---------------------------------------------------------------------------

// initTestGitRepo creates a git repo at t.TempDir() with two commits touching
// distinct units, and returns the root path.
func initTestGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")

	writeAndCommit(t, root, "pkg/hot/hot.go", "package hot\n", "first: add hot")
	writeAndCommit(t, root, "pkg/cold/cold.go", "package cold\n", "first: add cold")
	writeAndCommit(t, root, "pkg/hot/hot.go", "package hot\n\nfunc More() {}\n", "second: touch hot again")
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
}

func writeAndCommit(t *testing.T, root, relPath, content, msg string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	runGit(t, root, "add", relPath)
	runGit(t, root, "commit", "-q", "-m", msg)
}

func TestBuildChurnEntries_JoinsChurnWithComplexity(t *testing.T) {
	root := initTestGitRepo(t)
	units := []arch.UnitFact{
		{ID: "u_pkg_hot", Label: "hot", Path: "pkg/hot", File: "pkg/hot/hot.go", Line: 1},
		{ID: "u_pkg_cold", Label: "cold", Path: "pkg/cold", File: "pkg/cold/cold.go", Line: 1},
	}
	idx := &ports.Index{
		Files: map[uint32]*ports.FileMeta{1: {Path: "pkg/hot/hot.go"}},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 1}: {Name: "More"},
		},
	}

	entries := buildChurnEntries(root, units, idx)
	require.NotEmpty(t, entries)

	byPath := make(map[string]arch.ChurnEntry, len(entries))
	for _, e := range entries {
		byPath[e.Path] = e
	}

	hot, ok := byPath["pkg/hot"]
	require.True(t, ok, "pkg/hot must appear — it changed twice")
	assert.Equal(t, 2, hot.Commits, "hot.go was committed in two separate commits")
	assert.Equal(t, 1, hot.ChangedFiles, "same file both times — one distinct changed file")
	assert.Equal(t, 1, hot.Complexity, "one indexed symbol in hot.go")
	assert.Equal(t, hot.ChangedFiles*hot.Complexity, hot.Risk)

	cold, ok := byPath["pkg/cold"]
	require.True(t, ok, "pkg/cold must appear — it changed once")
	assert.Equal(t, 1, cold.Commits)
	assert.Equal(t, 0, cold.Complexity, "cold.go carries no indexed symbols in this fixture")
	assert.Equal(t, 0, cold.Risk)
}

func TestBuildChurnEntries_UnknownUnit_SkippedNotFabricated(t *testing.T) {
	root := initTestGitRepo(t)
	// No units supplied at all — churn exists in git history but has no
	// current fact-set match, so nothing should be fabricated into a row.
	entries := buildChurnEntries(root, nil, nil)
	assert.Empty(t, entries, "churn with no matching unit fact must not produce a row")
}

func TestBuildChurnEntries_NotAGitRepo_HonestNil(t *testing.T) {
	root := t.TempDir() // no `git init`
	entries := buildChurnEntries(root, []arch.UnitFact{{ID: "u_pkg_hot", Path: "pkg/hot"}}, nil)
	assert.Nil(t, entries)
}
