package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextWindowSize_KnownModel(t *testing.T) {
	assert.Equal(t, 200000, ContextWindowSize("claude-opus-4-6"))
	assert.Equal(t, 200000, ContextWindowSize("claude-3-5-sonnet-20241022"))
	assert.Equal(t, 200000, ContextWindowSize("claude-3-opus-20240229"))
}

func TestContextWindowSize_UnknownModel(t *testing.T) {
	assert.Equal(t, 200000, ContextWindowSize("unknown-model"))
	assert.Equal(t, 200000, ContextWindowSize(""))
}
