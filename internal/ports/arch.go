package ports

// ArchStore persists and retrieves content-addressed arch shards.
// Shards are pure cache: always re-derivable from the fact store.
//
// C3 bucket contract: arch_shards bucket carries a _version byte written on
// creation; version mismatch → drop-and-re-create; missing bucket → empty
// result (old binary opening a new DB simply ignores the bucket).
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
}
