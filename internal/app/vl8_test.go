package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// splitCommitAuthorLine / parseGitAuthorshipLog / topAuthor — pure parsing,
// no subprocess.
// ---------------------------------------------------------------------------

func TestSplitCommitAuthorLine_Valid(t *testing.T) {
	hash, author, ok := splitCommitAuthorLine("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa|Ada Lovelace")
	require.True(t, ok)
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", hash)
	assert.Equal(t, "Ada Lovelace", author)
}

func TestSplitCommitAuthorLine_RejectsFilePath(t *testing.T) {
	_, _, ok := splitCommitAuthorLine("internal/app/vl8.go")
	assert.False(t, ok)
}

func TestParseGitAuthorshipLog_CountsPerUnit(t *testing.T) {
	raw := []byte(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa|Alice\n" +
			"internal/domain/arch/model.go\n" +
			"\n" +
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb|Bob\n" +
			"internal/domain/arch/service.go\n" +
			"cccccccccccccccccccccccccccccccccccccccc|Alice\n" +
			"internal/domain/arch/model.go\n")

	counts := parseGitAuthorshipLog(raw)
	require.Contains(t, counts, "u_internal_domain_arch")
	assert.Equal(t, 2, counts["u_internal_domain_arch"]["Alice"])
	assert.Equal(t, 1, counts["u_internal_domain_arch"]["Bob"])
}

func TestTopAuthor_PicksHighestCount(t *testing.T) {
	assert.Equal(t, "Alice", topAuthor(map[string]int{"Alice": 3, "Bob": 1}))
}

func TestTopAuthor_TiesBreakAlphabetically(t *testing.T) {
	assert.Equal(t, "Alice", topAuthor(map[string]int{"Bob": 2, "Alice": 2}))
}

// ---------------------------------------------------------------------------
// buildOwnershipEntries — integration with real (bounded) git + CODEOWNERS.
// ---------------------------------------------------------------------------

func TestBuildOwnershipEntries_CodeownersDeclaredWins(t *testing.T) {
	root := initTestGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "CODEOWNERS"), []byte("/pkg/hot/ @alice\n"), 0o644))

	units := []arch.UnitFact{
		{ID: "u_pkg_hot", Path: "pkg/hot", File: "pkg/hot/hot.go", Line: 1},
	}
	entries := buildOwnershipEntries(root, units)
	require.Len(t, entries, 1)
	assert.Equal(t, "pkg/hot", entries[0].Path)
	assert.Equal(t, []string{"@alice"}, entries[0].Owners)
	assert.Equal(t, "declared", entries[0].Provenance)
	assert.Equal(t, "CODEOWNERS", entries[0].File)
}

func TestBuildOwnershipEntries_FallsBackToGitAuthorshipWhenNoCodeowners(t *testing.T) {
	root := initTestGitRepo(t) // no CODEOWNERS written
	units := []arch.UnitFact{
		{ID: "u_pkg_hot", Path: "pkg/hot", File: "pkg/hot/hot.go", Line: 1},
		{ID: "u_pkg_cold", Path: "pkg/cold", File: "pkg/cold/cold.go", Line: 1},
	}
	entries := buildOwnershipEntries(root, units)
	require.Len(t, entries, 2)

	byPath := make(map[string]arch.OwnershipEntry, len(entries))
	for _, e := range entries {
		byPath[e.Path] = e
	}
	hot, ok := byPath["pkg/hot"]
	require.True(t, ok)
	assert.Equal(t, "derived", hot.Provenance)
	assert.Equal(t, []string{"Test"}, hot.Owners) // initTestGitRepo commits as user.name "Test"
	assert.Equal(t, "pkg/hot/hot.go", hot.File)
}

func TestBuildOwnershipEntries_NoCodeownersNoGit_HonestNil(t *testing.T) {
	root := t.TempDir() // no `git init`, no CODEOWNERS
	units := []arch.UnitFact{{ID: "u_pkg_hot", Path: "pkg/hot", File: "pkg/hot/hot.go", Line: 1}}
	entries := buildOwnershipEntries(root, units)
	assert.Empty(t, entries)
}

func TestBuildOwnershipEntries_UnknownUnit_SkippedNotFabricated(t *testing.T) {
	root := initTestGitRepo(t)
	// A unit with no CODEOWNERS match and no git history under its path.
	units := []arch.UnitFact{{ID: "u_pkg_unknown", Path: "pkg/unknown", File: "pkg/unknown/x.go", Line: 1}}
	entries := buildOwnershipEntries(root, units)
	assert.Empty(t, entries, "a unit with neither CODEOWNERS nor git-authorship signal must not produce a fabricated row")
}
