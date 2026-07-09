package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wave 1a (pipe repair) — guidance rewrite. Transcript mining proved agents
// obeyed the old guidance and the graph still never arrived: the Claude Code
// harness injects a shell function grep() that beats PATH, so the flagship
// bare-`grep` example returns 0 bytes. Guidance must teach `aoa grep` /
// `aoa egrep` explicitly, and the broken `arch facts` hop (findings emit unit
// IDs facts cannot resolve) must be pulled until repaired.

func TestGuidance_TeachesAoaGrepExplicitly(t *testing.T) {
	g := aOaGuidance

	assert.Contains(t, g, "aoa grep",
		"guidance must teach the aoa-prefixed command — bare grep is intercepted by the harness")
	assert.Contains(t, g, "$ aoa grep",
		"the flagship example must be aoa-prefixed (the bare `$ grep` example returns 0 bytes live)")
	assert.NotContains(t, g, "\n$ grep ",
		"no example may rely on bare-grep PATH interception")
	assert.NotContains(t, g, "through Bash `grep`/`egrep`",
		"the old bare-grep doctrine sentence must be gone")
}

func TestGuidance_NoBrokenFactsHop(t *testing.T) {
	assert.NotContains(t, aOaGuidance, "arch facts",
		"the arch-facts hop is broken live (findings unit IDs don't resolve) — pulled from guidance until repaired")
}

func TestGuidance_DocumentsAgentModeKnob(t *testing.T) {
	g := aOaGuidance
	assert.Contains(t, g, "AOA_AGENT=1",
		"non-Claude hosts need the explicit agent-mode knob documented")
}

// TestRefreshGuidance_ReplacesOldBlockKeepsUserContent — ruling R-4: every
// previously-init'd repo teaches the dead bare-grep path; `aoa init
// --refresh-guidance` must idempotently replace the block without touching
// user content and without running a reindex.
func TestRefreshGuidance_ReplacesOldBlockKeepsUserContent(t *testing.T) {
	root := t.TempDir()
	oldBlock := aOaGuidanceSentinel + "\nOLD STALE GUIDANCE teaching bare grep\n" + aOaGuidanceEnd
	userContent := "# My Project\n\nUser rules that must survive.\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"),
		[]byte(oldBlock+"\n"+userContent), 0644))

	require.NoError(t, runRefreshGuidance(root))

	data, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	require.NoError(t, err)
	s := string(data)

	assert.NotContains(t, s, "OLD STALE GUIDANCE", "old block must be replaced")
	assert.Contains(t, s, "$ aoa grep", "new guidance must be present")
	assert.Contains(t, s, "User rules that must survive.", "user content untouched")
	assert.Equal(t, 1, strings.Count(s, aOaGuidanceSentinel), "exactly one guidance block")

	// Idempotent: second run changes nothing.
	require.NoError(t, runRefreshGuidance(root))
	data2, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	assert.Equal(t, s, string(data2), "refresh must be idempotent")
}
