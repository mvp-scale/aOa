// Package status generates status data for aOa.
//
// The daemon writes a JSON status file on every state change (observe, autotune).
// A hook script reads this JSON, combines it with Claude Code stdin data
// (context window, model), and formats the display.
//
// Every field in StatusData maps to a segment name in .aoa/status-line.conf.
package status

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/corey/aoa/internal/ports"
)

// StatusFile is the filename within the .aoa directory where status JSON is written.
const StatusFile = "status.json"

// StatusData is the JSON payload the daemon writes for the hook to read.
// All fields are available as segments in .aoa/status-line.conf.
type StatusData struct {
	// Live
	Intents          uint32   `json:"intents"`
	Domains          int      `json:"domains"`
	TopDomains       []string `json:"top_domains"`
	TokensSaved      int64    `json:"tokens_saved"`
	TimeSavedMs      int64    `json:"time_saved_ms"`
	BurnRatePerMin   float64  `json:"burn_rate_per_min"`
	GuidedRatio      float64  `json:"guided_ratio"`
	ReadCount        int      `json:"read_count"`
	GuidedReadCount  int      `json:"guided_read_count"`
	ShadowSaved      int64    `json:"shadow_saved"`
	CacheHitRate     float64  `json:"cache_hit_rate"`
	AutotuneProgress int      `json:"autotune_progress"`

	// Runway
	RunwayMinutes float64 `json:"runway_minutes"`
	DeltaMinutes  float64 `json:"delta_minutes"`

	// Intel (learner state)
	Mastered   int     `json:"mastered"`   // core domain count
	Observed   int     `json:"observed"`   // files learned from
	Vocabulary int     `json:"vocabulary"` // keywords extracted
	Concepts   int     `json:"concepts"`   // terms resolved
	Patterns   int     `json:"patterns"`   // bigrams captured
	Evidence   float64 `json:"evidence"`   // cumulative domain hits

	// Intel (derived)
	LearningSpeed float64 `json:"learning_speed"` // domains per prompt
	SignalClarity float64 `json:"signal_clarity"` // terms resolved / keywords
	Conversion    float64 `json:"conversion"`     // domains / keywords

	// Debrief (session metrics)
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	Flow            float64 `json:"flow"`             // burst throughput tok/s
	Pace            float64 `json:"pace"`             // visible conversation tok/s
	TurnTimeMs      int     `json:"turn_time_ms"`     // avg turn duration
	Leverage        float64 `json:"leverage"`         // tools per turn
	Amplification   float64 `json:"amplification"`    // output chars / input chars
	CostPerExchange float64 `json:"cost_per_exchange"` // cost / turns
	CacheSavedUSD   float64 `json:"cache_saved_usd"`  // cache read savings in dollars
	CostSavedUSD    float64 `json:"cost_saved_usd"`   // est. cost saved from guided reads
	TurnCount       int     `json:"turn_count"`

	Autotune *Tune `json:"autotune,omitempty"`
}

// Tune holds the most recent autotune results.
type Tune struct {
	Promoted int `json:"promoted"`
	Demoted  int `json:"demoted"`
	Decayed  int `json:"decayed"`
	Pruned   int `json:"pruned"`
}

// Metrics holds runtime values passed from the app layer into Generate.
type Metrics struct {
	// Live
	TokensSaved     int64
	TimeSavedMs     int64
	BurnRatePerMin  float64
	GuidedRatio     float64
	ReadCount       int
	GuidedReadCount int
	ShadowSaved     int64
	CacheHitRate    float64

	// Runway
	RunwayMinutes float64
	DeltaMinutes  float64

	// Debrief
	InputTokens     int
	OutputTokens    int
	Flow            float64
	Pace            float64
	TurnTimeMs      int
	Leverage        float64
	Amplification   float64
	CostPerExchange float64
	CacheSavedUSD   float64
	CostSavedUSD    float64
	TurnCount       int
	UserChars       int64
	AssistantChars  int64
	ToolCount       int
}

