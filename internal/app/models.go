package app

// modelWindows maps known Claude model identifiers to their context window sizes (in tokens).
var modelWindows = map[string]int{
	// Claude 4 family
	"claude-opus-4-6":           200000,
	"claude-opus-4-0-20250514":  200000,
	"claude-sonnet-4-0-20250514": 200000,
	"claude-sonnet-4-6":         200000,

	// Claude 3.5 family
	"claude-3-5-sonnet-20241022": 200000,
	"claude-3-5-sonnet-20240620": 200000,
	"claude-3-5-haiku-20241022":  200000,

	// Claude 3 family
	"claude-3-opus-20240229":   200000,
	"claude-3-sonnet-20240229": 200000,
	"claude-3-haiku-20240307":  200000,
}

const defaultContextWindow = 200000

// ContextWindowSize returns the context window size for the given model.
// Returns 200k as default for unknown models.
func ContextWindowSize(model string) int {
	if size, ok := modelWindows[model]; ok {
		return size
	}
	return defaultContextWindow
}
