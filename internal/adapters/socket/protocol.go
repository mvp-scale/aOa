// Package socket implements a JSON-over-Unix-socket protocol for the aOa daemon.
// The protocol uses newline-delimited JSON: each message is one JSON object + \n.
package socket

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/corey/aoa/internal/ports"
)

// SocketPath returns the Unix socket path for a given project root.
// Format: /tmp/aoa-{first12hex}.sock
// Includes UID in the hash for multi-user safety on shared systems.
func SocketPath(projectRoot string) string {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		abs = projectRoot
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", os.Getuid(), abs)))
	return fmt.Sprintf("/tmp/aoa-%x.sock", h[:6])
}

// LegacySocketPath returns the pre-UID socket path for migration.
// Used by daemon stop and init to clean up sockets from older versions.
func LegacySocketPath(projectRoot string) string {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		abs = projectRoot
	}
	h := sha256.Sum256([]byte(abs))
	return fmt.Sprintf("/tmp/aoa-%x.sock", h[:6])
}

// Method names for the protocol.
const (
	MethodSearch   = "search"
	MethodHealth   = "health"
	MethodShutdown = "shutdown"
	MethodFiles    = "files"
	MethodDomains  = "domains"
	MethodBigrams  = "bigrams"
	MethodStats    = "stats"
	MethodWipe     = "wipe"
	MethodReindex  = "reindex"
	MethodPeek     = "peek"

	// L19.16 — six arch methods (protocol stays exactly these six; reach/blast
	// are CLI-only aliases per ADR 2026-07-02, NOT added here).
	MethodArchViews    = "arch.views"    // params: ArchViewsParams    → ArchViewsResult
	MethodArchView     = "arch.view"     // params: ArchViewParams     → ArchViewResult
	MethodArchFindings = "arch.findings" // params: ArchFindingsParams → ArchFindingsResult
	MethodArchJourney  = "arch.journey"  // params: ArchJourneyParams  → stub (not yet implemented)
	MethodArchDerive   = "arch.derive"   // params: ArchDeriveParams   → ArchDeriveResult
	MethodArchFacts    = "arch.facts"    // params: ArchFactsParams    → ArchFactsResult
)

// Request is the wire format for client-to-server messages.
type Request struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// Response is the wire format for server-to-client messages.
type Response struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// SearchParams is the params for a search request.
type SearchParams struct {
	Query   string             `json:"query"`
	Options ports.SearchOptions `json:"options"`
}

// SearchResult is the result of a search request.
type SearchResult struct {
	Hits     []SearchHit `json:"hits"`
	Count    int         `json:"count"`
	ExitCode int         `json:"exit_code"`
	Elapsed  string      `json:"elapsed"`
	Hints    []string    `json:"hints,omitempty"`
}

// SearchHit is a single hit in search results (wire format).
type SearchHit struct {
	File         string         `json:"file"`
	Line         int            `json:"line"`
	Symbol       string         `json:"symbol"`
	Range        [2]int         `json:"range"`
	Domain       string         `json:"domain"`
	Tags         []string       `json:"tags"`
	Kind         string         `json:"kind,omitempty"`
	Content      string         `json:"content,omitempty"`
	ContextLines map[int]string `json:"context_lines,omitempty"`
	PeekCode     string         `json:"peek_code,omitempty"`
}

// HealthResult is the result of a health request.
type HealthResult struct {
	Status     string `json:"status"`
	FileCount  int    `json:"file_count"`
	TokenCount int    `json:"token_count"`
	Uptime     string `json:"uptime"`

	// L21.2 tri-state: independent daemon/db/web health. DaemonOK is implicit
	// (a served response proves the daemon); DBOK/WebOK flow from the app's
	// probe source. Additive — older daemons omit them (clients must derive
	// from Status, never trust zero-value false).
	DaemonOK bool `json:"daemon_ok,omitempty"`
	DBOK     bool `json:"db_ok,omitempty"`
	WebOK    bool `json:"web_ok,omitempty"`
}

// FilesParams is the params for a files request.
type FilesParams struct {
	Glob string `json:"glob,omitempty"` // fnmatch glob (for find)
	Name string `json:"name,omitempty"` // substring match (for locate)
}

// FilesResult is the result of a files request.
type FilesResult struct {
	Files []FileInfo `json:"files"`
	Count int        `json:"count"`
}

// FileInfo describes a single indexed file.
type FileInfo struct {
	Path     string `json:"path"`
	Language string `json:"language,omitempty"`
	Domain   string `json:"domain,omitempty"`
}

