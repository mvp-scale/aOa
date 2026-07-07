package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/peek"
	"github.com/corey/aoa/internal/ports"
)

// IndexQuerier returns a ports.IndexQuerier backed by this app's live index.
// Safe for concurrent use: only reads from the index (no writes).
func (a *App) IndexQuerier() ports.IndexQuerier {
	if a.Index == nil || a.searcher == nil {
		return nil
	}
	return &appIndexQuerier{
		idx:      a.Index,
		searcher: a.searcher,
		root:     a.ProjectRoot,
	}
}

// appIndexQuerier implements ports.IndexQuerier using the live index.
type appIndexQuerier struct {
	idx      *ports.Index
	searcher ports.Searcher
	root     string
}

// Peek resolves peek codes to source bodies.
// Mirrors socket.Server.handlePeek exactly — same index, same logic, same source of truth.
func (q *appIndexQuerier) Peek(codes []string) ([]ports.PeekHit, error) {
	result := make([]ports.PeekHit, len(codes))
	for i, code := range codes {
		result[i].Code = code

		fileID, startLine, err := peek.Decode(code)
		if err != nil {
			result[i].Error = fmt.Sprintf("invalid peek code: %v", err)
			continue
		}

		ref := ports.TokenRef{FileID: fileID, Line: startLine}
		sym := q.idx.Metadata[ref]
		if sym == nil {
			result[i].Error = "symbol not found"
			continue
		}
		file := q.idx.Files[fileID]
		if file == nil {
			result[i].Error = "file not found"
			continue
		}

		domain, tags := q.searcher.EnrichRef(ref)
		result[i].File = file.Path
		result[i].Symbol = sym.Name
		result[i].Signature = sym.Signature
		result[i].Span = [2]int{int(sym.StartLine), int(sym.EndLine)}
		result[i].Domain = domain
		result[i].Tags = tags

		// Read source lines from disk
		absPath := filepath.Join(q.root, file.Path)
		data, err := os.ReadFile(absPath)
		if err != nil {
			result[i].Error = fmt.Sprintf("read file: %v", err)
			continue
		}
		allLines := strings.Split(string(data), "\n")
		start := int(sym.StartLine) - 1 // 0-indexed
		end := int(sym.EndLine)          // exclusive
		if start < 0 {
			start = 0
		}
		if end > len(allLines) {
			end = len(allLines)
		}
		result[i].Body = strings.Join(allLines[start:end], "\n")
	}
	return result, nil
}

// Refs returns up to k posting-list entries for the given token.
// O(k) not O(corpus): at most k refs are read from the posting list.
func (q *appIndexQuerier) Refs(token string, k int) ports.RefsResult {
	refs := q.idx.Tokens[token]
	total := len(refs)
	truncated := false
	if k > 0 && total > k {
		refs = refs[:k]
		truncated = true
	}
	hits := make([]ports.RefHit, 0, len(refs))
	for _, ref := range refs {
		file := q.idx.Files[ref.FileID]
		if file == nil {
			continue
		}
		hit := ports.RefHit{
			File: file.Path,
			Line: int(ref.Line),
		}
		// Include symbol name and peek code if this ref is a known symbol
		if sym := q.idx.Metadata[ref]; sym != nil {
			hit.Symbol = sym.Name
			// Only include peek code if span <= MaxRange
			if int(sym.EndLine)-int(sym.StartLine) <= peek.MaxRange {
				hit.Peek = peek.Encode(ref.FileID, ref.Line)
			}
		}
		hits = append(hits, hit)
	}
	return ports.RefsResult{
		Token:     token,
		Total:     total,
		Refs:      hits,
		Truncated: truncated,
	}
}
