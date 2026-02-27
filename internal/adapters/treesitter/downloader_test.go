package treesitter

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlatformString(t *testing.T) {
	p := PlatformString()
	assert.Contains(t, p, runtime.GOOS)
	assert.Contains(t, p, runtime.GOARCH)
	assert.Equal(t, runtime.GOOS+"-"+runtime.GOARCH, p)
}

func TestGlobalGrammarDir(t *testing.T) {
	dir := GlobalGrammarDir()
	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, ".aoa")
	assert.Contains(t, dir, "grammars")
}

func TestExtensionToLanguage(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".py", "python"},
		{".go", "go"},
		{".js", "javascript"},
		{".ts", "typescript"},
		{".tsx", "tsx"},
		{".rs", "rust"},
		{".java", "java"},
		{".cpp", "cpp"},
		{".sh", "bash"},
		{".yaml", "yaml"},
		{".unknown", ""},
		{"", ""},
		{"Dockerfile", "dockerfile"},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.want, ExtensionToLanguage(tt.ext))
		})
	}
}
