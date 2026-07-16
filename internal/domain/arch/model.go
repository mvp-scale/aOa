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
	// Kind is the ontology concept token for this edge (e.g. "imports", "calls").
	// Additive; zero-value "" is treated as "imports" for backward compatibility.
	// TODO: not persistence-migration-safe yet (bbolt buckets predate this field).
	Kind string `json:"kind,omitempty"`
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
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Count string `json:"count"` // A3: the CALM caption — never carries a findings tail
	// FindingsClause is the "· ⚠ N findings"-shaped suffix DeriveCaption used to bake into Count.
	// Split out (A3 house ruling: calm like a map) so goldens/captions stay stable at derive time
	// and the viewer appends it only when the showFindings lens is on. Empty when there's nothing
	// to report.
	FindingsClause string `json:"findingsClause,omitempty"`
	Dir            string `json:"dir,omitempty"`
	Prov           Prov   `json:"prov"`

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
	Inferred bool     `json:"inferred"` // true when Layer/Part is inferred rather than declared — viewer suppresses band-violation findings when either endpoint is inferred. No omitempty: false is a real, meaningful value (future V2 declared-layer path) and Go's omitempty on bool drops false-valued fields entirely, which would silently defeat the viewer's strict sp.inferred===false gate.
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

	// Fields and Tech are entity-kind-only (COL-1, schema-collector): a
	// struct/table's column names and its source technology (e.g. "Go
	// struct"). Additive omitempty (D40 — no ArchSchemaVersion bump):
	// every non-entity Node leaves both zero-valued, so buckets/simple
	// consumers see no shape change on the wire.
	Fields []string `json:"fields,omitempty"`
	Tech   string   `json:"tech,omitempty"`
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
	ID       string // stable slug e.g. "g_domain"
	Label    string // display label e.g. "domain"
	Part     int    // band/layer order (lower = higher in diagram)
	Layer    string // canonical role/layer (core|edge|integration|data|external|supporting) → color pin
	Ico      string // icon key (hexagon|iface|plug|cylinder|cloud|gear)
	Inferred bool   // true when Layer/Part was inferred (roleFor heuristics) rather than declared by a V2 contract; always true today since no declared-layer input path exists yet — gates prescriptive findings (e.g. band violations) client-side
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

	// Components carries lockfile-derived dependency rows for the SBOM view
	// (VL-1a, board #35). Nil/empty → the view renders its honest "0
	// components" empty state (never a phantom shard — sbom/techportfolio/
	// glossary are mandatory views, unlike the conditional "code" view above).
	// Populated by the app layer from internal/adapters/lockfile readers.
	Components []Component

	// Technologies carries language/framework usage rows for the Tech Stack
	// (view id "techportfolio") view (VL-1b), joined from FileMeta.Language +
	// Components by the app layer.
	Technologies []TechEntry

	// GlossaryTerms carries atlas-harvested candidate term definitions for
	// the Glossary view (VL-1c). Always MIXED provenance (D2 — candidates,
	// harvested from keyword groupings, not ratified prose).
	GlossaryTerms []GlossaryEntry

	// ChurnEntries carries per-unit git-churn × complexity rows for the
	// Change Map view (VL-2, board #36). Nil/empty → the view renders its
	// honest "0 units" empty state (never a phantom shard — "change" is a
	// mandatory view). Populated by the app layer from a bounded git-log
	// read joined with indexed symbol counts (complexity proxy).
	ChurnEntries []ChurnEntry

	// Routes carries HTTP route-registration rows for the API Contract
	// (view id "api-contract") view (VL-3, board #37 — the first `route`-
	// kind fact, D1). Nil/empty → the view renders its honest "0 routes"
	// empty state (never a phantom shard — "api-contract" is a mandatory
	// view). Populated by the app layer from the treesitter route
	// extractor (net/http mux + gin idioms, Go only for v1).
	Routes []RouteEntry

	// Entities carries struct-entity rows for the Data Model / ER (view id
	// "datamodel") view (COL-1 — the first `entity`-kind fact, D1).
	// Nil/empty -> the view renders its honest "0 entities" empty state
	// (never a phantom shard — "datamodel" is a mandatory view). Populated
	// by the app layer from the treesitter schema extractor (Go structs
	// only for v1).
	Entities []EntityEntry

	// Deployments carries deploy-artifact rows for the Deployment (view id
	// "deployment") view (COL-2, board M6 — the first `deployment`-kind fact,
	// D1). Nil/empty -> the view renders its honest "0 artifacts" empty state
	// (never a phantom shard — "deployment" is a mandatory view). Populated
	// by the app layer from internal/adapters/deployfile readers
	// (Dockerfile/compose.yaml/Kubernetes manifests).
	Deployments []DeploymentEntry

	// OwnershipEntries carries COL-3's CODEOWNERS-parse + git-authorship rows
	// (board M6) for the Ownership (view id "ownership") view. Nil/empty ->
	// the view renders its honest "0 units with defined owners" empty state
	// (never a phantom shard — "ownership" is a mandatory view). Populated by
	// the app layer from internal/adapters/codeowners plus a bounded
	// git-authorship fallback.
	OwnershipEntries []OwnershipEntry

	// FileDomains carries the atlas domain vote for each file that scored one
	// (file path -> domain, DOM-1, board L22.23). Populated by the app layer
	// from index.SearchEngine.DeriveFileDomains() — this is the file-grain
	// result and is consumed ONLY by RenderDomains, which aggregates it up to
	// a proper unit-grain modal vote (per-directory, majority across every
	// file in the unit, ties broken lexicographically). This is deliberately
	// NOT the bridge's one-file shortcut (aggregateEdges/
	// unitFactsFromFactStore, which credits a unit with whichever single file
	// happened to define it first). Never written to UnitFact.Domain or
	// idx.Files[].Domain — rung-3 (grouping.go:189-194) stays dormant so
	// component/dsm/cycles are never silently regrouped (D35).
	FileDomains map[string]string
}