// DomainsResult is the result of a domains request.
type DomainsResult struct {
	Domains   []DomainInfo `json:"domains"`
	Count     int          `json:"count"`
	CoreCount int          `json:"core_count"`
}

// DomainInfo describes a single domain.
type DomainInfo struct {
	Name     string         `json:"name"`
	Hits     float64        `json:"hits"`
	Tier     string         `json:"tier"`
	State    string         `json:"state"`
	Source   string         `json:"source"`
	Terms    []string       `json:"terms,omitempty"`
	TermHits map[string]int `json:"term_hits,omitempty"` // term name -> total keyword hits (for popularity sort + flash)
}

// BigramsResult is the result of a bigrams request.
// Also includes cohit data for the dashboard n-gram metrics panel.
type BigramsResult struct {
	Bigrams         map[string]uint32 `json:"bigrams"`
	Count           int               `json:"count"`
	CohitKwTerm     map[string]uint32 `json:"cohit_kw_term,omitempty"`
	CohitTermDomain map[string]uint32 `json:"cohit_term_domain,omitempty"`
	CohitKwCount    int               `json:"cohit_kw_count"`
	CohitTdCount    int               `json:"cohit_td_count"`
}

// StatsResult is the result of a stats request.
type StatsResult struct {
	PromptCount  uint32 `json:"prompt_count"`
	DomainCount  int    `json:"domain_count"`
	CoreCount    int    `json:"core_count"`
	ContextCount int    `json:"context_count"`
	KeywordCount int    `json:"keyword_count"`
	TermCount    int    `json:"term_count"`
	BigramCount  int    `json:"bigram_count"`
	FileHitCount int    `json:"file_hit_count"`
	IndexFiles   int    `json:"index_files"`
	IndexTokens  int    `json:"index_tokens"`
}

