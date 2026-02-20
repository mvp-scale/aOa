package cmd

import (
	"fmt"
	"sort"
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
func formatSearchResult(result *socket.SearchResult, countOnly, quiet, noFilename, noColor bool) string {
	if quiet {
		return ""
	}

	// Resolve color codes — empty when --no-color is set
	cReset, cBold, cCyan, cMagenta, cGreen, cGray := colorReset, colorBold, colorCyan, colorMagenta, colorGreen, colorGray
	if noColor {
		cReset, cBold, cCyan, cMagenta, cGreen, cGray = "", "", "", "", "", ""
	}

	if countOnly {
		return fmt.Sprintf("%s⚡ %d hits%s │ %s", cBold, result.Count, cReset, result.Elapsed)
	}

	// Count unique files
	fileSet := make(map[string]struct{})
	for _, hit := range result.Hits {
		fileSet[hit.File] = struct{}{}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ %d hits%s │ %d files │ %s\n",
		cBold, len(result.Hits), cReset, len(fileSet), result.Elapsed))

	for _, hit := range result.Hits {
		// -L file-only hits
		if hit.Kind == "file" {
			if noFilename {
				continue
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s\n", cCyan, hit.File, cReset))
			continue
		}

		// Before-context lines
		if len(hit.ContextLines) > 0 {
			writeContextBefore(&sb, hit, noFilename, cCyan, cReset, cGray)
		}

		if hit.Kind == "content" {
			// Content hit: file:symbol[range]:line: content + tags (no domain)
			if !noFilename {
				sb.WriteString(fmt.Sprintf("  %s%s%s:", cCyan, hit.File, cReset))
			} else {
				sb.WriteString("  ")
			}

			// Enclosing symbol from tree-sitter (structural context)
			if hit.Symbol != "" {
				sb.WriteString(hit.Symbol)
			}
			if hit.Range[0] > 0 || hit.Range[1] > 0 {
				sb.WriteString(fmt.Sprintf("[%d-%d]", hit.Range[0], hit.Range[1]))
			}

			sb.WriteString(fmt.Sprintf(":%d: %s%s%s",
				hit.Line,
				cGray, hit.Content, cReset))

			if len(hit.Tags) > 0 {
				sb.WriteString("  ")
				for i, tag := range hit.Tags {
					if i > 0 {
						sb.WriteString(" ")
					}
					sb.WriteString(fmt.Sprintf("%s#%s%s", cGreen, tag, cReset))
				}
			}

			sb.WriteString("\n")
		} else {
			// Symbol hit: file:symbol[range]:line  @domain  #tags
			if !noFilename {
				sb.WriteString(fmt.Sprintf("  %s%s%s:", cCyan, hit.File, cReset))
			} else {
				sb.WriteString("  ")
			}

			if hit.Symbol != "" {
				sb.WriteString(hit.Symbol)
			}
			if hit.Range[0] > 0 || hit.Range[1] > 0 {
				sb.WriteString(fmt.Sprintf("[%d-%d]", hit.Range[0], hit.Range[1]))
			}
			sb.WriteString(fmt.Sprintf(":%d", hit.Line))

			if hit.Domain != "" {
				sb.WriteString(fmt.Sprintf("  %s%s%s", cMagenta, hit.Domain, cReset))
			}

			if len(hit.Tags) > 0 {
				sb.WriteString("  ")
				for i, tag := range hit.Tags {
					if i > 0 {
						sb.WriteString(" ")
					}
					sb.WriteString(fmt.Sprintf("%s#%s%s", cGreen, tag, cReset))
				}
			}

			sb.WriteString("\n")
		}

		// After-context lines
		if len(hit.ContextLines) > 0 {
			writeContextAfter(&sb, hit, noFilename, cCyan, cReset, cGray)
		}
	}

	return sb.String()
}

// writeContextBefore writes context lines that appear before the hit line.
func writeContextBefore(sb *strings.Builder, hit socket.SearchHit, noFilename bool, cCyan, cReset, cGray string) {
	lineNums := sortedContextLines(hit.ContextLines)
	for _, ln := range lineNums {
		if ln >= hit.Line {
			continue
		}
		writeContextLine(sb, hit.File, ln, hit.ContextLines[ln], noFilename, cCyan, cReset, cGray)
	}
}

// writeContextAfter writes context lines that appear after the hit line.
func writeContextAfter(sb *strings.Builder, hit socket.SearchHit, noFilename bool, cCyan, cReset, cGray string) {
	lineNums := sortedContextLines(hit.ContextLines)
	for _, ln := range lineNums {
		if ln <= hit.Line {
			continue
		}
		writeContextLine(sb, hit.File, ln, hit.ContextLines[ln], noFilename, cCyan, cReset, cGray)
	}
}

// writeContextLine writes a single context line in dimmed style.
func writeContextLine(sb *strings.Builder, file string, lineNum int, text string, noFilename bool, cCyan, cReset, cGray string) {
	if !noFilename {
		sb.WriteString(fmt.Sprintf("  %s%s%s", cCyan, file, cReset))
	} else {
		sb.WriteString(" ")
	}
	sb.WriteString(fmt.Sprintf(" %s│ %d: %s%s\n", cGray, lineNum, text, cReset))
}

// sortedContextLines returns the line numbers from a ContextLines map, sorted ascending.
func sortedContextLines(ctx map[int]string) []int {
	nums := make([]int, 0, len(ctx))
	for ln := range ctx {
		nums = append(nums, ln)
	}
	sort.Ints(nums)
	return nums
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
