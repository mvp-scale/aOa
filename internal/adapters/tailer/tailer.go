package tailer

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Tailer watches Claude Code session JSONL files and emits parsed events.
//
// It discovers the session directory for a project, finds the most recent
// JSONL file, seeks to the end (skipping old history), and polls for new lines.
// When a new session file appears (newer mtime), it switches automatically.
//
// Thread-safe: Start/Stop can be called from any goroutine.
type Tailer struct {
	sessionDir  string
	pollInterval time.Duration

	callback func(*SessionEvent) // called for each parsed event
	onError  func(error)         // called for parse errors (optional)

	// State
	currentFile string
	offset      int64
	seen        map[string]bool // UUID dedup set

	// L9.4: Subagent file tracking
	subagentFiles map[string]int64 // path → last byte offset

	mu      sync.Mutex
	done    chan struct{}
	started chan struct{} // closed after initial file discovery
	wg      sync.WaitGroup
}

// Config holds parameters for creating a Tailer.
type Config struct {
	// ProjectRoot is the absolute path to the project (e.g., /home/corey/aOa).
	// Used to compute the session directory path.
	ProjectRoot string

	// SessionDir overrides automatic session directory discovery.
	// If set, ProjectRoot is ignored for directory resolution.
	SessionDir string

	// PollInterval is how often to check for new lines. Default: 500ms.
	PollInterval time.Duration

	// Callback is called for each successfully parsed SessionEvent.
	// Must be non-nil.
	Callback func(*SessionEvent)

	// OnError is called when a JSONL line fails to parse. Optional.
	OnError func(error)
}

// New creates a Tailer. Does not start tailing until Start() is called.
func New(cfg Config) *Tailer {
	sessionDir := cfg.SessionDir
	if sessionDir == "" {
		sessionDir = SessionDirForProject(cfg.ProjectRoot)
	}

	interval := cfg.PollInterval
	if interval == 0 {
		interval = 500 * time.Millisecond
	}

	return &Tailer{
		sessionDir:    sessionDir,
		pollInterval:  interval,
		callback:      cfg.Callback,
		onError:       cfg.OnError,
		seen:          make(map[string]bool),
		subagentFiles: make(map[string]int64),
		done:          make(chan struct{}),
		started:       make(chan struct{}),
	}
}

// Start begins the tailing loop in a background goroutine.
// The tailer seeks to the end of the current file (no replay of old events).
func (t *Tailer) Start() {
	t.wg.Add(1)
	go t.loop()
}

// Stop terminates the tailing loop and waits for it to finish.
// Safe to call multiple times.
func (t *Tailer) Stop() {
	t.mu.Lock()
	select {
	case <-t.done:
		// Already stopped
		t.mu.Unlock()
		return
	default:
		close(t.done)
	}
	t.mu.Unlock()
	t.wg.Wait()
}

// SessionDir returns the directory being watched.
func (t *Tailer) SessionDir() string {
	return t.sessionDir
}

// CurrentFile returns the JSONL file currently being tailed.
func (t *Tailer) CurrentFile() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentFile
}

// Started returns a channel that closes after initial file discovery completes.
// Useful for tests that need to wait for the tailer to be ready before writing.
func (t *Tailer) Started() <-chan struct{} {
	return t.started
}

func (t *Tailer) loop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	// Initial file discovery — seek to end
	t.switchToLatestFile(true)
	close(t.started)

	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			t.checkForNewerFile()
			t.readNewLines()
			t.readSubagentLines() // L9.4
		}
	}
}

// switchToLatestFile finds the most recent .jsonl file and optionally seeks to end.
func (t *Tailer) switchToLatestFile(seekToEnd bool) {
	latest := t.findLatestJSONL()
	if latest == "" {
		return
	}

	t.mu.Lock()
	t.currentFile = latest
	t.mu.Unlock()

	if seekToEnd {
		info, err := os.Stat(latest)
		if err == nil {
			t.offset = info.Size()
		}
	} else {
		t.offset = 0
	}
}

