package lockfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackageJSON_PinnedAndRangeVersions(t *testing.T) {
	src := []byte(`{
  "name": "example",
  "dependencies": {
    "lodash": "4.17.21",
    "react": "^18.2.0"
  },
  "devDependencies": {
    "jest": "~29.0.0"
  }
}`)
	comps, err := ParsePackageJSON("package.json", src)
	require.NoError(t, err)
	require.Len(t, comps, 3)

	byName := map[string]Component{}
	for _, c := range comps {
		byName[c.Name] = c
	}

	lodash := byName["lodash"]
	assert.Equal(t, "4.17.21", lodash.Version)
	assert.Equal(t, "direct", lodash.Supplier)
	assert.Equal(t, "js", lodash.Language)
	assert.False(t, lodash.Unpinned, "exact semver is pinned")

	react := byName["react"]
	assert.Equal(t, "^18.2.0", react.Version)
	assert.True(t, react.Unpinned, "caret range is not a pinned version")

	jest := byName["jest"]
	assert.Equal(t, "dev", jest.Supplier)
	assert.True(t, jest.Unpinned, "tilde range is not a pinned version")
}

func TestParsePackageJSON_OptionalAndPeerDependencies(t *testing.T) {
	src := []byte(`{
  "name": "example",
  "optionalDependencies": { "fsevents": "2.3.3" },
  "peerDependencies": { "react": "*" }
}`)
	comps, err := ParsePackageJSON("package.json", src)
	require.NoError(t, err)
	require.Len(t, comps, 2)

	byName := map[string]Component{}
	for _, c := range comps {
		byName[c.Name] = c
	}
	assert.Equal(t, "optional", byName["fsevents"].Supplier)
	assert.Equal(t, "peer", byName["react"].Supplier)
	assert.True(t, byName["react"].Unpinned, "wildcard * is not pinned")
}

func TestParsePackageJSON_NoDependencies(t *testing.T) {
	src := []byte(`{"name": "example", "version": "1.0.0"}`)
	comps, err := ParsePackageJSON("package.json", src)
	require.NoError(t, err)
	assert.Empty(t, comps)
}

func TestParsePackageJSON_InvalidJSON_ReturnsError(t *testing.T) {
	_, err := ParsePackageJSON("package.json", []byte("{not json"))
	assert.Error(t, err)
}

func TestParsePackageJSON_Deterministic(t *testing.T) {
	src := []byte(`{"dependencies": {"b": "1.0.0", "a": "2.0.0"}}`)
	c1, err := ParsePackageJSON("package.json", src)
	require.NoError(t, err)
	c2, err := ParsePackageJSON("package.json", src)
	require.NoError(t, err)
	assert.Equal(t, c1, c2)
	// Sorted by name for byte-stable table rows (T4).
	require.Len(t, c1, 2)
	assert.Equal(t, "a", c1[0].Name)
	assert.Equal(t, "b", c1[1].Name)
}

func TestParsePackageJSON_RealFixture(t *testing.T) {
	comps, err := ParsePackageJSON("npm/aoa/package.json", []byte(realPackageJSONFixture))
	require.NoError(t, err)
	require.NotEmpty(t, comps)
	for _, c := range comps {
		assert.Equal(t, "js", c.Language)
		assert.Equal(t, "optional", c.Supplier)
	}
}

const realPackageJSONFixture = `{
  "name": "@mvpscale/aoa",
  "version": "0.0.0",
  "optionalDependencies": {
    "@mvpscale/aoa-linux-x64": "0.0.0",
    "@mvpscale/aoa-linux-arm64": "0.0.0"
  }
}`
