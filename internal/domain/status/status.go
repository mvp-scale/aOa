// Package status generates the pre-computed status line for aOa-go.
//
// The status line is written to a file on every state change (observe, autotune).
// A lightweight hook reads this file — no computation at hook time.
//
// Format:
//
//	⚡ aOa-go │ 150 intents │ 8 domains │ promoted:0 demoted:0 decayed:8 pruned:0
package status

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/corey/aoa-go/internal/domain/learner"
	"github.com/corey/aoa-go/internal/ports"
)

// DefaultPath is where the status line is written.
// StatusFile is the filename within the .aoa directory where the status line is written.
const StatusFile = "status-line.txt"

// Generate produces a status line string from current learner state.
// If autotune is non-nil, autotune stats are included.
func Generate(state *ports.LearnerState, autotune *learner.AutotuneResult) string {
	var parts []string

	// Prefix
	parts = append(parts, "\033[96m\033[1m⚡ aOa-go\033[0m")

	// Intent count
	parts = append(parts, fmt.Sprintf("%d intents", state.PromptCount))

	// Domain count
	domainCount := len(state.DomainMeta)
	parts = append(parts, fmt.Sprintf("%d domains", domainCount))

	// Autotune stats (if autotune just ran)
	if autotune != nil {
		stats := fmt.Sprintf("promoted:%d demoted:%d decayed:%d pruned:%d",
			autotune.Promoted, autotune.Demoted,
			autotune.Decayed, autotune.Pruned)
		parts = append(parts, stats)
	}

	// Top domains by hits (up to 3)
	top := topDomains(state, 3)
	if len(top) > 0 {
		parts = append(parts, strings.Join(top, " "))
	}

	return strings.Join(parts, " \033[2m│\033[0m ")
}

// GeneratePlain produces a status line without ANSI colors.
func GeneratePlain(state *ports.LearnerState, autotune *learner.AutotuneResult) string {
	var parts []string

	parts = append(parts, "⚡ aOa-go")
	parts = append(parts, fmt.Sprintf("%d intents", state.PromptCount))
	parts = append(parts, fmt.Sprintf("%d domains", len(state.DomainMeta)))

	if autotune != nil {
		stats := fmt.Sprintf("promoted:%d demoted:%d decayed:%d pruned:%d",
			autotune.Promoted, autotune.Demoted,
			autotune.Decayed, autotune.Pruned)
		parts = append(parts, stats)
	}

	top := topDomains(state, 3)
	if len(top) > 0 {
		parts = append(parts, strings.Join(top, " "))
	}

	return strings.Join(parts, " │ ")
}

// Write writes the status line to a file atomically.
func Write(path, line string) error {
	return os.WriteFile(path, []byte(line+"\n"), 0644)
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
		result[i] = "@" + domains[i].name
	}
	return result
}
