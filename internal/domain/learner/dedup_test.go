package learner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// L-04: Cohit Dedup
// =============================================================================

func TestDedup_MergesCohits(t *testing.T) {
	// "login" appears in "auth" (80) and "session" (30) → total 110 >= 100.
	// Winner: "auth" (80). Loser "login:session" removed.
	m := map[string]uint32{
		"login:auth":    80,
		"login:session": 30,
		"other:x":       5,
	}
	runDedup(m)
	assert.Equal(t, uint32(80), m["login:auth"])
	assert.Zero(t, m["login:session"], "loser should be removed")
	assert.Equal(t, uint32(5), m["other:x"], "unrelated entry preserved")
}

func TestDedup_OutputMatchesPythonLua(t *testing.T) {
	// Two entities, both with total >= 100.
	m := map[string]uint32{
		"kw1:termA": 60,
		"kw1:termB": 50, // total 110, termA wins
		"kw2:termC": 70,
		"kw2:termD": 40, // total 110, termC wins
	}
	runDedup(m)
	assert.Equal(t, uint32(60), m["kw1:termA"])
	assert.Zero(t, m["kw1:termB"])
	assert.Equal(t, uint32(70), m["kw2:termC"])
	assert.Zero(t, m["kw2:termD"])
}

func TestDedup_EmptyInput(t *testing.T) {
	m := map[string]uint32{}
	runDedup(m) // Should not panic
	assert.Empty(t, m)
}

func TestDedup_SingleEntry(t *testing.T) {
	m := map[string]uint32{"login:auth": 200}
	runDedup(m) // Single container per entity — no dedup
	assert.Equal(t, uint32(200), m["login:auth"])
}

func TestDedup_PreservesHighestHit(t *testing.T) {
	m := map[string]uint32{
		"kw:low":  30,
		"kw:high": 80, // total 110, high wins
	}
	runDedup(m)
	assert.Equal(t, uint32(80), m["kw:high"])
	assert.Zero(t, m["kw:low"])
}

func TestDedup_BelowThreshold_NoAction(t *testing.T) {
	m := map[string]uint32{
		"kw:a": 40,
		"kw:b": 50, // total 90 < 100 — no dedup
	}
	runDedup(m)
	assert.Equal(t, uint32(40), m["kw:a"])
	assert.Equal(t, uint32(50), m["kw:b"])
}

func TestDedup_TieBreaksAlphabetically(t *testing.T) {
	m := map[string]uint32{
		"kw:beta":  55,
		"kw:alpha": 55, // tie, alpha wins alphabetically
	}
	runDedup(m)
	assert.Equal(t, uint32(55), m["kw:alpha"], "alpha should win tie")
	assert.Zero(t, m["kw:beta"], "beta should be removed")
}
