package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestFiles(t *testing.T, dir string, files map[string]string) map[uint32]*ports.FileMeta {
	t.Helper()
	metas := make(map[uint32]*ports.FileMeta)
	var id uint32
	for name, content := range files {
		id++
		absPath := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
		require.NoError(t, os.WriteFile(absPath, []byte(content), 0o644))
		info, err := os.Stat(absPath)
		require.NoError(t, err)
		metas[id] = &ports.FileMeta{
			Path:         name,
			Size:         info.Size(),
			LastModified: info.ModTime().Unix(),
			Language:     strings.TrimPrefix(filepath.Ext(name), "."),
		}
	}
	return metas
}

func TestFileCache_WarmAndGet(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"main.go":  "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
		"util.go":  "package util\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n",
		"empty.go": "",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	// main.go should be cached
	lines := fc.GetLines(1)
	if lines == nil {
		// The map iteration order might assign different IDs
		// Find the ID for main.go
		for id, fm := range files {
			if fm.Path == "main.go" {
				lines = fc.GetLines(id)
				break
			}
		}
	}
	require.NotNil(t, lines, "main.go should be cached")
	assert.Equal(t, "package main", lines[0])
	assert.Contains(t, lines[3], "Println")

	// util.go should be cached
	for id, fm := range files {
		if fm.Path == "util.go" {
			lines = fc.GetLines(id)
			require.NotNil(t, lines, "util.go should be cached")
			assert.Equal(t, "package util", lines[0])
			break
		}
	}

	// Stats should reflect cached files
	count, mem, _ := fc.Stats()
	assert.GreaterOrEqual(t, count, 2, "at least 2 non-empty files cached")
	assert.Greater(t, mem, int64(0), "memory should be > 0")
}

func TestFileCache_SkipsBinaryExtension(t *testing.T) {
	dir := t.TempDir()

	// Create a .png and a .go file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), []byte("fake png"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package code\n"), 0o644))

	pngInfo, _ := os.Stat(filepath.Join(dir, "image.png"))
	goInfo, _ := os.Stat(filepath.Join(dir, "code.go"))

	files := map[uint32]*ports.FileMeta{
		1: {Path: "image.png", Size: pngInfo.Size(), LastModified: pngInfo.ModTime().Unix()},
		2: {Path: "code.go", Size: goInfo.Size(), LastModified: goInfo.ModTime().Unix()},
	}

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	assert.Nil(t, fc.GetLines(1), ".png should be skipped")
	assert.NotNil(t, fc.GetLines(2), ".go should be cached")
}

func TestFileCache_SkipsBinaryContent(t *testing.T) {
	dir := t.TempDir()

	// File with null bytes (binary content)
	binaryContent := []byte("hello\x00world\nline2\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "binary.txt"), binaryContent, 0o644))

	// Normal text file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "text.txt"), []byte("hello world\nline2\n"), 0o644))

	binInfo, _ := os.Stat(filepath.Join(dir, "binary.txt"))
	txtInfo, _ := os.Stat(filepath.Join(dir, "text.txt"))

	files := map[uint32]*ports.FileMeta{
		1: {Path: "binary.txt", Size: binInfo.Size(), LastModified: binInfo.ModTime().Unix()},
		2: {Path: "text.txt", Size: txtInfo.Size(), LastModified: txtInfo.ModTime().Unix()},
	}

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	assert.Nil(t, fc.GetLines(1), "binary file should be skipped")
	assert.NotNil(t, fc.GetLines(2), "text file should be cached")
}

func TestFileCache_Invalidate(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"a.go": "package a\n",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	var fileID uint32
	for id := range files {
		fileID = id
	}

	require.NotNil(t, fc.GetLines(fileID))

	fc.Invalidate(fileID)
	assert.Nil(t, fc.GetLines(fileID), "invalidated file should return nil")

	count, mem, _ := fc.Stats()
	assert.Equal(t, 0, count)
	assert.Equal(t, int64(0), mem)
}

func TestFileCache_MaxFileSize(t *testing.T) {
	dir := t.TempDir()

	// Create a file larger than 512KB
	bigContent := strings.Repeat("x", maxCacheFileSize+1) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.go"), []byte(bigContent), 0o644))

	// Create a small file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "small.go"), []byte("package small\n"), 0o644))

	bigInfo, _ := os.Stat(filepath.Join(dir, "big.go"))
	smallInfo, _ := os.Stat(filepath.Join(dir, "small.go"))

	files := map[uint32]*ports.FileMeta{
		1: {Path: "big.go", Size: bigInfo.Size(), LastModified: bigInfo.ModTime().Unix()},
		2: {Path: "small.go", Size: smallInfo.Size(), LastModified: smallInfo.ModTime().Unix()},
	}

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	assert.Nil(t, fc.GetLines(1), ">512KB file should be skipped")
	assert.NotNil(t, fc.GetLines(2), "small file should be cached")
}

func TestFileCache_MemoryBudget(t *testing.T) {
	dir := t.TempDir()

	// Create files that together exceed a tiny budget
	content := strings.Repeat("line of text\n", 100) // ~1300 bytes each
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.go"), []byte(content), 0o644))

	aInfo, _ := os.Stat(filepath.Join(dir, "a.go"))
	bInfo, _ := os.Stat(filepath.Join(dir, "b.go"))
	cInfo, _ := os.Stat(filepath.Join(dir, "c.go"))

	files := map[uint32]*ports.FileMeta{
		1: {Path: "a.go", Size: aInfo.Size(), LastModified: 3},
		2: {Path: "b.go", Size: bInfo.Size(), LastModified: 2},
		3: {Path: "c.go", Size: cInfo.Size(), LastModified: 1},
	}

	// Budget of 2000 bytes — should fit ~1 file but not all 3
	fc := NewFileCache(2000)
	fc.WarmFromIndex(files, dir)

	count, mem, atCap := fc.Stats()
	assert.Greater(t, count, 0, "at least one file should be cached")
	assert.Less(t, count, 3, "not all files should fit")
	assert.LessOrEqual(t, mem, int64(2000), "memory should not exceed budget")
	assert.True(t, atCap, "should be at capacity")
}

func TestFileCache_Stats(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"x.go": "package x\nfunc X() {}\n",
		"y.go": "package y\nfunc Y() {}\n",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	count, mem, atCap := fc.Stats()
	assert.Equal(t, 2, count)
	assert.Greater(t, mem, int64(0))
	assert.False(t, atCap, "250MB budget should not be hit by tiny files")
}

func TestFileCache_MissReturnsNil(t *testing.T) {
	fc := NewFileCache(0)
	assert.Nil(t, fc.GetLines(999), "cache miss should return nil")
}
