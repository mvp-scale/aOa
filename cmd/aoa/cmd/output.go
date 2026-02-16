package cmd

import (
	"fmt"
	"strings"

	"github.com/corey/aoa/internal/adapters/socket"
)

// ANSI color codes for terminal output.
const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorGray    = "\033[90m"
)

// formatSearchResult formats a SearchResult for terminal display.
// Matches the Python aOa output format:
//
//	⚡ N hits │ Xms
//	  file:symbol[range]:line  @domain  #tag1 #tag2
func formatSearchResult(result *socket.SearchResult, countOnly, quiet bool) string {
	if quiet {
		return ""
	}

	if countOnly {
		return fmt.Sprintf("%s⚡ %d hits%s │ %s", colorBold, result.Count, colorReset, result.Elapsed)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ %d hits%s │ %s\n", colorBold, len(result.Hits), colorReset, result.Elapsed))

	for _, hit := range result.Hits {
		sb.WriteString(fmt.Sprintf("  %s%s%s:", colorCyan, hit.File, colorReset))

		if hit.Symbol != "" {
			sb.WriteString(hit.Symbol)
		}
		if hit.Range[0] > 0 || hit.Range[1] > 0 {
			sb.WriteString(fmt.Sprintf("[%d-%d]", hit.Range[0], hit.Range[1]))
		}
		sb.WriteString(fmt.Sprintf(":%d", hit.Line))

		if hit.Domain != "" {
			sb.WriteString(fmt.Sprintf("  %s%s%s", colorMagenta, hit.Domain, colorReset))
		}

		if len(hit.Tags) > 0 {
			sb.WriteString("  ")
			for i, tag := range hit.Tags {
				if i > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(fmt.Sprintf("%s#%s%s", colorGreen, tag, colorReset))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatHealth formats a HealthResult for terminal display.
func formatHealth(h *socket.HealthResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ aOa daemon%s\n", colorBold, colorReset))
	sb.WriteString(fmt.Sprintf("  Status:  %s%s%s\n", colorGreen, h.Status, colorReset))
	sb.WriteString(fmt.Sprintf("  Files:   %d\n", h.FileCount))
	sb.WriteString(fmt.Sprintf("  Tokens:  %d\n", h.TokenCount))
	sb.WriteString(fmt.Sprintf("  Uptime:  %s\n", h.Uptime))
	return sb.String()
}

// formatFiles formats a FilesResult for terminal display.
func formatFiles(result *socket.FilesResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ %d files%s\n", colorBold, result.Count, colorReset))
	for _, f := range result.Files {
		sb.WriteString(fmt.Sprintf("  %s%s%s", colorCyan, f.Path, colorReset))
		if f.Domain != "" {
			sb.WriteString(fmt.Sprintf("  %s%s%s", colorMagenta, f.Domain, colorReset))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatDomains formats a DomainsResult for terminal display.
func formatDomains(result *socket.DomainsResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ %d domains%s (%d core)\n", colorBold, result.Count, colorReset, result.CoreCount))
	for _, d := range result.Domains {
		tierColor := colorGreen
		if d.Tier == "context" {
			tierColor = colorGray
		}
		stateStr := ""
		if d.State != "active" {
			stateStr = fmt.Sprintf(" %s[%s]%s", colorYellow, d.State, colorReset)
		}
		sb.WriteString(fmt.Sprintf("  %s@%-20s%s %6.1f hits  %s%-7s%s  %s%s\n",
			colorMagenta, d.Name, colorReset,
			d.Hits,
			tierColor, d.Tier, colorReset,
			d.Source, stateStr))
	}
	return sb.String()
}

// formatStats formats a StatsResult for terminal display.
func formatStats(result *socket.StatsResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ aOa stats%s\n", colorBold, colorReset))
	sb.WriteString(fmt.Sprintf("  Prompts:     %d\n", result.PromptCount))
	sb.WriteString(fmt.Sprintf("  Domains:     %d (%d core, %d context)\n", result.DomainCount, result.CoreCount, result.ContextCount))
	sb.WriteString(fmt.Sprintf("  Keywords:    %d\n", result.KeywordCount))
	sb.WriteString(fmt.Sprintf("  Terms:       %d\n", result.TermCount))
	sb.WriteString(fmt.Sprintf("  Bigrams:     %d\n", result.BigramCount))
	sb.WriteString(fmt.Sprintf("  File hits:   %d\n", result.FileHitCount))
	sb.WriteString(fmt.Sprintf("  Index files: %d\n", result.IndexFiles))
	sb.WriteString(fmt.Sprintf("  Index tokens: %d\n", result.IndexTokens))
	return sb.String()
}
