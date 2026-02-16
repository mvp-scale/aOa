// Package status generates status data for aOa.
//
// The daemon writes a JSON status file on every state change (observe, autotune).
// A hook script reads this JSON, combines it with Claude Code stdin data
// (context window, model), and formats the display.
package status

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/corey/aoa/internal/domain/learner"
	"github.com/corey/aoa/internal/ports"
)

// StatusFile is the filename within the .aoa directory where status JSON is written.
const StatusFile = "status.json"

// StatusData is the JSON payload the daemon writes for the hook to read.
type StatusData struct {
	Intents    uint32   `json:"intents"`
	Domains    int      `json:"domains"`
	TopDomains []string `json:"top_domains"`
	Autotune   *Tune    `json:"autotune,omitempty"`
}

// Tune holds the most recent autotune results.
type Tune struct {
	Promoted int `json:"promoted"`
	Demoted  int `json:"demoted"`
	Decayed  int `json:"decayed"`
	Pruned   int `json:"pruned"`
}

// Generate produces a StatusData from current learner state.
func Generate(state *ports.LearnerState, autotune *learner.AutotuneResult) *StatusData {
	sd := &StatusData{
		Intents:    state.PromptCount,
		Domains:    len(state.DomainMeta),
		TopDomains: topDomains(state, 3),
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
