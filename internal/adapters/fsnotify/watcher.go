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
	fw      *fsnotify.Watcher
	done    chan struct{}
	stopped bool
	mu      sync.Mutex
}

// NewWatcher creates a new file system watcher.
func NewWatcher() (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fw:   fw,
		done: make(chan struct{}),
	}, nil
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
	const debounceInterval = 50 * time.Millisecond

	go func() {
		for {
			select {
			case event, ok := <-w.fw.Events:
				if !ok {
					return
				}
				path := event.Name

				// For Create events, add new directories to the watch list
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(path); err == nil && info.IsDir() {
						if !shouldIgnoreDir(info.Name()) {
							w.fw.Add(path)
						}
					}
				}

				// Skip ignored files/dirs
				if shouldIgnorePath(path) {
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
	return w.fw.Close()
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
