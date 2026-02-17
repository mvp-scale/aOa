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
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Symbol string   `json:"symbol"`
	Range  [2]int   `json:"range"`
	Domain string   `json:"domain"`
	Tags   []string `json:"tags"`
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
	Name   string  `json:"name"`
	Hits   float64 `json:"hits"`
	Tier   string  `json:"tier"`
	State  string  `json:"state"`
	Source string  `json:"source"`
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
