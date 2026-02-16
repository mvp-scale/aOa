// Package ports defines the interfaces (contracts) that adapters must implement.
// These are the boundaries of the hexagonal architecture. Domain logic depends
// only on these interfaces, never on concrete implementations.
package ports

// Storage persists index and learner state to durable storage.
// The backing store (bbolt) is project-scoped: each projectID gets its own
// namespace. Concurrent reads are safe; writes are serialized by the adapter.
//
// Crash safety: SaveIndex and SaveLearnerState must be transactional.
// A crash mid-write must not corrupt previously committed data.
type Storage interface {
	// SaveIndex persists the full search index for a project.
	// Overwrites any prior index for this projectID.
	SaveIndex(projectID string, index *Index) error

	// LoadIndex retrieves the search index for a project.
	// Returns nil, nil if no index exists (fresh project).
	LoadIndex(projectID string) (*Index, error)

	// SaveLearnerState persists the full learner state for a project.
	// Called after every autotune cycle. Overwrites prior state.
	SaveLearnerState(projectID string, state *LearnerState) error

	// LoadLearnerState retrieves learner state for a project.
	// Returns nil, nil if no state exists (fresh project).
	LoadLearnerState(projectID string) (*LearnerState, error)

	// DeleteProject removes all data (index + learner state) for a project.
	// Idempotent: deleting a nonexistent project is not an error.
	DeleteProject(projectID string) error
}

// Index represents the searchable code index
type Index struct {
	Tokens   map[string][]TokenRef     // token -> locations
	Metadata map[TokenRef]*SymbolMeta  // (file_id, line) -> symbol info
	Files    map[uint32]*FileMeta      // file_id -> file info
}

// TokenRef is a compact reference to a code location
type TokenRef struct {
	FileID uint32
	Line   uint16
}

// SymbolMeta contains symbol information
type SymbolMeta struct {
	Name      string
	Signature string
	Kind      string
	StartLine uint16
	EndLine   uint16
	Parent    string
	Tags      []string // pre-computed display tags
}

// FileMeta contains file metadata
type FileMeta struct {
	Path         string
	LastModified int64
	Language     string
	Domain       string // assigned semantic domain (e.g., "@authentication")
}

// LearnerState holds all domain learning state for a project.
// This is the complete in-memory representation, persisted to bbolt
// after every autotune cycle.
//
// All hit maps use uint32 counts. During decay, Python applies
// int(float(count) * 0.90) — truncating toward zero. Go must match
// this behavior exactly for parity.
//
// Exception: DomainMeta.Hits is float64, NOT int-truncated during decay.
type LearnerState struct {
	KeywordHits      map[string]uint32      `json:"keyword_hits"`
	TermHits         map[string]uint32      `json:"term_hits"`
	DomainMeta       map[string]*DomainMeta `json:"domain_meta"`
	CohitKwTerm      map[string]uint32      `json:"cohit_kw_term"`
	CohitTermDomain  map[string]uint32      `json:"cohit_term_domain"`
	Bigrams          map[string]uint32      `json:"bigrams"`
	FileHits         map[string]uint32      `json:"file_hits"`
	KeywordBlocklist map[string]bool        `json:"keyword_blocklist"`
	GapKeywords      map[string]bool        `json:"gap_keywords"`
	PromptCount      uint32                 `json:"prompt_count"`
}

// DomainMeta holds per-domain metadata and lifecycle state.
//
// Hits is the decayed hit counter (float64, NOT int-truncated).
// TotalHits is lifetime accumulator (never decayed, for promotion decisions).
// HitsLastCycle is snapshot from previous autotune (for stale detection).
type DomainMeta struct {
	Hits          float64 `json:"hits"`            // Decayed hit count — stored as float, NOT truncated
	TotalHits     uint32  `json:"total_hits"`      // Lifetime hits — never decayed
	Tier          string  `json:"tier"`            // "core" or "context"
	Source        string  `json:"source"`          // "seeded", "learned", "skeleton", "intent"
	State         string  `json:"state"`           // "active", "stale", "deprecated"
	StaleCycles   uint32  `json:"stale_cycles"`    // Consecutive autotune cycles with 0 hits
	HitsLastCycle float64 `json:"hits_last_cycle"` // Snapshot from previous cycle (for stale detection)
	LastHitAt     uint32  `json:"last_hit_at"`     // Prompt count when domain was last hit
	CreatedAt     int64   `json:"created_at"`      // Unix timestamp of domain creation
}

// SearchOptions controls grep/egrep behavior. Passed through the search path
// to support all Unix grep parity flags documented in CLAUDE.md.
type SearchOptions struct {
	Mode         string // "literal" (default), "regex", "case_insensitive"
	AndMode      bool   // -a: intersection of comma-separated terms
	WordBoundary bool   // -w: match whole words only
	CountOnly    bool   // -c: return count, not results
	Quiet        bool   // -q: exit code only, no output
	MaxCount     int    // -m: limit results (default 20)
	IncludeGlob  string // --include: file pattern filter
	ExcludeGlob  string // --exclude: file pattern filter
	Since        int64  // --since: unix timestamp, files modified after
	Before       int64  // --before: unix timestamp, files modified before
}
