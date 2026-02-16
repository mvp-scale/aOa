// Package learner implements the domain learning system.
// It tracks keyword/term/domain hits from observe signals, runs the 21-step
// autotune algorithm every 50 intents, and manages competitive displacement
// (top 24 core domains, rest context, prune <0.3).
//
// All math must match the Python reference implementation exactly.
// Float precision rules:
//   - domain_meta.hits: float64, NO truncation during decay
//   - All other maps: int(float(count) * 0.90) — truncated toward zero
package learner

import (
	"encoding/json"

	"github.com/corey/aoa/internal/ports"
)

// Constants matching Python canonical values (from SPEC.md).
const (
	DecayRate         = 0.90
	AutotuneInterval  = 50
	PruneFloor        = 0.3
	DedupMinTotal     = 100
	CoreDomainsMax    = 24
	ContextDomainsMax = 20
	PromotionMinRatio = 0.5
	MinPromotionObs   = 3
	NoiseThreshold    = 1000
	PreserveThreshold = 5
)

// Learner manages all domain learning state in-memory.
// Thread safety: NOT safe for concurrent use. The caller must serialize access.
type Learner struct {
	state   *ports.LearnerState
	staging map[string]uint32 // bigram staging counts (not persisted)
}

// New creates a fresh Learner with all maps initialized and zero counters.
func New() *Learner {
	return &Learner{
		staging: make(map[string]uint32),
		state: &ports.LearnerState{
			KeywordHits:      make(map[string]uint32),
			TermHits:         make(map[string]uint32),
			DomainMeta:       make(map[string]*ports.DomainMeta),
			CohitKwTerm:      make(map[string]uint32),
			CohitTermDomain:  make(map[string]uint32),
			Bigrams:          make(map[string]uint32),
			FileHits:         make(map[string]uint32),
			KeywordBlocklist: make(map[string]bool),
			GapKeywords:      make(map[string]bool),
			PromptCount:      0,
		},
	}
}

// NewFromState creates a Learner from an existing state (e.g., loaded from bbolt).
// All nil maps are initialized to empty maps.
func NewFromState(state *ports.LearnerState) *Learner {
	if state.KeywordHits == nil {
		state.KeywordHits = make(map[string]uint32)
	}
	if state.TermHits == nil {
		state.TermHits = make(map[string]uint32)
	}
	if state.DomainMeta == nil {
		state.DomainMeta = make(map[string]*ports.DomainMeta)
	}
	if state.CohitKwTerm == nil {
		state.CohitKwTerm = make(map[string]uint32)
	}
	if state.CohitTermDomain == nil {
		state.CohitTermDomain = make(map[string]uint32)
	}
	if state.Bigrams == nil {
		state.Bigrams = make(map[string]uint32)
	}
	if state.FileHits == nil {
		state.FileHits = make(map[string]uint32)
	}
	if state.KeywordBlocklist == nil {
		state.KeywordBlocklist = make(map[string]bool)
	}
	if state.GapKeywords == nil {
		state.GapKeywords = make(map[string]bool)
	}
	return &Learner{state: state, staging: make(map[string]uint32)}
}

// NewFromJSON creates a Learner from a JSON-encoded state snapshot.
func NewFromJSON(data []byte) (*Learner, error) {
	var state ports.LearnerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return NewFromState(&state), nil
}

// State returns the underlying state (for persistence or inspection).
func (l *Learner) State() *ports.LearnerState {
	return l.state
}

// PromptCount returns the current prompt count.
func (l *Learner) PromptCount() uint32 {
	return l.state.PromptCount
}

// Snapshot serializes the learner state to JSON.
// Map keys are sorted for deterministic output.
func (l *Learner) Snapshot() ([]byte, error) {
	return json.Marshal(l.state)
}
