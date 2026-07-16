package codeowners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRead_NoFileAnywhere_HonestNilNoError(t *testing.T) {
	root := t.TempDir()
	rules, err := Read(root)
	require.NoError(t, err)
	assert.Nil(t, rules)
}

func TestRead_RootCODEOWNERS_Parsed(t *testing.T) {
	root := t.TempDir()
	content := "# comment\n\n/internal/app/ @alice @bob\n*.go @carol\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "CODEOWNERS"), []byte(content), 0o644))

	rules, err := Read(root)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "/internal/app/", rules[0].Pattern)
	assert.Equal(t, []string{"@alice", "@bob"}, rules[0].Owners)
	assert.Equal(t, "CODEOWNERS", rules[0].File)
	assert.Equal(t, uint32(3), rules[0].Line)
}

func TestRead_GithubDirCODEOWNERS_ProbedWhenRootMissing(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".github"), 0o755))
	content := "/pkg/ @dave\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, ".github", "CODEOWNERS"), []byte(content), 0o644))

	rules, err := Read(root)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, filepath.Join(".github", "CODEOWNERS"), rules[0].File)
}

func TestRead_RootTakesPrecedenceOverGithubDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "CODEOWNERS"), []byte("/a/ @root-owner\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".github"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".github", "CODEOWNERS"), []byte("/b/ @gh-owner\n"), 0o644))

	rules, err := Read(root)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "/a/", rules[0].Pattern)
}

func TestRead_SkipsPatternWithNoOwners(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "CODEOWNERS"), []byte("/nobody/\n/has/ @x\n"), 0o644))

	rules, err := Read(root)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "/has/", rules[0].Pattern)
}

func TestMatch_DirectoryPrefix(t *testing.T) {
	rules := []Rule{{Pattern: "/internal/app/", Owners: []string{"@alice"}}}
	owners, _, ok := Match(rules, "internal/app")
	require.True(t, ok)
	assert.Equal(t, []string{"@alice"}, owners)

	owners, _, ok = Match(rules, "internal/app/sub")
	require.True(t, ok)
	assert.Equal(t, []string{"@alice"}, owners)
}

func TestMatch_NoMatch_ReturnsFalse(t *testing.T) {
	rules := []Rule{{Pattern: "/internal/app/", Owners: []string{"@alice"}}}
	_, _, ok := Match(rules, "internal/other")
	assert.False(t, ok)
}

func TestMatch_LastRuleWins(t *testing.T) {
	rules := []Rule{
		{Pattern: "/internal/", Owners: []string{"@team"}},
		{Pattern: "/internal/app/", Owners: []string{"@alice"}},
	}
	owners, _, ok := Match(rules, "internal/app")
	require.True(t, ok)
	assert.Equal(t, []string{"@alice"}, owners)
}

func TestMatch_Wildcard_MatchesEverything(t *testing.T) {
	rules := []Rule{{Pattern: "*", Owners: []string{"@fallback"}}}
	owners, _, ok := Match(rules, "internal/app")
	require.True(t, ok)
	assert.Equal(t, []string{"@fallback"}, owners)
}

func TestMatch_RootPattern_MatchesOnlyRootUnit(t *testing.T) {
	rules := []Rule{{Pattern: "/", Owners: []string{"@root-owner"}}}
	owners, _, ok := Match(rules, "root")
	require.True(t, ok)
	assert.Equal(t, []string{"@root-owner"}, owners)

	_, _, ok = Match(rules, "internal/app")
	assert.False(t, ok)
}
