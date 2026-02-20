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

// contentRef is a posting list entry in the content token inverted index.
type contentRef struct {
	FileID  uint32
	LineNum uint16 // 1-based
}

// FileCache holds pre-read file contents in memory to eliminate disk I/O
// from the search path. Thread-safe via internal RWMutex.
type FileCache struct {
	mu            sync.RWMutex
	entries       map[uint32]*cacheEntry
	contentIndex  map[string][]contentRef    // token → posting list
	trigramIndex  map[[3]byte][]contentRef   // trigram → sorted posting list (fileID, lineNum)
	totalMem      int64
	maxTotalBytes int64
	atCapacity    bool
}

type cacheEntry struct {
	lines      []string
	lowerLines []string // pre-lowered for trigram verify and brute-force fallback
	size       int64
}

// NewFileCache creates a FileCache with the given memory budget.
// If maxBytes is 0, the default 250MB budget is used.
func NewFileCache(maxBytes int64) *FileCache {
	if maxBytes <= 0 {
		maxBytes = defaultCacheBytes
	}
	return &FileCache{
		entries:       make(map[uint32]*cacheEntry),
		contentIndex:  make(map[string][]contentRef),
		trigramIndex:  make(map[[3]byte][]contentRef),
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
	fc.contentIndex = make(map[string][]contentRef)
	fc.trigramIndex = make(map[[3]byte][]contentRef)
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

	// Build content token inverted index from cached entries
	fc.buildContentIndex()

	// Build trigram inverted index + pre-lowered lines from cached entries
	fc.buildTrigramIndex()
}

// buildContentIndex tokenizes every cached line and builds an inverted index
// mapping each token to its (fileID, lineNum) posting list. Must be called
// with fc.mu held (write lock).
func (fc *FileCache) buildContentIndex() {
	for fileID, entry := range fc.entries {
		for lineIdx, line := range entry.lines {
			tokens := TokenizeContentLine(line)
			if len(tokens) == 0 {
				continue
			}
			lineNum := uint16(lineIdx + 1)
			// Deduplicate tokens within the same line
			seen := make(map[string]bool, len(tokens))
			for _, tok := range tokens {
				if seen[tok] {
					continue
				}
				seen[tok] = true
				fc.contentIndex[tok] = append(fc.contentIndex[tok], contentRef{
					FileID:  fileID,
					LineNum: lineNum,
				})
			}
		}
	}
}

// ContentLookup returns the posting list for a content token.
func (fc *FileCache) ContentLookup(token string) []contentRef {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.contentIndex[token]
}

// HasContentIndex returns true if the content inverted index is populated.
func (fc *FileCache) HasContentIndex() bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return len(fc.contentIndex) > 0
}

// buildTrigramIndex extracts 3-byte substrings from every cached line (lowercased)
// and builds an inverted index mapping each trigram to its (fileID, lineNum) postings.
// Also populates lowerLines on each cache entry. Must be called with fc.mu held.
func (fc *FileCache) buildTrigramIndex() {
	// Iterate file IDs in sorted order so posting lists are naturally sorted
	fileIDs := make([]uint32, 0, len(fc.entries))
	for fid := range fc.entries {
		fileIDs = append(fileIDs, fid)
	}
	sort.Slice(fileIDs, func(i, j int) bool { return fileIDs[i] < fileIDs[j] })

	for _, fileID := range fileIDs {
		entry := fc.entries[fileID]
		entry.lowerLines = make([]string, len(entry.lines))
		for lineIdx, line := range entry.lines {
			lower := strings.ToLower(line)
			entry.lowerLines[lineIdx] = lower

			n := len(lower)
			if n < 3 {
				continue
			}

			lineNum := uint16(lineIdx + 1) // 1-based
			ref := contentRef{FileID: fileID, LineNum: lineNum}
			seen := make(map[[3]byte]bool)
			for i := 0; i <= n-3; i++ {
				key := [3]byte{lower[i], lower[i+1], lower[i+2]}
				if seen[key] {
					continue
				}
				seen[key] = true
				fc.trigramIndex[key] = append(fc.trigramIndex[key], ref)
			}
		}
	}
}

// TrigramLookup intersects posting lists for the given trigrams and returns
// candidate (fileID, lineNum) pairs. Returns nil if any trigram has no postings.
// Posting lists are sorted by (FileID, LineNum), enabling merge-join intersection.
func (fc *FileCache) TrigramLookup(trigrams [][3]byte) []contentRef {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	if len(trigrams) == 0 || len(fc.trigramIndex) == 0 {
		return nil
	}

	// Find the shortest posting list (start intersection with fewest candidates)
	shortestIdx := 0
	shortestLen := len(fc.trigramIndex[trigrams[0]])
	if shortestLen == 0 {
		return nil
	}
	for i := 1; i < len(trigrams); i++ {
		l := len(fc.trigramIndex[trigrams[i]])
		if l == 0 {
			return nil
		}
		if l < shortestLen {
			shortestIdx = i
			shortestLen = l
		}
	}

	// Copy the shortest posting list as our working set
	result := make([]contentRef, shortestLen)
	copy(result, fc.trigramIndex[trigrams[shortestIdx]])

	// Intersect with each other posting list
	for i, tri := range trigrams {
		if i == shortestIdx {
			continue
		}
		result = intersectContentRefs(result, fc.trigramIndex[tri])
		if len(result) == 0 {
			return nil
		}
	}

	return result
}

// intersectContentRefs performs a merge-join intersection of two sorted
// contentRef slices. Both inputs must be sorted by (FileID, LineNum).
func intersectContentRefs(a, b []contentRef) []contentRef {
	var result []contentRef
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].FileID < b[j].FileID || (a[i].FileID == b[j].FileID && a[i].LineNum < b[j].LineNum) {
			i++
		} else if a[i].FileID > b[j].FileID || (a[i].FileID == b[j].FileID && a[i].LineNum > b[j].LineNum) {
			j++
		} else {
			result = append(result, a[i])
			i++
			j++
		}
	}
	return result
}

// GetLowerLines returns the pre-lowered lines for a file, or nil on cache miss.
func (fc *FileCache) GetLowerLines(fileID uint32) []string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	if e, ok := fc.entries[fileID]; ok {
		return e.lowerLines
	}
	return nil
}

// HasTrigramIndex returns true if the trigram inverted index is populated.
func (fc *FileCache) HasTrigramIndex() bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return len(fc.trigramIndex) > 0
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
