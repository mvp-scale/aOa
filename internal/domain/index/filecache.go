package index

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/corey/aoa/internal/ports"
)

const (
	maxCacheFileSize  = 512 * 1024        // 512 KB per file
	defaultCacheBytes = 250 * 1024 * 1024 // 250 MB total budget
)

// binaryExtensions are file extensions skipped during cache warming.
var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	".bmp": true, ".tiff": true, ".webp": true, ".svg": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
	".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".bz2": true,
	".xz": true, ".7z": true, ".rar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true,
	".mp3": true, ".mp4": true, ".wav": true, ".ogg": true, ".flac": true,
	".avi": true, ".mkv": true, ".mov": true, ".webm": true,
	".db": true, ".sqlite": true, ".class": true, ".pyc": true,
}

// FileCache holds pre-read file contents in memory to eliminate disk I/O
// from the search path. Thread-safe via internal RWMutex.
type FileCache struct {
	mu            sync.RWMutex
	entries       map[uint32]*cacheEntry
	totalMem      int64
	maxTotalBytes int64
	atCapacity    bool
}

type cacheEntry struct {
	lines []string
	size  int64
}

// NewFileCache creates a FileCache with the given memory budget.
// If maxBytes is 0, the default 250MB budget is used.
func NewFileCache(maxBytes int64) *FileCache {
	if maxBytes <= 0 {
		maxBytes = defaultCacheBytes
	}
	return &FileCache{
		entries:       make(map[uint32]*cacheEntry),
		maxTotalBytes: maxBytes,
	}
}

// GetLines returns the cached lines for a file, or nil on cache miss.
func (fc *FileCache) GetLines(fileID uint32) []string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	if e, ok := fc.entries[fileID]; ok {
		return e.lines
	}
	return nil
}

// Invalidate removes a file from the cache.
func (fc *FileCache) Invalidate(fileID uint32) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if e, ok := fc.entries[fileID]; ok {
		fc.totalMem -= e.size
		delete(fc.entries, fileID)
	}
}

// Stats returns cache statistics.
func (fc *FileCache) Stats() (count int, memBytes int64, atCapacity bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return len(fc.entries), fc.totalMem, fc.atCapacity
}

// WarmFromIndex replaces all cache entries by reading eligible files from disk.
// Files are loaded in LastModified descending order (most recent first) until
// the memory budget is reached.
func (fc *FileCache) WarmFromIndex(files map[uint32]*ports.FileMeta, projectRoot string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Reset
	fc.entries = make(map[uint32]*cacheEntry, len(files))
	fc.totalMem = 0
	fc.atCapacity = false

	// Sort files by LastModified descending so recent files get priority
	type fileRef struct {
		id   uint32
		meta *ports.FileMeta
	}
	sorted := make([]fileRef, 0, len(files))
	for id, fm := range files {
		sorted = append(sorted, fileRef{id, fm})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].meta.LastModified > sorted[j].meta.LastModified
	})

	for _, fr := range sorted {
		fm := fr.meta

		// Skip files over size limit
		if fm.Size > maxCacheFileSize || fm.Size <= 0 {
			continue
		}

		// Skip binary extensions
		ext := strings.ToLower(filepath.Ext(fm.Path))
		if binaryExtensions[ext] {
			continue
		}

		// Check budget before reading
		if fc.totalMem+fm.Size > fc.maxTotalBytes {
			fc.atCapacity = true
			break
		}

		absPath := filepath.Join(projectRoot, fm.Path)
		lines, size, ok := readAndValidateFile(absPath)
		if !ok {
			continue
		}

		// Re-check budget with actual size
		if fc.totalMem+size > fc.maxTotalBytes {
			fc.atCapacity = true
			break
		}

		fc.entries[fr.id] = &cacheEntry{lines: lines, size: size}
		fc.totalMem += size
	}

	// Check if we're at >=90% capacity
	if !fc.atCapacity && fc.maxTotalBytes > 0 {
		fc.atCapacity = fc.totalMem >= (fc.maxTotalBytes*9/10)
	}
}

// readAndValidateFile reads a file, validates it's text content, and returns
// pre-split lines. Returns (nil, 0, false) if the file should be skipped.
func readAndValidateFile(absPath string) ([]string, int64, bool) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, 0, false
	}
	defer f.Close()

	// Read first 512 bytes for binary detection
	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil || n == 0 {
		return nil, 0, false
	}
	header = header[:n]

	// Null byte check
	for _, b := range header {
		if b == 0 {
			return nil, 0, false
		}
	}

	// MIME type check
	mime := http.DetectContentType(header)
	if !isTextMIME(mime) {
		return nil, 0, false
	}

	// Seek back to start and read all lines
	if _, err := f.Seek(0, 0); err != nil {
		return nil, 0, false
	}

	var lines []string
	var totalSize int64
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		totalSize += int64(len(line)) + 1 // +1 for newline
		lines = append(lines, line)
	}
	if scanner.Err() != nil {
		return nil, 0, false
	}

	return lines, totalSize, true
}

// isTextMIME returns true if the MIME type indicates text content.
func isTextMIME(mime string) bool {
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	// http.DetectContentType returns "application/octet-stream" for unknown types,
	// but also returns specific application types for JSON, XML, etc.
	switch {
	case strings.Contains(mime, "javascript"),
		strings.Contains(mime, "json"),
		strings.Contains(mime, "xml"),
		strings.Contains(mime, "yaml"):
		return true
	}
	return false
}

// readFileLines reads a file from disk and returns its lines.
// Returns nil if the file can't be read.
func readFileLines(absPath string) []string {
	f, err := os.Open(absPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil
	}
	return lines
}
