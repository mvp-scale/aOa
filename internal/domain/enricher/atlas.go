package enricher

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// DomainDef is the JSON schema for a domain in the atlas.
// Each atlas file contains an array of DomainDefs.
type DomainDef struct {
	Domain string              `json:"domain"`
	Terms  map[string][]string `json:"terms"` // term name -> keywords
}

// KeywordMatch identifies a keyword's owning domain and term.
// Shared keywords appear in multiple domains, so Lookup returns a slice.
type KeywordMatch struct {
	Domain string
	Term   string
}

// Atlas holds the parsed universal domain structure.
type Atlas struct {
	Domains  []DomainDef
	Keywords map[string][]KeywordMatch // keyword -> owning domain/term pairs

	DomainCount     int
	TermCount       int
	KeywordEntries  int // total keyword entries across all terms (includes cross-domain duplicates)
	UniqueKeywords  int // unique keyword count (map keys)
}

// LoadAtlas reads all JSON files from an fs.FS directory and builds the atlas.
// Files are loaded in sorted order for deterministic results.
// Returns an error if any file fails to parse or if the atlas is empty.
func LoadAtlas(fsys fs.FS, dir string) (*Atlas, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read atlas dir %q: %w", dir, err)
	}

	// Sort for deterministic load order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var allDomains []DomainDef

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := dir + "/" + entry.Name()
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		var domains []DomainDef
		if err := json.Unmarshal(data, &domains); err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}

		allDomains = append(allDomains, domains...)
	}

	if len(allDomains) == 0 {
		return nil, fmt.Errorf("atlas is empty: no domains found in %q", dir)
	}

	// Build keyword -> domain/term index
	keywords := make(map[string][]KeywordMatch)
	termCount := 0
	keywordEntries := 0

	for _, d := range allDomains {
		for term, kws := range d.Terms {
			termCount++
			keywordEntries += len(kws)
			for _, kw := range kws {
				keywords[kw] = append(keywords[kw], KeywordMatch{
					Domain: d.Domain,
					Term:   term,
				})
			}
		}
	}

	return &Atlas{
		Domains:        allDomains,
		Keywords:       keywords,
		DomainCount:    len(allDomains),
		TermCount:      termCount,
		KeywordEntries: keywordEntries,
		UniqueKeywords: len(keywords),
	}, nil
}
