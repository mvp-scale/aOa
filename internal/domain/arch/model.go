// Package arch implements the dependency-free rendition domain for F2 (L19.14/L19.15).
//
// This package consumes unit/dep facts and emits content-hashed shard JSON
// matching the 5-kind viewer contract. It is stdlib-only: no bbolt, no cobra,
// no socket imports (G4 hexagonal law).
//
// Type provenance note: raw import edges are ports.ImportEdge (F1 merged).
// UnitFact and DepFact remain local until ports.Fact (spec 01 FactStore) lands in F2.
// See the TODO comments on each type.
package arch

// SourceRef carries file:line attribution (G7 — every shard node/finding carries file:line).
type SourceRef struct {
	File string `json:"file"`
	Line uint32 `json:"line"`
}

// UnitFact is a provisional unit fact used before ports.FactStore lands.
// TODO: reconcile with ports.Fact{Kind:"unit"} when spec-01 FactStore merges.
// (F1 brought ports.ImportEdge/EdgeStore/FactParser — ports.Fact is a later addition.)
type UnitFact struct {
	ID     string     // e.g. "m_internal_domain_arch" — deterministic slug
	Label  string     // display label; ≤30 chars (view-standards budget)
	Path   string     // canonical import path / repo-relative directory
	File   string     // defining file (G7 source pointer)
	Line   uint32     // defining line (G7 source pointer)
	Domain string     // atlas domain (for grouping rung-3)
}

// DepFact is a provisional dependency edge used before ports.FactStore lands.
// Represents an aggregated unit→unit dependency (N raw import statements collapsed).
// TODO: reconcile with ports.Fact{Kind:"dep"} when spec-01 FactStore merges.
// (F1 brought ports.ImportEdge/EdgeStore/FactParser — ports.Fact is a later addition.)
type DepFact struct {
	FromUnit string // source unit ID
	ToUnit   string // target unit ID
	Count    int    // number of import statements backing this edge
	File     string // G7: representative file containing an import
	Line     uint32 // G7: representative line
}

// Prov records provenance of a shard.
// Three valid kinds: "derived", "mixed", "simulated".
// Mixing rule: min over contributing elements (derived > mixed > simulated).
type Prov struct {
	Kind  string `json:"kind"`  // "derived" | "mixed" | "simulated"
	Label string `json:"label"` // human string e.g. "REAL · derived from code"
}

// Shard is the top-level output of any renderer.
// JSON shape is byte-compatible with the existing viewer contract
// (build_c4_mockup.py:431-441, 493-499).
//
// Fields are ordered to match the common header + kind-specific body layout
// used by the playbook viewer. All slices use omitempty so unused fields
// are absent from JSON output (not null), keeping outputs small and
// kind-clearly-shaped.
type Shard struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Count   string `json:"count"`
	Dir     string `json:"dir,omitempty"`
	Prov    Prov   `json:"prov"`

	// buckets kind
	Buckets []Bucket    `json:"buckets,omitempty"`
	Edges   []ShardEdge `json:"edges,omitempty"`

	// matrix kind
	Items  []string        `json:"items,omitempty"`
	Matrix [][]interface{} `json:"matrix,omitempty"`

	// table kind
	Columns []string   `json:"columns,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`

	// simple / entity kind
	Nodes []Node `json:"nodes,omitempty"`
}

// Bucket represents a group in component/dsm/domains views (buckets kind).
type Bucket struct {
	ID       string   `json:"id"`
	Layer    string   `json:"layer,omitempty"`
	Label    string   `json:"label"`
	Part     int      `json:"part"`
	Boundary *bool    `json:"boundary,omitempty"`
	Ico      string   `json:"ico,omitempty"`
	Members  []Member `json:"members"`
}

// Member is an element inside a Bucket.
type Member struct {
	ID      string      `json:"id"`
	Label   string      `json:"label"`
	Sub     string      `json:"sub,omitempty"`
	Sources []SourceRef `json:"sources,omitempty"` // G7: file:line evidence
}

// Node is an element in simple/entity views.
type Node struct {
	ID      string      `json:"id"`
	Type    string      `json:"type,omitempty"`
	Label   string      `json:"label"`
	Sub     string      `json:"sub,omitempty"`
	Real    bool        `json:"real"`
	DrillTo string      `json:"drillTo,omitempty"`
	Sources []SourceRef `json:"sources,omitempty"` // G7: file:line evidence
}

// ShardEdge connects two elements in buckets/simple/entity views.
type ShardEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Count  int    `json:"count,omitempty"`
	Label  string `json:"label,omitempty"`
	Tag    string `json:"tag,omitempty"`
}

// Finding is produced by detectors and carried into renderers.
// Message phrasing mirrors build_c4_mockup.py:817-829 so dock text is identical.
type Finding struct {
	ID          string            `json:"id"`
	Rule        string            `json:"rule"`              // cycle|god|orphan|budget|dead-candidate|mutual|band|divergent|absent
	Severity    string            `json:"severity"`          // error|warn|info
	Scope       string            `json:"scope"`
	Message     string            `json:"message"`
	Subjects    []string          `json:"subjects"`
	Sources     []SourceRef       `json:"sources"`
	CheapestCut string            `json:"cheapestCut,omitempty"` // cycle findings only: "A → B (×N)" — reused by RenderCycles
	Attrs       map[string]string `json:"attrs,omitempty"`       // detector-specific key/value metadata (e.g. dead-candidate reflection caveat)
	New         bool              `json:"new,omitempty"`
}