// Component is one detected dependency/component entry from a manifest
// reader (internal/adapters/lockfile — go.mod/go.sum, package.json). Mirrors
// lockfile.Component field-for-field; the app layer converts between the two
// at the boundary (D25 pattern), keeping this domain package import-free of
// the adapters layer (G4).
type Component struct {
	Name     string
	Version  string
	Supplier string // "direct" | "indirect" | "replace" | "dev" | "optional" | "peer"
	Language string // "go" | "js"
	Unpinned bool
	File     string
	Line     uint32
}

// TechEntry is one row in the Tech Stack (view id "techportfolio") view: a
// technology — a detected source language or a lockfile dependency — plus
// where/how much it's used.
type TechEntry struct {
	Name     string
	Kind     string // "language" | "dependency"
	Count    int    // files (language rows) or manifest occurrences (dependency rows)
	Unpinned bool   // dependency rows only; language rows always false
	File     string
	Line     uint32
}

// ChurnEntry is one row in the Change Map (view id "change") view: a unit
// (package/directory grain, same keyspace as UnitFact.ID) plus how much it
// has recently changed (git churn, bounded commit-depth/time-window read)
// and how complex it is (indexed symbol count — the complexity proxy).
// Risk is the naive product of the two — the view's whole premise is that
// frequently-changed AND structurally-complex code is the highest-risk
// combination, not either signal alone.
type ChurnEntry struct {
	Path         string // unit path (repo-relative directory; UnitFact.Path)
	ChangedFiles int    // distinct files changed within the bounded window
	Commits      int    // commits touching this unit within the bounded window
	Complexity   int    // indexed symbol count (REAL) — complexity proxy
	Risk         int    // ChangedFiles * Complexity — churn×complexity score
	File         string // G7 source pointer (unit's defining file)
	Line         uint32 // G7 source pointer
}

