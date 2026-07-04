package ports

// SourceRef is the ports-layer DTO for a file:line attribution point.
// Mirrors arch.SourceRef — defined here so adapters need not import domain/arch.
// JSON tags are intentionally identical to arch.SourceRef for bucket compatibility.
type SourceRef struct {
	File string `json:"file"`
	Line uint32 `json:"line"`
}

// Finding is the ports-layer DTO for an arch detector finding.
// Mirrors arch.Finding — defined here so adapters need not import domain/arch.
// JSON tags are intentionally identical to arch.Finding for bucket compatibility:
// an existing facts_findings bucket written with arch.Finding bytes round-trips
// cleanly through ports.Finding (same field names, same omitempty rules).
type Finding struct {
	ID          string            `json:"id"`
	Rule        string            `json:"rule"`
	Severity    string            `json:"severity"`
	Scope       string            `json:"scope"`
	Message     string            `json:"message"`
	Subjects    []string          `json:"subjects"`
	Sources     []SourceRef       `json:"sources"`
	CheapestCut string            `json:"cheapestCut,omitempty"`
	Attrs       map[string]string `json:"attrs,omitempty"`
	New         bool              `json:"new,omitempty"`
}

// ArchStore persists and retrieves content-addressed arch shards and findings.
// Shards and findings are pure cache: always re-derivable from the fact store.
//
// C3 bucket contract: arch_shards and facts_findings buckets each carry a
// _version byte written on creation; version mismatch → drop-and-re-create;
// missing bucket → empty result (old binary ignores the bucket).
//
// C1 rule: ALL write methods (Save*, Delete*) must NOT be called while
// App.mu is held. Design the caller to snapshot state, release the mutex,
// then call the write method (snapshot-release-write pattern).
type ArchStore interface {
	// SaveShards atomically writes a batch of rendered shard JSON blobs.
	// Each key must be formatted as "{scope}/{view}@{hash}" where hash is the
	// 12-char ContentHash of the blob (byte-stable, matches the ETag contract).
	// Existing keys with the same scope prefix are NOT deleted — callers pass
	// the complete set for the scope and rely on manifest for the current view list.
	// C1: caller must NOT hold App.mu.
	SaveShards(projectID string, shards map[string][]byte) error

	// LoadShard returns the raw JSON bytes for a shard by key.
	// Key format: "{scope}/{view}@{hash}". Returns nil, nil if not found.
	LoadShard(projectID, key string) ([]byte, error)

	// SaveManifest writes the manifest JSON for a scope, keyed by scope alone.
	// The manifest is always small (<4 KB) and is the ETag anchor for the viewer.
	// C1: caller must NOT hold App.mu.
	SaveManifest(projectID, scope string, data []byte) error

	// LoadManifest returns the manifest JSON for a scope.
	// Returns nil, nil if no manifest has been written yet.
	LoadManifest(projectID, scope string) ([]byte, error)

	// DeleteShardsForScope removes all shard entries whose key starts with
	// "{scope}/" AND the manifest for that scope. Idempotent.
	// C1: caller must NOT hold App.mu.
	DeleteShardsForScope(projectID, scope string) error

	// HasArchBucket reports whether the arch_shards bucket exists and carries
	// the correct schema version for the project. Read-only (db.View). C1 n/a.
	HasArchBucket(projectID string) bool

	// SaveFindings persists arch detector findings for a (projectID, scope) pair.
	// Findings are pure cache — always re-derivable from edges + detectors.
	// C1: caller must NOT hold App.mu (snapshot-release-write pattern).
	// C3: bucket carries _version byte; version mismatch → drop-and-re-derive.
	SaveFindings(projectID, scope string, findings []Finding) error

	// LoadFindings retrieves arch findings for a (projectID, scope) pair.
	// Returns nil, nil if no findings exist or the bucket is absent/mismatched (C3).
	LoadFindings(projectID, scope string) ([]Finding, error)
}

