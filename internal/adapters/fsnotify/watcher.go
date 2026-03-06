// Package fsnotify implements the ports.Watcher interface using github.com/fsnotify/fsnotify.
// It recursively watches a project directory, filters out non-code files and directories,
// and debounces rapid events (editors often trigger multiple writes per save).
package fsnotify

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Directories to ignore when watching.
var ignoreDirs = map[string]bool{
	".git":           true,
	"node_modules":   true,
	".venv":          true,
	"__pycache__":    true,
	"vendor":         true,
	".idea":          true,
	".vscode":        true,
	"dist":           true,
	"build":          true,
	".aoa":           true,
	".next":          true,
	"target":         true,
}

// File extensions/suffixes to ignore.
var ignoreFiles = map[string]bool{
	".DS_Store": true,
	".swp":      true,
	".pyc":      true,
	".o":        true,
	".so":       true,
	".dylib":    true,
}

// Watcher implements ports.Watcher using fsnotify.
type Watcher struct {
	fw         *fsnotify.Watcher
	done       chan struct{}
	wg         sync.WaitGroup // tracks the event loop goroutine
	stopped    bool
	mu         sync.Mutex
	allowPaths map[string]bool // paths exempt from ignore rules

	// excludePrefixes holds absolute path prefixes that are silently dropped
	// before any other filtering. Used to ignore paths that aOa itself writes
	// (e.g. .aoa/, .claude/settings.local.json). Set via Exclude().
	excludePrefixes []string

	// gitIgnoredPrefixes holds absolute path prefixes derived from .gitignore.
	// Updated at startup and whenever a .gitignore file changes. Protected by mu
	// for safe concurrent access from the event loop.
	gitIgnoredPrefixes []string
}

// NewWatcher creates a new file system watcher.
func NewWatcher() (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fw:         fw,
		done:       make(chan struct{}),
		allowPaths: make(map[string]bool),
	}, nil
}

// Exclude registers absolute path prefixes to silently drop from event
// processing. Any event whose path starts with one of these prefixes is
// discarded before debounce or callback. This is used to avoid reacting
// to files that aOa itself writes (e.g. everything under .aoa/).
//
// Must be called before Watch. Not safe for concurrent use.
func (w *Watcher) Exclude(prefixes []string) {
	w.excludePrefixes = append(w.excludePrefixes, prefixes...)
}

// isExcluded returns true if path matches any registered exclude prefix
// (static or gitignore-derived). Uses string prefix comparison for efficiency.
func (w *Watcher) isExcluded(path string) bool {
	for _, pfx := range w.excludePrefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	w.mu.Lock()
	gitPrefixes := w.gitIgnoredPrefixes
	w.mu.Unlock()
	for _, pfx := range gitPrefixes {
		if strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}

// SetGitIgnored replaces the gitignore-derived exclude prefixes.
// Safe for concurrent use — called from the event loop when .gitignore changes.
func (w *Watcher) SetGitIgnored(prefixes []string) {
	w.mu.Lock()
	w.gitIgnoredPrefixes = prefixes
	w.mu.Unlock()
}

// Watch starts monitoring projectPath recursively.
// onChange is called with the absolute path of each changed file.
func (w *Watcher) Watch(projectPath string, onChange func(filePath string)) error {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	// Walk and add all directories
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			if shouldIgnoreDir(info.Name()) && path != absPath {
				return filepath.SkipDir
			}
			return w.fw.Add(path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Debounce state: track last event time per file
	debounce := make(map[string]time.Time)
	var dmu sync.Mutex
	const debounceInterval = 500 * time.Millisecond

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			select {
			case event, ok := <-w.fw.Events:
				if !ok {
					return
				}
				path := event.Name

				// Drop events for excluded paths (aOa's own files)
				if w.isExcluded(path) {
					continue
				}

				// For Create events, add new directories to the watch list
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(path); err == nil && info.IsDir() {
						if !shouldIgnoreDir(info.Name()) {
							w.fw.Add(path)
						}
					}
				}

				// Skip ignored files/dirs (unless in allow list)
				if shouldIgnorePath(path) && !w.isAllowed(path) {
					continue
				}

				// Debounce: skip if we've seen this file recently
				dmu.Lock()
				last, exists := debounce[path]
				now := time.Now()
				if exists && now.Sub(last) < debounceInterval {
					dmu.Unlock()
					continue
				}
				debounce[path] = now
				dmu.Unlock()

				// Fire callback for relevant operations
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) ||
					event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					onChange(path)
				}

			case _, ok := <-w.fw.Errors:
				if !ok {
					return
				}
				// Errors are swallowed — fsnotify recovers automatically

			case <-w.done:
				return
			}
		}
	}()

	return nil
}

// WatchExtra adds an additional directory to the watch list, bypassing ignore rules.
// Files within this directory will be allowed through the ignore filter.
func (w *Watcher) WatchExtra(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.allowPaths[abs] = true
	w.mu.Unlock()
	return w.fw.Add(abs)
}

// Stop ends monitoring and releases all resources.
// Safe to call multiple times.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return nil
	}
	w.stopped = true
	close(w.done)
	err := w.fw.Close()
	w.wg.Wait() // block until event loop goroutine exits
	return err
}

// isAllowed returns true if the path is under an explicitly allowed directory.
func (w *Watcher) isAllowed(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for dir := range w.allowPaths {
		if strings.HasPrefix(path, dir+string(filepath.Separator)) || path == dir {
			return true
		}
	}
	return false
}

// shouldIgnoreDir returns true if the directory name should be skipped.
func shouldIgnoreDir(name string) bool {
	return ignoreDirs[name]
}

// shouldIgnorePath returns true if the file path should not trigger onChange.
func shouldIgnorePath(path string) bool {
	base := filepath.Base(path)

	// Check ignored file names/extensions
	if ignoreFiles[base] {
		return true
	}
	for ext := range ignoreFiles {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}

	// Check if any path component is an ignored directory
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if ignoreDirs[part] {
			return true
		}
	}

	return false
}