// GlossaryEntry is one candidate term harvested from the atlas
// (internal/domain/glossary.Entry, converted at the boundary). Always
// surfaced with MIXED provenance — a real keyword grouping, not a ratified
// human definition (D2).
type GlossaryEntry struct {
	Term       string
	Domain     string
	Definition string
}

// RouteEntry is one row in the API Contract (view id "api-contract") view:
// an HTTP route-registration call found by the treesitter route extractor
// (VL-3, board #37 — the first `route`-kind fact, D1). Method/Path/Handler
// are read straight off the AST call site (REAL/derived, D2) via a
// syntactic method-name match (GET/POST/.../HandleFunc/Handle) — the same
// honesty tier as ImportEdge's literal spec: no type resolution, so a
// same-named method on an unrelated receiver would also match (documented
// v1 scope, not silently hidden).
type RouteEntry struct {
	Method    string // "GET" | "POST" | ... | "" (net/http Handle/HandleFunc carry no verb)
	Path      string
	Handler   string // raw handler expression text, best-effort
	Framework string // "gin" | "net/http"
	File      string // G7 source pointer
	Line      uint32 // G7 source pointer
}

// EntityEntry is one struct entity found by the treesitter schema extractor
// (COL-1 — the first `entity`-kind fact, D1) for the Data Model / ER (view
// id "datamodel") view. Fields are read straight off the AST (REAL/derived,
// D2) — no FK/relationship detection (D29 ruling: relationship verbs are
// MIXED/overlay-only, a later slice, not this one).
type EntityEntry struct {
	Name   string   // struct type name
	Fields []string // field names, declaration order (REAL)
	Tech   string   // source technology, e.g. "Go struct"
	File   string   // G7 source pointer
	Line   uint32   // G7 source pointer
}

// DeploymentEntry is one deploy artifact found by the deployfile readers
// (COL-2 — the first `deployment`-kind fact, D1) for the Deployment (view id
// "deployment") view: a Dockerfile's base image, one compose.yaml service, or
// one Kubernetes workload/service manifest document. Fields are read straight
// off the manifest (REAL/derived, D2) — no cross-manifest correlation (e.g.
// matching a compose service to a k8s Deployment by name) is attempted; that
// would be an invented edge, not a derived one.
type DeploymentEntry struct {
	ID        string   // service/resource/image name
	Kind      string   // "dockerfile" | "compose-service" | "k8s-deployment" | "k8s-statefulset" | "k8s-daemonset" | "k8s-service" | "k8s-cronjob"
	Image     string   // container image reference, when declared
	Ports     []string // exported/published ports, when declared (REAL)
	DependsOn []string // same-manifest dependency names (compose depends_on only, v1)
	File      string   // G7 source pointer
	Line      uint32   // G7 source pointer
}

// OwnershipEntry is one row in the Ownership (view id "ownership") view
// (COL-3 — the owner-collector, board M6): a unit's owner(s), joined at
// unit grain from two readers, tried in this order per unit:
//  1. CODEOWNERS (internal/adapters/codeowners) — Provenance "declared":
//     the pattern's owners are read straight off the repo's declared
//     ownership file (D2, D30 extensionless disk read).
//  2. Bounded git authorship (a single bounded `git log` subprocess, VL-2's
//     churnSinceWindow/churnMaxCommits subprocess discipline) — Provenance
//     "derived": the unit's top commit-author within the bounded window,
//     same heuristic-join honesty tier as ChurnEntry.
//
// A unit with neither signal produces no row at all — never a fabricated
// "unowned" placeholder; the view's own "N units with defined owners"
// caption already states the coverage gap honestly.
type OwnershipEntry struct {
	Path       string   // unit path (repo-relative directory; UnitFact.Path grain)
	Owners     []string // declared (CODEOWNERS) or derived (top git author) owner(s)
	Provenance string   // "declared" (CODEOWNERS) | "derived" (git authorship)
	File       string   // G7 source pointer: CODEOWNERS rule's line, or the unit's defining file
	Line       uint32   // G7 source pointer
}