// SessionMetricsResult is the result of a session metrics request.
type SessionMetricsResult struct {
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int     `json:"cache_write_tokens"`
	TurnCount        int     `json:"turn_count"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	SessionStartTs   int64   `json:"session_start_ts,omitempty"`
}

// ToolMetricsResult is the result of a tool metrics request.
type ToolMetricsResult struct {
	ReadCount    int            `json:"read_count"`
	WriteCount   int            `json:"write_count"`
	EditCount    int            `json:"edit_count"`
	BashCount    int            `json:"bash_count"`
	GrepCount    int            `json:"grep_count"`
	GlobCount    int            `json:"glob_count"`
	OtherCount   int            `json:"other_count"`
	TotalCount   int            `json:"total_count"`
	FileReads    map[string]int `json:"file_reads"`
	BashCommands map[string]int `json:"bash_commands"`
	GrepPatterns map[string]int `json:"grep_patterns"`
}

// ConversationFeedResult is the result of a conversation feed request.
type ConversationFeedResult struct {
	Turns []ConversationTurnResult `json:"turns"`
	Count int                      `json:"count"`
}

// ConversationTurnResult describes a single turn in the conversation.
type ConversationTurnResult struct {
	TurnID       string             `json:"turn_id"`
	Role         string             `json:"role"`
	Text         string             `json:"text"`
	ThinkingText string             `json:"thinking_text,omitempty"`
	DurationMs   int                `json:"duration_ms"`
	ToolNames    []string           `json:"tool_names"`
	Actions      []TurnActionResult `json:"actions,omitempty"`
	Timestamp    int64              `json:"timestamp"`
	Model        string             `json:"model"`
	InputTokens  int                `json:"input_tokens,omitempty"`
	OutputTokens int                `json:"output_tokens,omitempty"`
}

// TurnActionResult describes a single tool action within a conversation turn.
type TurnActionResult struct {
	Tool        string `json:"tool"`
	Target      string `json:"target"`
	Range       string `json:"range,omitempty"`
	Impact      string `json:"impact,omitempty"`
	Attrib      string `json:"attrib,omitempty"`
	Tokens      int    `json:"tokens,omitempty"`
	Savings     int    `json:"savings,omitempty"`
	TimeSavedMs int64  `json:"time_saved_ms,omitempty"`
	ResultChars int    `json:"result_chars,omitempty"`
	Pattern     string `json:"pattern,omitempty"`    // L9.2: search pattern (Grep/Glob)
	FilePath    string `json:"file_path,omitempty"`  // L9.2: file path (Read/Write/Edit)
	Command     string `json:"command,omitempty"`    // L9.2: shell command (Bash)
	ShadowChars int    `json:"shadow_chars,omitempty"` // L9.5: aOa shadow search chars
	ShadowSaved int    `json:"shadow_saved,omitempty"` // L9.5: chars saved vs native

	// Subagent telemetry (Agent tool only)
	SubagentTokens     int                `json:"subagent_tokens,omitempty"`
	SubagentToolUses   int                `json:"subagent_tool_uses,omitempty"`
	SubagentDurationMs int64              `json:"subagent_duration_ms,omitempty"`
	SubagentType       string             `json:"subagent_type,omitempty"`
	Children           []TurnActionResult `json:"children,omitempty"`

	// L18.4: Agent rows carry this flag so the UI renders '—' instead of the
	// known-wrong estimate. SubagentTokens is preserved for future L18.3
	// attribution but must not be displayed until attribution is reliable.
	AgentTokensPending bool `json:"agent_tokens_pending,omitempty"`
}

// ActivityEntryResult describes a single action in the activity feed.
type ActivityEntryResult struct {
	Action    string `json:"action"`
	Source    string `json:"source"`
	Attrib    string `json:"attrib"`
	Impact    string `json:"impact"`
	Learned   string `json:"learned,omitempty"`
	Tags      string `json:"tags"`
	Target    string `json:"target"`
	Timestamp int64  `json:"timestamp"`
}

// ActivityFeedResult is the result of an activity feed request.
type ActivityFeedResult struct {
	Entries []ActivityEntryResult `json:"entries"`
	Count   int                   `json:"count"`
}

// TopItemsResult is the result of a top items request.
type TopItemsResult struct {
	Items []RankedItem `json:"items"`
	Count int          `json:"count"`
	Kind  string       `json:"kind"`
}

// RankedItem describes a single ranked item (keyword, term, file, etc.).
type RankedItem struct {
	Name  string  `json:"name"`
	Count float64 `json:"count"`
}

// RunwayResult is the result of a runway projection request.
type RunwayResult struct {
	Model              string  `json:"model"`
	ContextWindowMax   int     `json:"context_window_max"`
	TokensUsed         int64   `json:"tokens_used"`
	BurnRatePerMin     float64 `json:"burn_rate_per_min"`
	CounterfactPerMin  float64 `json:"counterfact_per_min"`
	RunwayMinutes      float64 `json:"runway_minutes"`
	CounterfactMinutes float64 `json:"counterfact_minutes"`
	DeltaMinutes       float64 `json:"delta_minutes"`
	TokensSaved        int64   `json:"tokens_saved"`
	TimeSavedMs        int64   `json:"time_saved_ms"`
	MsPerToken         float64 `json:"ms_per_token"`
	ReadCount          int     `json:"read_count"`
	GuidedReadCount    int     `json:"guided_read_count"`
	CacheHitRate       float64 `json:"cache_hit_rate"`
	// L9.7: Burst throughput metrics
	BurstThroughput float64   `json:"burst_throughput,omitempty"`
	ActiveMs        int64     `json:"active_ms,omitempty"`
	TurnVelocities  []float64 `json:"turn_velocities,omitempty"`

	// L9.8: Shadow savings metrics
	ShadowTotalSaved  int64 `json:"shadow_total_saved,omitempty"`
	ShadowSearchCount int   `json:"shadow_search_count,omitempty"`

	// Context snapshot from status line hook (real Claude Code data)
	CtxUsed            int64   `json:"ctx_used"`
	CtxMax             int64   `json:"ctx_max"`
	CtxUsedPct         float64 `json:"ctx_used_pct"`
	CtxRemainingPct    float64 `json:"ctx_remaining_pct"`
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMs    int64   `json:"total_duration_ms"`
	TotalApiDurationMs int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
	CtxSnapshotAge     int64   `json:"ctx_snapshot_age"`
}

// UsageQuotaTierResult holds one tier from parsed /usage output.
type UsageQuotaTierResult struct {
	Label      string `json:"label"`
	UsedPct    int    `json:"used_pct"`
	ResetsAt   string `json:"resets_at"`
	ResetEpoch int64  `json:"reset_epoch"`
	Timezone   string `json:"timezone"`
}

// UsageQuotaResult holds the parsed /usage output for the API.
type UsageQuotaResult struct {
	Session      *UsageQuotaTierResult `json:"session,omitempty"`
	WeeklyAll    *UsageQuotaTierResult `json:"weekly_all,omitempty"`
	WeeklySonnet *UsageQuotaTierResult `json:"weekly_sonnet,omitempty"`
	CapturedAt   int64                 `json:"captured_at"`
}

// SessionSummaryResult describes a single persisted session in the API response.
type SessionSummaryResult struct {
	SessionID        string  `json:"session_id"`
	StartTime        int64   `json:"start_time"`
	EndTime          int64   `json:"end_time"`
	PromptCount      int     `json:"prompt_count"`
	ReadCount        int     `json:"read_count"`
	GuidedReadCount  int     `json:"guided_read_count"`
	GuidedRatio      float64 `json:"guided_ratio"`
	TokensSaved      int64   `json:"tokens_saved"`
	TimeSavedMs      int64   `json:"time_saved_ms"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	CacheWriteTokens int              `json:"cache_write_tokens"`
	Model            string           `json:"model"`
	ModelTokens      map[string]int64 `json:"model_tokens,omitempty"`
}

