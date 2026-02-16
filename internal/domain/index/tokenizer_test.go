package index

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenize_CamelCase(t *testing.T) {
	// Q19: getUserToken -> ["get", "user", "token"]
	assert.Equal(t, []string{"get", "user", "token"}, Tokenize("getUserToken"))
}

func TestTokenize_DottedName(t *testing.T) {
	// Q20: app.post -> ["app", "post"]
	assert.Equal(t, []string{"app", "post"}, Tokenize("app.post"))
}

func TestTokenize_Hyphenated(t *testing.T) {
	// Q21: tree-sitter -> ["tree", "sitter"]
	assert.Equal(t, []string{"tree", "sitter"}, Tokenize("tree-sitter"))
}

func TestTokenize_Unicode(t *testing.T) {
	// Q22: résumé -> graceful handling, no crash
	result := Tokenize("résumé")
	// After stripping non-ASCII: "rsum" -> ["rsum"] or nil depending on what remains
	// The accented chars are stripped, leaving "rsum" which is >= 2 chars
	assert.NotPanics(t, func() { Tokenize("résumé") })
	_ = result
}

func TestTokenize_ShortToken(t *testing.T) {
	// Q05: single char "a" below min length 2
	assert.Nil(t, Tokenize("a"))
}

func TestTokenize_Uppercase(t *testing.T) {
	// Q15: "LOGIN" -> ["login"]
	assert.Equal(t, []string{"login"}, Tokenize("LOGIN"))
}

func TestTokenize_Empty(t *testing.T) {
	assert.Nil(t, Tokenize(""))
}

func TestTokenize_Underscored(t *testing.T) {
	assert.Equal(t, []string{"get", "user", "by", "id"}, Tokenize("get_user_by_id"))
}

func TestTokenize_SlashSeparated(t *testing.T) {
	assert.Equal(t, []string{"services", "auth", "handler"}, Tokenize("services/auth/handler"))
}

func TestTokenize_APIKey(t *testing.T) {
	// SPEC.md: APIKey -> ["api", "key"]
	assert.Equal(t, []string{"api", "key"}, Tokenize("APIKey"))
}

func TestTokenize_MixedSeparators(t *testing.T) {
	assert.Equal(t, []string{"my", "cool", "func", "name"}, Tokenize("my-cool.func_name"))
}

func TestTokenize_NumbersPreserved(t *testing.T) {
	result := Tokenize("handler404Response")
	assert.Contains(t, result, "handler")
	assert.Contains(t, result, "404")
	assert.Contains(t, result, "response")
}

func TestTokenize_SingleValidToken(t *testing.T) {
	assert.Equal(t, []string{"login"}, Tokenize("login"))
}
