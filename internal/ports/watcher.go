package ports

// Watcher monitors a project directory for file changes and triggers re-indexing.
// The adapter (fsnotify) must filter out non-code files (.git, node_modules, etc.)
// before invoking onChange. Only one Watch call should be active at a time.
type Watcher interface {
	// Watch starts monitoring projectPath recursively. onChange is called with
	// the absolute path of each changed file. The callback may be invoked from
	// any goroutine. Returns an error if the directory doesn't exist or
	// permissions are insufficient.
	Watch(projectPath string, onChange func(filePath string)) error

	// Stop ends monitoring and releases all resources. After Stop returns,
	// no further onChange calls will fire. Safe to call multiple times.
	Stop() error
}
