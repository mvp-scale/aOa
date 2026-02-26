//go:build !recon

package app

// initDimEngine is a no-op in lean builds (no tree-sitter, no dimensional analysis).
func (a *App) initDimEngine() {}

// warmDimCache is a no-op in lean builds.
func (a *App) warmDimCache(logFn func(string)) (int, int, bool) {
	return 0, 0, false
}

// updateDimForFile is a no-op in lean builds.
func (a *App) updateDimForFile(fileID uint32, relPath string) {}