// ArchManifest is the top-level catalog of all rendered shards for one scope.
// It is the ETag anchor: a new manifest rev means at least one shard changed.
// JSON-serialised and stored in the arch_shards bucket under the scope key.
type ArchManifest struct {
	// Scope is the project scope (e.g. "local").
	Scope string `json:"scope"`
	// Rev is a 12-char hash derived from the combined facts input (units + deps).
	// Stable across re-renders of the same input; changes when facts change.
	Rev string `json:"rev"`
	// Views is the ordered list of rendered views (component, dsm, cycles, …).
	// Ordered by view ID (alphabetical) for byte-stability.
	Views []ArchViewEntry `json:"views"`
}

// ArchViewEntry is one row in the manifest — one rendered view.
type ArchViewEntry struct {
	// ID is the short view identifier (e.g. "component", "dsm", "cycles").
	ID string `json:"id"`
	// Key is the full bbolt key: "{scope}/{id}@{hash}".
	Key string `json:"key"`
	// Hash is the 12-char ContentHash of the shard JSON.
	// Used by the viewer as an immutable cache key (?v=<hash>).
	Hash string `json:"hash"`
	// Caption is the human-readable summary count string (from DeriveCaption).
	Caption string `json:"caption"`
	// Prov is the provenance kind: "derived" | "mixed" | "simulated".
	Prov string `json:"prov"`
}

// ArchQuerier is the high-level read interface for the arch service, served
// by the app layer wrapping domain/arch.Service. Returns nil when the arch
// flag is disabled (C4: no nil-dereference in callers — check before use).
//
// Implementations must be safe for concurrent use. Write paths (derive, etc.)
// must follow C1: snapshot-release-write, never under App.mu.
type ArchQuerier interface {
	// Manifest returns the current view catalog for the given scope.
	// Returns nil, nil when no shards have been derived yet.
	Manifest(scope string) (*ArchManifest, error)

	// View returns the raw JSON bytes of a rendered shard by scope + view ID.
	// Returns nil, nil when the view has not been rendered.
	View(scope, id string) ([]byte, error)

	// Findings returns the JSON-encoded []Finding for a scope.
	// Returns nil, nil when no findings have been computed.
	Findings(scope string) ([]byte, error)

	// Derive returns the shortest dep-path from "from" to "to" (unit IDs),
	// limited to k hops. Returns nil if no path exists within the hop budget.
	Derive(scope, from, to string, k int) ([]string, error)

	// Facts returns the JSON-encoded import-edge provenance trail for a subject.
	// Subject is matched as a substring against both FromFile and ImportPath.
	// Returns nil, nil when no edges match or no edge store is available.
	// limit ≤ 0 means unlimited.
	Facts(scope, subject string, limit int) ([]byte, error)

	// Graph returns the raw substrate knowledge graph as JSON for the Terrain tab.
	// grain="file": nodes are distinct source files and resolved import targets;
	//               edges carry G7 StartLine provenance.
	// grain="unit": package-directory aggregation (same grain as deriveArch).
	// SIZE GUARD (server-side, honest): if grain="file" would exceed 20,000 edges,
	// the response is automatically downgraded to grain="unit" and a "downgraded"
	// field is populated — never a silent truncation (no-silent-caps law).
	// Returns nil, nil when no edges exist or no store is available (C4 safe).
	Graph(scope string, grain string) ([]byte, error)
}

// GraphNode is one node in the substrate knowledge graph payload.
// Used by the Terrain tab for force-directed rendering.
type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
	Ext   bool   `json:"ext,omitempty"`
	Line  uint32 `json:"line,omitempty"` // first-occurrence line (file grain)
}

// GraphEdge is one directed edge in the substrate knowledge graph payload.
// Count is only populated for unit grain; File and Line carry G7 provenance.
type GraphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Count int    `json:"count,omitempty"` // unit grain: aggregated import count
	File  string `json:"file,omitempty"`  // G7: source file for this edge
	Line  uint32 `json:"line,omitempty"`  // G7: import statement line
}

// GraphPayload is the substrate knowledge graph response for /api/arch/graph.
// Fields are ordered for byte-stable JSON (grain→rev→downgraded→nodes→edges).
// Nodes and edges are sorted deterministically (by ID / by from+to).
type GraphPayload struct {
	Grain      string      `json:"grain"`
	Rev        string      `json:"rev"`
	Downgraded string      `json:"downgraded,omitempty"`
	Nodes      []GraphNode `json:"nodes"`
	Edges      []GraphEdge `json:"edges"`
}
