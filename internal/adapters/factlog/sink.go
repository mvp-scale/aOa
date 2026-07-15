// Package factlog implements ports.FactSink as an append-only JSONL writer to
// {root}/.aoa/facts/pending.jsonl (FDN-1, board #27; spec
// playbook/integration/01-facts-substrate.md §3.1, D5).
//
// The parse pass never holds a bbolt write transaction and never blocks on
// file I/O per fact: Emit only appends to an in-memory buffer; disk writes
// happen in Flush, off the hot path (§5 perf budget — buffered 64KB, flush
// off-loop).
package factlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/corey/aoa/internal/ports"
)

// bufferSize is the write-buffer capacity for the JSONL sink (§5: "sink
// buffers 64KB, flushes off-loop").
const bufferSize = 64 * 1024

// Sink implements ports.FactSink, appending one JSON line per Fact to
// {root}/.aoa/facts/pending.jsonl. Safe for concurrent use.
type Sink struct {
	mu   sync.Mutex
	w    *bufio.Writer
	f    *os.File
	path string
}

// New creates (or appends to) the pending facts JSONL file under root's
// .aoa/facts/ directory, creating the directory if it does not exist.
func New(root string) (*Sink, error) {
	dir := filepath.Join(root, ".aoa", "facts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("factlog: create %s: %w", dir, err)
	}
	path := filepath.Join(dir, "pending.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("factlog: open %s: %w", path, err)
	}
	return &Sink{
		w:    bufio.NewWriterSize(f, bufferSize),
		f:    f,
		path: path,
	}, nil
}

// Emit appends one JSON-encoded Fact line to the in-memory buffer. Never
// touches the filesystem directly (bufio only spills on its own overflow) —
// callers must call Flush to guarantee durability. A fact that fails to
// marshal (should never happen for the Fact type) is silently dropped
// rather than panicking the parse pass. Implements ports.FactSink.
func (s *Sink) Emit(f ports.Fact) {
	data, err := json.Marshal(f)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.w.Write(data)
	_ = s.w.WriteByte('\n')
}

// Flush writes any buffered lines to disk and fsyncs the underlying file.
// Callers invoke this off the parse-pass hot path (end of a build, or a
// debounced watcher tick). Implements ports.FactSink.
func (s *Sink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.w.Flush(); err != nil {
		return fmt.Errorf("factlog: flush %s: %w", s.path, err)
	}
	return s.f.Sync()
}

// Truncate resets the pending file to empty after a successful compact tx
// (§3: "truncates the JSONL"). Safe to call while the sink remains open for
// further Emit calls.
func (s *Sink) Truncate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.w.Flush(); err != nil {
		return fmt.Errorf("factlog: flush before truncate %s: %w", s.path, err)
	}
	if err := s.f.Truncate(0); err != nil {
		return fmt.Errorf("factlog: truncate %s: %w", s.path, err)
	}
	if _, err := s.f.Seek(0, 0); err != nil {
		return fmt.Errorf("factlog: seek %s: %w", s.path, err)
	}
	return nil
}

// Close flushes any buffered lines and closes the underlying file.
func (s *Sink) Close() error {
	if err := s.Flush(); err != nil {
		return err
	}
	return s.f.Close()
}

// NullSink is a no-op ports.FactSink for light builds (--light has no
// tree-sitter, so no facts are ever emitted — §4.3). Zero value is ready to
// use.
type NullSink struct{}

// Emit does nothing. Implements ports.FactSink.
func (NullSink) Emit(ports.Fact) {}

// Flush does nothing and never errors. Implements ports.FactSink.
func (NullSink) Flush() error { return nil }
