package ports

// ImportEdge records a single import statement found in a source file.
// Every field is mandatory — no zero-value edge may be emitted (G7: truth is stamped).
//
// FromFile is a project-relative path (e.g., "internal/app/app.go").
// ImportPath is the raw import spec string as it appears in the source
// (e.g., "fmt", "os/exec", "react", "numpy"). It is NOT resolved or classified
// at extraction time; resolution to intra-repo unit ID, "ext:" facts, or
// facts_unresolved happens in the App layer via domain/facts.Resolve (§2.4):
// doFlushEdgeBatch for watcher events; WarmCaches/Reindex for bulk paths.
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

	// HasEdgesBucket reports whether the edges bucket exists and carries the
	// correct schema version for the project. Used by WarmCaches to detect
	// upgrade-boot scenarios (T43): populated index + no valid edges bucket +
	// arch flag ON → background Reindex backfills edges without a manual step.
	// Read-only: uses db.View, never db.Update. C1 does not apply.
	HasEdgesBucket(projectID string) bool
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

// ---------------------------------------------------------------------------
// FDN-1 (board #27): the universal facts substrate, lifted near-verbatim from
// playbook/integration/01-facts-substrate.md §1/§3 (D1-D8). Additive only —
// nothing above this line is touched. FactParser above pre-dates this spec
// (D25: reconciliation of UnitFact/DepFact with ports.Fact is FDN-4 work, not
// this one) and is deliberately left alone; a widened FactParser returning
// []Fact is out of scope for FDN-1.
// ---------------------------------------------------------------------------

// FactKind enumerates the eight fact kinds (D1). String-typed for stable
// JSONL serialization and forward compatibility — unknown kinds are
// skippable on read (01-facts-substrate.md:50-63).
type FactKind string

const (
	FactUnit    FactKind = "unit"    // a module/package/component (the node grain)
	FactDep     FactKind = "dep"     // import/include/require edge (the keystone)
	FactRoute   FactKind = "route"   // HTTP/RPC endpoint exposure (Phase ③)
	FactSchema  FactKind = "schema"  // entity/table/migration shape (Phase ③)
	FactDeploy  FactKind = "deploy"  // container/k8s/compose topology (Phase ③)
	FactOwner   FactKind = "owner"   // CODEOWNERS / git authorship (Phase ③)
	FactDelta   FactKind = "delta"   // change vs a baseline ref (§4)
	FactFinding FactKind = "finding" // detector output: cycle, god, orphan (§4.4)
)

// Provenance is the honesty stamp from the scope-line ADR ladder (D2):
// .context/decisions/2026-06-11-core-competence-and-scope-line.md — layer 1
// derive=REAL, layer 2 infer=MIXED, layer 3 declare/ingest.
type Provenance string

const (
	ProvDerived  Provenance = "derived"  // REAL — tree-sitter / git / manifest
	ProvInferred Provenance = "inferred" // MIXED — agent named/grouped, never added
	ProvDeclared Provenance = "declared" // human declaration (.aoa/arch.yaml)
	ProvObserved Provenance = "observed" // ingested external truth (APM etc.)
)

// FactSource is the audit pointer (D3). Every fact carries one; this is what
// makes evidence packs auditable (ENHANCEMENT-GUIDE §2).
type FactSource struct {
	File   string `json:"f"`           // repo-relative path
	Line   uint32 `json:"l"`           // 1-based; 0 = whole-file fact
	Commit string `json:"c,omitempty"` // short hash at emission time
}

// Fact is the universal record (D4). Subject/Object are canonical IDs
// (`<ns>:<path>`, D7). Attrs is small and bounded (≤8 keys); large payloads
// are a design error.
type Fact struct {
	Kind    FactKind          `json:"k"`
	Subject string            `json:"s"`
	Object  string            `json:"o,omitempty"`
	Attrs   map[string]string `json:"a,omitempty"`
	Source  FactSource        `json:"src"`
	Prov    Provenance        `json:"p"`
	TS      int64             `json:"t,omitempty"` // unix seconds, set at emission
}

// DepEdge is one resolved unit-grain edge with evidence count.
type DepEdge struct {
	Unit  string // the other endpoint
	Count uint16 // number of file-grain import sites backing this edge
}

// DepAdjacency is the compactor's resolved graph (forward + reverse).
type DepAdjacency struct {
	Fwd map[string][]DepEdge
	Rev map[string][]DepEdge
}

// BaselineEdge is a sorted, deduped (subject, object) pair frozen into a
// FactBaseline.
type BaselineEdge struct{ S, O string }

// FactBaseline is a frozen snapshot for delta/conformance (§4.2, the ArchUnit
// pattern).
type FactBaseline struct {
	Ref       string // git ref or user-chosen name
	Commit    string // resolved short hash
	CreatedAt int64
	Units     []string
	Edges     []BaselineEdge // sorted, deduped: subject, object
	Findings  []string       // stable finding keys (rule|subject)
}

// FactSink receives facts during the parse pass. Implementations MUST be
// O(1) amortized per call and must never block the parse loop (buffered
// append; flush happens off the hot path). Adapter: internal/adapters/factlog
// (JSONL writer).
type FactSink interface {
	Emit(f Fact)
	Flush() error
}

// FactStore is the durable, queryable substrate (§3). Adapter:
// internal/adapters/bbolt (same DB file, new sub-buckets). All methods are
// project-scoped, mirroring ports.Storage (storage.go:12-56). Writes are
// transactional.
//
// C3 bucket contract (same standing rule as EdgeStore above): every fact
// bucket carries a `_version` byte, checked on open; a version mismatch
// drops and re-creates the bucket (facts are re-derivable cache, D10).
type FactStore interface {
	// ReplaceFactsForFile atomically swaps all raw facts attributed to one
	// file (the incremental unit of work, §4.1). Empty facts slice is a pure
	// delete.
	ReplaceFactsForFile(projectID, path string, facts []Fact) error

	// PutResolved writes compactor output: unit records + adjacency. Overwrites.
	PutResolved(projectID string, units []Fact, adj *DepAdjacency) error

	FactsByKind(projectID string, kind FactKind) ([]Fact, error)
	FactsForSubject(projectID, subject string) ([]Fact, error)

	// O(1) bucket get + one posting-list decode each (§3.2, §5).
	Dependencies(projectID, unit string) ([]DepEdge, error) // unit → its imports
	Dependents(projectID, unit string) ([]DepEdge, error)   // who imports unit

	SaveBaseline(projectID, name string, b *FactBaseline) error
	LoadBaseline(projectID, name string) (*FactBaseline, error) // nil,nil if absent

	DeleteProjectFacts(projectID string) error // wired into `aoa remove` / `aoa reset`
}