// SessionListResult is the result of a sessions list request.
type SessionListResult struct {
	Sessions []SessionSummaryResult `json:"sessions"`
	Count    int                    `json:"count"`
}

// TelemetryResult is the result of a telemetry snapshot request (L17.3).
// Lifetime includes persisted totals plus in-flight session delta.
// Session contains only the current session's counters.
type TelemetryResult struct {
	Lifetime TelemetryCounters `json:"lifetime"`
	Session  TelemetryCounters `json:"session"`
}

// TelemetryCounters holds a set of metric counters for either lifetime or session scope.
type TelemetryCounters struct {
	TokensSaved     int64 `json:"tokens_saved"`
	TimeSavedMs     int64 `json:"time_saved_ms"`
	Reads           int   `json:"reads"`
	GuidedReads     int   `json:"guided_reads"`
	Sessions        int   `json:"sessions"`
	Prompts         int   `json:"prompts"`
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	CacheReadTokens int64 `json:"cache_read_tokens"`
	ShadowSaved     int64 `json:"shadow_saved"`
}

// ProjectConfigResult is the result of a config request.
type ProjectConfigResult struct {
	ProjectRoot   string  `json:"project_root"`
	ProjectID     string  `json:"project_id"`
	DBPath        string  `json:"db_path"`
	SocketPath    string  `json:"socket_path"`
	IndexFiles    int     `json:"index_files"`
	IndexTokens   int     `json:"index_tokens"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	Version       string  `json:"version"`
	BuildDate     string  `json:"build_date"`
	HeapAllocMB   float64 `json:"heap_alloc_mb"`
	SysMB         float64 `json:"sys_mb"`
	HeapObjects   uint64  `json:"heap_objects"`
	NumGC         uint32  `json:"num_gc"`
	Goroutines    int     `json:"goroutines"`
	NumCPU        int     `json:"num_cpu"`
	Platform      string  `json:"platform"`
	GoVersion     string  `json:"go_version"`
	DBSizeBytes   int64   `json:"db_size_bytes"`
}

// ReindexResult is the result of a reindex request.
type ReindexResult struct {
	FileCount   int    `json:"file_count"`
	SymbolCount int    `json:"symbol_count"`
	TokenCount  int    `json:"token_count"`
	ElapsedMs   int64  `json:"elapsed_ms"`
}

// DimensionalFileResult holds dimensional analysis results for a single file.
// This is the DTO exposed via the AppQueries interface; it mirrors analyzer.FileAnalysis
// without importing the domain package.
type DimensionalFileResult struct {
	Path     string                       `json:"path"`
	Language string                       `json:"language"`
	Bitmask  [6]uint64                    `json:"bitmask"`
	Methods  []DimensionalMethodResult    `json:"methods"`
	Findings []DimensionalFindingResult   `json:"findings"`
	ScanTime int64                        `json:"scan_time_us"`
}

// DimensionalMethodResult holds per-method dimensional analysis.
type DimensionalMethodResult struct {
	Name     string                      `json:"name"`
	Line     int                         `json:"line"`
	EndLine  int                         `json:"end_line"`
	Bitmask  [6]uint64                   `json:"bitmask"`
	Score    int                         `json:"score"`
	Findings []DimensionalFindingResult  `json:"findings"`
}

// DimensionalFindingResult holds a single dimensional finding.
type DimensionalFindingResult struct {
	RuleID   string `json:"rule_id"`
	Line     int    `json:"line"`
	Symbol   string `json:"symbol"`
	Severity int    `json:"severity"`
}

// ── L19.16 arch protocol DTOs ────────────────────────────────────────────────

// ArchViewsParams is the params for an arch.views request.
type ArchViewsParams struct {
	Scope string `json:"scope,omitempty"` // defaults to "local"
}

// ArchViewParams is the params for an arch.view request.
type ArchViewParams struct {
	Scope string `json:"scope,omitempty"` // defaults to "local"
	View  string `json:"view"`            // e.g. "component", "dsm", "cycles"
}

// ArchFindingsParams is the params for an arch.findings request.
type ArchFindingsParams struct {
	Scope    string `json:"scope,omitempty"`
	Severity string `json:"severity,omitempty"` // "error"|"warn"|"info"
}

// ArchJourneyParams is the params for an arch.journey request (stub).
type ArchJourneyParams struct {
	ID   string `json:"id,omitempty"`
	List bool   `json:"list,omitempty"`
}

// ArchDeriveParams is the params for an arch.derive request.
// From/To are unit IDs (e.g. "u_internal_app") — the CLI converts directory
// paths to unit IDs before sending.
type ArchDeriveParams struct {
	Scope string `json:"scope,omitempty"` // defaults to "local"
	From  string `json:"from"`            // source unit ID
	To    string `json:"to"`              // destination unit ID
	K     int    `json:"k,omitempty"`     // max hops (default 10)
	Via   string `json:"via,omitempty"`   // reserved
}

// ArchFactsParams is the params for an arch.facts request.
type ArchFactsParams struct {
	Scope   string `json:"scope,omitempty"`
	Subject string `json:"subject"`         // substring matched against from_file or import_path
	Kind    string `json:"kind,omitempty"`  // reserved
	Limit   int    `json:"limit,omitempty"` // 0 means unlimited
}

// ArchViewsResult carries the manifest JSON for aoa arch views.
// Raw is nil when no shards have been derived yet.
type ArchViewsResult struct {
	Raw     json.RawMessage `json:"raw"`      // encoded ArchManifest or null
	HasData bool            `json:"has_data"` // false when no manifest exists
}

// ArchViewResult carries a rendered shard for aoa arch view.
// Raw is nil when the view has not been rendered.
type ArchViewResult struct {
	Raw   json.RawMessage `json:"raw"`   // encoded shard JSON or null
	Found bool            `json:"found"` // false when view not found → exit 1
}

// ArchFindingsResult carries arch findings.
// Raw is nil when no findings have been computed.
type ArchFindingsResult struct {
	Raw    json.RawMessage `json:"raw"`     // encoded []Finding or null
	HasNew bool            `json:"has_new"` // true when findings exist (--new gate)
}

// ArchDeriveResult carries a derived dep-path between two units.
type ArchDeriveResult struct {
	Path  []string `json:"path"`  // ordered unit IDs on the shortest path (empty when not found)
	Found bool     `json:"found"` // false when no path within k hops → exit 1
}

// ArchFactsResult carries import edge facts for a subject.
type ArchFactsResult struct {
	Facts json.RawMessage `json:"facts"` // encoded []ArchFactEntry or null
	Count int             `json:"count"`
}

// ArchFactEntry is one import edge in the provenance audit trail.
type ArchFactEntry struct {
	FromFile   string `json:"from_file"`
	ImportPath string `json:"import_path"`
	StartLine  uint32 `json:"start_line"` // 1-based line in FromFile (G7 provenance)
}

// DimScanProgress holds progress state for an in-flight dimensional scan.
type DimScanProgress struct {
	Running bool    `json:"running"`
	Total   int     `json:"total"`
	Done    int     `json:"done"`
	Cached  int     `json:"cached"`
	Pct     float64 `json:"pct"`
	Elapsed float64 `json:"elapsed"`
	ETA     float64 `json:"eta"`
}

// PeekParams is the params for a peek request.
type PeekParams struct {
	Codes []string `json:"codes"`
}

// PeekResult is the result of a peek request.
type PeekResult struct {
	Symbols []PeekSymbol `json:"symbols"`
}

// PeekSymbol is a single resolved peek code with source lines.
type PeekSymbol struct {
	Code   string   `json:"code"`
	File   string   `json:"file"`
	Symbol string   `json:"symbol"`
	Range  [2]int   `json:"range"`
	Domain string   `json:"domain,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Lines  []string `json:"lines"`
	Error  string   `json:"error,omitempty"`
}
