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

	// main.go should be cached (find by path since map iteration order is random)
	for id, fm := range files {
		if fm.Path == "main.go" {
			lines := fc.GetLines(id)
			require.NotNil(t, lines, "main.go should be cached")
			assert.Equal(t, "package main", lines[0])
			assert.Contains(t, lines[3], "Println")
			break
		}
	}

	// util.go should be cached
	for id, fm := range files {
		if fm.Path == "util.go" {
			lines := fc.GetLines(id)
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

func TestFileCache_ContentIndex_SingleToken(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"auth.go": "package auth\n\nfunc Login() {\n}\n",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	assert.True(t, fc.HasContentIndex(), "content index should be populated")

	refs := fc.ContentLookup("login")
	require.NotEmpty(t, refs, "should find 'login' token")

	// "Login" on line 3 → token "login"
	found := false
	for _, ref := range refs {
		if ref.LineNum == 3 {
			found = true
			break
		}
	}
	assert.True(t, found, "should find 'login' on line 3")
}

func TestFileCache_ContentIndex_CamelCase(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"parser.go": "package parser\n\n// TreeSitter parser\nfunc Parse() {}\n",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	// "TreeSitter" on line 3 should split into "tree" and "sitter"
	treeRefs := fc.ContentLookup("tree")
	sitterRefs := fc.ContentLookup("sitter")

	require.NotEmpty(t, treeRefs, "'tree' should have posting entries")
	require.NotEmpty(t, sitterRefs, "'sitter' should have posting entries")

	// Both should reference line 3
	var treeOnLine3, sitterOnLine3 bool
	for _, ref := range treeRefs {
		if ref.LineNum == 3 {
			treeOnLine3 = true
		}
	}
	for _, ref := range sitterRefs {
		if ref.LineNum == 3 {
			sitterOnLine3 = true
		}
	}
	assert.True(t, treeOnLine3, "'tree' should be on line 3")
	assert.True(t, sitterOnLine3, "'sitter' should be on line 3")
}

func TestFileCache_ContentIndex_DedupPerLine(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"dup.go": "login login login\n",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	refs := fc.ContentLookup("login")
	// Should have exactly 1 entry (deduplicated within the same line)
	count := 0
	for _, ref := range refs {
		if ref.LineNum == 1 {
			count++
		}
	}
	assert.Equal(t, 1, count, "should have single posting entry per line despite repeated token")
}

func TestFileCache_ContentIndex_ResetOnRewarm(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"old.go": "package old\nfunc OldFunc() {}\n",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	require.NotEmpty(t, fc.ContentLookup("old"), "'old' should exist after first warm")

	// Re-warm with different content
	files2 := makeTestFiles(t, dir, map[string]string{
		"new.go": "package fresh\nfunc NewFunc() {}\n",
	})
	fc.WarmFromIndex(files2, dir)

	assert.Empty(t, fc.ContentLookup("old"), "'old' should be gone after rewarm")
	assert.NotEmpty(t, fc.ContentLookup("fresh"), "'fresh' should exist after rewarm")
}

func TestFileCache_ContentLookup_Miss(t *testing.T) {
	dir := t.TempDir()
	files := makeTestFiles(t, dir, map[string]string{
		"a.go": "package a\n",
	})

	fc := NewFileCache(0)
	fc.WarmFromIndex(files, dir)

	refs := fc.ContentLookup("nonexistent")
	assert.Nil(t, refs, "unknown token should return nil")
}
