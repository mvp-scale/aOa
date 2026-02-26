// Package socket implements a JSON-over-Unix-socket protocol for the aOa daemon.
// The protocol uses newline-delimited JSON: each message is one JSON object + \n.
package socket

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"

	"github.com/corey/aoa/internal/ports"
)

// SocketPath returns the Unix socket path for a given project root.
// Format: /tmp/aoa-{first12hex}.sock
func SocketPath(projectRoot string) string {
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
}

// HealthResult is the result of a health request.
type HealthResult struct {
	Status     string `json:"status"`
	FileCount  int    `json:"file_count"`
	TokenCount int    `json:"token_count"`
	Uptime     string `json:"uptime"`
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
	CacheWriteTokens int     `json:"cache_write_tokens"`
	Model            string  `json:"model"`
}

// SessionListResult is the result of a sessions list request.
type SessionListResult struct {
	Sessions []SessionSummaryResult `json:"sessions"`
	Count    int                    `json:"count"`
}

// ProjectConfigResult is the result of a config request.
type ProjectConfigResult struct {
	ProjectRoot   string `json:"project_root"`
	ProjectID     string `json:"project_id"`
	DBPath        string `json:"db_path"`
	SocketPath    string `json:"socket_path"`
	IndexFiles    int    `json:"index_files"`
	IndexTokens   int    `json:"index_tokens"`
	UptimeSeconds int64  `json:"uptime_seconds"`
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
