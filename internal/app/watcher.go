package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
)

// onFileChanged handles a file create/modify/delete event from the watcher.
// It updates the index in-place and rebuilds the search engine.
func (a *App) onFileChanged(absPath string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext == "" {
		return
	}
	// When parser is nil (tokenization-only mode), use the default code extensions list.
	if a.Parser != nil && !a.Parser.SupportsExtension(ext) {
		return
	}
	if a.Parser == nil && !defaultCodeExtensions[ext] {
		return
	}

	relPath, err := filepath.Rel(a.ProjectRoot, absPath)
	if err != nil {
		return
	}

	// Find existing fileID for this path
	var existingID uint32
	for id, fm := range a.Index.Files {
		if fm.Path == relPath {
			existingID = id
			break
		}
	}

	// Check if file was deleted
	info, statErr := os.Stat(absPath)
	if statErr != nil {
		// File removed
		if existingID > 0 {
			a.removeFileFromIndex(existingID)
			a.Engine.Rebuild()
			if a.Store != nil {
				_ = a.Store.SaveIndex(a.ProjectID, a.Index)
			}
		}
		return
	}

	// Skip files > 1MB
	if info.Size() > 1<<20 {
		return
	}

	source, err := os.ReadFile(absPath)
	if err != nil {
		return
	}

	// Remove old entry if modifying existing file
	if existingID > 0 {
		a.removeFileFromIndex(existingID)
	}

	// Allocate new fileID (max existing + 1)
	var fileID uint32
	if existingID > 0 {
		fileID = existingID
	} else {
		for id := range a.Index.Files {
			if id >= fileID {
				fileID = id + 1
			}
		}
		if fileID == 0 {
			fileID = 1
		}
	}

	ext = strings.TrimPrefix(filepath.Ext(absPath), ".")
	a.Index.Files[fileID] = &ports.FileMeta{
		Path:         relPath,
		LastModified: info.ModTime().Unix(),
		Language:     ext,
		Size:         info.Size(),
	}

	// When parser is available, extract symbols; otherwise tokenize file content only.
	var metas []*ports.SymbolMeta
	if a.Parser != nil {
		var parseErr error
		metas, parseErr = a.Parser.ParseFileToMeta(absPath, source)
		if parseErr != nil {
			metas = nil
		}
	}

	if len(metas) == 0 {
		// No symbols (parser nil or no matches) — tokenize file content for file-level search.
		lines := strings.Split(string(source), "\n")
		for _, line := range lines {
			tokens := index.TokenizeContentLine(line)
			for _, tok := range tokens {
				ref := ports.TokenRef{FileID: fileID, Line: 0}
				a.Index.Tokens[tok] = append(a.Index.Tokens[tok], ref)
			}
		}
		a.Engine.Rebuild()
		if a.Store != nil {
			_ = a.Store.SaveIndex(a.ProjectID, a.Index)
		}
		return
	}

	for _, meta := range metas {
		ref := ports.TokenRef{FileID: fileID, Line: meta.StartLine}
		a.Index.Metadata[ref] = meta

		tokens := index.Tokenize(meta.Name)
		for _, tok := range tokens {
			a.Index.Tokens[tok] = append(a.Index.Tokens[tok], ref)
		}

		lower := strings.ToLower(meta.Name)
		if lower != "" {
			a.Index.Tokens[lower] = append(a.Index.Tokens[lower], ref)
		}
	}

	a.Engine.Rebuild()
	if a.Store != nil {
		_ = a.Store.SaveIndex(a.ProjectID, a.Index)
	}

	// If aoa-recon is available and parser is nil, trigger incremental enhancement.
	// When parser is non-nil, symbols are already extracted above.
	if a.Parser == nil {
		a.TriggerReconEnhanceFile(absPath)
	}
}

// removeFileFromIndex removes all entries for a fileID from the index maps.
// Must be called with a.mu held.
func (a *App) removeFileFromIndex(fileID uint32) {
	// Remove metadata entries for this file
	for ref := range a.Index.Metadata {
		if ref.FileID == fileID {
			delete(a.Index.Metadata, ref)
		}
	}

	// Remove token refs for this file; delete empty token entries
	for tok, refs := range a.Index.Tokens {
		var kept []ports.TokenRef
		for _, ref := range refs {
			if ref.FileID != fileID {
				kept = append(kept, ref)
			}
		}
		if len(kept) == 0 {
			delete(a.Index.Tokens, tok)
		} else {
			a.Index.Tokens[tok] = kept
		}
	}

	// Remove file entry
	delete(a.Index.Files, fileID)
}