// checkForNewerFile detects if a newer session file appeared (new session started).
func (t *Tailer) checkForNewerFile() {
	latest := t.findLatestJSONL()
	if latest == "" {
		return
	}

	t.mu.Lock()
	current := t.currentFile
	t.mu.Unlock()

	if latest != current {
		// New session file — switch to it, start from beginning
		t.mu.Lock()
		t.currentFile = latest
		t.mu.Unlock()
		t.offset = 0
	}
}

// readNewLines reads any new content appended since last read.
// Uses ReadBytes('\n') to track exact byte offsets (bufio.Scanner
// reads ahead and corrupts file position tracking).
func (t *Tailer) readNewLines() {
	t.mu.Lock()
	path := t.currentFile
	t.mu.Unlock()

	if path == "" {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		return // file gone or locked — skip this cycle
	}
	defer f.Close()

	// Check if file was truncated (rewritten)
	info, err := f.Stat()
	if err != nil {
		return
	}
	if info.Size() < t.offset {
		// File was truncated — start from beginning
		t.offset = 0
	}

	if info.Size() == t.offset {
		return // no new data
	}

	// Seek to last known position
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return
	}

	reader := bufio.NewReaderSize(f, 2*1024*1024) // 2MB buffer

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			break // EOF or error with no data
		}

		// Track consumed bytes (including the newline)
		t.offset += int64(len(line))

		// Trim newline and carriage return
		line = trimNewline(line)
		if len(line) == 0 {
			if err != nil {
				break
			}
			continue
		}

		// Cap line size — skip multi-MB tool outputs (known bug #23948)
		if len(line) > 512*1024 {
			if err != nil {
				break
			}
			continue
		}

		ev, parseErr := ParseLine(line)
		if parseErr != nil {
			if t.onError != nil {
				t.onError(parseErr)
			}
			if err != nil {
				break
			}
			continue
		}
		if ev == nil {
			if err != nil {
				break
			}
			continue
		}

		// L9.3: Resolve zero-char tool result sizes from persisted files
		if len(ev.ToolResultSizes) > 0 {
			t.resolvePersistedToolResults(ev)
		}

		// UUID dedup — skip already-seen events (known bug #5034)
		if ev.UUID != "" {
			if t.seen[ev.UUID] {
				if err != nil {
					break
				}
				continue
			}
			t.seen[ev.UUID] = true
		}

		// Skip meta messages (internal commands like /clear)
		if ev.IsMeta {
			if err != nil {
				break
			}
			continue
		}

		if t.callback != nil {
			t.callback(ev)
		}

		if err != nil {
			break // EOF after processing last line
		}
	}

	// Bound dedup set to prevent unbounded growth
	if len(t.seen) > 10000 {
		t.seen = make(map[string]bool)
	}
}

