package index

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
)

func TestRebuild_AfterMutation(t *testing.T) {
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	// Initial index with one symbol
	idx.Files[1] = &ports.FileMeta{Path: "a.go", Language: "go", Size: 100}
	ref := ports.TokenRef{FileID: 1, Line: 5}
	idx.Metadata[ref] = &ports.SymbolMeta{Name: "hello", Signature: "hello()", Kind: "function", StartLine: 5, EndLine: 10}
	idx.Tokens["hello"] = []ports.TokenRef{ref}

	engine := NewSearchEngine(idx, make(map[string]Domain), "")

	// Verify initial search works
	result := engine.Search("hello", ports.SearchOptions{})
	assert.Equal(t, 1, len(result.Hits))

	// Mutate: add a new symbol
	ref2 := ports.TokenRef{FileID: 1, Line: 20}
	idx.Metadata[ref2] = &ports.SymbolMeta{Name: "world", Signature: "world()", Kind: "function", StartLine: 20, EndLine: 25}
	idx.Tokens["world"] = []ports.TokenRef{ref2}

	// Before rebuild, "world" may not be findable via OR density scoring
	// because refToTokens is stale. Rebuild should fix this.
	engine.Rebuild()

	result = engine.Search("world", ports.SearchOptions{})
	assert.Equal(t, 1, len(result.Hits))
	assert.Equal(t, "world()", result.Hits[0].Symbol)

	// Original symbol still searchable
	result = engine.Search("hello", ports.SearchOptions{})
	assert.Equal(t, 1, len(result.Hits))
}
