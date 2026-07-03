package ports

// ImportEdge records a single import statement found in a source file.
// Every field is mandatory — no zero-value edge may be emitted (G7: truth is stamped).
//
// FromFile is a project-relative path (e.g., "internal/app/app.go").
// ImportPath is the raw import spec string as it appears in the source
// (e.g., "fmt", "os/exec", "react", "numpy"). It is NOT resolved or classified
// at extraction time; resolution to intra-repo unit ID, "ext:" facts, or
// facts_unresolved happens in the EdgeStore write path (L19.10).
// StartLine is the 1-based line number of the import statement in FromFile (G7 provenance).
type ImportEdge struct {
	FromFile   string // relative file path — never absolute (G7)
	ImportPath string // raw import spec, unresolved
	StartLine  uint32 // 1-based line number in FromFile (G7)
}

// EdgeStore persists and retrieves import edges keyed by file.
// All methods are project-scoped (projectID). Implementations must be C1-compliant:
// no db.Update while App.mu is held — callers must write outside the mutex.
//
// C3 bucket contract (standing rule — applies to every new bucket):
//   - `_version` byte is written on bucket creation and checked on open.
//   - New binary opening a DB without an edges bucket: CreateBucketIfNotExists
//     succeeds silently (no error surfaced to the caller).
//   - New binary opening a DB with a wrong-version edges bucket: drop + re-create
//     (edges are pure cache, always re-derivable from a full Reindex).
//   - An old binary (no EdgeStore code) opening a DB with an edges bucket: the
//     bucket is simply ignored — the old binary has no code to read it.
type EdgeStore interface {
	// SaveEdgesForFile replaces all edges for a single file.
	// Overwrites any prior edges for fileID. Thread-safe via bbolt transactions.
	SaveEdgesForFile(projectID string, fileID uint32, edges []ImportEdge) error

	// SaveEdgesBatch writes all accumulated file-edge deltas in a single bbolt
	// write transaction (C2 burst coalescing, L19.12). Each entry in fileEdges
	// is processed atomically inside one tx:
	//   - len(edges) > 0  → bucket.Put (overwrite / save)
	//   - len(edges) == 0 → bucket.Delete (remove stale entry for deleted file)
	// All-or-nothing: the tx is rolled back on any error.
	// C1: caller must NOT hold App.mu.
	SaveEdgesBatch(projectID string, fileEdges map[uint32][]ImportEdge) error

	// LoadEdgesForFile returns all edges for a single file.
	// Returns nil, nil if no edges exist for that file or bucket.
	LoadEdgesForFile(projectID string, fileID uint32) ([]ImportEdge, error)

	// DeleteEdgesForFile removes all edges for a single file.
	// O(edges-for-file) — one bucket.Delete call per fileID.
	// Idempotent: deleting a nonexistent file's edges is not an error.
	DeleteEdgesForFile(projectID string, fileID uint32) error

	// LoadAllEdges returns every edge stored for the project.
	// Returns nil, nil if no edges exist.
	LoadAllEdges(projectID string) ([]ImportEdge, error)

	// SaveUnresolved persists import specs that appear intra-repo (relative or
	// module-prefixed) but did not resolve to any file in the current index.
	// These are findings fuel: broken-import candidates that re-resolve cheaply
	// when a matching file appears (§2.4 unresolved handling).
	// Keyed by ImportPath+"\x00"+FromFile+"\x00"+StartLine — idempotent Put.
	// C1: caller must NOT hold App.mu.
	SaveUnresolved(projectID string, entries []ImportEdge) error

	// ReplaceAllEdges atomically clears the entire edges bucket for the project
	// and writes the provided file→edges map in a single bbolt write transaction.
	// Use this for WarmCaches and Reindex: stale file IDs from the previous build
	// (deleted or renumbered files) are eliminated before new data is committed
	// (finding 9 / T34). Passing nil or empty fileEdges still clears the bucket
	// (safe reset, e.g., arch disabled after first boot).
	// All-or-nothing: the tx is rolled back on any error.
	// C1: caller must NOT hold App.mu.
	ReplaceAllEdges(projectID string, fileEdges map[uint32][]ImportEdge) error
}

// FactParser is the extended parser interface that extracts both symbols and
// import edges from a source file in a single tree-sitter parse pass (G0:
// one traversal per file). Adapters that implement this interface register
// themselves for the C4 arch extraction path.
//
// ParseFileToMetaAndFacts parses the file once and returns:
//   - []*SymbolMeta: same output as ParseFileToMeta (zero regression)
//   - []ImportEdge:  one entry per top-level import statement, with G7 provenance
//   - error:         parse-level error; returns nil, nil, nil for unknown languages
//
// Callers MUST type-assert before using: `if fp, ok := p.(FactParser); ok { ... }`.
// Parsers that do not implement FactParser continue to work via ParseFileToMeta.
type FactParser interface {
	ParseFileToMetaAndFacts(path string, source []byte) ([]*SymbolMeta, []ImportEdge, error)
}