// readSubagentLines discovers and reads subagent JSONL files (L9.4).
// Subagent files live in {sessionDir}/subagents/agent-*.jsonl.
// Best-effort: short-lived subagents may be partially captured.
func (t *Tailer) readSubagentLines() {
	t.mu.Lock()
	currentFile := t.currentFile
	t.mu.Unlock()

	if currentFile == "" {
		return
	}

	sessionDir := filepath.Dir(currentFile)
	subagentDir := filepath.Join(sessionDir, "subagents")

	entries, err := os.ReadDir(subagentDir)
	if err != nil {
		return // no subagents directory — common case
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(subagentDir, entry.Name())

		// Check file size
		info, err := entry.Info()
		if err != nil {
			continue
		}

		offset := t.subagentFiles[path]
		if info.Size() <= offset {
			continue // no new data
		}
		if info.Size() < offset {
			offset = 0 // file truncated
		}

		// Read new lines from this subagent file
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if offset > 0 {
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				f.Close()
				continue
			}
		}

		reader := bufio.NewReaderSize(f, 256*1024) // 256KB buffer (subagent files are smaller)
		newOffset := offset

		for {
			line, readErr := reader.ReadBytes('\n')
			if len(line) == 0 && readErr != nil {
				break
			}

			newOffset += int64(len(line))
			line = trimNewline(line)
			if len(line) == 0 {
				if readErr != nil {
					break
				}
				continue
			}
			if len(line) > 512*1024 {
				if readErr != nil {
					break
				}
				continue
			}

			ev, parseErr := ParseLine(line)
			if parseErr != nil || ev == nil {
				if readErr != nil {
					break
				}
				continue
			}

			// Tag as subagent source
			ev.Source = "subagent"

			// UUID dedup
			if ev.UUID != "" {
				if t.seen[ev.UUID] {
					if readErr != nil {
						break
					}
					continue
				}
				t.seen[ev.UUID] = true
			}

			if ev.IsMeta {
				if readErr != nil {
					break
				}
				continue
			}

			if t.callback != nil {
				t.callback(ev)
			}

			if readErr != nil {
				break
			}
		}

		f.Close()
		t.subagentFiles[path] = newOffset
	}
}

// resolvePersistedToolResults checks tool-results/ for persisted files when
// tool result inline content is 0 chars. Claude Code writes large tool results
// to tool-results/toolu_{id}.txt instead of inline in the JSONL.
func (t *Tailer) resolvePersistedToolResults(ev *SessionEvent) {
	t.mu.Lock()
	currentFile := t.currentFile
	t.mu.Unlock()

	if currentFile == "" {
		return
	}

	// Derive session dir from the JSONL file path.
	// The JSONL sits directly in the session directory.
	sessionDir := filepath.Dir(currentFile)
	toolResultsDir := filepath.Join(sessionDir, "tool-results")

	for id, chars := range ev.ToolResultSizes {
		if chars > 0 {
			continue // already has inline content
		}
		// Check for persisted file: tool-results/toolu_{id}.txt
		// The ID from the JSONL is the full tool_use_id (e.g., "toolu_abc123")
		persistedPath := filepath.Join(toolResultsDir, id+".txt")
		info, err := os.Stat(persistedPath)
		if err != nil {
			continue // file doesn't exist or not readable
		}
		size := int(info.Size())
		if size > 0 {
			ev.ToolResultSizes[id] = size
			// Track which IDs were resolved from disk
			if ev.ToolPersistedIDs == nil {
				ev.ToolPersistedIDs = make(map[string]bool)
			}
			ev.ToolPersistedIDs[id] = true
		}
	}
}

// trimNewline removes trailing \n and \r\n from a line.
func trimNewline(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line
}

// findLatestJSONL returns the most recently modified .jsonl file in the session dir.
func (t *Tailer) findLatestJSONL() string {
	entries, err := os.ReadDir(t.sessionDir)
	if err != nil {
		return ""
	}

	type fileWithTime struct {
		path    string
		modTime time.Time
	}

	var jsonlFiles []fileWithTime
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		jsonlFiles = append(jsonlFiles, fileWithTime{
			path:    filepath.Join(t.sessionDir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	if len(jsonlFiles) == 0 {
		return ""
	}

	// Sort by modification time descending — most recent first
	sort.Slice(jsonlFiles, func(i, j int) bool {
		return jsonlFiles[i].modTime.After(jsonlFiles[j].modTime)
	})

	return jsonlFiles[0].path
}

// SessionDirForProject computes the Claude Code session directory for a project.
// Claude Code stores sessions at ~/.claude/projects/{encoded-path}/
// where encoded-path replaces "/" with "-".
// Example: /home/corey/aOa → ~/.claude/projects/-home-corey-aOa/
func SessionDirForProject(projectRoot string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	// Encode project path: replace "/" with "-"
	encoded := strings.ReplaceAll(projectRoot, "/", "-")

	return filepath.Join(home, ".claude", "projects", encoded)
}