// Generate produces a StatusData from current learner state and runtime metrics.
func Generate(state *ports.LearnerState, autotune *ports.AutotuneResult, m Metrics) *StatusData {
	// Intel metrics from learner state
	var totalEvidence float64
	var coreCount int
	for _, dm := range state.DomainMeta {
		totalEvidence += dm.Hits
		if dm.Tier == "core" && dm.State != "deprecated" {
			coreCount++
		}
	}

	// Intel derived ratios
	promptCount := int(state.PromptCount)
	kwCount := len(state.KeywordHits)
	termCount := len(state.TermHits)
	domainCount := len(state.DomainMeta)

	var learningSpeed, signalClarity, conversion float64
	if promptCount > 0 {
		learningSpeed = float64(domainCount) / float64(promptCount)
	}
	if kwCount > 0 {
		signalClarity = float64(termCount) / float64(kwCount)
		conversion = float64(domainCount) / float64(kwCount)
	}

	// Debrief derived metrics
	var leverage, amplification, costPerExchange float64
	var turnTimeMs int
	if m.TurnCount > 0 {
		leverage = float64(m.ToolCount) / float64(m.TurnCount)
		costPerExchange = m.CostPerExchange
		if m.TurnTimeMs > 0 {
			turnTimeMs = m.TurnTimeMs
		}
	}
	if m.UserChars > 0 {
		amplification = float64(m.AssistantChars) / float64(m.UserChars)
	}

	sd := &StatusData{
		// Live
		Intents:          state.PromptCount,
		Domains:          domainCount,
		TopDomains:       topDomains(state, 3),
		TokensSaved:      m.TokensSaved,
		TimeSavedMs:      m.TimeSavedMs,
		BurnRatePerMin:   m.BurnRatePerMin,
		GuidedRatio:      m.GuidedRatio,
		ReadCount:        m.ReadCount,
		GuidedReadCount:  m.GuidedReadCount,
		ShadowSaved:      m.ShadowSaved,
		CacheHitRate:     m.CacheHitRate,
		AutotuneProgress: int(state.PromptCount % 50),

		// Runway
		RunwayMinutes: m.RunwayMinutes,
		DeltaMinutes:  m.DeltaMinutes,

		// Intel
		Mastered:      coreCount,
		Observed:      len(state.FileHits),
		Vocabulary:    kwCount,
		Concepts:      termCount,
		Patterns:      len(state.Bigrams),
		Evidence:      totalEvidence,
		LearningSpeed: learningSpeed,
		SignalClarity: signalClarity,
		Conversion:    conversion,

		// Debrief
		InputTokens:     m.InputTokens,
		OutputTokens:    m.OutputTokens,
		Flow:            m.Flow,
		Pace:            m.Pace,
		TurnTimeMs:      turnTimeMs,
		Leverage:        leverage,
		Amplification:   amplification,
		CostPerExchange: costPerExchange,
		CacheSavedUSD:   m.CacheSavedUSD,
		CostSavedUSD:    m.CostSavedUSD,
		TurnCount:       m.TurnCount,
	}
	if autotune != nil {
		sd.Autotune = &Tune{
			Promoted: autotune.Promoted,
			Demoted:  autotune.Demoted,
			Decayed:  autotune.Decayed,
			Pruned:   autotune.Pruned,
		}
	}
	return sd
}

// WriteJSON writes the status data as JSON to a file.
func WriteJSON(path string, data *StatusData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// topDomains returns the top N domain names sorted by hits descending.
func topDomains(state *ports.LearnerState, n int) []string {
	if len(state.DomainMeta) == 0 {
		return nil
	}

	type dh struct {
		name string
		hits float64
	}

	var domains []dh
	for name, dm := range state.DomainMeta {
		if dm.State != "deprecated" && dm.Hits > 0 {
			domains = append(domains, dh{name, dm.Hits})
		}
	}

	sort.Slice(domains, func(i, j int) bool {
		if domains[i].hits != domains[j].hits {
			return domains[i].hits > domains[j].hits
		}
		return domains[i].name < domains[j].name
	})

	limit := n
	if limit > len(domains) {
		limit = len(domains)
	}

	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = domains[i].name
	}
	return result
}
