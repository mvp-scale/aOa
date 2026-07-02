package app

import (
	"encoding/json"
	"testing"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestL18_4_AgentRowHonestyGuard verifies L18.4: Agent-row token estimates
// are replaced with a pending sentinel in the wire format.  The JS then renders
// '—' (em-dash) with a "pending real usage attribution" tooltip instead of the
// known-wrong number (measured at 68×–1022× understated, L20 conformance audit).
func TestL18_4_AgentRowHonestyGuard(t *testing.T) {
	a := newTestApp(t)

	t.Run("Agent row gets AgentTokensPending flag", func(t *testing.T) {
		act := TurnAction{
			Tool:           "Agent",
			Target:         "general-purpose",
			SubagentTokens: 447, // stale regex estimate — known wrong (L20: up to 1022× understated)
		}
		r := a.actionToResult(act)

		assert.True(t, r.AgentTokensPending,
			"Agent rows must set AgentTokensPending=true so the UI renders '—' (L18.4 honesty guard)")
		// SubagentTokens must be preserved — L18.3 will use it for attribution once accurate.
		assert.Equal(t, 447, r.SubagentTokens,
			"SubagentTokens must be preserved for future L18.3 attribution, even when pending flag is set")
	})

	t.Run("non-Agent rows have no pending flag", func(t *testing.T) {
		cases := []struct {
			name string
			act  TurnAction
		}{
			{"Bash", TurnAction{Tool: "Bash", Target: "go test ./...", Tokens: 1200}},
			{"Read", TurnAction{Tool: "Read", Target: "/some/file.go", Tokens: 800}},
			{"Edit", TurnAction{Tool: "Edit", Target: "/some/file.go", Tokens: 50}},
		}
		for _, tc := range cases {
			r := a.actionToResult(tc.act)
			assert.False(t, r.AgentTokensPending,
				"%s row must not set AgentTokensPending", tc.name)
			assert.Equal(t, tc.act.Tokens, r.Tokens,
				"%s Tokens field must be preserved unchanged", tc.name)
		}
	})

	t.Run("pending flag serializes to JSON correctly for Agent", func(t *testing.T) {
		act := TurnAction{
			Tool:               "Agent",
			Target:             "explore",
			SubagentTokens:     63900,
			SubagentToolUses:   20,
			SubagentDurationMs: 37200,
		}
		r := a.actionToResult(act)
		data, err := json.Marshal(r)
		require.NoError(t, err)

		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &m))

		assert.Equal(t, true, m["agent_tokens_pending"],
			"JSON must contain agent_tokens_pending:true for Agent rows")
		// SubagentTokens preserved in JSON so L18.3 can read it when wired.
		assert.EqualValues(t, 63900, m["subagent_tokens"],
			"subagent_tokens must be preserved in JSON for L18.3")
	})

	t.Run("non-Agent JSON omits agent_tokens_pending", func(t *testing.T) {
		act := TurnAction{Tool: "Bash", Target: "npm test", Tokens: 300}
		r := a.actionToResult(act)

		data, err := json.Marshal(r)
		require.NoError(t, err)

		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &m))

		_, hasPending := m["agent_tokens_pending"]
		assert.False(t, hasPending,
			"non-Agent JSON must omit agent_tokens_pending (omitempty, false zero-value)")
		assert.EqualValues(t, 300, m["tokens"],
			"non-Agent token field must be present and correct")
	})

	t.Run("Agent child rows also get pending flag", func(t *testing.T) {
		// Children of an Agent are also Agent-dispatched tool calls — they carry
		// the same inaccuracy; child rows rendered inside the agent subtree must
		// also receive the pending flag.
		child := TurnAction{
			Tool:           "Agent",
			Target:         "beacon",
			SubagentTokens: 1234,
		}
		parent := TurnAction{
			Tool:           "Agent",
			Target:         "general-purpose",
			SubagentTokens: 5000,
			Children:       []TurnAction{child},
		}
		r := a.actionToResult(parent)

		assert.True(t, r.AgentTokensPending, "parent Agent row must have pending flag")
		require.Len(t, r.Children, 1)

		childResult := r.Children[0]
		assert.True(t, childResult.AgentTokensPending,
			"child Agent row must also have pending flag")
		assert.Equal(t, 1234, childResult.SubagentTokens,
			"child SubagentTokens preserved for L18.3")
	})

	// Verifies the socket.TurnActionResult type itself has the field — compile-time guard.
	var _ socket.TurnActionResult
}
