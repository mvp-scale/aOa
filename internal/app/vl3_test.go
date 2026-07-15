//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRouteEntries_GinAndNetHTTP(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "gin_main.go"),
		[]byte(`package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/ping", pingHandler)
	r.Run()
}
`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "http_main.go"),
		[]byte(`package main

import "net/http"

func serve() {
	http.HandleFunc("/status", statusHandler)
}
`),
		0644,
	))

	idx, _, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	entries := buildRouteEntries(tmpDir, idx, parser)
	require.Len(t, entries, 2)

	byPath := make(map[string]int)
	for i, e := range entries {
		byPath[e.Path] = i
	}
	require.Contains(t, byPath, "/ping")
	require.Contains(t, byPath, "/status")
	assert.Equal(t, "GET", entries[byPath["/ping"]].Method)
	assert.Equal(t, "gin", entries[byPath["/ping"]].Framework)
	assert.Equal(t, "gin_main.go", entries[byPath["/ping"]].File)
	assert.Equal(t, "", entries[byPath["/status"]].Method)
	assert.Equal(t, "net/http", entries[byPath["/status"]].Framework)
}

func TestBuildRouteEntries_NoGoFiles_ReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.py"), []byte("x = 1\n"), 0644))

	idx, _, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	entries := buildRouteEntries(tmpDir, idx, parser)
	assert.Nil(t, entries)
}

func TestBuildRouteEntries_NilParser_ReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	idx, _, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)
	assert.Nil(t, buildRouteEntries(tmpDir, idx, nil))
}

func TestBuildRouteEntries_NilIndex_ReturnsNil(t *testing.T) {
	parser := treesitter.NewParser()
	assert.Nil(t, buildRouteEntries(t.TempDir(), nil, parser))
}