// VLInputs bundles the view-library-specific data sources RenderAll
// optionally threads into RenderInput (VL-1: Components/Technologies/
// GlossaryTerms). Grouped into one struct — rather than three more
// positional RenderAll parameters — so later view-library slices (VL-2/
// VL-3, same GATE-V2 milestone) can extend this bundle without another
// signature break. Nil is valid: every field defaults empty and each
// corresponding view renders its honest empty state.
type VLInputs struct {
	Components    []Component
	Technologies  []TechEntry
	GlossaryTerms []GlossaryEntry
	// ChurnEntries carries VL-2's git-churn × complexity rows (board #36).
	ChurnEntries []ChurnEntry
	// Routes carries VL-3's HTTP route-registration rows (board #37).
	Routes []RouteEntry
	// Entities carries COL-1's struct-entity rows (schema-collector).
	Entities []EntityEntry
	// Deployments carries COL-2's deploy-artifact rows (deployment-collector,
	// board M6): Dockerfile/compose.yaml/Kubernetes-manifest facts.
	Deployments []DeploymentEntry
	// OwnershipEntries carries COL-3's ownership rows (owner-collector,
	// board M6): CODEOWNERS-parse (declared) + bounded git-authorship
	// (derived) fallback, joined at unit grain.
	OwnershipEntries []OwnershipEntry
	// FileDomains carries DOM-1's file-grain atlas domain votes (board L22.23,
	// see RenderInput.FileDomains's doc comment for the full contract).
	FileDomains map[string]string
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

// ArchSchemaVersion is the shard/manifest JSON shape version, stamped into
// every Manifest by Service.RenderAll (T64). Bump it whenever Bucket/Shard/
// Manifest JSON shape changes in a way that a running daemon's cached shards
// would no longer match. App-layer boot logic (hasLocalArchManifest) compares
// a persisted manifest's SchemaVersion against this constant and forces a
// re-derive on mismatch — including the zero value, which old (pre-T64)
// manifests carry since the field did not exist yet.
const ArchSchemaVersion = 1

// Manifest is the top-level catalog of all rendered shards for one scope.
// Byte-stable across two RenderAll calls on identical input: all slices
// sorted by ID; SchemaVersion is a fixed constant so it never breaks
// determinism. DerivedAt is the one exception — it is a wall-clock stamp,
// so RenderAll (pure/deterministic) leaves it at its zero value; the app
// layer (deriveArch) sets it once, right before persisting, so it reflects
// the actual derive/persist moment rather than a later serve-time read (T65).
type Manifest struct {
	Scope string `json:"scope"`
	Rev   string `json:"rev"`
	// SchemaVersion is stamped by Service.RenderAll from ArchSchemaVersion (T64).
	SchemaVersion int `json:"schemaVersion"`
	// DerivedAt is a UTC timestamp ("2006-01-02 15:04:05 UTC") set by the app
	// layer at persist time, not by RenderAll (T65). Empty when the manifest
	// came straight out of RenderAll and was never persisted.
	DerivedAt string      `json:"derivedAt,omitempty"`
	Views     []ViewEntry `json:"views"`
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

	// Grain (rung-2): the footprint anchor grain (footprint.go). Controls how
	// deep pathPrefixGroup groups. nil → default top-level-dir rule
	// (byte-identical to the pre-recon grouper, consensus §5 MUST-NOT-CUT).
	// Provenance stays "derived" (ruling A): a deterministic footprint grain is
	// as REAL as the rung-2 it refines — only Haiku-touched anchors go "mixed".
	Grain *Grain
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
		return Prov{Kind: "derived", Label: "REAL · imports (incl. test files) + deterministic grouping"}
	}
}
