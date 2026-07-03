package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PC3 / ledger T46 — `aoa arch findings --new` honest baseline semantics.
// These cover the pure baseline machinery (write → load → set-difference) that
// backs the CLI gate, without spawning a daemon or calling os.Exit.

func findingsJSON(ids ...string) json.RawMessage {
	type f struct {
		ID       string `json:"id"`
		Severity string `json:"severity"`
	}
	arr := make([]f, 0, len(ids))
	for _, id := range ids {
		arr = append(arr, f{ID: id, Severity: "warn"})
	}
	b, _ := json.Marshal(arr)
	return b
}

func TestPC3_FindingIDs_SortedSet(t *testing.T) {
	ids := findingIDs(findingsJSON("ccc", "aaa", "bbb"))
	assert.Equal(t, []string{"aaa", "bbb", "ccc"}, ids, "IDs must be extracted and sorted")

	assert.Empty(t, findingIDs(json.RawMessage("[]")), "empty array → no IDs")
	assert.Empty(t, findingIDs(json.RawMessage("garbage")), "malformed → no IDs, no panic")
}

func TestPC3_BaselineRoundTrip(t *testing.T) {
	root := t.TempDir()
	scope := "local"

	// No baseline on disk yet.
	base, have, err := loadBaseline(root, scope)
	require.NoError(t, err)
	assert.False(t, have, "no file → no baseline")
	assert.Nil(t, base)

	// Record a baseline of two findings.
	ids := findingIDs(findingsJSON("f1", "f2"))
	require.NoError(t, writeBaseline(root, scope, ids))
	assert.FileExists(t, filepath.Join(root, ".aoa", "arch", "findings-baseline.json"))

	// Re-load: same two IDs are baselined.
	base, have, err = loadBaseline(root, scope)
	require.NoError(t, err)
	require.True(t, have, "file present → baseline recognized")
	assert.True(t, base["f1"] && base["f2"])

	// Re-running --new against the same findings → zero new (exit 0 path).
	assert.Empty(t, newSinceBaseline(base, ids), "baselined findings are not new")

	// A NEW finding (f3) appears → exactly one new (exit 1 path).
	withNew := findingIDs(findingsJSON("f1", "f2", "f3"))
	assert.Equal(t, []string{"f3"}, newSinceBaseline(base, withNew), "f3 is new vs baseline")
}

func TestPC3_Baseline_MergesScopes(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, writeBaseline(root, "local", []string{"a"}))
	require.NoError(t, writeBaseline(root, "other", []string{"b"}))

	// Recording "other" must not drop "local".
	base, have, err := loadBaseline(root, "local")
	require.NoError(t, err)
	require.True(t, have)
	assert.True(t, base["a"], "recording another scope must preserve the first")
}

func TestPC3_NoBaseline_AllNew(t *testing.T) {
	root := t.TempDir()
	// File absent → no baseline; every current finding is treated as new.
	base, have, err := loadBaseline(root, "local")
	require.NoError(t, err)
	require.False(t, have)
	ids := findingIDs(findingsJSON("x", "y"))
	assert.Equal(t, ids, newSinceBaseline(base, ids), "no baseline → all findings new")
}

func TestPC3_Baseline_CorruptFile_Surfaces(t *testing.T) {
	root := t.TempDir()
	path := baselinePath(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o644))
	_, _, err := loadBaseline(root, "local")
	require.Error(t, err, "corrupt baseline must surface an error, not silently pass the gate")
}