// ThresholdOpts holds configurable detector thresholds (arch.yaml:thresholds).
type ThresholdOpts struct {
	GodIn  int // min fan-in to flag as god (default 3)
	GodOut int // min fan-out to flag as god (default 3)
}

// DefaultThresholds returns the default detector thresholds.
func DefaultThresholds() ThresholdOpts {
	return ThresholdOpts{GodIn: 3, GodOut: 3}
}

// GroupingResult maps each unit ID to its assigned group ID and group metadata.
type GroupingResult struct {
	// UnitGroup maps unit ID → group ID
	UnitGroup map[string]string
	// Groups is the ordered list of groups (stable, by part then id)
	Groups []GroupMeta
}

// GroupMeta holds display info for a single group bucket.
type GroupMeta struct {
	ID    string // stable slug e.g. "g_domain"
	Label string // display label e.g. "domain"
	Part  int    // band/layer order (lower = higher in diagram)
}

// RenderInput is the bundle passed to every renderer.
// All fields are pre-computed by the Service before calling Render.
type RenderInput struct {
	Scope     string
	Units     []UnitFact
	Deps      []DepFact
	Grouping  GroupingResult
	GroupProv string     // "derived" or "mixed" — propagated from grouping step
	SCCs      [][]string // pre-computed SCCs (from TarjanSCC); shared by cycles renderer
	Findings  []Finding  // detector output for this scope

	// CodeSymbols carries per-file symbol data for the code renderer (②b, L19.23).
	// Nil → code view omitted (conditional registration per kickoff §22 item 22).
	// The app layer populates this from a Clone of ports.Index (never aliased — race gate).
	// Provenance split: REAL for symbol file:line; MIXED for subset selection heuristic.
	CodeSymbols *CodeSymbolIndex
}

// CodeSymbolIndex is a symbol data bundle for the code renderer (②b, L19.23).
// Built by the app layer from a Clone of ports.Index.
// The arch domain is dependency-free; this type decouples the renderer from ports.SymbolMeta.
type CodeSymbolIndex struct {
	// ByFile maps file path → symbols in that file, sorted by StartLine ascending.
	ByFile map[string][]CodeSymbol
}

// CodeSymbol is a single symbol translated from ports.SymbolMeta + TokenRef.
// Used exclusively by the code renderer; other renderers ignore it.
type CodeSymbol struct {
	Name      string
	Signature string
	Kind      string
	File      string
	StartLine uint16
	EndLine   uint16
	Parent    string
}

// Manifest is the top-level catalog of all rendered shards for one scope.
// Byte-stable: all slices sorted by ID; no timestamps inside.
// Rev is a 12-char hash of the sorted unit+dep inputs — changes when facts change.
type Manifest struct {
	Scope string      `json:"scope"`
	Rev   string      `json:"rev"`
	Views []ViewEntry `json:"views"`
}

// ViewEntry is one rendered-view entry in the Manifest.
type ViewEntry struct {
	ID      string `json:"id"`      // e.g. "component"
	Key     string `json:"key"`     // bbolt key: "{scope}/{id}@{hash}"
	Hash    string `json:"hash"`    // 12-char ContentHash of shard JSON
	Caption string `json:"caption"` // human summary from DeriveCaption
	Prov    string `json:"prov"`    // "derived" | "mixed" | "simulated"
}

// GroupOptions controls the three-rung grouping cascade.
// Nil or zero-value → rung-2 path-prefix only (default, always REAL/derived).
type GroupOptions struct {
	// Declarations (rung-1): declared group for a unit, keyed by unit ID.
	// Takes priority over path-prefix. From arch.yaml role→path mappings,
	// pre-resolved to unit IDs by the app layer before passing here.
	Declarations map[string]string // unitID → group label

	// Overlays (rung-overlay): group assignments from a validated overlay file.
	// Pre-parsed and leash-validated (invalid unit IDs removed) by the app layer.
	// If any overlay assignment was applied, GroupProv is set to "mixed".
	Overlays map[string]string // unitID → group label

	// OverlayHadInvalidIDs is true when the overlay file contained IDs that
	// were absent from the fact set (leash violation). The invalid IDs have
	// already been removed from Overlays; this flag drives warning-finding generation.
	OverlayHadInvalidIDs bool
}

// OverlaySpec is the on-disk schema for .aoa/arch/overlays/<scope>.json.
// Schema identifier: "aoa.arch-overlay/v1".
type OverlaySpec struct {
	Schema string             `json:"$schema"`
	Groups []OverlayGroupSpec `json:"groups"`
}

// OverlayGroupSpec is one group entry in an overlay file.
type OverlayGroupSpec struct {
	ID      string   `json:"id"`      // group label (e.g. "infra")
	Label   string   `json:"label"`   // display label
	UnitIDs []string `json:"unitIds"` // unit IDs to assign to this group
}

// provFromKind converts a groupProv kind string to a Prov value.
// Empty string → "derived" (REAL; default when no overlay applied).
func provFromKind(kind string) Prov {
	switch kind {
	case "mixed":
		return Prov{Kind: "mixed", Label: "MIXED · overlay applied"}
	default:
		return Prov{Kind: "derived", Label: "REAL · imports + deterministic grouping"}
	}
}
