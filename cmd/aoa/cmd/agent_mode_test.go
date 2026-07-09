package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Wave 1a (pipe repair) — agent mode. The usage-truth finding: agents call
// `aoa grep` bare through the Bash tool (stdin = /dev/null, non-TTY) and get
// the GNU-compat degraded format — no peek codes, no [start-end], no domains —
// which breaks the grep→peek workflow. Hosts with pipe stdin hit the stdin
// route and hang. isAgentMode() detects the agent host (explicit AOA_AGENT,
// auto-detect CLAUDECODE) and (a) turns on the semantic agent grammar,
// (b) disables implicit stdin reads. Shim mode is untouched — its GNU
// contract is load-bearing (research consensus F6a).

// clearAgentEnv resets every env var the mode detection reads.
func clearAgentEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AOA_AGENT", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("AOA_SHIM", "")
	t.Setenv("AOA_PEEK", "")
	t.Setenv("AOA_HINTS", "")
}

func TestAgentMode_ExplicitOn(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("AOA_AGENT", "1")
	assert.True(t, isAgentMode(), "AOA_AGENT=1 must enable agent mode")
}

func TestAgentMode_ClaudeCodeAutoDetect(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("CLAUDECODE", "1")
	assert.True(t, isAgentMode(), "CLAUDECODE=1 must auto-enable agent mode")
}

func TestAgentMode_ExplicitOffBeatsAutoDetect(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("AOA_AGENT", "0")
	assert.False(t, isAgentMode(), "AOA_AGENT=0 must override host auto-detect")
}

func TestAgentMode_OffByDefault(t *testing.T) {
	clearAgentEnv(t)
	assert.False(t, isAgentMode(), "no env → not agent mode (humans keep GNU/TTY behavior)")
}

func TestAgentMode_EnablesPeekCodesAndHints(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("CLAUDECODE", "1")
	assert.True(t, showPeekCodes(), "agent mode must surface peek codes — the grep→peek contract")
	assert.True(t, showHints(), "agent mode must surface hints")
}

func TestAgentMode_PeekOptOutRespected(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("AOA_PEEK", "0")
	assert.False(t, showPeekCodes(), "AOA_PEEK=0 must still win inside agent mode")
}

func TestAgentMode_NeverReadsStdinImplicitly(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("CLAUDECODE", "1")
	assert.False(t, shouldReadStdin(), "agent mode must never route to the stdin reader — hang risk on pipe-stdin hosts")
}

func TestShimMode_Unchanged(t *testing.T) {
	clearAgentEnv(t)
	t.Setenv("AOA_SHIM", "1")
	assert.False(t, isAgentMode(), "shim mode is NOT agent mode — its GNU contract stands")
	assert.True(t, showPeekCodes(), "shim keeps its existing peek-code default")
}
