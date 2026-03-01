// dim-dump reads the bbolt dimensional cache and exports method-level
// finding data as a JSON fixture for scoring formula iteration.
//
// Usage:
//
//	go run ./cmd/dim-dump/ > internal/domain/analyzer/testdata/real_methods.json
//
// The output file is loaded by TestRealWorldDistribution in the scoring
// experiment test, enabling fast formula iteration without rebuilding.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/domain/analyzer"
	reconfs "github.com/corey/aoa/recon"
)

// MethodRecord is the fixture format: one record per method, carrying
// the raw line × bit topology needed by scoring formulas.
type MethodRecord struct {
	Name       string      `json:"name"`
	File       string      `json:"file"`
	TotalLines int         `json:"total_lines"`
	Hits       []HitRecord `json:"hits"`
}

type HitRecord struct {
	Line     int `json:"line"`
	Tier     int `json:"tier"`
	Bit      int `json:"bit"`
	Severity int `json:"severity"`
}

func main() {
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error finding project root: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(root, ".aoa", "aoa.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "no bbolt db at %s — run 'aoa init' first\n", dbPath)
		os.Exit(1)
	}

	// Load YAML rules for ruleID → (tier, bit) mapping
	rules, err := analyzer.LoadRulesFromFS(reconfs.FS, "rules")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading rules: %v\n", err)
		os.Exit(1)
	}
	ruleMap := make(map[string]analyzer.Rule, len(rules))
	for _, r := range rules {
		ruleMap[r.ID] = r
	}

	// Copy the db to a temp file so we can read without conflicting with daemon's lock.
	tmpFile, err := os.CreateTemp("", "aoa-dim-dump-*.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating temp file: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading bbolt db: %v\n", err)
		os.Exit(1)
	}
	if _, err := tmpFile.Write(dbData); err != nil {
		tmpFile.Close()
		fmt.Fprintf(os.Stderr, "error writing temp db: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()

	store, err := bbolt.NewStore(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening bbolt copy: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	projectID := filepath.Base(root)
	analyses, err := store.LoadAllDimensions(projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading dimensions: %v\n", err)
		os.Exit(1)
	}
	if analyses == nil {
		fmt.Fprintf(os.Stderr, "no dimensional data found for project %q\n", projectID)
		os.Exit(1)
	}

	// Convert to fixture format
	var records []MethodRecord
	for filePath, fa := range analyses {
		for _, m := range fa.Methods {
			totalLines := m.EndLine - m.Line + 1
			if totalLines <= 0 {
				// Synthetic <file> method — count distinct finding lines
				lineSet := make(map[int]bool, len(m.Findings))
				for _, f := range m.Findings {
					lineSet[f.Line] = true
				}
				totalLines = len(lineSet)
				if totalLines == 0 {
					totalLines = 1
				}
			}

			var hits []HitRecord
			for _, f := range m.Findings {
				r, ok := ruleMap[f.RuleID]
				if !ok {
					continue
				}
				hits = append(hits, HitRecord{
					Line:     f.Line,
					Tier:     int(r.Tier),
					Bit:      r.Bit,
					Severity: int(f.Severity),
				})
			}
			if len(hits) == 0 {
				continue
			}

			records = append(records, MethodRecord{
				Name:       m.Name,
				File:       filePath,
				TotalLines: totalLines,
				Hits:       hits,
			})
		}
	}

	fmt.Fprintf(os.Stderr, "exported %d methods from %d files\n", len(records), len(analyses))

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(records); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".aoa")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (no .aoa/ or go.mod found)")
		}
		dir = parent
	}
}
