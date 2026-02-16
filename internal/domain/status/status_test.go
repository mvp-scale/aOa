package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/domain/learner"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate_Basic(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 150,
		DomainMeta: map[string]*ports.DomainMeta{
			"authentication": {Hits: 10.0, State: "active", Tier: "core"},
			"rest_api":       {Hits: 8.0, State: "active", Tier: "core"},
			"database":       {Hits: 5.0, State: "active", Tier: "core"},
		},
	}

	data := Generate(state, nil)
	assert.Equal(t, uint32(150), data.Intents)
	assert.Equal(t, 3, data.Domains)
	assert.Contains(t, data.TopDomains, "authentication")
	assert.Contains(t, data.TopDomains, "rest_api")
	assert.Contains(t, data.TopDomains, "database")
	assert.Nil(t, data.Autotune)
}

func TestGenerate_WithAutotune(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 200,
		DomainMeta: map[string]*ports.DomainMeta{
			"auth": {Hits: 9.0, State: "active", Tier: "core"},
		},
	}
	result := &learner.AutotuneResult{
		Promoted: 1,
		Demoted:  2,
		Decayed:  8,
		Pruned:   3,
	}

	data := Generate(state, result)
	assert.Equal(t, uint32(200), data.Intents)
	require.NotNil(t, data.Autotune)
	assert.Equal(t, 1, data.Autotune.Promoted)
	assert.Equal(t, 2, data.Autotune.Demoted)
	assert.Equal(t, 8, data.Autotune.Decayed)
	assert.Equal(t, 3, data.Autotune.Pruned)
}

func TestGenerate_EmptyState(t *testing.T) {
	state := &ports.LearnerState{
		DomainMeta: map[string]*ports.DomainMeta{},
	}

	data := Generate(state, nil)
	assert.Equal(t, uint32(0), data.Intents)
	assert.Equal(t, 0, data.Domains)
	assert.Empty(t, data.TopDomains)
}

func TestGenerate_TopDomainsLimitedTo3(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 100,
		DomainMeta: map[string]*ports.DomainMeta{
			"a": {Hits: 10.0, State: "active"},
			"b": {Hits: 9.0, State: "active"},
			"c": {Hits: 8.0, State: "active"},
			"d": {Hits: 7.0, State: "active"},
			"e": {Hits: 6.0, State: "active"},
		},
	}

	data := Generate(state, nil)
	require.Len(t, data.TopDomains, 3)
	assert.Equal(t, "a", data.TopDomains[0])
	assert.Equal(t, "b", data.TopDomains[1])
	assert.Equal(t, "c", data.TopDomains[2])
}

func TestGenerate_SkipsDeprecatedDomains(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 50,
		DomainMeta: map[string]*ports.DomainMeta{
			"active_one":   {Hits: 10.0, State: "active"},
			"deprecated_x": {Hits: 20.0, State: "deprecated"},
		},
	}

	data := Generate(state, nil)
	require.Len(t, data.TopDomains, 1)
	assert.Equal(t, "active_one", data.TopDomains[0])
}

func TestGenerate_SkipsZeroHitDomains(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 50,
		DomainMeta: map[string]*ports.DomainMeta{
			"active":  {Hits: 5.0, State: "active"},
			"zerohit": {Hits: 0.0, State: "active"},
		},
	}

	data := Generate(state, nil)
	require.Len(t, data.TopDomains, 1)
	assert.Equal(t, "active", data.TopDomains[0])
}

func TestWriteJSON_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")

	data := &StatusData{
		Intents:    100,
		Domains:    3,
		TopDomains: []string{"auth", "api"},
	}
	err := WriteJSON(path, data)
	require.NoError(t, err)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded StatusData
	require.NoError(t, json.Unmarshal(raw, &loaded))
	assert.Equal(t, uint32(100), loaded.Intents)
	assert.Equal(t, 3, loaded.Domains)
	assert.Equal(t, []string{"auth", "api"}, loaded.TopDomains)
}

func TestWriteJSON_Overwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")

	require.NoError(t, WriteJSON(path, &StatusData{Intents: 1}))
	require.NoError(t, WriteJSON(path, &StatusData{Intents: 2}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded StatusData
	require.NoError(t, json.Unmarshal(raw, &loaded))
	assert.Equal(t, uint32(2), loaded.Intents)
}

func TestTopDomains_SortedByHits(t *testing.T) {
	state := &ports.LearnerState{
		DomainMeta: map[string]*ports.DomainMeta{
			"low":  {Hits: 1.0, State: "active"},
			"high": {Hits: 10.0, State: "active"},
			"mid":  {Hits: 5.0, State: "active"},
		},
	}

	top := topDomains(state, 3)
	require.Len(t, top, 3)
	assert.Equal(t, "high", top[0])
	assert.Equal(t, "mid", top[1])
	assert.Equal(t, "low", top[2])
}

func TestGenerate_JSONRoundtrip(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 50,
		DomainMeta:  map[string]*ports.DomainMeta{},
	}

	data := Generate(state, nil)
	b, err := json.Marshal(data)
	require.NoError(t, err)

	var loaded StatusData
	require.NoError(t, json.Unmarshal(b, &loaded))
	assert.Equal(t, uint32(50), loaded.Intents)
	assert.Equal(t, 0, loaded.Domains)
}
