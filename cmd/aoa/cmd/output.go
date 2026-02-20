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
// Symbol hits render with symbols/ranges/tags; content hits render grep-style.
//
//	⚡ 15 hits (10 symbol, 5 content) │ Xms
//	  file:symbol[range]:line  @domain  #tag1 #tag2
//	  file:line: matching content text
func formatSearchResult(result *socket.SearchResult, countOnly, quiet bool) string {
	if quiet {
		return ""
	}

	if countOnly {
		return fmt.Sprintf("%s⚡ %d hits%s │ %s", colorBold, result.Count, colorReset, result.Elapsed)
	}

	// Count unique files
	fileSet := make(map[string]struct{})
	for _, hit := range result.Hits {
		fileSet[hit.File] = struct{}{}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ %d hits%s │ %d files │ %s\n",
		colorBold, len(result.Hits), colorReset, len(fileSet), result.Elapsed))

	for _, hit := range result.Hits {
		if hit.Kind == "content" {
			// Content hit: file:symbol[range]:line: content + tags (no domain)
			sb.WriteString(fmt.Sprintf("  %s%s%s:", colorCyan, hit.File, colorReset))

			// Enclosing symbol from tree-sitter (structural context)
			if hit.Symbol != "" {
				sb.WriteString(hit.Symbol)
			}
			if hit.Range[0] > 0 || hit.Range[1] > 0 {
				sb.WriteString(fmt.Sprintf("[%d-%d]", hit.Range[0], hit.Range[1]))
			}

			sb.WriteString(fmt.Sprintf(":%d: %s%s%s",
				hit.Line,
				colorGray, hit.Content, colorReset))

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
			continue
		}

		// Symbol hit: file:symbol[range]:line  @domain  #tags
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

