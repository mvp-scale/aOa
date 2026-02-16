package status

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corey/aoa-go/internal/domain/learner"
	"github.com/corey/aoa-go/internal/ports"
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

	line := GeneratePlain(state, nil)
	assert.Contains(t, line, "150 intents")
	assert.Contains(t, line, "3 domains")
	assert.Contains(t, line, "@authentication")
	assert.Contains(t, line, "@rest_api")
	assert.Contains(t, line, "@database")
	// No autotune stats when nil
	assert.NotContains(t, line, "promoted:")
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

	line := GeneratePlain(state, result)
	assert.Contains(t, line, "200 intents")
	assert.Contains(t, line, "promoted:1")
	assert.Contains(t, line, "demoted:2")
	assert.Contains(t, line, "decayed:8")
	assert.Contains(t, line, "pruned:3")
}

func TestGenerate_EmptyState(t *testing.T) {
	state := &ports.LearnerState{
		DomainMeta: map[string]*ports.DomainMeta{},
	}

	line := GeneratePlain(state, nil)
	assert.Contains(t, line, "0 intents")
	assert.Contains(t, line, "0 domains")
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

	line := GeneratePlain(state, nil)
	// Should show top 3 only
	assert.Contains(t, line, "@a")
	assert.Contains(t, line, "@b")
	assert.Contains(t, line, "@c")
	assert.NotContains(t, line, "@d")
	assert.NotContains(t, line, "@e")
}

func TestGenerate_SkipsDeprecatedDomains(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 50,
		DomainMeta: map[string]*ports.DomainMeta{
			"active_one":   {Hits: 10.0, State: "active"},
			"deprecated_x": {Hits: 20.0, State: "deprecated"},
		},
	}

	line := GeneratePlain(state, nil)
	assert.Contains(t, line, "@active_one")
	assert.NotContains(t, line, "@deprecated_x")
}

func TestGenerate_SkipsZeroHitDomains(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 50,
		DomainMeta: map[string]*ports.DomainMeta{
			"active":  {Hits: 5.0, State: "active"},
			"zerohit": {Hits: 0.0, State: "active"},
		},
	}

	line := GeneratePlain(state, nil)
	assert.Contains(t, line, "@active")
	assert.NotContains(t, line, "@zerohit")
}

func TestGenerate_ColoredVersion(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 42,
		DomainMeta:  map[string]*ports.DomainMeta{},
	}

	line := Generate(state, nil)
	// Should contain ANSI escape codes
	assert.Contains(t, line, "\033[96m")
	assert.Contains(t, line, "42 intents")
}

func TestWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.txt")

	err := Write(path, "⚡ aOa-go │ 100 intents")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "⚡ aOa-go │ 100 intents\n", string(data))
}

func TestWrite_Overwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.txt")

	require.NoError(t, Write(path, "old"))
	require.NoError(t, Write(path, "new"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new\n", string(data))
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
	assert.Equal(t, "@high", top[0])
	assert.Equal(t, "@mid", top[1])
	assert.Equal(t, "@low", top[2])
}

func TestGeneratePlain_PipeDelimited(t *testing.T) {
	state := &ports.LearnerState{
		PromptCount: 50,
		DomainMeta:  map[string]*ports.DomainMeta{},
	}

	line := GeneratePlain(state, nil)
	parts := strings.Split(line, " │ ")
	require.GreaterOrEqual(t, len(parts), 3)
	assert.Equal(t, "⚡ aOa-go", parts[0])
	assert.Equal(t, "50 intents", parts[1])
	assert.Equal(t, "0 domains", parts[2])
}
